// Package trace extends the features of the Go execution tracer. This project
// is currently experimental and only contains a encoding subpackage.
package trace

/*
TODO

trace/ [depends: encoding & event]
  Trace
     Add(event.Event) -> like visit()

encoding/ [depends: event]
  Only emit Events no concept of trace, remove circular dep by doing this

event/ [depends: none]
  Event
  Versions
  Schemas


*/
