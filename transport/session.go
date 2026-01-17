package transport

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/zinrai/netback/config"
)

// Session represents an interactive session with a device
type Session struct {
	stdin   io.Writer
	stdout  io.Reader
	model   *config.Model
	timeout time.Duration
	buffer  bytes.Buffer
}

// NewSession creates a new session wrapper
func NewSession(stdin io.Writer, stdout io.Reader, model *config.Model, timeout time.Duration) *Session {
	return &Session{
		stdin:   stdin,
		stdout:  stdout,
		model:   model,
		timeout: timeout,
	}
}

// ReadUntilPrompt reads output until the prompt is detected
func (s *Session) ReadUntilPrompt() (string, error) {
	promptRe, err := s.model.PromptRegex()
	if err != nil {
		return "", err
	}
	return s.readUntil(promptRe)
}

// ReadUntilPattern reads output until the given pattern is detected
func (s *Session) ReadUntilPattern(pattern *regexp.Regexp) (string, error) {
	return s.readUntil(pattern)
}

func (s *Session) readUntil(pattern *regexp.Regexp) (string, error) {
	s.buffer.Reset()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(s.timeout)

	for {
		if time.Now().After(deadline) {
			return s.buffer.String(), fmt.Errorf("timeout waiting for pattern")
		}

		n, err := s.stdout.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return s.buffer.String(), fmt.Errorf("read error: %w", err)
		}

		if n > 0 {
			s.buffer.Write(buf[:n])

			// Process expect rules (pager handling, etc.)
			content := s.buffer.String()
			content, handled := s.processExpectRules(content)
			if handled {
				s.buffer.Reset()
				s.buffer.WriteString(content)
			}

			// Check for prompt
			if pattern.MatchString(s.buffer.String()) {
				break
			}
		}
	}

	return s.buffer.String(), nil
}

// processExpectRules handles expect patterns (like pager responses)
func (s *Session) processExpectRules(content string) (string, bool) {
	handled := false

	for _, rule := range s.model.Expect {
		re, err := rule.Regex()
		if err != nil {
			continue
		}

		if re.MatchString(content) {
			if rule.Send != "" {
				// Send response (e.g., space for pager)
				s.Send(rule.Send)
			}
			if rule.Replace != "" || rule.Send != "" {
				// Replace the pattern in output
				content = re.ReplaceAllString(content, rule.Replace)
				handled = true
			}
		}
	}

	return content, handled
}

// Send sends a command without waiting for response
func (s *Session) Send(cmd string) error {
	_, err := fmt.Fprint(s.stdin, cmd)
	return err
}

// SendLine sends a command followed by newline
func (s *Session) SendLine(cmd string) error {
	_, err := fmt.Fprintln(s.stdin, cmd)
	return err
}

// Execute sends a command and waits for the prompt
func (s *Session) Execute(cmd string) (string, error) {
	if err := s.SendLine(cmd); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}
	return s.ReadUntilPrompt()
}

// ExecutePostLogin runs the post-login commands
func (s *Session) ExecutePostLogin() error {
	for _, cmd := range s.model.Connection.PostLogin {
		if _, err := s.Execute(cmd); err != nil {
			return fmt.Errorf("execute %q: %w", cmd, err)
		}
	}
	return nil
}

// ExecutePreLogout runs the pre-logout command
func (s *Session) ExecutePreLogout() error {
	if s.model.Connection.PreLogout != "" {
		return s.SendLine(s.model.Connection.PreLogout)
	}
	return nil
}
