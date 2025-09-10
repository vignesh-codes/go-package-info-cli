package progress

import (
	"bytes"
	"io"
	"testing"
)

func TestProgressReader(t *testing.T) {
	data := []byte("Hello, World!")
	pr := &ProgressReader{
		Reader: bytes.NewReader(data),
		Total:  int64(len(data)),
	}

	buf := make([]byte, len(data))
	n, err := pr.Read(buf)

	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("got %d bytes", n)
	}
	if pr.Curr != int64(len(data)) {
		t.Errorf("got position %d", pr.Curr)
	}
}

func TestProgressReaderEOF(t *testing.T) {
	pr := &ProgressReader{
		Reader: bytes.NewReader([]byte("test")),
		Total:  4,
	}

	buf := make([]byte, 4)
	pr.Read(buf)

	buf = make([]byte, 10)
	n, err := pr.Read(buf)

	if err != io.EOF || n != 0 {
		t.Errorf("got %d, %v", n, err)
	}
}

func TestProgressRender(t *testing.T) {
	pr := &ProgressReader{Total: 0, Curr: 5}
	pr.render() // Should not panic with zero total
}

type errorReader struct{ err error }

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

func TestProgressError(t *testing.T) {
	pr := &ProgressReader{
		Reader: &errorReader{io.ErrUnexpectedEOF},
		Total:  100,
	}

	buf := make([]byte, 10)
	n, err := pr.Read(buf)

	if err != io.ErrUnexpectedEOF || n != 0 {
		t.Errorf("got %d, %v", n, err)
	}
}
