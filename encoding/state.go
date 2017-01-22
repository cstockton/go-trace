package encoding

import (
	"errors"
	"fmt"
)

type state struct {
	ver       Version
	count     int
	frameSize int
	state     int
	argOffset int

	// @TODO look behind state tracking
	lastSeq, lastP, lastG uint64
	freq, lastTs          float64

	strings map[uint64]string
	stacks  map[uint64]Stack
}

func newState() *state {
	return &state{
		ver:       Latest,
		frameSize: 4,
		stacks:    make(map[uint64]Stack),
		strings:   make(map[uint64]string),
	}
}

// visit the given event with this state. It is expected for event to have been
// normalized to version Latest.
func (s *state) visit(evt *Event) (err error) {
	if !evt.typ.Valid() {
		return fmt.Errorf(`Event %v was not valid`, evt)
	}

	// Fetch schema for validation
	sm := schemas[evt.typ]

	// Validate the arg len is at least as long as the schema
	if exp, got := len(sm.args), len(evt.args); exp > got {
		return fmt.Errorf(`Event %v only had %d of %d arguments`, evt, got, exp)
	}

	switch evt.typ {
	case EvFrequency:
		err = s.visitFrequency(evt)
	case EvString:
		err = s.visitString(evt)
	case EvStack:
		err = s.visitStack(evt)
	}

	// count the event
	if err == nil {
		s.count++
	}
	return
}

// visitFrequency will visit a frequency Event.
func (s *state) visitFrequency(evt *Event) error {
	if evt.typ != EvFrequency {
		return fmt.Errorf("event type %v may not be used as a frequency", evt)
	}
	if err := s.validateArgCount(evt, 1, 1); err != nil {
		return err
	}

	freq := float64(evt.args[0])
	if freq <= 0 {
		return fmt.Errorf(`frequency %v should be >= to 0`, freq)
	}

	s.freq = 1e9 / freq
	// @TODO
	return nil
}

// visitString will add a string Event to this state.
func (s *state) visitString(evt *Event) error {
	if evt.typ != EvString {
		return fmt.Errorf("event type %v may not be used as a string", evt)
	}
	if err := s.validateArgCount(evt, 1, 1); err != nil {
		return err
	}

	// stack id and size consistent across versions
	id := evt.args[0]
	if id == 0 {
		return errors.New(`invalid string id 0`)
	}

	// @TODO Decide how to store EvString and the mapping. Nil ref data, or
	// maybe skip allocating data to *Event all together.
	// evt.data = nil
	str := string(evt.data)
	return s.addString(id, str)
}

// visitStack will add a Stack to this state from a decoded stack Event
// according to the frameSize in the current state. The frameSize may be 1 or 4
// and determines the stack frame offsets when constructing the stack. This is
// to accomodate PC only frames in Version1. The FN will be called each
// iteration and expected to return a valid non-nil *Frame.
func (s *state) visitStack(evt *Event) error {
	if evt.typ != EvStack {
		return fmt.Errorf("event type %v may not be used as a stack", evt)
	}
	if err := s.validateArgCount(evt, 2, -1); err != nil {
		return err
	}

	// Select a func to build frames based on *state frameSize
	var frameFn func(evt *Event, s *state, pos int) *Frame
	switch s.frameSize {
	case 1:
		frameFn = decodeStackFuncSize1
	case 4:
		frameFn = decodeStackFuncSize4
	default:
		return fmt.Errorf("event type %v may not be used as a stack", evt)
	}

	// stack id and size consistent across versions
	id, size := evt.args[0], int(evt.args[1])
	if id == 0 {
		return errors.New(`invalid stack id 0`)
	}
	if maxStackSize < size {
		return fmt.Errorf(
			"stack size %v exceeds limit(%v)", size, maxStackSize)
	}

	// accomodate version frame size differences
	frames := size * s.frameSize
	if got := len(evt.args) - 2; got != frames {
		return fmt.Errorf(
			"stack size %v does not match arg count(%v)", size, got)
	}

	stk := make(Stack, size)
	for i := 0; i < size; i++ {
		// give frameFn the argument position relative to frameSize
		stk[i] = frameFn(evt, s, 2+i*s.frameSize)
	}
	return s.addStack(id, stk)
}

// decodeStackFuncSize1 decodes stack formats from Version1.
func decodeStackFuncSize1(evt *Event, s *state, pos int) *Frame {
	return &Frame{state: s, pc: evt.args[pos]}
}

// decodeStackFuncSize4 decodes stack formats from Version2 and above.
func decodeStackFuncSize4(evt *Event, s *state, pos int) *Frame {
	return &Frame{
		state: s,
		pc:    evt.args[pos],
		fn:    evt.args[pos+1],
		file:  evt.args[pos+2],
		line:  int(evt.args[pos+3]),
	}
}

// validateArgCount is a helper function used to validate the number of args in
// a normalized Event is between min and max.
func (s *state) validateArgCount(evt *Event, min, max int) error {
	if nil == evt {
		return errors.New(`attempt to validate nil Event`)
	}
	if got := len(evt.args); got < min {
		return fmt.Errorf(
			`Event %v was given %d of %d expected arguments`, evt, got, min)
	}
	if got := len(evt.args); max != -1 && got > max {
		return fmt.Errorf(
			`Event %v was given %d of %d expected arguments`, evt, got, max)
	}
	return nil
}

func (s *state) getStack(id uint64) (stk Stack, err error) {
	stk, ok := s.stacks[id]
	if !ok {
		err = fmt.Errorf(`trace stack ID %v could not be found`, id)
	}
	return
}

func (s *state) getString(id uint64) (str string, err error) {
	str, ok := s.strings[id]
	if !ok {
		err = fmt.Errorf(`trace string ID %v could not be found`, id)
	}
	return
}

func (s *state) addStack(id uint64, stk Stack) error {
	if _, ok := s.stacks[id]; ok {
		return errors.New(`trace stack already exists`)
	}
	s.stacks[id] = stk
	return nil
}

func (s *state) addString(id uint64, str string) error {
	if _, ok := s.strings[id]; ok {
		return errors.New(`trace string already exists`)
	}
	s.strings[id] = str
	return nil
}
