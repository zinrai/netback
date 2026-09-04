package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// Where a read boundary falls decides whether the prompt is detected too
// early, and a real connection cannot be made to place one reliably. Every
// other property of the read loop is asserted over SSH in collect_test.go.
type chunkedReader struct {
	chunks  []string
	next    int
	pending []byte
}

func (r *chunkedReader) SetReadDeadline(time.Time) error { return nil }

func (r *chunkedReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.next >= len(r.chunks) {
			return 0, os.ErrDeadlineExceeded
		}
		r.pending = []byte(r.chunks[r.next])
		r.next++
	}

	n := copy(p, r.pending)
	r.pending = r.pending[n:]

	return n, nil
}

func readLoopSession(t *testing.T, chunks ...string) *Session {
	t.Helper()

	model := &Model{Prompt: `\S+[#>]\s*$`, Commands: []string{"show running-config"}}
	if err := compileModel(model); err != nil {
		t.Fatalf("compile model: %v", err)
	}

	session := newSession(io.Discard, &chunkedReader{chunks: chunks}, model, time.Second)
	session.settle = time.Millisecond

	return session
}

func TestExecuteCapturesOutputSplitAcrossReads(t *testing.T) {
	session := readLoopSession(t,
		"show running-config\nhostname spine-01\n",
		"interface Ethernet1\n",
		"spine-01#",
	)

	out, err := session.Execute("show running-config")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "show running-config\nhostname spine-01\ninterface Ethernet1\nspine-01#"
	if out != want {
		t.Errorf("output:\n%q\nwant:\n%q", out, want)
	}
}

// A read boundary after a config line ending in a prompt character must not be
// taken for the prompt. Storing what was read so far would report success and
// overwrite a good backup with half a configuration.
func TestExecuteIgnoresPromptCharacterAtReadBoundary(t *testing.T) {
	session := readLoopSession(t,
		"show running-config\ninterface Ethernet1\n   description uplink >\n",
		"interface Ethernet2\n   description core\n",
		"spine-01#",
	)

	out, err := session.Execute("show running-config")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "interface Ethernet2") {
		t.Errorf("output was cut at the read boundary:\n%q", out)
	}
	if !strings.HasSuffix(out, "spine-01#") {
		t.Errorf("output does not end at the prompt:\n%q", out)
	}
}
