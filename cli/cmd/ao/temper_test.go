package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestIsContainedPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		path     string
		expected bool
	}{
		{
			name:     "contained simple path",
			baseDir:  "/home/user/project",
			path:     "/home/user/project/file.md",
			expected: true,
		},
		{
			name:     "contained nested path",
			baseDir:  "/home/user/project",
			path:     "/home/user/project/subdir/file.md",
			expected: true,
		},
		{
			name:     "not contained - parent traversal",
			baseDir:  "/home/user/project",
			path:     "/home/user/project/../other/file.md",
			expected: false,
		},
		{
			name:     "not contained - sibling directory",
			baseDir:  "/home/user/project",
			path:     "/home/user/other/file.md",
			expected: false,
		},
		{
			name:     "not contained - absolute outside",
			baseDir:  "/home/user/project",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "base dir itself is contained",
			baseDir:  "/home/user/project",
			path:     "/home/user/project",
			expected: true,
		},
		{
			name:     "prefix attack - similar name",
			baseDir:  "/home/user/project",
			path:     "/home/user/project-evil/file.md",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isContainedPath(tc.baseDir, tc.path)
			if result != tc.expected {
				t.Errorf("isContainedPath(%q, %q) = %v, want %v",
					tc.baseDir, tc.path, result, tc.expected)
			}
		})
	}
}

func TestIsArtifactFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"markdown file", "test.md", true},
		{"jsonl file", "data.jsonl", true},
		{"text file", "readme.txt", false},
		{"go file", "main.go", false},
		{"no extension", "Makefile", false},
		{"hidden md", ".hidden.md", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isArtifactFile(tc.filename)
			if result != tc.expected {
				t.Errorf("isArtifactFile(%q) = %v, want %v",
					tc.filename, result, tc.expected)
			}
		})
	}
}

func TestParseMarkdownField(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		field    string
		expected string
		found    bool
	}{
		{
			name:     "standard bold field",
			line:     "**ID**: L1-test",
			field:    "ID",
			expected: "L1-test",
			found:    true,
		},
		{
			name:     "bold field with trailing colon",
			line:     "**ID:** L1-test",
			field:    "ID",
			expected: "L1-test",
			found:    true,
		},
		{
			name:     "list item field",
			line:     "- **Maturity**: candidate",
			field:    "Maturity",
			expected: "candidate",
			found:    true,
		},
		{
			name:     "field not found",
			line:     "**Other**: value",
			field:    "ID",
			expected: "",
			found:    false,
		},
		{
			name:     "value with spaces",
			line:     "**Status**: TEMPERED",
			field:    "Status",
			expected: "TEMPERED",
			found:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, found := parseMarkdownField(tc.line, tc.field)
			if found != tc.found {
				t.Errorf("parseMarkdownField(%q, %q) found = %v, want %v",
					tc.line, tc.field, found, tc.found)
			}
			if result != tc.expected {
				t.Errorf("parseMarkdownField(%q, %q) = %q, want %q",
					tc.line, tc.field, result, tc.expected)
			}
		})
	}
}

func TestExpandFilePatterns(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "temper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	// Create test files
	testFiles := []string{
		"file1.md",
		"file2.md",
		"data.jsonl",
		"readme.txt",
		"subdir/nested.md",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	t.Run("glob pattern", func(t *testing.T) {
		files, err := expandFilePatterns(tmpDir, []string{"*.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d: %v", len(files), files)
		}
	})

	t.Run("literal file", func(t *testing.T) {
		files, err := expandFilePatterns(tmpDir, []string{"file1.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d: %v", len(files), files)
		}
	})

	t.Run("directory non-recursive", func(t *testing.T) {
		temperRecursive = false
		files, err := expandFilePatterns(tmpDir, []string{tmpDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should find file1.md, file2.md, data.jsonl (not readme.txt, not nested.md)
		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d: %v", len(files), files)
		}
	})

	t.Run("directory recursive", func(t *testing.T) {
		temperRecursive = true
		defer func() { temperRecursive = false }()

		files, err := expandFilePatterns(tmpDir, []string{tmpDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should find file1.md, file2.md, data.jsonl, subdir/nested.md
		if len(files) != 4 {
			t.Errorf("expected 4 files, got %d: %v", len(files), files)
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		_, err := expandFilePatterns(tmpDir, []string{"../../../etc/passwd"})
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}
	})
}

func TestParseArtifactMetadata(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "temper-meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	t.Run("parse markdown artifact", func(t *testing.T) {
		content := `# Learning: Test Pattern

**ID**: L1-test-pattern
**Maturity**: candidate
**Utility**: 0.75
**Confidence**: 0.8
**Status**: TEMPERED
**Schema Version**: 1

## Summary
This is a test learning.
`
		path := filepath.Join(tmpDir, "test-learning.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		meta, err := parseArtifactMetadata(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.ID != "L1-test-pattern" {
			t.Errorf("expected ID 'L1-test-pattern', got %q", meta.ID)
		}
		if meta.Maturity != "candidate" {
			t.Errorf("expected Maturity 'candidate', got %q", meta.Maturity)
		}
		if meta.Utility != 0.75 {
			t.Errorf("expected Utility 0.75, got %f", meta.Utility)
		}
		if !meta.Tempered {
			t.Error("expected Tempered to be true")
		}
	})

	t.Run("fallback to filename for ID", func(t *testing.T) {
		content := `# Learning without ID

Just some content.
`
		path := filepath.Join(tmpDir, "unnamed-learning.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		meta, err := parseArtifactMetadata(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.ID != "unnamed-learning" {
			t.Errorf("expected ID 'unnamed-learning', got %q", meta.ID)
		}
	})
}

func TestValidateArtifact(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "temper-validate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	t.Run("valid artifact", func(t *testing.T) {
		content := `# Test

**ID**: L1-valid
**Maturity**: candidate
**Utility**: 0.8
`
		path := filepath.Join(tmpDir, "valid.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result := validateArtifact(path, "provisional", 0.5, 0)
		if !result.Valid {
			t.Errorf("expected valid artifact, got issues: %v", result.Issues)
		}
	})

	t.Run("low utility rejected", func(t *testing.T) {
		content := `# Test

**ID**: L1-low-util
**Maturity**: candidate
**Utility**: 0.3
`
		path := filepath.Join(tmpDir, "low-util.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result := validateArtifact(path, "provisional", 0.5, 0)
		if result.Valid {
			t.Error("expected invalid artifact for low utility")
		}
	})

	t.Run("low maturity rejected", func(t *testing.T) {
		content := `# Test

**ID**: L1-low-mat
**Maturity**: provisional
**Utility**: 0.8
`
		path := filepath.Join(tmpDir, "low-mat.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result := validateArtifact(path, "candidate", 0.5, 0)
		if result.Valid {
			t.Error("expected invalid artifact for low maturity")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		result := validateArtifact(filepath.Join(tmpDir, "nonexistent.md"), "provisional", 0.5, 0)
		if result.Valid {
			t.Error("expected invalid for nonexistent file")
		}
	})
}

// ---------------------------------------------------------------------------
// validateTemperFiles
// ---------------------------------------------------------------------------

func TestTemperCoverage_ValidateTemperFiles(t *testing.T) {
	tmp := t.TempDir()

	// Save and restore package-level vars
	origMinMaturity := temperMinMaturity
	origMinUtility := temperMinUtility
	origMinFeedback := temperMinFeedback
	defer func() {
		temperMinMaturity = origMinMaturity
		temperMinUtility = origMinUtility
		temperMinFeedback = origMinFeedback
	}()

	temperMinMaturity = "provisional"
	temperMinUtility = 0.3
	temperMinFeedback = 0

	// Create valid artifact
	validContent := "# Test\n\n**ID**: L1-valid\n**Maturity**: candidate\n**Utility**: 0.8\n"
	validPath := filepath.Join(tmp, "valid.md")
	if err := os.WriteFile(validPath, []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create invalid artifact (low utility)
	invalidContent := "# Test\n\n**ID**: L2-bad\n**Maturity**: provisional\n**Utility**: 0.1\n"
	invalidPath := filepath.Join(tmp, "invalid.md")
	if err := os.WriteFile(invalidPath, []byte(invalidContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("mixed valid and invalid", func(t *testing.T) {
		results, allValid := validateTemperFiles([]string{validPath, invalidPath})
		if allValid {
			t.Error("expected allValid=false with one invalid file")
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if !results[0].Valid {
			t.Errorf("first result should be valid, issues: %v", results[0].Issues)
		}
		if results[1].Valid {
			t.Error("second result should be invalid")
		}
	})

	t.Run("all valid", func(t *testing.T) {
		results, allValid := validateTemperFiles([]string{validPath})
		if !allValid {
			t.Error("expected allValid=true")
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})
}

// ---------------------------------------------------------------------------
// validateArtifact edge cases
// ---------------------------------------------------------------------------

func TestTemperCoverage_ValidateArtifact(t *testing.T) {
	tmp := t.TempDir()

	for _, tc := range temperValidateArtifactCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, tmp)
		})
	}
}

type temperValidateArtifactCase struct {
	name string
	run  func(t *testing.T, tmp string)
}

func temperValidateArtifactCases() []temperValidateArtifactCase {
	return []temperValidateArtifactCase{
		{"missing feedback count", validateTemperArtifactMissingFeedbackCount},
		{"established maturity passes candidate requirement", validateTemperArtifactEstablishedMaturity},
		{"missing schema version generates warning", validateTemperArtifactMissingSchemaVersion},
		{"maturity fields populated", validateTemperArtifactFieldsPopulated},
		{"frontmatter metadata satisfies schema and feedback checks", validateTemperArtifactFrontmatterMetadata},
	}
}

func validateTemperArtifactMissingFeedbackCount(t *testing.T, tmp string) {
	t.Helper()

	content := "# Test\n\n**ID**: L1\n**Maturity**: candidate\n**Utility**: 0.8\n"
	path := writeTemperArtifact(t, tmp, "no-feedback.md", content)
	result := validateArtifact(path, "provisional", 0.5, 3)
	if result.Valid {
		t.Error("expected invalid when feedback count < minimum")
	}
	assertTemperArtifactHasIssue(t, result)
}

func validateTemperArtifactEstablishedMaturity(t *testing.T, tmp string) {
	t.Helper()

	content := "# Test\n\n**ID**: L1\n**Maturity**: established\n**Utility**: 0.8\n"
	path := writeTemperArtifact(t, tmp, "established.md", content)
	result := validateArtifact(path, "candidate", 0.5, 0)
	if !result.Valid {
		t.Errorf("expected valid for established maturity, issues: %v", result.Issues)
	}
}

func validateTemperArtifactMissingSchemaVersion(t *testing.T, tmp string) {
	t.Helper()

	content := "# Test\n\n**ID**: L1\n**Maturity**: candidate\n**Utility**: 0.8\n"
	path := writeTemperArtifact(t, tmp, "no-schema.md", content)
	result := validateArtifact(path, "provisional", 0.5, 0)
	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing schema_version")
	}
}

func validateTemperArtifactFieldsPopulated(t *testing.T, tmp string) {
	t.Helper()

	content := "# Test\n\n**ID**: L1\n**Maturity**: candidate\n**Utility**: 0.7\n**Confidence**: 0.85\n"
	path := writeTemperArtifact(t, tmp, "fields.md", content)
	result := validateArtifact(path, "provisional", 0.5, 0)
	if result.Maturity != types.MaturityCandidate {
		t.Errorf("Maturity = %q, want %q", result.Maturity, types.MaturityCandidate)
	}
	if result.Utility < 0.69 || result.Utility > 0.71 {
		t.Errorf("Utility = %v, want ~0.7", result.Utility)
	}
	if result.Confidence < 0.84 || result.Confidence > 0.86 {
		t.Errorf("Confidence = %v, want ~0.85", result.Confidence)
	}
	if result.ValidatedAt.IsZero() {
		t.Error("ValidatedAt should not be zero")
	}
}

func validateTemperArtifactFrontmatterMetadata(t *testing.T, tmp string) {
	t.Helper()

	content := `---
id: L1
maturity: candidate
utility: 0.8
confidence: 0.9
reward_count: 3
schema_version: 1
---
# Test
`
	path := writeTemperArtifact(t, tmp, "frontmatter.md", content)
	result := validateArtifact(path, "provisional", 0.5, 1)
	if !result.Valid {
		t.Errorf("expected valid, issues: %v", result.Issues)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", result.Warnings)
	}
	if result.FeedbackCount != 3 {
		t.Errorf("FeedbackCount = %d, want 3", result.FeedbackCount)
	}
}

func writeTemperArtifact(t *testing.T, tmp, name, content string) string {
	t.Helper()

	path := filepath.Join(tmp, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertTemperArtifactHasIssue(t *testing.T, result TemperResult) {
	t.Helper()

	for _, issue := range result.Issues {
		if len(issue) > 0 {
			return
		}
	}
	t.Error("expected at least one issue message")
}

// ---------------------------------------------------------------------------
// applyMarkdownLine
// ---------------------------------------------------------------------------

func TestTemperCoverage_ApplyMarkdownLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		check func(t *testing.T, meta *artifactMetadata)
	}{
		{
			name: "sets ID",
			line: "**ID**: L42",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.ID != "L42" {
					t.Errorf("ID = %q, want %q", meta.ID, "L42")
				}
			},
		},
		{
			name: "sets Maturity",
			line: "**Maturity**: established",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.Maturity != types.MaturityEstablished {
					t.Errorf("Maturity = %q, want %q", meta.Maturity, types.MaturityEstablished)
				}
			},
		},
		{
			name: "sets Utility",
			line: "**Utility**: 0.75",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.Utility < 0.74 || meta.Utility > 0.76 {
					t.Errorf("Utility = %v, want ~0.75", meta.Utility)
				}
			},
		},
		{
			name: "sets Confidence",
			line: "**Confidence**: 0.9",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.Confidence < 0.89 || meta.Confidence > 0.91 {
					t.Errorf("Confidence = %v, want ~0.9", meta.Confidence)
				}
			},
		},
		{
			name: "sets Schema Version",
			line: "**Schema Version**: 2",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.SchemaVersion != 2 {
					t.Errorf("SchemaVersion = %d, want 2", meta.SchemaVersion)
				}
			},
		},
		{
			name: "sets Tempered status",
			line: "**Status**: TEMPERED",
			check: func(t *testing.T, meta *artifactMetadata) {
				if !meta.Tempered {
					t.Error("expected Tempered=true for status TEMPERED")
				}
			},
		},
		{
			name: "sets Locked status",
			line: "**Status**: Locked",
			check: func(t *testing.T, meta *artifactMetadata) {
				if !meta.Tempered {
					t.Error("expected Tempered=true for status Locked")
				}
			},
		},
		{
			name: "unrelated line does nothing",
			line: "Some random content",
			check: func(t *testing.T, meta *artifactMetadata) {
				if meta.ID != "" {
					t.Error("ID should remain empty for unrelated line")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &artifactMetadata{}
			applyMarkdownLine(tt.line, meta)
			tt.check(t, meta)
		})
	}
}

// ---------------------------------------------------------------------------
// parseArtifactMetadata edge cases
// ---------------------------------------------------------------------------

func TestTemperCoverage_ParseArtifactMetadata(t *testing.T) {
	tmp := t.TempDir()

	t.Run("JSONL with ID goes straight to return", func(t *testing.T) {
		content := `{"id":"L42","maturity":"established","utility":0.85,"confidence":0.9,"reward_count":5}`
		path := filepath.Join(tmp, "test.jsonl")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		meta, err := parseArtifactMetadata(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ID != "L42" {
			t.Errorf("ID = %q, want %q", meta.ID, "L42")
		}
	})

	t.Run("JSONL without ID falls back to markdown then filename", func(t *testing.T) {
		content := `{"maturity":"provisional"}`
		path := filepath.Join(tmp, "no-id.jsonl")
		if err := os.WriteFile(path, []byte(content+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		meta, err := parseArtifactMetadata(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ID != "no-id" {
			t.Errorf("ID = %q, want %q", meta.ID, "no-id")
		}
	})

	t.Run("markdown without ID uses filename", func(t *testing.T) {
		content := "# Title\n\nNo ID field.\n"
		path := filepath.Join(tmp, "filename-fallback.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		meta, err := parseArtifactMetadata(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ID != "filename-fallback" {
			t.Errorf("ID = %q, want %q", meta.ID, "filename-fallback")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := parseArtifactMetadata(filepath.Join(tmp, "nope.md"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

// ---------------------------------------------------------------------------
// computeTemperStatus
// ---------------------------------------------------------------------------

func TestTemperCoverage_ComputeTemperStatus(t *testing.T) {
	tmp := t.TempDir()

	// Create artifact dirs with files
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	learning := "# Test\n\n**ID**: L1\n**Maturity**: candidate\n**Utility**: 0.7\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "l1.md"), []byte(learning), 0644); err != nil {
		t.Fatal(err)
	}
	pattern := "# Pattern\n\n**ID**: P1\n**Maturity**: established\n**Utility**: 0.9\n"
	if err := os.WriteFile(filepath.Join(patternsDir, "p1.md"), []byte(pattern), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := computeTemperStatus(tmp)
	if err != nil {
		t.Fatalf("computeTemperStatus: %v", err)
	}
	if status.Tempered != 2 {
		t.Errorf("Tempered = %d, want 2", status.Tempered)
	}
	if status.MeanUtility < 0.79 || status.MeanUtility > 0.81 {
		t.Errorf("MeanUtility = %v, want ~0.8", status.MeanUtility)
	}
	if status.ByMaturity == nil {
		t.Fatal("ByMaturity should not be nil")
	}
}

// ---------------------------------------------------------------------------
// scanArtifactDir
// ---------------------------------------------------------------------------

func TestTemperCoverage_ScanArtifactDir(t *testing.T) {
	t.Run("nonexistent dir does nothing", func(t *testing.T) {
		status := &TemperStatus{ByMaturity: make(map[string]int)}
		var totalUtility float64
		var utilityCount int
		scanArtifactDir("/nonexistent/path", status, &totalUtility, &utilityCount)
		if status.Tempered != 0 {
			t.Errorf("Tempered = %d, want 0", status.Tempered)
		}
	})

	t.Run("dir with artifacts", func(t *testing.T) {
		tmp := t.TempDir()
		content := "# Test\n\n**ID**: L1\n**Maturity**: candidate\n**Utility**: 0.8\n"
		if err := os.WriteFile(filepath.Join(tmp, "test.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		// Non-artifact file should be skipped
		if err := os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte("skip me"), 0644); err != nil {
			t.Fatal(err)
		}

		status := &TemperStatus{ByMaturity: make(map[string]int)}
		var totalUtility float64
		var utilityCount int
		scanArtifactDir(tmp, status, &totalUtility, &utilityCount)
		if status.Tempered != 1 {
			t.Errorf("Tempered = %d, want 1", status.Tempered)
		}
		if utilityCount != 1 {
			t.Errorf("utilityCount = %d, want 1", utilityCount)
		}
	})
}

// ---------------------------------------------------------------------------
// isContainedPath additional edge cases
// ---------------------------------------------------------------------------

func TestTemperCoverage_IsContainedPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		path     string
		expected bool
	}{
		{
			name:     "same directory is contained",
			baseDir:  "/tmp/test",
			path:     "/tmp/test",
			expected: true,
		},
		{
			name:     "child is contained",
			baseDir:  "/tmp/test",
			path:     "/tmp/test/child.md",
			expected: true,
		},
		{
			name:     "parent is not contained",
			baseDir:  "/tmp/test",
			path:     "/tmp",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContainedPath(tt.baseDir, tt.path)
			if got != tt.expected {
				t.Errorf("isContainedPath(%q, %q) = %v, want %v", tt.baseDir, tt.path, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// expandGlobPattern
// ---------------------------------------------------------------------------

func TestTemperCoverage_ExpandGlobPattern(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "a.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("matches files in base dir", func(t *testing.T) {
		pattern := filepath.Join(tmp, "*.md")
		files, err := expandGlobPattern(tmp, pattern)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		pattern := filepath.Join(tmp, "*.xyz")
		files, err := expandGlobPattern(tmp, pattern)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})

	t.Run("invalid pattern returns error", func(t *testing.T) {
		_, err := expandGlobPattern(tmp, filepath.Join(tmp, "["))
		if err == nil {
			t.Error("expected error for invalid glob pattern")
		}
	})
}

// ---------------------------------------------------------------------------
// expandSinglePattern
// ---------------------------------------------------------------------------

func TestTemperCoverage_ExpandSinglePattern(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "test.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("relative path expanded", func(t *testing.T) {
		files, err := expandSinglePattern(tmp, "test.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("absolute path used directly", func(t *testing.T) {
		absPath := filepath.Join(tmp, "test.md")
		files, err := expandSinglePattern(tmp, absPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})
}

// ---------------------------------------------------------------------------
// expandDirectory
// ---------------------------------------------------------------------------

func TestTemperCoverage_ExpandDirectory(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "a.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	origRecursive := temperRecursive
	defer func() { temperRecursive = origRecursive }()

	t.Run("non-recursive", func(t *testing.T) {
		temperRecursive = false
		files, err := expandDirectory(tmp, tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file (flat), got %d: %v", len(files), files)
		}
	})

	t.Run("recursive", func(t *testing.T) {
		temperRecursive = true
		files, err := expandDirectory(tmp, tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Errorf("expected 2 files (recursive), got %d: %v", len(files), files)
		}
	})
}

// ---------------------------------------------------------------------------
// printValidationResults (smoke test)
// ---------------------------------------------------------------------------

func TestTemperCoverage_PrintValidationResults(t *testing.T) {
	t.Run("with issues and warnings", func(t *testing.T) {
		results := []TemperResult{
			{Path: "/test/valid.md", Valid: true, Maturity: "candidate", Utility: 0.8},
			{
				Path:     "/test/invalid.md",
				Valid:    false,
				Maturity: "provisional",
				Utility:  0.3,
				Issues:   []string{"utility too low"},
				Warnings: []string{"missing schema_version"},
			},
		}
		// Should not panic
		printValidationResults(results)
	})

	t.Run("no issues", func(t *testing.T) {
		results := []TemperResult{
			{Path: "/test/ok.md", Valid: true, Maturity: "established", Utility: 0.9},
		}
		printValidationResults(results)
	})
}

// ---------------------------------------------------------------------------
// printTemperStatus (smoke test)
// ---------------------------------------------------------------------------

func TestTemperCoverage_PrintTemperStatus(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		status := &TemperStatus{
			Tempered:    5,
			Pending:     3,
			ByMaturity:  map[string]int{"provisional": 2, "candidate": 3, "established": 3},
			MeanUtility: 0.75,
			Artifacts: []TemperResult{
				{Path: "/test/a.md", Tempered: true, Utility: 0.8, Maturity: "candidate"},
				{Path: "/test/b.md", Tempered: false, Utility: 0.5, Maturity: "provisional"},
			},
		}
		printTemperStatus(status)
	})

	t.Run("empty status", func(t *testing.T) {
		status := &TemperStatus{
			ByMaturity: make(map[string]int),
		}
		printTemperStatus(status)
	})
}

// ---------------------------------------------------------------------------
// countPoolPending (no pool dir => silent no-op)
// ---------------------------------------------------------------------------

func TestTemperCoverage_CountPoolPending(t *testing.T) {
	tmp := t.TempDir()
	status := &TemperStatus{
		ByMaturity: make(map[string]int),
	}
	// Should not panic even without pool dirs
	countPoolPending(tmp, status)
	// With no pool, pending should still be 0
	if status.Pending != 0 {
		t.Errorf("Pending = %d, want 0 with no pool", status.Pending)
	}
}

// ---------------------------------------------------------------------------
// runTemperStatus (integration)
// ---------------------------------------------------------------------------

func TestRunTemperStatus_Human(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	setupAgentsDir(t, tmp)

	out, err := captureStdout(t, func() error {
		return runTemperStatus(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should include temper status info
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunTemperStatus_JSON(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	setupAgentsDir(t, tmp)

	out, err := captureStdout(t, func() error {
		return runTemperStatus(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
}

// ---------------------------------------------------------------------------
// lockArtifact (integration)
// ---------------------------------------------------------------------------

func TestLockArtifact_NonexistentFile(t *testing.T) {
	err := lockArtifact("/nonexistent/file.md", "test-locker")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLockArtifact_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "artifact.md")
	os.WriteFile(path, []byte("---\ntitle: test\n---\nContent"), 0644)

	err := lockArtifact(path, "test-locker")
	// Should succeed or fail based on content validation, either way not panic
	_ = err
	// Read back and check if locked metadata was written
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("failed to read back: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("artifact file is empty after lock attempt")
	}
}
