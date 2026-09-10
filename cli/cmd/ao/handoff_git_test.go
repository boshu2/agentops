package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestHandoffCollectIncludesUntrackedNames(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, gitDiscoveryEnv()
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	run("init", "-q")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "baseline")
	clean := collectHandoffState(dir)
	if clean == nil || clean.GitDirty {
		t.Fatalf("clean repository state = %+v", clean)
	}
	names := []string{"new-work.go", "café.go"}
	if runtime.GOOS != "windows" {
		names = append(names, " spaced\n.go ")
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package work\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	state := collectHandoffState(dir)
	if state == nil || !state.GitDirty {
		t.Fatalf("untracked work reported clean: %+v", state)
	}
	for _, name := range names {
		if !slices.Contains(state.ModifiedFiles, name) {
			t.Errorf("collected paths %q omit exact filename %q", state.ModifiedFiles, name)
		}
	}
	data, err := json.Marshal(handoffArtifact{SchemaVersion: 1, ID: "handoff-20260909T120000Z", CreatedAt: "2026-09-09T12:00:00Z", State: state})
	if err != nil {
		t.Fatal(err)
	}
	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		t.Fatal(err)
	}
	if err := compileHandoffSchema(t).Validate(instance); err != nil {
		t.Fatalf("collected state violates the handoff schema: %v", err)
	}
	files, err := gitChangedFiles(dir, 1)
	if err != nil || len(files) != 1 {
		t.Fatalf("bounded collection = %q, %v", files, err)
	}
}

func TestHandoffCollectUnavailableIsNotClean(t *testing.T) {
	if state := collectHandoffState(t.TempDir()); state != nil {
		t.Fatalf("non-repository produced a supposedly known Git state: %+v", state)
	}
}

func TestParseGitStatusPreservesRenameAndCopyPaths(t *testing.T) {
	raw := "R  new\nname\x00 old name \x00C  café.go\x00source.go\x00?? untracked.go\x00"
	got, err := parseGitStatus(raw)
	want := []string{"new\nname", " old name ", "café.go", "source.go", "untracked.go"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("paths = %q, %v; want %q", got, err, want)
	}
	for _, invalid := range []string{"?? unterminated", "x\x00", "??\x00", "R  target\x00", "C  target\x00\x00"} {
		if _, err := parseGitStatus(invalid); err == nil {
			t.Errorf("accepted incomplete status %q", invalid)
		}
	}
}
