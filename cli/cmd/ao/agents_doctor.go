package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var agentsDoctorJSON bool

var agentsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose .agents/ contract, lint, and on-disk surfaces",
	Long: `Compose the .agents/ write-surface contract, active skill-owned
subdirs, lint status, and observed top-level .agents/ directories into one
read-only diagnostic summary.`,
	RunE: runAgentsDoctor,
}

func init() {
	agentsCmd.AddCommand(agentsDoctorCmd)
	agentsDoctorCmd.Flags().BoolVar(&agentsDoctorJSON, "json", false, "Emit machine-readable JSON")
}

// AgentsDoctorDiagnostics is the shape returned by `ao agents doctor --json`.
type AgentsDoctorDiagnostics struct {
	Contract          string   `json:"contract"`
	AllowlistCount    int      `json:"allowlist_count"`
	SkillOwnedCount   int      `json:"skill_owned_count"`
	LintStatus        string   `json:"lint_status"`
	UnknownOnDiskDirs []string `json:"unknown_on_disk_dirs"`
	NextCommand       string   `json:"next_command"`
}

// AgentsDoctorError carries the process exit code for doctor diagnostics.
type AgentsDoctorError struct {
	ExitCode int
	Reason   string
}

func (e *AgentsDoctorError) Error() string {
	return fmt.Sprintf("agents doctor %s", e.Reason)
}

type agentsLintSummary struct {
	Status       string   `json:"status"`
	Undocumented []string `json:"undocumented"`
}

func runAgentsDoctor(cmd *cobra.Command, args []string) error {
	repoRoot, err := resolveAgentsRepoRoot()
	if err != nil {
		return &AgentsDoctorError{ExitCode: 2, Reason: err.Error()}
	}
	contract := filepath.Join(repoRoot, defaultAgentsContract)
	data, err := os.ReadFile(contract)
	if err != nil {
		return &AgentsDoctorError{ExitCode: 2, Reason: fmt.Sprintf("reading contract: %v", err)}
	}
	allowlist := parseAgentsAllowlist(string(data))
	skills := discoverActiveSkills(filepath.Join(repoRoot, "skills"))
	lintSummary, lintExitCode, err := runAgentsDoctorLint(repoRoot)
	if err != nil {
		return &AgentsDoctorError{ExitCode: 2, Reason: err.Error()}
	}

	diag := AgentsDoctorDiagnostics{
		Contract:          contract,
		AllowlistCount:    len(allowlist),
		SkillOwnedCount:   len(skills),
		LintStatus:        lintSummary.Status,
		UnknownOnDiskDirs: findUnknownAgentsDirs(repoRoot, allowlist, skills),
		NextCommand:       "ao agents lint --json",
	}
	if diag.LintStatus == "" {
		diag.LintStatus = "error"
	}

	if agentsDoctorJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(diag); err != nil {
			return err
		}
	} else {
		writeAgentsDoctorText(cmd, diag)
	}

	if lintExitCode != 0 || len(lintSummary.Undocumented) > 0 || len(diag.UnknownOnDiskDirs) > 0 {
		cmd.SilenceUsage = true
		return &AgentsDoctorError{ExitCode: 1, Reason: "found contract drift"}
	}
	return nil
}

func runAgentsDoctorLint(repoRoot string) (agentsLintSummary, int, error) {
	script := filepath.Join(repoRoot, defaultAgentsLintScript)
	if _, err := os.Stat(script); err != nil {
		return agentsLintSummary{}, 2, fmt.Errorf("lint script not found at %s: %w", script, err)
	}
	c := exec.Command("bash", script, "--json")
	c.Dir = repoRoot
	out, err := c.Output()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return agentsLintSummary{}, 2, fmt.Errorf("running lint script: %w", err)
		}
	}
	var summary agentsLintSummary
	if err := json.Unmarshal(out, &summary); err != nil {
		return agentsLintSummary{}, 2, fmt.Errorf("parsing lint JSON: %w", err)
	}
	return summary, exitCode, nil
}

func findUnknownAgentsDirs(repoRoot string, allowlist, skills []string) []string {
	allowed := make(map[string]bool, len(allowlist)+len(skills))
	for _, entry := range allowlist {
		allowed[entry] = true
	}
	for _, skill := range skills {
		allowed[skill] = true
	}
	entries, err := os.ReadDir(filepath.Join(repoRoot, ".agents"))
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !allowed[entry.Name()] {
			out = append(out, entry.Name())
		}
	}
	sort.Strings(out)
	return out
}

func writeAgentsDoctorText(cmd *cobra.Command, diag AgentsDoctorDiagnostics) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Contract: %s\n", diag.Contract)
	fmt.Fprintf(out, "Catalogued surfaces: %d\n", diag.AllowlistCount)
	fmt.Fprintf(out, "Skill-owned subdirs: %d\n", diag.SkillOwnedCount)
	fmt.Fprintf(out, "Lint status: %s\n", diag.LintStatus)
	fmt.Fprintf(out, "Unknown on-disk dirs: %d\n", len(diag.UnknownOnDiskDirs))
	for _, dir := range diag.UnknownOnDiskDirs {
		fmt.Fprintf(out, "  .agents/%s/\n", dir)
	}
	fmt.Fprintf(out, "Next command: %s\n", diag.NextCommand)
}
