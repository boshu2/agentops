package redact

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	redactlib "github.com/boshu2/agentops/cli/internal/redact"
)

func TestModule_Contract(t *testing.T) {
	contract := NewModule().Contract()
	if contract.ID != "ao.redact" {
		t.Fatalf("contract ID = %q, want ao.redact", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
	if contract.Args.Name != "no-args" {
		t.Fatalf("args = %q, want no-args", contract.Args.Name)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := NewModule().Command()
	if command.Use != "redact" {
		t.Errorf("Use = %q, want redact", command.Use)
	}
	if command.GroupID != "core" {
		t.Errorf("GroupID = %q, want core", command.GroupID)
	}
	if command.Short != "Scrub secrets from stdin to stdout (canonical redactor)" {
		t.Errorf("Short = %q", command.Short)
	}
}

// TestRunRedact_ScrubsStdinToStdout is the relocated L2 proof for ag-sz3h:
// `ao redact` reads stdin, scrubs secrets via the canonical redactor, and writes
// the scrubbed bytes to stdout while preserving non-sensitive content.
func TestRunRedact_ScrubsStdinToStdout(t *testing.T) {
	const safe = "This article is about caching strategy."
	secret := "sk-ant-api03-" + strings.Repeat("A", 45)
	in := safe + "\ntoken: " + secret + "\n"

	command := NewModule().Command()
	command.SetIn(strings.NewReader(in))
	var out bytes.Buffer
	command.SetOut(&out)

	if err := run(command, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()

	if strings.Contains(got, secret) {
		t.Fatalf("ao redact leaked the secret; got:\n%s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("ao redact missing [REDACTED] placeholder; got:\n%s", got)
	}
	if !strings.Contains(got, safe) {
		t.Fatalf("ao redact clobbered safe content; want %q in:\n%s", safe, got)
	}

	// Equivalence with the extracted redactor: the command must remain a
	// byte-exact pass-through of internal/redact.RedactBytes.
	if want := string(redactlib.RedactBytes([]byte(in))); got != want {
		t.Fatalf("ao redact output diverged from redact.RedactBytes:\n got:  %q\n want: %q", got, want)
	}
}
