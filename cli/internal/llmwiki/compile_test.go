package llmwiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mkVaultWithSources stands up a vault whose wiki/sources/ already holds
// ingested distillations (the input CompileStage consumes). Each entry maps a
// slug to the raw body that the (fake) compiler will derive artifacts from.
func mkVaultWithSources(t *testing.T, sources map[string]string) string {
	t.Helper()
	vault := t.TempDir()
	srcDir := filepath.Join(vault, "wiki", "sources")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir sources: %v", err)
	}
	for slug, body := range sources {
		header := renderFrontmatter(stageFrontmatter{
			Type:    "source",
			Stage:   StageIngest,
			Source:  "raw/" + slug + ".md",
			Created: fixedNow(),
			Attempt: 1,
		})
		full := header + "\n# " + slug + "\n\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(srcDir, slug+".md"), []byte(full), 0o644); err != nil {
			t.Fatalf("write source %s: %v", slug, err)
		}
	}
	return vault
}

// readArtifacts collects every .md file under wiki/ (relative to vault) so a
// test can assert on the full compiled artifact set.
func readArtifacts(t *testing.T, vault string) map[string]string {
	t.Helper()
	out := map[string]string{}
	root := filepath.Join(vault, "wiki")
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(vault, p)
		if rerr != nil {
			return rerr
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk wiki: %v", err)
	}
	return out
}

// ---------------------------------------------------------------------------
// CompileStage — happy path produces all five artifact kinds.
// ---------------------------------------------------------------------------

func TestCompileStage_CompileProducesAllArtifacts(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"alpha": "# Fitness Gradient\nAlpha covers Reliability and Membrane.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}

	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stage != StageCompile {
		t.Errorf("result.Stage = %s, want %s", result.Stage, StageCompile)
	}
	if result.Skipped {
		t.Errorf("expected non-skipped compile, got %+v", result)
	}

	arts := readArtifacts(t, vault)

	// 1. summary present at the expected path.
	if _, ok := arts["wiki/summaries/alpha.md"]; !ok {
		t.Errorf("missing summary artifact; got paths: %v", sortedKeys(arts))
	}
	// 2. at least one concept and one entity file.
	var concepts, entities int
	for p := range arts {
		if strings.HasPrefix(p, "wiki/concepts/") {
			concepts++
		}
		if strings.HasPrefix(p, "wiki/entities/") {
			entities++
		}
	}
	if concepts == 0 {
		t.Errorf("expected >=1 concept file, got 0; paths: %v", sortedKeys(arts))
	}
	if entities == 0 {
		t.Errorf("expected >=1 entity file, got 0; paths: %v", sortedKeys(arts))
	}
	// 3. index + log present.
	if _, ok := arts["wiki/index.md"]; !ok {
		t.Errorf("missing wiki/index.md")
	}
	if _, ok := arts["wiki/log.md"]; !ok {
		t.Errorf("missing wiki/log.md")
	}
}

func TestCompileStage_SummaryHasValidFrontmatterAndShape(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"beta": "# Topic Beta\nBeta is about Orchestration.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("Run: %v", err)
	}

	summary := filepath.Join(vault, "wiki", "summaries", "beta.md")
	data, err := os.ReadFile(summary)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Errorf("summary missing frontmatter delimiter: %s", text)
	}
	if !strings.Contains(text, "attempt: 1") {
		t.Errorf("summary missing 'attempt: 1': %s", text)
	}
	if !strings.Contains(text, "type: summary") {
		t.Errorf("summary missing 'type: summary': %s", text)
	}
	// hasValidArtifact must accept the written summary (idempotency probe).
	if !hasValidArtifact(summary) {
		t.Errorf("hasValidArtifact rejected a freshly written summary")
	}
}

func TestCompileStage_ConceptFilesAreWikilinkedFromIndex(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"gamma": "# Membrane Concept\nGamma defines the Verification membrane.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("Run: %v", err)
	}

	arts := readArtifacts(t, vault)
	index, ok := arts["wiki/index.md"]
	if !ok {
		t.Fatalf("missing index.md")
	}

	// Every concept file must be wikilinked from the index by its slug.
	var sawConceptLink bool
	for p := range arts {
		if !strings.HasPrefix(p, "wiki/concepts/") {
			continue
		}
		slug := strings.TrimSuffix(filepath.Base(p), ".md")
		link := "[[concepts/" + slug + "]]"
		if !strings.Contains(index, link) {
			t.Errorf("concept %q not wikilinked from index; index=%s", slug, index)
		}
		sawConceptLink = true
	}
	if !sawConceptLink {
		t.Fatalf("no concept files were produced to verify wikilinks")
	}
}

func TestCompileStage_EntityFileHasFrontmatterAndBacklink(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"delta": "# Source Delta\nDelta mentions Athena and Hephaestus.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entitiesDir := filepath.Join(vault, "wiki", "entities")
	entries, err := os.ReadDir(entitiesDir)
	if err != nil {
		t.Fatalf("read entities: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no entity files produced")
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(entitiesDir, e.Name()))
		if err != nil {
			t.Fatalf("read entity %s: %v", e.Name(), err)
		}
		text := string(data)
		if !strings.Contains(text, "type: entity") {
			t.Errorf("entity %s missing 'type: entity': %s", e.Name(), text)
		}
		// Entity pages backlink to the source they were extracted from.
		if !strings.Contains(text, "[[sources/delta]]") {
			t.Errorf("entity %s missing source backlink [[sources/delta]]: %s", e.Name(), text)
		}
	}
}

func TestCompileStage_LogRecordsCompiledSources(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"epsilon": "# Logged\nEpsilon body.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("Run: %v", err)
	}

	logData, err := os.ReadFile(filepath.Join(vault, "wiki", "log.md"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logData), "epsilon") {
		t.Errorf("log.md does not record compiled source 'epsilon': %s", logData)
	}
}

// ---------------------------------------------------------------------------
// Idempotency: compiling the same sources twice is byte-stable.
// ---------------------------------------------------------------------------

func TestCompileStage_IdempotentByteStable(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"alpha": "# Alpha\nAlpha covers Reliability.",
		"omega": "# Omega\nOmega covers Membrane and Orchestration.",
	})
	// Deliberately use the PRODUCTION time path (Now == nil -> time.Now()). A
	// fixed Now would mask a non-deterministic artifact (the log.md wall-clock
	// bug): idempotency must hold for real recompiles, not just injected-time ones.
	stage := &CompileStage{Compiler: NewDeterministicCompiler()}

	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("first compile: %v", err)
	}
	first := readArtifacts(t, vault)

	result, err := stage.Run(context.Background(), vault, 2)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}
	if !result.Skipped {
		t.Errorf("expected Skipped=true on no-op recompile, got %+v", result)
	}
	second := readArtifacts(t, vault)

	if len(first) != len(second) {
		t.Fatalf("artifact set changed across runs: %v vs %v", sortedKeys(first), sortedKeys(second))
	}
	for path, before := range first {
		after, ok := second[path]
		if !ok {
			t.Errorf("artifact %s disappeared on recompile", path)
			continue
		}
		if before != after {
			t.Errorf("artifact %s changed on no-op recompile\nbefore:\n%s\nafter:\n%s", path, before, after)
		}
	}
}

// TestCompileStage_CompileDistinctSlugCollisionValues asserts that two distinct
// concept values that slugify to the same base ("C++" and "C#" both -> "c") get
// DISTINCT pages rather than silently collapsing to one (data loss).
func TestCompileStage_CompileDistinctSlugCollisionValues(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"langs": "# C++\nNotes on C++.\n## C#\nNotes on C#.",
	})
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("Run: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(vault, "wiki", "concepts"))
	if err != nil {
		t.Fatalf("read concepts: %v", err)
	}
	md := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			md++
		}
	}
	if md < 2 {
		t.Fatalf("C++ and C# collapsed to %d concept page(s), want 2 distinct (slug collision)", md)
	}
	// And both must be wikilinked from the index (no silent drop).
	index, err := os.ReadFile(filepath.Join(vault, "wiki", "index.md"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if strings.Count(string(index), "[[concepts/") < 2 {
		t.Errorf("index links < 2 concept pages: %s", index)
	}
}

// TestPageSlug_CollisionResistantAcrossLossyClasses pins that pageSlug keeps
// DISTINCT values apart across every way slugify is lossy (stripped runes, the
// 80-char truncation cap, run-collapsing) while keeping canonical-equal values
// on one slug (dedup across sources). No fixed-width slug is perfectly injective
// (pigeonhole); this asserts the realistic lossy-collision classes are resolved
// and that the dedup invariant holds. A per-case detection scheme missed
// truncation, so the fix hashes the canonical value to close the whole class.
func TestPageSlug_CollisionResistantAcrossLossyClasses(t *testing.T) {
	long := strings.Repeat("a", 90)
	distinct := [][2]string{
		{"C++", "C#"},            // stripped-rune collision -> both slugify to "c"
		{long + "X", long + "Y"}, // 80-char truncation collision
		{"Foo.Bar", "Foo-Bar"},   // run-collapse collision -> both slugify to "foo-bar"
	}
	for _, c := range distinct {
		if pageSlug(c[0]) == pageSlug(c[1]) {
			t.Errorf("distinct values %q and %q collide on slug %q", c[0], c[1], pageSlug(c[0]))
		}
	}
	// Canonical-equal values (case / whitespace variants) MUST share a slug so a
	// concept named the same way in two sources stays one page.
	same := [][2]string{
		{"Recursion", "recursion"},
		{"Foo Bar", "foo  bar"},
	}
	for _, c := range same {
		if pageSlug(c[0]) != pageSlug(c[1]) {
			t.Errorf("canonical-equal values %q and %q must share a slug, got %q vs %q", c[0], c[1], pageSlug(c[0]), pageSlug(c[1]))
		}
	}
}

// ---------------------------------------------------------------------------
// Contracts preserved: scope guard, ctx cancellation.
// ---------------------------------------------------------------------------

func TestCompileStage_CtxCancellationStopsCompile(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{
		"a": "# A\nbody a",
		"b": "# B\nbody b",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	_, err := stage.Run(ctx, vault, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// No summary should have been written.
	summariesDir := filepath.Join(vault, "wiki", "summaries")
	entries, statErr := os.ReadDir(summariesDir)
	if statErr != nil && !os.IsNotExist(statErr) {
		t.Fatalf("unexpected stat error: %v", statErr)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("summary written despite cancelled ctx: %s", e.Name())
		}
	}
}

func TestCompileStage_NoSourcesReturnsSkipped(t *testing.T) {
	vault := t.TempDir()
	stage := &CompileStage{Now: fixedNow, Compiler: NewDeterministicCompiler()}
	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Skipped || result.SkipReason != "no-sources-dir" {
		t.Errorf("expected Skipped+no-sources-dir, got %+v", result)
	}
}

func TestCompileStage_NilCompilerErrors(t *testing.T) {
	vault := mkVaultWithSources(t, map[string]string{"x": "# X\nbody"})
	stage := &CompileStage{Now: fixedNow}
	if _, err := stage.Run(context.Background(), vault, 1); err == nil {
		t.Fatal("expected error when Compiler is nil")
	}
}

// ---------------------------------------------------------------------------
// DeterministicCompiler unit behavior.
// ---------------------------------------------------------------------------

func TestDeterministicCompiler_SummaryIsFirstNonHeadingLine(t *testing.T) {
	c := NewDeterministicCompiler()
	summary, err := c.Summarize(context.Background(), "# Heading\n\nThe real first line.\nSecond line.")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if summary != "The real first line." {
		t.Errorf("Summarize = %q, want %q", summary, "The real first line.")
	}
}

func TestDeterministicCompiler_ConceptsFromHeadings(t *testing.T) {
	c := NewDeterministicCompiler()
	concepts, err := c.Concepts(context.Background(), "# First Concept\nbody\n## Second Concept\nmore")
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	got := append([]string(nil), concepts...)
	sort.Strings(got)
	want := []string{"First Concept", "Second Concept"}
	if len(got) != len(want) {
		t.Fatalf("Concepts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Concepts[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeterministicCompiler_EntitiesAreDeduped(t *testing.T) {
	c := NewDeterministicCompiler()
	entities, err := c.Entities(context.Background(), "Athena met Athena and Hephaestus near the Forge.")
	if err != nil {
		t.Fatalf("Entities: %v", err)
	}
	seen := map[string]int{}
	for _, e := range entities {
		seen[e]++
	}
	for e, n := range seen {
		if n != 1 {
			t.Errorf("entity %q appears %d times, want deduped (1)", e, n)
		}
	}
	if seen["Athena"] != 1 {
		t.Errorf("expected 'Athena' extracted once, got %d", seen["Athena"])
	}
}

// sortedKeys is a small test helper for stable diagnostic output.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
