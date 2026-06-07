// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/agentsinspect"
	"github.com/boshu2/agentops/cli/internal/adapters/agentslint"
	"github.com/boshu2/agentops/cli/internal/adapters/agentsurface"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Inspect and manage .agents/ contracts and surfaces",
	Long: `Tooling for the .agents/ knowledge surface that backs the
AgentOps flywheel. Subcommands surface the catalogued write surfaces,
the active skill-owned subdirs, and (in future cycles) lint and
migration helpers.`,
}

var (
	agentsInspectJSON     bool
	agentsInspectContract string
	agentsLintScript      string
	agentsLintJSON        bool
)

const defaultAgentsContract = "docs/contracts/agents-write-surfaces.md"
const defaultAgentsLintScript = "scripts/check-agents-write-surfaces.sh"

var agentsInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Show catalogued .agents/ write surfaces and active skill-owned subdirs",
	Long: `Read the .agents/ write-surface contract and emit a structured
summary: the catalogued allowlist plus the set of active skills whose
.agents/<skill-name>/ subdirs are auto-allowed by
scripts/check-agents-write-surfaces.sh.`,
	RunE: runAgentsInspect,
}

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
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsInspectCmd)
	agentsCmd.AddCommand(agentsLintCmd)
	agentsInspectCmd.Flags().BoolVar(&agentsInspectJSON, "json", false, "Emit machine-readable JSON")
	agentsInspectCmd.Flags().StringVar(&agentsInspectContract, "contract",
		defaultAgentsContract,
		"Path to the .agents/ write-surfaces contract doc")
	agentsLintCmd.Flags().StringVar(&agentsLintScript, "script",
		defaultAgentsLintScript,
		"Path to the lint script")
	agentsLintCmd.Flags().BoolVar(&agentsLintJSON, "json", false,
		"Forward --json to the lint script")
}

// AgentsLintError is returned when the underlying lint script exits non-zero.
// It remains in the cmd package as an alias for root-level exit-code handling.
type AgentsLintError = agentslint.Error

// AgentsInventory is the shape returned by `ao agents inspect --json`.
type AgentsInventory = agentsurface.Inventory

func runAgentsInspect(cmd *cobra.Command, args []string) error {
	repoRoot, rootErr := resolveAgentsRepoRoot()
	contract := agentsInspectContract
	skillsDir := "skills"
	if rootErr == nil {
		contract = resolveAgentsDefaultPath(cmd, "contract", agentsInspectContract, defaultAgentsContract, repoRoot)
		skillsDir = filepath.Join(repoRoot, "skills")
	} else if shouldResolveAgentsDefaultPath(cmd, "contract", agentsInspectContract, defaultAgentsContract) {
		return rootErr
	}
	return agentsinspect.Run(agentsinspect.Options{
		Contract:  contract,
		SkillsDir: skillsDir,
		JSON:      agentsInspectJSON,
		Stdout:    cmd.OutOrStdout(),
	})
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

	err := agentslint.Run(agentslint.Options{
		Script: script,
		JSON:   agentsLintJSON,
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
	})
	if err == nil {
		return nil
	}
	var lintErr *agentslint.Error
	if errors.As(err, &lintErr) {
		cmd.SilenceUsage = true
	}
	return err
}

func resolveAgentsRepoRoot() (string, error) {
	startDir, err := resolveProjectDir()
	if err != nil {
		return "", err
	}
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolve project directory %s: %w", startDir, err)
	}
	for {
		if agentsRepoRootLooksValid(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("locating AgentOps repo root from %s", startDir)
		}
		dir = parent
	}
}

func agentsRepoRootLooksValid(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, defaultAgentsContract)); err != nil {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "skills")); err != nil || !info.IsDir() {
		return false
	}
	return true
}

func resolveAgentsDefaultPath(cmd *cobra.Command, flagName, value, defaultValue, repoRoot string) string {
	if !shouldResolveAgentsDefaultPath(cmd, flagName, value, defaultValue) {
		return value
	}
	return filepath.Join(repoRoot, defaultValue)
}

func shouldResolveAgentsDefaultPath(cmd *cobra.Command, flagName, value, defaultValue string) bool {
	return !filepath.IsAbs(value) && !cmd.Flags().Changed(flagName) && value == defaultValue
}

func parseAgentsAllowlist(content string) []string {
	return agentsurface.ParseAllowlist(content)
}

func discoverActiveSkills(skillsDir string) []string {
	return agentsurface.DiscoverActiveSkills(skillsDir)
}
