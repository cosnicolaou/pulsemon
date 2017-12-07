// Copyright 2017 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"path"
	"regexp"
	"strings"
	"sync"
)

var (
	pathFlag string
	srcFlag  string
)

func init() {
	flag.StringVar(&pathFlag, "path", "", "path to go package or command")
	flag.StringVar(&srcFlag, "src", "", "root of source code tree, needed for vendor imports")
}

func main() {
	flag.Parse()
	targets := flag.Args()
	if len(pathFlag) == 0 {
		log.Fatalf("please provide --path")
	}
	for _, target := range targets {
		re, err := regexp.Compile(target)
		if err != nil {
			log.Fatalf("failed to parse %v: %v", target, err)
		}
		path, found := find(0, pathFlag, re)
		if found {
			fmt.Printf("%v imports %v via %v\n", pathFlag, target, path)
		} else {
			fmt.Printf("%v does not depend on %v\n", pathFlag, target)
		}
	}
}

type result struct {
	path  string
	found bool
}

var pkgMemo sync.Map

func find(level int, pkgPath string, target *regexp.Regexp) (string, bool) {
	if pkgPath == "C" {
		pkgMemo.Store(pkgPath, result{"", false})
		return "", false
	}
	r, ok := pkgMemo.Load(pkgPath)
	if ok {
		tmp := r.(result)
		return tmp.path, tmp.found
	}
	if strings.HasPrefix(pkgPath, "golang_org") {
		pkgPath = path.Join("vendor", pkgPath)
	}
	pkg, err := build.Import(pkgPath, srcFlag, 0)
	if err != nil {
		log.Fatal(err)
	}
	for _, pkg := range pkg.Imports {
		if target.MatchString(pkg) {
			p := pkgPath + " -> " + pkg
			pkgMemo.Store(pkg, result{p, true})
			return p, true
		}
		if next, found := find(level+1, pkg, target); found {
			return pkgPath + " -> " + next, true
		}
	}
	pkgMemo.Store(pkgPath, result{"", false})
	return "", false
}
