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



## Packages

* cmap: a simple concurrent mapper.

