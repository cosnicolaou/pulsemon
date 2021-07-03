// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloudeng.io/cmdutil/subcmd"
	"github.com/cosnicolaou/pulsemon/internal"
)

type CommonFlags struct {
	TimeZoneLocation string `subcmd:"location,Local,'timezone specified as a location to use for interpreting times,'"`
}

type dumpFlags struct {
	StartDate string `subcmd:"start,,start of time period in MM-DD-YY or DD-MM-YY:HH:MM format"`
	EndDate   string `subcmd:"end,,end of time period in MM-DD-YY or DD-MM-YY:HH:MM format"`
	CommonFlags
}

type usageFlags struct {
	CommonFlags
	StartDate       string `subcmd:"start,,start of time period in MM-DD-YY format"`
	EndDate         string `subcmd:"end,,end of time period in MM-DD-YY format"`
	GallonsPerPulse int    `subcmd:"gallons-per-pulse,10,number of gallons per relay/meter pulse"`
	Period          string `subcmd:"period,24h,time period for usage calculations"`
}

var cmdSet *subcmd.CommandSet

func init() {
	dumpFS := subcmd.MustRegisterFlagStruct(&dumpFlags{}, nil, nil)
	dumpCmd := subcmd.NewCommand("dump", dumpFS, dumpTimestamps, subcmd.OptionalSingleArgument())
	dumpCmd.Document("dump the contents of a time stamp file, optionally within a specified time range. The output is in tsv format.")

	periodFS := subcmd.MustRegisterFlagStruct(&usageFlags{}, nil, nil)
	periodCmd := subcmd.NewCommand("usage", periodFS, usageCalculation, subcmd.OptionalSingleArgument())
	periodCmd.Document("calculate the usage over a given time period.")
	cmdSet = subcmd.NewCommandSet(dumpCmd, periodCmd)
}

func main() {
	ctx := context.Background()
	cmdSet.MustDispatch(ctx)

}

func parseDateOrTime(d string, def time.Time, loc *time.Location) (time.Time, error) {
	if len(d) == 0 {
		return def, nil
	}
	t, err := time.ParseInLocation("01-02-06", d, loc)
	if err == nil {
		return t, nil
	}
	return time.ParseInLocation("01-02-06:15:04", d, loc)
}

func parseDate(d string, def time.Time, loc *time.Location) (time.Time, error) {
	if len(d) == 0 {
		return def, nil
	}
	return time.ParseInLocation("01-02-06", d, loc)
}

func dumpTimestamps(ctx context.Context, values interface{}, args []string) error {
	cl := values.(*dumpFlags)
	ts := "-"
	if len(args) > 0 {
		ts = args[0]
	}

	location, err := time.LoadLocation(cl.TimeZoneLocation)
	if err != nil {
		return err
	}
	start, err := parseDateOrTime(cl.StartDate, time.Time{}, location)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %v", err)
	}
	end, err := parseDateOrTime(cl.EndDate, time.Now(), location)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %v", err)
	}
	return internal.ReadTimestamps(ts, start, end)
}

func usageCalculation(ctx context.Context, values interface{}, args []string) error {
	cl := values.(*usageFlags)

	location, err := time.LoadLocation(cl.TimeZoneLocation)
	if err != nil {
		return err
	}

	ts := os.Stdin
	if len(args) > 0 {
		var err error
		ts, err = os.Open(args[0])
		if err != nil {
			return err
		}
		defer ts.Close()
	}
	period, err := time.ParseDuration(cl.Period)
	if err != nil {
		return fmt.Errorf("failed to parse time period: %v", err)
	}
	start, err := parseDate(cl.StartDate, time.Time{}, location)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %v", err)
	}
	end, err := parseDate(cl.EndDate, time.Now(), location)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %v", err)
	}

	var (
		pulses        = 0
		totalPulses   = 0
		nextPeriodEnd time.Time
	)

	fmt.Printf("date\tpulses\tgallons\ttotal-pulses\ttotal-gallons\n")
	sc := internal.NewTimestampFileScanner(ts)
	for sc.Scan() {
		ns := sc.Time()
		if start.After(ns) || end.Before(ns) {
			continue
		}
		if nextPeriodEnd.IsZero() {
			nextPeriodEnd = ns.Add(-(period / 2)).Round(period).Add(period)
		}
		pulses++
		totalPulses++
		if ns.After(nextPeriodEnd) {
			fmt.Printf("%v\t%v\t%v\t%v\t%v\n", nextPeriodEnd.Format("01/02/06:15:04"), pulses, pulses*cl.GallonsPerPulse,
				totalPulses, totalPulses*cl.GallonsPerPulse)
			nextPeriodEnd = nextPeriodEnd.Add(period)
			pulses = 0
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

/*
// ReadTimestamps read and print the timestamps.
func ReadTimestamps(filename string, from, to time.Time) error {
	var rd *os.File
	var err error
	if filename == "-" {
		rd = os.Stdin
	} else {
		rd, err = os.Open(filename)
		if err != nil {
			return err
		}
	}
	defer rd.Close()
	pulseCounter := 0
	fmt.Printf("pulse\tnanosecond\ttime\n")
	sc := NewTimestampFileScanner(rd)
	for sc.Scan() {
		pulseCounter++
		ns := sc.Time()
		if from.After(ns) || to.Before(ns) {
			continue
		}
		fmt.Printf("%v\t%v\t%v\n", pulseCounter, ns.UnixNano(), ns)
	}
	return sc.Err()
}*/
