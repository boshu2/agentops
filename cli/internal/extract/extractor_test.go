package extract

import (
	"context"
	"strings"
	"testing"
)

// validChunkJSON is a well-formed structured-output response matching the
// schema CompileSchema produces.
const validChunkJSON = `{
  "entities": [
    {"id": "ao-gate", "label": "Go gate"},
    {"id": "pre-push", "label": "pre-push hook"}
  ],
  "relations": [
    {"from": "pre-push", "relation": "invokes", "to": "ao-gate"}
  ]
}`

const validChunkJSON2 = `{
  "entities": [{"id": "br", "label": "beads_rust tracker"}],
  "relations": []
}`

func TestExtractor(t *testing.T) {
	gen := &fakeGen{responses: []string{validChunkJSON}}
	client, err := NewClientWithGenerator(BackendBushidoLlama, gen)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	res, err := Extract(context.Background(), "short text within budget", sampleTemplate(), client)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(res.Entities) != 2 {
		t.Fatalf("entities: want 2, got %d: %v", len(res.Entities), res.Entities)
	}
	if res.Entities[0]["id"] != "ao-gate" {
		t.Errorf("entity[0].id = %v, want ao-gate", res.Entities[0]["id"])
	}
	if res.Entities[1]["label"] != "pre-push hook" {
		t.Errorf("entity[1].label = %v", res.Entities[1]["label"])
	}
	if len(res.Relations) != 1 {
		t.Fatalf("relations: want 1, got %d", len(res.Relations))
	}
	r := res.Relations[0]
	if r["from"] != "pre-push" || r["relation"] != "invokes" || r["to"] != "ao-gate" {
		t.Errorf("relation = %v", r)
	}
	if len(res.SurvivingChunks) != 1 || res.SurvivingChunks[0] != 0 {
		t.Errorf("surviving chunks = %v, want [0]", res.SurvivingChunks)
	}
	// No live model: the fake recorded exactly one call (one chunk).
	if gen.calls != 1 {
		t.Errorf("generator calls = %d, want 1", gen.calls)
	}
	// The prompt carried the template guideline (HOW).
	if !strings.Contains(gen.prompts[0], "Extract provenance entities and relations.") {
		t.Errorf("prompt missing guideline: %q", gen.prompts[0])
	}
}

func TestExtractor_FilterNoneResults(t *testing.T) {
	// Three chunks: valid, garbage (unparseable), valid. The middle must be
	// dropped; survivors are chunks 0 and 2, alignment preserved; valid subset
	// returned; no error.
	gen := &fakeGen{responses: []string{
		validChunkJSON,         // chunk 0 -> survives (2 entities, 1 relation)
		"not json at all <<<",  // chunk 1 -> filtered
		validChunkJSON2,        // chunk 2 -> survives (1 entity, 0 relations)
	}}
	client, err := NewClientWithGenerator(BackendBushidoLlama, gen)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	// Force three chunks with a tiny budget over a long input.
	long := strings.Repeat("alpha beta gamma delta ", 200) // > 4000 chars
	res, err := Extract3Chunks(t, client, long, gen)

	// Survivors are chunks 0 and 2 only (index alignment preserved — chunk 1 is
	// absent, not renumbered).
	if want := []int{0, 2}; !equalInts(res.SurvivingChunks, want) {
		t.Fatalf("surviving chunks = %v, want %v", res.SurvivingChunks, want)
	}
	// Valid subset merged: 2 (chunk0) + 1 (chunk2) = 3 entities; 1 relation.
	if len(res.Entities) != 3 {
		t.Errorf("entities: want 3 (valid subset), got %d: %v", len(res.Entities), res.Entities)
	}
	if len(res.Relations) != 1 {
		t.Errorf("relations: want 1, got %d", len(res.Relations))
	}
	if err != nil {
		t.Errorf("Extract returned error for a filterable malformed chunk: %v", err)
	}
}

// Extract3Chunks runs Extract with a forced 3-chunk split by using a small
// budget via a direct Chunk call to assert the chunking, then calling Extract
// (which uses DefaultChunkBudget). To guarantee exactly 3 chunks regardless of
// the default, the test feeds a long input and verifies the fake was called
// once per chunk.
func Extract3Chunks(t *testing.T, client *Client, text string, gen *fakeGen) (*Result, error) {
	t.Helper()
	// Confirm Chunk splits this input into >1 chunk at a small budget (the
	// chunking-before-extraction contract).
	if got := len(Chunk(text, 2000)); got < 2 {
		t.Fatalf("expected long input to chunk (>=2) at budget 2000, got %d", got)
	}
	res, err := extractWithBudget(client, text, sampleTemplate(), 2000)
	// The fake must have been called once per chunk (no live model).
	if gen.calls != len(Chunk(text, 2000)) {
		t.Errorf("generator calls = %d, want %d (one per chunk)", gen.calls, len(Chunk(text, 2000)))
	}
	return res, err
}

func TestChunk_WithinBudgetSingle(t *testing.T) {
	chunks := Chunk("tiny", 1000)
	if len(chunks) != 1 || chunks[0] != "tiny" {
		t.Errorf("Chunk(tiny) = %v", chunks)
	}
}

func TestChunk_SplitsLongInput(t *testing.T) {
	long := strings.Repeat("word ", 5000) // 25000 chars
	chunks := Chunk(long, 1000)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len([]rune(c)) > 1000 {
			t.Errorf("chunk %d exceeds budget: %d runes", i, len([]rune(c)))
		}
	}
	// Reassembly is lossless.
	if strings.Join(chunks, "") != long {
		t.Error("chunk reassembly is lossy")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
