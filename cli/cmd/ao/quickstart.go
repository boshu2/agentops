package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:     "quick-start",
	Aliases: []string{"quickstart"},
	Short:   "Set up AgentOps in your project (5 minutes)",
	Long: `Initialize AgentOps in your current project.

This command:
  1. Creates .agents/ directory structure
  2. Optionally initializes beads (git-native issues)
  3. Creates starter knowledge pack
  4. Shows the software-factory operator lane

Examples:
  ao quick-start              # Full setup with beads
  ao quick-start --no-beads   # Skip beads initialization
  ao quick-start --minimal    # Just .agents/ structure`,
	RunE: runQuickstart,
}

var (
	noBeads bool
	minimal bool
)

type quickstartResult struct {
	Path      string                     `json:"path"`
	DryRun    bool                       `json:"dry_run"`
	Minimal   bool                       `json:"minimal"`
	NoBeads   bool                       `json:"no_beads"`
	Beads     string                     `json:"beads"`
	Readiness *lifecycle.ReadinessReport `json:"readiness"`
}

func init() {
	quickstartCmd.GroupID = "start"
	rootCmd.AddCommand(quickstartCmd)
	quickstartCmd.Flags().BoolVar(&noBeads, "no-beads", false, "Skip beads initialization")
	quickstartCmd.Flags().BoolVar(&minimal, "minimal", false, "Minimal setup (just directories)")
}

// quickstartBeadsStep handles step 3: beads initialization or skip.
func quickstartBeadsStep(cwd string) {
	if !noBeads {
		fmt.Println("\n━━━ STEP 3: Beads initialization ━━━")
		if err := initBeads(cwd); err != nil {
			fmt.Printf("  ⚠ Beads init skipped: %v\n", err)
			fmt.Println("  → You can run 'bd init' later to enable git-native issues")
		}
	} else {
		fmt.Println("\n━━━ STEP 3: Skipping beads (--no-beads) ━━━")
		fmt.Println("  → Issues will be tracked in .agents/tasks.json instead")
		createTasksFile(cwd)
	}
}

// quickstartClaudeMdStep handles step 4: create CLAUDE.md if missing.
func quickstartClaudeMdStep(cwd string) {
	fmt.Println("\n━━━ STEP 4: Project configuration ━━━")
	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		if err := createProjectClaudeMd(cwd); err != nil {
			fmt.Printf("  ⚠ Warning: %v\n", err)
		} else {
			fmt.Println("  ✓ Created CLAUDE.md (project instructions)")
		}
	} else {
		fmt.Println("  ✓ CLAUDE.md already exists")
	}
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	jsonMode := GetOutput() == "json"
	opts := lifecycle.ReadinessOptions{
		Template: detectTemplate(cwd),
		DryRun:   GetDryRun(),
		Minimal:  minimal,
		NoBeads:  noBeads,
	}

	if GetDryRun() {
		return runQuickstartDryRun(cwd, opts)
	}

	if !jsonMode {
		fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                 AGENTOPS QUICK START                            ║
║           Setting up your project for knowledge compounding      ║
╚══════════════════════════════════════════════════════════════════╝`)
		fmt.Printf("Project: %s\n\n", cwd)
	}

	if minimal {
		return runQuickstartMinimal(cwd, opts, jsonMode)
	}

	return runQuickstartFull(cwd, opts, jsonMode)
}

func runQuickstartDryRun(cwd string, opts lifecycle.ReadinessOptions) error {
	report, err := lifecycle.PlanRepoSeed(cwd, opts)
	if err != nil {
		return err
	}
	return outputQuickstartResult(quickstartResult{
		Path:      cwd,
		DryRun:    true,
		Minimal:   minimal,
		NoBeads:   noBeads,
		Beads:     beadsReadinessStatus(cwd, noBeads),
		Readiness: report,
	})
}

func runQuickstartMinimal(cwd string, opts lifecycle.ReadinessOptions, jsonMode bool) error {
	if !jsonMode {
		fmt.Println("━━━ STEP 1: Creating .agents/ structure ━━━")
	}
	if err := createQuickstartDirs(cwd); err != nil {
		return err
	}
	report, err := lifecycle.InspectRepoReadiness(cwd, opts)
	if err != nil {
		return err
	}
	if jsonMode {
		return outputQuickstartResult(quickstartResult{
			Path:      cwd,
			Minimal:   true,
			NoBeads:   noBeads,
			Beads:     "skipped-minimal",
			Readiness: report,
		})
	}
	fmt.Println("\n✓ Minimal setup complete!")
	printReadinessSummary(report)
	showNextSteps(false)
	return nil
}

func runQuickstartFull(cwd string, opts lifecycle.ReadinessOptions, jsonMode bool) error {
	if !jsonMode {
		fmt.Println("━━━ STEP 1: Applying core repo seed ━━━")
	}
	claudePath := filepath.Join(cwd, "CLAUDE.md")
	claudeAlreadyExisted, err := ensureProjectClaudeMd(cwd, claudePath)
	if err != nil {
		return err
	}
	report, err := lifecycle.ApplyRepoSeed(cwd, opts)
	if err != nil {
		return err
	}
	if err := setupGitProtection(cwd, isGitRepository(cwd)); err != nil {
		return err
	}
	if !jsonMode {
		fmt.Println("  ✓ Core readiness seed applied")
		fmt.Println("\n━━━ STEP 2: Creating starter knowledge pack ━━━")
	}
	if err := createStarterPack(cwd); err != nil {
		if !jsonMode {
			fmt.Printf("  ⚠ Warning: %v\n", err)
		}
	}

	beadsStatus := beadsReadinessStatus(cwd, noBeads)
	if jsonMode {
		return outputQuickstartResult(quickstartResult{
			Path:      cwd,
			Minimal:   false,
			NoBeads:   noBeads,
			Beads:     beadsStatus,
			Readiness: report,
		})
	}
	finalizeQuickstartFull(cwd, claudePath, claudeAlreadyExisted, report)
	return nil
}

func ensureProjectClaudeMd(cwd, claudePath string) (bool, error) {
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		if err := createProjectClaudeMd(cwd); err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func finalizeQuickstartFull(cwd, claudePath string, claudeAlreadyExisted bool, report *lifecycle.ReadinessReport) {
	quickstartBeadsStep(cwd)
	fmt.Println("\n━━━ STEP 4: Project configuration ━━━")
	if claudeAlreadyExisted {
		fmt.Println("  ✓ CLAUDE.md already exists")
	}
	if lifecycle.HasSeedMarker(readFileBestEffort(claudePath)) {
		fmt.Println("  ✓ CLAUDE.md has AgentOps instructions")
	} else {
		fmt.Println("  ⚠ CLAUDE.md missing AgentOps instructions")
	}

	fmt.Println("\n━━━ SETUP COMPLETE ━━━")
	printReadinessSummary(report)
	showNextSteps(!noBeads)
}

func outputQuickstartResult(result quickstartResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if result.DryRun {
		fmt.Println("Dry run complete. No files were created.")
	}
	if result.Readiness != nil {
		printReadinessSummary(result.Readiness)
	}
	return nil
}

func createQuickstartDirs(cwd string) error {
	statePaths, err := lifecycle.ResolveReadinessPaths(cwd)
	if err != nil {
		return err
	}
	for _, dir := range append(append([]string{}, lifecycle.CoreAgentSubdirs...), lifecycle.CoreStorageSubdirs...) {
		path := filepath.Join(statePaths.AgentsDir, dir)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
		if GetOutput() != "json" {
			fmt.Printf("  ✓ %s/\n", filepath.ToSlash(filepath.Join(".agents", dir)))
		}
	}
	return nil
}

func printReadinessSummary(report *lifecycle.ReadinessReport) {
	fmt.Println("\nAgentOps repo readiness")
	for _, layer := range []lifecycle.ReadinessLayer{
		lifecycle.LayerCore,
		lifecycle.LayerGoals,
		lifecycle.LayerInstructions,
		lifecycle.LayerTracking,
		lifecycle.LayerProduct,
		lifecycle.LayerProgram,
		lifecycle.LayerSchedule,
	} {
		present, total, action := readinessLayerStatus(report, layer)
		status := "ready"
		if present < total {
			status = "next: " + action
		}
		if total == 0 {
			continue
		}
		fmt.Printf("  %-13s %s (%d/%d)\n", string(layer)+":", status, present, total)
	}
	fmt.Println("\nNext: pick a golden path below, or run /rpi \"your first objective\"")
}

func readinessLayerStatus(report *lifecycle.ReadinessReport, layer lifecycle.ReadinessLayer) (int, int, string) {
	var present, total int
	action := ""
	for _, item := range report.Items {
		if item.Layer != layer {
			continue
		}
		total++
		if item.Present {
			present++
			continue
		}
		if action == "" {
			action = item.Action
		}
	}
	if action == "" {
		action = "already configured"
	}
	return present, total, action
}

func beadsReadinessStatus(cwd string, disabled bool) string {
	if disabled {
		return "disabled"
	}
	if _, err := os.Stat(filepath.Join(cwd, ".beads")); err == nil {
		return "ready"
	}
	if GetOutput() == "json" {
		return "skipped-json"
	}
	return "pending"
}

func readFileBestEffort(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func createStarterPack(cwd string) error {
	// Create a few starter patterns that are universally useful
	patterns := map[string]string{
		".agents/patterns/context-boundaries.md": `# Pattern: Fresh Context Per Phase

**Tier:** 2 (Pattern)
**Source:** AgentOps multi-epic post-mortem

## Problem

Long sessions accumulate errors. Context pollution causes drift.

## Solution

Fresh Claude session for each RPI phase:
- /research → new session
- /plan → new session
- /implement → new session
- /post-mortem → new session

## The 40% Rule

| Context % | Success Rate |
|-----------|--------------|
| <40%      | 98%          |
| 40-60%    | ~50%         |
| >60%      | ~1%          |

At 35% context, checkpoint and consider new session.
`,
		".agents/patterns/pre-mortem-first.md": `# Pattern: Pre-Mortem Before Implementation

**Tier:** 2 (Pattern)
**Source:** Knowledge Flywheel post-mortem (2026-01-22)

## Problem

Implementation failures are expensive. Debugging takes longer than preventing.

## Solution

Run /pre-mortem on P0/P1 work BEFORE /crank:

` + "```bash" + `
/pre-mortem .agents/specs/my-feature.md
# Review findings
# Then implement
/crank
` + "```" + `

## Evidence

Pre-mortem caught 6 critical issues before implementation:
- API group mismatches
- Path resolution errors
- Migration assumptions
- Schema drift

## When to Skip

- Bug fixes (already understood)
- Single-file changes (<50 lines)
- P2/P3 priority work
`,
		".agents/learnings/session-hygiene.md": `# Learning: Session Hygiene

**Date:** Starter Pack
**Tier:** 1 (Learning)

## Key Practices

1. **Always push before saying done**
   - Work that isn't pushed didn't happen
   - ` + "`git push`" + ` is the final step

2. **Run /post-mortem after epics**
   - Captures learnings for the flywheel
   - Creates patterns from experience

3. **Check Smart Connections before starting**
   - Search for prior art: ` + "`mcp__smart-connections-work__lookup`" + `
   - Don't reinvent what exists

4. **Use beads for state**
   - ` + "`bd ready`" + ` shows unblocked work
   - beads auto-sync issue writes; use ` + "`bd export -o backup.jsonl`" + ` for a manual snapshot
`,
	}

	statePaths, err := lifecycle.ResolveReadinessPaths(cwd)
	if err != nil {
		return err
	}
	for path, content := range patterns {
		fullPath := filepath.Join(cwd, path)
		if strings.HasPrefix(path, ".agents/") {
			fullPath = filepath.Join(statePaths.AgentsDir, strings.TrimPrefix(path, ".agents/"))
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
			return err
		}
		if GetOutput() != "json" {
			fmt.Printf("  ✓ %s\n", path)
		}
	}

	return nil
}

func initBeads(cwd string) error {
	// Check if beads is available
	if _, err := exec.LookPath("bd"); err != nil {
		return fmt.Errorf("bd command not found (install: brew install beads)")
	}

	// Check if already initialized
	beadsDir := filepath.Join(cwd, ".beads")
	if _, err := os.Stat(beadsDir); err == nil {
		fmt.Println("  ✓ Beads already initialized")
		return nil
	}

	// Determine prefix from directory name
	dirName := filepath.Base(cwd)
	prefix := strings.ToLower(dirName)
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}

	fmt.Printf("  Initializing beads with prefix '%s'...\n", prefix)

	// Ask for confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("  Use prefix '%s'? [Y/n]: ", prefix)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "n" || response == "no" {
		fmt.Print("  Enter prefix: ")
		prefix, _ = reader.ReadString('\n')
		prefix = strings.TrimSpace(prefix)
	}

	// Run bd init
	cmd := exec.Command("bd", "init", "--prefix", prefix)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd init failed: %s", string(output))
	}

	fmt.Printf("  ✓ Beads initialized with prefix '%s'\n", prefix)
	return nil
}

func createTasksFile(cwd string) {
	statePaths, err := lifecycle.ResolveReadinessPaths(cwd)
	tasksPath := filepath.Join(cwd, ".agents/tasks.json")
	if err == nil {
		tasksPath = filepath.Join(statePaths.AgentsDir, "tasks.json")
	}
	content := `{
  "tasks": [],
  "note": "Beads-optional mode. Use 'bd init' to enable full git-native issues."
}
`
	//nolint:errcheck // quickstart setup, errors shown implicitly by missing output
	os.WriteFile(tasksPath, []byte(content), 0600) // #nosec G104
	if GetOutput() != "json" {
		fmt.Println("  ✓ Created .agents/tasks.json (beads-optional mode)")
	}
}

func createProjectClaudeMd(cwd string) error {
	dirName := filepath.Base(cwd)
	content := fmt.Sprintf(`# %s

## Quick Start

`+"```bash"+`
ao quick-start        # Repair or inspect the repo seed
bd ready              # See unblocked issues when beads is enabled
/rpi "objective"      # Run discovery, implementation, validation
`+"```"+`

## Session Protocol

`+"```bash"+`
# Start
ao status             # Check AgentOps state
bd ready              # Find available work

# End
git add .
git commit -m "..."
git push              # NEVER stop before pushing
`+"```"+`

## JIT Loading

| Working On | Load |
|------------|------|
| Research | .agents/research/ |
| Implementation | Check existing patterns first |
| Debugging | .agents/learnings/ |

`, dirName) + lifecycle.ClaudeMDSeedSection

	return os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte(content), 0600)
}

func showNextSteps(hasBeads bool) {
	fmt.Print(`
═══════════════════════════════════════════════════════════════════
                          GOLDEN PATHS
═══════════════════════════════════════════════════════════════════
`)

	if hasBeads {
		fmt.Println(`  1. First validated change:
     $ ao factory start --goal "your first objective"
     > /rpi "your first objective"

  2. Tracked work:
     $ bd ready
     $ bd create "My first task"

  3. Terminal-native lifecycle:
     $ ao rpi phased "your first objective"
     $ ao rpi status

  4. Close the learning loop:
     > /validation
     $ ao codex stop  # Codex hookless fallback only`)
	} else {
		fmt.Println(`  1. First validated change:
     $ ao factory start --goal "your first objective"
     > /rpi "your first objective"

  2. Start your agent in this repo:
     > /quickstart
     > /rpi "your first objective"

  3. Terminal-native lifecycle:
     $ ao rpi phased "your first objective"
     $ ao rpi status

  4. Add tracked execution when ready:
     $ bd init
     $ bd create "My first task"`)
	}

	fmt.Print(`
  Success signal: the run leaves validation evidence and reusable context in .agents/

═══════════════════════════════════════════════════════════════════

  "Stateful environment. Stateless agents. One explicit operator lane."

═══════════════════════════════════════════════════════════════════
`)
}
