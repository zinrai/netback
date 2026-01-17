package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// Device represents a single device entry in routerdb.yaml
type Device struct {
	Name     string        `yaml:"name"`
	IP       string        `yaml:"ip"`
	Model    string        `yaml:"model"`
	Group    string        `yaml:"group"`
	Port     int           `yaml:"port"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
	Timeout  time.Duration `yaml:"timeout"`
}

// RouterDB represents the top-level structure of routerdb.yaml
type RouterDB struct {
	Devices []Device `yaml:"devices"`
}

// EffectivePort returns the port to use, defaulting to 22 for SSH
func (d *Device) EffectivePort() int {
	if d.Port == 0 {
		return 22
	}
	return d.Port
}

// EffectiveTimeout returns the timeout to use, defaulting to 30 seconds
func (d *Device) EffectiveTimeout() time.Duration {
	if d.Timeout == 0 {
		return 30 * time.Second
	}
	return d.Timeout
}

// LoadRouterDB loads and parses routerdb.yaml from a file path
func LoadRouterDB(path string) (*RouterDB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open routerdb: %w", err)
	}
	defer f.Close()

	var db RouterDB
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&db); err != nil {
		return nil, fmt.Errorf("parse routerdb: %w", err)
	}

	if err := validateRouterDB(&db); err != nil {
		return nil, err
	}

	return &db, nil
}

func validateRouterDB(db *RouterDB) error {
	for i, d := range db.Devices {
		if d.Name == "" {
			return fmt.Errorf("device[%d]: name is required", i)
		}
		if d.IP == "" {
			return fmt.Errorf("device[%d] (%s): ip is required", i, d.Name)
		}
		if d.Model == "" {
			return fmt.Errorf("device[%d] (%s): model is required", i, d.Name)
		}
		if d.Group == "" {
			return fmt.Errorf("device[%d] (%s): group is required", i, d.Name)
		}
		if d.Username == "" {
			return fmt.Errorf("device[%d] (%s): username is required", i, d.Name)
		}
		if d.Password == "" {
			return fmt.Errorf("device[%d] (%s): password is required", i, d.Name)
		}
	}
	return nil
}
