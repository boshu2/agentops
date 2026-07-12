package eval

import (
	"context"
	"time"
)

type SessionOutcomeRequest struct {
	TranscriptPath, SessionID string
	DryRun                    bool
}
type SessionSignal struct {
	Name   string  `json:"name"`
	Value  bool    `json:"value"`
	Weight float64 `json:"weight"`
}
type SessionOutcomeResult struct {
	SessionID  string          `json:"session_id"`
	Reward     float64         `json:"reward"`
	Signals    []SessionSignal `json:"signals"`
	AnalyzedAt time.Time       `json:"analyzed_at"`
	Transcript string          `json:"transcript,omitempty"`
	TotalLines int             `json:"total_lines,omitempty"`
	DryRun     bool            `json:"-"`
}
type AliasOutput struct{ Stdout, Stderr string }

type AliasUseCases interface {
	SessionOutcome(context.Context, SessionOutcomeRequest) (SessionOutcomeResult, error)
	Chaos(context.Context) (AliasOutput, error)
}
