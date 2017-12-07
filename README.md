# Go packages and commands

This repository contains a collection of useful go packages and
cmds that I've built over the past few years.

## Commands

* deptrace: a simple command line tool to find how a particular package
  is imported by another. For example, the following will find the series
  of imports that lead grail-tidy to depend on testing.

  `
  deptrace --path grail.com/cmd/grail-tidy --src = $HOME/go/src/grail.com testing
  `



## Packages

* cmap: a simple concurrent mapper.

