package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const defaultPort = 22

type Device struct {
	Name         string        `yaml:"name"`
	IP           string        `yaml:"ip"`
	Model        string        `yaml:"model"`
	Group        string        `yaml:"group"`
	Port         int           `yaml:"port"`
	Username     string        `yaml:"username"`
	PasswordFile string        `yaml:"password_file"`
	Timeout      time.Duration `yaml:"timeout"`

	password string
}

type RouterDB struct {
	Devices []Device `yaml:"devices"`
}

// Defaults are filled in here rather than read through an accessor so that
// nothing downstream has to decide what an unset field means.
func loadRouterDB(path string, defaultTimeout time.Duration) (*RouterDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routerdb: %w", err)
	}

	var db RouterDB
	if err := yaml.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("parse routerdb: %w", err)
	}

	if len(db.Devices) == 0 {
		return nil, fmt.Errorf("no devices defined")
	}

	for i := range db.Devices {
		if err := prepareDevice(&db.Devices[i], defaultTimeout); err != nil {
			return nil, fmt.Errorf("%s: %w", deviceLabel(i, &db.Devices[i]), err)
		}
	}

	return &db, nil
}

func prepareDevice(d *Device, defaultTimeout time.Duration) error {
	if err := validateDevice(d); err != nil {
		return err
	}

	// Not deferred to connect time: an unreadable password file should stop
	// the run before any device is contacted.
	password, err := readPasswordFile(d.PasswordFile)
	if err != nil {
		return err
	}
	d.password = password

	if d.Port == 0 {
		d.Port = defaultPort
	}
	if d.Timeout == 0 {
		d.Timeout = defaultTimeout
	}

	return nil
}

// The name is what identifies a device to whoever has to fix the file, so it
// is not left out when it is the field that is missing.
func deviceLabel(i int, d *Device) string {
	if d.Name == "" {
		return fmt.Sprintf("device[%d]", i)
	}
	return fmt.Sprintf("device[%d] (%s)", i, d.Name)
}

func validateDevice(d *Device) error {
	required := []struct {
		field string
		value string
	}{
		{"name", d.Name},
		{"ip", d.IP},
		{"model", d.Model},
		{"group", d.Group},
		{"username", d.Username},
		{"password_file", d.PasswordFile},
	}

	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("%s is required", r.field)
		}
	}

	return nil
}

// Not TrimSpace: a password may end with a space, so only the newline a file
// almost always ends with is removed.
func readPasswordFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read password file: %w", err)
	}

	password := strings.TrimSuffix(string(data), "\n")
	password = strings.TrimSuffix(password, "\r")

	if password == "" {
		return "", fmt.Errorf("password file %s is empty", path)
	}

	return password, nil
}
