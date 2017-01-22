package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cstockton/go-trace/encoding"
)

const (
	flagHelpUsage   = "display usage information and exit"
	flagStripUsage  = "specify a string to strip from string data before writing"
	flagRegexpUsage = "regexp to match against the event name"
	flagInvertUsage = "invert matching, like grep -v"
	flagQuietUsage  = "do not write information to stderr"
)

var (
	flagHelp   bool
	flagQuiet  bool
	flagStrip  string
	flagRegexp string
	flagInvert bool
)

func init() {
	flag.BoolVar(&flagHelp, "h", false, flagHelpUsage)
	flag.BoolVar(&flagHelp, "help", false, ``)
	flag.BoolVar(&flagQuiet, "q", false, flagQuietUsage)
	flag.BoolVar(&flagQuiet, "quiet", false, ``)
	flag.StringVar(&flagRegexp, "r", ``, flagRegexpUsage)
	flag.StringVar(&flagRegexp, "regexp", ``, ``)
	flag.StringVar(&flagStrip, "s", ``, flagStripUsage)
	flag.StringVar(&flagStrip, "strip", ``, ``)
	flag.BoolVar(&flagInvert, "v", false, flagInvertUsage)
	flag.BoolVar(&flagInvert, "invert", false, ``)
}

func exit(code int) {
	fmt.Println(help)
	flag.PrintDefaults()
	os.Exit(code)
}

var (
	stdinNotice sync.Once
	eventCount  int64
)

func readerFromStdin() io.Reader {
	stdinNotice.Do(func() {
		go func() {
			select {
			case <-time.After(time.Second / 2):
				if atomic.LoadInt64(&eventCount) == 0 {
					fmt.Fprintln(os.Stderr, `tracegrep info: waiting for stdin...`)
				}
			}
		}()
	})
	return os.Stdin
}

func decode(r io.Reader) {
	d := encoding.NewDecoder(r)
	if d == nil {
		fmt.Println(`tracegrep decode err: expected non-nil decoder`)
		exit(1)
	}
	for d.More() {
		atomic.AddInt64(&eventCount, 1)
		evt, err := d.Decode()
		if err != nil {
			break
		}
		fmt.Fprintln(os.Stderr, `tracegrep event:`, evt)
	}
	if err := d.Err(); err != nil {
		fmt.Println(`tracegrep decode err:`, err)
		exit(1)
	}
}

func filter() {
	filter := func(evt *encoding.Event) bool {
		return false
	}
	if len(flagRegexp) > 0 {
		r, err := regexp.Compile(flagRegexp)
		if err != nil {
			fmt.Fprintln(os.Stderr, `tracegrep regexp err:`, err)
		}
		filter = func(evt *encoding.Event) bool {
			name := evt.Type().Name()
			if match := r.MatchString(name); match {
				if !flagQuiet {
					fmt.Fprintln(os.Stderr, `tracegrep filtered:`, name)
				}
				return true
			}
			if evt.Type() == encoding.EvString {
				if match := r.MatchString(evt.String()); match {
					if !flagQuiet {
						fmt.Fprintln(os.Stderr, `tracegrep filtered:`, evt)
					}
					return true
				}
			}
			return false
		}
	}

	d := encoding.NewDecoder(readerFromStdin())
	if d == nil {
		fmt.Println(`tracegrep err: expected non-nil decoder`)
		exit(1)
	}

	w := encoding.NewEncoder(os.Stdout)
	for d.More() {
		atomic.AddInt64(&eventCount, 1)

		// filter input stream
		evt, err := d.Decode()
		if err != nil {
			continue
		}
		if filter(evt) {
			continue
		}

		// strip input stream
		if len(flagStrip) > 0 {
			if evt.Type() == encoding.EvString {
				if !flagQuiet {
					fmt.Fprintln(os.Stderr, `event:`, evt)
				}
			}
		}

		// emit to output stream
		w.Emit(evt)
	}
	if err := d.Err(); err != nil {
		fmt.Fprintln(os.Stderr, `tracegrep decode err:`, err)
		exit(1)
	}
}

func main() {
	flag.Parse()

	switch {
	case flagHelp:
		exit(0)
	default:
		filter()
	}
}

var help = `Small utility for example purposes, for more info see:

  https://github.com/cstockton/go-trace

Example:

FutileWakeup

  # Strip a path or security sensitive stack info from String events
  cat test.trace | tracegrep -s '/my/private/homedir' > filtered.trace

  # Filter out unwanted events with -v
  cat test.trace | tracegrep -vr 'FutileWakeup|HeapAlloc' > filtered.trace

Usage:

  tracecat [flags...] [trace files...]

Flags:
`
