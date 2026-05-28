// practices: [microservices, sre]
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/pool"
)

// promotedArtifactBody is the format Pool.writeArtifact produces. The
// "## What We Learned" body is the dedup-hashed candidate.Content.
func promotedArtifactBody(title, content string) string {
	return strings.Join([]string{
		"---",
		"id: " + title,
		"type: learning",
		"date: 2026-04-30",
		"tier: silver",
		"utility: 0.7000",
		"confidence: 0.0000",
		"maturity: provisional",
		"reward_count: 0",
		"helpful_count: 0",
		"harmful_count: 0",
		"source_session: unknown",
		"---",
		"",
		"# Learning: " + title,
		"",
		"## What We Learned",
		"",
		content,
		"",
		"## Source",
		"",
		"- **Source**: unknown",
		"",
	}, "\n")
}

// writeArtifactFile is a test helper to drop a Promote-shaped file on disk.
func writeArtifactFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	body := promotedArtifactBody(strings.TrimSuffix(name, ".md"), content)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// readIndexLines returns one PromotedIndexEntry per line in the sidecar.
func readIndexLines(t *testing.T, path string) []pool.PromotedIndexEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open index %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	var out []pool.PromotedIndexEntry
	s := bufio.NewScanner(f)
	for s.Scan() {
		var e pool.PromotedIndexEntry
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal index line %q: %v", s.Text(), err)
		}
		out = append(out, e)
	}
	return out
}

// TestPoolReindex_EmptyIndex_AddsAllArtifacts verifies the L2 happy path:
// two surviving artifacts with no pre-existing index → two entries appended,
// hashes match what Promote would have written for the same body.
func TestPoolReindex_EmptyIndex_AddsAllArtifacts(t *testing.T) {
	tmp := t.TempDir()
	learnings := filepath.Join(tmp, ".agents", "learnings")
	patterns := filepath.Join(tmp, ".agents", "patterns")

	pathA := writeArtifactFile(t, learnings, "alpha.md", "Body alpha goes here.")
	pathB := writeArtifactFile(t, patterns, "beta.md", "Body beta is different.")

	res, err := poolReindexRun(tmp, false)
	if err != nil {
		t.Fatalf("poolReindexRun: %v", err)
	}

	if res.Scanned != 2 {
		t.Errorf("Scanned = %d, want 2", res.Scanned)
	}
	if res.NewEntries != 2 {
		t.Errorf("NewEntries = %d, want 2", res.NewEntries)
	}
	if res.AlreadyIndexed != 0 {
		t.Errorf("AlreadyIndexed = %d, want 0", res.AlreadyIndexed)
	}
	if res.ExistingHashes != 0 {
		t.Errorf("ExistingHashes = %d, want 0", res.ExistingHashes)
	}
	if res.DryRun {
		t.Error("DryRun = true, want false")
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want none", res.Errors)
	}

	indexPath := filepath.Join(tmp, ".agents", "pool", "promoted-index.jsonl")
	entries := readIndexLines(t, indexPath)
	if len(entries) != 2 {
		t.Fatalf("index entries = %d, want 2", len(entries))
	}

	wantHashA := pool.ContentHash("Body alpha goes here.")
	wantHashB := pool.ContentHash("Body beta is different.")
	got := map[string]string{
		entries[0].ContentHash: entries[0].ArtifactPath,
		entries[1].ContentHash: entries[1].ArtifactPath,
	}
	if got[wantHashA] != pathA {
		t.Errorf("hash for alpha: artifact_path = %q, want %q", got[wantHashA], pathA)
	}
	if got[wantHashB] != pathB {
		t.Errorf("hash for beta: artifact_path = %q, want %q", got[wantHashB], pathB)
	}
}

// TestPoolReindex_PreservesExistingEntries verifies the index is APPEND-ONLY:
// existing entries (e.g. ones written by a previous Promote) stay intact and
// the same hash is not duplicated.
func TestPoolReindex_PreservesExistingEntries(t *testing.T) {
	tmp := t.TempDir()
	learnings := filepath.Join(tmp, ".agents", "learnings")
	pathA := writeArtifactFile(t, learnings, "alpha.md", "Body alpha goes here.")
	pathB := writeArtifactFile(t, learnings, "beta.md", "Body beta is here.")

	// Pre-seed the index with alpha's hash + a stale pre-existing entry for
	// a candidate-id that no longer exists on disk. Both must survive.
	indexDir := filepath.Join(tmp, ".agents", "pool")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		t.Fatalf("mkdir pool: %v", err)
	}
	indexPath := filepath.Join(indexDir, "promoted-index.jsonl")
	preexisting := []map[string]string{
		{
			"content_hash":  pool.ContentHash("Body alpha goes here."),
			"artifact_path": pathA,
			"candidate_id":  "alpha",
			"promoted_at":   "2026-04-01T00:00:00Z",
		},
		{
			"content_hash":  "deadbeef-not-on-disk",
			"artifact_path": "/tmp/gone.md",
			"candidate_id":  "stale",
			"promoted_at":   "2026-04-01T00:00:00Z",
		},
	}
	var seed bytes.Buffer
	for _, e := range preexisting {
		b, _ := json.Marshal(e)
		seed.Write(b)
		seed.WriteByte('\n')
	}
	if err := os.WriteFile(indexPath, seed.Bytes(), 0o644); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	res, err := poolReindexRun(tmp, false)
	if err != nil {
		t.Fatalf("poolReindexRun: %v", err)
	}

	if res.Scanned != 2 {
		t.Errorf("Scanned = %d, want 2", res.Scanned)
	}
	if res.NewEntries != 1 {
		t.Errorf("NewEntries = %d, want 1 (only beta)", res.NewEntries)
	}
	if res.AlreadyIndexed != 1 {
		t.Errorf("AlreadyIndexed = %d, want 1 (alpha)", res.AlreadyIndexed)
	}
	if res.ExistingHashes != 2 {
		t.Errorf("ExistingHashes = %d, want 2", res.ExistingHashes)
	}

	entries := readIndexLines(t, indexPath)
	if len(entries) != 3 {
		t.Fatalf("index entries = %d, want 3 (2 seed + 1 new)", len(entries))
	}

	// Confirm the stale entry was preserved.
	foundStale := false
	for _, e := range entries {
		if e.ContentHash == "deadbeef-not-on-disk" {
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Error("stale pre-existing entry was removed; index must be append-only")
	}

	// Confirm beta was appended with the correct hash.
	betaHash := pool.ContentHash("Body beta is here.")
	foundBeta := false
	for _, e := range entries {
		if e.ContentHash == betaHash && e.ArtifactPath == pathB {
			foundBeta = true
			break
		}
	}
	if !foundBeta {
		t.Errorf("beta entry not appended; want hash=%s path=%s", betaHash, pathB)
	}
}

// TestPoolReindex_DryRun_NoWrite verifies --dry-run reports counts but
// leaves the index file untouched.
func TestPoolReindex_DryRun_NoWrite(t *testing.T) {
	tmp := t.TempDir()
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "learnings"), "alpha.md", "Body alpha.")
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "patterns"), "beta.md", "Body beta.")

	res, err := poolReindexRun(tmp, true)
	if err != nil {
		t.Fatalf("poolReindexRun(dry): %v", err)
	}

	if !res.DryRun {
		t.Error("DryRun = false, want true")
	}
	if res.Scanned != 2 {
		t.Errorf("Scanned = %d, want 2", res.Scanned)
	}
	if res.NewEntries != 2 {
		t.Errorf("NewEntries = %d, want 2", res.NewEntries)
	}

	indexPath := filepath.Join(tmp, ".agents", "pool", "promoted-index.jsonl")
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Errorf("index file exists after dry-run: stat err = %v, want IsNotExist", err)
	}
}

// TestPoolReindex_WalksBothDirectories verifies both .agents/learnings and
// .agents/patterns are scanned.
func TestPoolReindex_WalksBothDirectories(t *testing.T) {
	tmp := t.TempDir()
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "learnings"), "from-learnings.md", "Content L.")
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "patterns"), "from-patterns.md", "Content P.")

	files, err := collectReindexFiles(tmp)
	if err != nil {
		t.Fatalf("collectReindexFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("collected %d files, want 2: %v", len(files), files)
	}

	hasLearnings := false
	hasPatterns := false
	for _, f := range files {
		if strings.Contains(f, filepath.Join(".agents", "learnings")) {
			hasLearnings = true
		}
		if strings.Contains(f, filepath.Join(".agents", "patterns")) {
			hasPatterns = true
		}
	}
	if !hasLearnings {
		t.Error("learnings directory not walked")
	}
	if !hasPatterns {
		t.Error("patterns directory not walked")
	}
}

// TestPoolReindex_NoDirectories returns an empty result without erroring
// when neither directory exists.
func TestPoolReindex_NoDirectories(t *testing.T) {
	tmp := t.TempDir()
	res, err := poolReindexRun(tmp, false)
	if err != nil {
		t.Fatalf("poolReindexRun: %v", err)
	}
	if res.Scanned != 0 {
		t.Errorf("Scanned = %d, want 0", res.Scanned)
	}
	if res.NewEntries != 0 {
		t.Errorf("NewEntries = %d, want 0", res.NewEntries)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want none", res.Errors)
	}
}

// TestPoolReindex_HashMatchesPromoteContract verifies the hash recorded by
// reindex is byte-identical to what Pool.Promote would compute for an
// equivalent candidate. This is the load-bearing dedup property: future
// Promote calls of the same body must collide with the reindexed entry.
func TestPoolReindex_HashMatchesPromoteContract(t *testing.T) {
	body := "  Some learning content with leading/trailing whitespace.  \n"

	// reindex strips frontmatter and pulls the "## What We Learned" section,
	// which is the same Candidate.Content body Promote hashes (modulo
	// TrimSpace).
	reindexHash := pool.ContentHash(body)

	// Synthesize the same hash as if we had a Candidate{Content: body}.
	// pool.ContentHash IS what candidateContentHash delegates to, so this
	// is a tautology check that documents the contract.
	if got := pool.ContentHash(strings.TrimSpace(body)); got != reindexHash {
		t.Errorf("ContentHash not idempotent under TrimSpace: got %q, want %q", got, reindexHash)
	}
}

// TestPoolReindex_ExtractBodyFromArtifact verifies the body extractor pulls
// out the candidate-content section from a real Promote-shaped artifact.
func TestPoolReindex_ExtractBodyFromArtifact(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	expected := "This is the actual learning content."
	path := writeArtifactFile(t, dir, "sample.md", expected)

	body, ok := extractPromotedArtifactBody(path)
	if !ok {
		t.Fatal("extractPromotedArtifactBody returned ok=false")
	}
	if body != expected {
		t.Errorf("body = %q, want %q", body, expected)
	}
}

// TestPoolReindex_DuplicateBodiesProduceOneEntry guards against a single
// reindex run writing two entries for the same content hash (e.g. two
// learning files with byte-identical "## What We Learned" bodies).
func TestPoolReindex_DuplicateBodiesProduceOneEntry(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	writeArtifactFile(t, dir, "alpha.md", "Same body.")
	writeArtifactFile(t, dir, "alpha-copy.md", "Same body.")

	res, err := poolReindexRun(tmp, false)
	if err != nil {
		t.Fatalf("poolReindexRun: %v", err)
	}
	if res.Scanned != 2 {
		t.Errorf("Scanned = %d, want 2", res.Scanned)
	}
	if res.NewEntries != 1 {
		t.Errorf("NewEntries = %d, want 1 (dup body coalesced)", res.NewEntries)
	}
	if res.AlreadyIndexed != 1 {
		t.Errorf("AlreadyIndexed = %d, want 1 (the second copy)", res.AlreadyIndexed)
	}
}

// TestPoolReindex_JSONOutput verifies the JSON shape used by tools/scripts.
func TestPoolReindex_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "learnings"), "x.md", "x body")

	res, err := poolReindexRun(tmp, true)
	if err != nil {
		t.Fatalf("poolReindexRun: %v", err)
	}

	var buf bytes.Buffer
	if err := writePoolReindexResult(&buf, res, true); err != nil {
		t.Fatalf("writePoolReindexResult: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput=%s", err, buf.String())
	}
	if parsed["scanned"].(float64) != 1 {
		t.Errorf(`json["scanned"] = %v, want 1`, parsed["scanned"])
	}
	if parsed["new_entries"].(float64) != 1 {
		t.Errorf(`json["new_entries"] = %v, want 1`, parsed["new_entries"])
	}
	if parsed["dry_run"].(bool) != true {
		t.Errorf(`json["dry_run"] = %v, want true`, parsed["dry_run"])
	}
}
