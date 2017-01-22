package encoding

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"time"
)

// These are the types of events that may be emitted. They are copied directly
// from the runtime/trace.go source file.
const (
	EvNone           Type = 0  // unused
	EvBatch          Type = 1  // start of per-P batch of events [pid, timestamp]
	EvFrequency      Type = 2  // contains tracer timer frequency [frequency (ticks per second)]
	EvStack          Type = 3  // stack [stack id, number of PCs, array of {PC, func string ID, file string ID, line}]
	EvGomaxprocs     Type = 4  // current value of GOMAXPROCS [timestamp, GOMAXPROCS, stack id]
	EvProcStart      Type = 5  // start of P [timestamp, thread id]
	EvProcStop       Type = 6  // stop of P [timestamp]
	EvGCStart        Type = 7  // GC start [timestamp, seq, stack id]
	EvGCDone         Type = 8  // GC done [timestamp]
	EvGCScanStart    Type = 9  // GC mark termination start [timestamp]
	EvGCScanDone     Type = 10 // GC mark termination done [timestamp]
	EvGCSweepStart   Type = 11 // GC sweep start [timestamp, stack id]
	EvGCSweepDone    Type = 12 // GC sweep done [timestamp]
	EvGoCreate       Type = 13 // goroutine creation [timestamp, new goroutine id, new stack id, stack id]
	EvGoStart        Type = 14 // goroutine starts running [timestamp, goroutine id, seq]
	EvGoEnd          Type = 15 // goroutine ends [timestamp]
	EvGoStop         Type = 16 // goroutine stops (like in select{}) [timestamp, stack]
	EvGoSched        Type = 17 // goroutine calls Gosched [timestamp, stack]
	EvGoPreempt      Type = 18 // goroutine is preempted [timestamp, stack]
	EvGoSleep        Type = 19 // goroutine calls Sleep [timestamp, stack]
	EvGoBlock        Type = 20 // goroutine blocks [timestamp, stack]
	EvGoUnblock      Type = 21 // goroutine is unblocked [timestamp, goroutine id, seq, stack]
	EvGoBlockSend    Type = 22 // goroutine blocks on chan send [timestamp, stack]
	EvGoBlockRecv    Type = 23 // goroutine blocks on chan recv [timestamp, stack]
	EvGoBlockSelect  Type = 24 // goroutine blocks on select [timestamp, stack]
	EvGoBlockSync    Type = 25 // goroutine blocks on Mutex/RWMutex [timestamp, stack]
	EvGoBlockCond    Type = 26 // goroutine blocks on Cond [timestamp, stack]
	EvGoBlockNet     Type = 27 // goroutine blocks on network [timestamp, stack]
	EvGoSysCall      Type = 28 // syscall enter [timestamp, stack]
	EvGoSysExit      Type = 29 // syscall exit [timestamp, goroutine id, seq, real timestamp]
	EvGoSysBlock     Type = 30 // syscall blocks [timestamp]
	EvGoWaiting      Type = 31 // denotes that goroutine is blocked when tracing starts [timestamp, goroutine id]
	EvGoInSyscall    Type = 32 // denotes that goroutine is in syscall when tracing starts [timestamp, goroutine id]
	EvHeapAlloc      Type = 33 // memstats.heap_live change [timestamp, heap_alloc]
	EvNextGC         Type = 34 // memstats.next_gc change [timestamp, next_gc]
	EvTimerGoroutine Type = 35 // denotes timer goroutine [timer goroutine id]
	EvFutileWakeup   Type = 36 // denotes that the previous wakeup of this goroutine was futile [timestamp]
	EvString         Type = 37 // string dictionary entry [ID, length, string]
	EvGoStartLocal   Type = 38 // goroutine starts running on the same P as the last event [timestamp, goroutine id]
	EvGoUnblockLocal Type = 39 // goroutine is unblocked on the same P as the last event [timestamp, goroutine id, stack]
	EvGoSysExitLocal Type = 40 // syscall exit on the same P as the last event [timestamp, goroutine id, real timestamp]
	EvGoStartLabel   Type = 41 // goroutine starts running with label [timestamp, goroutine id, seq, label string id]
	EvGoBlockGC      Type = 42 // goroutine bloI see, cks on GC assist [timestamp, stack]
	EvCount          Type = 43
)

// Type represents the type of trace event.
type Type byte

// Valid returns true if the event Type is valid, false otherwise.
func (t Type) Valid() bool {
	return EvNone < t && t < EvCount
}

// Name returns the name of this event type.
func (t Type) Name() string {
	return schemas[t%EvCount].name
}

// Since returns the version that this event was introduced.
func (t Type) Since() Version {
	return schemas[t%EvCount].since
}

// Args returns an ordered list of arguments this type of event will contain.
func (t Type) Args() []string {
	return schemas[t%EvCount].args
}

// String implements fmt.Stringer by returning a helpful string describing this
// event type.
func (t Type) String() string {
	return fmt.Sprintf(`encoding.%v`, t.Name())
}

type Event struct {
	// @TODO THere is no way to create event outside package
	typ  Type
	off  int
	time int64
	seq  int64

	args  []uint64
	data  []byte
	state *state
}

// Type returns the offset of this event relative to the start position of the
// input stream.
func (e *Event) Type() Type {
	return e.typ
}

// Time returns the time this event occurred.
func (e *Event) Time() time.Time {
	t := time.Unix(e.time, 0)
	log.Println(t)
	return t
}

// Pos returns the offset of this event relative to the begining of the input
// stream along with the length in bytes.
func (e *Event) Pos() (offset, length int) {
	return e.off, 0
}

// Off returns the offset of this event relative to the begining of the input
// stream along with the length in bytes.
func (e *Event) Off() int {
	return e.off
}

// Stack returns a non-nil, but possibly empty Stack.
func (e *Event) Stack() (Stack, error) {
	// Ensures we won't exceed bounds or npr
	if err := e.validate(); err != nil {
		return nil, err
	}

	// @TODO lookup map[key{Type,string}]struct{}
	var stkID uint64
	for idx, arg := range schemas[e.typ%EvCount].args {
		if arg == ArgStackID {
			stkID = e.args[idx]
		}
	}

	stk, err := e.state.getStack(stkID)
	if err != nil {
		return nil, err
	}
	return stk, errors.New(`no stack associated to this Event`)
}

// String implements fmt.Stringer by returning a helpful string describing this
// event type.
func (e *Event) String() string {
	switch e.typ {
	case EvString:
		return fmt.Sprintf(`encoding.%v(%q)`, schemas[e.typ%EvCount].name, string(e.data))
	case EvStack:
		stk, _ := e.Stack()
		return stk.String()
	}
	return fmt.Sprintf(`encoding.%v`, schemas[e.typ%EvCount].name)
}

// validate returns any errors associated with the structure of this Event.
func (e *Event) validate() error {
	if nil == e.state {
		return fmt.Errorf(`Event %v had nil state`, e.typ.Name())
	}
	if !e.typ.Valid() {
		return fmt.Errorf(`Event %v type 0x%x is invalid`, e.typ.Name(), byte(e.typ))
	}

	// Fetch schema for validation
	sm := schemas[e.typ]

	// Validate the arg len is at least as long as the schema
	if exp, got := len(sm.args), len(e.args); exp > got {
		return fmt.Errorf(
			`Event %v only had %d of %d arguments`, e.typ.Name(), got, exp)
	}
	return nil
}

type Stack []*Frame

func (s Stack) Empty() bool {
	return len(s) == 0
}

func (s Stack) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("encoding.Stack[%v]:\n", len(s)))
	for _, frame := range s {
		buf.WriteString(frame.String() + "\n")
	}
	return buf.String()
}

type Frame struct {
	line         int
	pc, fn, file uint64
	state        *state
}

func (f Frame) getString(id uint64) string {
	if f.state == nil {
		return `(Unknown)`
	}
	if str, err := f.state.getString(id); err == nil {
		return str
	}
	return `(Unknown)`
}

func (f Frame) PC() uint64 {
	return f.pc
}

func (f Frame) Func() string {
	return f.getString(f.fn)
}

func (f Frame) File() string {
	return f.getString(f.file)
}

func (f Frame) Line() int {
	return f.line
}

func (f Frame) String() string {
	return fmt.Sprintf("%v [PC %v]\n\t%v:%v",
		f.Func(), f.PC(), f.File(), f.Line())
}
