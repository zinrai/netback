package main

import (
	"fmt"
	"log"
	"time"
)

func collectOutput(device *Device, model *Model) (comments, commands []string, err error) {
	client := &sshClient{device: device, model: model}

	session, err := client.connect()
	if err != nil {
		return nil, nil, err
	}
	defer client.close()

	log.Printf("%s: logging in...", device.Name)
	if err := session.login(); err != nil {
		return nil, nil, err
	}

	log.Printf("%s: executing comments...", device.Name)
	comments, err = runCommands(session, model.Comments)
	if err != nil {
		return nil, nil, err
	}

	log.Printf("%s: executing commands...", device.Name)
	commands, err = runCommands(session, model.Commands)
	if err != nil {
		return nil, nil, err
	}

	_ = session.logout()

	// Not closed immediately: the logout command is still on its way to the
	// device.
	time.Sleep(100 * time.Millisecond)

	return comments, commands, nil
}

func runCommands(session *Session, commands []string) ([]string, error) {
	outputs := make([]string, 0, len(commands))

	for _, cmd := range commands {
		output, err := session.Execute(cmd)
		if err != nil {
			return nil, fmt.Errorf("execute %q: %w", cmd, err)
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}
