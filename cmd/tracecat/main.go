package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/trace"
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
	flag.StringVar(&flagStrip, "s", ``, flagStripUsage)
	flag.StringVar(&flagStrip, "strip", ``, ``)
}

func exit(code int) {
	fmt.Println(help)
	flag.PrintDefaults()
	os.Exit(code)
}

func work(n int) int64 {
	sum, ch := new(int64), make(chan []byte)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			data := make([]byte, 16)
			ch <- data
		}()
		go func() {
			data := <-ch
			time.Sleep(time.Millisecond * 50)
			atomic.AddInt64(sum, int64(len(data)))
			wg.Done()
		}()
	}
	wg.Wait()
	return *sum
}

func generate() {
	if err := trace.Start(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}
	work(flagCount)
	trace.Stop()
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
		evt, err := d.Decode()
		if err != nil {
			return
		}
		fmt.Fprintln(os.Stdout, `tracecat event:`, evt)
	}
	if err := d.Err(); err != nil {
		fmt.Fprintln(os.Stderr, `tracecat decode err:`, err)
		exit(1)
	}
}

func cat() {
	args := flag.Args()
	if len(args) < 1 {
		decode(readerFromArg(`-`))
	}
	for _, arg := range args {
		fmt.Fprintf(os.Stdout, `tracecat info: decoding %q...`, arg)
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
	case flagGenerate:
		generate()
	default:
		cat()
	}
}

var help = `Small utility for example purposes, for more info see:

  https://github.com/cstockton/go-trace

Example:

	# Generate a trace file to test with
	tracecat -g > test.trace

  # If no trace files given, read stdin
  cat test.trace | tracecat

  # If trace files are given, read each trace file
  tracecat test.trace test.trace test.trace

  # Or stdin & trace files with "-" in place of stdin
  tracecat - test.trace

Usage:

  tracecat [flags...] [trace files...]

Flags:
`
