package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionBootstrapOnlyReportsLocalOrientation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	var output bytes.Buffer
	command := *sessionBootstrapCmd
	command.SetOut(&output)
	sessionBootstrapJSON = true
	t.Cleanup(func() { sessionBootstrapJSON = false })
	if err := runSessionBootstrap(&command, nil); err != nil {
		t.Fatal(err)
	}
	var status sessionBootstrapStatus
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.OrientationFiles) != 1 || status.OrientationFiles[0] != "AGENTS.md" {
		t.Fatalf("unexpected orientation files: %#v", status.OrientationFiles)
	}
}

func TestHandoffAndRehydratePreserveCallerTextWithoutLifecycleState(t *testing.T) {
	dir := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	handoffGoal = "prove one behavior"
	handoffContinuation = "caller will choose whether to revise"
	handoffCollect = false
	handoffDryRun = false
	t.Cleanup(func() {
		handoffGoal, handoffContinuation = "", ""
		handoffCollect, handoffDryRun = false, false
	})
	var written bytes.Buffer
	writeCommand := *handoffCmd
	writeCommand.SetOut(&written)
	if err := runHandoff(&writeCommand, []string{"candidate failed validation"}); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".agents", "handoff"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one handoff: entries=%d err=%v", len(entries), err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".agents", "handoff", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"claim", "reservation", "retry", "consumed", "next_work", "rpi_phase"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("handoff leaked lifecycle field %q: %s", forbidden, data)
		}
	}

	var restored bytes.Buffer
	readCommand := *rehydrateCmd
	readCommand.SetOut(&restored)
	rehydrateJSON = false
	if err := runRehydrate(&readCommand, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.String(), handoffContinuation) {
		t.Fatalf("caller continuation missing: %s", restored.String())
	}
	if after, err := os.ReadFile(filepath.Join(dir, ".agents", "handoff", entries[0].Name())); err != nil || !bytes.Equal(data, after) {
		t.Fatal("rehydrate mutated the handoff artifact")
	}
}
