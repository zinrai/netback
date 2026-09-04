package main

import (
	"io"
	"os"
	"time"
)

const readChunkSize = 4096

// Not a deadline on the connection underneath: the ssh package fills its
// output pipe from a background goroutine, so that deadline does not bound a
// read here. It makes the multiplexer fail instead, which tears down the whole
// connection and surfaces as an end of file on the next command.
type timedReader struct {
	results  chan readResult
	done     chan struct{}
	pending  []byte
	err      error
	deadline time.Time
}

type readResult struct {
	data []byte
	err  error
}

func newTimedReader(r io.Reader) *timedReader {
	t := &timedReader{
		results: make(chan readResult),
		done:    make(chan struct{}),
	}

	go func() {
		for {
			buf := make([]byte, readChunkSize)

			n, err := r.Read(buf)
			if n > 0 {
				select {
				case t.results <- readResult{data: buf[:n]}:
				case <-t.done:
					return
				}
			}
			if err != nil {
				select {
				case t.results <- readResult{err: err}:
				case <-t.done:
				}
				return
			}
		}
	}()

	return t
}

func (t *timedReader) SetReadDeadline(deadline time.Time) error {
	t.deadline = deadline
	return nil
}

func (t *timedReader) Read(p []byte) (int, error) {
	if len(t.pending) > 0 {
		n := copy(p, t.pending)
		t.pending = t.pending[n:]
		return n, nil
	}

	if t.err != nil {
		return 0, t.err
	}

	var timeout <-chan time.Time
	if !t.deadline.IsZero() {
		timer := time.NewTimer(time.Until(t.deadline))
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case result := <-t.results:
		if result.err != nil {
			t.err = result.err
			return 0, result.err
		}
		t.pending = result.data
		n := copy(p, t.pending)
		t.pending = t.pending[n:]
		return n, nil

	case <-timeout:
		return 0, os.ErrDeadlineExceeded
	}
}

// Abandoning the reader is not enough: its goroutine stays blocked handing
// over output nobody is going to read.
func (t *timedReader) close() {
	close(t.done)
}
