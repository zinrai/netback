package transport

import (
	"fmt"
	"net"
	"time"

	"github.com/zinrai/netback/config"
	"golang.org/x/crypto/ssh"
)

// SSHClient manages an SSH connection to a device
type SSHClient struct {
	device  *config.Device
	model   *config.Model
	client  *ssh.Client
	session *ssh.Session
}

// NewSSHClient creates a new SSH client for the device
func NewSSHClient(device *config.Device, model *config.Model) *SSHClient {
	return &SSHClient{
		device: device,
		model:  model,
	}
}

// Connect establishes the SSH connection
func (c *SSHClient) Connect() (*Session, error) {
	sshConfig := &ssh.ClientConfig{
		User: c.device.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.device.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.device.EffectiveTimeout(),
	}

	addr := fmt.Sprintf("%s:%d", c.device.IP, c.device.EffectivePort())

	conn, err := net.DialTimeout("tcp", addr, c.device.EffectiveTimeout())
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)

	c.session, err = c.client.NewSession()
	if err != nil {
		c.client.Close()
		return nil, fmt.Errorf("new session: %w", err)
	}

	// Request pseudo-terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := c.session.RequestPty("xterm", 80, 200, modes); err != nil {
		c.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdin, err := c.session.StdinPipe()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := c.session.StdoutPipe()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := c.session.Shell(); err != nil {
		c.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	session := NewSession(stdin, stdout, c.model, c.device.EffectiveTimeout())

	// Wait for initial prompt
	if _, err := session.ReadUntilPrompt(); err != nil {
		c.Close()
		return nil, fmt.Errorf("wait for initial prompt: %w", err)
	}

	// Execute post-login commands
	if err := session.ExecutePostLogin(); err != nil {
		c.Close()
		return nil, fmt.Errorf("post-login: %w", err)
	}

	return session, nil
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	var errs []error

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// ConnectAndExecute connects, executes all commands, and returns outputs per command
func ConnectAndExecute(device *config.Device, model *config.Model) ([]string, []string, error) {
	client := NewSSHClient(device, model)

	session, err := client.Connect()
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	// Execute comment commands (each output stored separately)
	commentsOutputs := make([]string, 0, len(model.Comments))
	for _, cmd := range model.Comments {
		result, err := session.Execute(cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("execute comment %q: %w", cmd, err)
		}
		commentsOutputs = append(commentsOutputs, result)
	}

	// Execute config commands (each output stored separately)
	commandsOutputs := make([]string, 0, len(model.Commands))
	for _, cmd := range model.Commands {
		result, err := session.Execute(cmd)
		if err != nil {
			return commentsOutputs, nil, fmt.Errorf("execute %q: %w", cmd, err)
		}
		commandsOutputs = append(commandsOutputs, result)
	}

	// Send pre-logout command (best effort)
	_ = session.ExecutePreLogout()

	// Small delay to allow graceful disconnect
	time.Sleep(100 * time.Millisecond)

	return commentsOutputs, commandsOutputs, nil
}
