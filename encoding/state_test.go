package encoding

import (
	"bufio"
	"bytes"
	"testing"
)

func testStateSetup(t *testing.T, v Version, data []byte) (decodeFn, reader, *state) {
	r := &offsetReader{Reader: bufio.NewReader(bytes.NewReader(data))}
	fn, s, err := decodeInitVersion(v)
	if err != nil {
		t.Fatal(err)
	}
	return fn, r, s
}

func testStateSetupEvents(t *testing.T) map[Type]*Event {
	m := make(map[Type]*Event)

	for _, test := range testEventsV3 {
		_, r, s := testStateSetup(t, Latest, test.from)
		evt, err := decodeEvent(r, s)
		if err != nil {
			t.Fatal(err)
		}
		m[evt.typ] = evt
	}
	return m
}

func TestStateValidateArgCount(t *testing.T) {
	evt := func(n int) *Event {
		return &Event{args: make([]uint64, n)}
	}
	tests := []struct {
		exp      bool
		min, max int
		evt      *Event
	}{
		{true, 0, 1, evt(1)},
		{true, 0, 2, evt(1)},
		{true, 1, 1, evt(1)},
		{true, 1, 3, evt(1)},
		{true, 1, 3, evt(2)},
		{false, 2, 2, nil},
		{false, 2, 2, evt(1)},
		{false, 2, 2, evt(3)},
		{false, 2, 3, evt(4)},
	}
	s := newState()
	for _, test := range tests {
		err := s.validateArgCount(test.evt, test.min, test.max)
		if test.exp && err != nil {
			t.Fatalf(`expected nil error; got %v`, err)
		}
		if !test.exp && err == nil {
			t.Fatal(`expected non-nil error`)
		}
	}
}

func TestStateVisit(t *testing.T) {
	s, m := newState(), testStateSetupEvents(t)
	for _, evt := range m {
		if err := s.visit(evt); err != nil {
			t.Fatal(err)
		}

		evt := &Event{typ: EvNone}
		if err := s.visit(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}

		// bad arg count
		evt = &Event{typ: EvBatch}
		if err := s.visit(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
	}
	if s.freq == 0 {
		t.Fatal(`expected non-zero frequency`)
	}
}

func TestStateVisitFrequency(t *testing.T) {
	s, m := newState(), testStateSetupEvents(t)
	evt, ok := m[EvFrequency]
	if !ok {
		t.Fatal(`expected to find a EvFrequency event`)
	}
	if err := s.visitFrequency(evt); err != nil {
		t.Fatal(err)
	}
	if s.freq == 0 {
		t.Fatal(`expected non-zero frequency`)
	}
	t.Run(`Negative`, func(t *testing.T) {
		// bad type
		evt := &Event{typ: EvNone}
		if err := s.visitFrequency(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad arg count
		evt.typ = EvFrequency
		if err := s.visitFrequency(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad frequency
		evt.args = []uint64{0}
		if err := s.visitFrequency(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
	})
}

func TestStateVisitString(t *testing.T) {
	_, _, gs := testStateSetup(t, Latest, nil)
	_, _, s := testStateSetup(t, Latest, nil)
	for idx, test := range testEventStrings {
		_, r, _ := testStateSetup(t, Latest, test.from)
		evt, err := decodeEvent(r, s)
		if err != nil {
			t.Fatalf(`exp nil err; got %v`, err)
		}
		if got := len(s.strings); idx != got {
			t.Fatalf(`expected %d strings; got %v`, idx, got)
		}

		strID, str := evt.args[0], string(evt.data)
		if err = s.visitString(evt); err != nil {
			t.Fatal(err)
		}
		// check top level visit
		if err = gs.visit(evt); err != nil {
			t.Fatal(err)
		}
		// vist this string again
		if err = s.visitString(evt); err == nil {
			t.Fatal(`expected non-nil error for duplicate string`)
		}
		if got := len(s.strings); idx+1 != got {
			t.Fatalf(`expected %d strings; got %v`, idx+1, got)
		}
		got, err := s.getString(strID)
		if err != nil {
			t.Fatal(err)
		}
		if got != str {
			t.Fatalf(`expected %q; got %q`, str, got)
		}
		got, err = gs.getString(strID)
		if err != nil {
			t.Fatal(err)
		}
		if got != str {
			t.Fatalf(`expected %q; got %q`, str, got)
		}
	}
	t.Run(`Negative`, func(t *testing.T) {
		// bad type
		evt := &Event{typ: EvNone}
		if err := s.visitString(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad arg count
		evt.typ = EvString
		if err := s.visitString(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad str id
		evt.args = []uint64{0}
		if err := s.visitString(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
	})
}

func TestStateVisitStackV1(t *testing.T) {
	_, _, s := testStateSetup(t, Version1, nil)
	// bad type
	evt := &Event{typ: EvNone}
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil error`)
	}
	// bad arg count
	evt.typ = EvStack
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil err1or`)
	}
	// bad stack id
	evt.args = []uint64{0, 1, 1, 1}
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil error`)
	}
	// bad stack size
	evt.args = []uint64{1, maxStackSize + 1, 1}
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil error`)
	}
	// extra frames
	evt.args = []uint64{150, 1, 1, 1}
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil error`)
	}
	// valid event
	evt.args = []uint64{150, 1, 1}
	if err := s.visitStack(evt); err != nil {
		t.Fatal(err)
	}
	// valid event, bad frame size
	s.frameSize = 5
	if err := s.visitStack(evt); err == nil {
		t.Fatal(`expected non-nil error`)
	}
}

func TestStateVisitStackV3(t *testing.T) {
	_, _, gs := testStateSetup(t, Latest, nil)
	_, _, s := testStateSetup(t, Latest, nil)
	for idx, test := range testEventStacks {
		_, r, _ := testStateSetup(t, Latest, test.from)
		evt, err := decodeEvent(r, s)
		if err != nil {
			t.Fatalf(`exp nil err; got %v`, err)
		}
		if got := len(s.stacks); idx != got {
			t.Fatalf(`expected %d stack; got %v`, idx, got)
		}

		stkID := evt.args[0]
		if err = s.visitStack(evt); err != nil {
			t.Fatal(err)
		}
		// check top level visit
		if err = gs.visit(evt); err != nil {
			t.Fatal(err)
		}
		if err = s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error for duplicate stack`)
		}
		if got := len(s.stacks); idx+1 != got {
			t.Fatalf(`expected %d stacks; got %v`, idx+1, got)
		}
		_, err = s.getStack(stkID + 1000)
		if err == nil {
			t.Fatal(`expected non-nil error for bogus stack id`)
		}
		stk, err := s.getStack(stkID)
		if err != nil {
			t.Fatal(err)
		}
		if len(stk) == 0 {
			t.Fatal(`expected non-zero stack size`)
		}
		stk, err = gs.getStack(stkID)
		if err != nil {
			t.Fatal(err)
		}
		if len(stk) == 0 {
			t.Fatal(`expected non-zero stack size`)
		}
		for _, frame := range stk {
			if frame.PC() == 0 {
				t.Fatal(`expected non-zero PC`)
			}
			if frame.Line() == 0 {
				t.Fatal(`expected non-zero line number`)
			}
			if len(frame.File()) == 0 {
				t.Fatal(`expected non-zero frame file length`)
			}
			if len(frame.Func()) == 0 {
				t.Fatal(`expected non-zero frame func length`)
			}
		}
	}

	// clear state for neg testing
	_, _, s = testStateSetup(t, Version3, nil)
	t.Run(`Negative`, func(t *testing.T) {
		// bad type
		evt := &Event{typ: EvNone}
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad arg count
		evt.typ = EvStack
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad stack id
		evt.args = []uint64{0, 1, 1, 1, 1, 1}
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// bad stack size
		evt.args = []uint64{1, maxStackSize + 1, 1, 1, 1, 1}
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// extra frames
		evt.args = []uint64{150, 1, 1, 1, 1, 1, 1}
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
		// valid event
		evt.args = []uint64{150, 1, 1, 1, 1, 1}
		if err := s.visitStack(evt); err != nil {
			t.Fatal(err)
		}
		// valid event, bad frame size
		s.frameSize = 5
		if err := s.visitStack(evt); err == nil {
			t.Fatal(`expected non-nil error`)
		}
	})
}
