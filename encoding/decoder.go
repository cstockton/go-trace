package encoding

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/cstockton/go-trace/event"
)

const (

	// Guards against a bad trace file or decoder bug from causing oom
	maxMakeSize = 1e6

	// Shift of the number of arguments in the first event byte.
	//
	//   src/runtime/trace.go:85~ traceArgCountShift = 6
	traceArgCountShift = 6
)

// Decoder reads events encoded in the Go trace format from an input stream.
type Decoder struct {
	state *state
	err   error
}

// NewDecoder returns a new decoder that reads from r. If the given r is a
// bufio.Reader then the decoder will use it for buffering, otherwise creating
// a new bufio.Reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{state: newState(r)}
}

// Reset the Decoder to read from r, if r is a bufio.Reader it will use it for
// buffering, otherwise resetting the existing bufio.Reader which may have been
// obtained from the caller of NewDecoder.
func (d *Decoder) Reset(r io.Reader) {
	if r == nil {
		d.err = errors.New(`nil io.Reader given to Reset`)
		return
	}
	d.err = nil
	d.state.Reset(r)
}

// Err returns the first error that occurred during decoding, if that error was
// io.EOF then Err() returns nil and the decoding was successful.
func (d *Decoder) Err() error {
	if d.err == io.EOF {
		return nil
	}
	return d.err
}

// Version retrieves the version information contained in the encoded trace. You
// do not need to call this function directly to begin retrieving events. No I/O
// occurs unless no prior calls to Decode() have been made.
func (d *Decoder) Version() (event.Version, error) {
	if d.state.ver == 0 {
		d.init()
	}
	if d.err != nil {
		return 0, d.err
	}
	return d.state.ver, d.err
}

// More returns true when events may still be retrieved, false otherwise. The
// first time More returns false, all future calls will return false until Reset
// is called.
func (d *Decoder) More() bool {
	if d.err != nil {
		return false
	}
	if d.state.Buffered() == 0 {
		if _, err := d.state.Peek(1); err != nil {
			d.halt(err)
			return false
		}
	}
	return true
}

// Decode the next event from the input stream into the given *event.Event.
//
// The evt argument must be non-nil or permanent failure occurs. Callers must
// call evt.Reset() if reusing *event.Event to prevent prior fields persisting.
//
// Allocating a zero-value event.Event is sufficient as the decoder will create
// the Args and Data slices on demand. For performance it will use the same
// slice backings if they already have sufficient capacity. This allows zero
// allocation decoding by reusing an event, object, i.e.:
//
//    // The below allocations are generous, Args contains an average of 3-6 vals
//    // with an exception of Stack traces being depth * Frames. Data simply holds
//    // strings like file paths and func names.
//    evt := &event.Event{Args: make(512), Data: make(4096)}
//    for { dec.Decode(evt); ... }
//
// Once a error is returned all future calls will return the same error until
// Reset is called. If the error is a io.EOF value then the Decoding was a
// success if at least one event has been read, otherwise io.ErrUnexpectedEOF is
// returned.
func (d *Decoder) Decode(evt *event.Event) error {
	if evt == nil {
		// We can't do anything useful, fail permanently.
		d.err = errors.New(`nil event.Event given to Decode`)
		return d.err
	}
	if d.state.ver == 0 {
		d.init()
	}
	if d.err != nil {
		// Once an error occurs the decoder may no longer be used.
		return d.err
	}
	if err := decodeEvent(d.state, evt); err != nil {
		return d.halt(err)
	}
	return nil
}

// halt is called anytime an error occurs, setting permanent error state for
// this Decoder.
func (d *Decoder) halt(err error) error {
	d.err = err
	return d.err
}

func (d *Decoder) init() {
	if err := decodeHeader(d.state); err != nil {
		d.halt(err)
		return
	}

	// Set the argoffset for v1 only since the latest versions have no offset.
	if d.state.ver == event.Version1 {
		d.state.argoff = 1
	}
}

type state struct {
	*bufio.Reader
	ver    event.Version
	off    int
	argoff int
}

func newState(r io.Reader) *state {
	return &state{Reader: bufio.NewReader(r)}
}

func (s *state) Reset(r io.Reader) {
	buf := s.Reader
	if buf == nil {
		buf = bufio.NewReader(r)
	} else {
		buf.Reset(r)
	}
	*s = state{Reader: buf}
}

func (s *state) Read(p []byte) (n int, err error) {
	n, err = s.Reader.Read(p)
	s.off += n
	return
}

func (s *state) ReadByte() (b byte, err error) {
	b, err = s.Reader.ReadByte()
	s.off++
	return
}

var headerLut = [9]byte{'t', 'r', 'a', 'c', 'e', 0, 0, 0, 0}

// decodeHeader will read a valid trace header consisting of exactly 16 bytes
// from r, updating state or returning an error on failure.
func decodeHeader(s *state) error {
	var b [16]byte
	if _, err := io.ReadFull(s, b[:]); err != nil {
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		return err
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  +++|-----------------------
	if b[0] != 'g' || b[1] != 'o' || b[2] != ' ' {
		return errors.New(`trace header prefix was malformed`)
	}

	// Small lookahead here for more intuitive error reporting.
	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxx++-+|-----------------------
	if b[3] != '1' || b[4] != '.' || b[6] != ' ' {
		return errors.New(`trace header version was malformed`)
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxxxx+x|----------------------
	switch b[5] {
	case '5':
		s.ver = event.Version1
	case '7':
		s.ver = event.Version2
	case '8':
		s.ver = event.Version3
	case '9':
		s.ver = event.Version4
	default:
		return errors.New(`trace header version was malformed`)
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxxxxx++++++++++++++++++++++|
	if !bytes.Equal(headerLut[:], b[7:]) {
		s.ver = 0
		return errors.New(`trace header suffix was malformed`)
	}
	return nil
}

// decodeEvent is the top level entry function for decoding events. It will
// decode from the given state into evt, returning an err on failure.
func decodeEvent(s *state, evt *event.Event) error {
	// Retrieve and validate the event type.
	args, err := decodeEventType(s, evt)
	if err != nil {
		return err
	}
	if evt.Type.Since() > s.ver {
		return fmt.Errorf(`version %v does not support event %v`, s.ver, evt.Type)
	}

	// Set the event offset, accommodating the Type we just read.
	evt.Off = s.off - 1

	// Decode the event data.
	return decodeEventData(s, evt, args)
}

// decodeEventData will decode event data from valid state into evt, returning
// an err on failure. It will read the arguments using the state argOffset
// which represents the current versions minimum inline arguments minus the
// target versions. This allows version 1 which always had two argument
// (see decodeEventType) to be shared across versions.
func decodeEventData(s *state, evt *event.Event, args int) error {
	switch {
	case evt.Type == event.EvString:
		// Strings are a special case, they contain a single StringID argument and
		// the remainder is the raw utf8 encoded bytes.
		if err := decodeEventInline(s, 1, evt); err != nil {
			return err
		}
		return decodeEventString(s, evt)
	case args < 4:
		// Arguments are inline if they do not exceed this boundary.
		return decodeEventInline(s, args+s.argoff, evt)
	default:
		return decodeEventArgs(s, evt)
	}
}

// decodeEventType will determine the event type from the first 6 bits and the
// number of args from the remaining 2.
//
// runtime/trace.go
//
//   // We have only 2 bits for number of arguments.
//   // If number is >= 3, then the event type is followed by event length in bytes.
//   if narg > 3 {
// 	   narg = 3
//   }
//
// The bit order has remained constant and will not likely change, however the
// count is interpreted differently across versions. All versions increment the
// args by 1 to account for the timestamp argument. Which is calculated using
// CPU ticks, when a batch starts the arg will contain the base tickets which
// future non-batch events will derive an offset from. See src/runtime const
// traceTickDiv:
//
//   traceTickDiv = 16 + 48*(sys.Goarch386|sys.GoarchAmd64|sys.GoarchAmd64p32)
//
// If the event has an argument count that fits within the 2 bits available
// in the event type byte, it will represents the number of unsigned leb128
// arguments.
//
// If it exceeds the 3 available args, then the next uleb128 value changes
// to represent the total number of bytes. You then decode uleb128 values
// until this count is exhausted.
//
// Strings are a special case that have less than 3 args (one a string id) but
// do not encode an arg count within the type. They specify a additional payload
// length that is the utf8 encoded string.
func decodeEventType(s *state, evt *event.Event) (int, error) {
	byt, err := s.ReadByte()
	if err != nil {
		return 0, err
	}

	// see func comment for +1
	var args int
	evt.Type, args = event.Type(byt<<2>>2), int(byt>>traceArgCountShift)+1
	if !evt.Type.Valid() {
		return 0, fmt.Errorf("invalid event type 0x%x", byte(evt.Type))
	}
	return args, nil
}

// decodeEventString will decode the message payload as a byte slice instead of uint64
// arguments.
func decodeEventString(s *state, evt *event.Event) error {
	// This first arg represents the byte length of the message.
	size, err := decodeUleb(s)
	if err != nil {
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	if maxMakeSize < size {
		return fmt.Errorf(
			"size %v exceeds allocation limit(%v)", size, maxMakeSize)
	}
	if int(size) > cap(evt.Data) {
		evt.Data = make([]byte, size)
	} else {
		evt.Data = evt.Data[0:size]
	}

	if _, err = io.ReadFull(s, evt.Data); err != nil {
		return err
	}
	return nil
}

// decodeEventArgs is used when the args packed in the event byte exceed the
// available bits, instead specifying to decode uleb values until exceeding the
// given message length received from the first uleb value.
func decodeEventArgs(s *state, evt *event.Event) error {
	v, err := decodeUleb(s)
	if err != nil {
		return err
	}
	if maxMakeSize < v {
		return fmt.Errorf(
			"argument count %v exceeds allocation limit(%v)", v, maxMakeSize)
	}
	evt.Args = evt.Args[0:0]

	until := s.off + int(v)
	for s.off < until {
		if v, err = decodeUleb(s); err != nil {
			return err
		}
		evt.Args = append(evt.Args, v)
	}
	return nil
}

// decodeEventInline is used when the args packed in the event byte fit within
// the available bits allowing specifying to read exactly n uleb values.
func decodeEventInline(r io.ByteReader, n int, evt *event.Event) error {
	if maxMakeSize < n {
		return fmt.Errorf("size %v exceeds allocation limit(%v)", n, maxMakeSize)
	}
	if n > cap(evt.Args) {
		evt.Args = make([]uint64, n)
	} else {
		evt.Args = evt.Args[0:n]
	}

	for i := 0; i < n; i++ {
		v, err := decodeUleb(r)
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		evt.Args[i] = v
	}
	return nil
}

// decodeUleb will read one Unsigned Little Endian base128 encoded value from r.
func decodeUleb(r io.ByteReader) (uint64, error) {
	// Maximum number of bytes to encode uint64 in base-128.
	//
	//   src/runtime.go:85~ traceBytesPerNumber = 10
	const traceBytesPerNumber = 10

	var v, y uint64
	for i := 0; i < traceBytesPerNumber; i, y = i+1, y+7 {
		byt, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		v |= uint64(byt&0x7f) << y
		if byt&0x80 == 0 {
			return v, nil
		}
	}
	return 0, fmt.Errorf("uleb128 value overflowed")
}
