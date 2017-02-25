package encoding_test

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/cstockton/go-trace/encoding"
	"github.com/cstockton/go-trace/event"
)

func Example() {
	data, err := ioutil.ReadFile(`../internal/tracefile/testdata/go1.8/log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}

	var i, created int
	evt := new(event.Event)
	dec := encoding.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		evt.Reset()
		err := dec.Decode(evt)
		if err != nil {
			break // err will be in Err()
		}
		if evt.Type == event.EvGoCreate {
			created++
		}
		if i += 1; i%40 == 0 {
			fmt.Println(evt.Type) // printing a sampling of data
		}
	}
	if err := dec.Err(); err != nil {
		fmt.Println(`Err:`, err)
	}
	fmt.Printf("\nCreated %v goroutines\n", created)

	// Output:
	// encoding.HeapAlloc
	// encoding.HeapAlloc
	// encoding.HeapAlloc
	// encoding.GoCreate
	// encoding.ProcStop
	// encoding.String
	// encoding.String
	// encoding.Stack
	//
	// Created 12 goroutines
}
