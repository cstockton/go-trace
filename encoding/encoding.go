// Package encoding implements a streaming Decoder and Encoder for all versions
// of the Go trace format. For a higher level interface see the parent trace
// package.
//
// Overview
//
// This library will Decode all previous versions of the trace codec, while only
// emitting Events in the latest version. Unlike the go tool it does not buffer
// events during decoding to make them immediately available. This limits the
// aggregation and correlation to look-behind operations and shared state, but
// enables the ability to stream events from applications in real time. Most of
// the API closely resembles events emitted from the runtime. To get a quick
// primer I suggest starting with the "Go Execution Tracer" design document
// located at: https://golang.org/s/go15trace
//
// In general Events have intuitive names and it's easy to correlate to your
// code, for when you can't it may help to better understand the scheduler by
// reading the design doc at https://golang.org/s/go11sched as well. It's a bit
// dated but remains conceptually accurate and serves as a good primer. After
// that https://github.com/golang/go/wiki/DesignDocuments for GC, preemption,
// syscalls and everything else.
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
// library into a single Event structure matching the latest version. If an
// Event argument is missing in the source version then we try to discover a
// sane default, in most cases a zero value.
package encoding

import (
	"errors"
	"fmt"
)

const (

	// Version1 was released in Go version 1.5 - 2015/08/19
	Version1 Version = 1

	// Version2 was released in Go version 1.7 - 2016/08/15
	Version2 Version = 2

	// Version3 was released in Go version 1.8 - 2017/01 TBD
	Version3 Version = 3

	// Latest always points to the newest version for convenience.
	Latest = Version3
)

// Arguments that may exist within an event, 1 or more of these are returned
// from calls to the Args method of Type.
const (
	ArgTimestamp      = `Timestamp`
	ArgRealTimestamp  = `RealTimestamp`
	ArgFrequency      = `Frequency`
	ArgSequence       = `Sequence`
	ArgSequenceGC     = `SequenceGC`
	ArgStackID        = `StackID`
	ArgStackSize      = `StackSize`
	ArgNewStackID     = `NewStackID`
	ArgStringID       = `StringID`
	ArgLabelStringID  = `LabelStringID`
	ArgThreadID       = `ThreadID`
	ArgProcessorID    = `ProcessorID`
	ArgGoroutineID    = `GoroutineID`
	ArgNewGoroutineID = `NewGoroutineID`
	ArgGomaxprocs     = `Gomaxprocs`
	ArgHeapAlloc      = `HeapAlloc`
	ArgNextGC         = `NextGC`
)

var (

	// ErrVersion occurs when a version is needed, but can not be determined.
	ErrVersion = errors.New(`trace header version was malformed`)

	// ErrEmpty occurs when a decoder reaches
	// ErrUnexpectedEOF = errors.New(`trace header version was malformed`)
)

// Version of Go declared in the header of the trace. Each version is
// represented in constant declarations with comments mentioning the associated
// Go version.
type Version byte

// Valid returns true if this version object is from a valid trace header, false
// otherwise.
func (v Version) Valid() bool {
	return Version1 <= v && v <= Latest
}

// Go returns the version of Go this version was released with.
func (v Version) Go() string {
	switch v {
	case Version1:
		return `1.5`
	case Version2:
		return `1.7`
	case Version3:
		return `1.8`
	}
	return ``
}

// Types returns this versions declared event types. The arguments declared by
// each Type will always have the latest versions signature.
func (v Version) Types() []Type {
	if v.Valid() {
		return versions[v].types
	}
	return nil
}

// String implements fmt.Stringer.
func (v Version) String() string {
	if !v.Valid() {
		return `Version(none)`
	}
	return fmt.Sprintf(`Version(#%d [Go %v])`, v, v.Go())
}

const (
	// Guards against a bad trace file or decoder bug from causing oom
	maxMakeSize  = 1e6
	maxStackSize = 1e3

	// Shift of the number of arguments in the first event byte.
	//
	//   src/runtime/trace.go:85~ traceArgCountShift = 6
	traceArgCountShift = 6
)

func init() {
	for typ, s := range schemas {
		for i := s.since; i <= Latest; i++ {
			versions[i].types = append(versions[i].types, Type(typ))
		}
	}
}

type version struct {
	gover string
	types []Type
}

var versions = [Latest + 1]version{
	0:        {gover: `None`},
	Version1: {gover: `1.5`},
	Version2: {gover: `1.7`},
	Version3: {gover: `1.8`},
}

type schema struct {
	name  string
	since Version
	args  []string
}

var schemas = [EvCount]schema{
	EvNone:           {"None", 0, []string{}},
	EvBatch:          {"Batch", Version1, []string{ArgProcessorID, ArgTimestamp}},
	EvFrequency:      {"Frequency", Version1, []string{ArgFrequency}},
	EvStack:          {"Stack", Version1, []string{ArgStackID, ArgStackSize}},
	EvGomaxprocs:     {"Gomaxprocs", Version1, []string{ArgTimestamp, ArgGomaxprocs, ArgStackID}},
	EvProcStart:      {"ProcStart", Version1, []string{ArgTimestamp, ArgThreadID}},
	EvProcStop:       {"ProcStop", Version1, []string{ArgTimestamp}},
	EvGCStart:        {"GCStart", Version1, []string{ArgTimestamp, ArgSequenceGC, ArgStackID}},
	EvGCDone:         {"GCDone", Version1, []string{ArgTimestamp}},
	EvGCScanStart:    {"GCScanStart", Version1, []string{ArgTimestamp}},
	EvGCScanDone:     {"GCScanDone", Version1, []string{ArgTimestamp}},
	EvGCSweepStart:   {"GCSweepStart", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGCSweepDone:    {"GCSweepDone", Version1, []string{ArgTimestamp}},
	EvGoCreate:       {"GoCreate", Version1, []string{ArgTimestamp, ArgNewGoroutineID, ArgNewStackID, ArgStackID}},
	EvGoStart:        {"GoStart", Version1, []string{ArgTimestamp, ArgGoroutineID, ArgSequence}},
	EvGoEnd:          {"GoEnd", Version1, []string{ArgTimestamp}},
	EvGoStop:         {"GoStop", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoSched:        {"GoSched", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoPreempt:      {"GoPreempt", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoSleep:        {"GoSleep", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlock:        {"GoBlock", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoUnblock:      {"GoUnblock", Version1, []string{ArgTimestamp, ArgGoroutineID, ArgSequence, ArgStackID}},
	EvGoBlockSend:    {"GoBlockSend", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlockRecv:    {"GoBlockRecv", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlockSelect:  {"GoBlockSelect", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlockSync:    {"GoBlockSync", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlockCond:    {"GoBlockCond", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoBlockNet:     {"GoBlockNet", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoSysCall:      {"GoSysCall", Version1, []string{ArgTimestamp, ArgStackID}},
	EvGoSysExit:      {"GoSysExit", Version1, []string{ArgTimestamp, ArgGoroutineID, ArgSequence, ArgRealTimestamp}},
	EvGoSysBlock:     {"GoSysBlock", Version1, []string{ArgTimestamp}},
	EvGoWaiting:      {"GoWaiting", Version1, []string{ArgTimestamp, ArgGoroutineID}},
	EvGoInSyscall:    {"GoInSyscall", Version1, []string{ArgTimestamp, ArgGoroutineID}},
	EvHeapAlloc:      {"HeapAlloc", Version1, []string{ArgTimestamp, ArgHeapAlloc}},
	EvNextGC:         {"NextGC", Version1, []string{ArgTimestamp, ArgNextGC}},
	EvTimerGoroutine: {"TimerGoroutine", Version1, []string{ArgGoroutineID}},
	EvFutileWakeup:   {"FutileWakeup", Version1, []string{ArgTimestamp}},
	EvString:         {"String", Version2, []string{ArgStringID}},
	EvGoStartLocal:   {"GoStartLocal", Version2, []string{ArgTimestamp, ArgGoroutineID}},
	EvGoUnblockLocal: {"GoUnblockLocal", Version2, []string{ArgTimestamp, ArgGoroutineID, ArgStackID}},
	EvGoSysExitLocal: {"GoSysExitLocal", Version2, []string{ArgTimestamp, ArgGoroutineID, ArgRealTimestamp}},
	EvGoStartLabel:   {"GoStartLabel", Version3, []string{ArgTimestamp, ArgGoroutineID, ArgSequence, ArgLabelStringID}},
	EvGoBlockGC:      {"GoBlockGC", Version3, []string{ArgTimestamp, ArgStackID}},
}
