// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"
)

// --- deriveRepoRootFromRPIOrchestrationLog ---

func TestDeriveRepoRoot_ValidLogPath(t *testing.T) {
	root, ok := deriveRepoRootFromRPIOrchestrationLog("/home/user/project/.agents/rpi/phased-orchestration.log")
	if !ok {
		t.Fatal("expected ok=true for valid path")
	}
	if root != "/home/user/project" {
		t.Errorf("root = %q, want %q", root, "/home/user/project")
	}
}

func TestDeriveRepoRoot_InvalidLogPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "not in rpi dir", path: "/home/user/.agents/logs/log.txt"},
		{name: "not in .agents", path: "/home/user/rpi/log.txt"},
		{name: "empty", path: ""},
		{name: "just rpi", path: "/rpi/log.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := deriveRepoRootFromRPIOrchestrationLog(tc.path)
			if ok {
				t.Errorf("expected ok=false for path %q", tc.path)
			}
		})
	}
}

// --- ledgerActionFromDetails ---

func TestLedgerActionFromDetails_Prefixes(t *testing.T) {
	tests := []struct {
		details string
		want    string
	}{
		{"started", "started"},
		{"completed in 5s", "completed"},
		{"failed: phase error", "failed"},
		{"FATAL: crash", "fatal"},
		{"retry attempt 2/3", "retry"},
		{"dry-run complete", "dry-run"},
		{"HANDOFF detected", "handoff"},
		{"epic=ag-1 verdicts=map[]", "summary"},
		{"", "event"},
		{"some random text", "some"},
	}
	for _, tc := range tests {
		t.Run(tc.details, func(t *testing.T) {
			got := ledgerActionFromDetails(tc.details)
			if got != tc.want {
				t.Errorf("ledgerActionFromDetails(%q) = %q, want %q", tc.details, got, tc.want)
			}
		})
	}
}
