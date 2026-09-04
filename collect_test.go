package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// What is asserted below is netback's own behaviour: that every command runs,
// that a device which stops answering fails, that a dropped connection is not
// a backup. Nothing here asserts anything about SSH itself.
//
// The connection is real because those three properties depend on how the ssh
// package hands output over, and an in-memory reader answers for behaviour the
// library does not have. The server runs in this process over loopback, so
// there is no external system involved.
type fakeDevice struct {
	prompt    string
	responses map[string]string
	silentOn  string // command the device receives and never answers
	closeOn   string // command after which the device drops the connection
}

func startFakeDevice(t *testing.T, device fakeDevice) int {
	t.Helper()

	config := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil },
	}
	config.AddHostKey(hostKey(t))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	stop := make(chan struct{})

	t.Cleanup(func() {
		close(stop)
		listener.Close()
	})

	go acceptConnections(listener, config, device, stop)

	return listener.Addr().(*net.TCPAddr).Port
}

func hostKey(t *testing.T) ssh.Signer {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}

	return signer
}

func acceptConnections(listener net.Listener, config *ssh.ServerConfig, device fakeDevice, stop <-chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go serveFakeDevice(conn, config, device, stop)
	}
}

func serveFakeDevice(conn net.Conn, config *ssh.ServerConfig, device fakeDevice, stop <-chan struct{}) {
	defer conn.Close()

	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}

		go replyToRequests(requests)
		serveShell(channel, device, stop)

		return
	}
}

func replyToRequests(requests <-chan *ssh.Request) {
	for req := range requests {
		if req.WantReply {
			req.Reply(true, nil)
		}
	}
}

func serveShell(channel ssh.Channel, device fakeDevice, stop <-chan struct{}) {
	channel.Write([]byte(device.prompt))

	scanner := bufio.NewScanner(channel)
	for scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())

		if device.silentOn != "" && command == device.silentOn {
			<-stop
			return
		}

		if device.closeOn != "" && command == device.closeOn {
			channel.Close()
			return
		}

		channel.Write([]byte(command + "\n" + device.responses[command] + device.prompt))
	}
}

func testDevice(t *testing.T, port int, timeout time.Duration) *Device {
	t.Helper()

	return &Device{
		Name:     "eos-01",
		IP:       "127.0.0.1",
		Port:     port,
		Model:    "eos",
		Group:    "dc-tokyo",
		Username: "admin",
		password: "admin",
		Timeout:  timeout,
	}
}

func testModel(t *testing.T) *Model {
	t.Helper()

	model := &Model{
		Prompt:  `\S+[#>]\s*$`,
		Comment: "! ",
		Connection: ConnectionConfig{
			PostLogin: []string{"enable", "terminal length 0"},
			PreLogout: "exit",
		},
		Comments: []string{"show inventory"},
		Commands: []string{"show running-config"},
	}

	if err := compileModel(model); err != nil {
		t.Fatalf("compile model: %v", err)
	}

	return model
}

// Every command after the first depends on the connection surviving the end of
// the previous one, which is the part an in-memory reader cannot show.
func TestCollectOutputRunsEveryCommand(t *testing.T) {
	port := startFakeDevice(t, fakeDevice{
		prompt: "spine-01#",
		responses: map[string]string{
			"enable":              "",
			"terminal length 0":   "",
			"show inventory":      "\nArista DCS-7050TX-64\n",
			"show running-config": "\nhostname spine-01\ninterface Ethernet1\n",
		},
	})

	comments, commands, err := collectOutput(testDevice(t, port, 5*time.Second), testModel(t))
	if err != nil {
		t.Fatalf("connectAndExecute: %v", err)
	}

	if len(comments) != 1 || !strings.Contains(comments[0], "Arista DCS-7050TX-64") {
		t.Errorf("comments = %q", comments)
	}

	if len(commands) != 1 {
		t.Fatalf("commands = %q, want one entry", commands)
	}
	for _, want := range []string{"show running-config", "hostname spine-01", "interface Ethernet1"} {
		if !strings.Contains(commands[0], want) {
			t.Errorf("command output missing %q:\n%q", want, commands[0])
		}
	}
	if !strings.HasSuffix(commands[0], "spine-01#") {
		t.Errorf("command output does not end at the prompt:\n%q", commands[0])
	}
}

// A device that accepts the command and then says nothing must fail on the
// timeout. Without one the worker slot is held for the rest of the run.
func TestCollectOutputTimesOutOnSilentDevice(t *testing.T) {
	port := startFakeDevice(t, fakeDevice{
		prompt: "spine-01#",
		responses: map[string]string{
			"enable":            "",
			"terminal length 0": "",
			"show inventory":    "\nArista DCS-7050TX-64\n",
		},
		silentOn: "show running-config",
	})

	done := make(chan error, 1)
	go func() {
		_, _, err := collectOutput(testDevice(t, port, time.Second), testModel(t))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("connectAndExecute succeeded against a silent device")
		}
		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("error = %v, want a timeout", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("connectAndExecute blocked on a silent device")
	}
}

// A connection that drops part way through must not be reported as a backup.
func TestCollectOutputFailsOnDroppedConnection(t *testing.T) {
	port := startFakeDevice(t, fakeDevice{
		prompt: "spine-01#",
		responses: map[string]string{
			"enable":            "",
			"terminal length 0": "",
			"show inventory":    "\nArista DCS-7050TX-64\n",
		},
		closeOn: "show running-config",
	})

	_, _, err := collectOutput(testDevice(t, port, 5*time.Second), testModel(t))
	if err == nil {
		t.Fatal("connectAndExecute succeeded after the connection dropped")
	}
}
