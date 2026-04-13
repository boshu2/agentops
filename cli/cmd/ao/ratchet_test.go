package main

import (
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status ratchet.StepStatus
		want   string
	}{
		{"locked", ratchet.StatusLocked, "✓"},
		{"skipped", ratchet.StatusSkipped, "⊘"},
		{"in_progress", ratchet.StatusInProgress, "◐"},
		{"pending", ratchet.StatusPending, "○"},
		{"unknown status", ratchet.StepStatus("unknown"), "○"},
		{"empty status", ratchet.StepStatus(""), "○"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusIcon(tt.status)
			if got != tt.want {
				t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestRatchetHelpDescribesPhasedRPIWorkflow(t *testing.T) {
	if !strings.Contains(ratchetCmd.Long, "phased RPI workflow") {
		t.Fatalf("ratchet help missing phased RPI wording:\n%s", ratchetCmd.Long)
	}
	if strings.Contains(ratchetCmd.Long, "Research-Plan-Implement") {
		t.Fatalf("ratchet help still contains stale expanded RPI wording:\n%s", ratchetCmd.Long)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"empty string", "", 10, ""},
		{"max 3 (minimum for ellipsis)", "hello", 3, "..."},
		{"single char over", "abcdef", 5, "ab..."},
		{"long string", "this is a very long string that should be truncated", 20, "this is a very lo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}
