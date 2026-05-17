package scenarioresults

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Writer appends scenario results to the runtime artifact without clobbering
// existing RPI evaluator artifacts. It writes atomically (tmp + rename).
type Writer struct {
	// Now supplies the generated_at timestamp; defaults to time.Now when nil.
	Now func() time.Time
}

// nowUTC returns the writer's clock in UTC, defaulting to the wall clock.
func (w Writer) nowUTC() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

// Append merges newResults into the artifact at ArtifactRelPath under projectRoot.
// A duplicate scenario_id is resolved by latest judged_at. The artifact's
// run_id and iteration are updated to runID/iteration. Other RPI artifacts in
// the same directory are untouched. The merged Artifact is returned.
func (w Writer) Append(projectRoot, runID string, iteration int, newResults []ScenarioResult) (*Artifact, error) {
	path := filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))

	existing, err := w.readExisting(path)
	if err != nil {
		return nil, err
	}

	merged := mergeResults(existing, newResults)
	artifact := &Artifact{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		Iteration:     iteration,
		GeneratedAt:   w.nowUTC().Format(time.RFC3339),
		Results:       merged,
	}

	if err := writeArtifact(path, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

// readExisting loads the prior results slice, returning an empty slice when the
// artifact is absent. A malformed artifact is a hard error here so a writer
// never silently discards prior results.
func (w Writer) readExisting(path string) ([]ScenarioResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read existing scenario results %s: %w", path, err)
	}
	var prior Artifact
	if err := json.Unmarshal(data, &prior); err != nil {
		return nil, fmt.Errorf("parse existing scenario results %s: %w", path, err)
	}
	return prior.Results, nil
}

// mergeResults combines existing and incoming results, keeping the latest
// judged_at per scenario_id. The output is sorted by scenario_id for
// deterministic serialization.
func mergeResults(existing, incoming []ScenarioResult) []ScenarioResult {
	byID := make(map[string]ScenarioResult)
	for _, r := range existing {
		keepLatest(byID, r)
	}
	for _, r := range incoming {
		keepLatest(byID, r)
	}

	out := make([]ScenarioResult, 0, len(byID))
	for _, r := range byID {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ScenarioID < out[j].ScenarioID
	})
	return out
}

// keepLatest inserts r into byID unless an entry with a later judged_at exists.
func keepLatest(byID map[string]ScenarioResult, r ScenarioResult) {
	prev, ok := byID[r.ScenarioID]
	if !ok || resultIsNewer(r, prev) {
		byID[r.ScenarioID] = r
	}
}

// resultIsNewer reports whether candidate judged at-or-after current. Ties and
// unparseable timestamps favor the candidate (last write wins).
func resultIsNewer(candidate, current ScenarioResult) bool {
	ct, cerr := time.Parse(time.RFC3339, candidate.JudgedAt)
	pt, perr := time.Parse(time.RFC3339, current.JudgedAt)
	if cerr != nil || perr != nil {
		return true
	}
	return !ct.Before(pt)
}

// writeArtifact serializes artifact to path atomically via tmp + rename.
func writeArtifact(path string, artifact *Artifact) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create scenario results directory: %w", err)
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scenario results: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write scenario results tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename scenario results artifact: %w", err)
	}
	return nil
}
