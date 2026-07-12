package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

type evalAliasAdapter struct{}

func (evalAliasAdapter) SessionOutcome(_ context.Context, request aoeval.SessionOutcomeRequest) (aoeval.SessionOutcomeResult, error) {
	path := request.TranscriptPath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = findMostRecentTranscript(filepath.Join(home, ".claude", "projects"))
		if path == "" {
			return aoeval.SessionOutcomeResult{}, fmt.Errorf("no transcript found; specify path as argument")
		}
	}
	if request.DryRun {
		return aoeval.SessionOutcomeResult{Transcript: path, DryRun: true}, nil
	}
	outcome, err := analyzeTranscript(path, request.SessionID)
	if err != nil {
		return aoeval.SessionOutcomeResult{}, fmt.Errorf("analyze transcript: %w", err)
	}
	result := aoeval.SessionOutcomeResult{SessionID: outcome.SessionID, Reward: outcome.Reward, AnalyzedAt: outcome.AnalyzedAt, Transcript: outcome.Transcript, TotalLines: outcome.TotalLines, Signals: make([]aoeval.SessionSignal, 0, len(outcome.Signals))}
	for _, signal := range outcome.Signals {
		result.Signals = append(result.Signals, aoeval.SessionSignal{Name: signal.Name, Value: signal.Value, Weight: signal.Weight})
	}
	return result, nil
}

func (evalAliasAdapter) Chaos(_ context.Context) (aoeval.AliasOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	var stdout, stderr bytes.Buffer
	err = tickSmoke(tickRuntime{workDir: workDir, stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr})
	return aoeval.AliasOutput{Stdout: stdout.String(), Stderr: stderr.String()}, err
}
