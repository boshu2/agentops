package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
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
type AgentsDoctorReport struct {
	Inventory        AgentsInventory   `json:"inventory"`
	LintScript       string            `json:"lint_script"`
	LintExitCode     int               `json:"lint_exit_code"`
	LintClean        bool              `json:"lint_clean"`
	OrphanSkills     []string          `json:"orphan_skills"`
	UndocumentedDirs []UndocumentedDir `json:"undocumented_dirs"`
	Strict           bool              `json:"strict"`
}

// UndocumentedDir is an entry under .agents/ that is neither catalogued in
// the contract allowlist nor a skill-owned subdir.
type UndocumentedDir struct {
	Name    string `json:"name"`
	FixHint string `json:"fix_hint"`
}

func runAgentsDoctor(cmd *cobra.Command, args []string) error {
	contractData, err := os.ReadFile(agentsDoctorContract)
	if err != nil {
		return fmt.Errorf("reading contract %s: %w", agentsDoctorContract, err)
	}

	inv := AgentsInventory{
		Contract:  agentsDoctorContract,
		Allowlist: parseAgentsAllowlist(string(contractData)),
		Skills:    discoverActiveSkills(agentsDoctorSkillsDir),
	}

	lintStdout := cmd.OutOrStdout()
	if agentsDoctorJSON {
		lintStdout = io.Discard
	}
	lintExit, lintErr := runDoctorLint(cmd, agentsDoctorScript, agentsDoctorJSON, lintStdout)
	lintClean := lintErr == nil

	orphanSkills := findOrphanSkills(inv.Skills, agentsDoctorAgentsDir)
	undocumented := findUndocumentedDirs(agentsDoctorAgentsDir, inv.Allowlist, inv.Skills)

	report := AgentsDoctorReport{
		Inventory:        inv,
		LintScript:       agentsDoctorScript,
		LintExitCode:     lintExit,
		LintClean:        lintClean,
		OrphanSkills:     orphanSkills,
		UndocumentedDirs: undocumented,
		Strict:           agentsDoctorStrict,
	}

	if agentsDoctorJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			return encErr
		}
	} else {
		printDoctorText(cmd.OutOrStdout(), report)
	}

	if !lintClean {
		cmd.SilenceUsage = true
		return &AgentsLintError{ExitCode: lintExit, Script: agentsDoctorScript}
	}
	if agentsDoctorStrict && (len(orphanSkills) > 0 || len(undocumented) > 0) {
		cmd.SilenceUsage = true
		return fmt.Errorf("strict: %d orphan skills, %d undocumented dirs",
			len(orphanSkills), len(undocumented))
	}
	return nil
}

// runDoctorLint executes the lint script and returns its exit code plus a
// nil/non-nil error mirror of agents_lint.go's behavior. It does not abort
// the doctor flow on script absence — it surfaces the absence as a
// distinct exit-code (-1) so the report can record it.
func runDoctorLint(cmd *cobra.Command, scriptPath string, jsonOut bool, stdout io.Writer) (int, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return -1, fmt.Errorf("lint script not found at %s: %w", scriptPath, err)
	}
	cmdArgs := []string{}
	if jsonOut {
		cmdArgs = append(cmdArgs, "--json")
	}
	c := exec.Command("bash", append([]string{scriptPath}, cmdArgs...)...)
	c.Stdout = stdout
	c.Stderr = cmd.ErrOrStderr()
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), &AgentsLintError{ExitCode: ee.ExitCode(), Script: scriptPath}
	}
	return -1, fmt.Errorf("running %s: %w", scriptPath, err)
}

// findOrphanSkills returns the names of skills that have no corresponding
// .agents/<skill>/ subdir. Result is sorted.
func findOrphanSkills(skills []string, agentsDir string) []string {
	out := []string{}
	for _, name := range skills {
		path := filepath.Join(agentsDir, name)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// findUndocumentedDirs returns entries under .agents/ that are neither in
// the catalogued allowlist nor a skill-owned subdir. Hidden dirs (.git,
// .archive) are skipped. Result is sorted by name.
func findUndocumentedDirs(agentsDir string, allowlist, skills []string) []UndocumentedDir {
	allowed := make(map[string]bool, len(allowlist)+len(skills))
	for _, e := range allowlist {
		allowed[e] = true
	}
	skillSet := make(map[string]bool, len(skills))
	for _, e := range skills {
		allowed[e] = true
		skillSet[e] = true
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}
	out := []UndocumentedDir{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name[0] == '.' {
			continue
		}
		if allowed[name] {
			continue
		}
		out = append(out, UndocumentedDir{
			Name:    name,
			FixHint: fmt.Sprintf("add `%s` to docs/contracts/agents-write-surfaces.md allowlist or remove the dir", name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func printDoctorText(out interface{ Write(p []byte) (int, error) }, r AgentsDoctorReport) {
	fmt.Fprintln(out, "ao agents doctor")
	fmt.Fprintf(out, "  Contract: %s\n", r.Inventory.Contract)
	fmt.Fprintf(out, "  Catalogued surfaces: %d\n", len(r.Inventory.Allowlist))
	fmt.Fprintf(out, "  Skill-owned subdirs: %d\n", len(r.Inventory.Skills))
	if r.LintClean {
		fmt.Fprintln(out, "  Lint: PASS")
	} else {
		fmt.Fprintf(out, "  Lint: FAIL (exit %d, script=%s)\n", r.LintExitCode, r.LintScript)
	}
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Orphan skills (skills/<name>/ with no .agents/<name>/): %d\n", len(r.OrphanSkills))
	for _, name := range r.OrphanSkills {
		fmt.Fprintf(out, "  %s — fix: create .agents/%s/ or document why it's exempt\n", name, name)
	}
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Undocumented dirs (.agents/<name>/ not catalogued, not a skill): %d\n", len(r.UndocumentedDirs))
	for _, d := range r.UndocumentedDirs {
		fmt.Fprintf(out, "  %s — fix: %s\n", d.Name, d.FixHint)
	}
}
