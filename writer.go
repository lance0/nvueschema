package nvueschema

import "io"

// errorWriter retains the first write failure and prevents further writes.
// It lets generators use fmt throughout while still returning output errors.
type errorWriter struct {
	dst io.Writer
	err error
}

func (w *errorWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.dst.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}
