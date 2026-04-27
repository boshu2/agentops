package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runContradictJSONForTest(t *testing.T, cwd string) ContradictResult {
	t.Helper()

	var out bytes.Buffer
	err := runContradictWithOptions(contradictOptions{
		Cwd:    cwd,
		Output: "json",
		Writer: &out,
	})
	if err != nil {
		t.Fatalf("runContradictWithOptions returned error: %v", err)
	}

	var result ContradictResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decoding JSON output: %v\noutput: %s", err, out.String())
	}
	return result
}

func TestContradictDetection_NoFiles(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}
	// Empty directory — no files

	result := runContradictJSONForTest(t, tmp)
	if result.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", result.TotalFiles)
	}
	if result.Contradictions != 0 {
		t.Errorf("Contradictions = %d, want 0", result.Contradictions)
	}
}

func TestContradictDetection_HumanNoDirsUsesWriter(t *testing.T) {
	var out bytes.Buffer
	err := runContradictWithOptions(contradictOptions{
		Cwd:    t.TempDir(),
		Output: "text",
		Writer: &out,
	})
	if err != nil {
		t.Fatalf("runContradictWithOptions returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No learnings or patterns directory found.") {
		t.Fatalf("expected no-directory message, got: %q", out.String())
	}
}

func TestContradictDetection_NoContradictions(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Two completely unrelated learnings
	file1 := filepath.Join(learningsDir, "learn-database.md")
	file2 := filepath.Join(learningsDir, "learn-network.md")

	if err := os.WriteFile(file1, []byte("---\ntitle: Database Indexing\n---\n# Database Indexing Strategies\n\nB-tree indexes improve query performance on large tables with sorted data."), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Network Protocols\n---\n# TCP vs UDP Protocol Selection\n\nTCP provides reliable delivery while UDP offers lower latency for streaming."), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	result := runContradictJSONForTest(t, tmp)

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.Contradictions != 0 {
		t.Errorf("Contradictions = %d, want 0", result.Contradictions)
	}
}

func TestContradictDetection_FindsContradiction(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Two learnings about the same topic with contradictory advice
	file1 := filepath.Join(learningsDir, "learn-mutex-yes.md")
	file2 := filepath.Join(learningsDir, "learn-mutex-no.md")

	if err := os.WriteFile(file1, []byte("---\ntitle: Mutex Usage\n---\n# Always Use Mutex for Shared State\n\nAlways use mutex locks when accessing shared state in concurrent goroutines. Mutex ensures data consistency and prevents race conditions in concurrent code."), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Mutex Avoidance\n---\n# Never Use Mutex for Shared State\n\nNever use mutex locks when accessing shared state in concurrent goroutines. Channels are the idiomatic approach and avoid deadlock risks in concurrent code."), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	result := runContradictJSONForTest(t, tmp)

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.Contradictions < 1 {
		t.Errorf("Contradictions = %d, want >= 1", result.Contradictions)
	}
	if len(result.Pairs) < 1 {
		t.Fatal("expected at least one contradiction pair")
	}

	pair := result.Pairs[0]
	if pair.Similarity < 0.4 {
		t.Errorf("Similarity = %.2f, want >= 0.4", pair.Similarity)
	}
	if pair.Reason == "" {
		t.Error("Reason should not be empty for detected contradiction")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		a, b    map[string]bool
		wantMin float64
		wantMax float64
	}{
		{
			name:    "identical sets",
			a:       map[string]bool{"mutex": true, "shared": true, "state": true},
			b:       map[string]bool{"mutex": true, "shared": true, "state": true},
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name:    "completely disjoint",
			a:       map[string]bool{"mutex": true, "lock": true},
			b:       map[string]bool{"database": true, "query": true},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "partial overlap",
			a:       map[string]bool{"mutex": true, "shared": true, "state": true, "lock": true},
			b:       map[string]bool{"mutex": true, "shared": true, "state": true, "channel": true},
			wantMin: 0.5,
			wantMax: 0.7,
		},
		{
			name:    "both empty",
			a:       map[string]bool{},
			b:       map[string]bool{},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "one empty",
			a:       map[string]bool{"mutex": true},
			b:       map[string]bool{},
			wantMin: 0.0,
			wantMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.a, tt.b)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("jaccardSimilarity() = %.4f, want [%.4f, %.4f]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	words := tokenize("Always use mutex for shared state. Don't forget!")
	// "use" is 3 chars so included, "for" is 3 chars so included
	if !words["always"] {
		t.Error("expected 'always' in token set")
	}
	if !words["mutex"] {
		t.Error("expected 'mutex' in token set")
	}
	if !words["don't"] {
		t.Error("expected \"don't\" in token set")
	}
	// "a" and "is" would be excluded (< 3 chars)
	if words["a"] {
		t.Error("did not expect 'a' in token set (too short)")
	}
}

func TestDetectContradiction_NegationAsymmetry(t *testing.T) {
	a := "Always use mutex locks for shared state in goroutines."
	b := "Never use mutex locks for shared state in goroutines."
	reason := detectContradiction(a, b)
	if reason == "" {
		t.Error("expected contradiction detection for always/never pair")
	}
}

func TestDetectContradiction_NoContradiction(t *testing.T) {
	a := "Use mutex locks for shared state."
	b := "Use channels for message passing."
	reason := detectContradiction(a, b)
	if reason != "" {
		t.Errorf("expected no contradiction, got: %s", reason)
	}
}

func TestCountNegations(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"always use mutex", 0},
		{"never use mutex", 1},
		{"don't avoid using mutex", 2},
		{"not recommended and never tried", 2},
	}
	for _, tt := range tests {
		got := countNegations(tt.text)
		if got != tt.want {
			t.Errorf("countNegations(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestDetectContradiction_OpposingTerms(t *testing.T) {
	// Use "enable" vs "disable" — neither is a negation word, so the negation
	// asymmetry check won't fire and the opposition pair check is reached.
	result := detectContradiction("enable logging for production", "disable logging for production")
	if result == "" {
		t.Fatal("expected opposing terms detection for 'enable' vs 'disable'")
	}
	if !strings.Contains(result, "opposing terms") {
		t.Errorf("expected 'opposing terms' in result, got %q", result)
	}
}
