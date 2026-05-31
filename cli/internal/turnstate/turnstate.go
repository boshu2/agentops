// Package turnstate is the state-machine PROJECTION half of the ag-lmdx
// "event-log canonical, state is a rebuildable projection" inversion. Where
// cli/internal/provenancegraph is the write model for the provenance *graph*
// and cli/internal/drrebuild proves the *graph* rebuilds from the ledger, this
// package establishes the same invariant for an artifact's lifecycle *state*:
//
//	context_artifact.state is NOT authored. It is a FOLD over the
//	append-only, hash-chained state_transition log.
//
// The parent epic (ag-lmdx) frames the unit of work as an "Evidenced-Turn
// state machine": an artifact advances through lifecycle states only by
// appending an evidence-backed Transition to its log. The current state of any
// artifact is then `Fold(log)` — derived by replaying transitions in canonical
// order — never an independently-written column. This makes the
// author!=judge and orphan-gate invariants true *by construction*: there is no
// state to corrupt out-of-band, only a log to append to and replay.
//
// Hash discipline mirrors cli/internal/rpi/ledger.go ComputeLedgerHashes and
// cli/internal/provenancegraph exactly, so a Transition chain is verifiable by
// the same rules the rest of the provenance substrate uses:
//
//	payload_hash = sha256(canonical JSON of every field EXCEPT
//	               prev_hash/payload_hash/hash)
//	hash         = sha256(payload_hash + "\n" + prev_hash)
//	prev_hash    = the previous record's hash, "" for the genesis record.
//
// This package is deliberately a pure, in-memory write+fold core (no Dolt, no
// file I/O): it is the testable kernel that proves the projection is faithful
// and deterministic. Wiring it to a durable ledger and forbidding direct
// context_artifact.state writes at every existing call site is a follow-up
// migration (a bounded slice beats a sprawling refactor — see ag-lmdx).
package turnstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is the const discriminator stamped on every transition so the
// log is self-describing and a mis-versioned record fails validation.
const SchemaVersion = "agentops-turnstate.v1"

// InitialState is the sentinel "before any transition" state. A genesis
// transition for an artifact must declare FromState == InitialState; folding an
// empty log yields this state.
const InitialState = ""

// Transition is one append-only state_transition event: an artifact moves from
// FromState to ToState, backed by evidence, at TS. The three hash fields are
// populated by Seal; callers supplying a new transition leave them empty.
//
// Field order and json tags are fixed so the canonical JSON used for hashing is
// deterministic and byte-stable across runs and machines.
type Transition struct {
	SchemaVersion string `json:"schema_version"`
	ArtifactID    string `json:"artifact_id"`
	FromState     string `json:"from_state"`
	ToState       string `json:"to_state"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	TS            string `json:"ts"`
	PrevHash      string `json:"prev_hash"`
	PayloadHash   string `json:"payload_hash"`
	Hash          string `json:"hash"`
}

// transitionPayload is the hash-input subset of a Transition: every field
// EXCEPT prev_hash (which enters only via the outer hash) and the two derived
// hash fields. Field order is fixed for deterministic canonical JSON.
type transitionPayload struct {
	SchemaVersion string `json:"schema_version"`
	ArtifactID    string `json:"artifact_id"`
	FromState     string `json:"from_state"`
	ToState       string `json:"to_state"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	TS            string `json:"ts"`
}

// hashHex returns the lowercase hex sha256 of data. Mirrors the rpi ledger and
// provenancegraph helpers so chains are cross-verifiable by the same rule.
func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ComputeHashes derives payload_hash and hash for a transition given its
// prev_hash, reusing the rpi/ledger.go + provenancegraph chaining semantics.
func ComputeHashes(t Transition) (payloadHash, hash string, err error) {
	payload := transitionPayload{
		SchemaVersion: t.SchemaVersion,
		ArtifactID:    t.ArtifactID,
		FromState:     t.FromState,
		ToState:       t.ToState,
		EvidenceRef:   t.EvidenceRef,
		TS:            t.TS,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal payload: %w", err)
	}
	payloadHash = hashHex(b)
	hash = hashHex([]byte(payloadHash + "\n" + t.PrevHash))
	return payloadHash, hash, nil
}

// ValidateFields checks a transition's non-hash fields. ToState must be
// non-empty (you cannot transition *to* the genesis sentinel), ArtifactID is
// required, the schema version must match, and TS must be UTC RFC3339. It does
// NOT check the hash chain (see VerifyChain) or fold ordering (see Fold).
func ValidateFields(t Transition) error {
	if t.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q, got %q", SchemaVersion, t.SchemaVersion)
	}
	if strings.TrimSpace(t.ArtifactID) == "" {
		return fmt.Errorf("artifact_id is required")
	}
	if strings.TrimSpace(t.ToState) == "" {
		return fmt.Errorf("to_state is required (cannot transition to the genesis state)")
	}
	return validateTimestamp(t.TS)
}

// validateTimestamp requires a UTC RFC3339/RFC3339Nano timestamp, matching the
// UTC-only discipline of the rpi ledger and provenancegraph.
func validateTimestamp(ts string) error {
	if strings.TrimSpace(ts) == "" {
		return fmt.Errorf("ts is required")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return fmt.Errorf("invalid ts: %w", err)
	}
	if t.UTC().Format(time.RFC3339Nano) != ts {
		return fmt.Errorf("ts must be UTC RFC3339")
	}
	return nil
}

// Seal validates the transition's fields, links it onto prevHash, and fills the
// payload_hash and hash fields. The schema_version is forced to the const so a
// caller cannot accidentally emit a mis-versioned record. This is the ONLY
// supported way to add a transition to a chain — there is no setter for the
// state itself, which is the point: state changes flow exclusively through an
// appended, hash-linked transition.
func Seal(t Transition, prevHash string) (Transition, error) {
	t.SchemaVersion = SchemaVersion
	if err := ValidateFields(t); err != nil {
		return Transition{}, err
	}
	t.PrevHash = prevHash
	payloadHash, hash, err := ComputeHashes(t)
	if err != nil {
		return Transition{}, err
	}
	t.PayloadHash = payloadHash
	t.Hash = hash
	return t, nil
}

// Append seals newTransition onto the tip of an existing, already-sealed log
// and returns the extended log. The input log is not mutated. This is the
// canonical append entry point: it derives prev_hash from the current tip so a
// caller never hand-links the chain.
func Append(log []Transition, newTransition Transition) ([]Transition, error) {
	prevHash := ""
	if len(log) > 0 {
		prevHash = log[len(log)-1].Hash
	}
	sealed, err := Seal(newTransition, prevHash)
	if err != nil {
		return nil, err
	}
	out := make([]Transition, len(log)+1)
	copy(out, log)
	out[len(log)] = sealed
	return out, nil
}

// VerifyChain validates that a single artifact's transitions form an intact
// hash chain: each record is field-valid, its prev_hash links to the prior
// record's hash, and its payload_hash/hash recompute. Returns the 1-based index
// of the first broken record and a descriptive error, or (0, nil) when the
// whole chain verifies. Mirrors provenancegraph.VerifyChain.
func VerifyChain(log []Transition) (int, error) {
	prevHash := ""
	for i, rec := range log {
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

// FoldError reports a transition whose FromState does not match the artifact's
// current folded state — i.e. an attempt to apply a transition out of order or
// from a stale state. This is the in-band guard that replaces "trust the
// state column": the only legal next state is the one the log produces.
type FoldError struct {
	ArtifactID string
	Index      int    // 1-based index within the artifact's transition slice
	Want       string // the current folded state the transition should have declared
	Got        string // the FromState the transition actually declared
}

func (e *FoldError) Error() string {
	return fmt.Sprintf("artifact %q transition %d: from_state %q does not match current state %q",
		e.ArtifactID, e.Index, e.Got, e.Want)
}

// Fold derives the current state of every artifact by replaying the append-only
// transition log. It is the pure projection function at the heart of the
// inversion: given the log, state is computed, never read.
//
// Determinism: transitions are grouped by artifact_id and replayed in the
// canonical order (ts, then hash as a total-order tie-breaker), so the result
// is identical for the same set of transitions regardless of input slice order.
//
// In-band guard: within each artifact, every transition's FromState must equal
// the state produced so far (genesis must declare InitialState). A mismatch is
// a FoldError — there is no way to "jump" the state without a contiguous,
// evidence-backed transition, which is how the state-machine invariants become
// true by construction. Fold does NOT verify the hash chain; callers that need
// tamper-evidence run VerifyChain first (FoldVerified does both).
func Fold(log []Transition) (map[string]string, error) {
	byArtifact := groupByArtifact(log)
	states := make(map[string]string, len(byArtifact))
	for _, id := range sortedKeys(byArtifact) {
		transitions := canonicalSort(byArtifact[id])
		current := InitialState
		for i, t := range transitions {
			if t.FromState != current {
				return nil, &FoldError{
					ArtifactID: id,
					Index:      i + 1,
					Want:       current,
					Got:        t.FromState,
				}
			}
			current = t.ToState
		}
		states[id] = current
	}
	return states, nil
}

// FoldVerified verifies each artifact's hash chain (in canonical replay order)
// and then folds. It is the integrity-checked projection: a tampered or broken
// chain fails before any state is derived, so a faithful rebuild is provable
// rather than hoped-for. Returns the folded states or the first failure.
func FoldVerified(log []Transition) (map[string]string, error) {
	byArtifact := groupByArtifact(log)
	for _, id := range sortedKeys(byArtifact) {
		ordered := canonicalSort(byArtifact[id])
		if idx, err := VerifyChain(ordered); err != nil {
			return nil, fmt.Errorf("artifact %q: %w (broken at %d)", id, err, idx)
		}
	}
	return Fold(log)
}

// StateOf folds the log and returns the current state of one artifact, plus
// whether that artifact appears in the log at all. A known artifact whose log
// is empty is reported as not-found rather than InitialState, so callers can
// distinguish "never transitioned" from "absent".
func StateOf(log []Transition, artifactID string) (state string, found bool, err error) {
	states, err := Fold(log)
	if err != nil {
		return "", false, err
	}
	s, ok := states[artifactID]
	return s, ok, nil
}

// groupByArtifact buckets transitions by artifact_id, preserving input order
// within each bucket (canonicalSort imposes the replay order afterward).
func groupByArtifact(log []Transition) map[string][]Transition {
	out := make(map[string][]Transition)
	for _, t := range log {
		out[t.ArtifactID] = append(out[t.ArtifactID], t)
	}
	return out
}

// sortedKeys returns the map keys sorted, so Fold iterates artifacts in a
// stable order (the per-artifact state is order-independent, but stable
// iteration keeps any derived output deterministic).
func sortedKeys(m map[string][]Transition) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// canonicalSort returns a new slice ordered by (ts, hash). TS is the primary
// replay order; hash is a total-order tie-breaker so transitions sharing a
// timestamp still replay deterministically. The input is not mutated.
func canonicalSort(transitions []Transition) []Transition {
	out := make([]Transition, len(transitions))
	copy(out, transitions)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TS != out[j].TS {
			return out[i].TS < out[j].TS
		}
		return out[i].Hash < out[j].Hash
	})
	return out
}
