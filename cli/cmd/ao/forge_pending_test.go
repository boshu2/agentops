// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/boshu2/agentops/cli/internal/extract"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/storage"
)

func TestWritePendingLearnings_WritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "test-session-abc123",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Always check error returns from file operations",
			"Use table-driven tests for multi-case functions",
			"Gini coefficient measures inequality in distributions",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 files written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 files in pending dir, got %d", len(entries))
	}

	// Verify first file content
	data, err := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Learning:") {
		t.Error("expected '# Learning:' heading in output")
	}
	if !strings.Contains(content, "**ID**:") {
		t.Error("expected '**ID**:' metadata in output")
	}
	if !strings.Contains(content, "**Category**:") {
		t.Error("expected '**Category**:' metadata in output")
	}
	if !strings.Contains(content, "**Confidence**: medium") {
		t.Error("expected '**Confidence**: medium' in output")
	}
}

func TestWritePendingLearnings_IncludesDecisions(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "test-decisions-def456",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Decisions: []string{
			"We decided to use auto-promote instead of relay",
			"Selected Go over Python for CLI performance",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 files, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)
	if !strings.Contains(content, "**Category**: decision") {
		t.Errorf("expected category 'decision' for decisions, got: %s", content)
	}
}

func TestWritePendingLearnings_EmptySession(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "empty-session",
		Date: time.Now(),
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 files for empty session, got %d", n)
	}
}

func TestWritePendingLearnings_NilSession(t *testing.T) {
	n, err := writePendingLearnings(nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 for nil session, got %d", n)
	}
}

func TestWritePendingLearnings_FrontmatterFormat(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:        "frontmatter-test-789",
		Date:      time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{"Test frontmatter is correct"},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("expected YAML frontmatter start")
	}
	if !strings.Contains(content, "date: 2026-03-20") {
		t.Error("expected date in frontmatter")
	}
	if !strings.Contains(content, "type: learning") {
		t.Error("expected type in frontmatter")
	}
	if !strings.Contains(content, "source: frontmatter-test-789") {
		t.Error("expected source session ID in frontmatter")
	}
}

func TestWritePendingLearnings_PoolIngestCompatible(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "compat-test-abc",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Pool ingest reads # Learning: headings with **ID**, **Category**, **Confidence** fields",
		},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	// Verify parseLearningBlocks can parse this
	blocks := parseLearningBlocks(content)
	if len(blocks) != 1 {
		t.Fatalf("expected parseLearningBlocks to find 1 block, got %d", len(blocks))
	}
	if blocks[0].ID == "" {
		t.Error("expected parsed block to have an ID")
	}
	if blocks[0].Category == "" {
		t.Error("expected parsed block to have a Category")
	}
	if blocks[0].Confidence == "" {
		t.Error("expected parsed block to have a Confidence")
	}
}

func TestWritePendingLearnings_ResearchProvenance(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "provenance-test-123",
		Date: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Based on .agents/research/2026-03-22-flywheel-gap.md we found that escape velocity alone is insufficient for compounding claims",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 file written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if !strings.Contains(content, "research_sources:") {
		t.Error("expected research_sources: in frontmatter when knowledge references research files")
	}
	if !strings.Contains(content, ".agents/research/2026-03-22-flywheel-gap.md") {
		t.Error("expected exact research file path in research_sources frontmatter")
	}
}

func TestWritePendingLearnings_NoResearchProvenance(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "no-provenance-456",
		Date: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Generic knowledge without research references",
		},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if strings.Contains(content, "research_sources:") {
		t.Error("expected NO research_sources in frontmatter when knowledge has no research references")
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"We decided to use Go", "decision"},
		{"Selected approach A over B", "decision"},
		{"The test failed because of a race condition", "failure"},
		{"Fixed the bug by adding a mutex", "solution"},
		{"Always run tests before committing", "learning"},
	}

	for _, tt := range tests {
		got := inferCategory(tt.text)
		if got != tt.expected {
			t.Errorf("inferCategory(%q) = %q, want %q", tt.text, got, tt.expected)
		}
	}
}

func TestPendingTitle(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Simple title", "Simple title"},
		{"First line\nSecond line", "First line"},
		{"# Markdown heading", "Markdown heading"},
		{"", "Extracted knowledge"},
		{strings.Repeat("x", 100), strings.Repeat("x", 77) + "..."},
	}

	for _, tt := range tests {
		got := pendingTitle(tt.text)
		if got != tt.expected {
			t.Errorf("pendingTitle(%q) = %q, want %q", tt.text, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// age-2jf: forge typed extraction opt-in (VALUE-GATED, default off).
// ---------------------------------------------------------------------------

// forgeFakeGen implements llm.Generator for the typed-path tests without a live
// model. It returns one scripted response (then empty results, which the
// extractor filters).
type forgeFakeGen struct {
	response string
	calls    int
}

func (f *forgeFakeGen) Generate(prompt string) (string, error) {
	f.calls++
	if f.calls == 1 {
		return f.response, nil
	}
	return "", nil
}
func (f *forgeFakeGen) Digest() string     { return "sha256:forge-fake" }
func (f *forgeFakeGen) ContextBudget() int { return 8192 }
func (f *forgeFakeGen) ModelName() string  { return "forge-fake-extractor" }

// TestForge_LearningShapeReconciled (ac-2jf.1): the canonical learning shape
// round-trips writer→reader. age-ktd reconciled the markdown path
// (TestLearning_CanonicalRoundTrip). This forge-level assertion confirms the
// emitted record stays schema-consistent: the writer-produced file is read back
// by the production reader and the load-bearing canonical fields survive.
func TestForge_LearningShapeReconciled(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "reconciled-session-xyz789",
		Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Prefer L2 integration tests for the forge seam",
		},
		Decisions: []string{
			"We decided to gate the typed path behind an opt-in flag",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 records written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files on disk, got %d", len(entries))
	}

	for _, e := range entries {
		path := filepath.Join(pendingDir, e.Name())
		got, err := search.ParseLearningFile(path)
		if err != nil {
			t.Fatalf("read back %s: %v", e.Name(), err)
		}
		// The reconciled canonical shape carries id/title/category, no filename
		// fallback for the id, no "Learning: " prefix on the title.
		if got.ID == "" {
			t.Errorf("%s: canonical ID empty after round-trip", e.Name())
		}
		if strings.HasPrefix(got.ID, e.Name()) {
			t.Errorf("%s: ID %q fell back to filename (reconciliation lost)", e.Name(), got.ID)
		}
		if got.Title == "" {
			t.Errorf("%s: Title empty after round-trip", e.Name())
		}
		if strings.HasPrefix(got.Title, "Learning:") {
			t.Errorf("%s: Title %q carries the body-heading prefix (not reconciled)", e.Name(), got.Title)
		}
		if got.Category == "" {
			t.Errorf("%s: Category empty after round-trip", e.Name())
		}
		if got.Summary == "" {
			t.Errorf("%s: Summary empty after round-trip", e.Name())
		}
	}
}

// TestForge_TypedEmissionSchemaValid (ac-2jf.2): with the typed opt-in ON and a
// fake Generator returning valid structured JSON, the typed path emits records
// that validate against schemas/learning.v1.schema.json. No live model.
func TestForge_TypedEmissionSchemaValid(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "typed-session-abc",
		Date: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"The pre-push gate rewrites the landed commit, so read merge SHA from origin.",
		},
		Decisions: []string{
			"We decided to wire the typed extractor behind a value gate.",
		},
	}

	// Fake structured-output JSON matching the provenance template's chunkOutput
	// shape (entities with node_type/id/summary).
	fakeJSON := `{
	  "entities": [
	    {"node_type": "decision", "id": "decision-value-gate", "summary": "Wire the typed extractor behind a value gate"},
	    {"node_type": "artifact", "id": "cli/cmd/ao/forge_typed.go", "summary": "Typed opt-in forge path"}
	  ],
	  "relations": []
	}`

	client, err := extract.NewClientWithGenerator(extract.BackendBushidoLlama, &forgeFakeGen{response: fakeJSON})
	if err != nil {
		t.Fatalf("build fake client: %v", err)
	}

	n, err := writeTypedPendingLearnings(session, dir, client)
	if err != nil {
		t.Fatalf("writeTypedPendingLearnings: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 typed records emitted, got %d", n)
	}

	schema := compileLearningSchemaForTest(t)
	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 typed files, got %d", len(entries))
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".learning.json") {
			t.Errorf("typed emission produced non-JSON file %s", e.Name())
			continue
		}
		data, err := os.ReadFile(filepath.Join(pendingDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		if err := schema.Validate(inst); err != nil {
			t.Errorf("%s violates learning.v1.schema.json: %v\n%s", e.Name(), err, data)
		}
	}
}

// TestForge_DefaultUnchangedUntilGate (ac-2jf.3, LOAD-BEARING): with the opt-in
// OFF (default), the pending-learning output is byte-for-byte identical to the
// pre-change heuristic behavior. emitPendingLearnings must route to
// writePendingLearnings verbatim when the gate is off. We assert by comparing
// emitPendingLearnings output against a direct writePendingLearnings call over
// the same fixture session, byte-for-byte.
func TestForge_DefaultUnchangedUntilGate(t *testing.T) {
	// Ensure the env opt-in is not set for this test.
	t.Setenv(forgeTypedEnv, "")
	if forgeTyped {
		t.Fatal("precondition: forgeTyped flag must default to false")
	}
	if typedExtractionEnabled() {
		t.Fatal("precondition: typed extraction must be OFF by default")
	}

	mkSession := func() *storage.Session {
		return &storage.Session{
			ID:   "default-unchanged-session-001",
			Date: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
			Knowledge: []string{
				"Always check error returns from file operations",
				"Use table-driven tests for multi-case functions",
			},
			Decisions: []string{
				"We decided to keep the heuristic fallback",
			},
		}
	}

	// Reference: the pre-change behavior is exactly writePendingLearnings.
	refDir := t.TempDir()
	refN, err := writePendingLearnings(mkSession(), refDir)
	if err != nil {
		t.Fatalf("reference writePendingLearnings: %v", err)
	}

	// Default path: emitPendingLearnings with the gate off.
	gotDir := t.TempDir()
	gotN, err := emitPendingLearnings(mkSession(), gotDir)
	if err != nil {
		t.Fatalf("emitPendingLearnings (default): %v", err)
	}

	if gotN != refN {
		t.Fatalf("default emit wrote %d records, reference wrote %d", gotN, refN)
	}

	refFiles := readPendingDirForTest(t, refDir)
	gotFiles := readPendingDirForTest(t, gotDir)
	if len(refFiles) != len(gotFiles) {
		t.Fatalf("file-count drift: default=%d reference=%d", len(gotFiles), len(refFiles))
	}
	for name, refContent := range refFiles {
		gotContent, ok := gotFiles[name]
		if !ok {
			t.Errorf("default path missing file %s present in reference", name)
			continue
		}
		if gotContent != refContent {
			t.Errorf("BYTE DRIFT in %s: default output differs from pre-change behavior\n--- reference ---\n%s\n--- default ---\n%s", name, refContent, gotContent)
		}
	}
	// Guard: no typed JSON files leaked into the default path.
	for name := range gotFiles {
		if strings.HasSuffix(name, ".learning.json") {
			t.Errorf("default path emitted a typed file %s (gate leaked)", name)
		}
	}
}

// readPendingDirForTest reads every file in the pending dir into a name→content
// map for byte-for-byte comparison.
func readPendingDirForTest(t *testing.T, baseDir string) map[string]string {
	t.Helper()
	pendingDir := filepath.Join(baseDir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir %s: %v", pendingDir, err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(pendingDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(data)
	}
	return out
}

// compileLearningSchemaForTest compiles the repo-root learning.v1 JSON Schema.
// Tests run with cwd = cli/cmd/ao, so the schema is three levels up.
func compileLearningSchemaForTest(t *testing.T) *jsonschema.Schema {
	t.Helper()
	const path = "../../../schemas/learning.v1.schema.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read learning schema %s: %v", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse learning schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("learning.v1.schema.json", doc); err != nil {
		t.Fatalf("add learning schema resource: %v", err)
	}
	schema, err := c.Compile("learning.v1.schema.json")
	if err != nil {
		t.Fatalf("compile learning schema: %v", err)
	}
	return schema
}

