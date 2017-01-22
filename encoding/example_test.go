package encoding_test

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/cstockton/go-trace/encoding"
)

func Example() {
	data, err := ioutil.ReadFile(`testdata/go1.8rc1/tiny_log.trace`)
	if err != nil {
		fmt.Println(`Err:`, err)
		return
	}

	var i, created int
	dec := encoding.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		evt, err := dec.Decode()
		if err != nil {
			break // err will be in Err()
		}
		if evt.Type() == encoding.EvGoCreate {
			created++
		}
		if i += 1; i%40 == 0 {
			fmt.Println(evt) // printing a sampling of data
		}
	}
	if err := dec.Err(); err != nil {
		fmt.Println(`Err:`, err)
	}
	fmt.Printf("Created %v goroutines\n", created)

	// Output:
	// encoding.HeapAlloc
	// encoding.HeapAlloc
	// encoding.HeapAlloc
	// encoding.GoStartLocal
	// encoding.ProcStop
	// encoding.String("/one/ws/godev1.8/go/src/time/sys_unix.go")
	// encoding.String("testing.(*M).before")
	// encoding.Stack[3]:
	// testing.(*M).before [PC 4917480]
	// 	/one/ws/godev1.8/go/src/testing/testing.go:914
	// testing.(*M).Run [PC 4913544]
	// 	/one/ws/godev1.8/go/src/testing/testing.go:815
	// main.main [PC 5212775]
	// 	log/_test/_testmain.go:56
	//
	// Created 12 goroutines
}
