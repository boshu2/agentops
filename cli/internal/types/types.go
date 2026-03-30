// Package types defines all data structures for the Olympus knowledge pipeline.
package types

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// TranscriptMessage represents a single message from a Claude Code JSONL transcript.
type TranscriptMessage struct {
	// Type indicates the message type (e.g., "user", "assistant", "tool_use", "tool_result").
	Type string `json:"type"`

	// Timestamp is when the message was recorded.
	Timestamp time.Time `json:"timestamp"`

	// Role is the message sender role (user, assistant, system).
	Role string `json:"role,omitempty"`

	// Content is the message text content.
	Content string `json:"content,omitempty"`

	// Tools contains any tool calls made in this message.
	Tools []ToolCall `json:"tools,omitempty"`

	// SessionID is the unique identifier for the conversation session.
	SessionID string `json:"session_id,omitempty"`

	// MessageIndex is the position of this message in the transcript.
	MessageIndex int `json:"message_index,omitempty"`
}

// ToolCall represents a single tool invocation within a message.
type ToolCall struct {
	// Name is the tool identifier (e.g., "Read", "Bash", "Edit").
	Name string `json:"name"`

	// Input contains the parameters passed to the tool.
	Input map[string]any `json:"input,omitempty"`

	// Output is the result returned by the tool.
	Output string `json:"output,omitempty"`

	// Error contains any error message if the tool failed.
	Error string `json:"error,omitempty"`

	// Duration is how long the tool call took.
	Duration time.Duration `json:"duration,omitempty"`
}

// KnowledgeType represents the category of extracted knowledge.
type KnowledgeType string

const (
	// KnowledgeTypeDecision is an architectural choice with rationale.
	KnowledgeTypeDecision KnowledgeType = "decision"

	// KnowledgeTypeSolution is a working fix for a problem.
	KnowledgeTypeSolution KnowledgeType = "solution"

	// KnowledgeTypeLearning is an insight gained from experience.
	KnowledgeTypeLearning KnowledgeType = "learning"

	// KnowledgeTypeFailure is what didn't work and why.
	KnowledgeTypeFailure KnowledgeType = "failure"

	// KnowledgeTypeReference is a pointer to a useful resource.
	KnowledgeTypeReference KnowledgeType = "reference"
)

// Tier represents the quality tier assignment for a candidate.
type Tier string

const (
	// TierGold is the highest quality tier (0.85-1.0 score).
	TierGold Tier = "gold"

	// TierSilver is high quality tier (0.70-0.84 score).
	TierSilver Tier = "silver"

	// TierBronze is acceptable quality tier (0.50-0.69 score).
	TierBronze Tier = "bronze"

	// TierDiscard is below threshold, not stored (<0.50 score).
	TierDiscard Tier = "discard"
)

// Source tracks the origin of extracted knowledge.
type Source struct {
	// TranscriptPath is the path to the source JSONL file.
	TranscriptPath string `json:"transcript_path"`

	// MessageIndex is the position in the transcript where this was extracted.
	MessageIndex int `json:"message_index"`

	// Timestamp is when the source message was recorded.
	Timestamp time.Time `json:"timestamp"`

	// SessionID is the conversation session identifier.
	SessionID string `json:"session_id"`
}

// Candidate represents a knowledge extraction candidate before scoring.
type Candidate struct {
	// ID is the unique identifier for this candidate (e.g., "ol-cand-abc123").
	ID string `json:"id"`

	// Type is the knowledge category (decision, solution, learning, failure, reference).
	Type KnowledgeType `json:"type"`

	// Content is the extracted knowledge text.
	Content string `json:"content"`

	// Context provides surrounding context for understanding.
	Context string `json:"context,omitempty"`

	// Source tracks where this candidate was extracted from.
	Source Source `json:"source"`

	// RawScore is the initial scoring before tier assignment.
	RawScore float64 `json:"raw_score,omitempty"`

	// Tier is the quality tier assigned after scoring.
	Tier Tier `json:"tier,omitempty"`

	// ProvenanceIDs links to provenance tracking records.
	ProvenanceIDs []string `json:"provenance_ids,omitempty"`

	// ExtractedAt is when this candidate was created.
	ExtractedAt time.Time `json:"extracted_at"`

	// Metadata contains additional extraction metadata.
	Metadata map[string]any `json:"metadata,omitempty"`

	// --- Supersession fields (ol-a46.1.4) ---

	// SupersededBy is the ID of a newer learning that replaces this one.
	// Empty string means this learning is not superseded.
	SupersededBy string `json:"superseded_by,omitempty"`

	// Supersedes is the ID of an older learning that this one replaces.
	// Creates a backward reference for traversal.
	Supersedes string `json:"supersedes,omitempty"`

	// IsCurrent indicates this is the terminal (latest) learning in a chain.
	// Enables O(1) lookup of current learnings without traversing chains.
	IsCurrent bool `json:"is_current"`

	// SupersessionDepth tracks how many times this has been superseded.
	// Used for validation: max depth = 3 to prevent unbounded chains.
	SupersessionDepth int `json:"supersession_depth,omitempty"`

	// --- Location fields (ol-a46.1.9) ---

	// Location identifies where this artifact was found in the multi-location hierarchy.
	// Values: "crew", "rig", "town", "plugins"
	// crew = most specific (current workspace), town = most general
	Location string `json:"location,omitempty"`

	// LocationPath is the absolute path where the artifact was resolved.
	// Enables verification and prevents path ambiguity.
	LocationPath string `json:"location_path,omitempty"`

	// --- Expiry fields (ol-a46.1.5) ---

	// ValidUntil is the date after which this learning may be stale.
	// Format: "2026-06-30" or "" if no expiry.
	// Used for time-sensitive learnings (API versions, library versions).
	ValidUntil string `json:"valid_until,omitempty"`

	// ExpiryStatus tracks the lifecycle state of the learning.
	// Values: "active" (default), "expired", "archived"
	ExpiryStatus ExpiryStatus `json:"expiry_status,omitempty"`

	// --- MemRL Utility Fields (ol-memrl) ---

	// Utility is the learned Q-value from the MemRL update rule.
	// Range: [0, 1]. Initial value: 0.5 (neutral).
	// Updated via: u_{t+1} = (1-α)·u_t + α·r
	Utility float64 `json:"utility,omitempty"`

	// LastReward is the most recent reward signal (0 or 1).
	LastReward float64 `json:"last_reward,omitempty"`

	// RewardCount tracks how many feedback events have occurred.
	// Used for confidence estimation.
	RewardCount int `json:"reward_count,omitempty"`

	// LastRewardAt is when the last reward was recorded.
	LastRewardAt time.Time `json:"last_reward_at,omitempty"`

	// --- CASS Maturity Fields (ol-cass) ---

	// Maturity is the lifecycle stage of this learning.
	// Values: "provisional" (new), "candidate" (promising), "established" (proven), "anti-pattern" (harmful)
	Maturity Maturity `json:"maturity,omitempty"`

	// Confidence is the certainty level in this learning's utility value.
	// Range: [0, 1]. Increases with more feedback, decays over time.
	// Formula: 1 - e^(-RewardCount/5) * decay_factor
	Confidence float64 `json:"confidence,omitempty"`

	// LastDecayAt is when confidence decay was last applied.
	// Used to calculate time-based decay on next access.
	LastDecayAt time.Time `json:"last_decay_at,omitempty"`

	// DecayCount tracks how many decay operations have been applied.
	// Used for diagnostics and to detect stale learnings.
	DecayCount int `json:"decay_count,omitempty"`

	// HelpfulCount tracks positive feedback events (helpful=true).
	HelpfulCount int `json:"helpful_count,omitempty"`

	// HarmfulCount tracks negative feedback events (harmful=true).
	HarmfulCount int `json:"harmful_count,omitempty"`
}

// RubricScores contains individual scoring dimensions.
type RubricScores struct {
	// Specificity measures named entities, concrete values (0.30 weight).
	Specificity float64 `json:"specificity"`

	// Actionability measures imperative verbs, clear steps (0.25 weight).
	Actionability float64 `json:"actionability"`

	// Novelty measures uniqueness vs. common knowledge (0.20 weight).
	Novelty float64 `json:"novelty"`

	// Context measures quality of surrounding context (0.15 weight).
	Context float64 `json:"context"`

	// Confidence measures assertion strength (0.10 weight).
	Confidence float64 `json:"confidence"`
}

// Scoring contains the full scoring result for a candidate.
type Scoring struct {
	// RawScore is the weighted sum of rubric scores (0.0-1.0).
	RawScore float64 `json:"raw_score"`

	// TierAssignment is the quality tier based on score thresholds.
	TierAssignment Tier `json:"tier_assignment"`

	// Rubric contains the individual dimension scores.
	Rubric RubricScores `json:"rubric"`

	// GateRequired indicates if human review is needed.
	GateRequired bool `json:"gate_required"`

	// ScoredAt is when scoring was performed.
	ScoredAt time.Time `json:"scored_at"`
}

// PoolStatus represents the status of a pool entry.
type PoolStatus string

const (
	// PoolStatusPending is awaiting processing.
	PoolStatusPending PoolStatus = "pending"

	// PoolStatusStaged is ready for promotion.
	PoolStatusStaged PoolStatus = "staged"

	// PoolStatusArchived has been archived to long-term storage.
	PoolStatusArchived PoolStatus = "archived"

	// PoolStatusRejected was rejected during review.
	PoolStatusRejected PoolStatus = "rejected"
)

// HumanReview contains the result of human review when required.
type HumanReview struct {
	// Reviewed indicates if human review has occurred.
	Reviewed bool `json:"reviewed"`

	// Approved indicates the review decision.
	Approved bool `json:"approved"`

	// Reviewer is who performed the review.
	Reviewer string `json:"reviewer,omitempty"`

	// Notes contains review comments.
	Notes string `json:"notes,omitempty"`

	// ReviewedAt is when the review occurred.
	ReviewedAt time.Time `json:"reviewed_at,omitempty"`
}

// PoolEntry represents an entry in a quality pool.
type PoolEntry struct {
	// Candidate is the knowledge candidate.
	Candidate Candidate `json:"candidate"`

	// ScoringResult contains the scoring details.
	ScoringResult Scoring `json:"scoring_result"`

	// HumanReview contains review results if required.
	HumanReview *HumanReview `json:"human_review,omitempty"`

	// Status is the current pool status.
	Status PoolStatus `json:"status"`

	// AddedAt is when this entry was added to the pool.
	AddedAt time.Time `json:"added_at"`

	// UpdatedAt is the last modification time.
	UpdatedAt time.Time `json:"updated_at"`
}

// Forge is the interface for knowledge extraction forges.
type Forge interface {
	// Name returns the forge identifier (e.g., "transcript").
	Name() string

	// Process extracts candidates from the input.
	Process(input io.Reader) ([]Candidate, error)

	// Validate checks if a candidate meets basic requirements.
	Validate(candidate Candidate) error
}

// --- Supersession constants and helpers (ol-a46.1.4) ---

const (
	// MaxSupersessionDepth is the maximum allowed depth for supersession chains.
	// Deeper chains cause expensive traversal and indicate poor knowledge hygiene.
	MaxSupersessionDepth = 3
)

// SupersessionError represents an error in supersession chain management.
type SupersessionError struct {
	Message string
	ChainID string
	Depth   int
}

func (e *SupersessionError) Error() string {
	return e.Message
}

// ValidateSupersessionDepth checks if a candidate's supersession depth is within limits.
// Returns an error if depth exceeds MaxSupersessionDepth.
func ValidateSupersessionDepth(c *Candidate) error {
	if c.SupersessionDepth > MaxSupersessionDepth {
		return &SupersessionError{
			Message: "supersession depth exceeds maximum (3): consider consolidating knowledge",
			ChainID: c.ID,
			Depth:   c.SupersessionDepth,
		}
	}
	return nil
}

// Supersede marks an older candidate as superseded by a newer one.
// Updates both forward (SupersededBy) and backward (Supersedes) references.
// Returns error if the new depth would exceed MaxSupersessionDepth.
func Supersede(older, newer *Candidate) error {
	// Calculate new depth
	newDepth := older.SupersessionDepth + 1
	if newDepth > MaxSupersessionDepth {
		return &SupersessionError{
			Message: "cannot supersede: would exceed max depth (3)",
			ChainID: older.ID,
			Depth:   newDepth,
		}
	}

	// Update the older candidate
	older.SupersededBy = newer.ID
	older.IsCurrent = false

	// Update the newer candidate
	newer.Supersedes = older.ID
	newer.IsCurrent = true
	newer.SupersessionDepth = newDepth

	return nil
}

// IsSuperseded returns true if this candidate has been replaced by a newer one.
func (c *Candidate) IsSuperseded() bool {
	return c.SupersededBy != ""
}

// --- Expiry types and helpers (ol-a46.1.5) ---

// ExpiryStatus represents the lifecycle state of a learning.
type ExpiryStatus string

const (
	// ExpiryStatusActive is the default state for learnings.
	ExpiryStatusActive ExpiryStatus = "active"

	// ExpiryStatusExpired indicates the valid_until date has passed.
	ExpiryStatusExpired ExpiryStatus = "expired"

	// ExpiryStatusArchived indicates the learning has been archived.
	ExpiryStatusArchived ExpiryStatus = "archived"
)

// --- CASS Maturity System (ol-cass) ---

// Maturity represents the lifecycle stage of a learning in the CASS system.
// Learnings progress through stages based on feedback and usage patterns.
type Maturity string

const (
	// MaturityProvisional is the initial stage for newly extracted learnings.
	// Learnings start here and need positive feedback to advance.
	MaturityProvisional Maturity = "provisional"

	// MaturityCandidate indicates a learning has received positive feedback
	// and is being considered for promotion to established.
	MaturityCandidate Maturity = "candidate"

	// MaturityEstablished indicates a learning has proven value through
	// consistent positive feedback and usage across multiple sessions.
	MaturityEstablished Maturity = "established"

	// MaturityAntiPattern indicates a learning that was consistently harmful
	// and should be surfaced as what NOT to do.
	MaturityAntiPattern Maturity = "anti-pattern"
)

// CASS maturity thresholds and parameters
const (
	// MaturityPromotionThreshold is the utility value needed to advance maturity.
	MaturityPromotionThreshold = 0.55

	// MaturityDemotionThreshold is the utility value below which demotion occurs.
	MaturityDemotionThreshold = 0.3

	// MaturityAntiPatternThreshold is when a learning becomes an anti-pattern.
	// Learnings with utility below this after multiple feedbacks become anti-patterns.
	MaturityAntiPatternThreshold = 0.2

	// MinFeedbackForPromotion is the minimum feedback count to consider promotion.
	MinFeedbackForPromotion = 3

	// MinFeedbackForAntiPattern is the minimum feedback count to mark as anti-pattern.
	MinFeedbackForAntiPattern = 3

	// ConfidenceDecayRate is the weekly decay rate for confidence (0.1 = 10%/week).
	// Lower than knowledge decay (0.17) because confidence decays slower.
	ConfidenceDecayRate = 0.1
)

// IsExpired checks if the candidate's valid_until date has passed.
// Supports both "2006-01-02" (date-only) and RFC3339 formats.
func (c *Candidate) IsExpired() bool {
	if c.ValidUntil == "" {
		return false
	}
	expiry, err := parseValidUntil(c.ValidUntil)
	if err != nil {
		return false
	}
	return time.Now().After(expiry)
}

// parseValidUntil parses a ValidUntil string in either date-only (2006-01-02) or RFC3339 format.
func parseValidUntil(s string) (time.Time, error) {
	// Try date-only format first (more common)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported date format: %s", s)
}

// UpdateExpiryStatus sets the expiry status based on valid_until date.
func (c *Candidate) UpdateExpiryStatus() {
	if c.ExpiryStatus == ExpiryStatusArchived {
		return // Don't change archived status
	}
	if c.IsExpired() {
		c.ExpiryStatus = ExpiryStatusExpired
	} else {
		c.ExpiryStatus = ExpiryStatusActive
	}
}

// --- Knowledge tier enforcement (ol-a46.1.7) ---

// KnowledgeTier defines MCP dependency level for knowledge operations.
type KnowledgeTier string

const (
	// KnowledgeTierStrict requires MCP to be available. Operations fail if MCP is down.
	KnowledgeTierStrict KnowledgeTier = "STRICT"

	// KnowledgeTierStandard is the default. Try MCP, continue on failure.
	KnowledgeTierStandard KnowledgeTier = "STANDARD"

	// KnowledgeTierMinimal skips MCP entirely. File artifacts only.
	KnowledgeTierMinimal KnowledgeTier = "MINIMAL"
)

// KnowledgeTierEnvVar is the environment variable name for tier configuration.
const KnowledgeTierEnvVar = "KNOWLEDGE_TIER"

// GetKnowledgeTier returns the configured knowledge tier from environment.
// Defaults to STANDARD if not set or invalid.
func GetKnowledgeTier() KnowledgeTier {
	tier := os.Getenv(KnowledgeTierEnvVar)
	switch KnowledgeTier(tier) {
	case KnowledgeTierStrict:
		return KnowledgeTierStrict
	case KnowledgeTierMinimal:
		return KnowledgeTierMinimal
	case KnowledgeTierStandard:
		return KnowledgeTierStandard
	default:
		return KnowledgeTierStandard
	}
}

// MCPRequired returns true if MCP operations must succeed.
func (t KnowledgeTier) MCPRequired() bool {
	return t == KnowledgeTierStrict
}

// MCPEnabled returns true if MCP operations should be attempted.
func (t KnowledgeTier) MCPEnabled() bool {
	return t != KnowledgeTierMinimal
}

// MCPError represents an error when MCP is unavailable.
type MCPError struct {
	Tier    KnowledgeTier
	Message string
}

func (e *MCPError) Error() string {
	return e.Message
}

// HandleMCPFailure decides how to handle an MCP operation failure.
// Returns error if tier is STRICT, nil otherwise (continue with fallback).
func HandleMCPFailure(tier KnowledgeTier, operation string, err error) error {
	if tier.MCPRequired() {
		return &MCPError{
			Tier:    tier,
			Message: fmt.Sprintf("MCP operation '%s' failed (STRICT mode): %v", operation, err),
		}
	}
	// STANDARD or MINIMAL: log and continue
	return nil
}

// TierBehavior describes how each tier handles MCP operations.
type TierBehavior struct {
	Tier        KnowledgeTier
	MCPRequired bool
	MCPEnabled  bool
	Description string
}

// GetTierBehaviors returns documentation for all tiers.
func GetTierBehaviors() []TierBehavior {
	return []TierBehavior{
		{
			Tier:        KnowledgeTierStrict,
			MCPRequired: true,
			MCPEnabled:  true,
			Description: "MCP required. Operations fail if MCP is unavailable.",
		},
		{
			Tier:        KnowledgeTierStandard,
			MCPRequired: false,
			MCPEnabled:  true,
			Description: "MCP optional. Try MCP, continue on failure. Default tier.",
		},
		{
			Tier:        KnowledgeTierMinimal,
			MCPRequired: false,
			MCPEnabled:  false,
			Description: "File artifacts only. Skip all MCP operations.",
		},
	}
}

// --- Citation tracking (ol-a46 Phase 0) ---

// CitationEvent records when an artifact is referenced in a session.
// These events drive the knowledge flywheel: σρ > δ/100 = operational escape velocity.
type CitationEvent struct {
	// ArtifactPath is the absolute path to the cited artifact.
	ArtifactPath string `json:"artifact_path"`

	// SessionID is the unique identifier for the session that cited the artifact.
	SessionID string `json:"session_id"`

	// CitedAt is when the citation occurred.
	CitedAt time.Time `json:"cited_at"`

	// CitationType indicates how the artifact was used.
	// Values: "retrieved" (surfaced via ol inject), "applied" (used in implementation),
	// "reference" (manual citation).
	CitationType string `json:"citation_type,omitempty"`

	// Query is the search query that surfaced this artifact (if applicable).
	Query string `json:"query,omitempty"`

	// --- MemRL Feedback Tracking (Phase 5) ---

	// FeedbackGiven indicates whether feedback has been recorded for this citation.
	// Used to prevent duplicate feedback for the same citation.
	FeedbackGiven bool `json:"feedback_given,omitempty"`

	// FeedbackReward is the reward signal recorded for this citation (0.0 - 1.0).
	// Only set when FeedbackGiven is true.
	FeedbackReward float64 `json:"feedback_reward,omitempty"`

	// UtilityBefore is the artifact's utility value before feedback was applied.
	// Enables tracking of utility changes over time.
	UtilityBefore float64 `json:"utility_before,omitempty"`

	// UtilityAfter is the artifact's utility value after feedback was applied.
	// UtilityAfter = (1-α)*UtilityBefore + α*FeedbackReward
	UtilityAfter float64 `json:"utility_after,omitempty"`

	// FeedbackAt is when the feedback was recorded.
	FeedbackAt time.Time `json:"feedback_at,omitempty"`
}

// --- Knowledge Flywheel Metrics (ol-a46 Phase 0) ---

// FlywheelMetrics captures the state of the knowledge flywheel equation:
// dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)
type FlywheelMetrics struct {
	// Timestamp is when these metrics were captured.
	Timestamp time.Time `json:"timestamp"`

	// PeriodStart is the beginning of the measurement period.
	PeriodStart time.Time `json:"period_start"`

	// PeriodEnd is the end of the measurement period.
	PeriodEnd time.Time `json:"period_end"`

	// --- Core Parameters ---

	// Delta is the average age in days of active knowledge.
	// The literature decay constant (0.17/week) still informs the model, but
	// operational flywheel checks compare sigma*rho against delta/100.
	Delta float64 `json:"delta"`

	// Sigma is retrieval coverage (0-1).
	// Measures what fraction of retrievable artifacts were surfaced in the
	// measurement window.
	Sigma float64 `json:"sigma"`

	// Rho is decision influence rate (0-1).
	// Measures what fraction of surfaced artifacts later received evidence-backed
	// use via "reference" or "applied" citations.
	Rho float64 `json:"rho"`

	// --- Derived Values ---

	// SigmaRho is σ × ρ, the effective retrieval-application rate.
	SigmaRho float64 `json:"sigma_rho"`

	// Velocity is σρ - δ/100, the net operational growth rate.
	// Positive = compounding, Negative = decaying.
	Velocity float64 `json:"velocity"`

	// AboveEscapeVelocity indicates if σρ > δ/100 (knowledge compounds).
	AboveEscapeVelocity bool `json:"above_escape_velocity"`

	// --- Counts ---

	// TotalArtifacts is the count of knowledge items (K).
	TotalArtifacts int `json:"total_artifacts"`

	// CitationsThisPeriod is the number of citation events in the period.
	CitationsThisPeriod int `json:"citations_this_period"`

	// UniqueCitedArtifacts is how many distinct artifacts were cited.
	UniqueCitedArtifacts int `json:"unique_cited_artifacts"`

	// NewArtifacts is I(t), new knowledge created this period.
	NewArtifacts int `json:"new_artifacts"`

	// StaleArtifacts is artifacts not cited in 90+ days.
	StaleArtifacts int `json:"stale_artifacts"`

	// --- Tier Distribution ---

	// TierCounts maps tier name to artifact count.
	TierCounts map[string]int `json:"tier_counts,omitempty"`

	// Promotions is count of tier promotions this period.
	Promotions int `json:"promotions,omitempty"`

	// Demotions is count of tier demotions this period.
	Demotions int `json:"demotions,omitempty"`

	// --- Loop Closure Metrics (R1 feedback) ---

	// LearningsCreated is artifacts written to .agents/learnings/ this period.
	LearningsCreated int `json:"learnings_created,omitempty"`

	// LearningsFound is unique learnings surfaced via search/inject this period.
	LearningsFound int `json:"learnings_found,omitempty"`

	// LoopClosureRatio is LearningsFound / LearningsCreated.
	// Values >1.0 = healthy (learnings reused more than once).
	// Values <1.0 = learnings not being discovered.
	LoopClosureRatio float64 `json:"loop_closure_ratio,omitempty"`

	// PriorArtBypasses is how many times users bypassed the prior art gate.
	PriorArtBypasses int `json:"prior_art_bypasses,omitempty"`

	// RetrosWithLearnings is post-mortems that created extractable learnings.
	RetrosWithLearnings int `json:"retros_with_learnings,omitempty"`

	// TotalRetros is total post-mortem count this period.
	TotalRetros int `json:"total_retros,omitempty"`

	// --- MemRL Utility Metrics (ol-memrl) ---

	// MeanUtility is the average utility across all artifacts.
	MeanUtility float64 `json:"mean_utility,omitempty"`

	// UtilityStdDev is the standard deviation of utilities.
	UtilityStdDev float64 `json:"utility_std_dev,omitempty"`

	// HighUtilityCount is artifacts with utility > 0.7.
	HighUtilityCount int `json:"high_utility_count,omitempty"`

	// LowUtilityCount is artifacts with utility < 0.3.
	LowUtilityCount int `json:"low_utility_count,omitempty"`

	// StigmergicScorecard captures repo-local prevention and backlog pressure.
	StigmergicScorecard *StigmergicScorecard `json:"stigmergic_scorecard,omitempty"`

	// GoldenSignals captures the four derived health indicators
	// that distinguish knowledge compounding from noise accumulation.
	GoldenSignals *GoldenSignals `json:"golden_signals,omitempty"`
}

// GoldenSignals captures the four derived health indicators
// that distinguish knowledge compounding from noise accumulation.
type GoldenSignals struct {
	// Signal 1: Compounding Velocity Trend
	// Is σρ-δ increasing over time?
	VelocityTrend7d  float64 `json:"velocity_trend_7d"`  // slope of 7d rolling velocity
	VelocityTrend30d float64 `json:"velocity_trend_30d"` // slope of 30d rolling velocity
	TrendVerdict     string  `json:"trend_verdict,omitempty"`

	// Signal 2: Citation Utility Pipeline
	// Are citations delivering value?
	HighUtilityCitationPct  float64 `json:"high_utility_citation_pct"`
	MedianUtilityDelta      float64 `json:"median_utility_delta"`
	AppliedToRetrievedRatio float64 `json:"applied_to_retrieved_ratio"`
	PipelineVerdict         string  `json:"pipeline_verdict,omitempty"`

	// Signal 3: Research-to-Learning Closure
	// Is research being mined?
	OrphanedResearchCount int     `json:"orphaned_research_count"`
	OrphanedResearchPct   float64 `json:"orphaned_research_pct"`
	AvgOrphanAgeDays      float64 `json:"avg_orphan_age_days"`
	ClosureVerdict        string  `json:"closure_verdict,omitempty"`

	// Signal 4: Knowledge Reuse Concentration
	// Is the whole pool active or just a few items?
	CitationGini         float64 `json:"citation_gini"`
	ActivePoolPct        float64 `json:"active_pool_pct"`
	Top10BottomRatio     float64 `json:"top10_bottom_ratio"`
	ConcentrationVerdict string  `json:"concentration_verdict,omitempty"`

	// Overall verdict: "compounding", "accumulating", or "decaying"
	OverallVerdict string `json:"overall_verdict,omitempty"`
}

// StigmergicScorecard captures the backlog and prevention surfaces that
// influence whether repo memory is being applied early enough to change work.
type StigmergicScorecard struct {
	PromotedFindings       int `json:"promoted_findings"`
	PlanningRules          int `json:"planning_rules"`
	PreMortemChecks        int `json:"pre_mortem_checks"`
	QueueEntries           int `json:"queue_entries"`
	UnconsumedBatches      int `json:"unconsumed_batches"`
	UnconsumedItems        int `json:"unconsumed_items"`
	HighSeverityUnconsumed int `json:"high_severity_unconsumed"`
}

// DefaultDelta is the literature-based decay rate (17%/week from Darr et al.).
const DefaultDelta = 0.17

// MemRL parameters (from MemRL paper, validated settings)
const (
	// DefaultAlpha is the EMA learning rate for utility updates.
	// α = 0.1 provides stable, slow learning.
	DefaultAlpha = 0.1

	// DefaultLambda weights utility vs similarity in scoring.
	// λ = 0.5 is balanced (validated by MemRL experiments).
	DefaultLambda = 0.5

	// InitialUtility is the starting Q-value for new artifacts.
	InitialUtility = 0.5
)

// EscapeVelocityStatus returns a human-readable status of the flywheel.
func (m *FlywheelMetrics) EscapeVelocityStatus() string {
	if m == nil {
		return "UNKNOWN"
	}
	if m.AboveEscapeVelocity {
		return "COMPOUNDING"
	}
	if m.Velocity > -0.05 {
		return "NEAR ESCAPE"
	}
	return "DECAYING"
}

// HealthStatus returns the user-facing flywheel health verdict.
// Golden-signal health takes priority over raw escape-velocity telemetry.
func (m *FlywheelMetrics) HealthStatus() string {
	if m == nil {
		return "UNKNOWN"
	}
	if m.GoldenSignals != nil {
		if verdict := strings.TrimSpace(m.GoldenSignals.OverallVerdict); verdict != "" {
			return strings.ToUpper(verdict)
		}
	}
	if m.EscapeVelocityStatus() == "NEAR ESCAPE" {
		return "ACCUMULATING"
	}
	return m.EscapeVelocityStatus()
}

// HealthCompounding reports whether the flywheel health verdict is compounding.
func (m *FlywheelMetrics) HealthCompounding() bool {
	if m == nil {
		return false
	}
	if m.GoldenSignals != nil && strings.TrimSpace(m.GoldenSignals.OverallVerdict) != "" {
		return strings.EqualFold(m.GoldenSignals.OverallVerdict, "compounding")
	}
	return m.AboveEscapeVelocity
}

// --- Plan Discovery (ol-a46.3) ---

// PlanStatus represents the lifecycle state of a plan.
type PlanStatus string

const (
	// PlanStatusActive is a plan that is currently in use.
	PlanStatusActive PlanStatus = "active"

	// PlanStatusCompleted means the plan was fully executed.
	PlanStatusCompleted PlanStatus = "completed"

	// PlanStatusAbandoned means the plan was not completed.
	PlanStatusAbandoned PlanStatus = "abandoned"

	// PlanStatusSuperseded means a newer plan replaced this one.
	PlanStatusSuperseded PlanStatus = "superseded"
)

// PlanManifestEntry represents a single plan in the manifest.
// Stored in .agents/plans/manifest.jsonl for robust plan discovery.
// Fixes G2 (fragile discovery), G4 (transcript parsing), G5 (hardcoded paths).
type PlanManifestEntry struct {
	// Path is the absolute or relative path to the plan file.
	// For ~/.claude/plans/: stores the full path.
	// For project plans: stores relative path from project root.
	Path string `json:"path"`

	// CreatedAt is when the plan was created.
	CreatedAt time.Time `json:"created_at"`

	// ProjectPath is the project/workspace the plan applies to.
	// Enables filtering plans by project context.
	ProjectPath string `json:"project_path,omitempty"`

	// PlanName is a human-readable name for the plan.
	// May be auto-generated (e.g., "peaceful-stirring-tome") or explicit.
	PlanName string `json:"plan_name,omitempty"`

	// Status tracks the plan lifecycle.
	Status PlanStatus `json:"status"`

	// BeadsID links to the epic/issue this plan implements (if any).
	// Enables beads → plan → implementation traceability.
	BeadsID string `json:"beads_id,omitempty"`

	// UpdatedAt is the last modification time.
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// Checksum is a hash of the plan content for integrity checking.
	// Enables detection of external modifications.
	Checksum string `json:"checksum,omitempty"`

	// SupersededBy is the path to a newer plan that replaces this one.
	SupersededBy string `json:"superseded_by,omitempty"`

	// Metadata contains additional plan metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}
