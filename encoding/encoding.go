// Package encoding implements a streaming Decoder and Encoder for all versions
// of the Go trace format. For a higher level interface see the parent trace
// package.
//
// Overview
//
// This library will Decode all previous versions of the trace codec, while only
// emitting Events in the latest version. Unlike the go tool it does not buffer
// events during decoding to make them immediately available without allocation
// with large performance gains. It is fast enough to easily allow decoding in
// the same process that is performing the tracing, enabling you to defer writes
// to network/disk until interesting occurrences happen.
//
// Most of the API closely resembles events emitted from the runtime. To get a
// quick primer I suggest starting with the "Go Execution Tracer" design
// document located at: https://golang.org/s/go15trace
//
// In general Events have intuitive names, when they are not it may help to read
// the scheduler design doc at https://golang.org/s/go11sched. It's a bit dated
// but remains conceptually accurate and serves as a good primer. After that
// https://github.com/golang/go/wiki/DesignDocuments for GC, preemption,
// syscalls and everything else. Recently I also came across runtime/HACKING.md
// in Go master which provides some good material as well.
//
// Compatibility
//
// The Go trace format seems to be evolving continuously as new events are added
// and old events refined. This is a good thing but it does make it difficult to
// provide backwards compatibility. The maintenance burden of representing each
// event as it's native versions format would be high and error prone. Not to
// mention difficult to consume as you special cased each version.
//
// So instead all prior trace format versions will be properly decoded by this
// library into a single Event structure matching the latest version. The args
// decoded by this package will always match their version. For example, EvBatch
// from event.Version1 (Go 1.5) has an additional sequence argument that will
// be left untouched.
package encoding
