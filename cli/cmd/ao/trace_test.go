// practices: [distributed-tracing, adr]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenance"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// ---------------------------------------------------------------------------
// repeatString
// ---------------------------------------------------------------------------

func TestTrace_repeatString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"repeat 3", "=", 3, "==="},
		{"repeat 0", "x", 0, ""},
		{"repeat 1", "ab", 1, "ab"},
		{"repeat 5 dash", "-", 5, "-----"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repeatString(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("repeatString(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// min
// ---------------------------------------------------------------------------

func TestTrace_min(t *testing.T) {
	tests := []struct {
		name string
		a, b int
		want int
	}{
		{"a < b", 3, 5, 3},
		{"a > b", 7, 2, 2},
		{"a == b", 4, 4, 4},
		{"negative", -1, 3, -1},
		{"both negative", -5, -2, -5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := min(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// printTraceTable (smoke test, output-only)
// ---------------------------------------------------------------------------

func TestTrace_printTraceTable_NoChain(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "/path/to/artifact.md",
	}
	out, _ := captureStdout(t, func() error { printTraceTable(result); return nil })
	if !strings.Contains(out, "artifact.md") {
		t.Errorf("expected artifact path in output, got: %s", out)
	}
}

func TestTrace_printTraceTable_WithChain(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "/path/to/session.md",
		Chain: []provenance.Record{
			{
				ID:           "prov-abc1234",
				ArtifactType: "session",
				SourcePath:   "/path/to/transcript.jsonl",
				SourceType:   "transcript",
				SessionID:    "sess-12345",
				CreatedAt:    time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC),
			},
		},
		Sources: []string{"/path/to/transcript.jsonl"},
	}
	out, _ := captureStdout(t, func() error { printTraceTable(result); return nil })
	for _, want := range []string{"session.md", "prov-abc1234", "transcript"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// printTraceGraph (smoke test, output-only)
// ---------------------------------------------------------------------------

func TestTrace_printTraceGraph_SingleRecord(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "/path/to/session.md",
		Chain: []provenance.Record{
			{
				ID:           "prov-abc",
				ArtifactType: "session",
				SourcePath:   "/path/to/transcript.jsonl",
				SourceType:   "transcript",
				SessionID:    "sess-12345",
				CreatedAt:    time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	out, _ := captureStdout(t, func() error { printTraceGraph(result); return nil })
	for _, want := range []string{"session.md", "prov-abc", "sess-12345"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTrace_printTraceGraph_ShortSessionID(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "/path/to/session.md",
		Chain: []provenance.Record{
			{
				ID:           "prov-x",
				ArtifactType: "session",
				SourcePath:   "/src.jsonl",
				SourceType:   "transcript",
				SessionID:    "ab",
				CreatedAt:    time.Now(),
			},
		},
	}
	out, _ := captureStdout(t, func() error { printTraceGraph(result); return nil })
	if !strings.Contains(out, "prov-x") {
		t.Errorf("output missing prov-x:\n%s", out)
	}
}

func TestTrace_printTraceGraph_EmptySessionID(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "/path/to/session.md",
		Chain: []provenance.Record{
			{
				ID:           "prov-y",
				ArtifactType: "session",
				SourcePath:   "/src.jsonl",
				SourceType:   "transcript",
				SessionID:    "",
				CreatedAt:    time.Now(),
			},
		},
	}
	out, _ := captureStdout(t, func() error { printTraceGraph(result); return nil })
	if !strings.Contains(out, "prov-y") {
		t.Errorf("output missing prov-y:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// traceOneArtifact with real provenance file
// ---------------------------------------------------------------------------

func TestTrace_traceOneArtifact_Found(t *testing.T) {
	tmp := t.TempDir()
	provDir := filepath.Join(tmp, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a provenance record
	record := provenance.Record{
		ID:           "prov-test123",
		ArtifactPath: "sessions/session.md",
		ArtifactType: "session",
		SourcePath:   "/path/to/transcript.jsonl",
		SourceType:   "transcript",
		SessionID:    "sess-xyz",
		CreatedAt:    time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC),
	}
	data, _ := json.Marshal(record)
	provPath := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := provenance.NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	// Trace the artifact
	err = traceOneArtifact(graph, "sessions/session.md")
	if err != nil {
		t.Fatalf("traceOneArtifact: %v", err)
	}
}

func TestTrace_traceOneArtifact_NotFound(t *testing.T) {
	tmp := t.TempDir()
	provDir := filepath.Join(tmp, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Empty provenance file
	provPath := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := provenance.NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	// Tracing a nonexistent artifact should not error (just print "No provenance found")
	err = traceOneArtifact(graph, "nonexistent.md")
	if err != nil {
		t.Fatalf("traceOneArtifact for nonexistent: %v", err)
	}
}

func TestTrace_traceOneArtifact_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	provDir := filepath.Join(tmp, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatal(err)
	}

	record := provenance.Record{
		ID:           "prov-json",
		ArtifactPath: "sessions/json-test.md",
		ArtifactType: "session",
		SourcePath:   "/transcript.jsonl",
		SourceType:   "transcript",
		CreatedAt:    time.Now(),
	}
	data, _ := json.Marshal(record)
	provPath := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := provenance.NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	// Should output JSON without error
	err = traceOneArtifact(graph, "sessions/json-test.md")
	if err != nil {
		t.Fatalf("traceOneArtifact JSON: %v", err)
	}
}

func TestTrace_traceOneArtifact_GraphMode(t *testing.T) {
	tmp := t.TempDir()
	provDir := filepath.Join(tmp, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatal(err)
	}

	record := provenance.Record{
		ID:           "prov-graph",
		ArtifactPath: "sessions/graph-test.md",
		ArtifactType: "session",
		SourcePath:   "/transcript.jsonl",
		SourceType:   "transcript",
		SessionID:    "sess-graph-123",
		CreatedAt:    time.Now(),
	}
	data, _ := json.Marshal(record)
	provPath := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := provenance.NewGraph(provPath)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldTraceGraph := traceGraph
	traceGraph = true
	defer func() { traceGraph = oldTraceGraph }()

	// Should output graph mode without error
	err = traceOneArtifact(graph, "sessions/graph-test.md")
	if err != nil {
		t.Fatalf("traceOneArtifact graph: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runTrace dry-run
// ---------------------------------------------------------------------------

func TestTrace_runTrace_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	// Should print dry-run message and return nil
	err := runTrace(nil, []string{"path1.md", "path2.md"})
	if err != nil {
		t.Fatalf("runTrace dry-run: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runTrace — no provenance records
// ---------------------------------------------------------------------------

func TestTrace_runTrace_NoProvenanceFile(t *testing.T) {
	tmp := t.TempDir()

	testProjectDir = tmp
	defer func() { testProjectDir = "" }()

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	// Create the directory structure but no provenance file
	provDir := filepath.Join(tmp, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Should handle gracefully (empty records)
	err := runTrace(nil, []string{"test.md"})
	if err != nil {
		// If provenance file is missing, NewGraph should return empty records
		if !strings.Contains(err.Error(), "provenance") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// trace.go — traceOneArtifact
// ---------------------------------------------------------------------------

func TestTrace_traceOneArtifact_jsonOutput(t *testing.T) {
	// Set up a temp dir with a provenance graph file
	tmpDir := t.TempDir()
	provDir := filepath.Join(tmpDir, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	record := provenance.Record{
		ID:           "prov-abc1234",
		ArtifactPath: "sessions/test-session.md",
		ArtifactType: "session",
		SourcePath:   "/tmp/transcript.jsonl",
		SourceType:   "transcript",
		SessionID:    "session-abc123",
		CreatedAt:    time.Now(),
	}
	data, _ := json.Marshal(record)

	provFile := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provFile, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write prov: %v", err)
	}

	graph, err := provenance.NewGraph(provFile)
	if err != nil {
		t.Fatalf("new graph: %v", err)
	}

	// Test JSON output mode
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = traceOneArtifact(graph, "sessions/test-session.md")

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("traceOneArtifact: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "session-abc123") {
		t.Errorf("expected JSON output to contain session ID, got: %s", out)
	}
}

func TestTrace_traceOneArtifact_noProvenance(t *testing.T) {
	// Create an empty provenance graph
	tmpDir := t.TempDir()
	provDir := filepath.Join(tmpDir, storage.DefaultBaseDir, storage.ProvenanceDir)
	if err := os.MkdirAll(provDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	provFile := filepath.Join(provDir, storage.ProvenanceFile)
	if err := os.WriteFile(provFile, []byte{}, 0644); err != nil {
		t.Fatalf("write prov: %v", err)
	}

	graph, err := provenance.NewGraph(provFile)
	if err != nil {
		t.Fatalf("new graph: %v", err)
	}

	// Test table output with no chain
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = traceOneArtifact(graph, "nonexistent-artifact.md")

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("traceOneArtifact: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "No provenance found") {
		t.Errorf("expected 'No provenance found', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// trace.go — printTraceGraph
// ---------------------------------------------------------------------------

func TestTrace_printTraceGraph(t *testing.T) {
	result := &provenance.TraceResult{
		Artifact: "sessions/test.md",
		Chain: []provenance.Record{
			{
				ID:           "prov-001",
				ArtifactPath: "sessions/test.md",
				ArtifactType: "session",
				SourcePath:   "/tmp/transcript.jsonl",
				SourceType:   "transcript",
				SessionID:    "sess-1234567890ab",
				CreatedAt:    time.Now(),
			},
		},
		Sources: []string{"/tmp/transcript.jsonl"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTraceGraph(result)

	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	// Should contain graph symbols and session ID (truncated to 12)
	if !strings.Contains(out, "Provenance Graph for:") {
		t.Errorf("expected graph header, got: %s", out)
	}
	if !strings.Contains(out, "sess-1234567") {
		t.Errorf("expected truncated session ID, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// trace.go — repeatString, min (additional cases)
// ---------------------------------------------------------------------------

func TestTrace_repeatString_Additional(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"=", 0, ""},
		{"=", 3, "==="},
		{"ab", 2, "abab"},
		{"", 5, ""},
	}
	for _, tc := range tests {
		got := repeatString(tc.s, tc.n)
		if got != tc.want {
			t.Errorf("repeatString(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestTrace_min_Additional(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{3, 5, 3},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
	}
	for _, tc := range tests {
		got := min(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("min(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
