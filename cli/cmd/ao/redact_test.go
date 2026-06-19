package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRunRedact_ScrubsStdinToStdout is the L2 proof for ag-sz3h: `ao redact`
// reads stdin, scrubs secrets via the canonical redactor, and writes the
// scrubbed bytes to stdout while preserving non-sensitive content.
func TestRunRedact_ScrubsStdinToStdout(t *testing.T) {
	const safe = "This article is about caching strategy."
	secret := "sk-ant-api03-" + strings.Repeat("A", 45)
	in := safe + "\ntoken: " + secret + "\n"

	cmd := redactCmd
	cmd.SetIn(strings.NewReader(in))
	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil); cmd.SetIn(nil) }) // age-ztf8: shared command; don't leak the writer/reader

	if err := runRedact(cmd, nil); err != nil {
		t.Fatalf("runRedact: %v", err)
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
}
