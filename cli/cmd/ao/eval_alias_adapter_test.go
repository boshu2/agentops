package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

func TestEvalAliasAdapterSessionOutcomeMapsSharedAnalyzer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte("tests passed\ngit commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := (evalAliasAdapter{}).SessionOutcome(context.Background(), aoeval.SessionOutcomeRequest{TranscriptPath: path, SessionID: "session-1"})
	if err != nil {
		t.Fatalf("SessionOutcome: %v", err)
	}
	if result.SessionID != "session-1" || result.Reward <= 0 || len(result.Signals) == 0 {
		t.Fatalf("result=%#v", result)
	}
}

func TestEvalAliasAdapterSessionOutcomeDryRunDoesNotAnalyze(t *testing.T) {
	result, err := (evalAliasAdapter{}).SessionOutcome(context.Background(), aoeval.SessionOutcomeRequest{TranscriptPath: "missing", DryRun: true})
	if err != nil {
		t.Fatalf("SessionOutcome: %v", err)
	}
	if !result.DryRun || result.Transcript != "missing" {
		t.Fatalf("result=%#v", result)
	}
}
