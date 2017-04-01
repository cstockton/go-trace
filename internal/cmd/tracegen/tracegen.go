package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cstockton/go-trace/encoding"
	"github.com/cstockton/go-trace/event"
)

const (
	flagHelpUsage   = "display usage information and exit"
	flagWorkUsage   = "send some trace data to test with to stdout"
	flagNumberUsage = "the number of iterations to generate data, -1 is max int32"
	flagSizeUsage   = "the max size of trace in KB, buffering usually causes a minimal of 100-200kb"
	flagCodeUsage   = "send some trace data to test with to stdout"
)

var (
	flagHelp   bool
	flagCode   bool
	flagWork   bool
	flagNumber int
	flagSize   int
)

var (
	stdinNotice sync.Once
	eventCount  int64
)

func init() {
	flag.BoolVar(&flagHelp, "h", false, flagHelpUsage)
	flag.BoolVar(&flagHelp, "help", false, ``)
	flag.IntVar(&flagNumber, "n", 10, flagNumberUsage)
	flag.IntVar(&flagNumber, "number", 10, ``)
	flag.IntVar(&flagSize, "s", 100, flagSizeUsage)
	flag.IntVar(&flagSize, "size", 100, ``)
	flag.BoolVar(&flagWork, "w", false, flagWorkUsage)
	flag.BoolVar(&flagWork, "work", false, ``)
	flag.BoolVar(&flagCode, "c", false, flagCodeUsage)
	flag.BoolVar(&flagCode, "code", false, ``)
}

func exit(code int) {
	fmt.Println(help)
	flag.PrintDefaults()
	os.Exit(code)
}

func worker(ctx context.Context, n int, ch chan int) {
	defer close(ch)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := rand.Int()
			select {
			case <-ctx.Done():
				return
			case ch <- n:
			}
		}()
		wg.Wait()
	}
}

func work(ctx context.Context, n int) {
	sum, ch := 0, make(chan int)
	go worker(ctx, n, ch)
	for n := range ch {
		sum += n
	}
}

type traceWriter struct {
	W io.Writer
	N int
	C context.CancelFunc
}

func (w *traceWriter) Write(p []byte) (n int, err error) {
	n, err = w.W.Write(p)
	w.N -= n
	if w.N <= 0 && w.C != nil {
		w.C()
		w.C = nil
	}
	return
}

func workgen() {
	if flagSize <= 0 {
		flagSize = 256
	}
	if flagNumber < 0 {
		flagNumber = math.MaxInt32
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := traceWriter{W: os.Stdout, N: flagSize, C: cancel}
	if err := trace.Start(&w); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}

	work(ctx, flagNumber)
	trace.Stop()
}

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

func genHeader(w io.Writer) {
	fmt.Fprintf(w, "package tracegen\n")
	fmt.Fprintf(w, "import \"github.com/cstockton/go-trace/event\"\n")
	fmt.Fprintf(w, "\ntype EventSource struct {\n")
	fmt.Fprintf(w, "\tType event.Type\n")
	fmt.Fprintf(w, "\tData int\n")
	fmt.Fprintf(w, "\tArgs []uint64\n")
	fmt.Fprintf(w, "\tSource []byte\n}\n")
	fmt.Fprintf(w, "\ntype SourceList struct {\n")
	fmt.Fprintf(w, "\tVersion event.Version\n")
	fmt.Fprintf(w, "\tSources []EventSource\n}\n")
}

func genStartSlice(w io.Writer, name string, v event.Version) {
	tpl := "var %v = SourceList{event.Version%v, []EventSource{\n"
	fmt.Fprintf(w, tpl, name, int(v))
}

func genCloseSlice(w io.Writer) {
	fmt.Fprintln(w, "}}")
}

func genEvent(w io.Writer, evt *event.Event, b []byte) {
	dataOff := -1
	if len(evt.Data) > 0 {
		dataOff = bytes.LastIndex(b, evt.Data)
	}
	fmt.Fprintf(w, "\t{event.Ev%v, %v,\n", evt.Type.Name(), dataOff)
	fmt.Fprintf(w, "\t\t%#v,\n", evt.Args)
	fmt.Fprintf(w, "\t\t%#v},\n", b)
}

func decode(r io.Reader) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, `read err:`, err)
		exit(1)
	}

	r = bytes.NewReader(b)
	d := encoding.NewDecoder(r)
	if d == nil {
		fmt.Println(`decode err: expected non-nil decoder`)
		exit(1)
	}

	v, err := d.Version()
	if err != nil {
		fmt.Fprintln(os.Stderr, `decode err:`, err)
		exit(1)
	}

	var (
		w    = os.Stdout
		cur  event.Event
		last event.Event
		seen = make(map[event.Type]int)
	)

	genHeader(w)
	genStartSlice(w, `Events`, v)
	for d.More() {
		atomic.AddInt64(&eventCount, 1)
		cur.Reset()
		if err := d.Decode(&cur); err != nil {
			break
		}
		if last.Off > 0 && seen[last.Type] < flagNumber {
			seen[last.Type]++
			genEvent(w, &last, b[last.Off:cur.Off])
		}
		last, cur = cur, last
	}
	if seen[last.Type] < flagNumber {
		seen[last.Type]++
		genEvent(w, &last, b[last.Off:])
	}
	genCloseSlice(w)

	if err := d.Err(); err != nil {
		fmt.Fprintln(os.Stderr, `decode err:`, err)
		exit(1)
	}
}

func codegen() {
	args := flag.Args()
	if len(args) < 1 {
		decode(readerFromArg(`-`))
	}
	for _, arg := range args {
		decode(readerFromArg(arg))
	}
}

func main() {
	flag.Parse()

	switch {
	case flagWork:
		workgen()
	case flagCode:
		codegen()
	case flagHelp:
		fallthrough
	default:
		exit(0)
	}
}

var help = `Small utility for example purposes, for more info see:

  https://github.com/cstockton/go-trace

Example:

  # Generate a trace file around 2kb large with defaults
  tracegen -g > test.trace

  # Generate a trace file at most 400kb big
  tracegen -s 400 > test.trace

	# Generate a slice of test structs containing 10 events of each type
	tracegen -number 10 -code ../../tracefile/testdata/go1.8/net_http.trace

  # If no trace files given, read stdin
  cat test.trace | tracegen

  # If trace files are given, read each trace file
  tracegen test.trace test.trace test.trace

  # Or stdin & trace files with "-" in place of stdin
  tracegen - test.trace

Usage:

  tracegen [flags...] [trace files...]

Flags:
`
