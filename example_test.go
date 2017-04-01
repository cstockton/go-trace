package trace_test

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"runtime/trace"
	"sync"
	"time"

	"github.com/cstockton/go-trace/encoding"
	"github.com/cstockton/go-trace/event"
)

func Example() {
	f, err := os.Open(`internal/tracefile/testdata/go1.8/log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}
	defer f.Close()

	var (
		evt event.Event
		d   = encoding.NewDecoder(f)
	)
	for d.More() {
		evt.Reset()
		if err := d.Decode(&evt); err != nil {
			break
		}
		if evt.Type == event.EvGoSysCall {
			fmt.Println(evt.Type) // print syscall events
		}
	}
	if err := d.Err(); err != nil {
		fmt.Println(`Err: `, err)
		return
	}

	// Output:
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
	// event.GoSysCall
}

func Example_runtimeDecoding() {
	sleepFn := func(wg *sync.WaitGroup) {
		defer wg.Done()
		<-time.After(time.Millisecond * 100)
	}

	r, w := io.Pipe()
	trace.Start(w)

	var wg sync.WaitGroup
	wg.Add(3)
	go sleepFn(&wg)
	go sleepFn(&wg)
	go sleepFn(&wg)
	wg.Wait()

	go func() {
		defer w.Close()
		trace.Stop()
	}()

	var (
		dec  = encoding.NewDecoder(r)
		evt  event.Event
		evts []*event.Event
	)

	v, err := dec.Version()
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}

	tr, err := event.NewTrace(v)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}

	for dec.More() {
		evt.Reset()
		if err := dec.Decode(&evt); err != nil {
			break
		}
		if err := tr.Visit(&evt); err != nil {
			fmt.Println(`Err:`, err)
		}
		evts = append(evts, evt.Copy())
	}
	if err := dec.Err(); err != nil {
		fmt.Println(`Err: `, err)
		return
	}

	// Lets make sure a goroutine is started for this func
	findPC := uint64(reflect.ValueOf(sleepFn).Pointer())
	findName := runtime.FuncForPC(uintptr(findPC)).Name()

	for _, e := range evts {
		if e.Type != event.EvGoCreate {
			continue
		}

		// We want a stack for the new StackID
		stack := tr.Stacks[e.Get(`NewStackID`)]
		if len(stack) < 1 {
			continue
		}

		name := runtime.FuncForPC(uintptr(stack[0].PC())).Name()
		if findName == name {
			stack, ok := tr.Stacks[e.Get(`StackID`)]
			if !ok {
				fmt.Println(`No stack exists for event:`, e)
			}

			// can't print stack in a unit test due to lack of determnism
			// fmt.Println(stack)
			fmt.Printf("\nFound EvGoCreate for `sleepFn` with new stack:\n====\n")
			for _, frame := range stack {
				fmt.Printf("  %v\n", frame.Func())
			}
		}
	}

	// Output:
	// Found EvGoCreate for `sleepFn` with new stack:
	// ====
	//   github.com/cstockton/go-trace_test.Example_runtimeDecoding
	//   testing.runExample
	//   testing.runExamples
	//   testing.(*M).Run
	//   main.main
	//
	// Found EvGoCreate for `sleepFn` with new stack:
	// ====
	//   github.com/cstockton/go-trace_test.Example_runtimeDecoding
	//   testing.runExample
	//   testing.runExamples
	//   testing.(*M).Run
	//   main.main
	//
	// Found EvGoCreate for `sleepFn` with new stack:
	// ====
	//   github.com/cstockton/go-trace_test.Example_runtimeDecoding
	//   testing.runExample
	//   testing.runExamples
	//   testing.(*M).Run
	//   main.main

}
