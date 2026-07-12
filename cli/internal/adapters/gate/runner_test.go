package gate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func tempGateRepo(t *testing.T, name, script string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "check-"+name+".sh"), []byte("#!/usr/bin/env bash\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunnerMapsExitClasses(t *testing.T) {
	tests := []struct {
		name   string
		exit   string
		status ports.GateStatus
	}{
		{name: "pass", exit: "0", status: ports.GateStatusPass},
		{name: "warn", exit: "2", status: ports.GateStatusWarn},
		{name: "skip", exit: "75", status: ports.GateStatusSkip},
		{name: "fail", exit: "1", status: ports.GateStatusFail},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := NewRunner(tempGateRepo(t, test.name, "echo marker; exit "+test.exit))
			verdict, err := runner.Run(context.Background(), ports.GateRunRequest{Name: ports.GateName(test.name)})
			if err != nil || verdict.Status != test.status || !strings.Contains(verdict.LogTail, "marker") {
				t.Fatalf("verdict=%+v err=%v", verdict, err)
			}
		})
	}
}

func TestRunnerMissingAndEmptyNamesAreUnknown(t *testing.T) {
	runner := NewRunner(t.TempDir())
	for _, name := range []ports.GateName{"", "missing"} {
		verdict, err := runner.Run(context.Background(), ports.GateRunRequest{Name: name})
		if err != nil || verdict.Status != ports.GateStatusUnknown {
			t.Fatalf("name=%q verdict=%+v err=%v", name, verdict, err)
		}
	}
}

func TestRunnerForwardsEnvironmentAndCapsLogTail(t *testing.T) {
	runner := NewRunner(tempGateRepo(t, "env", `printf '%05000d' 0; echo "KEY=$KEY"`))
	verdict, err := runner.Run(context.Background(), ports.GateRunRequest{Name: "env", Env: map[string]string{"KEY": "value"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(verdict.LogTail) > 4096 || !strings.Contains(verdict.LogTail, "KEY=value") {
		t.Fatalf("tail len=%d suffix=%q", len(verdict.LogTail), verdict.LogTail[len(verdict.LogTail)-32:])
	}
}

func TestRunnerHonorsCancellationAndRequiresRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewRunner(t.TempDir()).Run(ctx, ports.GateRunRequest{Name: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
	if _, err := NewRunner("").Run(context.Background(), ports.GateRunRequest{Name: "x"}); err == nil {
		t.Fatal("expected empty-root error")
	}
}
