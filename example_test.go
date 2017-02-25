package trace_test

import (
	"fmt"
	"os"

	trace "github.com/cstockton/go-trace"
	"github.com/cstockton/go-trace/event"
)

func Example() {
	f, err := os.Open(`internal/tracefile/testdata/go1.8/log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}
	defer f.Close()

	// var list event.List
	// stream := trace.Stream(f)
	//
	// fmt.Println(`done`, stream)

	// Output:
	//
}

func Example_visit() {
	f, err := os.Open(`internal/tracefile/testdata/go1.8/log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}
	defer f.Close()

	var (
		list event.List
	)
	if err := trace.Visit(f, &list); err != nil {
		fmt.Println(`Err:`, err)
	}

	fmt.Println(`done`, list)

	// Output:
	//
}

// func Example_correlator() {
// 	f, err := os.Open(`internal/tracefile/testdata/go1.8/log.trace`)
// 	if err != nil {
// 		fmt.Println(`Err:`, err)
// 		return
// 	}
// 	defer f.Close()
//
// 	c := trace.Correlate(f)
// 	correlate.With
//
// 	fmt.Println(`done`)
//
// 	// Output:
// 	//
// }

/*
	var i, created int
	evt := new(event.Event)
	dec := encoding.NewDecoder(bytes.NewReader(data))
	for dec.More() {
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
*/
