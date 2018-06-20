# Go packages and commands

This repository contains a collection of useful go packages and
cmds that I've built over the past few years.

## Commands

* deptrace: a simple command line tool to find how a particular package
  is imported by another. For example, the following will find the series
  of imports that lead grail-tidy to depend on testing. I've found it be much
  more useful than the various graphical tools for visualizing dependencies
  that don't seem to work that well. It's written to be parallized but I
  haven't needed to do so yet.

  `
  deptrace --path grail.com/cmd/grail-tidy --src = $HOME/go/src/grail.com testing
  `

## pulsemon - pulse counting for a water meter

pulsemon implements a simple water meter monitor using the piface board and
a water meter with a reed switch built in. It supports alerting, logging
of timestamps and is easily dockerized. It's also a decent example of how
simple it is to use go concurrency for monitoring applications and avoids
the complexity of interrupts and callbacks (which is handy since interrupts
are not currently implemented for the piface board!).

## Packages

* cmap: a simple concurrent mapper.

```go
	sleeper := func(v interface{}) interface{} {
		n := v.(int)
		time.Sleep(time.Millisecond * 200)
		return -n
	}
	input := make(cmap.Vector, 1000)
	for i := range input {
		input[i] = i
	}
	then := time.Now()
	output := cmap.Map(input, sleeper, 100)
	taken := time.Now().Sub(then)
	if taken < 10*time.Second {
		fmt.Printf("nice and fast!\n")
	}
	fmt.Printf("len: %v\n", len(output))
	// Output: nice and fast!
	// len: 1000
```

