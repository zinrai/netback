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
	log.Printf("%s: connecting...", c.device.Name)

	sshConfig := &ssh.ClientConfig{
		User: c.device.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.device.password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = c.device.password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.device.Timeout,
	}

	addr := net.JoinHostPort(c.device.IP, strconv.Itoa(c.device.Port))

	conn, err := net.DialTimeout("tcp", addr, c.device.Timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	log.Printf("%s: ssh connected", c.device.Name)

	c.session, err = c.client.NewSession()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("new session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Not the exec channel a shell command would use: network devices
	// generally offer an interactive shell and nothing else.
	if err := c.session.RequestPty("xterm", 80, 200, modes); err != nil {
		c.close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdin, err := c.session.StdinPipe()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := c.session.StdoutPipe()
	if err != nil {
		c.close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := c.session.Shell(); err != nil {
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
