package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

var (
	seedTemplate string
	seedForce    bool
)

var seedCmd = &cobra.Command{
	Use:   "seed [path]",
	Short: "Plant the seed in any repository",
	Long: `Plant the AgentOps seed in any repository.

This creates:
  .agents/          Directory structure for knowledge artifacts
  GOALS.md          Fitness goals (auto-detected or from template)
  Bootstrap learning  Initial learning artifact in .agents/learnings/
  CLAUDE.md section   Knowledge flywheel instructions

What it does NOT create:
  Hooks              Use "ao init --hooks" for hook registration
  Skills             Run: bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)

Templates (--template):
  go-cli       Go CLI project (detected via go.mod)
  python-lib   Python library (detected via pyproject.toml)
  web-app      Web application (detected via package.json)
  rust-cli     Rust project (detected via Cargo.toml)
  generic      Generic defaults

Auto-detection reads go.mod, package.json, pyproject.toml, and Cargo.toml
to pick the best template. Falls back to generic.

Examples:
  ao seed                       # Seed current directory (auto-detect)
  ao seed ./my-project          # Seed a specific path
  ao seed --template=go-cli     # Force Go CLI template
  ao seed --dry-run             # Show what would be created
  ao seed --force               # Overwrite existing seed files`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSeed,
}

func init() {
	seedCmd.Flags().StringVar(&seedTemplate, "template", "", "Goal template: go-cli, python-lib, web-app, rust-cli, generic (default: auto-detect)")
	seedCmd.Flags().BoolVar(&seedForce, "force", false, "Overwrite existing seed files")
	seedCmd.GroupID = "start"
	rootCmd.AddCommand(seedCmd)

	_ = seedCmd.RegisterFlagCompletionFunc("template", staticCompletionFunc(templateCompletionValues()...))
}

// validTemplates enumerates the allowed template names.
var validTemplates = lifecycle.ValidTemplates

// seedResult holds structured output for --json mode.
type seedResult struct {
	Path     string   `json:"path"`
	Template string   `json:"template"`
	Created  []string `json:"created"`
	Skipped  []string `json:"skipped"`
	DryRun   bool     `json:"dry_run"`
}

func runSeed(cmd *cobra.Command, args []string) error {
	if err := validateTemplateMapEntries(validTemplates, embedded.TemplatesFS); err != nil {
		return err
	}

	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Validate target exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("target path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("target path is not a directory: %s", absPath)
	}

	// Validate template if specified
	template := seedTemplate
	if template != "" {
		if !validTemplates[template] {
			return fmt.Errorf("unknown template %q (valid: go-cli, python-lib, web-app, rust-cli, generic)", template)
		}
	} else {
		template = detectTemplate(absPath)
	}

	result := seedResult{
		Path:     absPath,
		Template: template,
		DryRun:   GetDryRun(),
	}

	if err := executeSeedSteps(absPath, template, &result); err != nil {
		return err
	}

	return outputSeedResult(result)
}

func executeSeedSteps(absPath, template string, result *seedResult) error {
	if err := seedCreateAgentsDirs(absPath, result); err != nil {
		return err
	}

	isGitRepo := isGitRepository(absPath)
	if err := setupGitProtection(absPath, isGitRepo); err != nil {
		return err
	}
	if err := ensureNestedAgentsGitignore(absPath); err != nil {
		return err
	}
	if err := initStorage(absPath); err != nil {
		return err
	}

	if err := seedCreateGoals(absPath, template, result); err != nil {
		return err
	}
	if err := seedCreateBootstrapLearning(absPath, template, result); err != nil {
		return err
	}
	return seedAppendClaudeMD(absPath, result)
}

func outputSeedResult(result seedResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if GetDryRun() {
		fmt.Println("Dry run complete. No files were created.")
		return nil
	}

	fmt.Printf("Seeded %s with template %q\n", result.Path, result.Template)
	if len(result.Created) > 0 {
		fmt.Println("\nCreated:")
		for _, f := range result.Created {
			fmt.Printf("  %s\n", f)
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Println("\nSkipped (already exist, use --force to overwrite):")
		for _, f := range result.Skipped {
			fmt.Printf("  %s\n", f)
		}
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  ao init --hooks    # Register session hooks")
	fmt.Println("  ao flywheel status # Verify flywheel health")
	return nil
}

func validateTemplateMapEntries(templates map[string]bool, templatesFS fs.FS) error {
	return lifecycle.ValidateTemplateMapEntries(templates, templatesFS)
}

// detectTemplate inspects project files to determine the best template.
func detectTemplate(root string) string {
	return detectTemplateFromProjectRoot(root)
}

// seedCreateAgentsDirs creates the .agents/ directory structure.
func seedCreateAgentsDirs(root string, result *seedResult) error {
	// Use the same directory list as ao init for consistency
	for _, dir := range agentsDirs {
		target := filepath.Join(root, dir)
		if GetDryRun() {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				fmt.Printf("[dry-run] Would create %s/\n", dir)
				result.Created = append(result.Created, dir+"/")
			} else {
				fmt.Printf("[dry-run] Already exists: %s/\n", dir)
				result.Skipped = append(result.Skipped, dir+"/")
			}
			continue
		}
		if err := os.MkdirAll(target, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
		result.Created = append(result.Created, dir+"/")
	}
	return nil
}

// seedCreateGoals creates GOALS.md using the goals init logic.
func seedCreateGoals(root string, template string, result *seedResult) error {
	goalsPath := filepath.Join(root, "GOALS.md")

	// Check if already exists
	if _, err := os.Stat(goalsPath); err == nil && !seedForce {
		if GetDryRun() {
			fmt.Println("[dry-run] Would skip GOALS.md (already exists)")
		}
		result.Skipped = append(result.Skipped, "GOALS.md")
		return nil
	}

	gf := buildSeedGoalFile(root, template)

	// Detect gates from project structure
	detectedGoals := detectGates(root)
	gf.Goals = append(gf.Goals, detectedGoals...)

	content := goals.RenderGoalsMD(gf)

	if GetDryRun() {
		fmt.Printf("[dry-run] Would create GOALS.md (template: %s, %d gates)\n", template, len(gf.Goals))
		result.Created = append(result.Created, "GOALS.md")
		return nil
	}

	if err := os.WriteFile(goalsPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write GOALS.md: %w", err)
	}
	result.Created = append(result.Created, "GOALS.md")
	return nil
}

// buildSeedGoalFile creates a GoalFile tailored to the template.
func buildSeedGoalFile(root string, template string) *goals.GoalFile {
	return lifecycle.BuildSeedGoalFile(root, template)
}

// seedCreateBootstrapLearning creates the initial learning artifact.
func seedCreateBootstrapLearning(root string, template string, result *seedResult) error {
	learningsDir := filepath.Join(root, ".agents", "learnings")
	dateStr := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("%s-seed-bootstrap.md", dateStr)
	learningPath := filepath.Join(learningsDir, fileName)
	relPath := filepath.Join(".agents/learnings", fileName)

	// Check if already exists
	if _, err := os.Stat(learningPath); err == nil && !seedForce {
		if GetDryRun() {
			fmt.Printf("[dry-run] Would skip %s (already exists)\n", relPath)
		}
		result.Skipped = append(result.Skipped, relPath)
		return nil
	}

	content := fmt.Sprintf(`# Learning: Project Seeded

**Date:** %s
**Type:** decision
**Source:** ao seed

## Context

Seeded on %s with template %s.

## Decision

Adopted AgentOps knowledge compounding workflow:
- .agents/ directory for session artifacts
- GOALS.md for fitness gates
- Knowledge flywheel via MEMORY.md and session hooks

## Next Steps

- Run `+"`ao init --hooks`"+` to register session hooks
- Knowledge compounds automatically — MEMORY.md updates after each session
- Run `+"`ao flywheel status`"+` to check flywheel health
`, dateStr, dateStr, template)

	if GetDryRun() {
		fmt.Printf("[dry-run] Would create %s\n", relPath)
		result.Created = append(result.Created, relPath)
		return nil
	}

	// Ensure learnings dir exists (should already from step 1, but be safe)
	if err := os.MkdirAll(learningsDir, 0700); err != nil {
		return fmt.Errorf("create learnings dir: %w", err)
	}

	if err := os.WriteFile(learningPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write bootstrap learning: %w", err)
	}
	result.Created = append(result.Created, relPath)
	return nil
}

// claudeMDSeedSection is the section appended to CLAUDE.md by ao seed.
const claudeMDSeedSection = lifecycle.ClaudeMDSeedSection
const claudeMDSeedMarker = lifecycle.ClaudeMDSeedMarker
const claudeMDSeedMarkerLegacy = lifecycle.ClaudeMDSeedMarkerLegacy

// templateConfig and templateConfigs aliases for backwards compatibility with tests.
type templateConfig = lifecycle.TemplateConfig

var templateConfigs = lifecycle.TemplateConfigs

// hasSeedMarker returns true if content contains the current or legacy seed marker.
func hasSeedMarker(content string) bool {
	return lifecycle.HasSeedMarker(content)
}

// findSeedMarker returns the marker string found in content (current or legacy), or empty string.
func findSeedMarker(content string) string {
	return lifecycle.FindSeedMarker(content)
}

// seedAppendClaudeMD appends the seed section to CLAUDE.md (creating it if absent).
func seedAppendClaudeMD(root string, result *seedResult) error {
	claudePath := filepath.Join(root, "CLAUDE.md")

	// Check if file exists and already has the seed section (current or legacy)
	if data, err := os.ReadFile(claudePath); err == nil {
		if hasSeedMarker(string(data)) {
			if !seedForce {
				if GetDryRun() {
					fmt.Println("[dry-run] Would skip CLAUDE.md (seed section already present)")
				}
				result.Skipped = append(result.Skipped, "CLAUDE.md (seed section)")
				return nil
			}
		}
	}

	if GetDryRun() {
		if _, err := os.Stat(claudePath); os.IsNotExist(err) {
			fmt.Println("[dry-run] Would create CLAUDE.md with seed section")
			result.Created = append(result.Created, "CLAUDE.md")
		} else {
			fmt.Println("[dry-run] Would append seed section to CLAUDE.md")
			result.Created = append(result.Created, "CLAUDE.md (seed section)")
		}
		return nil
	}

	// If file doesn't exist, create it with a header
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		dirName := filepath.Base(root)
		header := fmt.Sprintf("# %s\n", dirName)
		content := header + claudeMDSeedSection
		if err := os.WriteFile(claudePath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("create CLAUDE.md: %w", err)
		}
		result.Created = append(result.Created, "CLAUDE.md")
		return nil
	}

	// Read existing, check for marker, append if missing (or force)
	data, err := os.ReadFile(claudePath)
	if err != nil {
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	if hasSeedMarker(string(data)) && !seedForce {
		result.Skipped = append(result.Skipped, "CLAUDE.md (seed section)")
		return nil
	}

	// If forcing, remove old section before appending new one
	content := string(data)
	if marker := findSeedMarker(content); seedForce && marker != "" {
		// Remove the old seed section (from marker to next ## or end of file)
		idx := strings.Index(content, marker)
		before := content[:idx]
		after := content[idx+len(marker):]
		// Find next section header
		if nextIdx := strings.Index(after, "\n## "); nextIdx >= 0 {
			after = after[nextIdx:]
		} else {
			after = ""
		}
		content = strings.TrimRight(before, "\n") + "\n" + after
	}

	// Ensure trailing newline before appending
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	content += claudeMDSeedSection

	if err := os.WriteFile(claudePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("update CLAUDE.md: %w", err)
	}
	result.Created = append(result.Created, "CLAUDE.md (seed section)")
	return nil
}
