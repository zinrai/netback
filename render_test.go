package main

import (
	"strings"
	"testing"
)

func secretsModel(t *testing.T, rules ...FilterRule) *Model {
	t.Helper()

	model := &Model{
		Prompt:   `\S+#\s*$`,
		Commands: []string{"show running-config"},
		Secrets:  rules,
	}

	if err := compileModel(model); err != nil {
		t.Fatalf("compile model: %v", err)
	}

	return model
}

// Masking is the one step whose failure leaves no trace: the backup still
// looks right, it just has the credential in it.
func TestMaskSecrets(t *testing.T) {
	model := secretsModel(t,
		FilterRule{Pattern: `^(snmp-server community).*`, Replace: `$1 <removed>`},
		FilterRule{Pattern: `(secret \w+) (\S+).*`, Replace: `$1 <removed>`},
	)

	input := strings.Join([]string{
		"hostname spine-01",
		"snmp-server community s3cr3t ro",
		"username admin secret sha512 $6$abcdef",
		"interface Ethernet1",
	}, "\n")

	got := maskSecrets(input, model.Secrets)

	for _, leaked := range []string{"s3cr3t", "$6$abcdef"} {
		if strings.Contains(got, leaked) {
			t.Errorf("secret %q survived masking:\n%s", leaked, got)
		}
	}

	if !strings.Contains(got, "interface Ethernet1") {
		t.Errorf("masking removed unrelated configuration:\n%s", got)
	}
}

// An anchored rule has to match wherever the line appears. Anchoring to the
// captured output as a whole masks the first occurrence and writes every later
// one to the backup in the clear.
func TestMaskSecretsAnchorsToLines(t *testing.T) {
	model := secretsModel(t, FilterRule{Pattern: `^(snmp-server community).*`, Replace: `$1 <removed>`})

	input := "hostname spine-01\nsnmp-server community first ro\nsnmp-server community second rw"

	got := maskSecrets(input, model.Secrets)

	if strings.Contains(got, "first") || strings.Contains(got, "second") {
		t.Errorf("an anchored rule missed a line:\n%s", got)
	}
}

// The stored file has to stay usable as a configuration, so only the command
// echo and the prompt after it are commented.
func TestCommentFirstLastLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "echo and prompt",
			input: "show running-config\nhostname spine-01\ninterface Ethernet1\nspine-01#",
			want:  "! show running-config\nhostname spine-01\ninterface Ethernet1\n! spine-01#",
		},
		{
			name:  "trailing blank line",
			input: "show running-config\nhostname spine-01\nspine-01#\n",
			want:  "! show running-config\nhostname spine-01\n! spine-01#\n",
		},
		{
			name:  "single line",
			input: "spine-01#",
			want:  "! spine-01#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commentFirstLastLines(tt.input, "! "); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
