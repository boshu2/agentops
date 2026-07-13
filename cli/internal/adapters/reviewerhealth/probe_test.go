package reviewerhealth

import (
	"context"
	"errors"
	"strings"
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

func TestProbeHangingReviewerTimesOutWithoutWedgingDoctor(t *testing.T) {
	reviewer := reviewerapp.Reviewer{Name: "codex", InstallCommand: "install codex"}
	probe := Probe{
		LookPath: func(string) (string, error) { return "/bin/codex", nil },
		Run: func(ctx context.Context, _ string, _ ...string) error {
			<-ctx.Done()
			return ctx.Err()
		},
		Now: time.Now,
	}
	started := time.Now()
	result := probe.Check(context.Background(), reviewer, 25*time.Millisecond)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("hanging reviewer wedged doctor for %s", elapsed)
	}
	if result.Status != "fail" || result.Live {
		t.Fatalf("hanging result = %+v", result)
	}
	if !strings.Contains(result.Detail, "timed out") || !strings.Contains(result.Detail, "codex --version") {
		t.Fatalf("timeout detail lacks diagnosis and repair command: %q", result.Detail)
	}
}

func TestProbeBrokenReviewerNamesInstallRepair(t *testing.T) {
	reviewer := reviewerapp.Reviewer{Name: "agy", InstallCommand: "reinstall agy"}
	probe := Probe{
		LookPath: func(string) (string, error) { return "/bin/agy", nil },
		Run:      func(context.Context, string, ...string) error { return errors.New("exit 7") },
		Now:      time.Now,
	}
	result := probe.Check(context.Background(), reviewer, time.Second)
	if result.Status != "fail" || result.Live || !strings.Contains(result.Detail, "reinstall agy") {
		t.Fatalf("broken result = %+v", result)
	}
}

func TestProbeMissingReviewerNamesExactInstallCommand(t *testing.T) {
	reviewer := reviewerapp.Reviewer{Name: "codex", InstallCommand: "npm install -g @openai/codex && codex login"}
	probe := Probe{
		LookPath: func(string) (string, error) { return "", errors.New("missing") },
		Run:      func(context.Context, string, ...string) error { return nil },
		Now:      time.Now,
	}
	started := time.Now()
	result := probe.Check(context.Background(), reviewer, time.Second)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("missing reviewer did not short-circuit: %s", elapsed)
	}
	if result.Status != "warn" || result.Live || !strings.Contains(result.Detail, reviewer.InstallCommand) {
		t.Fatalf("missing result = %+v", result)
	}
}
