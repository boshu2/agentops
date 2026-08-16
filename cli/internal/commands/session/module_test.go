package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for _, want := range []string{"bootstrap", "prune-agents", "rehydrate"} {
		if !seen[want] {
			t.Errorf("session missing subcommand %q", want)
		}
	}
}

func TestPruneAgentsCommandDryRunSeamOverridesExecute(t *testing.T) {
	dir := t.TempDir()
	handoffDir := filepath.Join(dir, ".agents", "ao", "handoff")
	if err := os.MkdirAll(handoffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		path := filepath.Join(handoffDir, fmt.Sprintf("handoff-%02d.json", i))
		if err := os.WriteFile(path, []byte("candidate\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 8, 16, 1, i, 0, 0, time.UTC)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	module := NewModule(clicontract.HostOptions{
		DryRun:      func() bool { return true },
		ProjectRoot: func() string { return dir },
		Now:         func() time.Time { return time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC) },
	})
	root := module.Command()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"prune-agents", "--execute"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(handoffDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 12 {
		t.Fatalf("global dry-run left %d handoffs, want 12", len(entries))
	}
	if !strings.Contains(output.String(), "DRY RUN COMPLETE") {
		t.Fatalf("global dry-run did not select read-only output:\n%s", output.String())
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

func TestRehydrateReadsCanonicalHandoffDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	handoffDir := filepath.Join(dir, ".agents", "ao", "handoff")
	if err := os.MkdirAll(handoffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","goal":"read the canonical handoff","continuation":"canonical path is visible"}` + "\n")
	artifactPath := filepath.Join(handoffDir, "handoff-20260816T000000.000000000Z.json")
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
	if !strings.Contains(restored.String(), "canonical path is visible") {
		t.Fatalf("canonical handoff missing: %s", restored.String())
	}
	if after, err := os.ReadFile(artifactPath); err != nil || !bytes.Equal(original, after) {
		t.Fatal("rehydrate mutated the canonical handoff artifact")
	}
}

func TestRehydrateChoosesLatestAcrossCanonicalAndLegacyDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	legacyDir := filepath.Join(dir, ".agents", "handoff")
	canonicalDir := filepath.Join(dir, ".agents", "ao", "handoff")
	for _, handoffDir := range []string{legacyDir, canonicalDir} {
		if err := os.MkdirAll(handoffDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	legacy := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"older legacy artifact"}` + "\n")
	canonical := []byte(`{"schema_version":1,"id":"handoff-20260816T000001.000000000Z","created_at":"2026-08-16T00:00:01Z","continuation":"newer canonical artifact"}` + "\n")
	if err := os.WriteFile(filepath.Join(legacyDir, "handoff-20260816T000000.000000000Z.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonicalDir, "handoff-20260816T000001.000000000Z.json"), canonical, 0o600); err != nil {
		t.Fatal(err)
	}

	root, _ := subcommand(t, "rehydrate")
	var restored bytes.Buffer
	root.SetOut(&restored)
	root.SetArgs([]string{"rehydrate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.String(), "newer canonical artifact") {
		t.Fatalf("latest handoff not selected across directories: %s", restored.String())
	}
}

func TestRehydrateChoosesNewerLegacyAcrossDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	legacyDir := filepath.Join(dir, ".agents", "handoff")
	canonicalDir := filepath.Join(dir, ".agents", "ao", "handoff")
	for _, handoffDir := range []string{legacyDir, canonicalDir} {
		if err := os.MkdirAll(handoffDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	canonical := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"older canonical artifact"}` + "\n")
	legacy := []byte(`{"schema_version":1,"id":"handoff-20260816T000001.000000000Z","created_at":"2026-08-16T00:00:01Z","continuation":"newer legacy artifact"}` + "\n")
	if err := os.WriteFile(filepath.Join(canonicalDir, "handoff-20260816T000000.000000000Z.json"), canonical, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "handoff-20260816T000001.000000000Z.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	root, _ := subcommand(t, "rehydrate")
	var restored bytes.Buffer
	root.SetOut(&restored)
	root.SetArgs([]string{"rehydrate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.String(), "newer legacy artifact") {
		t.Fatalf("newer legacy handoff not selected across directories: %s", restored.String())
	}
}

func TestRehydratePrefersCanonicalDirectoryForDuplicateName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	legacyDir := filepath.Join(dir, ".agents", "handoff")
	canonicalDir := filepath.Join(dir, ".agents", "ao", "handoff")
	for _, handoffDir := range []string{legacyDir, canonicalDir} {
		if err := os.MkdirAll(handoffDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	name := "handoff-20260816T000000.000000000Z.json"
	legacy := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"legacy duplicate"}` + "\n")
	canonical := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"canonical duplicate"}` + "\n")
	if err := os.WriteFile(filepath.Join(legacyDir, name), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonicalDir, name), canonical, 0o600); err != nil {
		t.Fatal(err)
	}

	root, _ := subcommand(t, "rehydrate")
	var restored bytes.Buffer
	root.SetOut(&restored)
	root.SetArgs([]string{"rehydrate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.String(), "canonical duplicate") {
		t.Fatalf("canonical directory did not win duplicate name: %s", restored.String())
	}
}

func TestRehydrateFailsClosedWhenCanonicalRootIsNotDirectory(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "human", args: []string{"rehydrate"}},
		{name: "json", args: []string{"rehydrate", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			agents := filepath.Join(dir, ".agents")
			legacyDir := filepath.Join(agents, "handoff")
			if err := os.MkdirAll(legacyDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(agents, "ao"), []byte("not a directory\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			legacy := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"valid legacy evidence"}` + "\n")
			if err := os.WriteFile(filepath.Join(legacyDir, "handoff-20260816T000000.000000000Z.json"), legacy, 0o600); err != nil {
				t.Fatal(err)
			}

			root, _ := subcommand(t, "rehydrate")
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(tc.args)
			err := root.Execute()
			if err == nil {
				t.Fatal("rehydrate succeeded despite a non-directory canonical root")
			}
			if !strings.Contains(err.Error(), "not a real directory") {
				t.Fatalf("error = %q, want unsafe canonical-root reason", err)
			}
			if strings.Contains(stdout.String(), "valid legacy evidence") || strings.TrimSpace(stdout.String()) == "{}" {
				t.Fatalf("stdout = %q, want no fallback artifact or empty-state document", stdout.String())
			}
		})
	}
}

func TestRehydrateFailsClosedOnSymlinkedHandoffSources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{
			name: "intermediate canonical root",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				agents := filepath.Join(dir, ".agents")
				external := t.TempDir()
				if err := os.MkdirAll(agents, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(agents, "ao")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
		{
			name: "matching artifact",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				canonical := filepath.Join(dir, ".agents", "ao", "handoff")
				if err := os.MkdirAll(canonical, 0o755); err != nil {
					t.Fatal(err)
				}
				external := filepath.Join(t.TempDir(), "outside.json")
				if err := os.WriteFile(external, []byte(`{"schema_version":1,"id":"handoff-20260816T000001.000000000Z","created_at":"2026-08-16T00:00:01Z","continuation":"outside secret"}`+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(canonical, "handoff-20260816T000001.000000000Z.json")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			tc.setup(t, dir)
			legacyDir := filepath.Join(dir, ".agents", "handoff")
			if err := os.MkdirAll(legacyDir, 0o755); err != nil {
				t.Fatal(err)
			}
			legacy := []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"valid legacy evidence"}` + "\n")
			if err := os.WriteFile(filepath.Join(legacyDir, "handoff-20260816T000000.000000000Z.json"), legacy, 0o600); err != nil {
				t.Fatal(err)
			}

			root, _ := subcommand(t, "rehydrate")
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"rehydrate", "--json"})
			err := root.Execute()
			if err == nil {
				t.Fatal("rehydrate succeeded through an unsafe handoff source")
			}
			if !strings.Contains(err.Error(), "not a real") {
				t.Fatalf("error = %q, want unsafe source reason", err)
			}
			if strings.Contains(stdout.String(), "outside secret") || strings.Contains(stdout.String(), "valid legacy evidence") || strings.TrimSpace(stdout.String()) == "{}" {
				t.Fatalf("stdout = %q, want no followed, fallback, or empty-state artifact", stdout.String())
			}
		})
	}
}

func TestRehydrateRejectsArtifactsOutsideHandoffV1Contract(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing required identity", body: `{"schema_version":1,"continuation":"not bound"}`},
		{name: "filename id mismatch", body: `{"schema_version":1,"id":"handoff-20260816T000001.000000000Z","created_at":"2026-08-16T00:00:00Z"}`},
		{name: "unknown property", body: `{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","lifecycle":"invented"}`},
		{name: "invalid nested state", body: `{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","state":{"git_branch":"main"}}`},
		{name: "null string", body: `{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","goal":null}`},
		{name: "null array", body: `{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","artifacts_produced":null}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			handoffDir := filepath.Join(dir, ".agents", "ao", "handoff")
			if err := os.MkdirAll(handoffDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(handoffDir, "handoff-20260816T000000.000000000Z.json"), []byte(tc.body+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			root, _ := subcommand(t, "rehydrate")
			var stdout bytes.Buffer
			root.SetOut(&stdout)
			root.SetArgs([]string{"rehydrate", "--json"})
			if err := root.Execute(); err == nil {
				t.Fatal("rehydrate accepted an artifact outside handoff.v1")
			}
			if strings.TrimSpace(stdout.String()) == "{}" {
				t.Fatal("invalid artifact was reported as an honest empty state")
			}
		})
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
