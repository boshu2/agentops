package session

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/sessionapp"
)

// subcommand returns the named child of the session command, constructing the
// module directly instead of mutating any package-global command state.
func subcommand(t *testing.T, name string) (root, child *cobra.Command) {
	t.Helper()
	root = NewModule(clicontract.HostOptions{}).Command()
	for _, candidate := range root.Commands() {
		if candidate.Name() == name {
			return root, candidate
		}
	}
	t.Fatalf("session command has no %q subcommand", name)
	return nil, nil
}

func TestModule_Contract(t *testing.T) {
	contract := NewModule(clicontract.HostOptions{}).Contract()
	if contract.ID != "ao.session" {
		t.Fatalf("contract ID = %q, want ao.session", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects&clicontract.EffectFilesystem == 0 {
		t.Fatalf("effects = %v, want filesystem", contract.Effects)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	root := NewModule(clicontract.HostOptions{}).Command()
	if root.Use != "session" {
		t.Fatalf("Use = %q, want session", root.Use)
	}
	if root.GroupID != "workflow" {
		t.Fatalf("GroupID = %q, want workflow", root.GroupID)
	}
	seen := map[string]bool{}
	for _, child := range root.Commands() {
		seen[child.Name()] = true
	}
	for _, want := range []string{"bootstrap", "rehydrate"} {
		if !seen[want] {
			t.Errorf("session missing subcommand %q", want)
		}
	}
}

func TestSessionBootstrapOnlyReportsLocalOrientation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	root, _ := subcommand(t, "bootstrap")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"bootstrap", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var status sessionapp.BootstrapStatus
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.OrientationFiles) != 1 || status.OrientationFiles[0] != "AGENTS.md" {
		t.Fatalf("unexpected orientation files: %#v", status.OrientationFiles)
	}
}

func TestRehydrateReadsCallerAuthoredBrief(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	handoffDir := filepath.Join(dir, ".agents", "handoff")
	if err := os.MkdirAll(handoffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a handoff artifact directly (the writer is the separate `ao session
	// handoff` command); rehydrate must read it without consuming or mutating it.
	original := []byte(`{"schema_version":1,"id":"handoff-20260719T000000.000000000Z","created_at":"2026-07-19T00:00:00Z","goal":"prove one behavior","continuation":"caller will choose whether to revise"}` + "\n")
	artifactPath := filepath.Join(handoffDir, "handoff-20260719T000000.000000000Z.json")
	if err := os.WriteFile(artifactPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	root, _ := subcommand(t, "rehydrate")
	var restored bytes.Buffer
	root.SetOut(&restored)
	root.SetArgs([]string{"rehydrate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.String(), "caller will choose whether to revise") {
		t.Fatalf("caller continuation missing: %s", restored.String())
	}
	if after, err := os.ReadFile(artifactPath); err != nil || !bytes.Equal(original, after) {
		t.Fatal("rehydrate mutated the handoff artifact")
	}
}

// TestRehydrateJSONEmptyStateEmitsEmptyObject asserts that --json with no
// handoff present emits exactly one JSON document `{}` on stdout (jq-safe), with
// the human hint on stderr and exit 0.
func TestRehydrateJSONEmptyStateEmitsEmptyObject(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	root, _ := subcommand(t, "rehydrate")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"rehydrate", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("rehydrate returned error: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "{}" {
		t.Fatalf("stdout = %q, want exactly \"{}\"", got)
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (raw stdout: %q)", err, stdout.String())
	}
	if !strings.Contains(stderr.String(), "no handoff found") {
		t.Errorf("stderr = %q, want the 'no handoff found' hint", stderr.String())
	}
}
