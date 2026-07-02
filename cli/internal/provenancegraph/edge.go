// Package provenancegraph is the write model for the SDLC provenance/intent
// graph (ag-x31t). It appends typed, evidence-backed provenance edges to the
// committed, per-record hash-chained ledger at docs/provenance/ledger.jsonl.
//
// Per CLAUDE.md and the council architecture
// (.agents/council/2026-05-30-debate-provenance-substrate.md) the committed
// JSONL ledger is the AUDIT authority and the source of truth; any Dolt
// provenance_edges table is a rebuildable projection and loses on disagreement.
// This package therefore writes the JSONL ledger directly.
//
// Edge events conform to schemas/agentops-sdlc-provenance.v1.schema.json and
// reuse the hashing discipline of cli/internal/rpi/ledger.go:
//
//	payload_hash = sha256(canonical JSON of every field EXCEPT
//	               prev_hash/payload_hash/hash)
//	hash         = sha256(payload_hash + "\n" + prev_hash)
//	prev_hash    = the previous record's hash, "" for the genesis record.
package provenancegraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is the const discriminator required by the v1 schema.
const SchemaVersion = "agentops-sdlc-provenance.v1"

// LedgerRelativePath is the repo-relative path to the committed provenance
// ledger. The committed JSONL is the audit authority.
const LedgerRelativePath = "docs/provenance/ledger.jsonl"

// NodeTypes is the closed set of edge-endpoint node types from the v1 schema.
var NodeTypes = []string{
	"decision", "artifact", "bead", "scenario", "commit",
	"pr", "ci_run", "verdict", "learning", "agent",
}

// Relations is the closed set of typed provenance relations from the v1
// schema, named with the W3C PROV-O / PROV-DM standard vocabulary so an
// external auditor recognizes the term (ag-lmdx.7). The enum enforces the
// standard verbs: a colloquial value such as "derives_from" or the prior
// AgentOps-local "artifact_derived_from" is rejected in favor of the PROV-O
// term "wasDerivedFrom". Mapping from the prior vocabulary:
// decision_produces_artifact->wasGeneratedBy, decision_authorizes->
// wasAssociatedWith, artifact_derived_from->wasDerivedFrom,
// scenario_covers_artifact->wasInformedBy, verdict_attests_artifact->
// wasAttributedTo, bead_scopes_decision->wasInfluencedBy,
// commit_implements_decision->wasRevisionOf, learning_revises_decision->
// wasInvalidatedBy.
var Relations = []string{
	"wasGeneratedBy",
	"wasAssociatedWith",
	"wasDerivedFrom",
	"wasInformedBy",
	"wasAttributedTo",
	"wasInfluencedBy",
	"wasRevisionOf",
	"wasInvalidatedBy",
}

// TrustTiers is the closed, monotonic set of trust tiers (authored > inferred
// > mined) from the v1 schema.
var TrustTiers = []string{"authored", "inferred", "mined"}

// Edge is one append-only provenance ledger event. Field order and json tags
// mirror schemas/agentops-sdlc-provenance.v1.schema.json exactly. The three
// hash fields are populated by Seal; callers supplying a new edge leave them
// empty.
type Edge struct {
	SchemaVersion string `json:"schema_version"`
	FromID        string `json:"from_id"`
	FromType      string `json:"from_type"`
	ToID          string `json:"to_id"`
	ToType        string `json:"to_type"`
	Relation      string `json:"relation"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	// BeadID and MergeSHA are additive, NON-payload mesh join keys (ag-5qltf,
	// epic ag-w0wr2). They denormalize the already-hashed from_id/to_id of a
	// bead→commit edge into the canonical (bead_id, merge_sha) join key the
	// yield↔provenance mesh joins on (bead_id is the universal key; merge_sha
	// anchors the bead→commit hop). Deliberately EXCLUDED from edgePayload: the
	// authoritative values are from_id/to_id, which the payload already covers,
	// so these projections need no independent hash protection — and excluding
	// them keeps every existing committed edge's payload_hash/VerifyChain intact.
	BeadID    string `json:"bead_id,omitempty"`
	MergeSHA  string `json:"merge_sha,omitempty"`
	TrustTier string `json:"trust_tier"`
	TS        string `json:"ts"`
	// v1.1 verdict-record enrichment (age-rk3r.3) — five OPTIONAL, additive
	// fields carrying reviewer metadata for verdict edges (the cost-of-verified-
	// done substrate; failover label; the receipts' structured evidence source).
	// UNLIKE the bead_id/merge_sha join keys above, these ARE part of edgePayload
	// and therefore hash-PROTECTED: a record that sets any of them has it covered
	// by payload_hash, so it is tamper-evident. Backward compatibility rests on
	// omitempty — a record predating these fields (or leaving them at their zero
	// value) omits them from the payload JSON entirely, so its payload_hash is
	// byte-identical to the pre-v1.1 layout and VerifyChain stays intact across the
	// whole committed history. "v1.1" is a DOCUMENTATION label only: SchemaVersion
	// is UNCHANGED, and consumers branch on field PRESENCE, never a version string.
	//
	// COMPATIBILITY BOUNDARY (documented, load-bearing): because the fields are IN
	// the payload, an OLDER ao binary that predates them unmarshals a v1.1 record
	// into a struct that DROPS them, recomputes the payload WITHOUT them, and so
	// reports a spurious payload_hash mismatch (a false "broken chain") on v1.1
	// records — while still verifying every pre-v1.1 record. A reader must be at or
	// above the version that knows these fields to verify v1.1 records; the
	// installed-hook ao-version floor is a separate bead (.6).
	ReviewerFamily string  `json:"reviewer_family,omitempty"`
	Degraded       bool    `json:"degraded,omitempty"`
	Rounds         int     `json:"rounds,omitempty"`
	DurationS      float64 `json:"duration_s,omitempty"`
	EvidencePath   string  `json:"evidence_path,omitempty"`
	PrevHash       string  `json:"prev_hash"`
	PayloadHash    string  `json:"payload_hash"`
	Hash           string  `json:"hash"`
}

// edgePayload is the hash-input subset of an Edge: every field EXCEPT
// prev_hash/payload_hash/hash AND the additive non-payload join keys
// (bead_id, merge_sha — denormalized projections of from_id/to_id, see Edge).
// The v1.1 enrichment fields (reviewer_family, degraded, rounds, duration_s,
// evidence_path) ARE included here — they are hash-protected, and their
// omitempty tags keep pre-v1.1 records byte-identical (all zero => omitted from
// the canonical JSON, so payload_hash is unchanged over history). Field order is
// fixed so the canonical JSON is deterministic.
type edgePayload struct {
	SchemaVersion  string  `json:"schema_version"`
	FromID         string  `json:"from_id"`
	FromType       string  `json:"from_type"`
	ToID           string  `json:"to_id"`
	ToType         string  `json:"to_type"`
	Relation       string  `json:"relation"`
	EvidenceRef    string  `json:"evidence_ref,omitempty"`
	TrustTier      string  `json:"trust_tier"`
	TS             string  `json:"ts"`
	ReviewerFamily string  `json:"reviewer_family,omitempty"`
	Degraded       bool    `json:"degraded,omitempty"`
	Rounds         int     `json:"rounds,omitempty"`
	DurationS      float64 `json:"duration_s,omitempty"`
	EvidencePath   string  `json:"evidence_path,omitempty"`
}

// inSet reports whether v is a member of allowed.
func inSet(v string, allowed []string) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}

// ValidateFields checks the non-hash fields of an edge against the v1 schema's
// enumerations and required-field constraints. It does NOT check the hash
// chain (see VerifyChain) or set any defaults.
func ValidateFields(e Edge) error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q, got %q", SchemaVersion, e.SchemaVersion)
	}
	required := []struct {
		val, name string
	}{
		{e.FromID, "from_id"},
		{e.ToID, "to_id"},
	}
	for _, f := range required {
		if strings.TrimSpace(f.val) == "" {
			return fmt.Errorf("%s is required", f.name)
		}
	}
	if !inSet(e.FromType, NodeTypes) {
		return fmt.Errorf("from_type %q is not a valid node_type", e.FromType)
	}
	if !inSet(e.ToType, NodeTypes) {
		return fmt.Errorf("to_type %q is not a valid node_type", e.ToType)
	}
	if !inSet(e.Relation, Relations) {
		return fmt.Errorf("relation %q is not a valid relation", e.Relation)
	}
	if !inSet(e.TrustTier, TrustTiers) {
		return fmt.Errorf("trust_tier %q is not one of authored|inferred|mined", e.TrustTier)
	}
	if err := validateTimestamp(e.TS); err != nil {
		return err
	}
	return nil
}

// validateTimestamp requires a UTC RFC3339/RFC3339Nano timestamp, mirroring
// the rpi ledger's UTC-only discipline.
func validateTimestamp(ts string) error {
	if strings.TrimSpace(ts) == "" {
		return fmt.Errorf("ts is required")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return fmt.Errorf("invalid ts: %w", err)
	}
	if t.Location() != time.UTC && t.UTC().Format(time.RFC3339Nano) != ts {
		return fmt.Errorf("ts must be UTC RFC3339")
	}
	return nil
}

// hashHex returns the lowercase hex sha256 of data.
func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ComputeHashes derives payload_hash and hash for an edge given its prev_hash,
// reusing the cli/internal/rpi/ledger.go chaining semantics.
func ComputeHashes(e Edge) (payloadHash, hash string, err error) {
	payload := edgePayload{
		SchemaVersion:  e.SchemaVersion,
		FromID:         e.FromID,
		FromType:       e.FromType,
		ToID:           e.ToID,
		ToType:         e.ToType,
		Relation:       e.Relation,
		EvidenceRef:    e.EvidenceRef,
		TrustTier:      e.TrustTier,
		TS:             e.TS,
		ReviewerFamily: e.ReviewerFamily,
		Degraded:       e.Degraded,
		Rounds:         e.Rounds,
		DurationS:      e.DurationS,
		EvidencePath:   e.EvidencePath,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal payload: %w", err)
	}
	payloadHash = hashHex(b)
	hash = hashHex([]byte(payloadHash + "\n" + e.PrevHash))
	return payloadHash, hash, nil
}

// Seal validates the edge's fields, links it onto prevHash, and fills the
// payload_hash and hash fields. The schema_version is forced to the const so
// callers cannot accidentally emit a mis-versioned record. Returns a fully
// sealed, schema-valid edge ready to append.
func Seal(e Edge, prevHash string) (Edge, error) {
	e.SchemaVersion = SchemaVersion
	if err := ValidateFields(e); err != nil {
		return Edge{}, err
	}
	e.PrevHash = prevHash
	payloadHash, hash, err := ComputeHashes(e)
	if err != nil {
		return Edge{}, err
	}
	e.PayloadHash = payloadHash
	e.Hash = hash
	return e, nil
}

// EdgeIdentity returns the dedupe key for an edge: the tuple that makes two
// edges "the same" for idempotency purposes (everything but ts and the chain
// hashes). Two edges with the same identity are duplicates regardless of when
// they were recorded.
func EdgeIdentity(e Edge) string {
	parts := []string{
		e.FromID, e.FromType, e.ToID, e.ToType,
		e.Relation, e.EvidenceRef, e.TrustTier,
	}
	return strings.Join(parts, "\x00")
}

// VerifyChain validates that records form an intact hash chain: each record is
// field-valid, its prev_hash links to the prior record's hash, and its
// payload_hash/hash recompute. Returns the index (1-based) of the first broken
// record and a descriptive error, or (0, nil) when the whole chain verifies.
func VerifyChain(records []Edge) (int, error) {
	prevHash := ""
	for i, rec := range records {
		if err := ValidateFields(rec); err != nil {
			return i + 1, fmt.Errorf("record %d: %w", i+1, err)
		}
		if rec.PrevHash != prevHash {
			return i + 1, fmt.Errorf("record %d: prev_hash mismatch: got %q want %q", i+1, rec.PrevHash, prevHash)
		}
		payloadHash, hash, err := ComputeHashes(rec)
		if err != nil {
			return i + 1, fmt.Errorf("record %d: %w", i+1, err)
		}
		if rec.PayloadHash != payloadHash {
			return i + 1, fmt.Errorf("record %d: payload_hash mismatch", i+1)
		}
		if rec.Hash != hash {
			return i + 1, fmt.Errorf("record %d: hash mismatch", i+1)
		}
		prevHash = rec.Hash
	}
	return 0, nil
}

// SortedNodeTypes returns the node types sorted for stable help/error text.
func SortedNodeTypes() []string {
	out := append([]string(nil), NodeTypes...)
	sort.Strings(out)
	return out
}
