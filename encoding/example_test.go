package encoding_test

import (
	"fmt"
	"os"

	"github.com/cstockton/go-trace/encoding"
	"github.com/cstockton/go-trace/event"
)

func Example() {
	f, err := os.Open(`../internal/tracefile/testdata/go1.8/log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}
	defer f.Close()

	var (
		created int
		evt     event.Event
		dec     = encoding.NewDecoder(f)
	)
	for i := 1; dec.More(); i++ {
		evt.Reset()
		if err := dec.Decode(&evt); err != nil {
			break // err will be in Err()
		}
		if evt.Type == event.EvGoCreate {
			created++ // Count all the GoCreate events.
		}
		if i%40 == 0 {
			fmt.Println(evt.Type) // printing a sampling of data
		}
	}
	if err := dec.Err(); err != nil {
		fmt.Println(`Err:`, err)
	}
	fmt.Printf("\nCreated %v goroutines\n", created)

	// Output:
	// event.HeapAlloc
	// event.HeapAlloc
	// event.HeapAlloc
	// event.GoCreate
	// event.ProcStop
	// event.String
	// event.String
	// event.Stack
	//
	// Created 12 goroutines
}
