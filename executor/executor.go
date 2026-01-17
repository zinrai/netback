package executor

import (
	"fmt"
	"strings"

	"github.com/zinrai/netback/config"
	"github.com/zinrai/netback/transport"
)

// Result represents the result of backing up a device
type Result struct {
	Device *config.Device
	Output string
	Error  error
}

// Execute connects to a device and collects the configuration
func Execute(device *config.Device, model *config.Model) *Result {
	result := &Result{Device: device}

	commentsOutputs, commandsOutputs, err := transport.ConnectAndExecute(device, model)
	if err != nil {
		result.Error = err
		return result
	}

	var outputParts []string

	// Process comments outputs (all lines commented for each command)
	for _, output := range commentsOutputs {
		processed, err := processOutput(output, model)
		if err != nil {
			result.Error = err
			return result
		}
		processed = commentAllLines(processed, model.Comment)
		if processed != "" {
			outputParts = append(outputParts, processed)
		}
	}

	// Process commands outputs (first and last lines commented for each command)
	for _, output := range commandsOutputs {
		processed, err := processOutput(output, model)
		if err != nil {
			result.Error = err
			return result
		}
		processed = commentFirstLastLines(processed, model.Comment)
		if processed != "" {
			outputParts = append(outputParts, processed)
		}
	}

	result.Output = strings.Join(outputParts, "\n")

	return result
}

// processOutput applies all filtering rules to the raw output
func processOutput(rawOutput string, model *config.Model) (string, error) {
	output := rawOutput

	// Apply secrets masking
	output, err := applySecrets(output, model.Secrets)
	if err != nil {
		return "", err
	}

	// Apply expect replacements (for any remaining patterns)
	output, err = applyExpectReplacements(output, model.Expect)
	if err != nil {
		return "", err
	}

	return output, nil
}

// applySecrets masks sensitive information in the output
func applySecrets(output string, secrets []config.FilterRule) (string, error) {
	for _, secret := range secrets {
		re, err := secret.Regex()
		if err != nil {
			return "", fmt.Errorf("secret pattern: %w", err)
		}
		output = re.ReplaceAllString(output, secret.Replace)
	}
	return output, nil
}

// applyExpectReplacements applies any replace rules from expect patterns
func applyExpectReplacements(output string, expects []config.ExpectRule) (string, error) {
	for _, expect := range expects {
		if expect.Replace == "" && expect.Send != "" {
			// This is a send-only rule, skip replacement
			continue
		}
		re, err := expect.Regex()
		if err != nil {
			return "", fmt.Errorf("expect pattern: %w", err)
		}
		output = re.ReplaceAllString(output, expect.Replace)
	}
	return output, nil
}

// commentAllLines prefixes each line with the comment string
func commentAllLines(output string, prefix string) string {
	if output == "" || prefix == "" {
		return output
	}
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// commentFirstLastLines comments only the first and last non-empty lines
func commentFirstLastLines(output string, prefix string) string {
	if output == "" || prefix == "" {
		return output
	}

	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return output
	}

	// Find first non-empty line
	firstIdx := -1
	for i, line := range lines {
		if line != "" {
			firstIdx = i
			break
		}
	}

	if firstIdx == -1 {
		// All lines are empty
		return output
	}

	// Find last non-empty line
	lastIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			lastIdx = i
			break
		}
	}

	// Comment the first line
	lines[firstIdx] = prefix + lines[firstIdx]

	// Comment the last line (if different from first)
	if lastIdx != firstIdx {
		lines[lastIdx] = prefix + lines[lastIdx]
	}

	return strings.Join(lines, "\n")
}
