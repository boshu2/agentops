package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesEvidenceStorageWithoutGitMutation(t *testing.T) {
	dir := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var output bytes.Buffer
	command := *initCmd
	command.SetOut(&output)
	if err := runInit(&command, nil); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		filepath.Join(".agentops", "verdicts", "sha256"),
		filepath.Join(".agents", "ao", "provenance"),
		filepath.Join(".agents", "handoff"),
	} {
		if info, err := os.Stat(filepath.Join(dir, relative)); err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("init must not edit Git ignore state")
	}
}
