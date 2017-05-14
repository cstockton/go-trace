package event

// Visitor is the interface that wraps the basic Visit method.
//
// Implementations of Visit indicate they may visit one or more events within
// a trace.
type Visitor interface {
	Visit(evt *Event) error
}

type errVisitor struct{ err error }

func (v errVisitor) Visit(evt *Event) (err error) {
	return v.err
}
