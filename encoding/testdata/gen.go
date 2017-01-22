package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cstockton/go-trace/encoding"
)

const (
	flagHelpUsage     = "display usage information and exit"
	flagCountUsage    = "how many goroutines to start when generating test data"
	flagGenerateUsage = "send some trace data to test with to stdout"
	flagStripUsage    = "specify a string to strip from string data"
)

var (
	flagHelp     bool
	flagGenerate bool
	flagCount    int
	flagStrip    string
)

func init() {
	flag.BoolVar(&flagHelp, "h", false, flagHelpUsage)
	flag.BoolVar(&flagHelp, "help", false, ``)
	flag.IntVar(&flagCount, "c", 100, flagCountUsage)
	flag.IntVar(&flagCount, "count", 100, ``)
	flag.BoolVar(&flagGenerate, "g", false, flagGenerateUsage)
	flag.BoolVar(&flagGenerate, "generate", false, ``)
}

func exit(code int) {
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
					fmt.Fprintln(os.Stderr, `tracecat info: waiting for stdin...`)
				}
			}
		}()
	})
	return os.Stdin
}

func readerFromArg(arg string) io.Reader {
	if arg == `-` {
		return readerFromStdin()
	}
	f, err := os.Open(arg)
	if err != nil {
		fmt.Println(`err:`, err)
		exit(1)
	}
	return f
}

func decode(r io.Reader) {
	d := encoding.NewDecoder(r)
	if d == nil {
		fmt.Println(`tracecat decode err: expected non-nil decoder`)
		exit(1)
	}
	for d.More() {
		atomic.AddInt64(&eventCount, 1)
		evt, err := d.Next()
		if err != nil {
			return
		}
		fmt.Fprintln(os.Stdout, `gen event:`, evt)
		// if evt.Type == encoding.String {
		// 	fmt.Fprintln(os.Stdout, `tracecat event:`, evt.String(), evt.Args[0], string(evt.Data))
		// } else {
		// 	fmt.Fprintln(os.Stdout, `tracecat event:`, evt.Type.Name())
		// }
	}
	if err := d.Err(); err != nil {
		fmt.Fprintln(os.Stderr, `gen decode err:`, err)
		exit(1)
	}
}

func gen() {
	args := flag.Args()
	if len(args) < 1 {
		decode(readerFromArg(`-`))
	}
	for _, arg := range args {
		fmt.Fprintf(os.Stdout, `gen info: decoding %q...`, arg)
		pr, pw := io.Pipe()
		go decode(pr)
		r := readerFromArg(arg)
		io.Copy(pw, r)
	}
}

func main() {
	flag.Parse()

	switch {
	case flagHelp:
		exit(0)
	default:
		gen()
	}
}
