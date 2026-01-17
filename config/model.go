package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/goccy/go-yaml"
)

// ModelFile represents the top-level structure of model.yaml
type ModelFile struct {
	Models map[string]*Model `yaml:"models"`
}

// Model represents a device model definition
type Model struct {
	Prompt      string           `yaml:"prompt"`
	Comment     string           `yaml:"comment"`
	Connection  ConnectionConfig `yaml:"connection"`
	Expect      []ExpectRule     `yaml:"expect"`
	Secrets     []FilterRule     `yaml:"secrets"`
	Comments    []string         `yaml:"comments"`
	Commands    []string         `yaml:"commands"`
	promptRegex *regexp.Regexp
}

// ConnectionConfig represents connection settings
type ConnectionConfig struct {
	PostLogin []string `yaml:"post_login"`
	PreLogout string   `yaml:"pre_logout"`
}

// ExpectRule represents an expect/response rule for interactive handling
type ExpectRule struct {
	Pattern string `yaml:"pattern"`
	Send    string `yaml:"send"`
	Replace string `yaml:"replace"`
	regex   *regexp.Regexp
}

// Regex returns the compiled regex for this rule
func (e *ExpectRule) Regex() (*regexp.Regexp, error) {
	if e.regex == nil {
		re, err := regexp.Compile(e.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile expect pattern %q: %w", e.Pattern, err)
		}
		e.regex = re
	}
	return e.regex, nil
}

// FilterRule represents a pattern replacement rule (for secrets or filters)
type FilterRule struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
	regex   *regexp.Regexp
}

// Regex returns the compiled regex for this rule
func (f *FilterRule) Regex() (*regexp.Regexp, error) {
	if f.regex == nil {
		re, err := regexp.Compile(f.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile filter pattern %q: %w", f.Pattern, err)
		}
		f.regex = re
	}
	return f.regex, nil
}

// PromptRegex returns the compiled prompt regex
func (m *Model) PromptRegex() (*regexp.Regexp, error) {
	if m.promptRegex == nil {
		re, err := regexp.Compile(m.Prompt)
		if err != nil {
			return nil, fmt.Errorf("compile prompt pattern %q: %w", m.Prompt, err)
		}
		m.promptRegex = re
	}
	return m.promptRegex, nil
}

// LoadModelFile loads and parses model.yaml
func LoadModelFile(path string) (*ModelFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model file: %w", err)
	}

	var mf ModelFile
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parse model file: %w", err)
	}

	if err := validateModelFile(&mf); err != nil {
		return nil, err
	}

	return &mf, nil
}

func validateModelFile(mf *ModelFile) error {
	if len(mf.Models) == 0 {
		return fmt.Errorf("no models defined")
	}

	for name, m := range mf.Models {
		if m.Prompt == "" {
			return fmt.Errorf("model %q: prompt is required", name)
		}

		// Validate prompt regex compiles
		if _, err := m.PromptRegex(); err != nil {
			return fmt.Errorf("model %q: %w", name, err)
		}

		// Validate expect patterns
		for i, e := range m.Expect {
			if _, err := e.Regex(); err != nil {
				return fmt.Errorf("model %q expect[%d]: %w", name, i, err)
			}
		}

		// Validate secret patterns
		for i, s := range m.Secrets {
			if _, err := s.Regex(); err != nil {
				return fmt.Errorf("model %q secrets[%d]: %w", name, i, err)
			}
		}

		// Validate at least one command is defined
		if len(m.Commands) == 0 {
			return fmt.Errorf("model %q: at least one command is required", name)
		}
	}

	return nil
}
