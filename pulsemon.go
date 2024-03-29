package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cosnicolaou/pulsemon/internal"
	"github.com/luismesas/goPi/piface"
	"github.com/luismesas/goPi/spi"
)

var (
	// number of pulses since start.
	pulseCounter int64
	// channel used to send pulse timestamps from the polling loop
	// to any other interested process
	pulseTimes chan time.Time

	configFileFlag string
	verboseFlag    bool
	globalConfig   internal.Configuration
)

func init() {
	flag.StringVar(&configFileFlag, "config", "", "configuration file in JSON format")
	flag.BoolVar(&verboseFlag, "verbose", false, "output debug/trace information to the console")
}

func main() {
	flag.Parse()
	if err := internal.ReadConfig(configFileFlag, &globalConfig); err != nil {
		panic(err)
	}

	pollingInterval := time.Duration(globalConfig.PollingInterval) * time.Millisecond
	pulseMeterPin := globalConfig.InputPin
	debounceDuration := time.Duration(globalConfig.InputDebounceMS) * time.Millisecond
	relayPin := globalConfig.OutputRelayPin
	relayHold := time.Duration(globalConfig.OutputRelayHoldMS) * time.Millisecond
	switchPin := globalConfig.OutputPin
	switchHold := time.Duration(globalConfig.OutputPinHoldMS) * time.Millisecond

	smtpClient, err := globalConfig.ConfigureEmail(true)
	if err != nil {
		panic(err)
	}
	if smtpClient == nil {
		fmt.Printf("email alerts are not configured")
	}

	timestampWriter, err := internal.NewTimestampFileWriter(
		globalConfig.PulseTimestampFile)
	if err != nil {
		panic(err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)

	pulseTimes = make(chan time.Time, 1024)

	// creates a new pifacedigital instance
	pfd := piface.NewPiFaceDigital(spi.DEFAULT_HARDWARE_ADDR, spi.DEFAULT_BUS, spi.DEFAULT_CHIP)

	// Initializes pifacedigital board
	if err := pfd.InitBoard(); err != nil {
		fmt.Printf("Error on init board: %s", err)
		return
	}

	// Log to console and append to the timestamp file.
	go console(pfd, timestampWriter, smtpClient, pulseTimes)

	// Generate an alert if a certain number of pulses per time period
	// are counted.
	go alert(globalConfig.AlertDuration,
		globalConfig.AlertPulses,
		int64(globalConfig.GallonsPerPulse),
		smtpClient)

	go idleAndLeak(globalConfig.IdleAlertDuration, globalConfig.LeakAlertDuration, smtpClient)

	// Poll for pulses.
	go poll(pfd, pulseMeterPin, pollingInterval, debounceDuration, pulseTimes)

	if relayPin >= 0 {
		go forwardRelay(pfd, 100*time.Millisecond, relayPin, relayHold)
	}

	if switchPin >= 0 {
		go forwardSwitch(pfd, 100*time.Millisecond, switchPin, switchHold)
	}

	// Send a daily email.
	go daily(globalConfig.StatusTime, int64(globalConfig.GallonsPerPulse), smtpClient)

	<-sigch
	fmt.Printf("closing %v\n", globalConfig.PulseTimestampFile)
	timestampWriter.Close()
}

func console(pfd *piface.PiFaceDigital,
	timestampFile *internal.TimestampFileWriter,
	smtp *internal.SMTPClient,
	pulseTimes <-chan time.Time) {
	var prev, cur int64
	storage := make([]byte, 0, 128)
	var buf []byte
	pfd.Leds[4].SetValue(0)
	pfd.Leds[5].SetValue(0)
	pfd.Leds[6].SetValue(0)

	for {
		time.Sleep(500 * time.Millisecond)
		cur = atomic.LoadInt64(&pulseCounter)
		if cur != prev {
			prev = cur
			val := byte(cur & 0xff)
			pfd.Leds[4].SetValue(val & 0x01)
			pfd.Leds[5].SetValue((val & 0x02) >> 1)
			pfd.Leds[6].SetValue((val & 0x04) >> 2)
			pfd.Leds[7].SetValue((val & 0x08) >> 3)
			buf = strconv.AppendInt(storage, cur, 10)
			now := time.Now().String()
			buf = append(buf, ' ', '-', ' ')
			buf = append(buf, []byte(now)...)
			buf = append(buf, '\n')
			os.Stderr.Write(buf)
			n := 0
			for {
				// drain all event times.
				select {
				case event := <-pulseTimes:
					if err := timestampFile.Append(event); err != nil {
						msg := fmt.Sprintf("ERROR appending to timestamp file: %v", err)
						fmt.Fprintf(os.Stderr, "%s\n", msg)
						if err := smtp.Alert(msg); err != nil {
							fmt.Fprintf(os.Stderr, "ERROR sending email: %v", err)
						}
					}
					n++
				default:
					goto drained
				}
			}
		drained:
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "drained %v pulse timestamps\n", n)
			}
		}
	}
}

func alert(interval time.Duration, pulses int64, gallonsPerPulse int64, smtp *internal.SMTPClient) {
	last := atomic.LoadInt64(&pulseCounter)
	for {
		time.Sleep(interval)
		cur := atomic.LoadInt64(&pulseCounter)
		if seen := cur - last; seen > pulses {
			msg := fmt.Sprintf("ALERT: %v gallons over %v: %v\n", seen*gallonsPerPulse, interval, time.Now())
			os.Stdout.WriteString(msg)
			if err := smtp.Alert(msg); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR sending email: %v", err)
			}
		}
		last = cur
	}
}

func idleAndLeak(idleInterval, leakInterval time.Duration, smtp *internal.SMTPClient) {
	last := atomic.LoadInt64(&pulseCounter)
	leakStart := time.Now()
	idle := false
	for {
		time.Sleep(idleInterval)
		cur := atomic.LoadInt64(&pulseCounter)
		if seen := cur - last; seen == 0 {
			msg := fmt.Sprintf("ALERT: no water flow for %v: %v\n", idleInterval, time.Now())
			os.Stdout.WriteString(msg)
			if err := smtp.Alert(msg); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR sending email: %v", err)
			}
			idle = true
		}
		last = cur
		if time.Now().After(leakStart.Add(leakInterval)) {
			if !idle {
				msg := fmt.Sprintf("ALERT: POSSIBLE LEAK: no idle period for %v: %v\n", leakInterval, time.Now())
				os.Stdout.WriteString(msg)
				if err := smtp.Alert(msg); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR sending email: %v", err)
				}
			}
			leakStart = time.Now()
			idle = false
		}
	}
}

func poll(pfd *piface.PiFaceDigital, pin int, interval, debounce time.Duration, pulseTimes chan<- time.Time) {
	fmt.Printf("polling pin %v, interval %v, debounce duration %v\n", pin, interval, debounce)
	debounceCount := int(debounce / interval)
	count := debounceCount
	for {
		time.Sleep(interval)
		val := pfd.InputPins[pin].Value()
		if val == 0 {
			// Circuit is open.
			if count < 0 {
				count = debounceCount
			}
			continue
		}
		// Circuit is closed.

		// Debounce by waiting for debounceCount iterations before
		// counting a pulse. Once a pulse is counted, let the counter
		// run negative until the pin reads 0 again; ie. a rising
		// edge trigger for a pulse longer than debouceCount is counted.
		count--
		if count == 0 {
			atomic.AddInt64(&pulseCounter, 1)
			pulseTimes <- time.Now()
		}
	}
}

func forwardRelay(pfd *piface.PiFaceDigital, interval time.Duration, relayPin int, relayHold time.Duration) {
	fmt.Printf("relay pin %v\n", relayPin)
	pfd.Relays[relayPin].AllOff()
	last := atomic.LoadInt64(&pulseCounter)
	for {
		time.Sleep(interval)
		cur := atomic.LoadInt64(&pulseCounter)
		if seen := cur - last; seen > 0 {
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "Forwarding %v pulses via a relay\n", seen)
			}
			for i := int64(0); i < seen; i++ {
				pfd.Relays[relayPin].AllOn()
				time.Sleep(relayHold)
				pfd.Relays[relayPin].AllOff()
			}
		}
		last = cur
	}
}

func forwardSwitch(pfd *piface.PiFaceDigital, interval time.Duration, outputPin int, outputHold time.Duration) {
	fmt.Printf("Output pin %v\n", outputPin)
	pfd.OutputPins[outputPin].AllOff()
	last := atomic.LoadInt64(&pulseCounter)
	for {
		time.Sleep(interval)
		cur := atomic.LoadInt64(&pulseCounter)
		if seen := cur - last; seen > 0 {
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "Forwarding %v pulses via cmos output\n", seen)
			}
			for i := int64(0); i < seen; i++ {
				pfd.OutputPins[outputPin].AllOn()
				time.Sleep(outputHold)
				pfd.OutputPins[outputPin].AllOff()
			}
		}
		last = cur
	}
}

var dstStr = map[bool]string{
	true:  "Daylight Savings Time",
	false: "Standard Time",
}

func daily(hhmm time.Time, gallonsPerPulse int64, smtp *internal.SMTPClient) {
	prev := atomic.LoadInt64(&pulseCounter)
	for {
		duration := internal.UntilHHMM(hhmm)
		dst := time.Now().IsDST()
		fmt.Printf("next daily email at %v in %v (%v)\n", internal.HHMM(hhmm), duration, dstStr[dst])
		<-time.After(duration)
		// send email
		cur := atomic.LoadInt64(&pulseCounter)
		seen := cur - prev
		gallons := seen * gallonsPerPulse
		msg := fmt.Sprintf("DAILY USAGE: %v gallons over %v @ %v\n",
			gallons,
			duration.Round(time.Minute),
			time.Now().Format(time.RFC822),
		)
		if err := smtp.Status(fmt.Sprintf(" %v gallons", gallons), msg); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR sending email: %v", err)
		}
		prev = cur
	}
}
