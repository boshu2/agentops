package gate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/quality"
)

const gateLogTailLimit = 4096

type Runner struct {
	repoRoot string
}

func NewRunner(repoRoot string) *Runner {
	return &Runner{repoRoot: repoRoot}
}

func (runner *Runner) Run(ctx context.Context, request ports.GateRunRequest) (ports.GateVerdict, error) {
	if err := ctx.Err(); err != nil {
		return ports.GateVerdict{}, err
	}
	if request.Name == "" {
		return ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: "empty GateName"}, nil
	}
	if runner.repoRoot == "" {
		return ports.GateVerdict{}, fmt.Errorf("gate runner: repo root required")
	}
	scriptPath := filepath.Join(runner.repoRoot, "scripts", "check-"+string(request.Name)+".sh")
	if !quality.FileExists(scriptPath) {
		return ports.GateVerdict{
			Status: ports.GateStatusUnknown,
			Reason: fmt.Sprintf("no script for gate %q at %s", request.Name, scriptPath),
		}, nil
	}

	command := exec.CommandContext(ctx, "bash", scriptPath)
	command.Dir = runner.repoRoot
	if len(request.Env) > 0 {
		environment := make([]string, 0, len(request.Env))
		for key, value := range request.Env {
			environment = append(environment, key+"="+value)
		}
		command.Env = append(command.Environ(), environment...)
	}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	runErr := command.Run()

	exitCode := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil {
		return ports.GateVerdict{
			Status:  ports.GateStatusUnknown,
			Reason:  fmt.Sprintf("subprocess error: %v", runErr),
			LogTail: gateLogTail(output.Bytes()),
		}, nil
	}

	verdict := gateExitVerdict(exitCode)
	verdict.LogTail = gateLogTail(output.Bytes())
	return verdict, nil
}

func gateExitVerdict(code int) ports.GateVerdict {
	switch code {
	case 0:
		return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "exit 0"}
	case 2:
		return ports.GateVerdict{Status: ports.GateStatusWarn, Reason: "exit 2 (advisory)"}
	case 75:
		return ports.GateVerdict{Status: ports.GateStatusSkip, Reason: "exit 75 (structural skip)"}
	default:
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: fmt.Sprintf("exit %d", code)}
	}
}

func gateLogTail(output []byte) string {
	if len(output) <= gateLogTailLimit {
		return string(output)
	}
	return strings.TrimSpace(string(output[len(output)-gateLogTailLimit:]))
}

var _ ports.GateRunnerPort = (*Runner)(nil)
