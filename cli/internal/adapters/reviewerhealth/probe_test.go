package reviewerhealth

import (
	"context"
	"errors"
	"testing"
	"time"

	reviewerapp "github.com/boshu2/agentops/cli/internal/reviewerhealth"
)

func TestProbeMissingAndReachable(t *testing.T) {
	reviewer := reviewerapp.Reviewer{Name: "codex", InstallCommand: "install codex"}
	missing := Probe{
		LookPath: func(string) (string, error) { return "", errors.New("missing") },
		Run:      func(context.Context, string, ...string) error { return nil }, Now: time.Now,
	}
	if result := missing.Check(context.Background(), reviewer, time.Second); result.Status != "warn" || result.Live {
		t.Fatalf("missing result = %+v", result)
	}
	now := time.Unix(0, 0)
	reachable := Probe{
		LookPath: func(string) (string, error) { return "/bin/codex", nil },
		Run:      func(context.Context, string, ...string) error { return nil },
		Now:      func() time.Time { current := now; now = now.Add(25 * time.Millisecond); return current },
	}
	if result := reachable.Check(context.Background(), reviewer, time.Second); result.Status != "pass" || !result.Live {
		t.Fatalf("reachable result = %+v", result)
	}
}
