package rpi

import (
	"errors"
	"regexp"
	"time"
)

// ErrQueueClaimConflict signals that a next-work item is no longer available.
var ErrQueueClaimConflict = errors.New("next-work item no longer available for this consumer")

var (
	// QueueProofTargetPattern matches bead-style IDs in free text.
	QueueProofTargetPattern = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*-[A-Za-z0-9][A-Za-z0-9-]*(?:\.[0-9]+)?\b`)
	// QueueProofPacketPathPattern extracts target IDs from evidence-only closure paths.
	QueueProofPacketPathPattern = regexp.MustCompile(`\.agents/(?:releases|council)/evidence-only-closures/([^/\s]+)\.json`)
)

// NextWorkEntry represents one line in next-work.jsonl.
type NextWorkEntry struct {
	SourceEpic           string         `json:"source_epic"`
	Timestamp            string         `json:"timestamp"`
	Items                []NextWorkItem `json:"items,omitempty"`
	Consumed             bool           `json:"consumed"`
	ClaimStatus          string         `json:"claim_status,omitempty"`
	ClaimedBy            *string        `json:"claimed_by,omitempty"`
	ClaimedAt            *string        `json:"claimed_at,omitempty"`
	ConsumedBy           *string        `json:"consumed_by"`
	ConsumedAt           *string        `json:"consumed_at"`
	FailedAt             *string        `json:"failed_at,omitempty"`
	CompletionEvidence   string         `json:"completion_evidence,omitempty"`
	CompletionEvidenceAt *string        `json:"completion_evidence_at,omitempty"`
	LegacyID             string         `json:"id,omitempty"`
	CreatedAt            string         `json:"created_at,omitempty"`
	Title                string         `json:"title,omitempty"`
	Type                 string         `json:"type,omitempty"`
	Severity             string         `json:"severity,omitempty"`
	Source               string         `json:"source,omitempty"`
	Description          string         `json:"description,omitempty"`
	Evidence             string         `json:"evidence,omitempty"`
	TargetRepo           string         `json:"target_repo,omitempty"`
	QueueIndex           int            `json:"-"`
}

// NextWorkProofRef holds a typed reference to completion proof.
type NextWorkProofRef struct {
	Kind     string `json:"kind"`
	TargetID string `json:"target_id,omitempty"`
	RunID    string `json:"run_id,omitempty"`
	Path     string `json:"path,omitempty"`
}

// NextWorkItem represents a single harvested work item.
type NextWorkItem struct {
	ID          string            `json:"id,omitempty"`
	Title       string            `json:"title"`
	Type        string            `json:"type"`
	Severity    string            `json:"severity"`
	Source      string            `json:"source"`
	Description string            `json:"description"`
	Evidence    string            `json:"evidence,omitempty"`
	TargetRepo  string            `json:"target_repo,omitempty"`
	File        string            `json:"file,omitempty"`
	Func        string            `json:"func,omitempty"`
	SourcePath  string            `json:"source_path,omitempty"`
	ProofRef    *NextWorkProofRef `json:"proof_ref,omitempty"`
	Confidence  string            `json:"confidence,omitempty"`
	WhyNow      string            `json:"why_now,omitempty"`
	TargetFiles []string          `json:"target_files,omitempty"`
	LikelyTests []string          `json:"likely_tests,omitempty"`
	MorningCmd  string            `json:"morning_command,omitempty"`
	PacketPath  string            `json:"packet_path,omitempty"`
	BeadID      string            `json:"bead_id,omitempty"`
	Consumed    bool              `json:"consumed,omitempty"`
	ClaimStatus string            `json:"claim_status,omitempty"`
	ClaimedBy   *string           `json:"claimed_by,omitempty"`
	ClaimedAt   *string           `json:"claimed_at,omitempty"`
	ConsumedBy  *string           `json:"consumed_by,omitempty"`
	ConsumedAt  *string           `json:"consumed_at,omitempty"`
	FailedAt    *string           `json:"failed_at,omitempty"`
}

// QueueSelection holds the selected item together with its source entry index
// so the caller can mark the correct entry consumed/failed.
type QueueSelection struct {
	Item       NextWorkItem
	EntryIndex int // 0-based index among parseable JSON entries in next-work.jsonl
	ItemIndex  int // index of the selected item within the entry
	SourceEpic string
	ClaimedBy  string
}

// QueuePreflightDecision is the outcome of a queue preflight check.
type QueuePreflightDecision struct {
	Consume bool
	Reason  string
}

// NextWorkProofDecision is the outcome of completion-proof classification.
type NextWorkProofDecision struct {
	Complete bool
	Source   string
	Detail   string
}

// EvidenceOnlyClosureProof holds a matched evidence-only closure.
type EvidenceOnlyClosureProof struct {
	TargetID   string
	PacketPath string
}

// EvidenceOnlyClosurePacket is the JSON structure of an evidence-only closure file.
type EvidenceOnlyClosurePacket struct {
	TargetID     string `json:"target_id"`
	EvidenceMode string `json:"evidence_mode"`
	Evidence     struct {
		Artifacts []string `json:"artifacts"`
	} `json:"evidence"`
}

// LoopCycleResult signals the loop iteration outcome.
type LoopCycleResult int

const (
	LoopContinue LoopCycleResult = iota
	LoopBreak
	LoopReturn
)

// CompileProducerState tracks the last compile producer tick time.
type CompileProducerState struct {
	LastTick time.Time
}
