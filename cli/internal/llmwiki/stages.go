package llmwiki

// Per-stage handler implementations for the Karpathy LLM-Wiki loop.
//
// Each handler implements StageHandler (executor.go) and respects three
// non-negotiable contracts:
//
//  1. SCOPE GUARD (amendment C5): every write goes through SafeAtomicWrite
//     so out-of-scope paths fail fast with *WriteScopeError.
//
//  2. ATOMIC WRITE (amendment A3): tmp+fsync+rename so a partial write never
//     leaves a corrupted artifact visible to readers. Resume after crash is
//     a frontmatter check on the destination file.
//
//  3. CTX PLUMBING (amendment A4): handlers check ctx.Err() before each new
//     write so cancellation aborts mid-batch without leaving torn state.
//
// The stage logic itself (NLP / extraction quality) is intentionally a stub
// in this issue. The contracts are what we lock in here. Future issues
// replace the stub bodies with real forge / harvest / knowledge calls.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/knowledge"
)

// HarvestPromoter is the local interface PromoteStage depends on. The real
// implementation wraps cli/internal/harvest.Promote; tests inject a mock.
type HarvestPromoter interface {
	// Promote moves mature wiki pages from sourceDir into authored content
	// rooted at destDir. Returns the number of files promoted. Already
	// idempotent post-TB-01 (skips destinations that already exist).
	Promote(sourceDir, destDir string, dryRun bool) (int, error)
}

// PromoteRequest is the JSON payload that PromoteStage looks for in
// vault/wiki/.promote-pending.json. Future issues populate it; for now the
// handler tolerates a missing file (Skipped=true) so the loop can run in
// vaults that have no pending promotions.
type PromoteRequest struct {
	SourceDir string `json:"source_dir"`
	DestDir   string `json:"dest_dir"`
}

// stageFrontmatter is the small frontmatter header we attach to every artifact
// the handlers create. Tests use these field names to validate idempotency
// (the "attempt" field gates the skip decision on re-run).
type stageFrontmatter struct {
	Type     string
	Source   string
	Stage    Stage
	Attempt  int
	Created  time.Time
	QueryKey string // QUERY only — the deterministic slug seed.
	Date     string // LINT only — YYYY-MM-DD.
}

// renderFrontmatter renders the header block as YAML between --- markers.
// Stable key order so atomic writes produce byte-deterministic output.
func renderFrontmatter(fm stageFrontmatter) string {
	var b strings.Builder
	b.WriteString("---\n")
	if fm.Type != "" {
		fmt.Fprintf(&b, "type: %s\n", fm.Type)
	}
	if fm.Stage != "" {
		fmt.Fprintf(&b, "stage: %s\n", fm.Stage)
	}
	if fm.Source != "" {
		fmt.Fprintf(&b, "source: %s\n", fm.Source)
	}
	if !fm.Created.IsZero() {
		fmt.Fprintf(&b, "created: %s\n", fm.Created.UTC().Format(time.RFC3339))
	}
	if fm.QueryKey != "" {
		fmt.Fprintf(&b, "query_key: %s\n", fm.QueryKey)
	}
	if fm.Date != "" {
		fmt.Fprintf(&b, "date: %s\n", fm.Date)
	}
	fmt.Fprintf(&b, "attempt: %d\n", fm.Attempt)
	b.WriteString("---\n")
	return b.String()
}

// hasValidArtifact reports whether path exists, parses as frontmatter, and
// carries a numeric "attempt" key. This is the per-stage idempotency probe:
// if true, the prior tick reached its atomic-commit point and we should skip.
func hasValidArtifact(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	fm := knowledge.ParseFrontmatter(string(data))
	if fm == nil {
		return false
	}
	if _, ok := fm["attempt"]; !ok {
		return false
	}
	return true
}

// slugify returns a filesystem-safe slug for the given input: lowercase
// alphanumerics + hyphens, collapsed, capped at 80 chars.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = out[:80]
	}
	if out == "" {
		out = "untitled"
	}
	return out
}

// ----------------------------------------------------------------------------
// IngestStage
// ----------------------------------------------------------------------------

// IngestStage walks vault/raw/ for unprocessed sources and renders a
// distilled wiki/sources/<slug>.md per file. The actual NLP distillation is
// stubbed (placeholder body); real forge.RunMinePass + entity/concept
// extraction wiring lands in a follow-up issue. The contracts (scope guard,
// atomic write, idempotency, ctx plumbing) are real today.
type IngestStage struct {
	// Now is injected for deterministic frontmatter timestamps in tests.
	Now func() time.Time
}

// Run scans vault/raw/ and emits wiki/sources/<slug>.md for each new entry.
func (s *IngestStage) Run(ctx context.Context, vault string, attempt int) (StageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rawDir := filepath.Join(vault, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		if os.IsNotExist(err) {
			return StageResult{Stage: StageIngest, Attempt: attempt, Skipped: true, SkipReason: "no-raw-dir"}, nil
		}
		return StageResult{}, fmt.Errorf("ingest: read raw dir: %w", err)
	}

	sourcesDir := filepath.Join(vault, "wiki", "sources")
	if err := os.MkdirAll(sourcesDir, 0o755); err != nil {
		return StageResult{}, fmt.Errorf("ingest: mkdir sources: %w", err)
	}

	now := s.now()
	names := collectIngestCandidates(entries)
	artifacts, skippedAll, err := s.ingestCandidates(ctx, vault, rawDir, sourcesDir, names, now, attempt)
	if err != nil {
		return StageResult{}, err
	}

	result := StageResult{
		Stage:         StageIngest,
		Attempt:       attempt,
		ArtifactsPath: artifacts,
	}
	if len(artifacts) == 0 && skippedAll {
		result.Skipped = true
		result.SkipReason = "all-sources-already-ingested"
	}
	return result, nil
}

func (s *IngestStage) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// collectIngestCandidates returns the regular-file names from raw/ in sorted
// order. Sorting is important for the ctx-cancel test which expects
// deterministic ordering across runs.
func collectIngestCandidates(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// ingestCandidates loops over the candidate names and writes one
// wiki/sources/<slug>.md per ingestable file. Returns the artifacts written
// plus whether every candidate was skipped (used to populate the Skipped
// reason on a no-op tick).
func (s *IngestStage) ingestCandidates(
	ctx context.Context,
	vault, rawDir, sourcesDir string,
	names []string,
	now time.Time,
	attempt int,
) ([]string, bool, error) {
	var artifacts []string
	skippedAll := true
	for _, name := range names {
		// Per amendment A4: check ctx between writes so cancel aborts
		// cleanly without leaving torn state.
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if !isIngestableExt(name) {
			continue
		}
		rawPath := filepath.Join(rawDir, name)
		slug := slugify(strings.TrimSuffix(name, filepath.Ext(name)))
		dest := filepath.Join(sourcesDir, slug+".md")
		if hasValidArtifact(dest) {
			// Prior tick succeeded. Per amendment A3, skip-with-Skipped.
			continue
		}
		skippedAll = false
		body, err := buildIngestBody(rawPath, now, attempt)
		if err != nil {
			return nil, false, fmt.Errorf("ingest: build body for %s: %w", name, err)
		}
		if err := SafeAtomicWrite(vault, dest, []byte(body), 0o644); err != nil {
			return nil, false, fmt.Errorf("ingest: write %s: %w", dest, err)
		}
		artifacts = append(artifacts, dest)
	}
	return artifacts, skippedAll, nil
}

// isIngestableExt reports whether the file extension indicates an
// ingest-eligible source. Keep the list small and explicit so binary blobs
// in raw/ never accidentally end up rendered as a wiki source.
func isIngestableExt(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt")
}

// buildIngestBody produces the placeholder distillation body for a raw file.
// Future issues replace the body content with real NLP extraction; the
// frontmatter contract stays stable.
func buildIngestBody(rawPath string, now time.Time, attempt int) (string, error) {
	header := renderFrontmatter(stageFrontmatter{
		Type:    "source",
		Stage:   StageIngest,
		Source:  rawPath,
		Created: now,
		Attempt: attempt,
	})
	title := strings.TrimSuffix(filepath.Base(rawPath), filepath.Ext(rawPath))
	body := fmt.Sprintf("%s\n# %s\n\n_Distilled placeholder — full extraction pending._\n\n- raw: `%s`\n",
		header, title, rawPath)
	return body, nil
}

// ----------------------------------------------------------------------------
// QueryStage
// ----------------------------------------------------------------------------

// QueryRequest is the JSON payload QueryStage looks for in
// vault/wiki/.query-pending.json. The slug is derived deterministically from
// the query text so re-runs reuse the same destination filename.
type QueryRequest struct {
	Query string `json:"query"`
}

// QueryStage answers a question against the wiki and writes a synthesis page.
// Stub body for now; the contract (atomic write, idempotency by slug, scope
// guard) is real.
type QueryStage struct {
	Now func() time.Time
}

func (s *QueryStage) Run(ctx context.Context, vault string, attempt int) (StageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pending := filepath.Join(vault, "wiki", ".query-pending.json")
	data, err := os.ReadFile(pending)
	if err != nil {
		if os.IsNotExist(err) {
			return StageResult{Stage: StageQuery, Attempt: attempt, Skipped: true, SkipReason: "no-pending-query"}, nil
		}
		return StageResult{}, fmt.Errorf("query: read pending: %w", err)
	}
	query := strings.TrimSpace(string(data))
	if query == "" {
		return StageResult{Stage: StageQuery, Attempt: attempt, Skipped: true, SkipReason: "empty-query"}, nil
	}
	if err := ctx.Err(); err != nil {
		return StageResult{}, err
	}

	slug := slugify(query)
	synthDir := filepath.Join(vault, "wiki", "synthesis")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		return StageResult{}, fmt.Errorf("query: mkdir synthesis: %w", err)
	}
	dest := filepath.Join(synthDir, "query-"+slug+".md")

	if hasValidArtifact(dest) {
		return StageResult{
			Stage:      StageQuery,
			Attempt:    attempt,
			Skipped:    true,
			SkipReason: "query-already-answered",
		}, nil
	}

	header := renderFrontmatter(stageFrontmatter{
		Type:     "synthesis",
		Stage:    StageQuery,
		Created:  s.now(),
		QueryKey: slug,
		Attempt:  attempt,
	})
	body := fmt.Sprintf("%s\n# Query: %s\n\n_Synthesis placeholder — answer pending._\n", header, query)
	if err := SafeAtomicWrite(vault, dest, []byte(body), 0o644); err != nil {
		return StageResult{}, fmt.Errorf("query: write %s: %w", dest, err)
	}
	return StageResult{
		Stage:         StageQuery,
		Attempt:       attempt,
		ArtifactsPath: []string{dest},
	}, nil
}

func (s *QueryStage) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ----------------------------------------------------------------------------
// LintStage
// ----------------------------------------------------------------------------

// LintStage walks the wiki tree looking for orphans / stale / contradictions
// and writes wiki/synthesis/lint-YYYY-MM-DD.md. Per amendment A3: overwrite
// IS the contract; re-run is always safe and the attempt counter is
// recorded in frontmatter.
type LintStage struct {
	Now func() time.Time
}

func (s *LintStage) Run(ctx context.Context, vault string, attempt int) (StageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return StageResult{}, err
	}

	now := s.now()
	date := now.UTC().Format("2006-01-02")
	synthDir := filepath.Join(vault, "wiki", "synthesis")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		return StageResult{}, fmt.Errorf("lint: mkdir synthesis: %w", err)
	}
	dest := filepath.Join(synthDir, "lint-"+date+".md")

	findings, err := collectLintFindings(vault)
	if err != nil {
		return StageResult{}, fmt.Errorf("lint: collect findings: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return StageResult{}, err
	}

	header := renderFrontmatter(stageFrontmatter{
		Type:    "lint",
		Stage:   StageLint,
		Created: now,
		Date:    date,
		Attempt: attempt,
	})
	body := renderLintBody(header, date, findings)

	if err := SafeAtomicWrite(vault, dest, []byte(body), 0o644); err != nil {
		return StageResult{}, fmt.Errorf("lint: write %s: %w", dest, err)
	}
	return StageResult{
		Stage:         StageLint,
		Attempt:       attempt,
		ArtifactsPath: []string{dest},
	}, nil
}

func (s *LintStage) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// collectLintFindings is a stub that returns the count of files in each
// allowed wiki subdir. Future issues replace this with real
// orphan/stale/contradiction detection.
func collectLintFindings(vault string) ([]string, error) {
	subdirs := []string{"sources", "entities", "concepts", "synthesis"}
	findings := make([]string, 0, len(subdirs))
	for _, sub := range subdirs {
		dir := filepath.Join(vault, "wiki", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				findings = append(findings, fmt.Sprintf("- `%s/`: missing", sub))
				continue
			}
			return nil, err
		}
		count := 0
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				count++
			}
		}
		findings = append(findings, fmt.Sprintf("- `%s/`: %d files", sub, count))
	}
	return findings, nil
}

func renderLintBody(header, date string, findings []string) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n# Lint Report — ")
	b.WriteString(date)
	b.WriteString("\n\n_Stub findings — full lint logic pending._\n\n## Counts\n\n")
	for _, f := range findings {
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}

// ----------------------------------------------------------------------------
// PromoteStage
// ----------------------------------------------------------------------------

// PromoteStage delegates to harvest.Promote (already idempotent post-TB-01)
// to move mature wiki pages into authored content. The stage reads
// vault/wiki/.promote-pending.json for the source/dest pair; absent file
// → Skipped (no pending promotions).
type PromoteStage struct {
	Harvest HarvestPromoter
	DryRun  bool
}

func (s *PromoteStage) Run(ctx context.Context, vault string, attempt int) (StageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return StageResult{}, err
	}
	if s.Harvest == nil {
		return StageResult{}, fmt.Errorf("promote: HarvestPromoter not wired")
	}

	pending := filepath.Join(vault, "wiki", ".promote-pending.json")
	data, err := os.ReadFile(pending)
	if err != nil {
		if os.IsNotExist(err) {
			return StageResult{
				Stage:      StagePromote,
				Attempt:    attempt,
				Skipped:    true,
				SkipReason: "no-pending-promotions",
			}, nil
		}
		return StageResult{}, fmt.Errorf("promote: read pending: %w", err)
	}

	req, err := parsePromoteRequest(data)
	if err != nil {
		return StageResult{}, fmt.Errorf("promote: parse pending: %w", err)
	}
	if req.SourceDir == "" || req.DestDir == "" {
		return StageResult{
			Stage:      StagePromote,
			Attempt:    attempt,
			Skipped:    true,
			SkipReason: "incomplete-promote-request",
		}, nil
	}

	count, err := s.Harvest.Promote(req.SourceDir, req.DestDir, s.DryRun)
	if err != nil {
		return StageResult{}, fmt.Errorf("promote: harvest: %w", err)
	}
	return StageResult{
		Stage:         StagePromote,
		Attempt:       attempt,
		ArtifactsPath: []string{fmt.Sprintf("%s -> %s (count=%d)", req.SourceDir, req.DestDir, count)},
	}, nil
}

// parsePromoteRequest tolerates the small JSON shape we expect today. We
// avoid a full encoding/json dependency in this hot path by parsing the two
// known fields directly; this also keeps the parser predictable for tests.
func parsePromoteRequest(data []byte) (PromoteRequest, error) {
	var req PromoteRequest
	text := strings.TrimSpace(string(data))
	if text == "" {
		return req, nil
	}
	// Use the same yaml-style parser knowledge.ParseFrontmatter relies on:
	// JSON is valid YAML, and we already depend on yaml.v3 transitively.
	// For simplicity here, we look for "source_dir" and "dest_dir" tokens.
	// This is intentionally simple — a future issue swaps in encoding/json
	// once the request shape stabilizes.
	for _, line := range strings.Split(text, ",") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "{}\"")
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.Trim(k, "\""))
		v = strings.TrimSpace(strings.Trim(v, "\""))
		switch k {
		case "source_dir":
			req.SourceDir = v
		case "dest_dir":
			req.DestDir = v
		}
	}
	return req, nil
}
