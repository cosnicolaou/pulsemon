package internal

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"
)

var hostname string

func init() {
	hostname, _ = os.Hostname()
}

// Configuration represents the configuration for the pulsemon family of tools.
type Configuration struct {
	// SMTP configuration for alert emails.
	Server  string   `json:"smtp_server"`
	Port    string   `json:"smtp_port"`
	Domain  string   `json:"smtp_domain"`
	To      []string `json:"smtp_to"`
	From    string   `json:"smtp_from"`
	Subject string   `json:"smtp_subject"`

	// Set the time of day to send a status email at in HH:MM [+|-]0700 format.
	StatusEmailTime    string `json:"status_email_time"`
	StatusEmailSubject string `json:"status_email_subject"`

	// DST offset for the required timezone as a string in time.Duration format.
	DSTAdjustment string `json:"daylight_savings_adjustment"`

	// Alert configuation, if more than AlertPulses are counted
	// over AlertInterval then an email is sent.
	AlertInterval     string `json:"alert_interval"`
	IdleAlertInterval string `json:"idle_alert_interval"`
	LeakAlertInterval string `json:"leak_alert_interval"`
	AlertPulses       int64  `json:"alert_pulses"`

	// Number of gallons per pulse.
	GallonsPerPulse int `json:"gallons_per_pulse"`

	// Record the time of each pulse in binary, little endian, 64 bit unix
	// nanoseconds.
	PulseTimestampFile string `json:"pulse_timestamps_file"`

	// Parsed and processed configuration information.

	// AlertInterval as a time.Duration.
	AlertDuration time.Duration

	// IdleAlertInterval as a time.Duration
	IdleAlertDuration time.Duration

	// LeakAlertInterval as a time.Duration
	LeakAlertDuration time.Duration

	// StatusEmailTime as a time.Time.
	StatusTime time.Time
	// DSTAdjustment as a time.Duration.
	DSTAdjustmentDuration time.Duration

	PollingInterval int `json:"polling_interval_ms"`

	// Hardware specific configuration, doesn't really belong here. Set to
	// -1 to disable.
	InputPin          int `json:"input_pin"`
	InputDebounceMS   int `json:"input_debounce_ms"`
	OutputRelayPin    int `json:"relay_pin"`
	OutputRelayHoldMS int `json:"relay_hold_ms"`
	OutputPin         int `json:"output_pin"`
	OutputPinHoldMS   int `json:"output_hold_ms"`
}

// ReadConfig reads the configuration from the specified file.
func ReadConfig(filename string, config *Configuration) error {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read: %v", filename)
	}
	if err := json.Unmarshal(buf, config); err != nil {
		return fmt.Errorf("failed to unmarshal %v: %v", filename, err)
	}

	interval, err := time.ParseDuration(config.AlertInterval)
	if err != nil {
		return fmt.Errorf("failed to parse alert_interval %q as time.Duration: %v", config.AlertInterval, err)
	}

	idle, err := time.ParseDuration(config.IdleAlertInterval)
	if err != nil {
		return fmt.Errorf("failed to parse idle_alert_interval %q as time.Duration: %v", config.IdleAlertInterval, err)
	}

	leak, err := time.ParseDuration(config.LeakAlertInterval)
	if err != nil {
		return fmt.Errorf("failed to parse leak_alert_interval %q as time.Duration: %v", config.LeakAlertInterval, err)
	}

	emailAt, err := time.Parse("15:04 -0700", config.StatusEmailTime)
	if err != nil {
		return fmt.Errorf("failed to parse %q in 15:04 -0700 format", config.StatusEmailTime)
	}
	config.DSTAdjustmentDuration, err = time.ParseDuration(config.DSTAdjustment)
	if err != nil {
		return fmt.Errorf("failed to parse %q as a time.Duration", config.DSTAdjustment)
	}
	if time.Now().IsDST() {
		emailAt.Add(config.DSTAdjustmentDuration)
	}

	config.StatusTime = emailAt
	config.AlertDuration = interval
	config.IdleAlertDuration = idle
	config.LeakAlertDuration = leak
	return nil
}

type SMTPClient struct {
	to                          []string
	host, domain, from          string
	port                        int
	alertSubject, statusSubject string
}

func (sc *SMTPClient) String() string {
	return fmt.Sprintf("%v: from %v, to %v, alert subject: %v, status subject %v", sc.host, sc.from, strings.Join(sc.to, ","), sc.alertSubject, sc.statusSubject)
}

// ConfigureEmail configures and optional tests an smtp email client by sending
// a 'hello' message.
func (config *Configuration) ConfigureEmail(sendHello bool) (*SMTPClient, error) {
	client := &SMTPClient{
		host:          config.Server,
		to:            config.To,
		from:          config.From,
		domain:        config.Domain,
		alertSubject:  config.Subject,
		statusSubject: config.StatusEmailSubject,
	}
	if len(client.host) == 0 {
		return nil, nil
	}
	port, err := strconv.Atoi(config.Port)
	if err != nil {
		return nil, err
	}
	dailyIn := UntilHHMM(config.StatusTime)
	client.port = port
	err = client.Status("", fmt.Sprintf("%v started on %v @ %v (next daily email for %v, %v UTC in %v)\n", os.Args[0], hostname, time.Now(), HHMM(config.StatusTime), HHMM(config.StatusTime.UTC()), dailyIn))
	if err != nil {
		return nil, err
	}
	fmt.Printf("sent hello email to %v\n", strings.Join(client.to, ","))
	return client, nil
}

// Alert sends an alert email.
func (sc *SMTPClient) Alert(body string) error {
	return sc.Send(sc.alertSubject, body)
}

// Status sends a status email.
func (sc *SMTPClient) Status(trailer, body string) error {
	return sc.Send(sc.statusSubject+trailer, body)
}

// Send sends a generic email.
func (sc *SMTPClient) Send(subject, body string) error {
	if sc == nil {
		return nil
	}
	msg := fmt.Sprintf("To: %v\r\nSubject: %v\r\n\r\n%v\r\nHost: %v\r\n",
		sc.to, subject, body, hostname)

	server := mail.NewSMTPClient()

	// SMTP Server
	server.Host = sc.host
	server.Port = sc.port
	server.Helo = sc.domain
	server.Encryption = mail.EncryptionSTARTTLS
	server.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// generate message ID
	now := time.Now()
	msgID := fmt.Sprintf("<%v.%v@cloudeng.io",
		now.UTC().Unix(),
		now.UTC().UnixMilli())

	smtpClient, err := server.Connect()

	if err != nil {
		return err
	}

	email := mail.NewMSG()
	email.SetFrom(sc.from).
		SetSubject(subject).
		AddTo(sc.to[0]).
		AddHeader("Message-ID", msgID).
		SetBody(mail.TextPlain, msg)

	if err := email.Send(smtpClient); err != nil {
		return fmt.Errorf("smtp.SendMail failed: %v, from: %v, to: %v: %v", sc.host, sc.from, sc.to, err)
	}
	return err
}
