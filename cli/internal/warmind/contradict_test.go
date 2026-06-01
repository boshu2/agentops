package warmind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContradictionDetector_DetectContradiction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-contradict-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ContradictionsFile = ".warmind/contradictions.jsonl"
	cfg.Contradict.MinSignals = 2 // Lower for testing

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0700)

	tracker := NewCitationTracker(tmpDir, cfg)
	mm := NewMaturityManager(tmpDir, cfg, tracker)
	cd := NewContradictionDetector(tmpDir, cfg, mm)

	t.Run("detects opposing do/don't recommendations", func(t *testing.T) {
		learning1 := `---
id: learn-001
author: alice
---
# Always Use Caching

You should always use caching for database queries.
Do use Redis for caching. It improves performance.
`
		learning2 := `---
id: learn-002
author: bob
---
# Avoid Caching

You should never use caching for this database.
Don't use Redis, it adds complexity.
`
		path1 := filepath.Join(learningsDir, "learn-001.md")
		path2 := filepath.Join(learningsDir, "learn-002.md")
		os.WriteFile(path1, []byte(learning1), 0644)
		os.WriteFile(path2, []byte(learning2), 0644)

		meta1 := LearningMetadata{ID: "learn-001", FilePath: path1, Author: "alice"}
		meta2 := LearningMetadata{ID: "learn-002", FilePath: path2, Author: "bob"}

		conflict := cd.detectContradiction(meta1, meta2)

		if conflict == nil {
			t.Error("Expected to detect contradiction between opposing learnings")
		}

		// Cleanup
		os.Remove(path1)
		os.Remove(path2)
	})

	t.Run("ignores unrelated learnings", func(t *testing.T) {
		learning1 := `---
id: learn-003
author: alice
---
# Python Best Practices

Use type hints in Python code. Always document functions.
`
		learning2 := `---
id: learn-004
author: bob
---
# Kubernetes Scaling

Use horizontal pod autoscaling. Configure resource limits.
`
		path1 := filepath.Join(learningsDir, "learn-003.md")
		path2 := filepath.Join(learningsDir, "learn-004.md")
		os.WriteFile(path1, []byte(learning1), 0644)
		os.WriteFile(path2, []byte(learning2), 0644)

		meta1 := LearningMetadata{ID: "learn-003", FilePath: path1, Author: "alice"}
		meta2 := LearningMetadata{ID: "learn-004", FilePath: path2, Author: "bob"}

		conflict := cd.detectContradiction(meta1, meta2)

		if conflict != nil {
			t.Error("Should not detect contradiction between unrelated learnings")
		}

		// Cleanup
		os.Remove(path1)
		os.Remove(path2)
	})
}

func TestContradictionDetector_Scan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-contradict-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.LearningsDir = ".warmind/learnings"
	cfg.ContradictionsFile = ".warmind/contradictions.jsonl"
	cfg.Contradict.MinSignals = 2

	learningsDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(learningsDir, 0700)

	tracker := NewCitationTracker(tmpDir, cfg)
	mm := NewMaturityManager(tmpDir, cfg, tracker)
	cd := NewContradictionDetector(tmpDir, cfg, mm)

	t.Run("scans all learnings", func(t *testing.T) {
		// Create test learnings
		os.WriteFile(filepath.Join(learningsDir, "a.md"), []byte(`---
id: a
author: alice
---
# Learning A
Use caching always. Do enable it.
`), 0644)
		os.WriteFile(filepath.Join(learningsDir, "b.md"), []byte(`---
id: b
author: bob
---
# Learning B
Never use caching. Don't enable it.
`), 0644)

		report, err := cd.Scan()
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}

		if report.Scanned != 2 {
			t.Errorf("Scanned = %d, want 2", report.Scanned)
		}
	})
}

func TestContradictionDetector_ResolveAndDismiss(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-contradict-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.ContradictionsFile = ".warmind/contradictions.jsonl"

	contradictionsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(contradictionsDir, 0700)

	// Create test contradiction
	contradictionsContent := `{"id":"contra-001","status":"pending_review"}
{"id":"contra-002","status":"pending_review"}
`
	contradictionsFile := filepath.Join(tmpDir, cfg.ContradictionsFile)
	os.WriteFile(contradictionsFile, []byte(contradictionsContent), 0644)

	cd := &ContradictionDetector{
		Config:             cfg.Contradict,
		ContradictionsFile: contradictionsFile,
	}

	t.Run("resolve marks as resolved", func(t *testing.T) {
		err := cd.Resolve("contra-001", "charlie", "Merged learnings")
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}

		all, _ := cd.LoadAll()
		var found *Contradiction
		for i := range all {
			if all[i].ID == "contra-001" {
				found = &all[i]
				break
			}
		}

		if found == nil {
			t.Fatal("Contradiction not found after resolve")
		}
		if found.Status != "resolved" {
			t.Errorf("Status = %q, want resolved", found.Status)
		}
		if found.ResolvedBy != "charlie" {
			t.Errorf("ResolvedBy = %q, want charlie", found.ResolvedBy)
		}
	})

	t.Run("dismiss marks as dismissed", func(t *testing.T) {
		err := cd.Dismiss("contra-002", "dave", "false positive")
		if err != nil {
			t.Fatalf("Dismiss failed: %v", err)
		}

		all, _ := cd.LoadAll()
		var found *Contradiction
		for i := range all {
			if all[i].ID == "contra-002" {
				found = &all[i]
				break
			}
		}

		if found == nil {
			t.Fatal("Contradiction not found after dismiss")
		}
		if found.Status != "dismissed" {
			t.Errorf("Status = %q, want dismissed", found.Status)
		}
	})

	t.Run("returns error for nonexistent contradiction", func(t *testing.T) {
		err := cd.Resolve("nonexistent", "user", "reason")
		if err == nil {
			t.Error("Expected error for nonexistent contradiction")
		}
	})
}

func TestContradictionDetector_GetPending(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-contradict-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.ContradictionsFile = ".warmind/contradictions.jsonl"

	contradictionsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(contradictionsDir, 0700)

	contradictionsContent := `{"id":"contra-001","status":"pending_review"}
{"id":"contra-002","status":"resolved"}
{"id":"contra-003","status":"pending_review"}
{"id":"contra-004","status":"dismissed"}
`
	contradictionsFile := filepath.Join(tmpDir, cfg.ContradictionsFile)
	os.WriteFile(contradictionsFile, []byte(contradictionsContent), 0644)

	cd := &ContradictionDetector{
		Config:             cfg.Contradict,
		ContradictionsFile: contradictionsFile,
	}

	t.Run("returns only pending contradictions", func(t *testing.T) {
		pending, err := cd.GetPending()
		if err != nil {
			t.Fatalf("GetPending failed: %v", err)
		}

		if len(pending) != 2 {
			t.Errorf("Expected 2 pending, got %d", len(pending))
		}

		for _, c := range pending {
			if c.Status != "pending_review" {
				t.Errorf("Expected pending_review status, got %q", c.Status)
			}
		}
	})
}

func TestAreOpposingStatements(t *testing.T) {
	cd := &ContradictionDetector{}

	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"do vs don't", "do use it", "don't use it", true},
		{"don't vs do", "don't try", "do try", true},
		{"always vs never", "always check", "never check", true},
		{"never vs always", "never skip", "always skip", true},
		{"unrelated", "run tests", "write docs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cd.areOpposingStatements(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("areOpposingStatements(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestContradictionKey(t *testing.T) {
	cd := &ContradictionDetector{}

	t.Run("produces same key regardless of order", func(t *testing.T) {
		key1 := cd.contradictionKey("path/a.md", "path/b.md")
		key2 := cd.contradictionKey("path/b.md", "path/a.md")

		if key1 != key2 {
			t.Errorf("Keys should be equal: %q vs %q", key1, key2)
		}
	})
}

func TestOpposingPairs(t *testing.T) {
	// Verify we have a reasonable number of opposing pairs
	if len(opposingPairs) < 10 {
		t.Errorf("Expected at least 10 opposing pairs, got %d", len(opposingPairs))
	}

	// Check that classic pairs exist
	expectedPairs := map[string]string{
		"always": "never",
		"do":     "don't",
		"use":    "avoid",
	}

	for a, b := range expectedPairs {
		found := false
		for _, pair := range opposingPairs {
			if pair.A == a && pair.B == b {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected opposing pair (%q, %q) not found", a, b)
		}
	}
}
