package encoding

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/cstockton/go-trace/event"
)

// Encoder writes events encoded in the Go trace format to an output stream.
//
// Events produced by the Encoder are always lexically correct, logical
// consistency with runtime produced events is the responsibility of the
// caller. It is included for testing systems that consume or parse trace
// events.
type Encoder struct {
	w      *offsetWriter
	err    error
	encode encodeFn
}

// NewEncoder returns a new encoder that emits events to w in the latest version
// of the Go trace format.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: &offsetWriter{w: w}}
}

// Err returns the first error that occurred during encoding, once an error
// occurs all future calls to Err() will return the same value.
func (e *Encoder) Err() error {
	return e.err
}

// Reset the Encoder for writing to w.
func (e *Encoder) Reset(w io.Writer) {
	e.err, e.w.off, e.w.w = nil, 0, w
}

// Emit writes a single event to the the output stream. If Emit returns a
// non-nil error then failure is permanent and all future calls will immediately
// return the same error.
func (e *Encoder) Emit(evt *event.Event) error {
	if e.encode == nil {
		e.init()
	}

	// Once an error occurs the encoder may no longer be used.
	if e.err != nil {
		return e.err
	}
	if err := e.encode(e.w, evt); err != nil {
		e.err = fmt.Errorf(`%v at 0x%x`, err, e.w.Off())
		return e.err
	}
	return nil
}

// init will initialize the Decoder so it may begin receiving events by decoding
// the trace header within the first 16 bytes of r.
func (e *Encoder) init() {
	if e.err != nil {
		return
	}
	if e.encode != nil {
		e.err = errors.New(`possible unsafe usage from multiple goroutines`)
		return
	}
	e.encode, e.err = encodeInit(e.w, event.Latest)
}

type writer interface {
	io.Writer
	io.ByteWriter
}

type offsetWriter struct {
	w   io.Writer
	off int
	buf [1]byte
}

func (r *offsetWriter) Off() int {
	return r.off
}

func (r *offsetWriter) Write(p []byte) (n int, err error) {
	n, err = r.w.Write(p)
	r.off += n
	return
}

func (r *offsetWriter) WriteByte(b byte) (err error) {
	r.buf[0] = b
	n, err := r.w.Write(r.buf[:])
	r.off += n
	return err
}

type encodeFn func(w writer, evt *event.Event) error

// encodeInit will simply send the header and return the Latest event fn.
func encodeInit(w writer, v event.Version) (encodeFn, error) {
	if err := encodeHeader(w, v); err != nil {
		return nil, err
	}
	return encodeEvent, nil
}

// encodeHeader will encode a valid trace version object into a well formed
// trace header.
func encodeHeader(w io.Writer, v event.Version) (err error) {
	var n int
	switch v {
	case event.Version1:
		n, err = w.Write([]byte("go 1.5 trace\x00\x00\x00\x00"))
	case event.Version2:
		n, err = w.Write([]byte("go 1.7 trace\x00\x00\x00\x00"))
	case event.Version3:
		n, err = w.Write([]byte("go 1.8 trace\x00\x00\x00\x00"))
	case event.Version4:
		n, err = w.Write([]byte("go 1.9 trace\x00\x00\x00\x00"))
	default:
		err = errors.New(`trace header version was invalid`)
	}
	if err == nil && n != 16 {
		err = io.ErrShortWrite
	}
	return err
}

// encodeEvent will encode the given event to w.
func encodeEvent(w writer, evt *event.Event) error {
	if !evt.Type.Valid() {
		return errors.New(`invalid trace event type`)
	}

	// From runtime/trace.go:530~
	//
	//   We have only 2 bits for number of arguments.
	//   If number is >= 3, then the event type is followed by
	//   event length in bytes.
	//
	// if narg > 3 {
	// 	narg = 3
	// }
	switch {
	case evt.Type == event.EvString:
		return encodeEventString(w, evt)
	case len(evt.Args) < 4:
		return encodeEventInline(w, evt)
	default:
		return encodeEventArgs(w, evt)
	}
}

// encodeEventInline will write a basic event with inline args to w.
func encodeEventInline(w writer, evt *event.Event) error {
	if len(evt.Args) == 0 {
		return errors.New(`expected at least 1 argument for event`)
	}

	typ, nargs := byte(evt.Type), byte(len(evt.Args)-1)
	if err := w.WriteByte(typ | nargs<<traceArgCountShift); err != nil {
		return err
	}
	for _, arg := range evt.Args {
		if err := encodeUleb(w, arg); err != nil {
			return err
		}
	}
	return nil
}

// encodeEventArgs will write a string event to w.
func encodeEventArgs(w writer, evt *event.Event) error {
	if len(evt.Args) < 4 {
		return errors.New(`expected 4 or more arguments arguments for event`)
	}

	var buf bytes.Buffer
	for _, arg := range evt.Args {
		encodeUleb(&buf, arg)
	}

	size := buf.Len()
	byt := byte(evt.Type) | byte(3)<<traceArgCountShift
	if err := w.WriteByte(byt); err != nil {
		return err
	}
	if err := encodeUleb(w, uint64(size)); err != nil {
		return err
	}

	_, err := io.Copy(w, &buf)
	return err
}

// encodeEventString will write a string event to w.
func encodeEventString(w writer, evt *event.Event) error {
	if len(evt.Args) == 0 {
		return errors.New(`expected at least 1 argument for event`)
	}

	var buf bytes.Buffer
	for _, arg := range evt.Args {
		encodeUleb(&buf, arg)
	}

	// Strings do not provide an arg count.
	if err := w.WriteByte(byte(evt.Type)); err != nil {
		return err
	}
	if err := encodeUleb(w, evt.Args[0]); err != nil {
		return err
	}

	size := len(evt.Data)
	if err := encodeUleb(w, uint64(size)); err != nil {
		return err
	}

	n, err := w.Write(evt.Data)
	if err == nil && n != len(evt.Data) {
		err = io.ErrShortWrite
	}
	return err
}

// encodeUleb will write one Unsigned Little Endian base128 encoded value to w.
func encodeUleb(w io.ByteWriter, v uint64) error {
	for ; v >= 0x80; v >>= 7 {
		if err := w.WriteByte(0x80 | byte(v)); err != nil {
			return err
		}
	}
	return w.WriteByte(byte(v))
}
