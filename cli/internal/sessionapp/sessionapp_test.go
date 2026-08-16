package sessionapp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const stableHandoffName = "handoff-20260816T000000.000000000Z.json"

func validStoredHandoff(continuation string) []byte {
	return []byte(`{"schema_version":1,"id":"handoff-20260816T000000.000000000Z","created_at":"2026-08-16T00:00:00Z","continuation":"` + continuation + `"}` + "\n")
}

func TestRehydrateFailsClosedWhenRootBindingChanges(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	agents := filepath.Join(dir, ".agents")
	canonical := filepath.Join(agents, "ao", "handoff")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonical, stableHandoffName), validStoredHandoff("original evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "handoff"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "handoff", stableHandoffName), validStoredHandoff("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	handoffReadTestHook = func(stage string) {
		if stage != "before-artifact-open" {
			return
		}
		handoffReadTestHook = nil
		if err := os.Rename(filepath.Join(agents, "ao"), filepath.Join(agents, "ao.original")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(agents, "ao")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}
	t.Cleanup(func() { handoffReadTestHook = nil })

	var stdout, stderr bytes.Buffer
	err := Rehydrate(RehydrateOptions{JSON: true, Stdout: &stdout, Stderr: &stderr})
	if err == nil {
		t.Fatal("rehydrate accepted a handoff after its root path changed identity")
	}
	if strings.Contains(stdout.String(), "outside secret") || strings.Contains(stdout.String(), "original evidence") || strings.TrimSpace(stdout.String()) == "{}" {
		t.Fatalf("unsafe rehydrate output = %q", stdout.String())
	}
}

func TestRehydrateFailsClosedWhenArtifactChangesDuringRead(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	canonical := filepath.Join(dir, ".agents", "ao", "handoff")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(canonical, stableHandoffName)
	if err := os.WriteFile(path, validStoredHandoff("first bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	handoffReadTestHook = func(stage string) {
		if stage != "after-first-read" {
			return
		}
		handoffReadTestHook = nil
		if err := os.WriteFile(path, validStoredHandoff("different and longer bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { handoffReadTestHook = nil })

	var stdout, stderr bytes.Buffer
	err := Rehydrate(RehydrateOptions{JSON: true, Stdout: &stdout, Stderr: &stderr})
	if err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("Rehydrate error = %v, want unstable-artifact refusal", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unstable artifact leaked output: %q", stdout.String())
	}
}

func TestDecodeStoredHandoffAcceptsSchemaValidDeprecatedFields(t *testing.T) {
	data := []byte(`{
  "schema_version": 1,
  "id": "handoff-20260816T000000.000000000Z",
  "created_at": "2026-08-16T00:00:00Z",
  "type": "rpi",
  "artifacts_produced": ["report.md"],
  "decisions_made": [],
  "open_risks": ["caller decides"],
  "rpi": {"phase": 3, "phase_name": "validation", "verdicts": {"scope": "PASS"}},
  "state": {"git_dirty": false, "open_beads_count": 0},
  "consumed": false,
  "consumed_at": null,
  "consumed_by": null
}`)
	var artifact storedHandoff
	if err := decodeStoredHandoff(data, stableHandoffName, &artifact); err != nil {
		t.Fatalf("schema-valid deprecated artifact rejected: %v", err)
	}
}
