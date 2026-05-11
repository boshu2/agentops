// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseCrankCompletion_GoldenFile verifies parseCrankCompletion against
// representative bd children output captured in testdata golden files.
// These files document the expected bd output format and serve as regression
// guards if the parsing logic changes.
func TestParseCrankCompletion_GoldenFile(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{
			name:     "partial: some closed some open",
			file:     "bd-children-output.txt",
			expected: "PARTIAL",
		},
		{
			name:     "done: all closed or checkmarked",
			file:     "bd-children-all-closed-output.txt",
			expected: "DONE",
		},
		{
			name:     "blocked: at least one blocked child",
			file:     "bd-children-blocked-output.txt",
			expected: "BLOCKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("testdata", tt.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden file %s: %v", path, err)
			}

			got := parseCrankCompletion(string(data))
			if got != tt.expected {
				t.Errorf("parseCrankCompletion(%s) = %q, want %q", tt.file, got, tt.expected)
			}
		})
	}
}

// TestExtractEpicID_GoldenFile verifies parseLatestEpicIDFromText against
// a representative bd list --type epic text output captured in a golden file.
// extractEpicID itself shells out to bd, so we test the text parsing helper
// directly, which is what extractEpicID delegates to on the text fallback path.
func TestExtractEpicID_GoldenFile(t *testing.T) {
	path := filepath.Join("testdata", "bd-list-epic-output.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}

	// The golden file contains three epics; parseLatestEpicIDFromText should
	// return the LAST matching epic ID (bd-17) since extractEpicID picks the
	// most recently created open epic.
	got, err := parseLatestEpicIDFromText(string(data))
	if err != nil {
		t.Fatalf("parseLatestEpicIDFromText: %v", err)
	}
	const wantEpicID = "bd-17"
	if got != wantEpicID {
		t.Errorf("parseLatestEpicIDFromText = %q, want %q", got, wantEpicID)
	}
}
