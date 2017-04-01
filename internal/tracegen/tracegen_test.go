package tracegen

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	gen, err := New(`../cmd/tracegen/`)
	if err != nil {
		t.Fatal(err)
	}
	if gen == nil {
		t.Fatal(`exp non-nil Generator`)
	}

	var buf bytes.Buffer
	if err = gen.Run(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.Len(); got < 1024 {
		t.Fatalf(`exp at least 1024 byte trace; got %v`, got)
	}
}
