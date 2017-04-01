package tracegen

import "github.com/cstockton/go-trace/event"

type EventSource struct {
	Type   event.Type
	Data   int
	Args   []uint64
	Source []byte
}

type SourceList struct {
	Version event.Version
	Sources []EventSource
}

var Events = []SourceList{EventsV1, EventsV2, EventsV3}
