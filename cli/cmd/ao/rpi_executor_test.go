package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutorInterface(t *testing.T) {
	var _ PhaseExecutor = &directExecutor{}
	var _ PhaseExecutor = &streamExecutor{statusPath: "/tmp/status.md", allPhases: nil}

	tests := []struct {
		exec PhaseExecutor
		want string
	}{
		{&directExecutor{}, "direct"},
		{&streamExecutor{}, "stream"},
	}
	for _, tt := range tests {
		if got := tt.exec.Name(); got != tt.want {
			t.Errorf("Name() = %q, want %q", got, tt.want)
		}
	}
}

func TestBackendSelectionAutoDirectWhenLiveStatusDisabled(t *testing.T) {
	caps := backendCapabilities{LiveStatusEnabled: false, RuntimeMode: "auto"}
	exec, reason := selectExecutorFromCaps(caps, "", nil, defaultPhasedEngineOptions())
	if exec.Name() != "direct" {
		t.Errorf("expected direct executor, got %q", exec.Name())
	}
	if !strings.Contains(reason, "live-status disabled") {
		t.Errorf("reason should explain auto/direct choice, got %q", reason)
	}
}

func TestBackendSelectionAutoStreamWhenLiveStatusEnabled(t *testing.T) {
	caps := backendCapabilities{LiveStatusEnabled: true, RuntimeMode: "auto"}
	exec, reason := selectExecutorFromCaps(caps, "/tmp/status.md", nil, defaultPhasedEngineOptions())
	if exec.Name() != "stream" {
		t.Errorf("expected stream executor, got %q", exec.Name())
	}
	if !strings.Contains(reason, "live-status enabled") {
		t.Errorf("reason should mention live-status, got %q", reason)
	}
}

func TestBackendSelectionForcedRuntimeModes(t *testing.T) {
	tests := []struct {
		name        string
		caps        backendCapabilities
		wantBackend string
		wantReason  string
	}{
		{
			name:        "runtime stream",
			caps:        backendCapabilities{LiveStatusEnabled: false, RuntimeMode: "stream"},
			wantBackend: "stream",
			wantReason:  "runtime=stream",
		},
		{
			name:        "runtime direct",
			caps:        backendCapabilities{LiveStatusEnabled: true, RuntimeMode: "direct"},
			wantBackend: "direct",
			wantReason:  "runtime=direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, reason := selectExecutorFromCaps(tt.caps, "/tmp/status.md", nil, defaultPhasedEngineOptions())
			if exec.Name() != tt.wantBackend {
				t.Errorf("backend = %q, want %q", exec.Name(), tt.wantBackend)
			}
			if !strings.Contains(reason, tt.wantReason) {
				t.Errorf("reason = %q, want substring %q", reason, tt.wantReason)
			}
		})
	}
}

func TestProbeBackendCapabilities_NormalizesRuntimeMode(t *testing.T) {
	caps := probeBackendCapabilities(true, " STREAM ")
	if !caps.LiveStatusEnabled {
		t.Fatal("LiveStatusEnabled should be true")
	}
	if caps.RuntimeMode != "stream" {
		t.Fatalf("RuntimeMode = %q, want stream", caps.RuntimeMode)
	}

	caps = probeBackendCapabilities(false, "")
	if caps.RuntimeMode != "auto" {
		t.Fatalf("empty runtime mode should normalize to auto, got %q", caps.RuntimeMode)
	}
}

func TestSelectExecutorWithLog_LogsSelection(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	opts := defaultPhasedEngineOptions()
	opts.RuntimeMode = "direct"
	opts.RuntimeCommand = "codex"
	exec := selectExecutorWithLog("", nil, logPath, "test-run-id", false, opts)
	if exec.Name() != "direct" {
		t.Errorf("expected direct, got %q", exec.Name())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not written: %v", err)
	}
	logContent := string(data)
	if !strings.Contains(logContent, "backend-selection") {
		t.Errorf("log should contain backend-selection entry, got: %q", logContent)
	}
	if !strings.Contains(logContent, "backend=direct") {
		t.Errorf("log should record selected backend (direct), got: %q", logContent)
	}
}

func TestSelectExecutorWithLog_NoLogPath(t *testing.T) {
	exec := selectExecutorWithLog("", nil, "", "", false, defaultPhasedEngineOptions())
	if exec == nil {
		t.Fatal("executor should not be nil")
	}
}

func TestExecutorUsesConfiguredRuntimeCommand(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.RuntimeMode = "direct"
	opts.RuntimeCommand = "codex"
	exec, _ := selectExecutorFromCaps(backendCapabilities{LiveStatusEnabled: false, RuntimeMode: "direct"}, "", nil, opts)

	direct, ok := exec.(*directExecutor)
	if !ok {
		t.Fatalf("expected direct executor type, got %T", exec)
	}
	if direct.runtimeCommand != "codex" {
		t.Fatalf("runtimeCommand = %q, want codex", direct.runtimeCommand)
	}
}
