// Copyright 2017 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package cmap_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cosnicolaou/go/cmap"
)

func ExampleMap() {
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
}

func TestMap(t *testing.T) {
	nitems := 1000
	sleep := time.Millisecond * 200
	record := sync.Map{}
	sleeper := func(v interface{}) interface{} {
		n := v.(int)
		record.Store(n, true)
		time.Sleep(sleep)
		return -n
	}
	input := make(cmap.Vector, nitems)
	for i := range input {
		input[i] = i
	}
	then := time.Now()
	output := cmap.Map(input, sleeper, 100)
	taken := time.Now().Sub(then)
	if taken > 10*time.Second {
		t.Errorf("too slow")
	}
	if got, want := len(output), len(input); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	for i := range input {
		if got, want := input[i].(int), output[i].(int); got != -want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
