package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"
)

type sshClient struct {
	device  *Device
	model   *Model
	client  *ssh.Client
	session *ssh.Session
	stdout  *timedReader
}

func (c *sshClient) connect() (*Session, error) {
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c.openShell()
}

func (c *sshClient) dial() error {
	log.Printf("%s: connecting...", c.device.Name)

	addr := net.JoinHostPort(c.device.IP, strconv.Itoa(c.device.Port))

	conn, err := net.DialTimeout("tcp", addr, c.device.Timeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, c.clientConfig())
	if err != nil {
		conn.Close()
		return fmt.Errorf("ssh handshake: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	log.Printf("%s: ssh connected", c.device.Name)

	return nil
}

func (c *sshClient) clientConfig() *ssh.ClientConfig {
	answerWithPassword := func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = c.device.password
		}
		return answers, nil
	}

	return &ssh.ClientConfig{
		User: c.device.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.device.password),
			ssh.KeyboardInteractive(answerWithPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.device.Timeout,
	}
}

func (c *sshClient) openShell() (*Session, error) {
	session, err := c.client.NewSession()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("new session: %w", err)
	}
	c.session = session

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Not the exec channel a shell command would use: network devices
	// generally offer an interactive shell and nothing else.
	if err := session.RequestPty("xterm", 80, 200, modes); err != nil {
		c.close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		c.close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	c.stdout = newTimedReader(stdout)

	return newSession(stdin, c.stdout, c.model, c.device.Timeout), nil
}

func (c *sshClient) close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		c.client.Close()
	}
	if c.stdout != nil {
		c.stdout.close()
	}
}
