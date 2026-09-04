package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/goccy/go-yaml"
)

type ModelFile struct {
	Models map[string]*Model `yaml:"models"`
}

// Patterns are not compiled on first use: one Model is shared by every device
// that references it, and those devices are backed up concurrently.
type Model struct {
	Prompt     string           `yaml:"prompt"`
	Comment    string           `yaml:"comment"`
	Connection ConnectionConfig `yaml:"connection"`
	Secrets    []FilterRule     `yaml:"secrets"`
	Comments   []string         `yaml:"comments"`
	Commands   []string         `yaml:"commands"`

	promptRe *regexp.Regexp
}

type ConnectionConfig struct {
	PostLogin []string `yaml:"post_login"`
	PreLogout string   `yaml:"pre_logout"`
}

type FilterRule struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`

	re *regexp.Regexp
}

func loadModelFile(path string) (*ModelFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model file: %w", err)
	}

	var mf ModelFile
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parse model file: %w", err)
	}

	if len(mf.Models) == 0 {
		return nil, fmt.Errorf("no models defined")
	}

	for name, m := range mf.Models {
		// Not a Model with zero values: a key written under models with no
		// body below it unmarshals to a nil pointer.
		if m == nil {
			return nil, fmt.Errorf("model %q: no definition", name)
		}

		if err := compileModel(m); err != nil {
			return nil, fmt.Errorf("model %q: %w", name, err)
		}
	}

	return &mf, nil
}

func compileModel(m *Model) error {
	if m.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if len(m.Commands) == 0 {
		return fmt.Errorf("at least one command is required")
	}

	promptRe, err := regexp.Compile(m.Prompt)
	if err != nil {
		return fmt.Errorf("compile prompt %q: %w", m.Prompt, err)
	}
	m.promptRe = promptRe

	for i := range m.Secrets {
		// Not compiled as written: in Go an unflagged ^ anchors to the whole
		// captured output, so an anchored rule would mask only the first line
		// of a configuration and leave every later occurrence in the backup.
		re, err := regexp.Compile("(?m)" + m.Secrets[i].Pattern)
		if err != nil {
			return fmt.Errorf("compile secrets[%d] %q: %w", i, m.Secrets[i].Pattern, err)
		}
		m.Secrets[i].re = re
	}

	return nil
}
