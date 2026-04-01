package provenance

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper to create a test provenance file
func createTestProvFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "graph.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	return path
}

func TestNewGraph(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		content := `{"id":"test-1","artifact_path":"/out/session.md","artifact_type":"session","source_path":"/in/transcript.jsonl","source_type":"transcript","session_id":"sess-001","created_at":"2026-01-25T10:00:00Z"}`
		path := createTestProvFile(t, content)

		g, err := NewGraph(path)
		if err != nil {
			t.Fatalf("NewGraph() error = %v", err)
		}
		if len(g.Records) != 1 {
			t.Errorf("Expected 1 record, got %d", len(g.Records))
		}
		if g.Records[0].ID != "test-1" {
			t.Errorf("Record ID = %q, want test-1", g.Records[0].ID)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		g, err := NewGraph("/nonexistent/path/graph.jsonl")
		if err != nil {
			t.Fatalf("NewGraph() should handle non-existent file: %v", err)
		}
		if len(g.Records) != 0 {
			t.Errorf("Expected 0 records for non-existent file, got %d", len(g.Records))
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := createTestProvFile(t, "")
		g, err := NewGraph(path)
		if err != nil {
			t.Fatalf("NewGraph() error = %v", err)
		}
		if len(g.Records) != 0 {
			t.Errorf("Expected 0 records for empty file, got %d", len(g.Records))
		}
	})

	t.Run("malformed lines", func(t *testing.T) {
		content := `{"id":"valid-1","artifact_path":"/out/1.md","artifact_type":"session","source_path":"/in/1.jsonl","source_type":"transcript"}
not valid json
{"id":"valid-2","artifact_path":"/out/2.md","artifact_type":"session","source_path":"/in/2.jsonl","source_type":"transcript"}`
		path := createTestProvFile(t, content)

		g, err := NewGraph(path)
		if err != nil {
			t.Fatalf("NewGraph() error = %v", err)
		}
		// Should have 2 valid records (malformed line skipped)
		if len(g.Records) != 2 {
			t.Errorf("Expected 2 valid records, got %d", len(g.Records))
		}
	})
}

func TestGraph_Trace(t *testing.T) {
	content := `{"id":"prov-1","artifact_path":"/out/session-a.md","artifact_type":"session","source_path":"/in/transcript-a.jsonl","source_type":"transcript","session_id":"sess-a"}
{"id":"prov-2","artifact_path":"/out/session-b.md","artifact_type":"session","source_path":"/in/transcript-b.jsonl","source_type":"transcript","session_id":"sess-b"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	t.Run("trace existing artifact", func(t *testing.T) {
		result, err := g.Trace("/out/session-a.md")
		if err != nil {
			t.Fatalf("Trace() error = %v", err)
		}
		if result.Artifact != "/out/session-a.md" {
			t.Errorf("Artifact = %q, want /out/session-a.md", result.Artifact)
		}
		if len(result.Chain) != 1 {
			t.Errorf("Expected 1 record in chain, got %d", len(result.Chain))
		}
		if len(result.Sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(result.Sources))
		}
		if result.Sources[0] != "/in/transcript-a.jsonl" {
			t.Errorf("Source = %q, want /in/transcript-a.jsonl", result.Sources[0])
		}
	})

	t.Run("trace by filename", func(t *testing.T) {
		result, err := g.Trace("session-b.md")
		if err != nil {
			t.Fatalf("Trace() error = %v", err)
		}
		// Should match by filename
		if len(result.Chain) != 1 {
			t.Errorf("Expected 1 record in chain (filename match), got %d", len(result.Chain))
		}
	})

	t.Run("trace non-existent artifact", func(t *testing.T) {
		result, err := g.Trace("/nonexistent/path.md")
		if err != nil {
			t.Fatalf("Trace() error = %v", err)
		}
		if len(result.Chain) != 0 {
			t.Errorf("Expected 0 records for non-existent artifact, got %d", len(result.Chain))
		}
	})
}

func TestGraph_FindBySession(t *testing.T) {
	content := `{"id":"prov-1","artifact_path":"/out/1.md","artifact_type":"session","source_path":"/in/1.jsonl","source_type":"transcript","session_id":"sess-001"}
{"id":"prov-2","artifact_path":"/out/2.md","artifact_type":"index","source_path":"/in/1.jsonl","source_type":"transcript","session_id":"sess-001"}
{"id":"prov-3","artifact_path":"/out/3.md","artifact_type":"session","source_path":"/in/2.jsonl","source_type":"transcript","session_id":"sess-002"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	t.Run("existing session", func(t *testing.T) {
		results := g.FindBySession("sess-001")
		if len(results) != 2 {
			t.Errorf("Expected 2 records for sess-001, got %d", len(results))
		}
	})

	t.Run("non-existent session", func(t *testing.T) {
		results := g.FindBySession("nonexistent")
		if len(results) != 0 {
			t.Errorf("Expected 0 records for non-existent session, got %d", len(results))
		}
	})
}

func TestGraph_FindBySource(t *testing.T) {
	content := `{"id":"prov-1","artifact_path":"/out/1.md","artifact_type":"session","source_path":"/in/transcript-a.jsonl","source_type":"transcript"}
{"id":"prov-2","artifact_path":"/out/2.md","artifact_type":"session","source_path":"/in/transcript-a.jsonl","source_type":"transcript"}
{"id":"prov-3","artifact_path":"/out/3.md","artifact_type":"session","source_path":"/in/transcript-b.jsonl","source_type":"transcript"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	t.Run("existing source", func(t *testing.T) {
		results := g.FindBySource("/in/transcript-a.jsonl")
		if len(results) != 2 {
			t.Errorf("Expected 2 records from transcript-a, got %d", len(results))
		}
	})

	t.Run("non-existent source", func(t *testing.T) {
		results := g.FindBySource("/nonexistent.jsonl")
		if len(results) != 0 {
			t.Errorf("Expected 0 records for non-existent source, got %d", len(results))
		}
	})
}

func TestGraph_GetStats(t *testing.T) {
	content := `{"id":"prov-1","artifact_path":"/out/1.md","workspace_path":"/work/a","artifact_type":"session","source_path":"/in/1.jsonl","source_type":"transcript","session_id":"sess-001"}
{"id":"prov-2","artifact_path":"/out/2.jsonl","workspace_path":"/work/a","artifact_type":"index","source_path":"/in/1.jsonl","source_type":"transcript","session_id":"sess-001"}
{"id":"prov-3","artifact_path":"/out/3.md","workspace_path":"/work/b","artifact_type":"session","source_path":"/in/2.jsonl","source_type":"transcript","session_id":"sess-002"}
{"id":"prov-4","artifact_path":"/out/4.md","workspace_path":"/work/b","artifact_type":"learning","source_path":"/out/3.md","source_type":"session"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	stats := g.GetStats()

	if stats.TotalRecords != 4 {
		t.Errorf("TotalRecords = %d, want 4", stats.TotalRecords)
	}

	// Check artifact types
	if stats.ArtifactTypes["session"] != 2 {
		t.Errorf("ArtifactTypes[session] = %d, want 2", stats.ArtifactTypes["session"])
	}
	if stats.ArtifactTypes["index"] != 1 {
		t.Errorf("ArtifactTypes[index] = %d, want 1", stats.ArtifactTypes["index"])
	}
	if stats.ArtifactTypes["learning"] != 1 {
		t.Errorf("ArtifactTypes[learning] = %d, want 1", stats.ArtifactTypes["learning"])
	}

	// Check source types
	if stats.SourceTypes["transcript"] != 3 {
		t.Errorf("SourceTypes[transcript] = %d, want 3", stats.SourceTypes["transcript"])
	}
	if stats.SourceTypes["session"] != 1 {
		t.Errorf("SourceTypes[session] = %d, want 1", stats.SourceTypes["session"])
	}

	// Check unique sessions
	if stats.UniqueSessions != 2 {
		t.Errorf("UniqueSessions = %d, want 2", stats.UniqueSessions)
	}
	if stats.UniqueWorkspaces != 2 {
		t.Errorf("UniqueWorkspaces = %d, want 2", stats.UniqueWorkspaces)
	}
}

func TestGraph_GetStats_Empty(t *testing.T) {
	path := createTestProvFile(t, "")
	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	stats := g.GetStats()

	if stats.TotalRecords != 0 {
		t.Errorf("TotalRecords = %d, want 0", stats.TotalRecords)
	}
	if stats.UniqueSessions != 0 {
		t.Errorf("UniqueSessions = %d, want 0", stats.UniqueSessions)
	}
}

func TestRecord_Fields(t *testing.T) {
	record := Record{
		ID:           "prov-test",
		ArtifactPath: "/out/artifact.md",
		ArtifactType: "session",
		SourcePath:   "/in/source.jsonl",
		SourceType:   "transcript",
		SessionID:    "sess-123",
		CreatedAt:    time.Now(),
		Metadata: map[string]any{
			"key": "value",
		},
	}

	if record.ID != "prov-test" {
		t.Errorf("ID = %q, want prov-test", record.ID)
	}
	if record.Metadata["key"] != "value" {
		t.Error("Metadata not set correctly")
	}
}

func TestTraceResult_Fields(t *testing.T) {
	result := TraceResult{
		Artifact: "/path/to/artifact.md",
		Chain: []Record{
			{ID: "prov-1"},
			{ID: "prov-2"},
		},
		Sources: []string{"/in/source.jsonl"},
	}

	if result.Artifact != "/path/to/artifact.md" {
		t.Errorf("Artifact = %q", result.Artifact)
	}
	if len(result.Chain) != 2 {
		t.Errorf("Chain length = %d, want 2", len(result.Chain))
	}
	if len(result.Sources) != 1 {
		t.Errorf("Sources length = %d, want 1", len(result.Sources))
	}
}

func TestGraph_Trace_AbsolutePath(t *testing.T) {
	// Create a temp file so we can get a real absolute path
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "session.md")

	content := `{"id":"prov-1","artifact_path":"` + artifactPath + `","artifact_type":"session","source_path":"/in/transcript.jsonl","source_type":"transcript"}`
	provPath := filepath.Join(tmpDir, "graph.jsonl")
	if err := os.WriteFile(provPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	g, err := NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	// Trace using the absolute path
	result, err := g.Trace(artifactPath)
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(result.Chain) != 1 {
		t.Errorf("Expected 1 record in chain, got %d", len(result.Chain))
	}
}

func TestNewGraph_PermissionError(t *testing.T) {
	tmpDir := t.TempDir()
	provPath := filepath.Join(tmpDir, "graph.jsonl")
	if err := os.WriteFile(provPath, []byte(`{"id":"test"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make file unreadable
	if err := os.Chmod(provPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(provPath, 0644) })

	_, err := NewGraph(provPath)
	if err == nil {
		t.Error("expected error when provenance file is unreadable")
	}
}

func TestGraph_Trace_NonTranscriptSource(t *testing.T) {
	content := `{"id":"prov-1","artifact_path":"/out/learning.md","artifact_type":"learning","source_path":"/out/session.md","source_type":"session"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	result, err := g.Trace("/out/learning.md")
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(result.Chain) != 1 {
		t.Errorf("Expected 1 record in chain, got %d", len(result.Chain))
	}
	// Non-transcript source should NOT be in Sources
	if len(result.Sources) != 0 {
		t.Errorf("Expected 0 sources (non-transcript), got %d", len(result.Sources))
	}
}

func TestGraph_Trace_FilenameMatchNonTranscript(t *testing.T) {
	// When first pass (exact path match) finds nothing, fallback to filename match
	content := `{"id":"prov-1","artifact_path":"/different/path/unique-file.md","artifact_type":"learning","source_path":"/out/session.md","source_type":"session"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	// Use just the filename -- should trigger filename-match fallback
	result, err := g.Trace("unique-file.md")
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(result.Chain) != 1 {
		t.Errorf("Expected 1 record via filename match, got %d", len(result.Chain))
	}
	// Source type is "session" not "transcript"
	if len(result.Sources) != 0 {
		t.Errorf("Expected 0 transcript sources, got %d", len(result.Sources))
	}
}

func TestGraph_FindBySource_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "transcript.jsonl")

	content := `{"id":"prov-1","artifact_path":"/out/session.md","artifact_type":"session","source_path":"` + sourcePath + `","source_type":"transcript"}`
	provPath := filepath.Join(tmpDir, "graph.jsonl")
	if err := os.WriteFile(provPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	g, err := NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	// Find using the absolute path
	results := g.FindBySource(sourcePath)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestGraph_Trace_AbsError_FallsBackToRawPath(t *testing.T) {
	// When absPathFunc fails, Trace falls back to the raw artifact path
	// and still matches records via the literal path comparison in matchByAbsPath.
	content := `{"id":"prov-abs","artifact_path":"relative/session.md","artifact_type":"session","source_path":"/in/transcript.jsonl","source_type":"transcript"}`
	path := createTestProvFile(t, content)

	g, err := NewGraph(path)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	orig := absPathFunc
	absPathFunc = func(string) (string, error) {
		return "", fmt.Errorf("simulated abs failure")
	}
	t.Cleanup(func() { absPathFunc = orig })

	result, err := g.Trace("relative/session.md")
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(result.Chain) != 1 {
		t.Errorf("Expected 1 record in chain (fallback match), got %d", len(result.Chain))
	}
	if result.Chain[0].ID != "prov-abs" {
		t.Errorf("Chain[0].ID = %q, want %q", result.Chain[0].ID, "prov-abs")
	}
	if len(result.Sources) != 1 || result.Sources[0] != "/in/transcript.jsonl" {
		t.Errorf("Sources = %v, want [/in/transcript.jsonl]", result.Sources)
	}
}

// --- Benchmarks ---

func benchProvFile(b *testing.B, numRecords int) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "graph.jsonl")
	var content string
	for range numRecords {
		content += `{"artifact_path":".agents/learnings/` + time.Now().Format("2006-01-02") + `-bench.md","source_session":"sess-` + time.Now().Format("150405") + `","source_path":"cli/internal/provenance/bench_test.go","timestamp":"2025-01-15T12:00:00Z"}` + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("write: %v", err)
	}
	return path
}

func BenchmarkNewGraph(b *testing.B) {
	path := benchProvFile(b, 50)
	b.ResetTimer()
	for range b.N {
		_, _ = NewGraph(path)
	}
}

func BenchmarkGraphGetStats(b *testing.B) {
	path := benchProvFile(b, 50)
	g, err := NewGraph(path)
	if err != nil {
		b.Fatalf("NewGraph: %v", err)
	}
	b.ResetTimer()
	for range b.N {
		g.GetStats()
	}
}
