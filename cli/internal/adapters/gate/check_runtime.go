package gate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/gates"
)

type CheckRuntime struct{}

func (CheckRuntime) ApplyRangeScope(scope gates.Scope) error {
	spec, ok := gates.ScopeRange(scope)
	if !ok {
		return nil
	}
	if err := gates.ValidateRangeSpec(spec); err != nil {
		return err
	}
	if err := os.Setenv("AGENTOPS_GATE_RANGE", spec); err != nil {
		return fmt.Errorf("set AGENTOPS_GATE_RANGE: %w", err)
	}
	return nil
}

func (CheckRuntime) WorkflowCoverage(registry *gates.Registry, root, workflowPath string) (*gates.WorkflowCoverage, error) {
	return gates.RegistryWorkflowCoverage(registry, root, workflowPath)
}

func ResolveRepoRoot(start string) (string, error) {
	if start == "" {
		return "", fmt.Errorf("resolve gate repo root: start directory required")
	}
	command := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err == nil {
		if root := strings.TrimSpace(string(output)); root != "" {
			return root, nil
		}
	}
	return start, nil
}

var _ gateapp.CheckRuntime = CheckRuntime{}
