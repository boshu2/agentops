// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/agentsdoctor"
	"github.com/boshu2/agentops/cli/internal/adapters/agentslint"
)

var (
	agentsDoctorJSON      bool
	agentsDoctorStrict    bool
	agentsDoctorContract  string
	agentsDoctorScript    string
	agentsDoctorAgentsDir string
	agentsDoctorSkillsDir string
)

var agentsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Combined health check: inspect + lint + orphan/stray-dir report",
	Long: `Combine ao agents inspect and ao agents lint into a single
pass and report on cross-surface orphans:
  - "orphan skills"    — skills/<name>/ exists but no .agents/<name>/ subdir
  - "undocumented dirs" — .agents/<name>/ exists but is neither catalogued
                          in the contract nor a skill-owned subdir

Exits non-zero when the lint script fails. With --strict, also exits
non-zero when any orphans or undocumented dirs are reported.`,
	RunE: runAgentsDoctor,
}

func init() {
	agentsCmd.AddCommand(agentsDoctorCmd)
	agentsDoctorCmd.Flags().BoolVar(&agentsDoctorJSON, "json", false, "Emit machine-readable JSON")
	agentsDoctorCmd.Flags().BoolVar(&agentsDoctorStrict, "strict", false,
		"Exit non-zero on any orphan or undocumented surface")
	agentsDoctorCmd.Flags().StringVar(&agentsDoctorContract, "contract",
		"docs/contracts/agents-write-surfaces.md",
		"Path to the .agents/ write-surfaces contract doc")
	agentsDoctorCmd.Flags().StringVar(&agentsDoctorScript, "script",
		"scripts/check-agents-write-surfaces.sh",
		"Path to the lint script")
	agentsDoctorCmd.Flags().StringVar(&agentsDoctorAgentsDir, "agents-dir",
		".agents",
		"Path to the .agents/ directory under inspection")
	agentsDoctorCmd.Flags().StringVar(&agentsDoctorSkillsDir, "skills-dir",
		"skills",
		"Path to the skills/ directory used for skill-owned-subdir cross-check")
}

// AgentsDoctorReport is the shape returned by `ao agents doctor --json`.
type AgentsDoctorReport = agentsdoctor.Report

// UndocumentedDir is an entry under .agents/ that is neither catalogued in the
// contract allowlist nor a skill-owned subdir.
type UndocumentedDir = agentsdoctor.UndocumentedDir

func runAgentsDoctor(cmd *cobra.Command, args []string) error {
	err := agentsdoctor.Run(agentsdoctor.Options{
		Contract:  agentsDoctorContract,
		Script:    agentsDoctorScript,
		AgentsDir: agentsDoctorAgentsDir,
		SkillsDir: agentsDoctorSkillsDir,
		JSON:      agentsDoctorJSON,
		Strict:    agentsDoctorStrict,
		Stdout:    cmd.OutOrStdout(),
		Stderr:    cmd.ErrOrStderr(),
	})
	if err == nil {
		return nil
	}
	var lintErr *agentslint.Error
	var strictErr *agentsdoctor.StrictError
	if errors.As(err, &lintErr) || errors.As(err, &strictErr) {
		cmd.SilenceUsage = true
	}
	return err
}
