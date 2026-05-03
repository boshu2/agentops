package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/skillshealth"
)

var (
	skillsCheckJSON   bool
	skillsCheckStrict bool
	skillsCheckOnly   string
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Inspect and validate the skills/ tree",
	Long: `Tooling for the skills/ source-of-truth and its skills-codex/
parity sibling. Subcommands surface health (frontmatter completeness,
broken reference links, codex parity drift) without mutating either
tree.`,
}

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Audit skills/ frontmatter, references, and codex parity",
	Long: `Walk skills/ and skills-codex/, validating each skill's YAML
frontmatter (name + description present, name matches dir), checking
that every references/*.md is linked from SKILL.md (and vice versa),
and reporting parity drift against skills-codex/.

Exits 0 by default. With --strict, exits 1 if any finding (missing
frontmatter, broken reference, parity drift) is reported, suitable for
CI gating.`,
	RunE: runSkillsCheck,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsCheckCmd)
	skillsCheckCmd.Flags().BoolVar(&skillsCheckJSON, "json", false, "Emit machine-readable JSON")
	skillsCheckCmd.Flags().BoolVar(&skillsCheckStrict, "strict", false, "Exit non-zero on any finding (CI mode)")
	skillsCheckCmd.Flags().StringVar(&skillsCheckOnly, "skill", "", "Restrict the audit to a single skill name")
}

func runSkillsCheck(cmd *cobra.Command, args []string) error {
	skillsDir, codexDir := resolveSkillsRoots()
	opts := skillshealth.Options{
		SkillsDir: skillsDir,
		CodexDir:  codexDir,
		OnlySkill: skillsCheckOnly,
		Strict:    skillsCheckStrict,
	}
	report, err := skillshealth.Audit(opts)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if skillsCheckJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Skills audit (%s)\n", report.Generated)
		fmt.Fprintf(out, "================\n")
		fmt.Fprintf(out, "Skills audited: %d\n", len(report.Skills))
		fmt.Fprintf(out, "Errors:         %d\n", len(report.Errors))
		fmt.Fprintf(out, "Parity drift:   %d\n\n", len(report.ParityDrift))

		if len(report.Errors) > 0 {
			fmt.Fprintln(out, "Errors:")
			for _, e := range report.Errors {
				fmt.Fprintf(out, "  - %s\n", e)
			}
			fmt.Fprintln(out)
		}
		if len(report.ParityDrift) > 0 {
			fmt.Fprintln(out, "Codex parity drift:")
			for _, e := range report.ParityDrift {
				fmt.Fprintf(out, "  - %s\n", e)
			}
		}
		if len(report.Errors) == 0 && len(report.ParityDrift) == 0 {
			fmt.Fprintln(out, "All skills healthy.")
		}
	}

	if skillsCheckStrict && (len(report.Errors) > 0 || len(report.ParityDrift) > 0) {
		// Use SilenceUsage to avoid printing usage on this expected non-zero exit.
		cmd.SilenceUsage = true
		return fmt.Errorf("skills check failed: %d errors, %d parity-drift",
			len(report.Errors), len(report.ParityDrift))
	}
	return nil
}

// resolveSkillsRoots locates the skills/ and skills-codex/ directories
// relative to the current working directory, walking up the tree until both
// are found. Falls back to literal "skills" / "skills-codex" if not found,
// which produces a clear error from os.ReadDir.
func resolveSkillsRoots() (string, string) {
	const skills = "skills"
	const codex = "skills-codex"
	cwd, err := os.Getwd()
	if err != nil {
		return skills, codex
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		s := filepath.Join(dir, skills)
		c := filepath.Join(dir, codex)
		if isDir(s) && isDir(c) {
			return s, c
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return skills, codex
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
