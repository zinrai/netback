package main

import "strings"

// Output from commands is not commented in full: it is the configuration
// itself, and the body has to stay usable as one.
func renderOutput(comments, commands []string, model *Model) string {
	parts := make([]string, 0, len(comments)+len(commands))

	for _, out := range comments {
		if part := commentAllLines(maskSecrets(out, model.Secrets), model.Comment); part != "" {
			parts = append(parts, part)
		}
	}

	for _, out := range commands {
		if part := commentFirstLastLines(maskSecrets(out, model.Secrets), model.Comment); part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "\n")
}

func maskSecrets(output string, secrets []FilterRule) string {
	for i := range secrets {
		output = secrets[i].re.ReplaceAllString(output, secrets[i].Replace)
	}
	return output
}

func commentAllLines(output, prefix string) string {
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

// Not the first and last lines as they come: a response can end in blank lines
// and the prompt is the last line with anything on it.
func commentFirstLastLines(output, prefix string) string {
	if output == "" || prefix == "" {
		return output
	}

	lines := strings.Split(output, "\n")

	first := -1
	for i, line := range lines {
		if line != "" {
			first = i
			break
		}
	}

	if first == -1 {
		return output
	}

	last := first
	for i := len(lines) - 1; i > first; i-- {
		if lines[i] != "" {
			last = i
			break
		}
	}

	lines[first] = prefix + lines[first]
	if last != first {
		lines[last] = prefix + lines[last]
	}

	return strings.Join(lines, "\n")
}
