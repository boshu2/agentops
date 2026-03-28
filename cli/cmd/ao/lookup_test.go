package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesID_ExactMatch(t *testing.T) {
	if !matchesID("learn-001", "/path/to/learn-001.md", "learn-001") {
		t.Error("expected exact ID match")
	}
}

func TestMatchesID_CaseInsensitive(t *testing.T) {
	if !matchesID("Learn-001", "/path/to/file.md", "learn-001") {
		t.Error("expected case-insensitive match")
	}
}

func TestMatchesID_FilenameMatch(t *testing.T) {
	if !matchesID("some-other-id", "/path/to/2026-02-22-cross-language.md", "cross-language") {
		t.Error("expected filename substring match")
	}
}

func TestMatchesID_NoMatch(t *testing.T) {
	if matchesID("learn-001", "/path/to/learn-001.md", "learn-999") {
		t.Error("expected no match")
	}
}

func TestFilterByBead(t *testing.T) {
	learnings := []learning{
		{ID: "l1", SourceBead: "ag-mrr"},
		{ID: "l2", SourceBead: "ag-xyz"},
		{ID: "l3", SourceBead: "ag-mrr"},
		{ID: "l4", SourceBead: ""},
	}
	filtered := filterByBead(learnings, "ag-mrr")
	if len(filtered) != 2 {
		t.Errorf("expected 2 matches, got %d", len(filtered))
	}
	for _, l := range filtered {
		if l.SourceBead != "ag-mrr" {
			t.Errorf("unexpected bead: %s", l.SourceBead)
		}
	}
}

func TestFilterByBead_CaseInsensitive(t *testing.T) {
	learnings := []learning{
		{ID: "l1", SourceBead: "AG-MRR"},
	}
	filtered := filterByBead(learnings, "ag-mrr")
	if len(filtered) != 1 {
		t.Errorf("expected 1 match, got %d", len(filtered))
	}
}

func TestFilterByBead_Empty(t *testing.T) {
	filtered := filterByBead(nil, "ag-mrr")
	if len(filtered) != 0 {
		t.Errorf("expected 0 matches, got %d", len(filtered))
	}
}

func TestRelPath(t *testing.T) {
	cwd := "/Users/test/project"
	path := "/Users/test/project/.agents/learnings/test.md"
	got := relPath(cwd, path)
	if got != ".agents/learnings/test.md" {
		t.Errorf("relPath = %q, want .agents/learnings/test.md", got)
	}
}

func TestLookupByID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Create empty learnings dir
	os.MkdirAll(filepath.Join(tmpDir, ".agents", "learnings"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".agents", "patterns"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	err := lookupByID(tmpDir, "nonexistent-id", nil)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "no artifact found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatLookupAge(t *testing.T) {
	tests := []struct {
		weeks float64
		want  string
	}{
		{0.05, "<1d"},
		{0.3, "2d"},
		{1.0, "1w"},
		{1.5, "2w"},
		{4.0, "4w"},
		{8.0, "2mo"},
	}
	for _, tt := range tests {
		got := formatLookupAge(tt.weeks)
		if got != tt.want {
			t.Errorf("formatLookupAge(%v) = %q, want %q", tt.weeks, got, tt.want)
		}
	}
}

func TestEmptyIfMissing(t *testing.T) {
	if got := emptyIfMissing(""); got != "-" {
		t.Errorf("emptyIfMissing(\"\") = %q, want \"-\"", got)
	}
	if got := emptyIfMissing("hello"); got != "hello" {
		t.Errorf("emptyIfMissing(\"hello\") = %q, want \"hello\"", got)
	}
}

// ---------------------------------------------------------------------------
// lookup.go — matchesID
// ---------------------------------------------------------------------------

func TestLookup_matchesID(t *testing.T) {
	tests := []struct {
		name     string
		itemID   string
		filePath string
		searchID string
		want     bool
	}{
		{
			name:     "exact ID match",
			itemID:   "learn-2026-01-20-cross-lang",
			searchID: "learn-2026-01-20-cross-lang",
			want:     true,
		},
		{
			name:     "case insensitive ID match",
			itemID:   "LEARN-ABC",
			searchID: "learn-abc",
			want:     true,
		},
		{
			name:     "filename match without extension",
			itemID:   "some-id",
			filePath: "/tmp/learn-2026-01-20-cross-lang.md",
			searchID: "learn-2026-01-20-cross-lang",
			want:     true,
		},
		{
			name:     "partial filename match",
			itemID:   "some-id",
			filePath: "/tmp/learn-2026-01-20-cross-lang.md",
			searchID: "cross-lang",
			want:     true,
		},
		{
			name:     "no match at all",
			itemID:   "id-alpha",
			filePath: "/tmp/beta.md",
			searchID: "gamma",
			want:     false,
		},
		{
			name:     "empty file path only checks ID",
			itemID:   "target-id",
			filePath: "",
			searchID: "target-id",
			want:     true,
		},
		{
			name:     "empty file path no match",
			itemID:   "alpha",
			filePath: "",
			searchID: "beta",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesID(tc.itemID, tc.filePath, tc.searchID)
			if got != tc.want {
				t.Errorf("matchesID(%q, %q, %q) = %v, want %v",
					tc.itemID, tc.filePath, tc.searchID, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lookup.go — filterByBead
// ---------------------------------------------------------------------------

func TestLookup_filterByBead(t *testing.T) {
	learnings := []learning{
		{ID: "l1", SourceBead: "ag-abc"},
		{ID: "l2", SourceBead: "ag-def"},
		{ID: "l3", SourceBead: "ag-abc"},
		{ID: "l4", SourceBead: ""},
	}

	t.Run("filters matching bead", func(t *testing.T) {
		filtered := filterByBead(learnings, "ag-abc")
		if len(filtered) != 2 {
			t.Errorf("expected 2 matches, got %d", len(filtered))
		}
		for _, l := range filtered {
			if l.SourceBead != "ag-abc" {
				t.Errorf("unexpected bead %q in results", l.SourceBead)
			}
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		filtered := filterByBead(learnings, "AG-ABC")
		if len(filtered) != 2 {
			t.Errorf("expected 2 matches (case insensitive), got %d", len(filtered))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		filtered := filterByBead(learnings, "ag-xyz")
		if len(filtered) != 0 {
			t.Errorf("expected 0 matches, got %d", len(filtered))
		}
	})
}

// ---------------------------------------------------------------------------
// lookup.go — formatLookupAge
// ---------------------------------------------------------------------------

func TestLookup_formatLookupAge(t *testing.T) {
	tests := []struct {
		ageWeeks float64
		want     string
	}{
		{0.0, "<1d"},
		{0.13, "<1d"},
		{0.15, "1d"},
		{0.5, "4d"},
		{1.0, "1w"},
		{3.5, "4w"},
		{5.0, "1mo"},
		{8.57, "2mo"},
	}

	for _, tc := range tests {
		got := formatLookupAge(tc.ageWeeks)
		if got != tc.want {
			t.Errorf("formatLookupAge(%.2f) = %q, want %q", tc.ageWeeks, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// lookup.go — relPath
// ---------------------------------------------------------------------------

func TestLookup_relPath(t *testing.T) {
	cwd := "/Users/test/project"

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "subpath made relative",
			path: "/Users/test/project/src/file.go",
			want: "src/file.go",
		},
		{
			name: "unrelated path returns original",
			path: "/var/log/syslog",
			want: "../../../var/log/syslog",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := relPath(cwd, tc.path)
			if got != tc.want {
				t.Errorf("relPath(%q, %q) = %q, want %q", cwd, tc.path, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lookup.go — outputResults
// ---------------------------------------------------------------------------

func TestLookup_outputResults_noResults(t *testing.T) {
	// Save and restore the module-level flag
	oldLookupJSON := lookupJSON
	lookupJSON = false
	defer func() { lookupJSON = oldLookupJSON }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputResults("/tmp", nil, nil, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputResults: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "No matching artifacts found") {
		t.Errorf("expected 'No matching artifacts found', got: %s", out)
	}
}

func TestLookup_outputResults_withLearnings(t *testing.T) {
	oldLookupJSON := lookupJSON
	lookupJSON = false
	defer func() { lookupJSON = oldLookupJSON }()

	learnings := []learning{
		{
			ID:             "l-test-1",
			Title:          "Test Learning",
			Summary:        "A summary",
			Source:         "/tmp/src.md",
			Utility:        0.75,
			AgeWeeks:       2.0,
			CompositeScore: 0.80,
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputResults("/tmp", learnings, nil, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputResults: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "l-test-1") {
		t.Errorf("expected learning ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Test Learning") {
		t.Errorf("expected learning title in output, got: %s", out)
	}
}

func TestLookup_outputResults_jsonMode(t *testing.T) {
	oldLookupJSON := lookupJSON
	lookupJSON = true
	defer func() { lookupJSON = oldLookupJSON }()

	learnings := []learning{
		{ID: "l-json-1", Title: "JSON Test"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputResults("/tmp", learnings, nil, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputResults: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, `"learnings"`) {
		t.Errorf("expected JSON with 'learnings' key, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// lookup.go — outputLearning (text mode, no-cite)
// ---------------------------------------------------------------------------

func TestLookup_outputLearning_textMode(t *testing.T) {
	oldLookupJSON := lookupJSON
	lookupJSON = false
	defer func() { lookupJSON = oldLookupJSON }()

	oldLookupNoCite := lookupNoCite
	lookupNoCite = true
	defer func() { lookupNoCite = oldLookupNoCite }()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test-learning.md")
	if err := os.WriteFile(srcFile, []byte("# Full Content\nDetails here."), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := learning{
		ID:             "learn-output-test",
		Title:          "Output Test Learning",
		Summary:        "A test summary",
		Source:         srcFile,
		SourceBead:     "ag-test",
		SourcePhase:    "implement",
		Utility:        0.80,
		AgeWeeks:       1.0,
		CompositeScore: 0.85,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputLearning(tmpDir, l)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputLearning: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	checks := []string{
		"learn-output-test",
		"Output Test Learning",
		"A test summary",
		"Full Content",
		"ag-test",
		"implement",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, out)
		}
	}
}

func TestOutputPattern_TextMode(t *testing.T) {
	dir := t.TempDir()

	// Create a source file for the pattern
	agentsDir := filepath.Join(dir, ".agents", "patterns")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(agentsDir, "test-pattern.md")
	if err := os.WriteFile(srcPath, []byte("# Pattern content\nDetails here."), 0o644); err != nil {
		t.Fatal(err)
	}

	oldJSON := lookupJSON
	lookupJSON = false
	oldNoCite := lookupNoCite
	lookupNoCite = true // skip citation recording
	defer func() {
		lookupJSON = oldJSON
		lookupNoCite = oldNoCite
	}()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := pattern{
		Name:           "test-pattern",
		Description:    "A test pattern",
		Utility:        0.8,
		AgeWeeks:       2,
		CompositeScore: 3.5,
		FilePath:       srcPath,
	}

	err := outputPattern(dir, p)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPattern: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "## test-pattern") {
		t.Errorf("expected pattern header, got:\n%s", out)
	}
	if !strings.Contains(out, "A test pattern") {
		t.Errorf("expected description, got:\n%s", out)
	}
	if !strings.Contains(out, "Utility: 0.80") {
		t.Errorf("expected utility score, got:\n%s", out)
	}
	if !strings.Contains(out, "Pattern content") {
		t.Errorf("expected file content, got:\n%s", out)
	}
}

func TestOutputPattern_JSONMode(t *testing.T) {
	oldJSON := lookupJSON
	lookupJSON = true
	defer func() { lookupJSON = oldJSON }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := pattern{
		Name:           "json-pat",
		Utility:        0.5,
		CompositeScore: 2.0,
	}

	err := outputPattern("", p)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPattern JSON: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()

	var parsed pattern
	if err := json.Unmarshal(buf[:n], &parsed); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, string(buf[:n]))
	}
	if parsed.Name != "json-pat" {
		t.Errorf("Name = %q, want %q", parsed.Name, "json-pat")
	}
}

func TestOutputFinding_TextMode(t *testing.T) {
	dir := t.TempDir()

	oldJSON := lookupJSON
	lookupJSON = false
	oldNoCite := lookupNoCite
	lookupNoCite = true
	defer func() {
		lookupJSON = oldJSON
		lookupNoCite = oldNoCite
	}()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := knowledgeFinding{
		ID:             "f-test-01",
		Title:          "Test Finding",
		Severity:       "high",
		Detectability:  "mechanical",
		Status:         "active",
		Utility:        0.9,
		AgeWeeks:       1,
		CompositeScore: 4.0,
		Summary:        "A critical finding.",
	}

	err := outputFinding(dir, f)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputFinding: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "## f-test-01") {
		t.Errorf("expected finding header, got:\n%s", out)
	}
	if !strings.Contains(out, "**Test Finding**") {
		t.Errorf("expected title, got:\n%s", out)
	}
	if !strings.Contains(out, "Severity: high") {
		t.Errorf("expected severity, got:\n%s", out)
	}
	if !strings.Contains(out, "A critical finding.") {
		t.Errorf("expected summary, got:\n%s", out)
	}
}

func TestOutputFinding_JSONMode(t *testing.T) {
	oldJSON := lookupJSON
	lookupJSON = true
	defer func() { lookupJSON = oldJSON }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := knowledgeFinding{
		ID:       "f-json-01",
		Title:    "JSON Finding",
		Severity: "medium",
		Status:   "active",
	}

	err := outputFinding("", f)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputFinding JSON: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()

	var parsed knowledgeFinding
	if err := json.Unmarshal(buf[:n], &parsed); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, string(buf[:n]))
	}
	if parsed.ID != "f-json-01" {
		t.Errorf("ID = %q, want %q", parsed.ID, "f-json-01")
	}
}

// TestLookup_TopicalDiscoverability proves known flywheel-related learnings are
// surfaced by topical queries. This is acceptance criterion #2 for ag-73u.5.
func TestLookup_TopicalDiscoverability(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Seed learnings with flywheel-related content
	learningFiles := map[string]string{
		"2026-03-20-escape-velocity.md": `---
type: learning
maturity: provisional
confidence: high
utility: 0.8
---
# Escape velocity threshold for knowledge compounding

Escape velocity (σρ > δ) is necessary but not sufficient for true compounding.
Golden signals must also be healthy.`,

		"2026-03-20-research-closure.md": `---
type: learning
maturity: provisional
confidence: high
utility: 0.8
---
# Research closure reduces orphaned research

Learnings that carry .agents/research/ provenance improve closure metrics
and reduce the orphaned research percentage in flywheel health.`,

		"2026-03-20-unrelated.md": `---
type: learning
maturity: provisional
confidence: high
utility: 0.8
---
# Database connection pooling best practices

Use connection pooling to avoid exhausting database connections under load.`,
	}

	for name, content := range learningFiles {
		if err := os.WriteFile(filepath.Join(learningsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		query       string
		expectFound string // substring of title that should appear
		expectMiss  string // substring of title that should NOT appear
	}{
		{
			query:       "flywheel",
			expectFound: "Research closure",
			expectMiss:  "Database",
		},
		{
			query:       "escape velocity",
			expectFound: "Escape velocity",
			expectMiss:  "Database",
		},
		{
			query:       "compounding",
			expectFound: "Escape velocity",
			expectMiss:  "Database",
		},
		{
			query:       "orphaned research",
			expectFound: "Research closure",
			expectMiss:  "Database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := collectLearnings(dir, tt.query, 10, "", 0)
			if err != nil {
				t.Fatalf("collectLearnings(%q): %v", tt.query, err)
			}

			found := false
			foundMiss := false
			for _, l := range results {
				if strings.Contains(l.Title, tt.expectFound) {
					found = true
				}
				if strings.Contains(l.Title, tt.expectMiss) {
					foundMiss = true
				}
			}

			if !found {
				titles := make([]string, len(results))
				for i, l := range results {
					titles[i] = l.Title
				}
				t.Errorf("query %q should surface learning containing %q, got: %v",
					tt.query, tt.expectFound, titles)
			}
			if foundMiss {
				t.Errorf("query %q should NOT surface learning containing %q",
					tt.query, tt.expectMiss)
			}
		})
	}
}

func TestRecordLookupCitations(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learnings := []learning{
		{Source: filepath.Join(dir, ".agents", "learnings", "l1.md")},
	}
	patterns := []pattern{
		{FilePath: filepath.Join(dir, ".agents", "patterns", "p1.md")},
	}
	findings := []knowledgeFinding{
		{Source: filepath.Join(dir, ".agents", "findings", "f1.md")},
	}

	recordLookupCitations(dir, learnings, patterns, findings, "test-session", "test query", "retrieved")

	citPath := filepath.Join(aoDir, "citations.jsonl")
	data, err := os.ReadFile(citPath)
	if err != nil {
		t.Fatalf("read citations: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 citation lines, got %d", len(lines))
	}
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("line %d: parse citation: %v", i, err)
		}
		if got := event["citation_type"]; got != "retrieved" {
			t.Fatalf("line %d: citation_type = %v, want retrieved", i, got)
		}
	}
}
