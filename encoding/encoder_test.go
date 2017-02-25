package encoding

import (
	"bytes"
	"errors"
	"io/ioutil"
	"math"
	"testing"

	"github.com/cstockton/go-trace/event"
)

func TestNewEncoder(t *testing.T) {
	enc := NewEncoder(ioutil.Discard)
	if enc == nil {
		t.Fatal(`expected non-nil encoder`)
	}
	if enc.encode != nil {
		t.Fatal(`encode func should be nil before init()`)
	}

	enc.init()
	if enc.encode == nil {
		t.Fatal(`encode func should be non-nil after init()`)
	}
	if err := enc.Emit(&event.Event{Type: event.EvBatch, Args: []uint64{0, 0}}); err != nil {
		t.Fatalf(`Emit for valid event should have nil err; got %v`, err)
	}

	enc.init()
	err := enc.Err()
	if err == nil {
		t.Fatal(`exp error for multiple init calls`)
	}

	// all future calls should be same error
	enc.init()
	if err2 := enc.Err(); err2 != err {
		t.Fatalf(`exp err %v; got %v`, err, err2)
	}
}

func TestEncoderErrors(t *testing.T) {
	enc := NewEncoder(ioutil.Discard)
	sentinel := enc.Emit(&event.Event{Type: event.EvBatch, Args: []uint64{}})
	for i := 0; i < 10; i++ {
		if err := enc.Err(); err != sentinel {
			t.Fatal(`exp non-nil identical err for all future calls`)
		}

		sentinel2 := enc.Emit(&event.Event{Type: event.EvFrequency})
		if sentinel2 != sentinel {
			t.Fatal(`exp err to remain unchanged`)
		}
	}

	enc.Reset(ioutil.Discard)
	if err := enc.Err(); err != nil {
		t.Fatalf(`error should clear after Reset, but got: %v`, err)
	}
	if off := enc.w.Off(); off != 0 {
		t.Fatalf(`writer offset should clear after Reset, but got: %v`, off)
	}
}

func TestEncodeInit(t *testing.T) {
	fn, err := encodeInit(&offsetWriter{w: ioutil.Discard}, event.Latest)
	if err != nil {
		t.Fatal(err)
	}
	if fn == nil {
		t.Fatal(`exp non-nil fn`)
	}
	t.Run(`Propagation`, func(t *testing.T) {
		rwl := &rwLimiter{w: ioutil.Discard, n: 0}
		fn, err := encodeInit(&offsetWriter{w: rwl}, event.Latest)
		if err == nil {
			t.Fatal(`exp non-nil err for writer error`)
		}
		if fn != nil {
			t.Fatal(`exp nil fn for writer error`)
		}
	})
}

func TestEncodeHeader(t *testing.T) {
	err := encodeHeader(ioutil.Discard, event.Latest)
	if err != nil {
		t.Fatal(err)
	}
	t.Run(`Propagation`, func(t *testing.T) {
		for _, v := range []event.Version{event.Version1, event.Version2, event.Version3, 0} {
			w := &rwLimiter{w: ioutil.Discard, n: 0}
			if err := encodeHeader(w, v); err == nil {
				t.Fatal(`exp non-nil err for writer error`)
			}
		}
	})
}

func testEncodeFn(t *testing.T, fn encodeFn, evt *event.Event) {
	sentinel := errors.New(`expected error`)
	wrt := func(limit int, err error) writer {
		rwl := &rwLimiter{w: ioutil.Discard, n: limit, err: err}
		return &offsetWriter{w: rwl}
	}
	chk := func(errn *int, err error) {
		if err != nil {
			*errn++
		}
	}

	var errn int
	for i := 0; i < 12; i++ {
		chk(&errn, fn(wrt(i, sentinel), evt))
		chk(&errn, fn(wrt(i, nil), evt))
	}
	chk(&errn, fn(wrt(32, nil), &event.Event{}))

	if errn == 0 {
		t.Fatal(`expected at least 1 failure`)
	}
}

func TestEncoderResilience(t *testing.T) {
	max, b := uint64(math.MaxUint64), makeNonZeroBuf(16)
	args := []uint64{max, max, max, max, max}
	run := func(fn encodeFn, evt *event.Event) {
		testEncodeFn(t, fn, evt)          // test the given fn
		testEncodeFn(t, encodeEvent, evt) // run through the top level encodeFn too
	}
	run(encodeEventArgs, &event.Event{Type: event.EvStack, Args: args})
	run(encodeEventInline, &event.Event{
		Type: event.EvBatch, Args: []uint64{max, max}})
	run(encodeEventString, &event.Event{
		Type: event.EvString, Args: []uint64{max}, Data: b})
}

func TestOffsetWriter(t *testing.T) {
	t.Run(`Allocs`, func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 0, 1024))
		w := &offsetWriter{w: buf}
		byt := byte(12)
		allocs := testing.AllocsPerRun(1000, func() {
			if err := w.WriteByte(byt); err != nil {
				t.Fatal(err)
			}
			buf.Reset()
		})
		if allocs > 0 {
			t.Fatal(`io.ByteWriter implementation should not allocate`)
		}
	})
}
