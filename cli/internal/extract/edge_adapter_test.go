package extract

import (
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// tempStore returns a provenancegraph.Store bound to a fresh temp ledger file
// (no network, no shared state). The file does not exist yet; the first append
// creates it.
func tempStore(t *testing.T) *provenancegraph.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	return provenancegraph.NewStore(path)
}

func TestEdgeAdapter_AppendRoundTrip(t *testing.T) {
	store := tempStore(t)
	rel := Record{
		"from_id":  "dec-1",
		"relation": "wasGeneratedBy",
		"to_id":    "artifact-1",
	}
	nodeTypes := map[string]string{
		"dec-1":      "decision",
		"artifact-1": "artifact",
	}

	res, err := AppendRelation(store, rel, nodeTypes, "mined")
	if err != nil {
		t.Fatalf("AppendRelation: %v", err)
	}
	if res.Skipped {
		t.Fatalf("first append must not be skipped")
	}

	// Read the edge back from the ledger.
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge in ledger, got %d", len(edges))
	}
	got := edges[0]

	if got.FromID != "dec-1" {
		t.Errorf("from_id = %q, want %q", got.FromID, "dec-1")
	}
	if got.Relation != "wasGeneratedBy" {
		t.Errorf("relation = %q, want %q", got.Relation, "wasGeneratedBy")
	}
	if got.ToID != "artifact-1" {
		t.Errorf("to_id = %q, want %q", got.ToID, "artifact-1")
	}
	if got.FromType != "decision" {
		t.Errorf("from_type = %q, want %q", got.FromType, "decision")
	}
	if got.ToType != "artifact" {
		t.Errorf("to_type = %q, want %q", got.ToType, "artifact")
	}
	if got.TrustTier != "mined" {
		t.Errorf("trust_tier = %q, want %q", got.TrustTier, "mined")
	}

	// The edge must carry a valid recomputed hash chain.
	if got.Hash == "" || got.PayloadHash == "" {
		t.Fatalf("edge missing hashes: payload_hash=%q hash=%q", got.PayloadHash, got.Hash)
	}
	if broken, err := provenancegraph.VerifyChain(edges); err != nil {
		t.Errorf("VerifyChain failed at record %d: %v", broken, err)
	}
}

func TestEdgeAdapter_Idempotent(t *testing.T) {
	store := tempStore(t)
	rel := Record{
		"from_id":  "dec-1",
		"relation": "wasDerivedFrom",
		"to_id":    "artifact-2",
	}
	nodeTypes := map[string]string{
		"dec-1":      "decision",
		"artifact-2": "artifact",
	}

	first, err := AppendRelation(store, rel, nodeTypes, "mined")
	if err != nil {
		t.Fatalf("first AppendRelation: %v", err)
	}
	if first.Skipped {
		t.Fatalf("first append must not be skipped")
	}

	second, err := AppendRelation(store, rel, nodeTypes, "mined")
	if err != nil {
		t.Fatalf("second AppendRelation: %v", err)
	}
	if !second.Skipped {
		t.Errorf("second append of identical relation must be Skipped=true, got Skipped=%v", second.Skipped)
	}

	// No duplicate edge written.
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge after idempotent re-append, got %d", len(edges))
	}
}

func TestEdgeAdapter_RejectsUnknownVerb(t *testing.T) {
	store := tempStore(t)
	rel := Record{
		"from_id":  "dec-1",
		"relation": "derives_from", // colloquial, NOT a PROV-O verb
		"to_id":    "artifact-3",
	}
	nodeTypes := map[string]string{
		"dec-1":      "decision",
		"artifact-3": "artifact",
	}

	_, err := AppendRelation(store, rel, nodeTypes, "mined")
	if err == nil {
		t.Fatalf("expected rejection of non-PROV-O verb, got nil error")
	}

	// Nothing must be sealed/written: the ledger file must hold zero edges.
	edges, readErr := store.Read()
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}
	if len(edges) != 0 {
		t.Errorf("rejected relation must not be sealed; ledger has %d edges", len(edges))
	}
}

// TestEdgeAdapter_FindingMapsToArtifact covers the age-48z node-type gap: the
// extractor's 'finding' entity type is not in provenancegraph.NodeTypes and
// must be mapped to 'artifact' at write time, and an unknown id falls back to a
// valid node type rather than sealing an invalid edge.
func TestEdgeAdapter_FindingMapsToArtifact(t *testing.T) {
	store := tempStore(t)
	rel := Record{
		"from_id":  "finding-7",
		"relation": "wasInformedBy",
		"to_id":    "artifact-9",
	}
	nodeTypes := map[string]string{
		"finding-7": "finding", // not a valid NodeType -> artifact
		// artifact-9 deliberately absent -> defaultNodeType (artifact)
	}

	res, err := AppendRelation(store, rel, nodeTypes, "mined")
	if err != nil {
		t.Fatalf("AppendRelation: %v", err)
	}
	if res.Skipped {
		t.Fatalf("append must not be skipped")
	}
	if res.Edge.FromType != "artifact" {
		t.Errorf("finding from_type mapped to %q, want %q", res.Edge.FromType, "artifact")
	}
	if res.Edge.ToType != "artifact" {
		t.Errorf("unknown id to_type fell back to %q, want %q", res.Edge.ToType, "artifact")
	}
	// The sealed edge must be field-valid (valid node types).
	if err := provenancegraph.ValidateFields(res.Edge); err != nil {
		t.Errorf("sealed edge is not field-valid: %v", err)
	}
}
