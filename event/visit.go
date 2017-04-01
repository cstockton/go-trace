package event

type Visitor interface {
	Visit(evt *Event) error
}

type errVisitor struct{ err error }

func (v errVisitor) Visit(evt *Event) (err error) {
	return v.err
}
