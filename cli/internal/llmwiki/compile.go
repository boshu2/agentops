package llmwiki

// CompileStage is the OpenKB-style compilation pass
// (age-port-openkb-into-agentops-go-5qw.3). It consumes the ingested source
// distillations under wiki/sources/ and emits the derived wiki artifacts:
//
//   - wiki/summaries/<slug>.md  — one summary per ingested source.
//   - wiki/concepts/<slug>.md   — one page per extracted concept.
//   - wiki/entities/<slug>.md   — one page per extracted entity.
//   - wiki/index.md             — wikilink index of every generated page.
//   - wiki/log.md               — compile log of what was compiled this run.
//
// It honors the same three non-negotiable contracts as the other stages
// (scope guard via SafeAtomicWrite, atomic write, ctx plumbing) and is
// idempotent: a no-op recompile (every source already has a valid summary
// artifact) is byte-stable and reports Skipped. Extraction itself is delegated
// to a pluggable Compiler so the real LLM call can be swapped in without
// touching the artifact-writing logic.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// pageSlug returns the filesystem slug for a concept/entity VALUE: a readable
// slugify prefix plus a deterministic 64-bit hash of the canonical value. No
// fixed-width slug can be perfectly injective (pigeonhole), but the hash makes
// collisions between distinct REALISTIC values negligible while closing every
// slugify lossiness class at once — stripped non-alnum runes ("C++"/"C#" both
// slugify to "c"), the 80-char truncation cap, and run-collapsing — rather than
// detecting each (a losing game; truncation slipped past the first attempt). The
// canonical key lower-cases and collapses whitespace, so "Foo Bar" and
// "foo  bar" dedup to ONE page while genuinely distinct values practically never
// collide (a 64-bit collision needs ~2^32 distinct values by the birthday bound).
func pageSlug(value string) string {
	canonical := strings.ToLower(strings.Join(strings.Fields(value), " "))
	return slugify(value) + "-" + shortHash(canonical)
}

// shortHash returns the first 16 hex chars (64 bits) of the SHA-256 of s — a
// stable, collision-resistant slug disambiguator. Not a security primitive.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s)) // #nosec G401 nosemgrep -- slug disambiguation, not a security primitive.
	return hex.EncodeToString(sum[:])[:16]
}

// CompileStage compiles ingested sources into summary/concept/entity pages
// plus an index and a compile log. The Compiler is injected; tests use the
// DeterministicCompiler so no model call is required.
type CompileStage struct {
	// Now is injected for deterministic frontmatter timestamps in tests.
	Now func() time.Time
	// Compiler distills each source into summary/concepts/entities. Required.
	Compiler Compiler
}

func (s *CompileStage) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// compiledSource holds the per-source extraction result, keyed by the source
// slug, used to build the index and log after all sources are processed.
type compiledSource struct {
	slug     string
	summary  string
	concepts []string
	entities []string
}

// Run compiles every wiki/sources/<slug>.md into derived artifacts. It is
// idempotent: when every source already has a valid summary artifact and the
// index/log already exist, it rewrites nothing new and returns Skipped.
func (s *CompileStage) Run(ctx context.Context, vault string, attempt int) (StageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.Compiler == nil {
		return StageResult{}, fmt.Errorf("compile: Compiler not wired")
	}
	if err := ctx.Err(); err != nil {
		return StageResult{}, err
	}

	sourcesDir := filepath.Join(vault, "wiki", "sources")
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return StageResult{Stage: StageCompile, Attempt: attempt, Skipped: true, SkipReason: "no-sources-dir"}, nil
		}
		return StageResult{}, fmt.Errorf("compile: read sources dir: %w", err)
	}

	slugs := collectSourceSlugs(entries)
	if len(slugs) == 0 {
		return StageResult{Stage: StageCompile, Attempt: attempt, Skipped: true, SkipReason: "no-sources"}, nil
	}

	now := s.now()
	compiled, artifacts, err := s.compileSources(ctx, vault, sourcesDir, slugs, now, attempt)
	if err != nil {
		return StageResult{}, err
	}

	// Index + log are always (re)written deterministically from the full set
	// of compiled sources so they stay in sync even when individual source
	// pages were skipped. Their content is a pure function of the inputs, so
	// SafeAtomicWrite of identical bytes is a stable no-op on re-run.
	indexPath, indexWrote, err := s.writeIndex(ctx, vault, compiled)
	if err != nil {
		return StageResult{}, err
	}
	logPath, logWrote, err := s.writeLog(ctx, vault, compiled)
	if err != nil {
		return StageResult{}, err
	}

	result := StageResult{
		Stage:         StageCompile,
		Attempt:       attempt,
		ArtifactsPath: artifacts,
	}
	if len(artifacts) == 0 && !indexWrote && !logWrote {
		result.Skipped = true
		result.SkipReason = "all-sources-already-compiled"
	} else {
		result.ArtifactsPath = append(result.ArtifactsPath, indexPath, logPath)
	}
	return result, nil
}

// collectSourceSlugs returns the sorted slug list (filename without .md) for
// every regular .md file in wiki/sources/. Sorting makes the index/log
// deterministic across runs.
func collectSourceSlugs(entries []os.DirEntry) []string {
	slugs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(name, filepath.Ext(name)))
	}
	sort.Strings(slugs)
	return slugs
}

// compileSources distills each source and writes its summary/concept/entity
// pages. It always returns the full compiledSource set (so the index/log
// reflect every source, even skipped ones) plus the list of pages newly
// written this run.
func (s *CompileStage) compileSources(
	ctx context.Context,
	vault, sourcesDir string,
	slugs []string,
	now time.Time,
	attempt int,
) ([]compiledSource, []string, error) {
	compiled := make([]compiledSource, 0, len(slugs))
	var artifacts []string

	for _, slug := range slugs {
		// Per amendment A4: check ctx between sources so cancel aborts cleanly.
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		raw, err := os.ReadFile(filepath.Join(sourcesDir, slug+".md"))
		if err != nil {
			return nil, nil, fmt.Errorf("compile: read source %s: %w", slug, err)
		}
		body := stripFrontmatterBody(string(raw))

		summary, err := s.Compiler.Summarize(ctx, body)
		if err != nil {
			return nil, nil, fmt.Errorf("compile: summarize %s: %w", slug, err)
		}
		concepts, err := s.Compiler.Concepts(ctx, body)
		if err != nil {
			return nil, nil, fmt.Errorf("compile: concepts %s: %w", slug, err)
		}
		entities, err := s.Compiler.Entities(ctx, body)
		if err != nil {
			return nil, nil, fmt.Errorf("compile: entities %s: %w", slug, err)
		}
		sort.Strings(concepts)
		sort.Strings(entities)

		cs := compiledSource{slug: slug, summary: summary, concepts: concepts, entities: entities}
		compiled = append(compiled, cs)

		wrote, err := s.writeSourceArtifacts(ctx, vault, cs, now, attempt)
		if err != nil {
			return nil, nil, err
		}
		artifacts = append(artifacts, wrote...)
	}
	return compiled, artifacts, nil
}

// writeSourceArtifacts writes the summary page plus one page per concept and
// entity for a single source, skipping any destination that already holds a
// valid artifact (idempotency). Returns the paths newly written this run.
func (s *CompileStage) writeSourceArtifacts(
	ctx context.Context,
	vault string,
	cs compiledSource,
	now time.Time,
	attempt int,
) ([]string, error) {
	var wrote []string

	// Summary.
	summaryPath := filepath.Join(vault, "wiki", "summaries", cs.slug+".md")
	body := renderSummaryBody(cs, now, attempt)
	w, err := writeIfNew(vault, summaryPath, body)
	if err != nil {
		return nil, fmt.Errorf("compile: write summary %s: %w", cs.slug, err)
	}
	if w {
		wrote = append(wrote, summaryPath)
	}

	// One page per concept.
	for _, concept := range cs.concepts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cslug := pageSlug(concept)
		path := filepath.Join(vault, "wiki", "concepts", cslug+".md")
		cbody := renderConceptBody(concept, cs.slug, now, attempt)
		w, err := writeIfNew(vault, path, cbody)
		if err != nil {
			return nil, fmt.Errorf("compile: write concept %s: %w", cslug, err)
		}
		if w {
			wrote = append(wrote, path)
		}
	}

	// One page per entity.
	for _, entity := range cs.entities {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		eslug := pageSlug(entity)
		path := filepath.Join(vault, "wiki", "entities", eslug+".md")
		ebody := renderEntityBody(entity, cs.slug, now, attempt)
		w, err := writeIfNew(vault, path, ebody)
		if err != nil {
			return nil, fmt.Errorf("compile: write entity %s: %w", eslug, err)
		}
		if w {
			wrote = append(wrote, path)
		}
	}
	return wrote, nil
}

// writeIfNew writes contents through SafeAtomicWrite unless dest already holds
// a valid artifact (frontmatter with an attempt key). Returns whether a write
// happened. This is the per-page idempotency gate.
func writeIfNew(vault, dest, contents string) (bool, error) {
	if hasValidArtifact(dest) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := SafeAtomicWrite(vault, dest, []byte(contents), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// writeAlways writes contents through SafeAtomicWrite, but only commits when
// the on-disk bytes differ, so identical re-renders are stable no-ops. Returns
// whether a write happened. Used for index/log, whose content is a pure
// function of the input set (so overwrite-with-identical is safe).
func writeAlways(vault, dest, contents string) (bool, error) {
	if existing, err := os.ReadFile(dest); err == nil && string(existing) == contents {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := SafeAtomicWrite(vault, dest, []byte(contents), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// writeIndex (re)writes wiki/index.md linking every generated page via
// wikilinks. Content is deterministic in the sorted compiled set.
func (s *CompileStage) writeIndex(ctx context.Context, vault string, compiled []compiledSource) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	dest := filepath.Join(vault, "wiki", "index.md")
	body := renderIndexBody(compiled)
	wrote, err := writeAlways(vault, dest, body)
	if err != nil {
		return "", false, fmt.Errorf("compile: write index: %w", err)
	}
	return dest, wrote, nil
}

// writeLog (re)writes wiki/log.md recording which sources were compiled. The
// content is a PURE function of the compiled set (no timestamp/attempt), so a
// no-op recompile is byte-stable — re-running must NOT rewrite the log.
func (s *CompileStage) writeLog(ctx context.Context, vault string, compiled []compiledSource) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	dest := filepath.Join(vault, "wiki", "log.md")
	body := renderLogBody(compiled)
	wrote, err := writeAlways(vault, dest, body)
	if err != nil {
		return "", false, fmt.Errorf("compile: write log: %w", err)
	}
	return dest, wrote, nil
}

// stripFrontmatterBody returns the body of an ingested source page with its
// leading YAML frontmatter block removed, so the compiler operates on the
// distilled markdown rather than the metadata header.
func stripFrontmatterBody(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	rest := text[len("---\n"):]
	if idx := strings.Index(rest, "\n---\n"); idx >= 0 {
		return strings.TrimLeft(rest[idx+len("\n---\n"):], "\n")
	}
	return text
}

// ---------------------------------------------------------------------------
// renderers
// ---------------------------------------------------------------------------

func renderSummaryBody(cs compiledSource, now time.Time, attempt int) string {
	header := renderFrontmatter(stageFrontmatter{
		Type:    "summary",
		Stage:   StageCompile,
		Source:  "wiki/sources/" + cs.slug + ".md",
		Created: now,
		Attempt: attempt,
	})
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n# Summary: %s\n\n", cs.slug)
	summary := cs.summary
	if summary == "" {
		summary = "_(no summary extracted)_"
	}
	fmt.Fprintf(&b, "%s\n\n", summary)
	fmt.Fprintf(&b, "Source: [[sources/%s]]\n", cs.slug)
	if len(cs.concepts) > 0 {
		b.WriteString("\n## Concepts\n\n")
		for _, c := range cs.concepts {
			fmt.Fprintf(&b, "- [[concepts/%s]]\n", pageSlug(c))
		}
	}
	if len(cs.entities) > 0 {
		b.WriteString("\n## Entities\n\n")
		for _, e := range cs.entities {
			fmt.Fprintf(&b, "- [[entities/%s]]\n", pageSlug(e))
		}
	}
	return b.String()
}

func renderConceptBody(concept, sourceSlug string, now time.Time, attempt int) string {
	header := renderFrontmatter(stageFrontmatter{
		Type:    "concept",
		Stage:   StageCompile,
		Source:  "wiki/sources/" + sourceSlug + ".md",
		Created: now,
		Attempt: attempt,
	})
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n# %s\n\n", concept)
	fmt.Fprintf(&b, "Concept extracted from [[sources/%s]].\n", sourceSlug)
	return b.String()
}

func renderEntityBody(entity, sourceSlug string, now time.Time, attempt int) string {
	header := renderFrontmatter(stageFrontmatter{
		Type:    "entity",
		Stage:   StageCompile,
		Source:  "wiki/sources/" + sourceSlug + ".md",
		Created: now,
		Attempt: attempt,
	})
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n# %s\n\n", entity)
	fmt.Fprintf(&b, "Entity extracted from [[sources/%s]].\n", sourceSlug)
	return b.String()
}

// renderIndexBody builds the wikilink index over the full compiled set. Pages
// are deduplicated across sources (a concept/entity mentioned by two sources
// is one page, linked once) and emitted in sorted order.
func renderIndexBody(compiled []compiledSource) string {
	summarySlugs := make([]string, 0, len(compiled))
	conceptSet := map[string]bool{}
	entitySet := map[string]bool{}
	for _, cs := range compiled {
		summarySlugs = append(summarySlugs, cs.slug)
		for _, c := range cs.concepts {
			conceptSet[pageSlug(c)] = true
		}
		for _, e := range cs.entities {
			entitySet[pageSlug(e)] = true
		}
	}
	sort.Strings(summarySlugs)
	concepts := sortedSetKeys(conceptSet)
	entities := sortedSetKeys(entitySet)

	var b strings.Builder
	b.WriteString("# Wiki Index\n\n")
	b.WriteString("Generated by the OpenKB compile stage. Pages compiled from sources land here.\n\n")

	b.WriteString("## Summaries\n\n")
	if len(summarySlugs) == 0 {
		b.WriteString("_(none)_\n")
	}
	for _, slug := range summarySlugs {
		fmt.Fprintf(&b, "- [[summaries/%s]]\n", slug)
	}

	b.WriteString("\n## Concepts\n\n")
	if len(concepts) == 0 {
		b.WriteString("_(none)_\n")
	}
	for _, slug := range concepts {
		fmt.Fprintf(&b, "- [[concepts/%s]]\n", slug)
	}

	b.WriteString("\n## Entities\n\n")
	if len(entities) == 0 {
		b.WriteString("_(none)_\n")
	}
	for _, slug := range entities {
		fmt.Fprintf(&b, "- [[entities/%s]]\n", slug)
	}
	return b.String()
}

// renderLogBody builds the compile log: a deterministic manifest of which
// sources were compiled and how many concepts/entities each yielded. The body
// is a PURE function of the compiled set — no wall-clock timestamp and no
// attempt counter — so a no-op recompile of unchanged sources is byte-stable
// (the idempotency contract). A wall-clock timestamp here was the bug that made
// production recompiles rewrite log.md every run.
func renderLogBody(compiled []compiledSource) string {
	var b strings.Builder
	b.WriteString("# Wiki Log\n\n")
	b.WriteString("Deterministic manifest of the compiled source set (idempotent: a no-op\nrecompile is byte-stable).\n\n")
	if len(compiled) == 0 {
		b.WriteString("_(no sources compiled)_\n")
		return b.String()
	}
	for _, cs := range compiled {
		fmt.Fprintf(&b, "- compiled `%s`: %d concept(s), %d entity(ies)\n",
			cs.slug, len(cs.concepts), len(cs.entities))
	}
	return b.String()
}

// sortedSetKeys returns the keys of set in sorted order.
func sortedSetKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
