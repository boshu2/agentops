package main

import (
	"strings"
	"testing"
)

func TestParseOmnigentVerdict(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		wantStatus string
		wantBranch string
	}{
		{
			name:       "worthy sentinel with branch",
			stdout:     "Zeus report\nVERDICT: WORTHY branch=feat/olympus\n",
			wantStatus: "WORTHY",
			wantBranch: "feat/olympus",
		},
		{
			name:       "unworthy sentinel with branch",
			stdout:     "review notes\nVERDICT: UNWORTHY branch=fix/reject\n",
			wantStatus: "UNWORTHY",
			wantBranch: "fix/reject",
		},
		{
			name:       "fallback prose worthy",
			stdout:     "After review, the completed branch is WORTHY of landing.",
			wantStatus: "WORTHY",
		},
		{
			name:       "unknown without sentinel or prose",
			stdout:     "Zeus ended before producing a machine-readable final verdict.",
			wantStatus: "UNKNOWN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOmnigentVerdict(tc.stdout)
			if got.Status != tc.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Branch != tc.wantBranch {
				t.Fatalf("Branch = %q, want %q", got.Branch, tc.wantBranch)
			}
		})
	}
}

func TestOmnigentDispatchHelp(t *testing.T) {
	out, err := executeCommand("omnigent", "dispatch", "--help")
	if err != nil {
		t.Fatalf("ao omnigent dispatch --help returned error: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"--bundle", "--task", "--timeout-seconds", "--receipt", "--packet"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ao omnigent dispatch --help missing %q\noutput:\n%s", want, out)
		}
	}
}
