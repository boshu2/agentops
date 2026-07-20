package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandoffPreservesCallerTextWithoutLifecycleState proves the `ao session
// handoff` writer records the caller-authored text and no lifecycle state. The
// bootstrap and rehydrate read sides moved to internal/commands/session with
// the session carve-out; handoff itself (the writer) still lives in package
// main and shares the session parent attached by newSessionCommand.
func TestHandoffPreservesCallerTextWithoutLifecycleState(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

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
	if !strings.Contains(string(data), "caller will choose whether to revise") {
		t.Fatalf("handoff dropped caller continuation: %s", data)
	}
	for _, forbidden := range []string{"claim", "reservation", "retry", "consumed", "next_work", "rpi_phase"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("handoff leaked lifecycle field %q: %s", forbidden, data)
		}
	}
}
