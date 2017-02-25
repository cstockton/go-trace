package trace

import (
	"io"
	"runtime/trace"
)

// Start enables tracing for the current program. See the trace.Start function
// in the standard library for further documentation.
func Start(w io.Writer) error {
	return trace.Start(w)
}

// Stop stops the current tracing, if any. See the trace.Stop function in the
// standard library for further documentation.
func Stop() {
	// Call trace.Stop rather than runtime.StopTrace to ensure forward
	// compatibility with any changes to the trace package internals.
	trace.Stop()
}
