package encoding

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/cstockton/go-trace/event"
)

var runLongTests = flag.Bool("long", false, ``)

func TestAllocs(t *testing.T) {
	if `` != testing.CoverMode() {
		t.Skip(`skipping testing during cover mode`)
	}
	if !*runLongTests {
		t.Skip(`skipping allocs test without -long`)
	}

	t.Run(`Reset`, func(t *testing.T) {
		exp := 10
		evt := new(event.Event)
		fn := func(count int) {
			for i := 0; i < count; i++ {
				evt.Args = append(evt.Args, uint64(i))
				evt.Data = append(evt.Data, byte(i))
			}
		}
		fn(exp)

		res := testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				evt.Reset()
				fn(exp)
				if got := len(evt.Args); got != exp {
					b.Fatalf(`exp Args len %v; got %v`, exp, got)
				}
				if got := len(evt.Data); got != exp {
					b.Fatalf(`exp Data len %v; got %v`, exp, got)
				}
			}
		})
		if got := res.MemBytes; got > 0 {
			t.Fatalf(`exp 0 bytes; got %v`, got)
		}
	})
}

func TestDecoder(t *testing.T) {
	t.Run(`Decode`, func(t *testing.T) {
		runDecoderTest(t, func(dec *Decoder) {
			evt := new(event.Event)
			err := dec.Decode(evt)
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if evt == nil {
				t.Fatal(`decoded event event should be non-nil`)
			}
		})
	})
	t.Run(`Version`, func(t *testing.T) {
		runDecoderTest(t, func(dec *Decoder) {
			ver, err := dec.Version()
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if ver != event.Latest {
				t.Fatal(`decoded version should be event.Latest`)
			}
		})
	})
	t.Run(`Reset`, func(t *testing.T) {
		dec := NewDecoder(new(bytes.Buffer))
		if dec.Reset(nil); dec.err == nil {
			t.Error(`exp non-nil err`)
		}
	})
	t.Run(`NilEvent`, func(t *testing.T) {
		dec := NewDecoder(new(bytes.Buffer))
		err := dec.Decode(nil)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
	})
	t.Run(`UnexpectedEOF`, func(t *testing.T) {
		dec := NewDecoder(new(bytes.Buffer))
		evt := new(event.Event)
		sentinel := dec.Decode(evt)
		if sentinel != io.ErrUnexpectedEOF {
			t.Fatalf(`exp io.ErrUnexpectedEOF sentinel err, got: %v`, sentinel)
		}
		if evt.Type != event.EvNone {
			t.Fatalf(`decoded event type should be EvNone; got %v`, evt)
		}

		// cause next decode to yield unexpected iof during Decode()
		buf := makeBuffer(t, event.Latest, 1)
		dec.Reset(bytes.NewReader(buf.Bytes()[:buf.Len()-2]))
		checkDecoderInit(t, dec)

		sentinel = dec.Decode(evt)
		if sentinel != io.ErrUnexpectedEOF {
			t.Fatalf(`exp io.ErrUnexpectedEOF sentinel err, got: %v`, sentinel)
		}
	})
}

func TestState(t *testing.T) {
	t.Run(`Reset`, func(t *testing.T) {
		// nil state Reader should get a new Reader
		s := newState(nil)
		s.Reset(new(bytes.Buffer))
		if s.Reader == nil {
			t.Fatal(`expected non-nil *bufio.Reader`)
		}
	})
}

func testDecodeSetup(t *testing.T, v event.Version, b []byte) *state {
	buf := new(bytes.Buffer)
	buf.Write(makeHeader(t, v))
	if b == nil {
		b = makeEvents(t, v, 1)
	}
	buf.Write(b)
	dec := NewDecoder(buf)
	dec.init()
	if dec.err != nil {
		t.Fatal(dec.err)
	}
	return dec.state
}

func checkDecoder(t *testing.T, dec *Decoder, exp error) {
	if dec == nil {
		t.Fatal(`exp non-nil decoder`)
	}
	if err := dec.Err(); err != exp {
		t.Fatalf(`exp Err() to be %v; got %v`, err, exp)
	}
	if exp != nil && dec.More() {
		t.Fatal(`More() should return false while Err() is non-nil`)
	}
	if exp == nil {
		return
	}

	// all future calls should return same err
	for i := 0; i < 3; i++ {
		if err := dec.Err(); err != exp {
			t.Errorf(`exp err %q; got %q`, exp, err)
			t.Fatal(`exp non-nil identical err for all future calls`)
		}

		evt := new(event.Event)
		err := dec.Decode(evt)
		if err != exp {
			t.Fatal(`exp err to remain unchanged`)
		}
		if evt.Type != event.EvNone {
			t.Fatalf(`decoded event type should be EvNone; got %v`, evt)
		}
		if dec.More() {
			t.Fatal(`more should return false while d.err is non nil`)
		}
	}
}

func checkDecoderInit(t *testing.T, dec *Decoder) {
	if dec == nil {
		t.Fatal(`exp non-nil decoder`)
	}
	if dec.state == nil {
		t.Fatal(`initial decode state should be non-nil`)
	}
	if dec.state.argoff != 0 {
		t.Fatal(`initial decode state argOffset should be 0`)
	}
	if off := dec.state.off; off != 0 {
		t.Fatalf(`initial writer offset should clear, but got: %v`, off)
	}
}

func runDecoderTest(t *testing.T, fn func(dec *Decoder)) {
	r := makeBuffer(t, event.Latest, 1)
	dec := NewDecoder(r)
	checkDecoderInit(t, dec)

	fn(dec)
	checkDecoder(t, dec, nil)

	// check error propagation with sentinel err
	sentinel := errors.New(`sentinel`)
	dec.halt(sentinel)
	checkDecoder(t, dec, sentinel)

	_, err := dec.Version()
	if err != sentinel {
		t.Errorf(`exp err %q; got %q`, sentinel, err)
	}

	evt := new(event.Event)
	err = dec.Decode(evt)
	if err != sentinel {
		t.Errorf(`exp err %q; got %q`, sentinel, err)
	}

	// all future calls should be same error
	if _, err2 := dec.Version(); err2 != err {
		t.Fatalf(`exp err %v; got %v`, err, err2)
	}
	if err2 := dec.Err(); err2 != err {
		t.Fatalf(`exp err %v; got %v`, err, err2)
	}

	// ensure decoder recovers after reset and reuses same bufio.Reader
	buf := dec.state.Reader
	dec.Reset(makeBuffer(t, event.Latest, 1))
	if !reflect.DeepEqual(buf, dec.state.Reader) {
		t.Fatal(`expected decoder to reuse same iobuf.Reader`)
	}
	checkDecoderInit(t, dec)

	cur := dec.state.Reader
	dec.Reset(makeBuffer(t, event.Latest, 1))
	if cur != dec.state.Reader {
		t.Fatal(`expected decoder to use given iobuf.Reader`)
	}
	checkDecoderInit(t, dec)

	dec.Reset(makeBuffer(t, event.Latest, 1))
	checkDecoderInit(t, dec)
}

func TestDecodeErrors(t *testing.T) {
	negMulti := func(t *testing.T, fn func() io.Reader) {
		evt := new(event.Event)
		err := decodeEventString(newState(fn()), evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}

		*evt = event.Event{}
		err = decodeEventInline(newState(fn()), maxMakeSize+1, evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}

		*evt = event.Event{}
		err = decodeEventArgs(newState(fn()), evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
	}
	t.Run(`EOF`, func(t *testing.T) {
		// EOF Initial position for single & multi-byte
		negMulti(t, func() io.Reader {
			return bytes.NewReader(nil)
		})

		// EOF After first read
		negMulti(t, func() io.Reader {
			return bytes.NewReader([]byte{0x1})
		})

		// EOF After first read + half of second read.
		negMulti(t, func() io.Reader {
			return bytes.NewReader([]byte{0x80, 0x2, 0x1, 0x1})
		})
	})
	t.Run(`SizeOverflow`, func(t *testing.T) {
		b := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}
		s := newState(bytes.NewReader(b))

		evt := new(event.Event)
		err := decodeEventString(s, evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if !strings.Contains(err.Error(), "allocation limit") {
			t.Errorf(`exp err for exceeding allocation limit; got %v`, err)
		}
		if len(evt.Data) != 0 {
			t.Error(`exp zero data`)
		}

		*evt = event.Event{}
		s = newState(bytes.NewReader(b))
		err = decodeEventArgs(s, evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if !strings.Contains(err.Error(), "allocation limit") {
			t.Errorf(`exp err for exceeding allocation limit; got %v`, err)
		}
		if len(evt.Args) != 0 {
			t.Error(`exp zero args`)
		}
	})
}

func TestDecodeHeader(t *testing.T) {
	t.Run(`Latest`, func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.Write(makeHeader(t, event.Latest))
		dec := NewDecoder(buf)
		err := decodeHeader(dec.state)
		if err != nil {
			t.Error(err)
		}
	})
	t.Run(`Invalid`, func(t *testing.T) {
		buf, header := new(bytes.Buffer), makeHeader(t, event.Latest)
		header[5] = '0' // set invalid version
		buf.Write(header)

		dec := NewDecoder(buf)
		err := decodeHeader(dec.state)
		if err == nil {
			t.Error(`exp non-nil error`)
		}
	})
}

func TestDecodeUleb(t *testing.T) {
	// Generated with:
	//
	// 	makeTests := func(n uint64) {
	// 		var b []byte
	// 		exp := n
	// 		for {
	// 			y := uint8(n & 0x7f)
	// 			n >>= 7
	// 			if n != 0 {
	// 				y |= 0x80
	// 			}
	// 			b = append(b, y)
	// 			if y&0x80 == 0 {
	// 				break
	// 			}
	// 		}
	// 		fmt.Printf("{%v, %#v},\n", exp, b)
	// 	}
	// 	for y := uint64(math.MaxUint64) - 1; y > 0; y /= 128 {
	// 		makeTests(y + 1)
	// 		makeTests(y)
	// 		makeTests(y - 1)
	// 	}
	type testDecodeUleb struct {
		exp  uint64
		from []byte
	}
	t.Run(`Valid`, func(t *testing.T) {
		tests := []testDecodeUleb{
			{18446744073709551615, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{18446744073709551614, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{18446744073709551613, []byte{0xfd, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{144115188075855872, []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x2}},
			{144115188075855871, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{144115188075855870, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{1125899906842624, []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x2}},
			{1125899906842623, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{1125899906842622, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{8796093022208, []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x2}},
			{8796093022207, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{8796093022206, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{68719476736, []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x2}},
			{68719476735, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{68719476734, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{536870912, []byte{0x80, 0x80, 0x80, 0x80, 0x2}},
			{536870911, []byte{0xff, 0xff, 0xff, 0xff, 0x1}},
			{536870910, []byte{0xfe, 0xff, 0xff, 0xff, 0x1}},
			{4194304, []byte{0x80, 0x80, 0x80, 0x2}},
			{4194303, []byte{0xff, 0xff, 0xff, 0x1}},
			{4194302, []byte{0xfe, 0xff, 0xff, 0x1}},
			{32768, []byte{0x80, 0x80, 0x2}},
			{32767, []byte{0xff, 0xff, 0x1}},
			{32766, []byte{0xfe, 0xff, 0x1}},
			{256, []byte{0x80, 0x2}},
			{255, []byte{0xff, 0x1}},
			{254, []byte{0xfe, 0x1}},
			{2, []byte{0x2}},
			{1, []byte{0x1}},
			{0, []byte{0x0}},
		}
		for i, test := range tests {
			t.Logf(`test #%v exp %v from %v bytes`, i, test.exp, len(test.from))
			v, err := decodeUleb(bytes.NewReader(test.from))
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if v != test.exp {
				t.Errorf(`exp %v; got %v`, test.exp, v)
			}
		}
	})
	t.Run(`Overflow`, func(t *testing.T) {
		tests := []testDecodeUleb{
			{0, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
			{0, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}},
			{0, []byte{}},
		}
		for i, test := range tests {
			t.Logf(`test #%v exp %v from %#v`, i, test.exp, test.from)
			v, err := decodeUleb(bytes.NewReader(test.from))
			if err == nil {
				t.Fatalf(`exp non-nil err; got err %v and value %v`, err, v)
			}
			if v != test.exp {
				t.Errorf(`exp %v; got %v`, test.exp, v)
			}
		}
	})
}

func runDecodeEventTest(t *testing.T, v event.Version, tests []testDecodeEvent) {
	for i, test := range tests {
		t.Logf("test #%v exp %v args in %v bytes for %v\n",
			i, len(test.exp), len(test.from), test.typ)

		evt := new(event.Event)
		s := testDecodeSetup(t, v, test.from)

		// quick hack to prevent forward-converting version1 events for this test
		if v == event.Version1 {
			s.ver = event.Latest
		}
		err := decodeEvent(s, evt)
		if err != nil {
			t.Fatalf(`exp nil err; got %v`, err)
		}
		if test.typ != evt.Type {
			t.Fatalf(`exp event type %v; got %v`, test.typ, evt.Type)
		}
		if !reflect.DeepEqual(test.exp, evt.Args) {
			t.Fatalf(`exp %v; got %v`, test.exp, evt.Args)
		}
	}
	neg := func(t *testing.T, data []byte) {
		s := testDecodeSetup(t, v, data)
		evt := new(event.Event)
		err := decodeEvent(s, evt)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
	}
	t.Run(`Negative`, func(t *testing.T) {
		for _, test := range tests {
			t.Run(`EventType`, func(t *testing.T) {
				from := make([]byte, len(test.from))
				copy(from, test.from)
				from[0] = '0'
				neg(t, from)
			})
			t.Run(`ArgsInvalidUleb`, func(t *testing.T) {
				from := make([]byte, len(test.from))
				copy(from, test.from[0:1])
				for i := 1; i < len(test.from); i++ {
					from[i]--
				}
				neg(t, from)
			})
		}
		t.Run(`Empty`, func(t *testing.T) {
			neg(t, make([]byte, 0, 0))
		})
	})
}

func TestDecodeEvents(t *testing.T) {
	t.Run(event.Version1.Go(), func(t *testing.T) {
		runDecodeEventTest(t, event.Version1, testEventsV1)
		t.Run(`Unsupported`, func(t *testing.T) {
			test := testEventsV2[len(testEventsV2)-1]
			s := testDecodeSetup(t, event.Version1, test.from)

			evt := new(event.Event)
			err := decodeEvent(s, evt)
			if err == nil {
				t.Error(`exp non-nil err`)
			}
		})
	})
	t.Run(event.Version2.Go(), func(t *testing.T) {
		runDecodeEventTest(t, event.Version2, testEventsV2)
		t.Run(`Unsupported`, func(t *testing.T) {
			test := testEventsV3[len(testEventsV3)-1]
			s := testDecodeSetup(t, event.Version2, test.from)

			evt := new(event.Event)
			err := decodeEvent(s, evt)
			if err == nil {
				t.Error(`exp non-nil err`)
			}
		})
	})
	t.Run(event.Version3.Go(), func(t *testing.T) {
		runDecodeEventTest(t, event.Version4, testEventsV4)
	})
	t.Run(event.Version4.Go(), func(t *testing.T) {
		runDecodeEventTest(t, event.Version4, testEventsV4)
	})
}

func TestDecodeEventString(t *testing.T) {
	t.Run(`Strings`, func(t *testing.T) {
		for i, test := range testEventStrings {
			t.Logf("test #%v exp %v args in %v bytes for String\n",
				i, len(test.exp), len(test.from))

			s := testDecodeSetup(t, event.Latest, test.from)
			evt := new(event.Event)
			err := decodeEvent(s, evt)
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if got := string(evt.Data); test.exp != got {
				t.Fatalf(`exp %q; got %q`, test.exp, got)
			}

			// check failing on id
			s = testDecodeSetup(t, event.Latest, test.from[0:1])
			if err := decodeEvent(s, evt); err == nil {
				t.Fatal(`exp non-nil err`)
			}
		}
	})
}

func TestDecodeEventStack(t *testing.T) {
	t.Run(`Stacks`, func(t *testing.T) {
		for i, test := range testEventStacks {
			t.Logf("test #%v exp %v args in %v bytes for Stack\n",
				i, len(test.exp), len(test.from))

			s := testDecodeSetup(t, event.Latest, test.from)
			evt := new(event.Event)
			err := decodeEvent(s, evt)
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if !reflect.DeepEqual(test.exp, evt.Args) {
				t.Fatalf(`exp %v; got %v`, test.exp, evt.Args)
			}
		}
	})
}
