package encoding

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Decoder reads events encoded in the Go trace format from an input stream.
type Decoder struct {
	err    error
	buf    *offsetReader
	state  *state
	decode func(reader, *state) (*Event, error)
}

// NewDecoder returns a new decoder that reads from r. If the given r is a
// bufio.Reader then the decoder will use it for buffering, otherwise creating
// a new bufio.Reader.
func NewDecoder(r io.Reader) *Decoder {
	buf, ok := r.(*bufio.Reader)
	if !ok {
		buf = bufio.NewReader(r)
	}
	return &Decoder{buf: &offsetReader{Reader: buf}}
}

// Reset the Decoder to read from r, if r is a bufio.Reader it will use it for
// buffering, otherwise resetting the existing bufio.Reader which may have been
// obtained from the caller of NewDecoder.
func (d *Decoder) Reset(r io.Reader) {
	buf, ok := r.(*bufio.Reader)
	if ok {
		d.buf.Reader = buf
	} else {
		d.buf.Reset(r)
	}
	d.err, d.state, d.decode, d.buf.off = nil, nil, nil, 0
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
// do not need to call this function directly to begin retrieving events, it is
// done on the first call to Event if it was not called prior. Only the first
// call to Version results in I/O to the underlying reader.
func (d *Decoder) Version() (Version, error) {
	if d.err != nil {
		return 0, d.err
	}
	if d.decode == nil {
		d.init()
	}
	return d.state.ver, nil
}

// More returns true when events may still be retrieved, false otherwise. The
// first time More returns false, all future calls will return false until Reset
// is called.
func (d *Decoder) More() bool {
	if d.err != nil {
		return false
	}
	if d.buf.Buffered() == 0 {
		_, d.err = d.buf.Peek(1)
	}
	return d.err == nil
}

// Decode returns the next trace event from the input stream. If Decode returns
// a non-nil error then *Event will be nil. Any error returned indicates
// permanent failure and all future calls will return the same error until
// Reset. If the error is a io.EOF value then the Decoding was a success if at
// least one event has been read, otherwise io.ErrUnexpectedEOF is returned.
func (d *Decoder) Decode() (*Event, error) {
	if d.decode == nil {
		d.init()
	}
	if d.err != nil {
		// Once an error occurs the decoder may no longer be used.
		return nil, d.err
	}

	// decode a new event.
	evt, err := d.decode(d.buf, d.state)
	if err != nil {

		// If we have an io.EOF before we received a event return UnexpectedEOF.
		if err == io.EOF && d.state.count == 0 {
			err = io.ErrUnexpectedEOF
		}
		d.err = err
		return nil, err
	}

	return evt, nil
}

// init will initialize the Decoder so it may begin receiving events by decoding
// the trace header within the first 16 bytes of r.
func (d *Decoder) init() {
	if d.err != nil {
		return
	}
	if d.decode != nil {
		d.err = errors.New(`possible unsafe usage from multiple goroutines`)
		return
	}
	d.decode, d.state, d.err = decodeInit(d.buf)
}

type reader interface {
	io.Reader
	io.ByteReader
	Off() int
}

type offsetReader struct {
	*bufio.Reader
	off int
}

func (r *offsetReader) Off() int {
	return r.off
}

func (r *offsetReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.off += n
	return
}

func (r *offsetReader) ReadByte() (b byte, err error) {
	b, err = r.Reader.ReadByte()
	r.off++
	return
}

// decodeFn is a function that knows how to decode events for a specific version
// of the Go trace format. They all return the latest versions Event structure,
// making a best effort to deal with any cross version discrepancies.
type decodeFn func(r reader, s *state) (*Event, error)

// decodeInit will read the header and initialize the associated codec.
func decodeInit(r io.Reader) (decodeFn, *state, error) {
	ver, err := decodeHeader(r)
	if err != nil {
		return decodeEventVersionErr, nil, err
	}

	// We have a valid version, we initialize with it and will be ready to receive
	// events.
	return decodeInitVersion(ver)
}

// decodeInit will read the header and initialize the associated codec.
func decodeInitVersion(v Version) (fn decodeFn, s *state, err error) {
	s = newState()
	s.ver = v

	switch s.ver {
	case Version1:
		s.argOffset, s.frameSize = 1, 1
		fn = decodeEventVersion1
	case Version2:
		fn = decodeEventVersion2
	case Version3:
		fn = decodeEventVersion3
	default:
		fn = decodeEventVersionErr
		err = ErrVersion
	}
	return
}

var headerLut = [9]byte{'t', 'r', 'a', 'c', 'e', 0, 0, 0, 0}

// decodeHeader will read a valid trace header consisting of exactly 16 bytes from
// r, returning the version on success and an error on failure.
func decodeHeader(r io.Reader) (Version, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		}
		return 0, err
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  +++|-----------------------
	if b[0] != 'g' || b[1] != 'o' || b[2] != ' ' {
		return 0, errors.New(`trace header prefix was malformed`)
	}

	// Small lookahead here for more intuitive error reporting.
	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxx++-+|-----------------------
	if b[3] != '1' || b[4] != '.' || b[6] != ' ' {
		return 0, ErrVersion
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxxxx+x|----------------------
	var ver Version
	switch b[5] {
	case '5':
		ver = Version1
	case '7':
		ver = Version2
	case '8':
		ver = Version3
	default:
		return 0, ErrVersion
	}

	// "go 1.8 trace\x00\x00\x00\x00"
	//  xxxxxx++++++++++++++++++++++|
	if !bytes.Equal(headerLut[:], b[7:]) {
		return 0, errors.New(`trace header suffix was malformed`)
	}
	return ver, nil
}

// decodeEventVersionErr will simply return ErrVersion.
func decodeEventVersionErr(reader, *state) (*Event, error) {
	return nil, ErrVersion
}

// decodeEventVersion1 will decode a v1 trace event. The primary difference from
// future versions is that it will always have 2 leading args, sequence and the
// tick diff. See:
//
// runtime/trace.go:512:
//
//   if narg == 3 {
// 	   // Reserve the byte for length assuming that length < 128.
// 	   buf.varint(0)
// 	   lenp = &buf.arr[buf.pos-1]
//   }
//   buf.varint(seqDiff)
//   buf.varint(tickDiff)
//   for _, a := range args {
// 	   buf.varint(a)
//   }
//
func decodeEventVersion1(r reader, s *state) (*Event, error) {
	e, err := decodeEvent(r, s)
	if err != nil {
		return nil, err
	}

	// @TODO compute ts args from seq + tick diff
	var ts uint64
	// if s.lastTs != 0 {}

	a := e.args
	switch e.typ {

	// EvGoSysExit is skipped for now, because:
	//
	//   The constant declaration in Go 1.5.4 says:
	//     [timestamp, goroutine id, real timestamp]
	//
	//   But it appears the function emits a sequence as well:
	//     traceEvent(traceEvGoSysExit, -1, uint64(getg().m.curg.goid), seq, uint64(ts)/traceTickDiv)
	//
	//   Which aligns it with latest events signature:
	//     [timestamp, goroutine id, seq, real timestamp]
	//
	// case EvGoSysExit:
	//   break

	case EvStack:
		// Stack events don't need modified

	case EvBatch:
		// Had an unused arg in middle of event.
		//
		// 1.5
		//   buf.byte(traceEvBatch | 1<<traceArgCountShift)
		//   buf.varint(uint64(pid))
		//   buf.varint(seq)
		//   buf.varint(ticks)
		//
		// 1.8
		//   buf.byte(traceEvBatch | 1<<traceArgCountShift)
		//   buf.varint(uint64(pid))
		//   buf.varint(ticks)
		e.args[0], e.args[2] = a[0], a[2]

	case EvFrequency, EvTimerGoroutine:
		// Had trailing nul bytes, i.e.:
		//
		// 1.5:
		//   data = append(data, traceEvFrequency|0<<traceArgCountShift)
		//   data = traceAppend(data, uint64(freq))
		//   data = traceAppend(data, 0)
		//
		// 1.8:
		//   data = append(data, traceEvFrequency|0<<traceArgCountShift)
		//   data = traceAppend(data, uint64(freq))
		e.args = e.args[:1]

	case EvGCStart:
		// 1.5: [timestamp, stack id]
		// 1.8: [timestamp, seq, stack id]
		e.args[0], e.args[1], e.args[2] = ts, 0, a[2]

	case EvGoStart:
		// 1.5: [timestamp, goroutine id]
		// 1.8: [timestamp, goroutine id, seq]
		e.args[0], e.args[1], e.args[2] = ts, a[2], 0

	case EvGoUnblock:
		// 1.5: [timestamp, goroutine id, stack]
		//   traceEvent(traceEvGoUnblock, skip, uint64(gp.goid))
		// 1.8: [timestamp, goroutine id, seq, stack]
		e.args[0], e.args[1], e.args[2], e.args[3] = ts, a[2], 0, a[3]

	default:
		// All other events are normalized by removing the extraneous arg. I use the
		// same slice backing to avoid alloc.
		e.args = e.args[1:]
		e.args[0] = ts
	}

	if err := s.visit(e); err != nil {
		return nil, err
	}
	return e, nil
}

// decodeEventVersion2 will decode a v2 trace event, in v2 the arguments no
// longer always included a seq, meaning you do not have to increment for an
// additional argument. See:
//
// go1.7/runtime/trace.go:505:
//
//   if narg == 3 {
// 	   // Reserve the byte for length assuming that length < 128.
// 	   buf.varint(0)
// 	   lenp = &buf.arr[buf.pos-1]
//   }
//   buf.varint(tickDiff)
//   for _, a := range args {
// 	   buf.varint(a)
//   }
func decodeEventVersion2(r reader, s *state) (evt *Event, err error) {
	if evt, err = decodeEvent(r, s); err != nil {
		return nil, err
	}

	if err := s.visit(evt); err != nil {
		return nil, err
	}
	return evt, nil
}

// decodeEventVersion3 will decode a v3 trace event. There is no structural
// changes in v3, just new events.
func decodeEventVersion3(r reader, s *state) (evt *Event, err error) {
	if evt, err = decodeEvent(r, s); err != nil {
		return nil, err
	}

	if err := s.visit(evt); err != nil {
		return nil, err
	}
	return evt, nil
}

// These are the general codec unaware funcs.
//
// decodeEvent ->
//   decodeEventType ->
//   decodeEventArgs ->
//     (Type == EvString) ->
//       decodeArgsN(1) ->
//         decodeUleb
//       decodeData
//         io.ReadFull
//     (args < 4) ->
//       decodeArgsN(arg count) ->
//         decodeUleb
//       decodeArgs
//         decodeUleb ...
//

// decodeEvent will decode a valid event from a given reader and state, or
// return an err on failure. It will read the arguments using the state
// argOffset, which represents the current versions minimum inline arguments
// minus the target versions. This allows version 1 which always had two
// argument (see decodeEventType) to be shared accross versions.
func decodeEvent(r reader, s *state) (*Event, error) {
	// Get the number of args count and type from the first byte.
	typ, args, err := decodeEventType(r)
	if err != nil {
		return nil, err
	}

	// Quick lint check for this event.
	if typ.Since() > s.ver {
		return nil, fmt.Errorf(`version %v does not support event %v`, s.ver, typ)
	}

	// Create the event and decode the arguments. The -1 here accomodates for the
	// single byte read in decodingEventType.
	evt := &Event{typ: typ, off: r.Off() - 1, state: s}
	switch {
	case evt.typ == EvString:
		// Strings are a special case, they contain a string arg but also a payload
		// for the variable argument length. We grab the string id then fall through
		// to decode the string value.
		if evt.args, err = decodeEventInline(r, 1); err == nil {
			evt.data, err = decodeEventData(r)
		}
	case args < 4:
		// Arguments are inline if they do not exceed this boundary.
		evt.args, err = decodeEventInline(r, args+s.argOffset)
	default:
		evt.args, err = decodeEventArgs(r, s)
	}
	if err != nil {
		return nil, err
	}
	return evt, nil
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
func decodeEventType(r io.ByteReader) (Type, int, error) {
	byt, err := r.ReadByte()
	if err != nil {
		return EvNone, 0, err
	}

	// see func comment for +1
	typ, args := Type(byt<<2>>2), int(byt>>traceArgCountShift)+1
	if !typ.Valid() {
		return EvNone, 0, fmt.Errorf("invalid event type 0x%x", byte(typ))
	}
	return typ, args, nil
}

// decodeEventData will decode the message payload as a byte slice instead of uint64
// arguments.
func decodeEventData(r reader) ([]byte, error) {
	// This first arg represents the byte length of the message.
	size, err := decodeUleb(r)
	if err != nil {
		return nil, err
	}
	if maxMakeSize < size {
		return nil, fmt.Errorf(
			"size %v exceeds allocation limit(%v)", size, maxMakeSize)
	}

	data := make([]byte, size)
	if _, err = io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// decodeEventArgs is used when the args packed in the event byte exceed the
// available bits, instead specifying to decode uleb values until exceeding the
// given message length received from the first uleb value.
func decodeEventArgs(r reader, s *state) (args []uint64, err error) {
	var v uint64
	if v, err = decodeUleb(r); err != nil {
		return
	}
	if maxMakeSize < v {
		return nil, fmt.Errorf(
			"argument count %v exceeds allocation limit(%v)", v, maxMakeSize)
	}

	until := r.Off() + int(v)
	for r.Off() < until {
		if v, err = decodeUleb(r); err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return
}

// decodeEventInline is used when the args packed in the event byte fit within
// the available bits allowing specifying to read exactly n uleb values.
func decodeEventInline(r io.ByteReader, n int) ([]uint64, error) {
	if maxMakeSize < n {
		return nil, fmt.Errorf(
			"size %v exceeds allocation limit(%v)", n, maxMakeSize)
	}
	args := make([]uint64, n)

	var err error
	for i := 0; i < n; i++ {
		if args[i], err = decodeUleb(r); err != nil {
			return nil, err
		}
	}
	return args, nil
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
