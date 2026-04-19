package search

import "time"

// Learning represents a single harvested learning from the knowledge store.
type Learning struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	Source          string  `json:"source,omitempty"`
	SourceBead      string  `json:"source_bead,omitempty"`  // Bead ID that produced this learning
	SourcePhase     string  `json:"source_phase,omitempty"` // RPI phase (research|plan|implement|validate)
	FreshnessScore  float64 `json:"freshness_score,omitempty"`
	AgeWeeks        float64 `json:"age_weeks,omitempty"`
	Utility         float64 `json:"utility,omitempty"`         // MemRL utility value
	CompositeScore  float64 `json:"composite_score,omitempty"` // Two-Phase ranking score
	Maturity        string  `json:"maturity,omitempty"`        // CASS maturity level
	SessionType     string  `json:"session_type,omitempty"`    // career, research, debug, implement, brainstorm
	SectionHeading  string  `json:"section_heading,omitempty"`
	SectionLocator  string  `json:"section_locator,omitempty"`
	MatchedSnippet  string  `json:"matched_snippet,omitempty"`
	MatchConfidence float64 `json:"match_confidence,omitempty"`
	MatchProvenance string  `json:"match_provenance,omitempty"`
	BodyText        string  `json:"-"` // Full body text for search (populated on demand)
	Stability       string  `json:"-"` // "experimental" | "stable", default "stable"
	Superseded      bool    `json:"-"` // Internal flag - not serialized
	Global          bool    `json:"-"` // Internal flag: from global dir
}

// Scorable interface implementation for Learning.
func (l *Learning) GetFreshness() float64  { return l.FreshnessScore }
func (l *Learning) GetUtility() float64    { return l.Utility }
func (l *Learning) GetMaturity() string    { return l.Maturity }
func (l *Learning) SetComposite(v float64) { l.CompositeScore = v }

// Pattern represents an active pattern from the knowledge store.
type Pattern struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	FilePath       string  `json:"file_path,omitempty"`
	FreshnessScore float64 `json:"freshness_score,omitempty"`
	AgeWeeks       float64 `json:"age_weeks,omitempty"`
	Utility        float64 `json:"utility,omitempty"`
	CompositeScore float64 `json:"composite_score,omitempty"`
	Global         bool    `json:"-"` // Internal flag: from global dir
}

// Scorable interface implementation for Pattern.
func (p *Pattern) GetFreshness() float64  { return p.FreshnessScore }
func (p *Pattern) GetUtility() float64    { return p.Utility }
func (p *Pattern) GetMaturity() string    { return "" }
func (p *Pattern) SetComposite(v float64) { p.CompositeScore = v }

// KnowledgeFinding represents a finding from vibe-check or other analysis skills.
type KnowledgeFinding struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Summary             string   `json:"summary"`
	Source              string   `json:"source,omitempty"`
	SourceSkill         string   `json:"source_skill,omitempty"`
	Severity            string   `json:"severity,omitempty"`
	Detectability       string   `json:"detectability,omitempty"`
	Status              string   `json:"status,omitempty"`
	CompilerTargets     []string `json:"compiler_targets,omitempty"`
	ScopeTags           []string `json:"scope_tags,omitempty"`
	ApplicableWhen      []string `json:"applicable_when,omitempty"`
	ApplicableLanguages []string `json:"applicable_languages,omitempty"`
	HitCount            int      `json:"hit_count,omitempty"`
	LastCited           string   `json:"last_cited,omitempty"`
	RetiredBy           string   `json:"retired_by,omitempty"`
	FreshnessScore      float64  `json:"freshness_score,omitempty"`
	AgeWeeks            float64  `json:"age_weeks,omitempty"`
	Utility             float64  `json:"utility,omitempty"`
	CompositeScore      float64  `json:"composite_score,omitempty"`
	Global              bool     `json:"-"`
}

// Scorable interface implementation for KnowledgeFinding.
func (f *KnowledgeFinding) GetFreshness() float64  { return f.FreshnessScore }
func (f *KnowledgeFinding) GetUtility() float64    { return f.Utility }
func (f *KnowledgeFinding) GetMaturity() string    { return "" }
func (f *KnowledgeFinding) SetComposite(v float64) { f.CompositeScore = v }

// Session represents a recent session summary from the knowledge store.
type Session struct {
	Date    string `json:"date"`
	Summary string `json:"summary"`
	Path    string `json:"path,omitempty"`
}

// PredecessorContext holds structured context from a predecessor agent's handoff.
type PredecessorContext struct {
	WorkingOn  string `json:"working_on,omitempty"`
	Progress   string `json:"progress,omitempty"`
	Blocker    string `json:"blocker,omitempty"`
	NextStep   string `json:"next_step,omitempty"`
	SessionAge string `json:"session_age,omitempty"`
	RawSummary string `json:"raw_summary,omitempty"` // Fallback when no structured headers found
}

// InjectedKnowledge is the top-level container for all knowledge injected into a session.
type InjectedKnowledge struct {
	Predecessor *PredecessorContext `json:"predecessor,omitempty"`
	Learnings   []Learning          `json:"learnings,omitempty"`
	Patterns    []Pattern           `json:"patterns,omitempty"`
	Sessions    []Session           `json:"sessions,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
	Query       string              `json:"query,omitempty"`
	BeadID      string              `json:"bead_id,omitempty"`
}
