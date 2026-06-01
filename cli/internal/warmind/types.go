// Package warmind provides team knowledge sharing via git-tracked learnings.
// It extends AgentOps' single-engineer model to share knowledge across a team
// using pool staging, scoring, citation-gated promotion, and maturity lifecycle.
package warmind

import (
	"time"
)

// Tier represents the quality tier of a warmind candidate.
type Tier string

const (
	TierGold    Tier = "gold"    // Score > 0.85, auto-promote after 24h
	TierSilver  Tier = "silver"  // Score 0.70-0.85, promote after 1 citation
	TierBronze  Tier = "bronze"  // Score 0.50-0.70, promote after 3 citations
	TierDiscard Tier = "discard" // Score < 0.50, auto-reject
)

// Status represents the pool status of a warmind candidate.
type Status string

const (
	StatusPending  Status = "pending"  // Awaiting scoring
	StatusStaged   Status = "staged"   // Scored, awaiting citations
	StatusPromoted Status = "promoted" // Moved to learnings
	StatusRejected Status = "rejected" // Rejected (low quality or contradiction)
	StatusArchived Status = "archived" // Expired due to non-use
)

// Maturity represents the maturity level of a warmind learning.
type Maturity string

const (
	MaturityProvisional Maturity = "provisional"  // New, needs validation
	MaturityEstablished Maturity = "established"  // 3+ citations, proven useful
	MaturityAntiPattern Maturity = "anti-pattern" // Marked as bad practice
)

// Candidate represents a learning candidate in the warmind pool.
type Candidate struct {
	// ID is the unique identifier for this candidate.
	ID string `json:"id"`

	// Title is the short description of the learning.
	Title string `json:"title"`

	// Content is the full learning body.
	Content string `json:"content"`

	// Author is the git username who contributed this learning.
	Author string `json:"author"`

	// AuthorEmail is the git email for dedup across machines.
	AuthorEmail string `json:"author_email"`

	// SourcePath is the original path in .agents/learnings/.
	SourcePath string `json:"source_path"`

	// SourceSessionID is the session that generated this learning.
	SourceSessionID string `json:"source_session_id,omitempty"`

	// Category is the learning category (architecture, debugging, etc.).
	Category string `json:"category,omitempty"`

	// Tags are searchable tags for this learning.
	Tags []string `json:"tags,omitempty"`

	// Confidence is the author's confidence in this learning (0-1).
	Confidence float64 `json:"confidence"`

	// CreatedAt is when the original learning was created.
	CreatedAt time.Time `json:"created_at"`

	// ContentHash is the SHA256 hash of normalized content for dedup.
	ContentHash string `json:"content_hash"`
}

// ScoringResult holds the scoring dimensions for a candidate.
type ScoringResult struct {
	// Novelty is how different from existing team learnings (0-1).
	Novelty float64 `json:"novelty"`

	// Specificity is how concrete vs vague (0-1).
	Specificity float64 `json:"specificity"`

	// Actionability is how directly applicable (0-1).
	Actionability float64 `json:"actionability"`

	// ConfidenceScore is the weighted author confidence (0-1).
	ConfidenceScore float64 `json:"confidence_score"`

	// CompositeScore is the weighted sum of all dimensions.
	CompositeScore float64 `json:"composite_score"`

	// Tier is the quality tier based on composite score.
	Tier Tier `json:"tier"`

	// ScoredAt is when scoring was performed.
	ScoredAt time.Time `json:"scored_at"`
}

// PoolEntry represents a candidate in the warmind pool.
type PoolEntry struct {
	// Candidate holds the learning content and metadata.
	Candidate Candidate `json:"candidate"`

	// Scoring holds the quality assessment.
	Scoring ScoringResult `json:"scoring"`

	// Status is the current pool status.
	Status Status `json:"status"`

	// AddedAt is when this entry was added to the pool.
	AddedAt time.Time `json:"added_at"`

	// UpdatedAt is when this entry was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// CitationCount is citations from OTHER engineers.
	CitationCount int `json:"citation_count"`

	// LastCitedAt is when this was last cited by another engineer.
	LastCitedAt *time.Time `json:"last_cited_at,omitempty"`

	// PromotedAt is when this was promoted to learnings (if promoted).
	PromotedAt *time.Time `json:"promoted_at,omitempty"`

	// RejectedAt is when this was rejected (if rejected).
	RejectedAt *time.Time `json:"rejected_at,omitempty"`

	// RejectionReason explains why this was rejected.
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// Citation represents a citation event for a warmind learning.
type Citation struct {
	// ArtifactPath is the path to the cited learning.
	ArtifactPath string `json:"artifact_path"`

	// ArtifactID is the learning ID.
	ArtifactID string `json:"artifact_id"`

	// CitedAt is when the citation occurred.
	CitedAt time.Time `json:"cited_at"`

	// CitedBy is the git username who cited this.
	CitedBy string `json:"cited_by"`

	// CitedByEmail is the git email of the citer.
	CitedByEmail string `json:"cited_by_email"`

	// SessionID is the session in which the citation occurred.
	SessionID string `json:"session_id"`

	// Query is the search query that surfaced this learning.
	Query string `json:"query,omitempty"`

	// WorkspacePath is the repo where the citation occurred.
	WorkspacePath string `json:"workspace_path"`

	// IsSelfCitation is true if author == citer.
	IsSelfCitation bool `json:"is_self_citation"`
}

// Contradiction represents a detected contradiction between learnings.
type Contradiction struct {
	// ID is the unique identifier for this contradiction.
	ID string `json:"id"`

	// DetectedAt is when this contradiction was detected.
	DetectedAt time.Time `json:"detected_at"`

	// LearningA is the first learning involved.
	LearningA string `json:"learning_a"`

	// LearningAAuthor is the author of learning A.
	LearningAAuthor string `json:"learning_a_author"`

	// LearningB is the second learning involved.
	LearningB string `json:"learning_b"`

	// LearningBAuthor is the author of learning B.
	LearningBAuthor string `json:"learning_b_author"`

	// ConflictType describes the type of conflict.
	ConflictType string `json:"conflict_type"`

	// Summary is a human-readable description of the conflict.
	Summary string `json:"summary"`

	// Status is the resolution status.
	Status string `json:"status"` // pending_review, resolved, dismissed

	// ResolvedAt is when this was resolved (if resolved).
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// ResolvedBy is who resolved this (if resolved).
	ResolvedBy string `json:"resolved_by,omitempty"`

	// Resolution is how this was resolved (if resolved).
	Resolution string `json:"resolution,omitempty"`
}

// Learning represents a promoted warmind learning.
type Learning struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Title is the short description.
	Title string `json:"title"`

	// Content is the full body.
	Content string `json:"content"`

	// Author is who contributed this.
	Author string `json:"author"`

	// AuthorEmail is the git email.
	AuthorEmail string `json:"author_email"`

	// Category is the learning category.
	Category string `json:"category,omitempty"`

	// Tags are searchable tags.
	Tags []string `json:"tags,omitempty"`

	// Confidence is the author's confidence.
	Confidence float64 `json:"confidence"`

	// Utility is the computed utility (freshness * citations).
	Utility float64 `json:"utility"`

	// Maturity is the current maturity level.
	Maturity Maturity `json:"maturity"`

	// CreatedAt is when this was originally created.
	CreatedAt time.Time `json:"created_at"`

	// PromotedAt is when this was promoted from pool.
	PromotedAt time.Time `json:"promoted_at"`

	// LastCitedAt is when this was last cited.
	LastCitedAt *time.Time `json:"last_cited_at,omitempty"`

	// CitationCount is total citations.
	CitationCount int `json:"citation_count"`

	// Tier is the quality tier at promotion time.
	Tier Tier `json:"tier"`

	// ContentHash is for dedup.
	ContentHash string `json:"content_hash"`

	// FilePath is the on-disk location.
	FilePath string `json:"-"`
}

// ChainEvent records a pool operation for audit.
type ChainEvent struct {
	// Timestamp is when this event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Operation is the action (add, score, stage, promote, reject, archive).
	Operation string `json:"operation"`

	// CandidateID is the affected candidate.
	CandidateID string `json:"candidate_id"`

	// FromStatus is the previous status.
	FromStatus Status `json:"from_status,omitempty"`

	// ToStatus is the new status.
	ToStatus Status `json:"to_status,omitempty"`

	// Actor is who performed this operation.
	Actor string `json:"actor,omitempty"`

	// Reason explains why.
	Reason string `json:"reason,omitempty"`

	// ArtifactPath is the destination for promotions.
	ArtifactPath string `json:"artifact_path,omitempty"`
}

// Config holds warmind configuration.
type Config struct {
	// LearningsDir is where promoted learnings go.
	LearningsDir string `yaml:"learnings_dir" json:"learnings_dir"`

	// PoolDir is where the pool is stored.
	PoolDir string `yaml:"pool_dir" json:"pool_dir"`

	// CitationsFile is where citations are tracked.
	CitationsFile string `yaml:"citations_file" json:"citations_file"`

	// ContradictionsFile is where contradictions are stored.
	ContradictionsFile string `yaml:"contradictions_file" json:"contradictions_file"`

	// ArchiveDir is where expired learnings go.
	ArchiveDir string `yaml:"archive_dir" json:"archive_dir"`

	// Weight is the ranking weight vs local learnings.
	Weight float64 `yaml:"weight" json:"weight"`

	// Scoring configuration.
	Scoring ScoringConfig `yaml:"scoring" json:"scoring"`

	// Promotion configuration.
	Promotion PromotionConfig `yaml:"promotion" json:"promotion"`

	// Maturity configuration.
	Maturity MaturityConfig `yaml:"maturity" json:"maturity"`

	// Contradict configuration.
	Contradict ContradictConfig `yaml:"contradict" json:"contradict"`
}

// ScoringConfig holds scoring weights.
type ScoringConfig struct {
	NoveltyWeight       float64 `yaml:"novelty_weight" json:"novelty_weight"`
	SpecificityWeight   float64 `yaml:"specificity_weight" json:"specificity_weight"`
	ActionabilityWeight float64 `yaml:"actionability_weight" json:"actionability_weight"`
	ConfidenceWeight    float64 `yaml:"confidence_weight" json:"confidence_weight"`
}

// PromotionConfig holds promotion rules.
type PromotionConfig struct {
	GoldAutoPromoteHours    int  `yaml:"gold_auto_promote_hours" json:"gold_auto_promote_hours"`
	SilverCitationThreshold int  `yaml:"silver_citation_threshold" json:"silver_citation_threshold"`
	BronzeCitationThreshold int  `yaml:"bronze_citation_threshold" json:"bronze_citation_threshold"`
	SelfCitationAllowed     bool `yaml:"self_citation_allowed" json:"self_citation_allowed"`
}

// MaturityConfig holds maturity lifecycle rules.
type MaturityConfig struct {
	ProvisionalExpireDays int     `yaml:"provisional_expire_days" json:"provisional_expire_days"`
	EstablishedExpireDays int     `yaml:"established_expire_days" json:"established_expire_days"`
	MinCorpusSize         int     `yaml:"min_corpus_size" json:"min_corpus_size"`
	DecayRate             float64 `yaml:"decay_rate" json:"decay_rate"`
}

// ContradictConfig holds contradiction detection rules.
type ContradictConfig struct {
	MinSignals    int  `yaml:"min_signals" json:"min_signals"`
	AutoBlockSync bool `yaml:"auto_block_sync" json:"auto_block_sync"`
}

// DefaultConfig returns the default warmind configuration.
func DefaultConfig() Config {
	return Config{
		LearningsDir:       ".warmind/learnings",
		PoolDir:            ".warmind/pool",
		CitationsFile:      ".warmind/citations.jsonl",
		ContradictionsFile: ".warmind/contradictions.jsonl",
		ArchiveDir:         ".warmind/archive",
		Weight:             0.9,
		Scoring: ScoringConfig{
			NoveltyWeight:       0.35,
			SpecificityWeight:   0.25,
			ActionabilityWeight: 0.20,
			ConfidenceWeight:    0.20,
		},
		Promotion: PromotionConfig{
			GoldAutoPromoteHours:    24,
			SilverCitationThreshold: 1,
			BronzeCitationThreshold: 3,
			SelfCitationAllowed:     false,
		},
		Maturity: MaturityConfig{
			ProvisionalExpireDays: 30,
			EstablishedExpireDays: 90,
			MinCorpusSize:         10,
			DecayRate:             0.17,
		},
		Contradict: ContradictConfig{
			MinSignals:    3,
			AutoBlockSync: false,
		},
	}
}

// TierFromScore returns the tier based on composite score.
func TierFromScore(score float64) Tier {
	switch {
	case score > 0.85:
		return TierGold
	case score >= 0.70:
		return TierSilver
	case score >= 0.50:
		return TierBronze
	default:
		return TierDiscard
	}
}

// TierOrder returns the numeric order of a tier (higher = better).
func TierOrder(t Tier) int {
	switch t {
	case TierGold:
		return 3
	case TierSilver:
		return 2
	case TierBronze:
		return 1
	default:
		return 0
	}
}
