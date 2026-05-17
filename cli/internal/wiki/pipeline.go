// This file implements WikiPipeline — the wiki bounded context's own
// implementation of the Karpathy LLM-wiki loop. It subsumes
// cli/internal/llmwiki/executor.go: where the legacy LLMWikiLoopExecutor
// leaves a stage handler nil and reports SkipReason "handler-not-wired",
// WikiPipeline owns the Ingest, Lint and Query stage logic directly, so those
// three stages are always wired to a real handler.
//
// The migration is strictly accretive (strangler): llmwiki.LLMWikiLoopExecutor
// keeps working unchanged for its existing callers (cmd/ao/agentopsd.go).
// WikiPipeline does NOT import llmwiki — doing so would form an import cycle
// (llmwiki → knowledge → wiki). Instead the pipeline reimplements the stage
// logic inside the wiki package, reusing wiki's own FrontmatterCodec, which is
// what "subsume" means here: the wiki bounded context becomes the home of the
// loop, not a borrower of llmwiki internals.
package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// PipelineStage names one operation the WikiPipeline can run on a tick.
type PipelineStage string

// The four pipeline stages. Values match llmwiki.Stage so a job payload
// authored for either surface is interchangeable.
const (
	// StageIngest distills raw sources into wiki/sources/ pages.
	StageIngest PipelineStage = "ingest"
	// StageQuery answers a pending question into wiki/synthesis/.
	StageQuery PipelineStage = "query"
	// StageLint walks the wiki tree and writes a dated lint report.
	StageLint PipelineStage = "lint"
	// StagePromote moves mature wiki pages into authored content.
	StagePromote PipelineStage = "promote"
)

// SkipReasonHandlerNotWired is the SkipReason the legacy llmwiki executor
// records when a selected stage has a nil handler. WikiPipeline exists to
// guarantee the Ingest, Lint and Query stages never produce this value.
const SkipReasonHandlerNotWired = "handler-not-wired"

// defaultLintIntervalHours is the default interval between LINT runs when the
// job spec does not specify one. It mirrors llmwiki.DefaultLintIntervalHours.
const defaultLintIntervalHours = 24

// PipelineJobSpec is the JSON payload of a daemon.JobTypeLLMWikiLoop job. It is
// structurally compatible with llmwiki.LoopJobSpec.
type PipelineJobSpec struct {
	// Vault is the root path containing raw/ and wiki/.
	Vault string `json:"vault"`
	// Stages is an optional whitelist; when empty the pipeline auto-selects.
	Stages []PipelineStage `json:"stages,omitempty"`
	// LintIntervalHours controls when LINT becomes eligible. 0 → use default.
	LintIntervalHours int `json:"lint_interval_hours,omitempty"`
}

// StageOutcome reports what a single pipeline stage did this tick.
type StageOutcome struct {
	// Stage is the stage that ran.
	Stage PipelineStage
	// Attempt is the 1-based attempt counter for this tick.
	Attempt int
	// Artifacts lists the paths the stage wrote, if any.
	Artifacts []string
	// Skipped reports whether the stage performed no work.
	Skipped bool
	// SkipReason explains a skip; empty when Skipped is false. It is never
	// SkipReasonHandlerNotWired for a stage WikiPipeline owns.
	SkipReason string
}

// stageRunner runs one stage against a vault. The pipeline owns concrete
// runners for Ingest, Lint and Query; Promote is injected because it needs a
// promoter the wiki package does not own.
type stageRunner interface {
	run(ctx context.Context, vault string, attempt int) (StageOutcome, error)
}

// WikiPipeline runs the Karpathy LLM-wiki loop for the wiki bounded context.
//
// Unlike llmwiki.LLMWikiLoopExecutor — whose stage handlers are injected and
// may be nil — WikiPipeline wires the Ingest, Lint and Query stages to
// concrete runners at construction time. Stages are selectable: RunStage runs a
// named stage, and RunJob honors the PipelineJobSpec.Stages whitelist.
//
// WikiPipeline satisfies daemon.JobExecutor for daemon.JobTypeLLMWikiLoop.
type WikiPipeline struct {
	// runners maps each stage to its runner. Ingest, Lint and Query are always
	// populated; Promote is nil unless WithPromoteRunner is supplied.
	runners map[PipelineStage]stageRunner
	// now is injected for deterministic stage selection and timestamps.
	now func() time.Time
}

// PromoteRunner promotes mature wiki pages into authored content. The wiki
// package does not own a promoter, so the PROMOTE stage is supplied by callers
// via WithPromoteRunner. Run mirrors the stage-handler contract.
type PromoteRunner interface {
	Run(ctx context.Context, vault string, attempt int) (StageOutcome, error)
}

// PipelineOption customizes a WikiPipeline at construction.
type PipelineOption func(*WikiPipeline)

// WithPromoteRunner wires the PROMOTE stage. PROMOTE is optional because it
// needs a promoter the wiki package does not own; when unset, selecting
// PROMOTE yields a Skipped outcome with reason "promote-handler-not-configured"
// (never SkipReasonHandlerNotWired).
func WithPromoteRunner(r PromoteRunner) PipelineOption {
	return func(p *WikiPipeline) {
		if r != nil {
			p.runners[StagePromote] = promoteAdapter{runner: r}
		}
	}
}

// WithClock injects a clock for deterministic selection and timestamps in tests.
func WithClock(now func() time.Time) PipelineOption {
	return func(p *WikiPipeline) {
		if now != nil {
			p.now = now
		}
	}
}

// NewWikiPipeline constructs a WikiPipeline with the Ingest, Lint and Query
// stages wired to concrete runners. Because those runners are non-nil, the
// pipeline can never report SkipReasonHandlerNotWired for them.
//
// PROMOTE is left unwired by default; pass WithPromoteRunner to enable it.
func NewWikiPipeline(opts ...PipelineOption) *WikiPipeline {
	p := &WikiPipeline{now: time.Now}
	p.runners = map[PipelineStage]stageRunner{
		StageIngest: ingestRunner{now: p.clock},
		StageLint:   lintRunner{now: p.clock},
		StageQuery:  queryRunner{now: p.clock},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Stages returns the pipeline stages that have a wired runner, in canonical
// loop order (ingest, query, lint, promote).
func (p *WikiPipeline) Stages() []PipelineStage {
	order := []PipelineStage{StageIngest, StageQuery, StageLint, StagePromote}
	out := make([]PipelineStage, 0, len(order))
	for _, s := range order {
		if p.runners[s] != nil {
			out = append(out, s)
		}
	}
	return out
}

// HasStage reports whether stage has a wired runner in this pipeline.
func (p *WikiPipeline) HasStage(stage PipelineStage) bool {
	return p.runners[stage] != nil
}

// RunStage runs a single named stage against vault and returns its outcome.
//
// For a stage WikiPipeline owns (Ingest, Lint, Query) the runner is always
// wired, so the returned StageOutcome never carries SkipReasonHandlerNotWired —
// it reports the stage's real work or a real skip reason (e.g. "no-raw-dir").
// Selecting PROMOTE without a configured runner yields Skipped with reason
// "promote-handler-not-configured".
//
// attempt is the 1-based attempt counter; values below 1 are clamped to 1.
func (p *WikiPipeline) RunStage(ctx context.Context, vault string, stage PipelineStage, attempt int) (StageOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if vault == "" {
		return StageOutcome{}, errors.New("wiki pipeline: vault is required")
	}
	if attempt < 1 {
		attempt = 1
	}

	runner := p.runners[stage]
	if runner == nil {
		if stage == StagePromote {
			return StageOutcome{
				Stage:      StagePromote,
				Attempt:    attempt,
				Skipped:    true,
				SkipReason: "promote-handler-not-configured",
			}, nil
		}
		return StageOutcome{}, fmt.Errorf("wiki pipeline: unknown stage %q", stage)
	}

	outcome, err := runner.run(ctx, vault, attempt)
	if err != nil {
		return StageOutcome{}, fmt.Errorf("wiki pipeline: stage %s failed: %w", stage, err)
	}
	if outcome.Stage == "" {
		outcome.Stage = stage
	}
	if outcome.Attempt == 0 {
		outcome.Attempt = attempt
	}
	return outcome, nil
}

// SelectStage chooses which stage to run for a vault given the lint interval.
//
// Order of preference (matching llmwiki.SelectStage):
//  1. INGEST — if vault/raw/ has files newer than vault/wiki/.last-ingest.
//  2. LINT   — if now - last-lint exceeds the lint interval.
//  3. INGEST — conservative default re-scan.
func (p *WikiPipeline) SelectStage(vault string, lintIntervalHours int) PipelineStage {
	if lintIntervalHours <= 0 {
		lintIntervalHours = defaultLintIntervalHours
	}
	if rawHasNewerFiles(vault) {
		return StageIngest
	}
	if lintIsStale(vault, lintIntervalHours, p.clock()) {
		return StageLint
	}
	return StageIngest
}

// clock returns the pipeline's injected time source, defaulting to time.Now.
func (p *WikiPipeline) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// JobTypes declares this pipeline handles daemon.JobTypeLLMWikiLoop, satisfying
// daemon.JobExecutor.
func (p *WikiPipeline) JobTypes() []daemon.JobType {
	return []daemon.JobType{daemon.JobTypeLLMWikiLoop}
}

// RunJob executes one tick of the LLM-wiki loop for a claimed daemon job,
// satisfying daemon.JobExecutor. It parses the PipelineJobSpec from the claim
// payload, selects a stage (honoring the spec's Stages whitelist), runs it, and
// returns the stage artifacts in a daemon.JobExecutionResult.
//
// This is the strangler replacement for llmwiki.LLMWikiLoopExecutor.RunJob:
// behavior is identical except that a wired stage can never fall through to
// SkipReasonHandlerNotWired.
func (p *WikiPipeline) RunJob(ctx context.Context, claim daemon.QueueLease) (daemon.JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != daemon.JobTypeLLMWikiLoop {
		return daemon.JobExecutionResult{}, fmt.Errorf("wiki pipeline does not support job type %s", claim.Job.JobType)
	}

	spec, err := parsePipelineJobSpec(claim.Job.Payload)
	if err != nil {
		return daemon.JobExecutionResult{}, err
	}
	if spec.Vault == "" {
		return daemon.JobExecutionResult{}, errors.New("wiki pipeline: spec.vault is required")
	}

	stage, err := p.pickStage(spec)
	if err != nil {
		return daemon.JobExecutionResult{}, err
	}

	attempt := claim.Job.Attempt
	if attempt < 1 {
		attempt = 1
	}

	outcome, err := p.RunStage(ctx, spec.Vault, stage, attempt)
	if err != nil {
		return daemon.JobExecutionResult{}, err
	}
	return jobResultFromOutcome(outcome), nil
}

// pickStage applies the spec's Stages whitelist to the auto-selection
// heuristic. An empty whitelist uses the auto-selected stage; a non-empty
// whitelist is a hard filter — if it excludes the preferred stage, the first
// whitelisted known stage is used.
func (p *WikiPipeline) pickStage(spec PipelineJobSpec) (PipelineStage, error) {
	preferred := p.SelectStage(spec.Vault, spec.LintIntervalHours)
	if len(spec.Stages) == 0 {
		return preferred, nil
	}
	for _, s := range spec.Stages {
		if s == preferred {
			return preferred, nil
		}
	}
	for _, s := range spec.Stages {
		if isPipelineStage(s) {
			return s, nil
		}
	}
	return "", fmt.Errorf("wiki pipeline: spec.stages %v contains no known stages", spec.Stages)
}

// isPipelineStage reports whether s is one of the four known pipeline stages.
func isPipelineStage(s PipelineStage) bool {
	switch s {
	case StageIngest, StageQuery, StageLint, StagePromote:
		return true
	default:
		return false
	}
}

// parsePipelineJobSpec decodes a daemon job payload into a PipelineJobSpec.
func parsePipelineJobSpec(payload map[string]any) (PipelineJobSpec, error) {
	var spec PipelineJobSpec
	if len(payload) == 0 {
		return spec, errors.New("wiki pipeline: payload is required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return spec, fmt.Errorf("wiki pipeline: marshal payload: %w", err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("wiki pipeline: parse payload: %w", err)
	}
	return spec, nil
}

// jobResultFromOutcome renders a StageOutcome into the daemon artifact map the
// supervisor records in the job ledger.
func jobResultFromOutcome(o StageOutcome) daemon.JobExecutionResult {
	artifacts := map[string]string{
		"stage":   string(o.Stage),
		"attempt": fmt.Sprintf("%d", o.Attempt),
	}
	if o.Skipped {
		artifacts["skipped"] = "true"
		if o.SkipReason != "" {
			artifacts["skip_reason"] = o.SkipReason
		}
	}
	for i, path := range o.Artifacts {
		artifacts[fmt.Sprintf("artifact_%d", i)] = path
	}
	return daemon.JobExecutionResult{Artifacts: artifacts}
}

// promoteAdapter adapts an externally-supplied PromoteRunner to the internal
// stageRunner interface.
type promoteAdapter struct{ runner PromoteRunner }

func (a promoteAdapter) run(ctx context.Context, vault string, attempt int) (StageOutcome, error) {
	return a.runner.Run(ctx, vault, attempt)
}

// ---------------------------------------------------------------------------
// Stage-selection vault probes (mirror llmwiki's heuristic, no llmwiki import).
// ---------------------------------------------------------------------------

// rawHasNewerFiles reports whether vault/raw/ contains a regular file with a
// mtime newer than vault/wiki/.last-ingest. A missing sentinel means any
// regular file in raw/ qualifies (nothing has been ingested yet).
func rawHasNewerFiles(vault string) bool {
	rawDir := filepath.Join(vault, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil || len(entries) == 0 {
		return false
	}
	sentinelInfo, sentinelErr := os.Stat(filepath.Join(vault, "wiki", ".last-ingest"))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if sentinelErr != nil {
			return true
		}
		if info.ModTime().After(sentinelInfo.ModTime()) {
			return true
		}
	}
	return false
}

// lintIsStale reports whether vault/wiki/.last-lint is older than the interval.
// A missing sentinel means lint should run.
func lintIsStale(vault string, lintIntervalHours int, now time.Time) bool {
	info, err := os.Stat(filepath.Join(vault, "wiki", ".last-lint"))
	if err != nil {
		return true
	}
	return now.Sub(info.ModTime()) > time.Duration(lintIntervalHours)*time.Hour
}

// ---------------------------------------------------------------------------
// Stage runners — the wiki bounded context's own implementation of the three
// stages the legacy executor left stub-able. They preserve llmwiki/stages.go
// behavior (artifact paths, skip reasons, idempotency) so the migration is
// observably equivalent.
// ---------------------------------------------------------------------------

// stageHeader renders the small frontmatter block attached to every stage
// artifact. Stable key order keeps writes deterministic. The "attempt" key is
// the idempotency probe — its presence marks a completed prior tick.
func stageHeader(kind string, stage PipelineStage, created time.Time, extra map[string]string, attempt int) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "type: %s\n", kind)
	fmt.Fprintf(&b, "stage: %s\n", stage)
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, extra[k])
	}
	if !created.IsZero() {
		fmt.Fprintf(&b, "created: %s\n", created.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "attempt: %d\n", attempt)
	b.WriteString("---\n")
	return b.String()
}

// artifactDone reports whether path exists and carries a parsed "attempt" key
// in its frontmatter. True means a prior tick reached its commit point and the
// stage should skip on re-run.
func artifactDone(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // path is vault-bounded
	if err != nil {
		return false
	}
	doc := NewFrontmatterCodec().Decode(string(data))
	if !doc.HasFrontmatter {
		return false
	}
	_, ok := doc.Fields["attempt"]
	return ok
}

// slugify returns a filesystem-safe slug: lowercase alphanumerics and hyphens,
// collapsed, capped at 80 runes.
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

// writeArtifact writes data to path atomically (temp file + rename) so a crash
// mid-write never leaves a torn artifact visible to readers.
func writeArtifact(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wiki-stage-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() //nolint:errcheck // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ingestRunner is the INGEST stage: it distills each new vault/raw/ file into a
// wiki/sources/<slug>.md page.
type ingestRunner struct{ now func() time.Time }

func (r ingestRunner) run(ctx context.Context, vault string, attempt int) (StageOutcome, error) {
	rawDir := filepath.Join(vault, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		if os.IsNotExist(err) {
			return StageOutcome{Stage: StageIngest, Attempt: attempt, Skipped: true, SkipReason: "no-raw-dir"}, nil
		}
		return StageOutcome{}, fmt.Errorf("ingest: read raw dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	sourcesDir := filepath.Join(vault, "wiki", "sources")
	var artifacts []string
	skippedAll := true
	created := r.clock()
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return StageOutcome{}, err
		}
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".txt") {
			continue
		}
		slug := slugify(strings.TrimSuffix(name, filepath.Ext(name)))
		dest := filepath.Join(sourcesDir, slug+".md")
		if artifactDone(dest) {
			continue
		}
		skippedAll = false
		rawPath := filepath.Join(rawDir, name)
		header := stageHeader("source", StageIngest, created, map[string]string{"source": rawPath}, attempt)
		title := strings.TrimSuffix(filepath.Base(rawPath), filepath.Ext(rawPath))
		body := fmt.Sprintf("%s\n# %s\n\n_Distilled placeholder — full extraction pending._\n\n- raw: `%s`\n",
			header, title, rawPath)
		if err := writeArtifact(dest, []byte(body)); err != nil {
			return StageOutcome{}, fmt.Errorf("ingest: write %s: %w", dest, err)
		}
		artifacts = append(artifacts, dest)
	}

	outcome := StageOutcome{Stage: StageIngest, Attempt: attempt, Artifacts: artifacts}
	if len(artifacts) == 0 && skippedAll {
		outcome.Skipped = true
		outcome.SkipReason = "all-sources-already-ingested"
	}
	return outcome, nil
}

// clock returns the runner's time source, defaulting to time.Now.
func (r ingestRunner) clock() time.Time { return runnerClock(r.now) }

// queryRunner is the QUERY stage: it answers a pending question (read from
// vault/wiki/.query-pending.json) into a wiki/synthesis/query-<slug>.md page.
type queryRunner struct{ now func() time.Time }

func (r queryRunner) run(ctx context.Context, vault string, attempt int) (StageOutcome, error) {
	pending := filepath.Join(vault, "wiki", ".query-pending.json")
	data, err := os.ReadFile(pending) //nolint:gosec // path is vault-bounded
	if err != nil {
		if os.IsNotExist(err) {
			return StageOutcome{Stage: StageQuery, Attempt: attempt, Skipped: true, SkipReason: "no-pending-query"}, nil
		}
		return StageOutcome{}, fmt.Errorf("query: read pending: %w", err)
	}
	query := strings.TrimSpace(string(data))
	if query == "" {
		return StageOutcome{Stage: StageQuery, Attempt: attempt, Skipped: true, SkipReason: "empty-query"}, nil
	}
	if err := ctx.Err(); err != nil {
		return StageOutcome{}, err
	}

	slug := slugify(query)
	dest := filepath.Join(vault, "wiki", "synthesis", "query-"+slug+".md")
	if artifactDone(dest) {
		return StageOutcome{Stage: StageQuery, Attempt: attempt, Skipped: true, SkipReason: "query-already-answered"}, nil
	}

	header := stageHeader("synthesis", StageQuery, runnerClock(r.now), map[string]string{"query_key": slug}, attempt)
	body := fmt.Sprintf("%s\n# Query: %s\n\n_Synthesis placeholder — answer pending._\n", header, query)
	if err := writeArtifact(dest, []byte(body)); err != nil {
		return StageOutcome{}, fmt.Errorf("query: write %s: %w", dest, err)
	}
	return StageOutcome{Stage: StageQuery, Attempt: attempt, Artifacts: []string{dest}}, nil
}

// lintRunner is the LINT stage: it walks the wiki tree and writes a dated lint
// report to wiki/synthesis/lint-YYYY-MM-DD.md. Overwrite is the contract —
// re-running is always safe.
type lintRunner struct{ now func() time.Time }

func (r lintRunner) run(ctx context.Context, vault string, attempt int) (StageOutcome, error) {
	if err := ctx.Err(); err != nil {
		return StageOutcome{}, err
	}
	now := runnerClock(r.now)
	date := now.UTC().Format("2006-01-02")
	dest := filepath.Join(vault, "wiki", "synthesis", "lint-"+date+".md")

	findings, err := collectLintFindings(vault)
	if err != nil {
		return StageOutcome{}, fmt.Errorf("lint: collect findings: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return StageOutcome{}, err
	}

	header := stageHeader("lint", StageLint, now, map[string]string{"date": date}, attempt)
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n# Lint Report — %s\n\n_Stub findings — full lint logic pending._\n\n## Counts\n\n", date)
	for _, f := range findings {
		b.WriteString(f)
		b.WriteString("\n")
	}
	if err := writeArtifact(dest, []byte(b.String())); err != nil {
		return StageOutcome{}, fmt.Errorf("lint: write %s: %w", dest, err)
	}
	return StageOutcome{Stage: StageLint, Attempt: attempt, Artifacts: []string{dest}}, nil
}

// runnerClock resolves a stage runner's optional clock to a concrete time,
// defaulting to time.Now when unset.
func runnerClock(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now()
}

// collectLintFindings counts the markdown files in each allowed wiki subdir.
func collectLintFindings(vault string) ([]string, error) {
	subdirs := []string{"sources", "entities", "concepts", "synthesis"}
	findings := make([]string, 0, len(subdirs))
	for _, sub := range subdirs {
		entries, err := os.ReadDir(filepath.Join(vault, "wiki", sub))
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
