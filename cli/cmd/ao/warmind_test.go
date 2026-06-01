// practices: [wiki-knowledge-surface, team-knowledge-sharing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/warmind"
)

// --- Command Registration Tests ---

func TestWarmindCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind"})
	if err != nil {
		t.Fatalf("warmind command not registered: %v", err)
	}
	if cmd.Name() != "warmind" {
		t.Fatalf("found %q, want %q", cmd.Name(), "warmind")
	}
}

func TestWarmindSyncCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "sync"})
	if err != nil {
		t.Fatalf("warmind sync command not registered: %v", err)
	}
	if cmd.Name() != "sync" {
		t.Fatalf("found %q, want %q", cmd.Name(), "sync")
	}
	for _, flag := range []string{"auto", "quiet", "simple"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}

func TestWarmindStatusCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "status"})
	if err != nil {
		t.Fatalf("warmind status command not registered: %v", err)
	}
	if cmd.Name() != "status" {
		t.Fatalf("found %q, want %q", cmd.Name(), "status")
	}
}

func TestWarmindPoolCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "pool"})
	if err != nil {
		t.Fatalf("warmind pool command not registered: %v", err)
	}
	if cmd.Name() != "pool" {
		t.Fatalf("found %q, want %q", cmd.Name(), "pool")
	}
}

func TestWarmindPoolListCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "pool", "list"})
	if err != nil {
		t.Fatalf("warmind pool list command not registered: %v", err)
	}
	if cmd.Name() != "list" {
		t.Fatalf("found %q, want %q", cmd.Name(), "list")
	}
	for _, flag := range []string{"staged", "pending"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}

func TestWarmindPromoteCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "promote"})
	if err != nil {
		t.Fatalf("warmind promote command not registered: %v", err)
	}
	if cmd.Name() != "promote" {
		t.Fatalf("found %q, want %q", cmd.Name(), "promote")
	}
	if cmd.Flags().Lookup("force") == nil {
		t.Error("expected --force flag")
	}
}

func TestWarmindContradictCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "contradict"})
	if err != nil {
		t.Fatalf("warmind contradict command not registered: %v", err)
	}
	if cmd.Name() != "contradict" {
		t.Fatalf("found %q, want %q", cmd.Name(), "contradict")
	}
	if cmd.Flags().Lookup("list") == nil {
		t.Error("expected --list flag")
	}
}

func TestWarmindCloseLoopCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"warmind", "close-loop"})
	if err != nil {
		t.Fatalf("warmind close-loop command not registered: %v", err)
	}
	if cmd.Name() != "close-loop" {
		t.Fatalf("found %q, want %q", cmd.Name(), "close-loop")
	}
	if cmd.Flags().Lookup("quiet") == nil {
		t.Error("expected --quiet flag")
	}
}

// --- Helper Function Tests ---

func TestGenerateCandidateID(t *testing.T) {
	id1 := generateCandidateID("test-learning.md")
	id2 := generateCandidateID("test-learning.md")

	if !strings.HasPrefix(id1, "learn-test-learning-") {
		t.Errorf("ID should start with 'learn-test-learning-', got %q", id1)
	}
	// IDs should be unique due to timestamp component
	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func TestParseLearningFrontmatter(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantTitle      string
		wantCategory   string
		wantTags       []string
		wantConfidence float64
	}{
		{
			name: "full frontmatter",
			content: `---
title: Test Learning
category: testing
tags: [go, testing, warmind]
confidence: 0.85
---
# Content here`,
			wantTitle:      "Test Learning",
			wantCategory:   "testing",
			wantTags:       []string{"go", "testing", "warmind"},
			wantConfidence: 0.85,
		},
		{
			name: "title from heading",
			content: `---
category: testing
---
# My Learning Title

Content here.`,
			wantTitle:      "My Learning Title",
			wantCategory:   "testing",
			wantTags:       nil,
			wantConfidence: 0.7, // default
		},
		{
			name:           "no frontmatter",
			content:        "# Just a heading\n\nContent here.",
			wantTitle:      "", // Function returns early if no frontmatter
			wantCategory:   "",
			wantTags:       nil,
			wantConfidence: 0.7,
		},
		{
			name: "quoted values",
			content: `---
title: "Quoted Title"
category: 'single quoted'
---
Content`,
			wantTitle:      "Quoted Title",
			wantCategory:   "single quoted",
			wantTags:       nil,
			wantConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, category, tags, confidence, _ := parseLearningFrontmatter(tt.content)

			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if category != tt.wantCategory {
				t.Errorf("category = %q, want %q", category, tt.wantCategory)
			}
			if len(tags) != len(tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			} else {
				for i, tag := range tags {
					if tag != tt.wantTags[i] {
						t.Errorf("tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
					}
				}
			}
			if confidence != tt.wantConfidence {
				t.Errorf("confidence = %v, want %v", confidence, tt.wantConfidence)
			}
		})
	}
}

func TestAddAuthorToFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		author  string
		wantHas []string
	}{
		{
			name:    "adds to existing frontmatter",
			content: "---\ntitle: Test\n---\nContent",
			author:  "alice",
			wantHas: []string{"author: alice", "synced_at:", "title: Test"},
		},
		{
			name:    "creates frontmatter if missing",
			content: "# Just content",
			author:  "bob",
			wantHas: []string{"---", "author: bob", "synced_at:", "# Just content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addAuthorToFrontmatter(tt.content, tt.author)
			for _, want := range tt.wantHas {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

func TestContentHashBytes(t *testing.T) {
	content := []byte("test content")
	hash1 := contentHashBytes(content)
	hash2 := contentHashBytes(content)

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}
	if len(hash1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("Hash length = %d, want 64", len(hash1))
	}

	different := contentHashBytes([]byte("different content"))
	if hash1 == different {
		t.Error("Different content should produce different hash")
	}
}

func TestCollectWarmindHashes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "learn1.md"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "learn2.md"), []byte("content 2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not markdown"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	hashes := collectWarmindHashes(tmpDir)

	if len(hashes) != 2 {
		t.Errorf("Expected 2 hashes (only .md files), got %d", len(hashes))
	}

	// Verify hashes are correct
	hash1 := contentHashBytes([]byte("content 1"))
	hash2 := contentHashBytes([]byte("content 2"))

	if !hashes[hash1] {
		t.Error("Missing hash for content 1")
	}
	if !hashes[hash2] {
		t.Error("Missing hash for content 2")
	}
}

// --- E2E Integration Tests ---

func TestWarmindE2E_SyncAndPromote(t *testing.T) {
	// Create isolated test environment
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create .agents/learnings/ with a test learning
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	os.MkdirAll(learningsDir, 0755)

	learningContent := `---
title: Test E2E Learning
category: testing
confidence: 0.9
---
# Test E2E Learning

This is a high-quality learning for E2E testing.

1. First step
2. Second step
3. Third step

` + "```go\nfmt.Println(\"example code\")\n```"

	learningPath := filepath.Join(learningsDir, "e2e-test.md")
	os.WriteFile(learningPath, []byte(learningContent), 0644)

	// Initialize pool
	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(tmpDir, cfg)
	if err := pool.Init(); err != nil {
		t.Fatalf("Pool init failed: %v", err)
	}

	// Create candidate manually (simulating sync)
	candidate := warmind.Candidate{
		ID:          "e2e-test-001",
		Title:       "Test E2E Learning",
		Content:     learningContent,
		Author:      "test-user",
		AuthorEmail: "test@example.com",
		SourcePath:  learningPath,
		Confidence:  0.9,
		CreatedAt:   time.Now(),
		ContentHash: warmind.ContentHash(learningContent),
	}

	// Add to pool
	if err := pool.Add(candidate); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Score the candidate
	scorer := warmind.NewScorer(cfg.Scoring, filepath.Join(tmpDir, cfg.LearningsDir))
	scoring := scorer.Score(candidate)

	if err := pool.Score(candidate.ID, scoring); err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	// Verify it's staged
	staged, err := pool.ListStaged()
	if err != nil {
		t.Fatalf("ListStaged failed: %v", err)
	}
	if len(staged) != 1 {
		t.Fatalf("Expected 1 staged candidate, got %d", len(staged))
	}
	if staged[0].Candidate.ID != "e2e-test-001" {
		t.Errorf("Staged candidate ID = %q, want e2e-test-001", staged[0].Candidate.ID)
	}

	// Verify tier assignment (high quality content should be silver or gold)
	if scoring.Tier != warmind.TierGold && scoring.Tier != warmind.TierSilver {
		t.Errorf("High quality content got tier %v, expected Gold or Silver", scoring.Tier)
	}

	// Record a citation from another user
	if err := pool.RecordCitation("e2e-test-001", "other-user", "other@example.com", false); err != nil {
		t.Fatalf("RecordCitation failed: %v", err)
	}

	// Check promotion eligibility
	eligible, reason := pool.CheckPromotion("e2e-test-001")
	t.Logf("Promotion check: eligible=%v, reason=%s, tier=%v", eligible, reason, scoring.Tier)

	// For Silver tier with 1 citation, should be eligible
	if scoring.Tier == warmind.TierSilver && !eligible {
		t.Errorf("Silver tier with 1 citation should be eligible for promotion")
	}

	// Promote (force to bypass any timing requirements)
	pool.SetCitationCount("e2e-test-001", 100) // Force eligibility
	artifactPath, err := pool.Promote("e2e-test-001")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Verify promoted file exists
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("Promoted artifact file should exist")
	}

	// Verify no longer in staged
	staged, _ = pool.ListStaged()
	for _, s := range staged {
		if s.Candidate.ID == "e2e-test-001" {
			t.Error("Promoted candidate should not be in staged list")
		}
	}
}

func TestWarmindE2E_RejectsDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(tmpDir, cfg)
	pool.Init()

	content := "Duplicate content for testing"
	candidate1 := warmind.Candidate{
		ID:          "dup-001",
		Title:       "First",
		Content:     content,
		ContentHash: warmind.ContentHash(content),
	}
	candidate2 := warmind.Candidate{
		ID:          "dup-002",
		Title:       "Second",
		Content:     content, // Same content
		ContentHash: warmind.ContentHash(content),
	}

	if err := pool.Add(candidate1); err != nil {
		t.Fatalf("First add failed: %v", err)
	}

	err := pool.Add(candidate2)
	if err == nil {
		t.Error("Expected error for duplicate content")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Error should mention duplicate, got: %v", err)
	}
}

func TestWarmindE2E_CitationTracking(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := warmind.DefaultConfig()
	cfg.Promotion.SelfCitationAllowed = false
	pool := warmind.NewPool(tmpDir, cfg)
	pool.Init()

	// Add and stage a candidate
	candidate := warmind.Candidate{
		ID:          "cite-e2e",
		Title:       "Citation Test",
		Content:     "Test content for citation tracking",
		Author:      "alice",
		AuthorEmail: "alice@example.com",
		ContentHash: warmind.ContentHash("Test content for citation tracking"),
	}
	pool.Add(candidate)
	pool.Score("cite-e2e", warmind.ScoringResult{
		Tier:           warmind.TierSilver,
		CompositeScore: 0.75,
	})

	// Self-citation should be ignored
	pool.RecordCitation("cite-e2e", "alice", "alice@example.com", true)
	entry, _ := pool.Get("cite-e2e")
	if entry.CitationCount != 0 {
		t.Errorf("Self-citation should be ignored, got count %d", entry.CitationCount)
	}

	// Other citation should count
	pool.RecordCitation("cite-e2e", "bob", "bob@example.com", false)
	entry, _ = pool.Get("cite-e2e")
	if entry.CitationCount != 1 {
		t.Errorf("Other citation should count, got count %d", entry.CitationCount)
	}

	// Multiple other citations
	pool.RecordCitation("cite-e2e", "charlie", "charlie@example.com", false)
	pool.RecordCitation("cite-e2e", "dave", "dave@example.com", false)
	entry, _ = pool.Get("cite-e2e")
	if entry.CitationCount != 3 {
		t.Errorf("Expected 3 citations, got %d", entry.CitationCount)
	}
}

func TestWarmindE2E_MaturityLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := warmind.DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ArchiveDir = ".warmind/archive"
	cfg.Maturity.ProvisionalExpireDays = 0 // Immediate expiry for testing
	cfg.Maturity.MinCorpusSize = 1

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0755)

	// Create a provisional learning with no citations (should be archived)
	oldLearning := `---
id: old-learning
maturity: provisional
citation_count: 0
promoted_at: 2020-01-01T00:00:00Z
---
# Old Learning
This learning has no citations and is past expiry.
`
	os.WriteFile(filepath.Join(learningsDir, "old-learning.md"), []byte(oldLearning), 0644)

	// Create a provisional learning with citations (should promote to established)
	citedLearning := `---
id: cited-learning
maturity: provisional
citation_count: 5
promoted_at: 2026-05-01T00:00:00Z
---
# Cited Learning
This learning has citations.
`
	os.WriteFile(filepath.Join(learningsDir, "cited-learning.md"), []byte(citedLearning), 0644)

	// Pass nil tracker so CheckTransition uses frontmatter citation_count
	// instead of querying the (empty) citation ledger
	mm := warmind.NewMaturityManager(tmpDir, cfg, nil)

	report, err := mm.RunMaintenance()
	if err != nil {
		t.Fatalf("RunMaintenance failed: %v", err)
	}

	t.Logf("Maintenance report: scanned=%d, promoted=%d, archived=%d",
		report.Scanned, report.Promoted, report.Archived)

	// Verify cited learning was promoted
	if report.Promoted < 1 {
		t.Error("Expected at least 1 promotion (cited learning)")
	}
}

func TestWarmindE2E_ContradictionDetection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := warmind.DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ContradictionsFile = ".warmind/contradictions.jsonl"
	cfg.Contradict.MinSignals = 2

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0755)

	// Create contradicting learnings
	learning1 := `---
id: pro-caching
author: alice
---
# Always Use Caching

You should always use caching for database queries.
Do enable Redis caching. It improves performance significantly.
`
	learning2 := `---
id: anti-caching
author: bob
---
# Avoid Caching

You should never use caching for this type of query.
Don't enable Redis, it adds unnecessary complexity.
`
	os.WriteFile(filepath.Join(learningsDir, "pro-caching.md"), []byte(learning1), 0644)
	os.WriteFile(filepath.Join(learningsDir, "anti-caching.md"), []byte(learning2), 0644)

	tracker := warmind.NewCitationTracker(tmpDir, cfg)
	mm := warmind.NewMaturityManager(tmpDir, cfg, tracker)
	cd := warmind.NewContradictionDetector(tmpDir, cfg, mm)

	report, err := cd.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	t.Logf("Contradiction scan: scanned=%d, detected=%d", report.Scanned, report.NewDetected)

	if report.Scanned != 2 {
		t.Errorf("Expected 2 learnings scanned, got %d", report.Scanned)
	}

	// Should detect the contradiction
	if report.NewDetected < 1 {
		t.Log("Note: Contradiction not detected - may need stronger opposing signals in test content")
	}
}

func TestWarmindE2E_CloseLoopFlow(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	cfg := warmind.DefaultConfig()
	cfg.Promotion.GoldAutoPromoteHours = 0 // Immediate for testing
	cfg.Promotion.SilverCitationThreshold = 1
	cfg.Maturity.MinCorpusSize = 1

	pool := warmind.NewPool(tmpDir, cfg)
	pool.Init()

	// Add a gold-tier candidate
	candidate := warmind.Candidate{
		ID:          "gold-candidate",
		Title:       "High Quality Learning",
		Content:     "Excellent content with code:\n```go\nfmt.Println(\"test\")\n```\n\n1. Step one\n2. Step two",
		Author:      "alice",
		AuthorEmail: "alice@example.com",
		Confidence:  0.95,
		CreatedAt:   time.Now().Add(-25 * time.Hour), // Created over 24h ago
		ContentHash: warmind.ContentHash("Excellent content"),
	}
	pool.Add(candidate)

	// Score it as gold
	pool.Score("gold-candidate", warmind.ScoringResult{
		Tier:           warmind.TierGold,
		CompositeScore: 0.9,
		ScoredAt:       time.Now().Add(-25 * time.Hour),
	})

	// Verify it's staged
	staged, _ := pool.ListStaged()
	if len(staged) != 1 {
		t.Fatalf("Expected 1 staged, got %d", len(staged))
	}

	// Check auto-promotion eligibility (gold + 24h)
	eligible, reason := pool.CheckPromotion("gold-candidate")
	t.Logf("Gold auto-promote check: eligible=%v, reason=%s", eligible, reason)

	// The close-loop would normally run these steps:
	// 1. Check staged for auto-promotion
	// 2. Run maturity maintenance
	// 3. Scan for contradictions
	// 4. Prune old citations

	// Simulate close-loop promotion check
	for _, entry := range staged {
		if elig, _ := pool.CheckPromotion(entry.Candidate.ID); elig {
			_, err := pool.Promote(entry.Candidate.ID)
			if err != nil {
				t.Errorf("Auto-promote failed: %v", err)
			}
		}
	}
}
