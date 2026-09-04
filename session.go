package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

const readBufSize = 4096

// Not zero: the prompt pattern is tested against everything read so far, so a
// read boundary landing after a config line that ends in a prompt character
// matches too. Waiting for the output to go quiet tells the two apart.
const settleTimeout = 200 * time.Millisecond

// Not a plain io.Reader: the deadline is what makes an unresponsive device
// fail instead of blocking a worker for the rest of the run.
type deadlineReader interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

type Session struct {
	stdin   io.Writer
	stdout  deadlineReader
	model   *Model
	timeout time.Duration
	settle  time.Duration
	buffer  bytes.Buffer
}

func newSession(stdin io.Writer, stdout deadlineReader, model *Model, timeout time.Duration) *Session {
	return &Session{
		stdin:   stdin,
		stdout:  stdout,
		model:   model,
		timeout: timeout,
		settle:  settleTimeout,
	}
}

func (s *Session) Execute(cmd string) (string, error) {
	if err := s.sendLine(cmd); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}
	return s.readUntilPrompt()
}

func (s *Session) readUntilPrompt() (string, error) {
	return s.readUntil(s.model.promptRe)
}

func (s *Session) readUntil(pattern *regexp.Regexp) (string, error) {
	s.buffer.Reset()
	buf := make([]byte, readBufSize)

	defer func() { _ = s.stdout.SetReadDeadline(time.Time{}) }()

	for {
		matched := pattern.Match(s.buffer.Bytes())

		timeout := s.timeout
		if matched {
			timeout = s.settle
		}

		if err := s.stdout.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return s.buffer.String(), fmt.Errorf("set read deadline: %w", err)
		}

		n, err := s.stdout.Read(buf)
		if n > 0 {
			s.buffer.Write(buf[:n])
			continue
		}
		if err == nil {
			continue
		}

		if matched {
			return s.buffer.String(), nil
		}

		// Not a successful read: without the prompt the response is
		// incomplete, and reporting it as a backup would overwrite a good
		// file with half a configuration.
		switch {
		case errors.Is(err, os.ErrDeadlineExceeded):
			return s.buffer.String(), fmt.Errorf("timeout waiting for %s", pattern)
		case errors.Is(err, io.EOF):
			return s.buffer.String(), fmt.Errorf("connection closed before prompt")
		default:
			return s.buffer.String(), fmt.Errorf("read: %w", err)
		}
	}
}

func (s *Session) sendLine(cmd string) error {
	_, err := fmt.Fprintln(s.stdin, cmd)
	return err
}

func (s *Session) login() error {
	if _, err := s.readUntilPrompt(); err != nil {
		return fmt.Errorf("wait for initial prompt: %w", err)
	}

	for _, cmd := range s.model.Connection.PostLogin {
		if _, err := s.Execute(cmd); err != nil {
			return fmt.Errorf("post_login %q: %w", cmd, err)
		}
	}

	return nil
}

// The reply is not read back: the connection is torn down either way, and a
// device that closes on logout would only produce a spurious error.
func (s *Session) logout() error {
	if s.model.Connection.PreLogout == "" {
		return nil
	}
	return s.sendLine(s.model.Connection.PreLogout)
}
