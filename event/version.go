package event

import "fmt"

const (

	// Version1 was released in Go version 1.5 - 2015/08/19
	Version1 Version = 1

	// Version2 was released in Go version 1.7 - 2016/08/15
	Version2 Version = 2

	// Version3 was released in Go version 1.8 - 2017/02/16
	Version3 Version = 3

	// Version4 is in tip, currently marked in the header as 1.9.
	Version4 Version = 4

	// Latest always points to the newest released version for convenience.
	Latest = Version4
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
	ArgKind           = `Kind`
)

// Version of Go declared in the header of the trace. Each version is
// represented in constant declarations with comments mentioning the associated
// Go version.
type Version byte

// Valid returns true if this version object is from a valid trace header, false
// otherwise.
func (v Version) Valid() bool {
	return Version1 <= v && v <= Version4
}

// Go returns the version of Go this version was released with.
func (v Version) Go() string {
	if !v.Valid() {
		return `None`
	}
	return versions[v].gover
}

// Types returns this versions declared event types. The arguments declared by
// each Type will always have the latest versions signature. The returned value
// must not be mutated and may be nil if the Version is invalid.
func (v Version) Types() []Type {
	if !v.Valid() {
		return nil
	}
	return versions[v].types
}

// // Schemas returns the schema for each event in this version. The returned value
// // must not be mutated.
// func (v Version) Schemas() []*Schema {
// 	return versions[v%Latest].schemas
// }

// String implements fmt.Stringer.
func (v Version) String() string {
	if !v.Valid() {
		return `Version(none)`
	}
	return fmt.Sprintf(`Version(#%d [Go %v])`, v, v.Go())
}

// // GoString implements fmt.GoStringer for this event type.
// func (v Version) GoString() string {
// 	return fmt.Sprintf(`event.Version%d`, int(v))
// }

func init() {
	for typ, s := range schemas {
		for i := s.Since; i <= Version4; i++ {
			versions[i].schemas = append(versions[i].schemas, s)
			versions[i].types = append(versions[i].types, Type(typ))
		}
	}
}

// version is the private version info that gets stored in a lut
type version struct {
	gover     string
	types     []Type
	schemas   []schema
	argOffset int
	frameSize int
}

const versionsCount = Version(len(versions)) // Version T for cmp

var versions = [...]version{
	0:        {gover: `None`},
	Version1: {gover: `1.5`, argOffset: 1, frameSize: 1},
	Version2: {gover: `1.7`, frameSize: 4},
	Version3: {gover: `1.8`, frameSize: 4},
	Version4: {gover: `1.9`, frameSize: 4},
}

type schema struct {
	// Type  Type
	Name  string
	Since Version
	Args  []string
}

const schemasCount = len(schemas)

var schemas = [...]schema{
	{"None", 0, []string{}},
	{"Batch", Version1, []string{ArgProcessorID, ArgTimestamp}},
	{"Frequency", Version1, []string{ArgFrequency}},
	{"Stack", Version1, []string{ArgStackID, ArgStackSize}},
	{"Gomaxprocs", Version1, []string{
		ArgTimestamp, ArgGomaxprocs, ArgStackID}},
	{"ProcStart", Version1, []string{ArgTimestamp, ArgThreadID}},
	{"ProcStop", Version1, []string{ArgTimestamp}},
	{"GCStart", Version1, []string{
		ArgTimestamp, ArgSequenceGC, ArgStackID}},
	{"GCDone", Version1, []string{ArgTimestamp}},
	{"GCSTWStart", Version1, []string{ArgTimestamp, ArgKind}},
	{"GCSTWDone", Version1, []string{ArgTimestamp}},
	{"GCSweepStart", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GCSweepDone", Version1, []string{ArgTimestamp}},
	{"GoCreate", Version1, []string{
		ArgTimestamp, ArgNewGoroutineID, ArgNewStackID, ArgStackID}},
	{"GoStart", Version1, []string{
		ArgTimestamp, ArgGoroutineID, ArgSequence}},
	{"GoEnd", Version1, []string{ArgTimestamp}},
	{"GoStop", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoSched", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoPreempt", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoSleep", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlock", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoUnblock", Version1, []string{
		ArgTimestamp, ArgGoroutineID, ArgSequence, ArgStackID}},
	{"GoBlockSend", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlockRecv", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlockSelect", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlockSync", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlockCond", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoBlockNet", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoSysCall", Version1, []string{ArgTimestamp, ArgStackID}},
	{"GoSysExit", Version1, []string{
		ArgTimestamp, ArgGoroutineID, ArgSequence, ArgRealTimestamp}},
	{"GoSysBlock", Version1, []string{ArgTimestamp}},
	{"GoWaiting", Version1, []string{ArgTimestamp, ArgGoroutineID}},
	{"GoInSyscall", Version1, []string{ArgTimestamp, ArgGoroutineID}},
	{"HeapAlloc", Version1, []string{ArgTimestamp, ArgHeapAlloc}},
	{"NextGC", Version1, []string{ArgTimestamp, ArgNextGC}},
	{"TimerGoroutine", Version1, []string{ArgGoroutineID}},
	{"FutileWakeup", Version1, []string{ArgTimestamp}},
	{"String", Version2, []string{ArgStringID}},
	{"GoStartLocal", Version2, []string{ArgTimestamp, ArgGoroutineID}},
	{"GoUnblockLocal", Version2, []string{
		ArgTimestamp, ArgGoroutineID, ArgStackID}},
	{"GoSysExitLocal", Version2, []string{
		ArgTimestamp, ArgGoroutineID, ArgRealTimestamp}},
	{"GoStartLabel", Version3, []string{
		ArgTimestamp, ArgGoroutineID, ArgSequence, ArgLabelStringID}},
	{"GoBlockGC", Version3, []string{ArgTimestamp, ArgStackID}},
	{"EvGCMarkAssistStart", Version4, []string{ArgTimestamp, ArgStackID}},
	{"EvGCMarkAssistDone", Version4, []string{ArgTimestamp}},
}
