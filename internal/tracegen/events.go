package tracegen

import "github.com/cstockton/go-trace/event"

// EventSource is internal and should not procuce a lint warning.
type EventSource struct {
	Type   event.Type
	Data   int
	Args   []uint64
	Source []byte
}

// SourceList is internal and should not procuce a lint warning.
type SourceList struct {
	Version event.Version
	Sources []EventSource
}

// Events is internal and should not procuce a lint warning.
var Events = []SourceList{EventsV1, EventsV2, EventsV3}
