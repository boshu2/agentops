// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeClaude(t *testing.T, script string) string {
	t.Helper()

	binDir := t.TempDir()
	path := filepath.Join(binDir, "claude")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	return binDir
}

func TestSpawnClaudeDirectImpl_TimesOut(t *testing.T) {
	binDir := writeFakeClaude(t, "#!/bin/sh\nsleep 5\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	err := spawnClaudeDirectImpl("test prompt", t.TempDir(), 2, 150*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestSpawnClaudePhaseWithStream_TimesOut(t *testing.T) {
	binDir := writeFakeClaude(t, "#!/bin/sh\necho '{\"type\":\"init\",\"session_id\":\"s1\",\"model\":\"m\"}'\nsleep 5\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "live-status.md")
	allPhases := []PhaseProgress{{Name: "discovery", CurrentAction: "starting"}}

	startedAt := time.Now()
	err := spawnClaudePhaseWithStream("test prompt", tmpDir, "run-1", 1, statusPath, allPhases, 200*time.Millisecond, 0, 0, 30*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("expected timeout to return promptly, took %s", elapsed)
	}
}

func TestSpawnClaudePhaseWithStream_StallDetected(t *testing.T) {
	// Fake claude: emit one init event then hang — no further activity.
	binDir := writeFakeClaude(t, "#!/bin/sh\necho '{\"type\":\"init\",\"session_id\":\"s1\",\"model\":\"m\"}'\nsleep 10\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "live-status.md")
	allPhases := []PhaseProgress{{Name: "discovery", CurrentAction: "starting"}}

	err := spawnClaudePhaseWithStream("test prompt", tmpDir, "run-stall", 1, statusPath, allPhases, 0, 100*time.Millisecond, 0, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected stall error")
	}
	if !strings.Contains(err.Error(), string(failReasonStall)) {
		t.Fatalf("expected stall failure reason in error, got: %v", err)
	}
}

func TestSpawnClaudePhaseWithStream_StartupTimeout(t *testing.T) {
	// Fake claude: no output, just hang.
	binDir := writeFakeClaude(t, "#!/bin/sh\nsleep 10\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "live-status.md")
	allPhases := []PhaseProgress{{Name: "discovery", CurrentAction: "starting"}}

	err := spawnClaudePhaseWithStream("test prompt", tmpDir, "run-startup-timeout", 1, statusPath, allPhases, 0, 0, 120*time.Millisecond, 40*time.Millisecond)
	if err == nil {
		t.Fatal("expected startup timeout error")
	}
	if !strings.Contains(err.Error(), "stream startup timeout") {
		t.Fatalf("expected startup timeout in error, got: %v", err)
	}
}

func TestStallTimeoutClassification(t *testing.T) {
	if failReasonTimeout == failReasonStall {
		t.Error("failReasonTimeout and failReasonStall must be distinct")
	}
	if failReasonTimeout == failReasonExit {
		t.Error("failReasonTimeout and failReasonExit must be distinct")
	}
	if failReasonStall == failReasonExit {
		t.Error("failReasonStall and failReasonExit must be distinct")
	}

	timeoutMsg := string(failReasonTimeout)
	stallMsg := string(failReasonStall)

	exampleTimeout := "phase 1 (timeout) timed out after 30m0s"
	if !strings.Contains(exampleTimeout, timeoutMsg) {
		t.Errorf("expected %q in timeout error format %q", timeoutMsg, exampleTimeout)
	}
	exampleStall := "phase 1 (stall): stall detected: no stream activity for 5m0s"
	if !strings.Contains(exampleStall, stallMsg) {
		t.Errorf("expected %q in stall error format %q", stallMsg, exampleStall)
	}
}
