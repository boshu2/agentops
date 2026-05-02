package evalsubstrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ManifestPath(evalsRoot, runID string) string {
	return filepath.Join(evalsRoot, "runs", runID, "manifest.json")
}

func RunDir(evalsRoot, runID string) string {
	return filepath.Join(evalsRoot, "runs", runID)
}

// RunWriter manages the §4 lifecycle of a single Run manifest.
// All mutations go through WriteAtomic — never plain os.WriteFile.
type RunWriter struct {
	root     string
	runID    string
	manifest *Manifest
}

func NewRunWriter(root, runID string, m Manifest) (*RunWriter, error) {
	if root == "" {
		return nil, errors.New("NewRunWriter: empty root")
	}
	if runID == "" {
		return nil, errors.New("NewRunWriter: empty runID")
	}
	dir := RunDir(root, runID)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("NewRunWriter: run dir already exists: %s", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("NewRunWriter: mkdir: %w", err)
	}

	now := timeNow().UTC()
	if m.SchemaVersion == 0 {
		m.SchemaVersion = SchemaVersion
	}
	m.ID = runID
	m.Status = StatusPending
	if m.StartedAt == "" {
		m.StartedAt = now.Format(time.RFC3339)
	}
	if m.StartedAtUnixMs == 0 {
		m.StartedAtUnixMs = now.UnixNano() / int64(time.Millisecond)
	}
	if m.Kind == "" {
		m.Kind = "task"
	}
	if m.CapturedBy == "" {
		m.CapturedBy = "ao eval task run"
	}
	if m.Seeds == nil {
		m.Seeds = []int{}
	}
	if m.ValidityGatesPassed == nil {
		m.ValidityGatesPassed = []string{}
	}

	w := &RunWriter{root: root, runID: runID, manifest: &m}
	if err := w.flush(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RunWriter) Manifest() Manifest { return *w.manifest }
func (w *RunWriter) Path() string       { return ManifestPath(w.root, w.runID) }
func (w *RunWriter) Dir() string        { return RunDir(w.root, w.runID) }

func (w *RunWriter) Transition(next RunStatus, mut func(*Manifest)) error {
	if !legalTransition(w.manifest.Status, next) {
		return fmt.Errorf("illegal transition: %s -> %s", w.manifest.Status, next)
	}
	w.manifest.Status = next
	if mut != nil {
		mut(w.manifest)
	}
	switch next {
	case StatusComplete, StatusFailed, StatusAborted:
		if w.manifest.FinishedAtUnixMs == 0 {
			now := timeNow().UTC()
			w.manifest.FinishedAt = now.Format(time.RFC3339)
			w.manifest.FinishedAtUnixMs = now.UnixNano() / int64(time.Millisecond)
		}
	case StatusRetracted:
		ts := timeNow().UTC().UnixNano() / int64(time.Millisecond)
		w.manifest.RetractedAtUnixMs = &ts
	}
	return w.flush()
}

func (w *RunWriter) flush() error {
	data, err := json.MarshalIndent(w.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("RunWriter: marshal: %w", err)
	}
	data = append(data, '\n')
	return WriteAtomic(w.Path(), data)
}

func LoadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadManifest %q: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("LoadManifest %q: parse: %w", path, err)
	}
	return &m, nil
}

// legalTransition encodes the §5 Run state machine.
//
//	pending  -> running | aborted | failed | retracted
//	running  -> complete | failed | aborted | retracted
//	complete -> retracted
//	failed   -> retracted
//	aborted  -> retracted
//	retracted (terminal)
func legalTransition(cur, next RunStatus) bool {
	allowed := map[RunStatus]map[RunStatus]bool{
		StatusPending: {
			StatusRunning:   true,
			StatusFailed:    true,
			StatusAborted:   true,
			StatusRetracted: true,
		},
		StatusRunning: {
			StatusComplete:  true,
			StatusFailed:    true,
			StatusAborted:   true,
			StatusRetracted: true,
		},
		StatusComplete:  {StatusRetracted: true},
		StatusFailed:    {StatusRetracted: true},
		StatusAborted:   {StatusRetracted: true},
		StatusRetracted: {},
	}
	return allowed[cur][next]
}

var RequiredManifestFields = []string{
	"schema_version", "id", "status",
	"started_at_unix_ms",
	"task_ref", "harness_ref", "harness_content_hash",
	"model_spec_ref", "model_spec_hash",
	"ground_truth_ref", "ground_truth_hash",
	"sample_split", "n_samples", "seeds",
	"inspect_command", "inspect_version",
	"validity_gates_passed",
	"rig_id",
}

// ValidateForComplete returns the list of missing rc2-required fields
// that would prevent a `complete` Run from being authoritative per §4.
func ValidateForComplete(m *Manifest) []string {
	checks := []struct {
		name   string
		absent bool
	}{
		{"schema_version", m.SchemaVersion == 0},
		{"id", m.ID == ""},
		{"status", m.Status == ""},
		{"started_at_unix_ms", m.StartedAtUnixMs == 0},
		{"task_ref", m.TaskRef == ""},
		{"harness_ref", m.HarnessRef == ""},
		{"harness_content_hash", m.HarnessContentHash == ""},
		{"model_spec_ref", m.ModelSpecRef == ""},
		{"model_spec_hash", m.ModelSpecHash == ""},
		{"ground_truth_ref", m.GroundTruthRef == ""},
		{"ground_truth_hash", m.GroundTruthHash == ""},
		{"sample_split", m.SampleSplit == ""},
		{"n_samples", m.NSamples == 0},
		{"seeds(>=3)", len(m.Seeds) < 3},
		{"inspect_command", m.InspectCommand == ""},
		{"inspect_version", m.InspectVersion == ""},
		{"rig_id", m.RigID == ""},
		{"validity_gates_passed", m.ValidityGatesPassed == nil},
	}
	var missing []string
	for _, c := range checks {
		if c.absent {
			missing = append(missing, c.name)
		}
	}
	return missing
}

// GenerateRunID returns a deterministic Run ID of the form
// run-YYYY-MM-DD-HHMM-<rig8>-<rand6>.
func GenerateRunID(rigID string) string {
	now := timeNow().UTC()
	rig := rigID
	if rig == "" {
		rig = "unknown"
	}
	rig = strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(rig)
	if len(rig) > 8 {
		rig = rig[:8]
	}
	return fmt.Sprintf("run-%s-%s-%s",
		now.Format("2006-01-02-1504"),
		rig,
		randomSuffix(6),
	)
}
