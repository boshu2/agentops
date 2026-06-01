package warmind

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeDecay(t *testing.T) {
	cfg := DefaultConfig()
	manager := &MaturityManager{Config: cfg.Maturity}

	tests := []struct {
		name        string
		weeksAgo    float64
		minExpected float64
		maxExpected float64
	}{
		{"brand new", 0, 0.95, 1.0},
		{"one week old", 1, 0.8, 0.9},
		{"one month old", 4, 0.4, 0.6},
		{"very old floors at 0.1", 52, 0.1, 0.11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promotedAt := time.Now().Add(-time.Duration(tt.weeksAgo*7*24) * time.Hour)
			got := manager.ComputeDecay(promotedAt)

			if got < tt.minExpected || got > tt.maxExpected {
				t.Errorf("ComputeDecay(%v weeks ago) = %v, want between %v and %v",
					tt.weeksAgo, got, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestComputeUtility(t *testing.T) {
	cfg := DefaultConfig()
	manager := &MaturityManager{Config: cfg.Maturity}

	t.Run("fresh learning with citations gets bonus", func(t *testing.T) {
		meta := LearningMetadata{
			PromotedAt:    time.Now(),
			CitationCount: 5,
		}

		utility := manager.ComputeUtility(meta)

		// Freshness ~1.0, citation bonus 1.5 (5 * 0.1 + 1.0)
		if utility < 1.4 || utility > 1.6 {
			t.Errorf("Utility = %v, want approximately 1.5", utility)
		}
	})

	t.Run("old learning with no citations has low utility", func(t *testing.T) {
		meta := LearningMetadata{
			PromotedAt:    time.Now().Add(-8 * 7 * 24 * time.Hour), // 8 weeks
			CitationCount: 0,
		}

		utility := manager.ComputeUtility(meta)

		// Should be low due to decay
		if utility > 0.5 {
			t.Errorf("Utility = %v, want < 0.5 for old uncited learning", utility)
		}
	})

	t.Run("citation bonus caps at 2x", func(t *testing.T) {
		meta := LearningMetadata{
			PromotedAt:    time.Now(),
			CitationCount: 100, // Way more than needed for cap
		}

		utility := manager.ComputeUtility(meta)

		// Freshness ~1.0, citation bonus capped at 2.0
		if utility > 2.1 {
			t.Errorf("Utility = %v, want <= 2.0 (citation bonus should cap)", utility)
		}
	})
}

func TestCheckTransition(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("provisional with 3+ citations promotes to established", func(t *testing.T) {
		manager := &MaturityManager{Config: cfg.Maturity}

		meta := LearningMetadata{
			Maturity:      MaturityProvisional,
			CitationCount: 3,
			PromotedAt:    time.Now(),
		}

		newMaturity, reason := manager.CheckTransition(meta)

		if newMaturity != MaturityEstablished {
			t.Errorf("newMaturity = %v, want established", newMaturity)
		}
		if reason == "" {
			t.Error("Expected reason for promotion")
		}
	})

	t.Run("provisional with 0 citations after expiry archives", func(t *testing.T) {
		manager := &MaturityManager{Config: cfg.Maturity}

		meta := LearningMetadata{
			Maturity:      MaturityProvisional,
			CitationCount: 0,
			PromotedAt:    time.Now().Add(-time.Duration(cfg.Maturity.ProvisionalExpireDays+1) * 24 * time.Hour),
		}

		newMaturity, reason := manager.CheckTransition(meta)

		if newMaturity != "archive" {
			t.Errorf("newMaturity = %v, want archive", newMaturity)
		}
		if reason == "" {
			t.Error("Expected reason for archival")
		}
	})

	t.Run("provisional within expiry window stays provisional", func(t *testing.T) {
		manager := &MaturityManager{Config: cfg.Maturity}

		meta := LearningMetadata{
			Maturity:      MaturityProvisional,
			CitationCount: 1, // Some citations but not enough to promote
			PromotedAt:    time.Now().Add(-7 * 24 * time.Hour), // 1 week ago
		}

		newMaturity, reason := manager.CheckTransition(meta)

		if newMaturity != MaturityProvisional {
			t.Errorf("newMaturity = %v, want provisional", newMaturity)
		}
		if reason != "" {
			t.Errorf("Expected no reason (no transition), got %q", reason)
		}
	})

	t.Run("established with recent citation stays established", func(t *testing.T) {
		manager := &MaturityManager{Config: cfg.Maturity}

		recentCite := time.Now().Add(-7 * 24 * time.Hour)
		meta := LearningMetadata{
			Maturity:    MaturityEstablished,
			PromotedAt:  time.Now().Add(-60 * 24 * time.Hour), // 60 days ago
			LastCitedAt: &recentCite,
		}

		newMaturity, reason := manager.CheckTransition(meta)

		if newMaturity != MaturityEstablished {
			t.Errorf("newMaturity = %v, want established", newMaturity)
		}
		if reason != "" {
			t.Errorf("Expected no reason (no transition), got %q", reason)
		}
	})

	t.Run("established with no recent citations archives", func(t *testing.T) {
		manager := &MaturityManager{Config: cfg.Maturity}

		oldCite := time.Now().Add(-time.Duration(cfg.Maturity.EstablishedExpireDays+1) * 24 * time.Hour)
		meta := LearningMetadata{
			Maturity:    MaturityEstablished,
			PromotedAt:  time.Now().Add(-120 * 24 * time.Hour), // 120 days ago
			LastCitedAt: &oldCite,
		}

		newMaturity, reason := manager.CheckTransition(meta)

		if newMaturity != "archive" {
			t.Errorf("newMaturity = %v, want archive", newMaturity)
		}
		if reason == "" {
			t.Error("Expected reason for archival")
		}
	})
}

func TestScanLearnings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-maturity-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ArchiveDir = ".warmind/archive"

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0700)

	// Create test learnings
	learning1 := `---
id: learn-001
author: alice
author_email: alice@example.com
maturity: provisional
citation_count: 2
promoted_at: 2026-05-20T10:00:00Z
---
# Learning 1
Content here.
`
	os.WriteFile(filepath.Join(learningsDir, "learn-001.md"), []byte(learning1), 0644)

	learning2 := `---
id: learn-002
author: bob
maturity: established
citation_count: 5
promoted_at: 2026-05-01T10:00:00Z
---
# Learning 2
More content.
`
	os.WriteFile(filepath.Join(learningsDir, "learn-002.md"), []byte(learning2), 0644)

	manager := NewMaturityManager(tmpDir, cfg, nil)

	t.Run("scans all learning files", func(t *testing.T) {
		learnings, err := manager.ScanLearnings()
		if err != nil {
			t.Fatalf("ScanLearnings failed: %v", err)
		}

		if len(learnings) != 2 {
			t.Errorf("Expected 2 learnings, got %d", len(learnings))
		}
	})

	t.Run("parses metadata correctly", func(t *testing.T) {
		learnings, _ := manager.ScanLearnings()

		var learn1 *LearningMetadata
		for i := range learnings {
			if learnings[i].ID == "learn-001" {
				learn1 = &learnings[i]
				break
			}
		}

		if learn1 == nil {
			t.Fatal("Could not find learn-001")
		}

		if learn1.Author != "alice" {
			t.Errorf("Author = %q, want alice", learn1.Author)
		}
		if learn1.Maturity != MaturityProvisional {
			t.Errorf("Maturity = %v, want provisional", learn1.Maturity)
		}
		if learn1.CitationCount != 2 {
			t.Errorf("CitationCount = %d, want 2", learn1.CitationCount)
		}
	})
}

func TestRunMaintenance(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-maturity-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ArchiveDir = ".warmind/archive"
	cfg.Maturity.MinCorpusSize = 2 // Lower for testing

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0700)

	t.Run("skips if corpus too small", func(t *testing.T) {
		cfgSmall := cfg
		cfgSmall.Maturity.MinCorpusSize = 100

		manager := NewMaturityManager(tmpDir, cfgSmall, nil)

		// Create just one learning
		os.WriteFile(filepath.Join(learningsDir, "single.md"), []byte(`---
id: single
maturity: provisional
promoted_at: 2026-05-01T10:00:00Z
---
# Single
`), 0644)

		report, err := manager.RunMaintenance()
		if err != nil {
			t.Fatalf("RunMaintenance failed: %v", err)
		}

		if report.SkippedReason == "" {
			t.Error("Expected skip reason for small corpus")
		}

		// Cleanup
		os.Remove(filepath.Join(learningsDir, "single.md"))
	})

	t.Run("promotes provisional with enough citations", func(t *testing.T) {
		manager := NewMaturityManager(tmpDir, cfg, nil)

		// Create learnings - one ready for promotion
		learning1 := `---
id: promote-me
maturity: provisional
citation_count: 5
promoted_at: 2026-05-20T10:00:00Z
---
# Should Promote
`
		learning2 := `---
id: stay-provisional
maturity: provisional
citation_count: 1
promoted_at: 2026-05-20T10:00:00Z
---
# Should Stay
`
		os.WriteFile(filepath.Join(learningsDir, "promote-me.md"), []byte(learning1), 0644)
		os.WriteFile(filepath.Join(learningsDir, "stay-provisional.md"), []byte(learning2), 0644)

		report, err := manager.RunMaintenance()
		if err != nil {
			t.Fatalf("RunMaintenance failed: %v", err)
		}

		if report.Promoted != 1 {
			t.Errorf("Promoted = %d, want 1", report.Promoted)
		}

		// Verify file was updated
		content, _ := os.ReadFile(filepath.Join(learningsDir, "promote-me.md"))
		if !contains(string(content), "maturity: established") {
			t.Error("Expected maturity to be updated to established")
		}

		// Cleanup
		os.Remove(filepath.Join(learningsDir, "promote-me.md"))
		os.Remove(filepath.Join(learningsDir, "stay-provisional.md"))
	})
}

func TestArchive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-maturity-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ArchiveDir = ".warmind/archive"

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	archiveDir := filepath.Join(tmpDir, ".warmind", "archive")
	os.MkdirAll(learningsDir, 0700)

	manager := NewMaturityManager(tmpDir, cfg, nil)

	t.Run("moves file to archive", func(t *testing.T) {
		learningPath := filepath.Join(learningsDir, "to-archive.md")
		os.WriteFile(learningPath, []byte(`---
id: to-archive
maturity: provisional
---
# To Archive
`), 0644)

		meta := LearningMetadata{
			ID:       "to-archive",
			FilePath: learningPath,
			Maturity: MaturityProvisional,
		}

		err := manager.archive(meta, "test archival")
		if err != nil {
			t.Fatalf("archive failed: %v", err)
		}

		// Original should not exist
		if _, err := os.Stat(learningPath); !os.IsNotExist(err) {
			t.Error("Original file should be moved")
		}

		// Archived should exist
		archivedPath := filepath.Join(archiveDir, "to-archive.md")
		if _, err := os.Stat(archivedPath); os.IsNotExist(err) {
			t.Error("Archived file should exist")
		}

		// Verify archive metadata
		content, _ := os.ReadFile(archivedPath)
		if !contains(string(content), "archived_at:") {
			t.Error("Expected archived_at in frontmatter")
		}
		if !contains(string(content), "archive_reason:") {
			t.Error("Expected archive_reason in frontmatter")
		}
	})
}

func TestDecayRate(t *testing.T) {
	// Verify the 17%/week decay rate matches expected Ebbinghaus curve
	cfg := DefaultConfig()
	if math.Abs(cfg.Maturity.DecayRate-0.17) > 0.001 {
		t.Errorf("DecayRate = %v, want 0.17 (Ebbinghaus rate)", cfg.Maturity.DecayRate)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
