package wiki

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// pipelineFixedClock returns a clock function pinned to t.
func pipelineFixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestWikiPipeline_Stages is the acceptance test for soc-wiki.6: a
// JobTypeLLMWikiLoop with stage=lint run through WikiPipeline must NOT return
// SkipReason "handler-not-wired", and the Ingest/Lint/Query stages must be
// individually selectable with their real behavior.
func TestWikiPipeline_Stages(t *testing.T) {
	clock := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	t.Run("lint stage via RunJob is not handler-not-wired", func(t *testing.T) {
		vault := t.TempDir()
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		claim := daemon.QueueLease{Job: daemon.QueueJobState{
			JobType: daemon.JobTypeLLMWikiLoop,
			Attempt: 1,
			Payload: map[string]any{
				"vault":  vault,
				"stages": []any{string(StageLint)},
			},
		}}

		res, err := pipeline.RunJob(context.Background(), claim)
		if err != nil {
			t.Fatalf("RunJob returned error: %v", err)
		}
		if got := res.Artifacts["skip_reason"]; got == SkipReasonHandlerNotWired {
			t.Fatalf("lint stage returned SkipReason %q; want it wired", SkipReasonHandlerNotWired)
		}
		if got := res.Artifacts["stage"]; got != string(StageLint) {
			t.Fatalf("ran stage %q; want %q", got, StageLint)
		}
		if got := res.Artifacts["skipped"]; got == "true" {
			t.Fatalf("lint stage was skipped; want it to run and write a report")
		}
		// Lint always writes a report (overwrite-is-the-contract).
		reportPath := res.Artifacts["artifact_0"]
		wantSuffix := filepath.Join("wiki", "synthesis", "lint-2026-05-17.md")
		if !strings.HasSuffix(reportPath, wantSuffix) {
			t.Fatalf("lint artifact %q; want suffix %q", reportPath, wantSuffix)
		}
		if _, err := os.Stat(reportPath); err != nil {
			t.Fatalf("lint report not written: %v", err)
		}
	})

	t.Run("lint stage via RunStage writes a dated report", func(t *testing.T) {
		vault := t.TempDir()
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		outcome, err := pipeline.RunStage(context.Background(), vault, StageLint, 1)
		if err != nil {
			t.Fatalf("RunStage(lint) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("RunStage(lint) reported %q", SkipReasonHandlerNotWired)
		}
		if outcome.Skipped {
			t.Fatalf("lint stage skipped: %q", outcome.SkipReason)
		}
		if outcome.Stage != StageLint {
			t.Fatalf("outcome stage %q; want %q", outcome.Stage, StageLint)
		}
		if len(outcome.Artifacts) != 1 {
			t.Fatalf("lint wrote %d artifacts; want 1", len(outcome.Artifacts))
		}
	})

	t.Run("ingest stage wired: no raw dir yields no-raw-dir skip", func(t *testing.T) {
		vault := t.TempDir()
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		outcome, err := pipeline.RunStage(context.Background(), vault, StageIngest, 1)
		if err != nil {
			t.Fatalf("RunStage(ingest) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("ingest stage reported %q; want it wired", SkipReasonHandlerNotWired)
		}
		if !outcome.Skipped || outcome.SkipReason != "no-raw-dir" {
			t.Fatalf("ingest outcome = %+v; want Skipped with reason no-raw-dir", outcome)
		}
	})

	t.Run("ingest stage wired: distills a raw source", func(t *testing.T) {
		vault := t.TempDir()
		rawDir := filepath.Join(vault, "raw")
		if err := os.MkdirAll(rawDir, 0o755); err != nil {
			t.Fatalf("mkdir raw: %v", err)
		}
		if err := os.WriteFile(filepath.Join(rawDir, "note.md"), []byte("# Note\nbody\n"), 0o644); err != nil {
			t.Fatalf("write raw note: %v", err)
		}
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		outcome, err := pipeline.RunStage(context.Background(), vault, StageIngest, 1)
		if err != nil {
			t.Fatalf("RunStage(ingest) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("ingest stage reported %q; want it wired", SkipReasonHandlerNotWired)
		}
		if outcome.Skipped {
			t.Fatalf("ingest skipped despite a raw source: %q", outcome.SkipReason)
		}
		if len(outcome.Artifacts) != 1 {
			t.Fatalf("ingest wrote %d artifacts; want 1", len(outcome.Artifacts))
		}
		want := filepath.Join(vault, "wiki", "sources", "note.md")
		if outcome.Artifacts[0] != want {
			t.Fatalf("ingest artifact %q; want %q", outcome.Artifacts[0], want)
		}
	})

	t.Run("query stage wired: no pending query yields no-pending-query skip", func(t *testing.T) {
		vault := t.TempDir()
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		outcome, err := pipeline.RunStage(context.Background(), vault, StageQuery, 1)
		if err != nil {
			t.Fatalf("RunStage(query) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("query stage reported %q; want it wired", SkipReasonHandlerNotWired)
		}
		if !outcome.Skipped || outcome.SkipReason != "no-pending-query" {
			t.Fatalf("query outcome = %+v; want Skipped with reason no-pending-query", outcome)
		}
	})

	t.Run("query stage wired: answers a pending query", func(t *testing.T) {
		vault := t.TempDir()
		wikiDir := filepath.Join(vault, "wiki")
		if err := os.MkdirAll(wikiDir, 0o755); err != nil {
			t.Fatalf("mkdir wiki: %v", err)
		}
		if err := os.WriteFile(filepath.Join(wikiDir, ".query-pending.json"), []byte("how does ingest work"), 0o644); err != nil {
			t.Fatalf("write query pending: %v", err)
		}
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))

		outcome, err := pipeline.RunStage(context.Background(), vault, StageQuery, 1)
		if err != nil {
			t.Fatalf("RunStage(query) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("query stage reported %q; want it wired", SkipReasonHandlerNotWired)
		}
		if outcome.Skipped {
			t.Fatalf("query skipped despite a pending query: %q", outcome.SkipReason)
		}
		if len(outcome.Artifacts) != 1 {
			t.Fatalf("query wrote %d artifacts; want 1", len(outcome.Artifacts))
		}
		want := filepath.Join(vault, "wiki", "synthesis", "query-how-does-ingest-work.md")
		if outcome.Artifacts[0] != want {
			t.Fatalf("query artifact %q; want %q", outcome.Artifacts[0], want)
		}
	})

	t.Run("stages are selectable: whitelist overrides auto-selection", func(t *testing.T) {
		vault := t.TempDir()
		// Make a vault where auto-selection picks INGEST: a fresh .last-lint
		// sentinel keeps lint non-stale, so SelectStage falls through to the
		// ingest default. The whitelist then pins QUERY, proving the whitelist
		// is a hard filter rather than a hint.
		wikiDir := filepath.Join(vault, "wiki")
		if err := os.MkdirAll(wikiDir, 0o755); err != nil {
			t.Fatalf("mkdir wiki: %v", err)
		}
		if err := os.WriteFile(filepath.Join(wikiDir, ".last-lint"), nil, 0o644); err != nil {
			t.Fatalf("write last-lint sentinel: %v", err)
		}
		if err := os.WriteFile(filepath.Join(wikiDir, ".query-pending.json"), []byte("a pinned question"), 0o644); err != nil {
			t.Fatalf("write query pending: %v", err)
		}
		pipeline := NewWikiPipeline(WithClock(pipelineFixedClock(clock)))
		if auto := pipeline.SelectStage(vault, 24); auto != StageIngest {
			t.Fatalf("auto-selected stage %q; expected the ingest fallback", auto)
		}

		claim := daemon.QueueLease{Job: daemon.QueueJobState{
			JobType: daemon.JobTypeLLMWikiLoop,
			Attempt: 1,
			Payload: map[string]any{
				"vault":  vault,
				"stages": []any{string(StageQuery)},
			},
		}}
		res, err := pipeline.RunJob(context.Background(), claim)
		if err != nil {
			t.Fatalf("RunJob error: %v", err)
		}
		if got := res.Artifacts["stage"]; got != string(StageQuery) {
			t.Fatalf("whitelist not honored: ran %q; want %q", got, StageQuery)
		}
	})

	t.Run("Stages reports wired stages without an unconfigured promote", func(t *testing.T) {
		pipeline := NewWikiPipeline()
		got := pipeline.Stages()
		want := []PipelineStage{StageIngest, StageQuery, StageLint}
		if len(got) != len(want) {
			t.Fatalf("Stages() = %v; want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Stages()[%d] = %q; want %q", i, got[i], want[i])
			}
		}
		if pipeline.HasStage(StagePromote) {
			t.Fatalf("HasStage(promote) = true; promote is unwired by default")
		}
		for _, s := range want {
			if !pipeline.HasStage(s) {
				t.Fatalf("HasStage(%q) = false; want wired", s)
			}
		}
	})

	t.Run("promote unwired yields a real reason, never handler-not-wired", func(t *testing.T) {
		vault := t.TempDir()
		pipeline := NewWikiPipeline()

		outcome, err := pipeline.RunStage(context.Background(), vault, StagePromote, 1)
		if err != nil {
			t.Fatalf("RunStage(promote) error: %v", err)
		}
		if outcome.SkipReason == SkipReasonHandlerNotWired {
			t.Fatalf("promote reported %q; want a pipeline-specific reason", SkipReasonHandlerNotWired)
		}
		if !outcome.Skipped || outcome.SkipReason != "promote-handler-not-configured" {
			t.Fatalf("promote outcome = %+v; want Skipped promote-handler-not-configured", outcome)
		}
	})

	t.Run("promote stage is selectable once a runner is wired", func(t *testing.T) {
		vault := t.TempDir()
		runner := &staticPromoteRunner{}
		pipeline := NewWikiPipeline(WithPromoteRunner(runner))

		if !pipeline.HasStage(StagePromote) {
			t.Fatalf("HasStage(promote) = false after WithPromoteRunner")
		}
		outcome, err := pipeline.RunStage(context.Background(), vault, StagePromote, 1)
		if err != nil {
			t.Fatalf("RunStage(promote) error: %v", err)
		}
		if !runner.ran {
			t.Fatalf("promote runner was not invoked")
		}
		if outcome.Skipped {
			t.Fatalf("promote skipped despite a wired runner: %q", outcome.SkipReason)
		}
		if outcome.Stage != StagePromote || len(outcome.Artifacts) != 1 {
			t.Fatalf("promote outcome = %+v; want stage=promote with 1 artifact", outcome)
		}
	})
}

// staticPromoteRunner is a test PromoteRunner that records whether it ran.
type staticPromoteRunner struct{ ran bool }

func (s *staticPromoteRunner) Run(_ context.Context, _ string, attempt int) (StageOutcome, error) {
	s.ran = true
	return StageOutcome{Stage: StagePromote, Attempt: attempt, Artifacts: []string{"promoted"}}, nil
}

// TestWikiPipeline_JobTypeMismatch asserts RunJob rejects a non-llmwiki job.
func TestWikiPipeline_JobTypeMismatch(t *testing.T) {
	pipeline := NewWikiPipeline()
	claim := daemon.QueueLease{Job: daemon.QueueJobState{JobType: daemon.JobType("other.job")}}

	_, err := pipeline.RunJob(context.Background(), claim)
	if err == nil {
		t.Fatalf("RunJob accepted a non-llmwiki job type; want error")
	}
	if !strings.Contains(err.Error(), "does not support job type") {
		t.Fatalf("error = %v; want a job-type-mismatch message", err)
	}
}

// TestWikiPipeline_JobTypes asserts the pipeline declares JobTypeLLMWikiLoop.
func TestWikiPipeline_JobTypes(t *testing.T) {
	pipeline := NewWikiPipeline()
	types := pipeline.JobTypes()
	if len(types) != 1 || types[0] != daemon.JobTypeLLMWikiLoop {
		t.Fatalf("JobTypes() = %v; want [%s]", types, daemon.JobTypeLLMWikiLoop)
	}
}
