package encoding

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/cstockton/go-trace/event"
)

func BenchmarkDecoding(b *testing.B) {
	tfs := traceList.ByVersion(event.Latest).ByName(`log.trace`)
	if len(tfs) != 1 {
		b.Fatal(`couldn't find log.trace in traceList`)
	}
	data := tfs[0].Bytes()
	expCount := 331

	r := bytes.NewReader(data)
	buf := bufio.NewReaderSize(r, len(data))
	dec := NewDecoder(buf)
	b.ResetTimer()

	b.Run(`Decode`, func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			r.Reset(data)
			dec.Reset(r)

			var count int
			for dec.More() {
				evt := new(event.Event)
				err := dec.Decode(evt)
				if err != nil {
					b.Fatal(err)
				}
				if evt == nil {
					b.Fatal(`nil event`)
				}
				if evt.Type == event.EvNone {
					b.Fatal(`bad event type`)
				}
				count++
			}
			if count != expCount {
				b.Fatalf(`exp %v events; got %v`, expCount, count)
			}
		}
	})
	b.Run(`DecodeReuse`, func(b *testing.B) {
		evt := new(event.Event)
		evt.Args = make([]uint64, 0, 1024)
		evt.Data = make([]byte, 0, 4096)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			r.Reset(data)
			dec.Reset(r)

			var count int
			for dec.More() {
				evt.Reset()
				err := dec.Decode(evt)
				if err != nil {
					b.Logf(`count: %v evt: %v`, count, evt)
					b.Fatal(err)
				}
				if evt == nil {
					b.Fatal(`nil event`)
				}
				if evt.Type == event.EvNone {
					b.Fatal(`bad event type`)
				}
				count++
			}
			if count != expCount {
				b.Fatalf(`exp %v events; got %v`, expCount, count)
			}
		}
	})
}
