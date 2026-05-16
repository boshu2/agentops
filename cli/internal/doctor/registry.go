package doctor

import (
	"io"
	"sort"
	"sync"
)

// Evidence is the witness data attached to a Finding.
type Evidence struct {
	File  string `json:"file,omitempty"`
	Lines []int  `json:"lines,omitempty"`
	Query string `json:"query,omitempty"`
	Hash  string `json:"hash,omitempty"`
}

// Remediation describes how a Finding can be addressed.
type Remediation struct {
	Command          string `json:"command"`
	ExplainCommand   string `json:"explain_command"`
	AutoFixable      bool   `json:"auto_fixable"`
	EstimatedActions int    `json:"estimated_actions"`
}

// Finding is a single diagnosed issue produced by a Detector.
type Finding struct {
	ID          string      `json:"id"`
	Severity    string      `json:"severity"`
	Subsystem   string      `json:"subsystem"`
	Title       string      `json:"title"`
	Confidence  float64     `json:"confidence"`
	Evidence    Evidence    `json:"evidence"`
	Remediation Remediation `json:"remediation"`
}

// DetectEnv carries read-only context into a Detector's Detect method.
type DetectEnv struct {
	RepoRoot  string
	CWD       string
	HomeDir   string
	TargetSHA string
	Online    bool
	Logger    io.Writer
}

// Detector inspects workspace state and emits Findings. Detect MUST be pure:
// it reads but never writes. Each Detector is registered once, from an init()
// function, by a per-subsystem file.
type Detector interface {
	ID() string
	Subsystem() string
	Severity() string
	Describe() string
	EstimatedCostMS() int
	OnlineRequired() bool
	QuickPath() bool
	Detect(env *DetectEnv) ([]Finding, error)
}

// FixResult is returned by a Fixer after attempting repair.
type FixResult struct {
	FixerID      string
	FindingIDs   []string
	ActionsTaken int
	Fixed        bool
	Err          error
}

// Fixer repairs the issues a Detector finds. Every disk write a Fixer performs
// MUST flow through Mutate. Fixers are registered once, from an init()
// function, by a per-subsystem file.
type Fixer interface {
	ID() string
	Preconditions() []string
	WritesTo() []string
	Ops() []string
	Reversible() bool
	Idempotent() bool
	AutoFixable() bool
	Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error)
}

// registry holds the package-level detector and fixer sets.
var registry = struct {
	mu        sync.RWMutex
	detectors map[string]Detector
	fixers    map[string]Fixer
}{
	detectors: make(map[string]Detector),
	fixers:    make(map[string]Fixer),
}

// RegisterDetector adds a Detector to the package registry. It is intended to
// be called from init() in per-subsystem files. A duplicate ID panics, which
// surfaces the programming error at startup.
func RegisterDetector(d Detector) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.detectors[d.ID()]; exists {
		panic("doctor: duplicate detector ID: " + d.ID())
	}
	registry.detectors[d.ID()] = d
}

// RegisterFixer adds a Fixer to the package registry. It is intended to be
// called from init() in per-subsystem files. A duplicate ID panics.
func RegisterFixer(f Fixer) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.fixers[f.ID()]; exists {
		panic("doctor: duplicate fixer ID: " + f.ID())
	}
	registry.fixers[f.ID()] = f
}

// Detectors returns all registered detectors sorted deterministically by ID.
func Detectors() []Detector {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Detector, 0, len(registry.detectors))
	for _, d := range registry.detectors {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Fixers returns all registered fixers sorted deterministically by ID.
func Fixers() []Fixer {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Fixer, 0, len(registry.fixers))
	for _, f := range registry.fixers {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// FixerByID returns the registered fixer with the given ID, or nil.
func FixerByID(id string) Fixer {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.fixers[id]
}
