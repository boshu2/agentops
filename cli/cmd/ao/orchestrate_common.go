package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// timeoutCommandRunner bounds each substrate probe so preflight/tools cannot hang.
type timeoutCommandRunner struct {
	inner   orchestration.CommandRunner
	timeout time.Duration
}

func (t timeoutCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	c, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.inner.Run(c, name, args...)
}

func orchestrateRunner() orchestration.CommandRunner {
	return timeoutCommandRunner{inner: execCommandRunner{}, timeout: 5 * time.Second}
}

func orchestrateRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, orchestration.ToolsContractRelPath)); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	return "", fmt.Errorf("orchestration contracts not found; run from agentops repo root")
}

func writeInstrumentResult(w io.Writer, result orchestration.InstrumentResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Fprintf(w, "command=%s verdict=%s confidence=%s\n",
		result.Command, result.Verdict.Status, result.Verdict.Confidence)
	if result.CoordinationDegraded {
		fmt.Fprintln(w, "coordination_degraded=true")
	}
	if result.LedgerUnwritten {
		fmt.Fprintln(w, "ledger_unwritten=true")
	}
	if result.Route != nil && result.Route.NextCommand != "" {
		fmt.Fprintf(w, "next_command=%s\n", result.Route.NextCommand)
	}
	return nil
}

func finishInstrumentCommand(cmd *cobra.Command, result orchestration.InstrumentResult) error {
	if err := writeInstrumentResult(cmd.OutOrStdout(), result, orchestrateJSON); err != nil {
		return err
	}
	if orchestration.ExitCodeForVerdict(result.Verdict.Status) != 0 {
		return fmt.Errorf("orchestrate %s: %s", result.Command, result.Verdict.Status)
	}
	return nil
}

var orchestrateJSON bool
