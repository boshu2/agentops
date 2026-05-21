// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEvolveWriteStopMarker_Table covers the mechanical no-self-stop contract
// for `ao evolve write-stop-marker` introduced by soc-hwax. Burst mode must
// write the marker file (canonicalized to upper-case); loop mode must refuse
// every marker variant with a stderr message referencing 'operator-stop'.
func TestEvolveWriteStopMarker_Table(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		marker      string
		reason      string
		wantErr     bool
		wantErrSub  string
		wantFile    string
		wantFileBody string
	}{
		{
			name:         "burst dormant writes file",
			mode:         "burst",
			marker:       "dormant",
			reason:       "queue stable",
			wantErr:      false,
			wantFile:     "DORMANT",
			wantFileBody: "queue stable",
		},
		{
			name:        "loop dormant refused",
			mode:        "loop",
			marker:      "dormant",
			reason:      "agent self-halt attempt",
			wantErr:     true,
			wantErrSub:  "refused under --mode=loop",
		},
		{
			name:        "loop stop refused",
			mode:        "loop",
			marker:      "stop",
			reason:      "operator override",
			wantErr:     true,
			wantErrSub:  "refused under --mode=loop",
		},
		{
			name:        "loop kill refused",
			mode:        "loop",
			marker:      "kill",
			reason:      "kill switch",
			wantErr:     true,
			wantErrSub:  "refused under --mode=loop",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := chdirTemp(t)

			out, err := executeCommand(
				"evolve", "write-stop-marker",
				"--mode", tc.mode,
				"--marker", tc.marker,
				"--reason", tc.reason,
			)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (output=%q)", out)
				}
				combined := err.Error() + "\n" + out
				if !strings.Contains(combined, tc.wantErrSub) {
					t.Fatalf("error/output missing %q:\nerr=%v\nout=%s", tc.wantErrSub, err, out)
				}
				// Verify no marker file was written under refusal.
				markerDir := filepath.Join(dir, ".agents", "evolve")
				entries, statErr := os.ReadDir(markerDir)
				if statErr == nil {
					for _, entry := range entries {
						name := entry.Name()
						if name == "DORMANT" || name == "STOP" || name == "KILL" {
							t.Fatalf("loop mode wrote marker file %s; should have refused", name)
						}
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v\noutput: %s", err, out)
			}
			path := filepath.Join(dir, ".agents", "evolve", tc.wantFile)
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read marker file %s: %v", path, readErr)
			}
			if string(data) != tc.wantFileBody {
				t.Fatalf("marker body = %q, want %q", string(data), tc.wantFileBody)
			}
		})
	}
}

// TestEvolveWriteStopMarker_RegisteredOnEvolve confirms the subcommand is
// reachable via `ao evolve write-stop-marker` (catches accidental removal of
// the AddCommand call in init).
func TestEvolveWriteStopMarker_RegisteredOnEvolve(t *testing.T) {
	var found bool
	for _, sub := range evolveCmd.Commands() {
		if sub.Name() == "write-stop-marker" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evolve write-stop-marker subcommand should be registered on evolveCmd")
	}
}

// TestNormalizeStopMarkerName covers the marker-name canonicalization helper
// directly so the table test above doesn't need to enumerate every value.
func TestNormalizeStopMarkerName(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"dormant", "DORMANT", false},
		{"stop", "STOP", false},
		{"kill", "KILL", false},
		{"", "", true},
		{"DORMANT", "", true}, // canonical input is lowercase
		{"bogus", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeStopMarkerName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeStopMarkerName(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeStopMarkerName(%q) error = %v, want nil", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeStopMarkerName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
