package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestRehydrateJSONEmptyStateEmitsEmptyObject asserts that --json with no
// handoff present emits exactly one JSON document `{}` on stdout (jq-safe), with
// the human hint on stderr and exit 0. RED before the fix: the "no handoff found"
// prose is printed to stdout regardless of --json, so json.Decoder fails.
func TestRehydrateJSONEmptyStateEmitsEmptyObject(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	readCommand := *rehydrateCmd
	readCommand.SetOut(&stdout)
	readCommand.SetErr(&stderr)
	rehydrateJSON = true
	t.Cleanup(func() { rehydrateJSON = false })
	if err := runRehydrate(&readCommand, nil); err != nil {
		t.Fatalf("rehydrate returned error: %v", err)
	}

	// Shape-exact: the empty state must print the literal `{}` and nothing else
	// on stdout. A byte-exact check rejects `null` (which decodes into a nil map
	// and would pass len==0) and any second document, without relying on
	// Decoder.More() for the one-document guarantee.
	if got := strings.TrimSpace(stdout.String()); got != "{}" {
		t.Fatalf("stdout = %q, want exactly \"{}\"", got)
	}
	// Also confirm it parses as one JSON object for good measure.
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (raw stdout: %q)", err, stdout.String())
	}
	if !strings.Contains(stderr.String(), "no handoff found") {
		t.Errorf("stderr = %q, want the 'no handoff found' hint", stderr.String())
	}
}
