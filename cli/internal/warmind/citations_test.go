package warmind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCitationTracker_RecordAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	// Create a test artifact
	artifactDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(artifactDir, 0700)
	artifactPath := filepath.Join(artifactDir, "test-001.md")
	os.WriteFile(artifactPath, []byte(`---
author: testuser
author_email: test@example.com
---
# Test Learning
`), 0644)

	t.Run("record citation creates file", func(t *testing.T) {
		err := tracker.RecordCitation(artifactPath, "test-001", "test query", "session-123")
		if err != nil {
			t.Fatalf("RecordCitation failed: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(tracker.CitationsFile); os.IsNotExist(err) {
			t.Error("Citations file was not created")
		}
	})

	t.Run("load citations returns recorded citation", func(t *testing.T) {
		citations, err := tracker.LoadAll()
		if err != nil {
			t.Fatalf("LoadAll failed: %v", err)
		}

		if len(citations) == 0 {
			t.Fatal("No citations loaded")
		}

		c := citations[0]
		if c.ArtifactID != "test-001" {
			t.Errorf("ArtifactID = %v, want test-001", c.ArtifactID)
		}
		if c.Query != "test query" {
			t.Errorf("Query = %v, want 'test query'", c.Query)
		}
		if c.SessionID != "session-123" {
			t.Errorf("SessionID = %v, want session-123", c.SessionID)
		}
	})
}

func TestCitationTracker_GetCitations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	// Create test artifacts
	artifactDir := filepath.Join(tmpDir, ".warmind", "learnings")
	os.MkdirAll(artifactDir, 0700)

	artifact1 := filepath.Join(artifactDir, "artifact-001.md")
	os.WriteFile(artifact1, []byte("---\nauthor: user1\n---\n# Artifact 1"), 0644)

	artifact2 := filepath.Join(artifactDir, "artifact-002.md")
	os.WriteFile(artifact2, []byte("---\nauthor: user2\n---\n# Artifact 2"), 0644)

	// Record citations for different artifacts
	tracker.RecordCitation(artifact1, "artifact-001", "query1", "s1")
	tracker.RecordCitation(artifact1, "artifact-001", "query2", "s2")
	tracker.RecordCitation(artifact2, "artifact-002", "query3", "s3")

	t.Run("get citations for specific artifact", func(t *testing.T) {
		citations, err := tracker.GetCitations("artifact-001")
		if err != nil {
			t.Fatalf("GetCitations failed: %v", err)
		}

		if len(citations) != 2 {
			t.Errorf("Expected 2 citations for artifact-001, got %d", len(citations))
		}
	})

	t.Run("get citations for artifact with none", func(t *testing.T) {
		citations, err := tracker.GetCitations("nonexistent")
		if err != nil {
			t.Fatalf("GetCitations failed: %v", err)
		}

		if len(citations) != 0 {
			t.Errorf("Expected 0 citations for nonexistent artifact, got %d", len(citations))
		}
	})
}

func TestCitationTracker_GetOtherCitationCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	// Manually create citations file with mixed self/other citations
	citationsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(citationsDir, 0700)

	citationsContent := `{"artifact_id":"test-001","cited_by":"alice","is_self_citation":false}
{"artifact_id":"test-001","cited_by":"bob","is_self_citation":false}
{"artifact_id":"test-001","cited_by":"author","is_self_citation":true}
{"artifact_id":"test-002","cited_by":"charlie","is_self_citation":false}
`
	os.WriteFile(tracker.CitationsFile, []byte(citationsContent), 0644)

	t.Run("counts only non-self citations", func(t *testing.T) {
		count, err := tracker.GetOtherCitationCount("test-001", "author@example.com")
		if err != nil {
			t.Fatalf("GetOtherCitationCount failed: %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 other citations, got %d", count)
		}
	})
}

func TestCitationTracker_Prune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	// Manually create citations with different timestamps
	citationsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(citationsDir, 0700)

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	citationsContent := `{"artifact_id":"old-001","cited_at":"` + old.Format(time.RFC3339) + `"}
{"artifact_id":"recent-001","cited_at":"` + recent.Format(time.RFC3339) + `"}
{"artifact_id":"old-002","cited_at":"` + old.Format(time.RFC3339) + `"}
`
	os.WriteFile(tracker.CitationsFile, []byte(citationsContent), 0644)

	t.Run("prunes old citations", func(t *testing.T) {
		pruned, err := tracker.Prune(24 * time.Hour)
		if err != nil {
			t.Fatalf("Prune failed: %v", err)
		}

		if pruned != 2 {
			t.Errorf("Expected to prune 2 citations, pruned %d", pruned)
		}

		// Verify only recent citation remains
		remaining, _ := tracker.LoadAll()
		if len(remaining) != 1 {
			t.Errorf("Expected 1 remaining citation, got %d", len(remaining))
		}
	})
}

func TestCitationTracker_GetLastCitedAt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	citationsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(citationsDir, 0700)

	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)

	citationsContent := `{"artifact_id":"test-001","cited_at":"` + t1.Format(time.RFC3339) + `"}
{"artifact_id":"test-001","cited_at":"` + t2.Format(time.RFC3339) + `"}
`
	os.WriteFile(tracker.CitationsFile, []byte(citationsContent), 0644)

	t.Run("returns latest citation time", func(t *testing.T) {
		lastCited, err := tracker.GetLastCitedAt("test-001")
		if err != nil {
			t.Fatalf("GetLastCitedAt failed: %v", err)
		}

		if lastCited == nil {
			t.Fatal("Expected non-nil lastCited")
		}

		// Allow 1 second tolerance for time comparison
		if lastCited.Sub(t2) > time.Second || t2.Sub(*lastCited) > time.Second {
			t.Errorf("LastCitedAt = %v, want approximately %v", lastCited, t2)
		}
	})

	t.Run("returns nil for artifact with no citations", func(t *testing.T) {
		lastCited, err := tracker.GetLastCitedAt("nonexistent")
		if err != nil {
			t.Fatalf("GetLastCitedAt failed: %v", err)
		}

		if lastCited != nil {
			t.Errorf("Expected nil for nonexistent artifact, got %v", lastCited)
		}
	})
}

func TestCitationTracker_GetCitationStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	citationsDir := filepath.Join(tmpDir, ".warmind")
	os.MkdirAll(citationsDir, 0700)

	citationsContent := `{"artifact_id":"art-001","cited_by":"alice","is_self_citation":false}
{"artifact_id":"art-001","cited_by":"bob","is_self_citation":false}
{"artifact_id":"art-002","cited_by":"alice","is_self_citation":true}
{"artifact_id":"art-002","cited_by":"charlie","is_self_citation":false}
`
	os.WriteFile(tracker.CitationsFile, []byte(citationsContent), 0644)

	t.Run("computes correct stats", func(t *testing.T) {
		stats, err := tracker.GetCitationStats()
		if err != nil {
			t.Fatalf("GetCitationStats failed: %v", err)
		}

		if stats.Total != 4 {
			t.Errorf("Total = %d, want 4", stats.Total)
		}
		if stats.SelfCount != 1 {
			t.Errorf("SelfCount = %d, want 1", stats.SelfCount)
		}
		if stats.OtherCount != 3 {
			t.Errorf("OtherCount = %d, want 3", stats.OtherCount)
		}
		if stats.ByCiter["alice"] != 2 {
			t.Errorf("ByCiter[alice] = %d, want 2", stats.ByCiter["alice"])
		}
		if stats.ByArtifact["art-001"] != 2 {
			t.Errorf("ByArtifact[art-001] = %d, want 2", stats.ByArtifact["art-001"])
		}
	})
}

func TestGetArtifactAuthor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name          string
		content       string
		expectedName  string
		expectedEmail string
	}{
		{
			name: "extracts author and email",
			content: `---
author: testuser
author_email: test@example.com
---
# Content`,
			expectedName:  "testuser",
			expectedEmail: "test@example.com",
		},
		{
			name: "handles quoted values",
			content: `---
author: "Test User"
author_email: "test@example.com"
---
# Content`,
			expectedName:  "Test User",
			expectedEmail: "test@example.com",
		},
		{
			name: "handles missing email",
			content: `---
author: testuser
---
# Content`,
			expectedName:  "testuser",
			expectedEmail: "",
		},
		{
			name:          "handles no frontmatter",
			content:       "# Just content\nNo frontmatter here.",
			expectedName:  "",
			expectedEmail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".md")
			os.WriteFile(path, []byte(tt.content), 0644)

			name, email := getArtifactAuthor(path)

			if name != tt.expectedName {
				t.Errorf("author = %q, want %q", name, tt.expectedName)
			}
			if email != tt.expectedEmail {
				t.Errorf("author_email = %q, want %q", email, tt.expectedEmail)
			}
		})
	}
}

func TestCitationTracker_LoadAll_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-citations-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.CitationsFile = ".warmind/citations.jsonl"
	tracker := NewCitationTracker(tmpDir, cfg)

	t.Run("returns nil for nonexistent file", func(t *testing.T) {
		citations, err := tracker.LoadAll()
		if err != nil {
			t.Fatalf("LoadAll failed: %v", err)
		}
		if citations != nil {
			t.Errorf("Expected nil for nonexistent file, got %v", citations)
		}
	})

	t.Run("returns empty for empty file", func(t *testing.T) {
		citationsDir := filepath.Join(tmpDir, ".warmind")
		os.MkdirAll(citationsDir, 0700)
		os.WriteFile(tracker.CitationsFile, []byte(""), 0644)

		citations, err := tracker.LoadAll()
		if err != nil {
			t.Fatalf("LoadAll failed: %v", err)
		}
		if len(citations) != 0 {
			t.Errorf("Expected 0 citations for empty file, got %d", len(citations))
		}
	})
}
