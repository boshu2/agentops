// practices: [in-toto-provenance, adr]

// Package drwitness is the CI WITNESS cross-check for the ag-lmdx
// event-log-canonical design (ag-lmdx.3). It is the watcher that watches the
// watcher: it re-derives the git-committed, hash-chained JSONL ledger
// *from the Dolt projection* and hash-compares against the committed witness,
// proving Dolt is a faithful rendering of the canonical event log.
//
// Why this complements drrebuild (ag-lmdx.3's sibling, #646):
//
//   - drrebuild proves you can rebuild the Dolt context graph FROM the witness
//     (JSONL + git blobs). That defends "the live store is a rebuildable
//     projection."
//   - drwitness proves the inverse direction: that re-deriving the witness FROM
//     the live Dolt rows reproduces the committed, git-anchored chain. That
//     defends "Dolt has not silently diverged from the canonical log."
//
// Together they pin both ends of the projection. The thesis (CLAUDE.md):
// "Dolt history is rewritable (reset/--force/root) so it is NOT tamper-evidence.
// The git-anchored JSONL witness is." Git and Dolt have different write
// semantics; a CI gate that re-derives one from the other and hash-compares is
// the only way to catch a Dolt rewrite that did not also rewrite the committed
// witness.
//
// Hash discipline is identical to the real schema and to drrebuild:
// payload_hash = sha256(canonical-JSON of every field except
// prev_hash/payload_hash/hash); hash = sha256(payload_hash + "\n" + prev_hash);
// prev_hash links the previous record (empty for genesis). The Dolt projection
// is rows WITHOUT the export-time chain fields (Dolt is the live, queryable
// projection — sealing into a chain is an export operation), so the witness
// re-derives the chain deterministically via canonical ordering + re-seal. Two
// runs over the same Dolt rows in any order produce a byte-identical witness;
// that determinism is what makes the hash-compare meaningful.
package drwitness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/drrebuild"
)

// DoltRow is one row of the Dolt provenance_edges projection: the canonical
// edge payload WITHOUT the export-time hash-chain fields. Field tags match the
// schema so a Dolt CSV/JSONL export parses directly. Dolt holds the live,
// queryable projection; the hash chain is sealed only at git-export time, which
// is why a row carries no prev_hash/payload_hash/hash.
type DoltRow struct {
	SchemaVersion string `json:"schema_version"`
	FromID        string `json:"from_id"`
	FromType      string `json:"from_type"`
	ToID          string `json:"to_id"`
	ToType        string `json:"to_type"`
	Relation      string `json:"relation"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	TrustTier     string `json:"trust_tier"`
	TS            string `json:"ts"`
}

// toLedgerEvent lifts a Dolt row into a chain-field-empty LedgerEvent ready for
// re-sealing.
func (r DoltRow) toLedgerEvent() drrebuild.LedgerEvent {
	return drrebuild.LedgerEvent{
		SchemaVersion: r.SchemaVersion,
		FromID:        r.FromID,
		FromType:      r.FromType,
		ToID:          r.ToID,
		ToType:        r.ToType,
		Relation:      r.Relation,
		EvidenceRef:   r.EvidenceRef,
		TrustTier:     r.TrustTier,
		TS:            r.TS,
	}
}

// ParseDoltRows reads a Dolt-export JSONL stream (one DoltRow per line). Blank
// lines are skipped. This is the hermetic stand-in for `dolt sql -r json` /
// `dolt table export provenance_edges` output the live gate would consume.
func ParseDoltRows(r io.Reader) ([]DoltRow, error) {
	var rows []DoltRow
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var row DoltRow
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			return nil, fmt.Errorf("dolt row line %d: %w", line, err)
		}
		if row.FromID == "" || row.ToID == "" {
			return nil, fmt.Errorf("dolt row line %d: edge missing from_id/to_id", line)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading dolt rows: %w", err)
	}
	return rows, nil
}

// canonicalLess gives the Dolt rows a total order so the re-derived chain is
// independent of Dolt's row iteration order. The tuple mirrors
// provenancegraph.CanonicalLess (ts, from_id, to_id, relation, then the
// remaining fields as tie-breakers) so the witness orders edges the same way an
// `ao provenance export` would.
func canonicalLess(a, b drrebuild.LedgerEvent) bool {
	switch {
	case a.TS != b.TS:
		return a.TS < b.TS
	case a.FromID != b.FromID:
		return a.FromID < b.FromID
	case a.ToID != b.ToID:
		return a.ToID < b.ToID
	case a.Relation != b.Relation:
		return a.Relation < b.Relation
	case a.FromType != b.FromType:
		return a.FromType < b.FromType
	case a.ToType != b.ToType:
		return a.ToType < b.ToType
	case a.TrustTier != b.TrustTier:
		return a.TrustTier < b.TrustTier
	default:
		return a.EvidenceRef < b.EvidenceRef
	}
}

// ReDeriveWitness re-seals the Dolt rows into a fresh, intact hash chain in
// canonical order, reproducing the witness JSONL the way an export would. Each
// event's prev_hash links the prior recomputed hash (genesis prev_hash = "")
// and payload_hash/hash are computed via the schema discipline (drrebuild's
// ComputeEventHashes). Identical rows in any order produce a byte-identical
// result — the determinism the hash-compare relies on.
func ReDeriveWitness(rows []DoltRow) []drrebuild.LedgerEvent {
	events := make([]drrebuild.LedgerEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, row.toLedgerEvent())
	}
	sort.SliceStable(events, func(i, j int) bool { return canonicalLess(events[i], events[j]) })

	prev := ""
	out := make([]drrebuild.LedgerEvent, 0, len(events))
	for _, ev := range events {
		ev.PrevHash = prev
		ph, h := drrebuild.ComputeEventHashes(ev)
		ev.PayloadHash = ph
		ev.Hash = h
		prev = h
		out = append(out, ev)
	}
	return out
}

// SerializeWitness renders events as canonical JSONL (one compact JSON object
// per line, schema field order, trailing newline per record). This is the exact
// byte form the committed witness ledger is stored in, so the output can be
// compared byte-for-byte against the git-committed file.
func SerializeWitness(events []drrebuild.LedgerEvent) ([]byte, error) {
	var buf bytes.Buffer
	for i, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			return nil, fmt.Errorf("marshal event %d (%s->%s): %w", i, ev.FromID, ev.ToID, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// ChainHead returns the final record hash (the chain tip) of a witness, or ""
// for an empty witness. The head is a single value that summarizes the entire
// chain: if any record changed, the head changes. Comparing heads is the
// cheap "did Dolt diverge from the committed log" check.
func ChainHead(events []drrebuild.LedgerEvent) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].Hash
}

// CrossCheckResult reports the outcome of a Dolt->JSONL re-derive cross-check.
type CrossCheckResult struct {
	Match            bool
	RederivedHead    string
	CommittedHead    string
	BytesMatch       bool
	CommittedChainOK bool
	Detail           string
}

// CrossCheck is the witness gate's core: re-derive the witness from Dolt rows,
// verify the COMMITTED witness chain is internally intact (a tampered witness is
// itself untrustworthy), then hash-compare the re-derived chain head against the
// committed head AND byte-compare the full JSONL. A faithful Dolt projection
// passes both; any divergence (a rewritten Dolt row, a reordered/edited witness)
// fails. The committed JSONL is the git-anchored authority; ledger wins on
// disagreement, so a re-derive that does not reproduce it means Dolt diverged.
func CrossCheck(doltRows []DoltRow, committedJSONL []byte) (CrossCheckResult, error) {
	committed, err := drrebuild.ParseLedger(bytes.NewReader(committedJSONL))
	if err != nil {
		return CrossCheckResult{}, fmt.Errorf("parse committed witness: %w", err)
	}
	// The committed witness must itself be a valid chain — otherwise it is not a
	// witness at all and there is nothing trustworthy to compare against.
	if err := drrebuild.VerifyChain(committed); err != nil {
		return CrossCheckResult{
			Match:            false,
			CommittedChainOK: false,
			Detail:           fmt.Sprintf("committed witness chain is broken (not tamper-evidence): %v", err),
		}, nil
	}

	rederived := ReDeriveWitness(doltRows)
	rederivedJSONL, err := SerializeWitness(rederived)
	if err != nil {
		return CrossCheckResult{}, fmt.Errorf("serialize re-derived witness: %w", err)
	}

	rederivedHead := ChainHead(rederived)
	committedHead := ChainHead(committed)
	bytesMatch := bytes.Equal(rederivedJSONL, committedJSONL)
	headMatch := rederivedHead == committedHead && rederivedHead != ""

	res := CrossCheckResult{
		Match:            headMatch && bytesMatch,
		RederivedHead:    rederivedHead,
		CommittedHead:    committedHead,
		BytesMatch:       bytesMatch,
		CommittedChainOK: true,
	}
	switch {
	case res.Match:
		res.Detail = fmt.Sprintf("faithful: Dolt re-derives to committed witness (head %s, %d records)", rederivedHead, len(rederived))
	case !headMatch:
		res.Detail = fmt.Sprintf("DIVERGED: re-derived chain head %s != committed head %s (Dolt rewritten without rewriting the git witness)", rederivedHead, committedHead)
	default:
		res.Detail = "DIVERGED: chain heads match but JSONL bytes differ (record count/ordering/serialization drift between Dolt and the committed witness)"
	}
	return res, nil
}
