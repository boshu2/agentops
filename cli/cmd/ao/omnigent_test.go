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
			// Fail-closed: a bare "WORTHY" in prose without the sentinel is NOT a
			// pass (a model can write "worthy of more review"). Stays UNKNOWN.
			name:       "fallback prose worthy is NOT trusted (fail-closed)",
			stdout:     "After review, the completed branch is WORTHY of landing.",
			wantStatus: "UNKNOWN",
		},
		{
			// Fail-closed still honors a clear UNWORTHY in prose — failing is safe.
			name:       "fallback prose unworthy still fails",
			stdout:     "The diff is UNWORTHY; blocking issues remain.",
			wantStatus: "UNWORTHY",
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

// TestParseOmnigentVerdict_LastSentinelWins pins the fail-CLOSED gate: when stdout
// contains multiple VERDICT: sentinels, the LAST one is authoritative for Status,
// Branch and Reason. An earlier stray WORTHY (the agent quoting its instructions or
// an embedded sub-agent report) must NOT mask a real trailing UNWORTHY/BLOCKED.
func TestParseOmnigentVerdict_LastSentinelWins(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		wantStatus string
		wantBranch string
		wantReason string
	}{
		{
			name:       "trailing unworthy overrides earlier worthy",
			stdout:     "intro VERDICT: WORTHY branch=feat/early\nmore prose\nVERDICT: UNWORTHY branch=fix/real\n",
			wantStatus: "UNWORTHY",
			wantBranch: "fix/real",
		},
		{
			name:       "trailing blocked with reason overrides earlier worthy",
			stdout:     "VERDICT: WORTHY branch=feat/early\nlogs...\nVERDICT: BLOCKED reason=protected branch push denied\n",
			wantStatus: "BLOCKED",
			wantBranch: "",
			wantReason: "protected branch push denied",
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
			if got.Reason != tc.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tc.wantReason)
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
