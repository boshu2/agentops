package scenarioresults

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Status classifies the outcome of loading the scenario results artifact.
type Status string

const (
	// StatusOK means the artifact loaded and validated cleanly.
	StatusOK Status = "ok"
	// StatusUnknown means the artifact is absent — treated as skip, never a
	// false pass.
	StatusUnknown Status = "unknown"
	// StatusMalformed means the artifact exists but failed to parse or validate.
	StatusMalformed Status = "malformed"
)

// LoadResult is the typed outcome of a Load call.
type LoadResult struct {
	// Status classifies the load outcome.
	Status Status
	// Artifact is the parsed document; nil when Status is StatusUnknown or
	// StatusMalformed.
	Artifact *Artifact
	// Warning is a non-fatal message (e.g. malformed artifact in non-strict
	// mode, or a missing artifact).
	Warning string
}

// IsSkip reports whether the load outcome should be treated as a skip/unknown
// signal — i.e. the caller has no satisfaction evidence and must not infer a
// pass.
func (r LoadResult) IsSkip() bool {
	return r.Status != StatusOK
}

// Load reads the scenario results artifact at ArtifactRelPath under projectRoot.
//
//   - Missing artifact: returns StatusUnknown with a warning, never an error.
//   - Malformed artifact: in strict mode returns a path-specific error; in
//     non-strict mode returns StatusMalformed with a warning and no error.
//   - Valid artifact: returns StatusOK with results sorted by scenario_id.
func Load(projectRoot string, strict bool) (LoadResult, error) {
	path := filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LoadResult{
				Status:  StatusUnknown,
				Warning: fmt.Sprintf("scenario results artifact not found at %s; treating as skip", ArtifactRelPath),
			}, nil
		}
		return malformed(path, strict, fmt.Errorf("read scenario results %s: %w", path, err))
	}

	artifact, defect := parseAndValidate(path, data)
	if defect != nil {
		return malformed(path, strict, defect)
	}
	return LoadResult{Status: StatusOK, Artifact: artifact}, nil
}

// parseAndValidate unmarshals data and applies structural validation, returning
// a path-specific error on the first defect.
func parseAndValidate(path string, data []byte) (*Artifact, error) {
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("parse scenario results %s: %w", path, err)
	}
	if artifact.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf(
			"scenario results %s: schema_version %q != %q",
			path, artifact.SchemaVersion, SchemaVersion)
	}
	for i, r := range artifact.Results {
		if defect := validateResult(r); defect != "" {
			return nil, fmt.Errorf("scenario results %s: results[%d] %s", path, i, defect)
		}
	}
	artifact.Results = dedupeLatest(artifact.Results)
	return &artifact, nil
}

// dedupeLatest collapses duplicate scenario_id entries to the latest judged_at,
// returning a slice sorted by scenario_id.
func dedupeLatest(results []ScenarioResult) []ScenarioResult {
	byID := make(map[string]ScenarioResult)
	for _, r := range results {
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

// malformed builds the LoadResult/error pair for a malformed artifact: a
// path-specific error in strict mode, a warning otherwise.
func malformed(path string, strict bool, defect error) (LoadResult, error) {
	if strict {
		return LoadResult{Status: StatusMalformed}, defect
	}
	return LoadResult{
		Status:  StatusMalformed,
		Warning: fmt.Sprintf("malformed scenario results (%s): %v", filepath.ToSlash(path), defect),
	}, nil
}
