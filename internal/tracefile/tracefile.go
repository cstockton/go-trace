package tracefile

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cstockton/go-trace/event"
)

var (
	Names    = []string{`log.trace`, `net_http.trace`, `sync_atomic.trace`}
	Versions = [...]event.Version{
		event.Version1,
		event.Version2,
		event.Version3,
		event.Version4,
	}
)

// Load will load the trace files from the testdata dir.
func Load(root string) (out TraceList, err error) {
	for _, ver := range Versions {
		for _, name := range Names {
			// path: /path/to/cwd/testdata/go1.5/log.trace
			path := filepath.Join(root, `testdata`, `go`+ver.Go(), name)
			tr, err := NewTrace(ver, path)
			if err != nil {
				return nil, err
			}
			out = append(out, tr)
		}
	}
	return
}

type Trace struct {
	Version event.Version
	Size    int
	Path    string
	Name    string
	Data    []byte
}

func NewTrace(ver event.Version, path string) (*Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tr := &Trace{ver, int(info.Size()), path, filepath.Base(path), data}
	return tr, nil
}

func (tf Trace) Bytes() []byte {
	out := make([]byte, len(tf.Data))
	copy(out, tf.Data)
	return out
}

type TraceList []*Trace

func (s TraceList) String() string {
	var buf bytes.Buffer
	if len(s) == 0 {
		return `TraceList()`
	}

	buf.WriteString(`TraceList(` + s[0].Name)
	for _, tr := range s[1:] {
		buf.WriteString(`, ` + tr.Name)
	}
	return buf.String() + `)`
}

func (s TraceList) ByName(name string) (out TraceList) {
	for _, tf := range s {
		if tf.Name == name {
			out = append(out, tf)
		}
	}
	return
}

func (s TraceList) ByVersion(ver event.Version) (out TraceList) {
	for _, tf := range s {
		if tf.Version == ver {
			out = append(out, tf)
		}
	}
	return
}

func (s TraceList) ByMaxSize(n int) (out TraceList) {
	for _, tf := range s {
		if tf.Size < n {
			out = append(out, tf)
		}
	}
	return
}
