package encoding

import (
	"bufio"
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestDecoder(t *testing.T) {
	t.Run(`Decode`, func(t *testing.T) {
		runDecoderTest(t, func(dec *Decoder) {
			evt, err := dec.Decode()
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
			if ver != Latest {
				t.Fatal(`decoded version should be Latest`)
			}
		})
	})
	t.Run(`UnexpectedEOF`, func(t *testing.T) {
		dec := NewDecoder(new(bytes.Buffer))
		evt, sentinel := dec.Decode()
		if sentinel != io.ErrUnexpectedEOF {
			t.Fatal(`exp io.ErrUnexpectedEOF sentinel err`)
		}
		if evt != nil {
			t.Fatalf(`decoded event should be nil; got %v`, evt)
		}
		checkDecoder(t, dec, sentinel)

		// cause next decode to yield unexpected iof during Decode()
		buf := makeTrace(t, Latest, 1)
		dec.Reset(bytes.NewReader(buf.Bytes()[:buf.Len()-2]))
		evt, sentinel = dec.Decode()
		if sentinel != io.ErrUnexpectedEOF {
			t.Fatal(`exp io.ErrUnexpectedEOF sentinel err`)
		}
		if evt != nil {
			t.Fatalf(`decoded event should be nil; got %v`, evt)
		}
		checkDecoder(t, dec, sentinel)
	})
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

		evt, err := dec.Decode()
		if err != exp {
			t.Fatal(`exp err to remain unchanged`)
		}
		if evt != nil {
			t.Fatalf(`decoded event should be nil; got %v`, evt)
		}
		if dec.More() != false {
			t.Fatal(`more should return false while d.err is non nil`)
		}
	}
}

func checkDecoderInit(t *testing.T, dec *Decoder) {
	if dec == nil {
		t.Fatal(`exp non-nil decoder`)
	}
	if dec.decode == nil {
		t.Fatal(`decode func should be non-nil after init()`)
	}
	if dec.state == nil {
		t.Fatal(`decode state should be non-nil after init()`)
	}
	if off := dec.buf.Off(); off == 0 {
		t.Fatalf(`writer offset should clear after Reset, but got: %v`, off)
	}
}

func checkDecoderReset(t *testing.T, dec *Decoder) {
	if dec.decode != nil {
		t.Fatal(`decode func should be nil before init()`)
	}
	if dec.state != nil {
		t.Fatal(`decode state should be nil before init()`)
	}
	if off := dec.buf.Off(); off != 0 {
		t.Fatalf(`writer offset should clear after Reset, but got: %v`, off)
	}
}

func runDecoderTest(t *testing.T, fn func(dec *Decoder)) {
	dec := NewDecoder(makeTrace(t, Latest, 1))
	checkDecoderReset(t, dec)

	fn(dec)
	checkDecoder(t, dec, nil)
	checkDecoderInit(t, dec)

	dec.init()
	sentinel := dec.Err()
	if sentinel == nil {
		t.Fatal(`exp error for multiple init calls`)
	}
	checkDecoder(t, dec, sentinel)
	checkDecoderInit(t, dec)

	_, err := dec.Version()
	if err != sentinel {
		t.Errorf(`exp err %q; got %q`, sentinel, err)
	}

	_, err = dec.Decode()
	if err != sentinel {
		t.Errorf(`exp err %q; got %q`, sentinel, err)
	}

	// all future calls should be same error
	dec.init()
	if err2 := dec.Err(); err2 != err {
		t.Fatalf(`exp err %v; got %v`, err, err2)
	}

	// ensure decoder recovers after reset and reuses same bufio.Reader
	buf := dec.buf.Reader
	dec.Reset(makeTrace(t, Latest, 1))
	if !reflect.DeepEqual(buf, dec.buf.Reader) {
		t.Fatal(`expected decoder to reuse same iobuf.Reader`)
	}
	checkDecoderReset(t, dec)

	rdr := bufio.NewReader(makeTrace(t, Latest, 1))
	dec.Reset(rdr)
	if !reflect.DeepEqual(rdr, dec.buf.Reader) {
		t.Fatal(`expected decoder to use given iobuf.Reader`)
	}
	checkDecoderReset(t, dec)

	dec.Reset(makeTrace(t, Latest, 1))
	checkDecoderReset(t, dec)
}

func TestDecodeInitVersion(t *testing.T) {
	fn, s, err := decodeInitVersion(Latest)
	if err != nil {
		t.Error(err)
	}
	if s == nil {
		t.Error(`exp non-nil state`)
	}
	if fn == nil {
		t.Error(`exp non-nil fn`)
	}

	t.Run(`InvalidVersion`, func(t *testing.T) {
		fn, s, err := decodeInitVersion(0)
		if err == nil {
			t.Error(`exp non-nil error`)
		}
		if s == nil {
			t.Error(`exp non-nil state`)
		}
		if fn == nil {
			t.Error(`exp nil fn`)
		}
	})
}

func TestDecodeErrors(t *testing.T) {
	negByte := func(t *testing.T, r reader, s *state) {
		evt, err := decodeEvent(r, s)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if evt != nil {
			t.Error(`exp nil Event`)
		}

		typ, argn, err := decodeEventType(r)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if argn != 0 {
			t.Error(`exp zero args`)
		}
		if typ != EvNone {
			t.Error(`exp EvNone args`)
		}
	}
	negMulti := func(t *testing.T, s *state, fn func() reader) {
		data, err := decodeEventData(fn())
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if len(data) != 0 {
			t.Error(`exp zero data`)
		}

		args, err := decodeEventInline(fn(), maxMakeSize+1)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if len(args) != 0 {
			t.Error(`exp zero args`)
		}

		args, err = decodeEventArgs(fn(), s)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if len(args) != 0 {
			t.Error(`exp zero args`)
		}
	}
	t.Run(`EOF`, func(t *testing.T) {
		// EOF Initial position for single & multi-byte
		r := &offsetReader{Reader: bufio.NewReader(bytes.NewReader(nil))}
		negByte(t, r, newState())
		negMulti(t, newState(), func() reader {
			return r
		})

		// EOF After first read
		negMulti(t, newState(), func() reader {
			return &offsetReader{Reader: bufio.NewReader(
				bytes.NewReader([]byte{0x1}))}
		})

		// EOF After first read + half of second read.
		negMulti(t, newState(), func() reader {
			return &offsetReader{Reader: bufio.NewReader(
				bytes.NewReader([]byte{0x80, 0x2, 0x1, 0x1}))}
		})
	})
	t.Run(`SizeOverflow`, func(t *testing.T) {
		b := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x1}
		r := &offsetReader{Reader: bufio.NewReader(bytes.NewReader(b))}

		data, err := decodeEventData(r)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if !strings.Contains(err.Error(), "allocation limit") {
			t.Errorf(`exp err for exceeding allocation limit; got %v`, err)
		}
		if len(data) != 0 {
			t.Error(`exp zero data`)
		}

		r = &offsetReader{Reader: bufio.NewReader(bytes.NewReader(b))}
		args, err := decodeEventArgs(r, newState())
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if !strings.Contains(err.Error(), "allocation limit") {
			t.Errorf(`exp err for exceeding allocation limit; got %v`, err)
		}
		if len(args) != 0 {
			t.Error(`exp zero args`)
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

func runDecodeEventTest(t *testing.T, v Version, tests []testDecodeEvent) {
	for i, test := range tests {
		t.Logf("test #%v exp %v args in %v bytes for %v\n",
			i, len(test.exp), len(test.from), test.typ)

		_, r, s := testStateSetup(t, v, test.from)
		evt, err := decodeEvent(r, s)
		if err != nil {
			t.Fatalf(`exp nil err; got %v`, err)
		}
		if test.typ != evt.typ {
			t.Fatalf(`exp event type %v; got %v`, test.typ, evt.typ)
		}
		if !reflect.DeepEqual(test.exp, evt.args) {
			t.Fatalf(`exp %v; got %v`, test.exp, evt.args)
		}
	}
	neg := func(t *testing.T, data []byte) {
		_, r, s := testStateSetup(t, Latest, data)
		evt, err := decodeEvent(r, s)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if evt != nil {
			t.Fatalf(`exp nil event; got %v`, evt)
		}
	}
	t.Run(`Negative`, func(t *testing.T) {
		for _, test := range tests {

			// Sabotage event type
			t.Run(`EventType`, func(t *testing.T) {
				from := make([]byte, len(test.from))
				copy(from, test.from)
				from[0] = '0'
				neg(t, from)
			})

			// Overflow every value except event type.
			t.Run(`OverflowArgs`, func(t *testing.T) {
				from := make([]byte, len(test.from))
				copy(from, test.from[0:1])
				for i := 1; i < len(test.from); i++ {
					from[i]--
				}
				neg(t, from)
			})
		}
	})
}

func TestDecodeEventVersionErr(t *testing.T) {
	evt, err := decodeEventVersionErr(nil, nil)
	if err == nil {
		t.Error(`exp non-nil err`)
	}
	if evt != nil {
		t.Fatalf(`exp nil event; got %v`, evt)
	}
}

func TestDecodeEventVersion1(t *testing.T) {
	t.Run(Version1.Go(), func(t *testing.T) {
		runDecodeEventTest(t, Version1, testEventsV1)
	})
	t.Run(`Unsupported`, func(t *testing.T) {
		test := testEventsV2[len(testEventsV2)-1]
		_, r, s := testStateSetup(t, Version1, test.from)
		evt, err := decodeEvent(r, s)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if evt != nil {
			t.Fatalf(`exp nil event; got %v`, evt)
		}
	})
}

func TestDecodeEventVersion2(t *testing.T) {
	t.Run(Version2.Go(), func(t *testing.T) {
		runDecodeEventTest(t, Version2, testEventsV2)
	})
	t.Run(`Unsupported`, func(t *testing.T) {
		test := testEventsV3[len(testEventsV3)-1]
		_, r, s := testStateSetup(t, Version2, test.from)
		evt, err := decodeEvent(r, s)
		if err == nil {
			t.Error(`exp non-nil err`)
		}
		if evt != nil {
			t.Fatalf(`exp nil event; got %v`, evt)
		}
	})
}

func TestDecodeEventVersion3(t *testing.T) {
	t.Run(Version3.Go(), func(t *testing.T) {
		runDecodeEventTest(t, Version3, testEventsV3)
	})
}

func TestDecodeEventString(t *testing.T) {
	t.Run(`Strings`, func(t *testing.T) {
		for i, test := range testEventStrings {
			t.Logf("test #%v exp %v args in %v bytes for String\n",
				i, len(test.exp), len(test.from))

			_, r, s := testStateSetup(t, Latest, test.from)
			evt, err := decodeEvent(r, s)
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if got := string(evt.data); test.exp != got {
				t.Fatalf(`exp %q; got %q`, test.exp, got)
			}
		}
	})
}

func TestDecodeEventStack(t *testing.T) {
	t.Run(`Stacks`, func(t *testing.T) {
		for i, test := range testEventStacks {
			t.Logf("test #%v exp %v args in %v bytes for Stack\n",
				i, len(test.exp), len(test.from))

			_, r, s := testStateSetup(t, Latest, test.from)
			evt, err := decodeEvent(r, s)
			if err != nil {
				t.Fatalf(`exp nil err; got %v`, err)
			}
			if !reflect.DeepEqual(test.exp, evt.args) {
				t.Fatalf(`exp %v; got %v`, test.exp, evt.args)
			}
		}
	})
}
