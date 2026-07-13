package search

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// ConstraintIndex represents the .agents/constraints/index.json schema.
type ConstraintIndex struct {
	SchemaVersion int               `json:"schema_version"`
	Constraints   []ConstraintEntry `json:"constraints"`
}

// ConstraintAppliesTo encodes scope filters for a constraint.
type ConstraintAppliesTo struct {
	Scope      string   `json:"scope,omitempty"`
	IssueTypes []string `json:"issue_types,omitempty"`
	PathGlobs  []string `json:"path_globs,omitempty"`
	Languages  []string `json:"languages,omitempty"`
}

// ConstraintDetector encodes how a constraint is detected.
type ConstraintDetector struct {
	Kind      string `json:"kind,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Exclude   string `json:"exclude,omitempty"`
	Companion string `json:"companion,omitempty"`
	Command   string `json:"command,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ConstraintEvidence is the safe, content-free proof projection that allowed a
// detector to enter shadow mode and may later authorize blocking activation.
type ConstraintEvidence struct {
	PositiveRefs         []string `json:"positive_refs"`
	NegativeControlRefs  []string `json:"negative_control_refs"`
	PrecisionEvidenceRef string   `json:"precision_evidence_ref,omitempty"`
	ShadowSamples        int      `json:"shadow_samples,omitempty"`
	TruePositives        int      `json:"true_positives,omitempty"`
	FalsePositives       int      `json:"false_positives,omitempty"`
}

// ConstraintEntry represents a single compiled constraint.
type ConstraintEntry struct {
	ID              string              `json:"id"`
	FindingID       string              `json:"finding_id,omitempty"`
	Title           string              `json:"title"`
	Source          string              `json:"source"`
	SourceArtifact  string              `json:"source_artifact,omitempty"`
	SourceType      string              `json:"source_type,omitempty"`
	CompilerTargets []string            `json:"compiler_targets,omitempty"`
	Detectability   string              `json:"detectability,omitempty"`
	Status          string              `json:"status"`
	EnforcementMode string              `json:"enforcement_mode,omitempty"`
	CompiledAt      string              `json:"compiled_at"`
	ReviewFile      string              `json:"review_file,omitempty"`
	AppliesTo       ConstraintAppliesTo `json:"applies_to,omitempty"`
	Detector        ConstraintDetector  `json:"detector,omitempty"`
	Evidence        ConstraintEvidence  `json:"evidence,omitempty"`
	File            string              `json:"file"`
}

// ConstraintIndexPath returns the canonical path to the index file.
func ConstraintIndexPath() string {
	return filepath.Join(".agents", "constraints", "index.json")
}

// ConstraintLockPath returns the canonical path to the compile lock file.
func ConstraintLockPath() string {
	return filepath.Join(".agents", "constraints", "compile.lock")
}

// PublishedConstraintIndexRelPath returns the canonical path to the TRACKED,
// committed constraint surface (relative to the repo root). ConstraintIndexPath
// lives under the gitignored .agents/, so a clean CI checkout / fresh clone
// enforces nothing; the published surface travels with the repo so a constraint
// learned on one box hardens it for everyone. It carries ONLY the enforceable
// detector surface (see SanitizeForPublish) — no private findings/evidence. (EM.2.9)
func PublishedConstraintIndexRelPath() string {
	return filepath.Join("docs", "constraints", "published.json")
}

// SanitizeForPublish returns a published projection of e built from an ALLOWLIST
// of only the enforceable fields — NOT a denylist that strips known-private ones.
// An allowlist is the robust guarantee: any field NOT listed here (the private
// finding_id + the .agents/-pointing source_artifact/review_file/file, AND any
// field added in the future) is dropped by construction, so it can never leak
// private findings/evidence into the tracked surface. The detector + applies_to +
// status is all the enforcement gate needs. A residual private path that somehow
// rode along in a KEPT field (Title/Message/a glob) is caught by PublishedLeaks at
// write time (defense in depth). (EM.2.9)
func SanitizeForPublish(e ConstraintEntry) ConstraintEntry {
	return ConstraintEntry{
		ID:              e.ID,
		Title:           e.Title,
		Source:          e.Source,
		SourceType:      e.SourceType,
		CompilerTargets: e.CompilerTargets,
		Detectability:   e.Detectability,
		Status:          e.Status,
		EnforcementMode: e.EnforcementMode,
		CompiledAt:      e.CompiledAt,
		AppliesTo:       e.AppliesTo,
		Detector:        e.Detector,
	}
}

// PublishedLeaks returns the ids of constraints in idx whose serialized form still
// contains the private ".agents" runtime marker (a residual leak in a kept field
// such as a detector message or a path glob). It is a DEFENSE-IN-DEPTH backstop
// behind SanitizeForPublish's allowlist (which is the real guarantee: the private
// path-bearing fields are dropped by construction). The marker is matched on the
// bare segment ".agents" — NOT a specific separator — so it catches every variant
// (forward slash, backslash, or a bare reference) rather than chasing each one.
// publish refuses to write rather than leak. (EM.2.9)
func PublishedLeaks(idx *ConstraintIndex) []string {
	if idx == nil {
		return nil
	}
	var leaked []string
	for _, c := range idx.Constraints {
		if data, err := json.Marshal(c); err == nil && strings.Contains(string(data), ".agents") {
			leaked = append(leaked, c.ID)
		}
	}
	return leaked
}

// LoadConstraintIndex reads and parses the constraint index.
func LoadConstraintIndex() (*ConstraintIndex, error) {
	path := ConstraintIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no constraints found — run constraint-compiler.sh first")
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var idx ConstraintIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &idx, nil
}

// WithConstraintLock acquires the compile lock and runs fn.
func WithConstraintLock(fn func() error) error {
	lockPath := ConstraintLockPath()
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create constraints dir: %w", err)
	}

	var lockFile *os.File
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("acquire constraint lock: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("acquire constraint lock: %w", err)
	}
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}()

	return fn()
}

// SaveConstraintIndexUnlocked writes the index without acquiring the lock.
func SaveConstraintIndexUnlocked(idx *ConstraintIndex) error {
	path := ConstraintIndexPath()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling index: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create constraints dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "index.json.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp constraint index: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp constraint index: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp constraint index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp constraint index: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename constraint index: %w", err)
	}
	return nil
}

// SaveConstraintIndex writes the constraint index back to disk under lock.
func SaveConstraintIndex(idx *ConstraintIndex) error {
	return WithConstraintLock(func() error {
		return SaveConstraintIndexUnlocked(idx)
	})
}

// BuildConstraintEntry builds a warn-only shadow ConstraintEntry from a
// finding's mechanical detector metadata and deterministic replay result, or
// (zero, false) when metadata is incomplete. Replay itself is owned by the
// FindingCompilerPort and must have passed before this constructor is called.
//
// Required frontmatter: detectability=mechanical, detector_pattern (non-empty),
// constraint_path_globs (non-empty), compiled_at; detector_kind defaults to and
// must be "regex". The entry is always warn-only status="shadow". It can become
// blocking only after `ao constraint activate` verifies cited precision evidence.
// Field names match the enforcement gate exactly (kind=regex / pattern /
// path_globs).
func BuildConstraintEntry(id string, fm map[string]string, replay ports.DetectorReplayResult) (ConstraintEntry, bool) {
	if strings.TrimSpace(fm["detectability"]) != "mechanical" {
		return ConstraintEntry{}, false
	}
	pattern := strings.TrimSpace(fm["detector_pattern"])
	if pattern == "" {
		return ConstraintEntry{}, false
	}
	kind := strings.TrimSpace(fm["detector_kind"])
	if kind == "" {
		kind = "regex"
	}
	if kind != "regex" {
		return ConstraintEntry{}, false
	}
	globs := splitConstraintCSV(fm["constraint_path_globs"])
	if len(globs) == 0 {
		return ConstraintEntry{}, false
	}
	compiledAt := strings.TrimSpace(fm["compiled_at"])
	if compiledAt == "" {
		return ConstraintEntry{}, false
	}
	if len(replay.PositiveRefs) == 0 || len(replay.NegativeControlRefs) == 0 {
		return ConstraintEntry{}, false
	}
	title := strings.TrimSpace(fm["title"])
	if title == "" {
		title = id
	}
	return ConstraintEntry{
		ID:              id,
		FindingID:       id,
		Title:           title,
		Source:          "finding",
		SourceArtifact:  ".agents/findings/" + id + ".md",
		SourceType:      strings.TrimSpace(fm["source"]),
		CompilerTargets: []string{"constraint"},
		Detectability:   "mechanical",
		Status:          "shadow",
		EnforcementMode: "warn",
		CompiledAt:      compiledAt,
		ReviewFile:      ".agents/constraints/" + id + ".sh",
		File:            ".agents/constraints/" + id + ".sh",
		AppliesTo:       ConstraintAppliesTo{PathGlobs: globs},
		Detector: ConstraintDetector{
			Kind:    "regex",
			Mode:    strings.TrimSpace(fm["detector_mode"]),
			Pattern: pattern,
			Exclude: strings.TrimSpace(fm["detector_exclude"]),
			Message: strings.TrimSpace(fm["detector_message"]),
		},
		Evidence: constraintEvidenceFromReplay(replay),
	}, true
}

const minimumConstraintPrecision = 0.95

func constraintEvidenceFromReplay(replay ports.DetectorReplayResult) ConstraintEvidence {
	evidence := ConstraintEvidence{
		PositiveRefs:        append([]string(nil), replay.PositiveRefs...),
		NegativeControlRefs: append([]string(nil), replay.NegativeControlRefs...),
	}
	if precision := replay.Precision; precision != nil {
		evidence.PrecisionEvidenceRef = precision.EvidenceRef
		evidence.ShadowSamples = precision.Samples
		evidence.TruePositives = precision.TruePositives
		evidence.FalsePositives = precision.FalsePositives
	}
	return evidence
}

// ValidateConstraintActivation fails closed until a shadow detector carries a
// cited precision measurement at or above the blocking threshold.
func ValidateConstraintActivation(entry ConstraintEntry) error {
	if entry.Status != "shadow" || entry.EnforcementMode != "warn" {
		return fmt.Errorf("constraint %q is %q/%q; can only activate from warn-only shadow", entry.ID, entry.Status, entry.EnforcementMode)
	}
	if len(entry.Evidence.PositiveRefs) == 0 || len(entry.Evidence.NegativeControlRefs) == 0 {
		return fmt.Errorf("constraint %q lacks positive and negative replay evidence", entry.ID)
	}
	if strings.TrimSpace(entry.Evidence.PrecisionEvidenceRef) == "" {
		return fmt.Errorf("constraint %q lacks precision evidence", entry.ID)
	}
	if entry.Evidence.ShadowSamples <= 0 ||
		entry.Evidence.TruePositives < 0 || entry.Evidence.FalsePositives < 0 ||
		entry.Evidence.TruePositives+entry.Evidence.FalsePositives != entry.Evidence.ShadowSamples {
		return fmt.Errorf("constraint %q has invalid precision evidence", entry.ID)
	}
	precision := float64(entry.Evidence.TruePositives) / float64(entry.Evidence.ShadowSamples)
	if precision < minimumConstraintPrecision {
		return fmt.Errorf("constraint %q precision %.3f is below %.2f", entry.ID, precision, minimumConstraintPrecision)
	}
	return nil
}

// splitConstraintCSV splits a comma-separated value into trimmed, non-empty
// entries.
func splitConstraintCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// UpsertConstraintAt merges a single constraint entry into the index rooted at
// root (root-aware so callers like membrane derive-checks, which resolve a
// project dir distinct from CWD, write the correct index). It loads-or-inits the
// index, then: if an entry with the same ID exists and force is false, it is a
// no-op (idempotent re-run); otherwise the entry is inserted/replaced. The save
// is atomic and lock-guarded. Returns whether the index was written.
//
// A compiled constraint is persisted as status="shadow" and warn-only. Explicit
// activation remains impossible until cited precision evidence is present.
func UpsertConstraintAt(root string, entry ConstraintEntry, force bool) (bool, error) {
	indexPath := filepath.Join(root, ConstraintIndexPath())
	lockPath := filepath.Join(root, ConstraintLockPath())
	wrote := false
	err := withConstraintLockAt(lockPath, func() error {
		idx, err := loadConstraintIndexAtPath(indexPath)
		if err != nil {
			return err
		}
		if existing := FindConstraint(idx, entry.ID); existing != nil {
			if !force {
				return nil // idempotent: present, leave it
			}
			*existing = entry
		} else {
			idx.Constraints = append(idx.Constraints, entry)
		}
		if err := saveConstraintIndexAtPath(indexPath, idx); err != nil {
			return err
		}
		wrote = true
		return nil
	})
	return wrote, err
}

// loadConstraintIndexAtPath reads/parses an index at an explicit path, returning
// an empty index when the file is absent (the legitimate empty state).
func loadConstraintIndexAtPath(path string) (*ConstraintIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConstraintIndex{SchemaVersion: 1}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var idx ConstraintIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	return &idx, nil
}

// saveConstraintIndexAtPath atomically writes the index to an explicit path.
func saveConstraintIndexAtPath(path string, idx *ConstraintIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling index: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create constraints dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "index.json.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp constraint index: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp constraint index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp constraint index: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename constraint index: %w", err)
	}
	return nil
}

// withConstraintLockAt is the root-aware twin of WithConstraintLock.
func withConstraintLockAt(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create constraints dir: %w", err)
	}
	var lockFile *os.File
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("acquire constraint lock: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("acquire constraint lock: %w", err)
	}
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}()
	return fn()
}

// FindConstraint locates a constraint by ID and returns its pointer.
func FindConstraint(idx *ConstraintIndex, id string) *ConstraintEntry {
	for i := range idx.Constraints {
		if idx.Constraints[i].ID == id {
			return &idx.Constraints[i]
		}
	}
	return nil
}

// FilterStaleConstraints returns non-retired constraints compiled before cutoff.
func FilterStaleConstraints(entries []ConstraintEntry, cutoff time.Time) []ConstraintEntry {
	stale := make([]ConstraintEntry, 0)
	for _, c := range entries {
		if c.Status == "retired" {
			continue
		}
		compiled, parseErr := time.Parse(time.RFC3339, c.CompiledAt)
		if parseErr != nil {
			compiled, parseErr = time.Parse("2006-01-02", c.CompiledAt)
			if parseErr != nil {
				continue
			}
		}
		if compiled.Before(cutoff) {
			stale = append(stale, c)
		}
	}
	return stale
}
