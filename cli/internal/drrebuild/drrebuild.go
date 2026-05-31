// practices: [in-toto-provenance, dora-metrics, adr]

// Package drrebuild is the disaster-recovery PROOF for the ag-lmdx
// event-log-canonical design: it demonstrates that the Dolt context graph
// (nodes + provenance_edges) is a *rebuildable projection* of two durable
// sources — the committed, per-record hash-chained provenance ledger
// (schema agentops-sdlc-provenance.v1 at docs/provenance/ledger.jsonl) plus
// content-addressed git blobs — and is therefore never a valid sole source of
// truth.
//
// The thesis (ag-lmdx parent): "invert source of truth: append-only event log
// canonical, state is a rebuildable projection." Per CLAUDE.md, the committed
// ledger is the AUDIT authority and wins on disagreement with the Dolt write
// model. "If you can't rebuild from the audit, you have a hope, not a backup."
//
// This package is a PROOF, not the production `ao provenance rebuild` service.
// It is a hermetic, deterministic demonstration that the rebuild is *possible
// and faithful*:
//
//   - Rebuild matches original: replaying the verified ledger reconstructs a
//     graph whose content hash matches the original Dolt projection.
//   - Missing blob is detected, not silently dropped: a ledger edge whose
//     evidence content_hash has no git blob aborts the rebuild loudly, naming
//     the dangling content_ref, rather than presenting a partial graph as
//     complete (the do-not-ship failure mode).
//
// Ledger event shape and hash discipline match the real schema exactly:
// payload_hash = sha256(canonical-JSON of every field except
// prev_hash/payload_hash/hash); hash = sha256(payload_hash + "\n" + prev_hash);
// prev_hash links the previous record (empty for genesis). This mirrors
// cli/internal/rpi/ledger.go ComputeLedgerHashes, which the schema cites.
//
// Content addressing matches git's blob object hashing exactly
// (`git hash-object`): sha1("blob " + len + "\x00" + content). That is what
// makes "rebuild from git blobs" literal — a content_hash in the ledger is the
// same OID `git cat-file blob <hash>` resolves.
package drrebuild

import (
	"bufio"
	"crypto/sha1" // #nosec G505 nosemgrep -- git object IDs are SHA-1 by definition; not a security primitive here.
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// LedgerEvent is one append-only record of the agentops-sdlc-provenance.v1
// ledger: a typed, evidence-backed provenance edge between two SDLC nodes,
// plus the export-time hash-chain fields. Field tags match the schema so the
// proof parses the real committed ledger format, not an invented one.
type LedgerEvent struct {
	SchemaVersion string `json:"schema_version"`
	FromID        string `json:"from_id"`
	FromType      string `json:"from_type"`
	ToID          string `json:"to_id"`
	ToType        string `json:"to_type"`
	Relation      string `json:"relation"`
	EvidenceRef   string `json:"evidence_ref,omitempty"`
	TrustTier     string `json:"trust_tier"`
	TS            string `json:"ts"`
	PrevHash      string `json:"prev_hash"`
	PayloadHash   string `json:"payload_hash"`
	Hash          string `json:"hash"`
}

// ledgerPayload is the hash-covered subset of a LedgerEvent: every field
// EXCEPT prev_hash, payload_hash, and hash. Marshaled in the schema's field
// order (struct field order is stable in encoding/json) to reproduce
// payload_hash deterministically.
type ledgerPayload struct {
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

// Node is a rebuilt context-graph node. The node set is itself a projection:
// it is the deduplicated union of every edge endpoint in the ledger. A node's
// trust tier is the *strongest* (max over the authored>inferred>mined order)
// tier among edges that assert it — the conservative read for "do we believe
// this node exists at all".
type Node struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	TrustTier string `json:"trust_tier"`
}

// EdgeView is a rebuilt provenance_edge in the projected graph, including the
// resolved evidence content fetched from the blob store (empty when the edge
// carries no content-addressed evidence_ref).
type EdgeView struct {
	FromID      string `json:"from_id"`
	ToID        string `json:"to_id"`
	Relation    string `json:"relation"`
	TrustTier   string `json:"trust_tier"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	Evidence    string `json:"evidence,omitempty"`
}

// Graph is the rebuilt projection of the context graph — what Dolt would hold
// live, reconstructed purely from the ledger + blob store.
type Graph struct {
	Nodes []Node     `json:"nodes"`
	Edges []EdgeView `json:"edges"`
}

// BlobStore resolves a content_hash (git blob OID) to its bytes. The real git
// object database is the canonical implementation; tests use MapBlobStore.
// Resolve must return found=false (not an empty body) when the hash is absent,
// so the rebuild can fail loudly on a dangling content_ref.
type BlobStore interface {
	Resolve(contentHash string) (body []byte, found bool)
}

// MapBlobStore is an in-memory BlobStore for hermetic tests and for staging a
// blob set extracted from `git cat-file batch`.
type MapBlobStore map[string][]byte

// Resolve implements BlobStore.
func (m MapBlobStore) Resolve(contentHash string) ([]byte, bool) {
	b, ok := m[contentHash]
	return b, ok
}

// GitBlobOID computes the git object ID of content, identical to
// `git hash-object --stdin`: sha1("blob " + len + "\x00" + content). This lets
// a ledger content_hash be resolved against the real git object store during
// production recovery.
func GitBlobOID(content []byte) string {
	h := sha1.New() // #nosec G401 nosemgrep -- git blob IDs are SHA-1; matching git, not securing anything.
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// HashHex returns the lowercase-hex sha256 of data — the ledger's hashing
// primitive (mirrors cli/internal/rpi/ledger.go HashHex).
func HashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DanglingRefError is returned when an edge's content-addressed evidence_ref
// has no blob in the store. The whole point of the DR proof: this must surface,
// naming the offending ref, rather than producing a partial graph presented as
// complete.
type DanglingRefError struct {
	FromID      string
	ToID        string
	ContentHash string
}

func (e *DanglingRefError) Error() string {
	return fmt.Sprintf("dangling content_ref: edge %s->%s references evidence content_hash %q which is absent from the blob store (git GC may have pruned it); refusing to present a partial graph as complete",
		e.FromID, e.ToID, e.ContentHash)
}

// trustRank orders trust tiers; higher is stronger. Unknown tiers rank lowest.
func trustRank(tier string) int {
	switch tier {
	case "authored":
		return 3
	case "inferred":
		return 2
	case "mined":
		return 1
	default:
		return 0
	}
}

// gitOIDLen is the hex length of a SHA-1 git object id.
const gitOIDLen = 40

// isContentRef reports whether evidence_ref is a content-addressed git blob OID
// (40 lowercase hex chars) versus some other evidence pointer (CI URL, path,
// bead id). Only content refs are resolved against the blob store.
func isContentRef(evidenceRef string) bool {
	if len(evidenceRef) != gitOIDLen {
		return false
	}
	for _, c := range evidenceRef {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ParseLedger reads the append-only JSONL ledger. Blank lines are skipped.
func ParseLedger(r io.Reader) ([]LedgerEvent, error) {
	var events []LedgerEvent
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var ev LedgerEvent
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			return nil, fmt.Errorf("ledger line %d: %w", line, err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading ledger: %w", err)
	}
	return events, nil
}

// computeHashes returns the (payload_hash, hash) a record must carry, using the
// schema's discipline: payload_hash = sha256(canonical payload), hash =
// sha256(payload_hash + "\n" + prev_hash).
func computeHashes(ev LedgerEvent) (payloadHash, hash string, err error) {
	payload := ledgerPayload{
		SchemaVersion: ev.SchemaVersion,
		FromID:        ev.FromID,
		FromType:      ev.FromType,
		ToID:          ev.ToID,
		ToType:        ev.ToType,
		Relation:      ev.Relation,
		EvidenceRef:   ev.EvidenceRef,
		TrustTier:     ev.TrustTier,
		TS:            ev.TS,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal payload: %w", err)
	}
	payloadHash = HashHex(payloadBytes)
	hash = HashHex([]byte(payloadHash + "\n" + ev.PrevHash))
	return payloadHash, hash, nil
}

// ComputeEventHashes returns the (payload_hash, hash) that ev must carry given
// its PrevHash, per the schema's discipline. Exported so a ledger writer (and
// the proof's fixture generator) can chain records the same way the rebuild
// verifies them. Panics only on a JSON marshal failure of a plain struct,
// which cannot occur for LedgerEvent's string-only fields.
func ComputeEventHashes(ev LedgerEvent) (payloadHash, hash string) {
	ph, h, err := computeHashes(ev)
	if err != nil {
		panic(fmt.Sprintf("drrebuild: ComputeEventHashes: %v", err))
	}
	return ph, h
}

// VerifyChain checks the per-record payload_hash, hash, and prev-hash linkage.
// A tamper-evident witness is only a witness if the chain verifies; a broken
// chain means the ledger itself is untrustworthy and rebuild must not proceed.
func VerifyChain(events []LedgerEvent) error {
	prev := ""
	for i, ev := range events {
		payloadHash, hash, err := computeHashes(ev)
		if err != nil {
			return fmt.Errorf("event %d (%s->%s): %w", i, ev.FromID, ev.ToID, err)
		}
		if ev.PayloadHash != payloadHash {
			return fmt.Errorf("event %d (%s->%s): payload_hash mismatch: ledger=%q computed=%q (tampered)", i, ev.FromID, ev.ToID, ev.PayloadHash, payloadHash)
		}
		if ev.Hash != hash {
			return fmt.Errorf("event %d (%s->%s): hash mismatch: ledger=%q computed=%q (tampered)", i, ev.FromID, ev.ToID, ev.Hash, hash)
		}
		if ev.PrevHash != prev {
			return fmt.Errorf("event %d (%s->%s): prev_hash mismatch: ledger=%q expected=%q (chain broken/reordered)", i, ev.FromID, ev.ToID, ev.PrevHash, prev)
		}
		prev = ev.Hash
	}
	return nil
}

// Rebuild replays the ledger against the blob store and returns the
// reconstructed graph. It is the core of the DR proof: Dolt is wiped, and this
// reconstitutes the node set and provenance_edges purely from JSONL + git
// blobs.
//
// A dangling content_ref aborts with *DanglingRefError — a partial graph is
// never returned alongside a nil error.
func Rebuild(events []LedgerEvent, blobs BlobStore) (*Graph, error) {
	nodes := map[string]*Node{}
	var nodeOrder []string // first-seen order; output is sorted by Canonicalize
	var edges []EdgeView

	observe := func(id, typ, tier string) {
		n := nodes[id]
		if n == nil {
			nodes[id] = &Node{ID: id, Type: typ, TrustTier: tier}
			nodeOrder = append(nodeOrder, id)
			return
		}
		// Strongest trust tier wins; a node believed via an authored edge is
		// authored even if also referenced by a mined one.
		if trustRank(tier) > trustRank(n.TrustTier) {
			n.TrustTier = tier
		}
		if n.Type == "" {
			n.Type = typ
		}
	}

	for i, ev := range events {
		if ev.FromID == "" || ev.ToID == "" {
			return nil, fmt.Errorf("event %d: edge missing from_id/to_id", i)
		}
		observe(ev.FromID, ev.FromType, ev.TrustTier)
		observe(ev.ToID, ev.ToType, ev.TrustTier)

		view := EdgeView{
			FromID:      ev.FromID,
			ToID:        ev.ToID,
			Relation:    ev.Relation,
			TrustTier:   ev.TrustTier,
			EvidenceRef: ev.EvidenceRef,
		}
		if isContentRef(ev.EvidenceRef) {
			body, found := blobs.Resolve(ev.EvidenceRef)
			if !found {
				return nil, &DanglingRefError{FromID: ev.FromID, ToID: ev.ToID, ContentHash: ev.EvidenceRef}
			}
			// Defense-in-depth: the resolved blob must actually hash to the ref.
			if got := GitBlobOID(body); got != ev.EvidenceRef {
				return nil, fmt.Errorf("edge %s->%s: blob store returned content hashing to %q but ledger evidence_ref is %q (corrupt blob)", ev.FromID, ev.ToID, got, ev.EvidenceRef)
			}
			view.Evidence = string(body)
		}
		edges = append(edges, view)
	}

	g := &Graph{}
	for _, id := range nodeOrder {
		g.Nodes = append(g.Nodes, *nodes[id])
	}
	g.Edges = edges
	return g, nil
}

// Canonicalize returns a deterministic, order-independent serialization of the
// graph. Nodes sort by ID; edges by (FromID, ToID, Relation, EvidenceRef).
// This makes a hash-compare meaningful: rebuilt-vs-original equality must not
// depend on replay order or map iteration order.
func (g *Graph) Canonicalize() []byte {
	nodes := append([]Node(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	edges := append([]EdgeView(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromID != edges[j].FromID {
			return edges[i].FromID < edges[j].FromID
		}
		if edges[i].ToID != edges[j].ToID {
			return edges[i].ToID < edges[j].ToID
		}
		if edges[i].Relation != edges[j].Relation {
			return edges[i].Relation < edges[j].Relation
		}
		return edges[i].EvidenceRef < edges[j].EvidenceRef
	})
	type canonGraph struct {
		Nodes []Node     `json:"nodes"`
		Edges []EdgeView `json:"edges"`
	}
	out, err := json.Marshal(canonGraph{Nodes: nodes, Edges: edges})
	if err != nil {
		// Unreachable: canonGraph holds only strings/slices-of-strings, which
		// always marshal. Panic rather than silently emit a "null" body that
		// would poison the graph hash.
		panic(fmt.Sprintf("drrebuild: canonicalize graph: %v", err))
	}
	return out
}

// Hash returns the SHA-256 of the canonical graph serialization. Two graphs are
// state-equal iff their Hash values match — the hash-compare the bead's
// "Rebuild matches original" scenario asserts.
func (g *Graph) Hash() string {
	return HashHex(g.Canonicalize())
}

// RebuildFromLedger is the end-to-end DR path: parse, verify the chain, then
// replay against the blob store. This is the function the production
// `ao provenance rebuild` would wrap; the proof exercises it directly.
func RebuildFromLedger(r io.Reader, blobs BlobStore) (*Graph, error) {
	events, err := ParseLedger(r)
	if err != nil {
		return nil, err
	}
	if err := VerifyChain(events); err != nil {
		return nil, fmt.Errorf("ledger chain verification failed (witness untrustworthy, refusing rebuild): %w", err)
	}
	return Rebuild(events, blobs)
}
