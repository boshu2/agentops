package pool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func promotedMarkdownForBody(id, body string) string {
	return strings.Join([]string{
		"---",
		"id: " + id,
		"type: learning",
		"date: 2026-04-30",
		"tier: silver",
		"utility: 0.7000",
		"confidence: 0.8000",
		"maturity: provisional",
		"reward_count: 0",
		"helpful_count: 0",
		"harmful_count: 0",
		"source_session: unknown",
		"---",
		"",
		"# Learning: " + id,
		"",
		"## What We Learned",
		"",
		body,
		"",
		"## Source",
		"",
		"- **Source**: unknown",
		"",
	}, "\n")
}

func TestPoolPromote_LiveScansArtifactsWhenPromotedIndexMissing(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	body := "Close-loop promotion must live-scan promoted bodies when the sidecar index is missing."

	artifactDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}
	existingPath := filepath.Join(artifactDir, "2026-04-30-existing.md")
	if err := os.WriteFile(existingPath, []byte(promotedMarkdownForBody("existing", body)), 0o600); err != nil {
		t.Fatalf("write existing artifact: %v", err)
	}

	candidate := types.Candidate{
		ID:      "stale-pool-candidate",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: body,
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage(candidate.ID, types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	promotedPath, err := p.Promote(candidate.ID)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if promotedPath != existingPath {
		t.Fatalf("Promote path = %q, want existing artifact %q", promotedPath, existingPath)
	}

	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		t.Fatalf("read artifact dir: %v", err)
	}
	mdCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdCount++
		}
	}
	if mdCount != 1 {
		t.Fatalf("artifact count = %d, want 1", mdCount)
	}

	indexData, err := os.ReadFile(p.PromotedIndexPath())
	if err != nil {
		t.Fatalf("read backfilled promoted index: %v", err)
	}
	var indexed PromotedIndexEntry
	if err := json.Unmarshal(bytesTrimSpace(indexData), &indexed); err != nil {
		t.Fatalf("decode promoted index: %v\n%s", err, string(indexData))
	}
	if indexed.ContentHash != ContentHash(body) || indexed.ArtifactPath != existingPath {
		t.Fatalf("index entry = %+v, want hash=%s path=%s", indexed, ContentHash(body), existingPath)
	}

	chainData, err := os.ReadFile(filepath.Join(p.PoolPath, ChainFile))
	if err != nil {
		t.Fatalf("read chain: %v", err)
	}
	if !strings.Contains(string(chainData), "dedup: content hash already promoted") {
		t.Fatalf("chain missing dedup reason:\n%s", string(chainData))
	}
}

func TestPoolPromote_LiveScansArchivedArtifactsWhenPromotedIndexMissing(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	body := "Close-loop promotion must not recreate a body archived by corpus cleanup."

	archiveDir := filepath.Join(tmpDir, ".agents", "defrag", "2026-05-01-artifact-dedup", "files", ".agents", "learnings")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	archivedPath := filepath.Join(archiveDir, "2026-04-30-archived.md")
	if err := os.WriteFile(archivedPath, []byte(promotedMarkdownForBody("archived", body)), 0o600); err != nil {
		t.Fatalf("write archived artifact: %v", err)
	}

	candidate := types.Candidate{
		ID:      "archived-pool-candidate",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: body,
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage(candidate.ID, types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	promotedPath, err := p.Promote(candidate.ID)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if promotedPath != archivedPath {
		t.Fatalf("Promote path = %q, want archived artifact %q", promotedPath, archivedPath)
	}
	activeFiles, err := CollectPromotedArtifactFiles(tmpDir)
	if err != nil {
		t.Fatalf("collect active artifacts: %v", err)
	}
	if len(activeFiles) != 0 {
		t.Fatalf("active promoted artifacts = %d, want 0: %v", len(activeFiles), activeFiles)
	}

	chainData, err := os.ReadFile(filepath.Join(p.PoolPath, ChainFile))
	if err != nil {
		t.Fatalf("read chain: %v", err)
	}
	if !strings.Contains(string(chainData), "dedup: content hash already archived") {
		t.Fatalf("chain missing archived dedup reason:\n%s", string(chainData))
	}
}

func bytesTrimSpace(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}
