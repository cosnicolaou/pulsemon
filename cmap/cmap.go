// Copyright 2017 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Package cmap provides a simple, concurrent, map operation.
package cmap

import (
	"sort"
	"sync"
)

// Vector represents the data to be mapped and the results of that mapping.
type Vector []interface{}

type item struct {
	payload interface{}
	order   int
}

type Mapper func(interface{}) interface{}

// Map uses the requested number of goroutines to apply fn to each element
// in vector. The resulting vector will be the same size as the original and
// will be correpsonding order, that is, result[i] = fn(in[i]).
func Map(in Vector, fn Mapper, nGoroutines int) Vector {

	workCh := make(chan *item, len(in))
	resultCh := make(chan item, len(in))

	results := make([]item, 0, len(in))
	items := make([]item, len(in))
	for i, v := range in {
		items[i].payload = v
		items[i].order = i
		workCh <- &items[i]
	}
	close(workCh)

	var waiter sync.WaitGroup
	waiter.Add(nGoroutines)

	for i := 0; i < nGoroutines; i++ {
		go func() {
			for {
				select {
				case work := <-workCh:
					if work == nil {
						waiter.Done()
						return
					}
					result := item{order: work.order}
					result.payload = fn(work.payload)
					resultCh <- result
				}
			}

		}()
	}
	waiter.Wait()
	close(resultCh)

	for r := range resultCh {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].order < results[j].order })

	returned := make(Vector, len(in))
	for i, r := range results {
		returned[i] = r.payload
	}
	return returned
}
