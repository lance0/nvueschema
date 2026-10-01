package nvueschema

import (
	"errors"
	"io"
	"testing"
)

var errTestWrite = errors.New("test write failure")

type failingWriter struct {
	calls, failAt int
	short         bool
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.short {
			return len(p) / 2, nil
		}
		return 0, errTestWrite
	}
	return len(p), nil
}

func TestGeneratorsPropagateWriteFailures(t *testing.T) {
	schema := &Config{Properties: map[string]*Config{"child": {Properties: map[string]*Config{"value": {Type: "string"}}}}}
	generators := map[string]func(io.Writer, *Config, map[string]any) error{
		"pydantic": WritePydantic, "yang": WriteYANG,
		"protobuf": func(w io.Writer, s *Config, i map[string]any) error { return WriteProtobuf(w, s, i, false) },
	}
	for name, generate := range generators {
		t.Run(name, func(t *testing.T) {
			for _, failAt := range []int{1, 10} {
				for _, short := range []bool{false, true} {
					writer := &failingWriter{failAt: failAt, short: short}
					want := errTestWrite
					if short {
						want = io.ErrShortWrite
					}
					if err := generate(writer, schema, nil); !errors.Is(err, want) {
						t.Errorf("failAt=%d short=%t error=%v, want %v", failAt, short, err, want)
					}
					if writer.calls != failAt {
						t.Errorf("continued writing after failure: calls=%d, want %d", writer.calls, failAt)
					}
				}
			}
		})
	}
}
