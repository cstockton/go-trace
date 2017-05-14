package event

import (
	"errors"
	"fmt"
)

// Trace maintains the shared satate across events.
type Trace struct {
	Version      Version
	Strings      map[uint64]string
	Stacks       map[uint64]Stack
	Count        int
	stackVisitFn func(evt *Event) error
}

// NewTrace will create a new trace for the given version, or return an error if
// the version is unknown.
func NewTrace(v Version) (*Trace, error) {
	tr := &Trace{
		Version: v,
		Stacks:  make(map[uint64]Stack),
		Strings: make(map[uint64]string),
	}
	if err := tr.init(); err != nil {
		return nil, err
	}
	return tr, nil
}

// Reset will reset this event for reuse.
func (tr *Trace) Reset() {
	*tr = Trace{}
	tr.Stacks = make(map[uint64]Stack)
	tr.Strings = make(map[uint64]string)
}

func (tr *Trace) init() error {
	if !tr.Version.Valid() {
		return fmt.Errorf(`Version %v has unknown frame size`, tr.Version)
	}
	if tr.Version > Version1 {
		tr.stackVisitFn = tr.visitStackSize4
	} else {
		tr.stackVisitFn = tr.visitStackSize1
	}
	return nil
}

// Stack returns the Stack trace associated with the given event, if any. It's
// possible that events which should have a stack are the zero value for one o
// two reasons, the stack event was not yet sent over the wire or the Stack was
// omitted entirely by the runtime. Stacks may be shared across multiple
// events and should not be mutated, make a copy instead. This will not retrieve
// the new stack of a EvCreate event, you may use e.Get(ArgNewStackID) tr.Stacks
// for that.
func (tr *Trace) Stack(evt *Event) (Stack, error) {
	return tr.getStack(evt.Get(ArgStackID))
}

// Visit the given event with this Trace.
func (tr *Trace) Visit(evt *Event) (err error) {
	if tr.Count == 0 {
		if err = tr.init(); err != nil {
			return err
		}
	}

	tr.Count++
	if nil == evt {
		return errors.New(`attempt to validate nil Event`)
	}
	if !evt.Type.Valid() {
		return fmt.Errorf(`event type %v was not valid`, evt.Type)
	}

	// Fetch schema for validation
	sm := schemas[evt.Type]

	// Validate the arg len is at least as long as the schema
	if exp, got := len(sm.Args), len(evt.Args); exp > got {
		return fmt.Errorf(
			`event type %v only had %d of %d arguments`, evt.Type, got, exp)
	}

	switch evt.Type {
	case EvFrequency:
		// err = tr.visitFrequency(evt)
	case EvString:
		err = tr.visitString(evt)
	case EvStack:
		err = tr.visitStack(evt)
	}
	return
}

// validateArgCount is a helper function used to validate the number of args in
// a Event is between min and max.
func (tr *Trace) validateArgCount(evt *Event, min, max int) error {
	if nil == evt {
		return errors.New(`attempt to validate nil Event`)
	}
	if got := len(evt.Args); got < min {
		return fmt.Errorf(
			`Event %v was given %d of %d expected arguments`, evt, got, min)
	}
	if got := len(evt.Args); max != -1 && got > max {
		return fmt.Errorf(
			`Event %v was given %d of %d expected arguments`, evt, got, max)
	}
	return nil
}

// visitString will add a string Event to this state.
func (tr *Trace) visitString(evt *Event) error {
	if evt.Type != EvString {
		return fmt.Errorf("event type %v may not be used as a string", evt)
	}
	if err := tr.validateArgCount(evt, 1, 1); err != nil {
		return err
	}

	// stack id and size consistent across versions
	id := evt.Args[0]
	if id == 0 {
		return errors.New(`invalid string id 0`)
	}

	// @TODO Decide how to store EvString and the mapping. Nil ref data, or
	// maybe skip allocating data to *Event all together.
	// evt.Data = nil
	str := string(evt.Data)
	return tr.addString(id, str)
}

// visitStack will add a Stack to this state from a decoded stack Event
// according to the FrameSize in the current state. The FrameSize may be 1 or 4
// and determines the stack frame offsets when constructing the stack. This is
// to accommodate PC only frames in Version1. The FN will be called each
// iteration and expected to return a valid non-nil *Frame.
func (tr *Trace) visitStack(evt *Event) error {
	if evt.Type != EvStack {
		return fmt.Errorf("event type %v may not be used as a stack", evt)
	}
	if err := tr.validateArgCount(evt, 2, -1); err != nil {
		return err
	}

	// stack id and size consistent across versions
	if evt.Args[0] == 0 {
		return errors.New(`invalid stack id 0`)
	}
	if size := evt.Args[1]; maxStackSize < size {
		return fmt.Errorf(
			"stack size %v exceeds limit(%v)", size, maxStackSize)
	}
	return tr.stackVisitFn(evt)
}

// visitFrequency will visit a frequency Event.
func (tr *Trace) visitFrequency(evt *Event) error {
	if evt.Type != EvFrequency {
		return fmt.Errorf("event type %v may not be used as a frequency", evt)
	}
	if err := tr.validateArgCount(evt, 1, 1); err != nil {
		return err
	}

	freq := float64(evt.Args[0])
	if freq <= 0 {
		return fmt.Errorf(`frequency %v should be >= to 0`, freq)
	}

	// tr.freq = 1e9 / freq
	// @TODO
	return nil
}

// visitStackSize1 builds for formats from Version1.
func (tr *Trace) visitStackSize1(evt *Event) (err error) {
	const frameSize = 1
	id, size := evt.Args[0], int(evt.Args[1])
	if got := len(evt.Args) - 2; got != size*frameSize {
		return fmt.Errorf(
			"stack size %v does not match arg count(%v)", size, got)
	}

	stack := make(Stack, size)
	for i := 0; i < size; i++ {
		stack[i] = Frame{tr: tr, pc: evt.Args[2+i*frameSize]}
	}
	return tr.addStack(id, stack)
}

// visitStackSize4 builds stack for Version2 and above.
func (tr *Trace) visitStackSize4(evt *Event) (err error) {
	const frameSize = 4
	id, size := evt.Args[0], int(evt.Args[1])
	if got := len(evt.Args) - 2; got != size*frameSize {
		return fmt.Errorf(
			"stack size %v does not match arg count(%v)", size, got)
	}

	stack := make(Stack, size)
	for i := 0; i < size; i++ {
		pos := 2 + i*frameSize
		stack[i] = Frame{
			tr:   tr,
			pc:   evt.Args[pos],
			fn:   evt.Args[pos+1],
			file: evt.Args[pos+2],
			line: int(evt.Args[pos+3]),
		}
	}
	return tr.addStack(id, stack)
}

func (tr *Trace) getStack(id uint64) (stk Stack, err error) {
	stk, ok := tr.Stacks[id]
	if !ok {
		err = fmt.Errorf(`trace stack ID %v could not be found`, id)
	}
	return
}

func (tr *Trace) getStringDefault(id uint64) string {
	if tr != nil {
		if str, ok := tr.Strings[id]; ok {
			return str
		}
	}
	return fmt.Sprintf(`ID(%v missing)`, id)
}

func (tr *Trace) getString(id uint64) (string, error) {
	if tr == nil {
		return ``, fmt.Errorf(`trace: cannot find string ID %v in nil Trace`, id)
	}
	if s, ok := tr.Strings[id]; ok {
		return s, nil
	}
	return ``, fmt.Errorf(`trace: cannot find string ID %v in Trace`, id)
}

func (tr *Trace) addStack(id uint64, stk Stack) error {
	if _, ok := tr.Stacks[id]; ok {
		return errors.New(`trace stack already exists`)
	}
	tr.Stacks[id] = stk
	return nil
}

func (tr *Trace) addString(id uint64, str string) error {
	if _, ok := tr.Strings[id]; ok {
		return errors.New(`trace string already exists`)
	}
	tr.Strings[id] = str
	return nil
}
