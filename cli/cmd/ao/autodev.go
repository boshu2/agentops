// practices: [ai-assisted-dev, agile-manifesto]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/autodev"
	"github.com/spf13/cobra"
)

var (
	autodevFile  string
	autodevForce bool
)

var autodevCmd = &cobra.Command{
	Use:   "autodev",
	Short: "Manage the PROGRAM.md operational contract for autonomous development",
	Long: `Define, inspect, and validate the repo-local PROGRAM.md contract.

PROGRAM.md is the operational layer for bounded autonomous development:
- mutable and immutable scope
- experiment unit
- validation bundle
- decision policy
- escalation rules
- stop conditions`,
}

type autodevValidateResult struct {
	Path                 string   `json:"path"`
	Valid                bool     `json:"valid"`
	Errors               []string `json:"errors,omitempty"`
	Format               string   `json:"format,omitempty"`
	Objective            string   `json:"objective,omitempty"`
	MutableScopeCount    int      `json:"mutable_scope_count,omitempty"`
	ImmutableScopeCount  int      `json:"immutable_scope_count,omitempty"`
	ValidationCommandCnt int      `json:"validation_command_count,omitempty"`
	StopConditionCount   int      `json:"stop_condition_count,omitempty"`
}

func init() {
	autodevCmd.GroupID = "workflow"
	autodevCmd.PersistentFlags().StringVar(&autodevFile, "file", "", "Path to PROGRAM.md or AUTODEV.md (auto-detects PROGRAM.md then AUTODEV.md)")
	rootCmd.AddCommand(autodevCmd)

	autodevCmd.AddCommand(autodevInitCmd)
	autodevCmd.AddCommand(autodevValidateCmd)
	autodevCmd.AddCommand(autodevShowCmd)
}

var autodevInitCmd = &cobra.Command{
	Use:   "init [objective]",
	Short: "Create a PROGRAM.md template",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.TrimSpace(autodevFile)
		if target == "" {
			target = "PROGRAM.md"
		}
		objective := "Define and run bounded autonomous development experiments for this repo."
		if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			objective = strings.TrimSpace(args[0])
		}

		if !autodevForce {
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("%s already exists (use --force to overwrite)", target)
			}
		}

		content := renderProgramTemplate(objective)
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		fmt.Printf("Created %s\n", target)
		return nil
	},
}

var autodevValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate PROGRAM.md structure",
	RunE: func(cmd *cobra.Command, args []string) error {
		prog, path, err := loadAutodevProgramForCommand()
		result := autodevValidateResult{Path: displayProgramPath(path)}
		if err != nil {
			result.Errors = []string{err.Error()}
			return outputAutodevValidateResult(result)
		}

		result.Format = prog.Format
		result.Objective = prog.Objective
		result.MutableScopeCount = len(prog.MutableScope)
		result.ImmutableScopeCount = len(prog.ImmutableScope)
		result.ValidationCommandCnt = len(prog.ValidationCommands)
		result.StopConditionCount = len(prog.StopConditions)
		if errs := autodev.ValidateProgram(prog); len(errs) > 0 {
			for _, err := range errs {
				result.Errors = append(result.Errors, err.Error())
			}
			return outputAutodevValidateResult(result)
		}
		result.Valid = true
		return outputAutodevValidateResult(result)
	},
}

var autodevShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the parsed PROGRAM.md contract",
	RunE: func(cmd *cobra.Command, args []string) error {
		prog, _, err := loadAutodevProgramForCommand()
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(prog)
		}
		fmt.Printf("Objective: %s\n", prog.Objective)
		fmt.Printf("Mutable Scope: %d\n", len(prog.MutableScope))
		for _, item := range prog.MutableScope {
			fmt.Printf("  - %s\n", item)
		}
		fmt.Printf("Immutable Scope: %d\n", len(prog.ImmutableScope))
		for _, item := range prog.ImmutableScope {
			fmt.Printf("  - %s\n", item)
		}
		fmt.Printf("Validation Commands: %d\n", len(prog.ValidationCommands))
		for _, cmd := range prog.ValidationCommands {
			fmt.Printf("  - %s\n", cmd)
		}
		fmt.Printf("Stop Conditions: %d\n", len(prog.StopConditions))
		for _, item := range prog.StopConditions {
			fmt.Printf("  - %s\n", item)
		}
		return nil
	},
}

func init() {
	autodevInitCmd.Flags().BoolVar(&autodevForce, "force", false, "Overwrite an existing program file")
}

func loadAutodevProgramForCommand() (*autodev.Program, string, error) {
	target := strings.TrimSpace(autodevFile)
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		rel := autodev.ResolveProgramPath(cwd)
		if rel == "" {
			return nil, "", fmt.Errorf("PROGRAM.md not found (looked for PROGRAM.md and AUTODEV.md)")
		}
		target = filepath.Join(cwd, rel)
	}
	return autodev.LoadProgram(target)
}

func outputAutodevValidateResult(result autodevValidateResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Valid {
		fmt.Printf("VALID: %s (%s)\n", result.Path, result.Format)
		fmt.Printf("  Mutable scope: %d\n", result.MutableScopeCount)
		fmt.Printf("  Immutable scope: %d\n", result.ImmutableScopeCount)
		fmt.Printf("  Validation commands: %d\n", result.ValidationCommandCnt)
		fmt.Printf("  Stop conditions: %d\n", result.StopConditionCount)
		return nil
	}

	fmt.Printf("INVALID: %s\n", result.Path)
	for _, err := range result.Errors {
		fmt.Printf("  ERROR: %s\n", err)
	}
	return fmt.Errorf("validation failed")
}

func displayProgramPath(path string) string {
	if path == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, relErr := filepath.Rel(cwd, path); relErr == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func renderProgramTemplate(objective string) string {
	return fmt.Sprintf(`# PROGRAM.md

## Objective

%s

## Mutable Scope

- cli/cmd/ao/**

## Immutable Scope

- hooks/**

## Experiment Unit

One bounded TDD slice that updates tests first, then implementation, then validation.

## Validation Commands

- `+"`cd cli && go test ./cmd/ao/...`"+`

## Decision Policy

- Keep changes only when the validation bundle is green.
- Prefer simpler diffs when outcomes tie.
- Revert non-improving or out-of-scope experiments.

## Escalation Rules

Open a bead when required work escapes mutable scope.

## Stop Conditions

- Validation bundle is green.
- No unhandled out-of-scope findings remain.
`, objective)
}
