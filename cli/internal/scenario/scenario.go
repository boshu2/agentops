// Package scenario creates and describes AgentOps behavioral validation
// scenarios. Create is the single scenario-authoring path shared by
// `ao scenario add` and `ao goals scenarios --create`, so the two commands
// can never drift in how they shape a scenario file.
package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IDPattern matches a scenario ID: human "s-YYYY-MM-DD-NNN" or agent "auto-*".
var IDPattern = regexp.MustCompile(`^(s-\d{4}-\d{2}-\d{2}-\d{3}|auto-.+)$`)

// directiveIDPattern is the stable directive-ID format. It is duplicated from
// cli/internal/goals (rather than imported) to keep this package a leaf with
// no dependency on the goals package.
var directiveIDPattern = regexp.MustCompile(`^d-[a-z0-9][a-z0-9-]*$`)

// AcceptanceVector is one measurable dimension of scenario satisfaction.
type AcceptanceVector struct {
	Dimension string  `json:"dimension"`
	Threshold float64 `json:"threshold"`
	Check     string  `json:"check,omitempty"`
}

// Scenario is the on-disk scenario JSON shape (schemas/scenario.v1.schema.json).
type Scenario struct {
	ID                    string             `json:"id"`
	DirectiveID           string             `json:"directive_id,omitempty"`
	Version               int                `json:"version"`
	Date                  string             `json:"date"`
	Goal                  string             `json:"goal"`
	Narrative             string             `json:"narrative"`
	ExpectedOutcome       string             `json:"expected_outcome"`
	AcceptanceVectors     []AcceptanceVector `json:"acceptance_vectors,omitempty"`
	SatisfactionThreshold float64            `json:"satisfaction_threshold"`
	Source                string             `json:"source,omitempty"`
	Status                string             `json:"status"`
}

// ValidStatus reports whether s is an allowed scenario lifecycle status.
func ValidStatus(s string) bool {
	switch s {
	case "active", "draft", "retired":
		return true
	default:
		return false
	}
}

// ValidSource reports whether s is an allowed scenario provenance value.
func ValidSource(s string) bool {
	switch s {
	case "human", "agent", "prod-telemetry":
		return true
	default:
		return false
	}
}

// CreateOptions configures Create.
type CreateOptions struct {
	Goal            string
	Narrative       string // optional; inferred from Goal when empty
	ExpectedOutcome string // optional; inferred from Goal when empty
	Threshold       float64
	Status          string
	Source          string
	// DirectiveID, when set, marks the scenario as a promoted spec scenario
	// linked to a GOALS.md directive (see docs/adr/ADR-0003).
	DirectiveID string
	// Dir is the directory the scenario file is written to. Empty defaults to
	// the ad hoc holdout directory.
	Dir string
	// Now is an injectable clock for deterministic IDs in tests.
	Now func() time.Time
}

// CreateResult is the outcome of a successful Create.
type CreateResult struct {
	Scenario Scenario
	Path     string
}

// Create authors a schema-compliant scenario file. It validates inputs, picks
// the next free same-day ID, and writes the JSON. A validation failure leaves
// the filesystem untouched.
func Create(opts CreateOptions) (*CreateResult, error) {
	goal := strings.TrimSpace(opts.Goal)
	if goal == "" {
		return nil, fmt.Errorf("goal is required")
	}
	if opts.Threshold < 0 || opts.Threshold > 1 {
		return nil, fmt.Errorf("threshold %.2f out of range [0, 1]", opts.Threshold)
	}
	if !ValidStatus(opts.Status) {
		return nil, fmt.Errorf("invalid status %q (must be active, draft, or retired)", opts.Status)
	}
	if !ValidSource(opts.Source) {
		return nil, fmt.Errorf("invalid source %q (must be human, agent, or prod-telemetry)", opts.Source)
	}
	if opts.DirectiveID != "" && !directiveIDPattern.MatchString(opts.DirectiveID) {
		return nil, fmt.Errorf("invalid directive_id %q (must match %s)", opts.DirectiveID, directiveIDPattern.String())
	}

	dir := opts.Dir
	if dir == "" {
		dir = filepath.Join(".agents", "holdout")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating scenario directory: %w", err)
	}

	date := now().UTC().Format("2006-01-02")
	id, err := NextID(dir, date)
	if err != nil {
		return nil, err
	}
	sc := Scenario{
		ID:                    id,
		DirectiveID:           opts.DirectiveID,
		Version:               1,
		Date:                  date,
		Goal:                  goal,
		Narrative:             inferNarrative(goal, opts.Narrative),
		ExpectedOutcome:       inferOutcome(goal, opts.ExpectedOutcome),
		SatisfactionThreshold: opts.Threshold,
		Source:                opts.Source,
		Status:                opts.Status,
	}
	path := filepath.Join(dir, id+".json")
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling scenario: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("writing scenario %s: %w", path, err)
	}
	return &CreateResult{Scenario: sc, Path: path}, nil
}

// NextID returns the next free scenario ID for date in dir (s-DATE-NNN).
func NextID(dir, date string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading scenario directory: %w", err)
	}
	prefix := "s-" + date + "-"
	maxSeq := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		for _, candidate := range idCandidates(dir, entry.Name()) {
			if seq, ok := sequence(candidate, prefix); ok && seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1), nil
}

// idCandidates returns the filename-derived and JSON-declared IDs for a file.
func idCandidates(dir, fileName string) []string {
	candidates := []string{strings.TrimSuffix(fileName, filepath.Ext(fileName))}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		return candidates
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &decoded); err == nil && strings.TrimSpace(decoded.ID) != "" {
		candidates = append(candidates, strings.TrimSpace(decoded.ID))
	}
	return candidates
}

// sequence extracts the 3-digit sequence number from an ID with the prefix.
func sequence(id, prefix string) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	seqText := strings.TrimPrefix(id, prefix)
	if len(seqText) != 3 {
		return 0, false
	}
	seq, err := strconv.Atoi(seqText)
	if err != nil {
		return 0, false
	}
	return seq, true
}

// inferNarrative returns override when set, else a narrative built from goal.
func inferNarrative(goal, override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	return fmt.Sprintf("A user or evaluator exercises the system behavior for this goal: %s.", goal)
}

// inferOutcome returns override when set, else an outcome built from goal.
func inferOutcome(goal, override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	return fmt.Sprintf("The implementation satisfies the goal in observable behavior: %s.", goal)
}
