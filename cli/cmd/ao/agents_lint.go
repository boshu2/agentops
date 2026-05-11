// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	agentsLintScript string
	agentsLintJSON   bool
)

const defaultAgentsLintScript = "scripts/check-agents-write-surfaces.sh"

var agentsLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run the .agents/ write-surfaces contract lint",
	Long: `Wrap scripts/check-agents-write-surfaces.sh and surface its
result through the ao agents namespace. Exits 0 on a clean contract;
non-zero on a contract violation or invocation error. With --json the
script's machine-readable output is forwarded verbatim.`,
	RunE: runAgentsLint,
}

func init() {
	agentsCmd.AddCommand(agentsLintCmd)
	agentsLintCmd.Flags().StringVar(&agentsLintScript, "script",
		defaultAgentsLintScript,
		"Path to the lint script")
	agentsLintCmd.Flags().BoolVar(&agentsLintJSON, "json", false,
		"Forward --json to the lint script")
}

// AgentsLintError is returned when the underlying lint script exits
// non-zero. The ExitCode field carries the script's exit code so the
// main loop can map it onto the host process exit status.
type AgentsLintError struct {
	ExitCode int
	Script   string
}

func (e *AgentsLintError) Error() string {
	return fmt.Sprintf("%s exited with code %d", e.Script, e.ExitCode)
}

func runAgentsLint(cmd *cobra.Command, args []string) error {
	script := agentsLintScript
	if shouldResolveAgentsDefaultPath(cmd, "script", agentsLintScript, defaultAgentsLintScript) {
		repoRoot, err := resolveAgentsRepoRoot()
		if err != nil {
			return err
		}
		script = resolveAgentsDefaultPath(cmd, "script", agentsLintScript, defaultAgentsLintScript, repoRoot)
	}
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("lint script not found at %s: %w", script, err)
	}

	cmdArgs := []string{}
	if agentsLintJSON {
		cmdArgs = append(cmdArgs, "--json")
	}
	c := exec.Command("bash", append([]string{script}, cmdArgs...)...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	err := c.Run()
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		cmd.SilenceUsage = true
		return &AgentsLintError{ExitCode: ee.ExitCode(), Script: script}
	}
	return fmt.Errorf("running %s: %w", script, err)
}
