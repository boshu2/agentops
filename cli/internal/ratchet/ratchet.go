// Package ratchet implements the Brownian Ratchet workflow tracking.
// It provides a tool-agnostic way to track progress through the RPI workflow.
package ratchet

import (
	"strings"
	"time"
)

// Step represents a ratchet step in the workflow chain.
type Step string

const (
	StepResearch   Step = "research"
	StepPreMortem  Step = "premortem"
	StepPlan       Step = "plan"
	StepImplement  Step = "implement"
	StepCrank      Step = "crank"
	StepVibe       Step = "vibe"
	StepPostMortem Step = "postmortem"
)

// AllSteps returns all valid steps in workflow order.
func AllSteps() []Step {
	return []Step{
		StepResearch,
		StepPreMortem,
		StepPlan,
		StepImplement,
		StepCrank,
		StepVibe,
		StepPostMortem,
	}
}

// stepAliases maps alternative names to canonical step names.
var stepAliases = map[string]Step{
	// Canonical names
	"research":   StepResearch,
	"premortem":  StepPreMortem,
	"plan":       StepPlan,
	"implement":  StepImplement,
	"crank":      StepCrank,
	"vibe":       StepVibe,
	"postmortem": StepPostMortem,

	// Permanent legacy aliases with hyphens
	"pre-mortem":  StepPreMortem,
	"post-mortem": StepPostMortem,

	// Aliases with underscore
	"pre_mortem":  StepPreMortem,
	"post_mortem": StepPostMortem,

	// Semantic aliases
	"formulate": StepPlan, // Legacy alias - formulate is now plan
	"autopilot": StepCrank,
	"validate":  StepVibe,
	"review":    StepPostMortem,
	"execute":   StepCrank,

	// Phased-mode canonical phase names → ratchet steps
	"discovery":  StepResearch, // Phase 1 (discovery) maps to research step
	"validation": StepVibe,     // Phase 3 (validation) maps to vibe step
}

// ParseStep normalizes a step name to its canonical form.
// Returns empty string if the step is not recognized.
func ParseStep(name string) Step {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if step, ok := stepAliases[normalized]; ok {
		return step
	}
	return ""
}

// IsValid returns true if the step is a recognized step name.
func (s Step) IsValid() bool {
	return ParseStep(string(s)) != ""
}

// Tier represents knowledge quality tier (from the 5-tier system).
type Tier int

const (
	TierObservation Tier = 0 // .agents/candidates/ - raw observations
	TierLearning    Tier = 1 // .agents/learnings/ - 2+ citations
	TierPattern     Tier = 2 // .agents/patterns/ - 3+ sessions
	TierSkill       Tier = 3 // plugins/*/skills/ - tested workflow
	TierCore        Tier = 4 // CLAUDE.md - 10+ uses
)

// String returns the tier name.
func (t Tier) String() string {
	switch t {
	case TierObservation:
		return "observation"
	case TierLearning:
		return "learning"
	case TierPattern:
		return "pattern"
	case TierSkill:
		return "skill"
	case TierCore:
		return "core"
	default:
		return unknownValue
	}
}

// Location returns the canonical storage location for a tier.
func (t Tier) Location() string {
	switch t {
	case TierObservation:
		return ".agents/candidates/"
	case TierLearning:
		return ".agents/learnings/"
	case TierPattern:
		return ".agents/patterns/"
	case TierSkill:
		return "plugins/*/skills/"
	case TierCore:
		return "CLAUDE.md"
	default:
		return ""
	}
}

// ChainEntry records a single step completion in the ratchet chain.
type ChainEntry struct {
	// Step is the workflow step completed.
	Step Step `json:"step"`

	// Timestamp is when the step was completed.
	Timestamp time.Time `json:"timestamp"`

	// Input is the path or ID of the input artifact.
	Input string `json:"input,omitempty"`

	// Output is the path or ID of the output artifact.
	Output string `json:"output"`

	// Locked indicates the step cannot be re-run (ratchet engaged).
	Locked bool `json:"locked"`

	// Skipped indicates the step was intentionally skipped.
	Skipped bool `json:"skipped,omitempty"`

	// Reason explains why a step was skipped.
	Reason string `json:"reason,omitempty"`

	// Tier is the quality tier of the output (if applicable).
	Tier *Tier `json:"tier,omitempty"`

	// Location indicates where the artifact was found/stored.
	Location string `json:"location,omitempty"`

	// Cycle is the RPI iteration number (1 for first cycle, 2+ for iterations).
	Cycle int `json:"cycle,omitempty"`

	// ParentEpic is the epic ID from the prior RPI cycle (empty for first cycle).
	ParentEpic string `json:"parent_epic,omitempty"`
}

// Chain represents the full ratchet chain state for a workflow.
type Chain struct {
	// ID is the unique chain identifier (typically epic ID or timestamp).
	ID string `json:"id"`

	// Started is when the chain was initiated.
	Started time.Time `json:"started"`

	// Entries are the recorded step completions (JSONL order = time order).
	Entries []ChainEntry `json:"chain"`

	// EpicID is the associated beads epic (if any).
	EpicID string `json:"epic_id,omitempty"`

	// path is the file path where the chain is stored.
	path string
}

// GateResult contains the result of a gate check.
type GateResult struct {
	// Step is the step being checked.
	Step Step `json:"step"`

	// Passed indicates if the gate is satisfied.
	Passed bool `json:"passed"`

	// Message describes the gate result.
	Message string `json:"message"`

	// Input is the path to the input artifact (if found).
	Input string `json:"input,omitempty"`

	// Location is where the input artifact was found.
	Location string `json:"location,omitempty"`
}

// ValidationResult contains the result of step validation.
type ValidationResult struct {
	// Step is the step being validated.
	Step Step `json:"step"`

	// Valid indicates if validation passed.
	Valid bool `json:"valid"`

	// Issues lists any problems found.
	Issues []string `json:"issues,omitempty"`

	// Warnings lists non-blocking concerns.
	Warnings []string `json:"warnings,omitempty"`

	// Tier is the assessed quality tier.
	Tier *Tier `json:"tier,omitempty"`

	// Lenient indicates if validation was run in lenient mode.
	Lenient bool `json:"lenient,omitempty"`

	// LenientExpiryDate is the expiry date for lenient bypass (if applicable).
	LenientExpiryDate *string `json:"lenient_expiry_date,omitempty"`

	// LenientExpiringSoon is true if expiry is within 30 days.
	LenientExpiringSoon bool `json:"lenient_expiring_soon,omitempty"`
}

// ValidateOptions controls validator behavior.
type ValidateOptions struct {
	// Lenient allows legacy artifacts without schema_version to pass validation.
	Lenient bool

	// LenientExpiryDate specifies the expiry date for lenient mode (default: now + 90 days).
	LenientExpiryDate *time.Time
}

// FindResult contains the result of a multi-location artifact search.
type FindResult struct {
	// Pattern is the search pattern used.
	Pattern string `json:"pattern"`

	// Matches are the found artifacts.
	Matches []FindMatch `json:"matches"`

	// Warnings lists any concerns (e.g., duplicates).
	Warnings []string `json:"warnings,omitempty"`
}

// FindMatch is a single artifact found in a search.
type FindMatch struct {
	// Path is the artifact file path.
	Path string `json:"path"`

	// Location is the search location (crew, rig, town, plugins).
	Location string `json:"location"`

	// Priority indicates search precedence (lower = higher priority).
	Priority int `json:"priority"`
}
