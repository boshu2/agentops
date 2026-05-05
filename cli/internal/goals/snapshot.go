package goals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// SnapshotSummary holds aggregate counts and weighted score.
//
// Score is the headline weighted-pass percentage: passing-weight /
// (passing-weight + failing-weight) * 100. Skipped goals (timeouts,
// dormant preconditions, exit 77) are excluded from both numerator
// and denominator.
//
// CodeDrivenScore is the same calculation restricted to goals that do
// NOT carry the `runtime-artifact` tag. Runtime-artifact goals
// (compile-freshness, compile-no-oscillation) flip every run because
// they read .agents/defrag/latest.json which is gitignored — including
// them in the headline delta inflates morning digests. Per the nightly
// routine, the *code-driven* score is the comparison number; the raw
// score and the runtime-artifact slice are tabulated separately.
type SnapshotSummary struct {
	Total                  int     `json:"total"`
	Passing                int     `json:"passing"`
	Failing                int     `json:"failing"`
	Skipped                int     `json:"skipped"`
	Score                  float64 `json:"score"`
	CodeDrivenTotal        int     `json:"code_driven_total"`
	CodeDrivenPassing      int     `json:"code_driven_passing"`
	CodeDrivenFailing      int     `json:"code_driven_failing"`
	CodeDrivenSkipped      int     `json:"code_driven_skipped"`
	CodeDrivenScore        float64 `json:"code_driven_score"`
	RuntimeArtifactTotal   int     `json:"runtime_artifact_total"`
	RuntimeArtifactPassing int     `json:"runtime_artifact_passing"`
	RuntimeArtifactFailing int     `json:"runtime_artifact_failing"`
}

// Snapshot captures a point-in-time measurement of all goals.
type Snapshot struct {
	Timestamp string          `json:"timestamp"`
	GitSHA    string          `json:"git_sha"`
	Goals     []Measurement   `json:"goals"`
	Summary   SnapshotSummary `json:"summary"`
}

// jsonMarshalIndentFn is the indented marshaler used by SaveSnapshot. Override
// in tests to simulate encoding failures.
var jsonMarshalIndentFn = json.MarshalIndent

// SaveSnapshot writes a snapshot to disk as indented JSON.
// Returns the path of the written file.
func SaveSnapshot(s *Snapshot, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("creating snapshot dir: %w", err)
	}

	ts := time.Now().UTC().Format("2006-01-02T15-04-05.000")
	filename := filepath.Join(dir, ts+".json")

	data, err := jsonMarshalIndentFn(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling snapshot: %w", err)
	}

	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return "", fmt.Errorf("writing snapshot: %w", err)
	}

	return filename, nil
}

// LoadSnapshot reads a snapshot from a JSON file.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing snapshot %s: %w", path, err)
	}

	return &s, nil
}

// LoadLatestSnapshot finds the most recent snapshot in dir by filename
// (timestamps sort lexicographically).
func LoadLatestSnapshot(dir string) (*Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}

	if len(jsonFiles) == 0 {
		return nil, fmt.Errorf("no snapshots found in %s", dir)
	}

	slices.Sort(jsonFiles)
	latest := filepath.Join(dir, jsonFiles[len(jsonFiles)-1])

	return LoadSnapshot(latest)
}
