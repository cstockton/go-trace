# Go Package: trace

  [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/cstockton/go-trace)
  [![Go Report Card](https://goreportcard.com/badge/github.com/cstockton/go-trace?style=flat-square)](https://goreportcard.com/report/github.com/cstockton/go-trace)
  [![Coverage Status](https://img.shields.io/codecov/c/github/cstockton/go-trace/master.svg?style=flat-square)](https://codecov.io/github/cstockton/go-trace?branch=master)
  [![Build Status](http://img.shields.io/travis/cstockton/go-trace.svg?style=flat-square)](https://travis-ci.org/cstockton/go-trace)
  [![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/cstockton/go-trace/master/LICENSE)

  > Get:
  > ```bash
  > go get -u github.com/cstockton/go-trace
  > ```
  >
  > Example:
  > ```Go
  > mknod trace.fifo p
  > go test -trace trace.fifo &
  > tracecat trace.fifo | grep GoCreate | wc -l
  > ```
  >
  > Output:
  > ```Go
  > PASS
  > ok  	github.com/cstockton/go-trace/encoding	2.208s
  > 335 <===== 335 goroutines created
  > [1]+  Done                    go test -trace trace.fifo
  > ```


## Intro

Package trace extends the features of the Go execution tracer. This project is
currently experimental, you can read [issue #1](https://github.com/cstockton/go-trace/issues/1) for more information.

While keeping in mind they are meant to serve as a example rather than useful
tools, feel free to check the cmd directory for tracecat & tracegrep which use
the encoding package.

### Sub Package: Encoding

  Package encoding implements a streaming Decoder and Encoder for all versions
  of the Go trace format. For a higher level interface see the parent trace
  package.

#### Overview

  This library will Decode all previous versions of the trace codec, while only
  emitting Events in the latest version. Unlike the go tool it does not buffer
  events during decoding to make them immediately available. This limits the
  aggregation and correlation to look-behind operations and shared state, but
  enables the ability to stream events from applications in real time. Most of
  the API closely resembles events emitted from the runtime. To get a quick
  primer I suggest starting with the "Go Execution Tracer" design document
  located at: https://golang.org/s/go15trace

  In general Events have intuitive names and it's easy to correlate to your
  code, for when you can't it may help to better understand the scheduler by
  reading the design doc at https://golang.org/s/go11sched as well. It's a bit
  dated but remains conceptually accurate and serves as a good primer. After
  that https://github.com/golang/go/wiki/DesignDocuments for GC, preemption,
  syscalls and everything else.

#### Compatibility

  The Go trace format seems to be evolving continuously as new events are added
  and old events refined. This is a good thing but it does make it difficult to
  provide backwards compatibility. The maintenance burden of representing each
  event as it's native versions format would be high and error prone. Not to
  mention difficult to consume as you special cased each version.

  So instead all prior trace format versions will be properly decoded by this
  library into a single Event structure matching the latest version. If an
  Event argument is missing in the source version then we try to discover a
  sane default, in most cases a zero value.

  > Example:
  > ```Go
  > data, err := ioutil.ReadFile(`testdata/go1.8rc1/tiny_log.trace`)
  > if err != nil {
  > 	fmt.Println(`Err:`, err)
  > 	return
  > }

  > var i, created int
  > dec := encoding.NewDecoder(bytes.NewReader(data))
  > for dec.More() {
  > 	evt, err := dec.Decode()
  > 	if err != nil {
  > 		break // err will be in Err()
  > 	}
  > 	if evt.Type() == encoding.EvGoCreate {
  > 		created++
  > 	}
  > 	if i += 1; i%40 == 0 {
  > 		fmt.Println(evt) // printing a sampling of data
  > 	}
  > }
  > if err := dec.Err(); err != nil {
  > 	fmt.Println(`Err:`, err)
  > }
  > fmt.Printf("Created %v goroutines\n", created)
  > ```
  >
  > Output:
  > ```Go
  > // Output:
  > // encoding.HeapAlloc
  > // encoding.HeapAlloc
  > // encoding.HeapAlloc
  > // encoding.GoStartLocal
  > // encoding.ProcStop
  > // encoding.String("/one/ws/godev1.8/go/src/time/sys_unix.go")
  > // encoding.String("testing.(*M).before")
  > // encoding.Stack[3]:
  > // testing.(*M).before [PC 4917480]
  > // 	/one/ws/godev1.8/go/src/testing/testing.go:914
  > // testing.(*M).Run [PC 4913544]
  > // 	/one/ws/godev1.8/go/src/testing/testing.go:815
  > // main.main [PC 5212775]
  > // 	log/_test/_testmain.go:56
  > //
  > // Created 12 goroutines
  > ```


## Contributing

Feel free to create issues for bugs, please ensure code coverage remains 100%
with any pull requests.


## Bugs and Patches

  Feel free to report bugs and submit pull requests.

  * bugs:
    <https://github.com/cstockton/go-trace/issues>
  * patches:
    <https://github.com/cstockton/go-trace/pulls>
