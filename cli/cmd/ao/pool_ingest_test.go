package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestParseLearningBlocks(t *testing.T) {
	md := `# Learnings: ag-xyz — Something

**Date:** 2026-01-01

# Learning: First Title

**ID**: L1
**Category**: process
**Confidence**: high

## What We Learned

Do the thing.

# Learning: Second Title

**ID**: L2
**Category**: architecture
**Confidence**: medium

## What We Learned

Do the other thing.
`

	blocks := parseLearningBlocks(md)
	if len(blocks) != 2 {
		t.Fatalf("blocks=%d, want 2", len(blocks))
	}
	if blocks[0].Title != "First Title" || blocks[0].ID != "L1" || blocks[0].Category != "process" || blocks[0].Confidence != "high" {
		t.Fatalf("block0=%+v", blocks[0])
	}
	if blocks[1].Title != "Second Title" || blocks[1].ID != "L2" || blocks[1].Category != "architecture" || blocks[1].Confidence != "medium" {
		t.Fatalf("block1=%+v", blocks[1])
	}
}

func TestParseLearningBlocksLegacyFrontmatter(t *testing.T) {
	md := `---
type: learning
source: manual
date: 2026-02-20
---

# Fix shell PATH mismatch for ao detection

Ensure command checks run in the same shell context as runtime.
`

	blocks := parseLearningBlocks(md)
	if len(blocks) != 1 {
		t.Fatalf("blocks=%d, want 1", len(blocks))
	}
	if blocks[0].Category != "learning" {
		t.Fatalf("category=%q, want learning", blocks[0].Category)
	}
	if blocks[0].Confidence != "medium" {
		t.Fatalf("confidence=%q, want medium default", blocks[0].Confidence)
	}
	if blocks[0].Title == "" {
		t.Fatal("expected non-empty title")
	}
}

func TestResolveIngestFilesDefaultIncludesLegacyKnowledge(t *testing.T) {
	tmp := t.TempDir()
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	rootKnowledge := filepath.Join(tmp, ".agents", "knowledge")
	if err := os.MkdirAll(pendingDir, 0o700); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	if err := os.MkdirAll(rootKnowledge, 0o700); err != nil {
		t.Fatalf("mkdir knowledge: %v", err)
	}

	pendingFile := filepath.Join(pendingDir, "2026-02-20-a.md")
	legacyFile := filepath.Join(rootKnowledge, "2026-02-20-b.md")
	if err := os.WriteFile(pendingFile, []byte("# Learning: A"), 0o600); err != nil {
		t.Fatalf("write pending: %v", err)
	}
	if err := os.WriteFile(legacyFile, []byte("# Learning: B"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	files, err := resolveIngestFiles(tmp, filepath.Join(".agents", "knowledge", "pending"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	seen := make(map[string]bool)
	for _, f := range files {
		seen[f] = true
	}
	if !seen[pendingFile] {
		t.Fatalf("missing pending file in default ingest set: %s", pendingFile)
	}
	if !seen[legacyFile] {
		t.Fatalf("missing legacy file in default ingest set: %s", legacyFile)
	}
}

func TestIngestAutoPromoteAndIndex(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pendingFile := filepath.Join(pendingDir, "2026-01-01-ag-xyz-learnings.md")
	if err := os.WriteFile(pendingFile, []byte(`# Learnings: ag-xyz — Something

**Date:** 2026-01-01

# Learning: First Title

**ID**: L1
**Category**: process
**Confidence**: high

## What We Learned

Run command -v ao first.

## Source

Session: ag-xyz
`), 0600); err != nil {
		t.Fatalf("write pending: %v", err)
	}

	ingRes, err := ingestPendingFilesToPool(tmp, []string{pendingFile})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if ingRes.Added != 1 {
		t.Fatalf("added=%d, want 1 (res=%+v)", ingRes.Added, ingRes)
	}

	p := pool.NewPool(tmp)
	entries, err := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("list pool: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("pool entries=%d, want 1", len(entries))
	}

	// Auto-promotion now requires citation evidence.
	if err := ratchet.RecordCitation(tmp, types.CitationEvent{
		ArtifactPath: entries[0].FilePath,
		SessionID:    "session-ingest-test",
		CitedAt:      time.Now(),
		CitationType: "retrieved",
		Query:        "session hygiene",
	}); err != nil {
		t.Fatalf("record citation: %v", err)
	}

	// With a 1h threshold, a 2026-01-01 AddedAt should be eligible.
	autoRes, err := autoPromoteAndPromoteToArtifacts(p, time.Hour, true)
	if err != nil {
		t.Fatalf("auto-promote: %v", err)
	}
	if autoRes.Promoted != 1 || len(autoRes.Artifacts) != 1 {
		t.Fatalf("autoRes=%+v", autoRes)
	}

	if _, err := os.Stat(autoRes.Artifacts[0]); err != nil {
		t.Fatalf("artifact missing: %v", err)
	}

	// Ensure artifact landed in .agents/learnings.
	if filepath.Base(filepath.Dir(autoRes.Artifacts[0])) != "learnings" {
		t.Fatalf("artifact dir=%s, want learnings", filepath.Dir(autoRes.Artifacts[0]))
	}

	indexed, indexPath, err := storeIndexUpsert(tmp, autoRes.Artifacts, true)
	if err != nil {
		t.Fatalf("store index: %v", err)
	}
	if indexed != 1 {
		t.Fatalf("indexed=%d, want 1", indexed)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index missing: %v", err)
	}

	f, err := os.Open(indexPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("expected index entry line")
	}
	var ie IndexEntry
	if err := json.Unmarshal(sc.Bytes(), &ie); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	if ie.Category != "process" {
		t.Fatalf("index category=%q, want %q", ie.Category, "process")
	}
}
