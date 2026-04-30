package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// TestPromote_DedupsAcrossWriters is the cross-writer dedup contract from
// soc-f2q4 (flywheel phase 4-A). There are TWO writers that promote
// learning artifacts to disk:
//
//  1. harvest.Promote (cli/internal/harvest/catalog.go)
//  2. pool.(*Pool).Promote (cli/internal/pool/pool.go)
//
// Each writer must dedup by content body, regardless of the ID drift in
// candidate/artifact metadata. The contract is per-writer: each writer
// produces exactly one on-disk file when fed the same body via different
// IDs. (Writers may produce parallel files because they target different
// directories, but each writer is content-deduped.)
//
// Without this contract, a regression in either writer's default
// (e.g., the SkipGlobalHub=true / guarded-append pair from agentops
// 4af82384 + f6fce986) re-bloats the global hub at ~/.agents/learnings/.
func TestPromote_DedupsAcrossWriters(t *testing.T) {
	body := "When fmt.Errorf wraps with %w you preserve the underlying error chain"
	bodySum := sha256.Sum256([]byte(body))
	contentHash := hex.EncodeToString(bodySum[:])

	t.Run("harvest writer dedups same body via different IDs", func(t *testing.T) {
		assertHarvestPromoteDedups(t, body, contentHash)
	})

	t.Run("pool writer dedups same body via different IDs", func(t *testing.T) {
		assertPoolPromoteDedups(t, body)
	})
}

func assertHarvestPromoteDedups(t *testing.T, body, contentHash string) {
	t.Helper()
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Two source files with identical body but different filenames; same
	// ContentHash so BuildCatalog collapses to one artifact, and Promote
	// then writes one file. Mirrors how the close-loop / harvest pipeline
	// would feed two rewrites of the same insight.
	srcA := filepath.Join(srcDir, "a.md")
	srcB := filepath.Join(srcDir, "b.md")
	frontmatter := "---\ntype: learning\nconfidence: 0.9\n---\n\n"
	if err := os.WriteFile(srcA, []byte(frontmatter+body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcB, []byte(frontmatter+body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	arts := []harvest.Artifact{
		{
			ID:          "harvest-cand-a",
			Type:        "learning",
			SourceRig:   "rig-a",
			SourcePath:  srcA,
			ContentHash: contentHash,
			Confidence:  0.9,
			Date:        "2026-04-30",
		},
		{
			ID:          "harvest-cand-b",
			Type:        "learning",
			SourceRig:   "rig-b",
			SourcePath:  srcB,
			ContentHash: contentHash,
			Confidence:  0.9,
			Date:        "2026-04-30",
		},
	}

	cat := harvest.BuildCatalog(arts, 0.5)
	if len(cat.Promoted) != 1 {
		t.Fatalf("BuildCatalog promoted %d artifacts, want 1 (content-hash dedup)", len(cat.Promoted))
	}

	count, err := harvest.Promote(cat, destDir, false)
	if err != nil {
		t.Fatalf("harvest.Promote failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("harvest.Promote count=%d, want 1", count)
	}

	mdCount := countMDFiles(t, filepath.Join(destDir, "learning"))
	if mdCount != 1 {
		t.Fatalf("harvest destDir holds %d .md files, want exactly 1", mdCount)
	}
}

func assertPoolPromoteDedups(t *testing.T, body string) {
	t.Helper()
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	// Two candidates with identical body but different IDs. pool.Promote
	// must dedup by content hash.
	for _, id := range []string{"pool-cand-a", "pool-cand-b"} {
		candidate := types.Candidate{
			ID:      id,
			Tier:    types.TierSilver,
			Type:    types.KnowledgeTypeLearning,
			Content: body,
		}
		if err := p.Add(candidate, types.Scoring{}); err != nil {
			t.Fatalf("pool.Add(%s) failed: %v", id, err)
		}
		if err := p.Stage(id, types.TierBronze); err != nil {
			t.Fatalf("pool.Stage(%s) failed: %v", id, err)
		}
	}

	pathA, err := p.Promote("pool-cand-a")
	if err != nil {
		t.Fatalf("first pool.Promote failed: %v", err)
	}
	pathB, err := p.Promote("pool-cand-b")
	if err != nil {
		t.Fatalf("second pool.Promote failed: %v", err)
	}
	if pathB != pathA {
		t.Fatalf("pool.Promote returned different paths for body-equal candidates: %q vs %q", pathA, pathB)
	}

	mdCount := countMDFiles(t, filepath.Dir(pathA))
	if mdCount != 1 {
		t.Fatalf("pool destDir holds %d .md files, want exactly 1", mdCount)
	}
}

// countMDFiles counts non-directory entries in dir whose name ends in .md.
// Returns 0 if dir does not exist (treated as zero promoted files).
func countMDFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}
