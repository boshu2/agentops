// practices: [pragmatic-programmer, agile-manifesto]
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
  4. Shows one live operating-loop path
  5. Ends one command away from a first verdict: it readies the provenance
     ledger path, checks a reviewer CLI is reachable (the same check as
     'ao doctor'), and prints the exact 'ao verify' command to run next

Examples:
  ao quick-start              # One-screen setup, tailored to this environment
  ao quick-start --no-beads   # Skip beads initialization
  ao quick-start --minimal    # Just .agents/ structure
  ao quick-start --verbose    # Full step-by-step long form`,
	RunE: runQuickstart,
}

var (
	noBeads           bool
	minimal           bool
	quickstartVerbose bool
)

// quickstartNextSkill is the single next action the diet output ends on: the
// skill that shapes the user's first capability. It is a skill path (always
// resolvable), never a removed cobra command — /rpi is a tombstone in the
// journey guards, so /plan is the canonical single Next.
const quickstartNextSkill = "/plan"

// quickstartDocsLink is the one docs pointer the diet output prints.
const quickstartDocsLink = "docs/first-value-path.md"

type quickstartResult struct {
	Path      string                     `json:"path"`
	DryRun    bool                       `json:"dry_run"`
	Minimal   bool                       `json:"minimal"`
	NoBeads   bool                       `json:"no_beads"`
	Beads     string                     `json:"beads"`
	Readiness *lifecycle.ReadinessReport `json:"readiness"`
	// FirstVerdict is the final quick-start step: ledger readiness + reviewer
	// reachability + the exact next command. Nil on --dry-run (no writes, no
	// probes).
	FirstVerdict *firstVerdictInfo `json:"first_verdict,omitempty"`
}

func init() {
	quickstartCmd.GroupID = "start"
	rootCmd.AddCommand(quickstartCmd)
	quickstartCmd.Flags().BoolVar(&noBeads, "no-beads", false, "Skip beads initialization")
	quickstartCmd.Flags().BoolVar(&minimal, "minimal", false, "Minimal setup (just directories)")
	quickstartCmd.Flags().BoolVar(&quickstartVerbose, "verbose", false, "Full step-by-step long form (default is a one-screen diet)")
}

// quickstartBeadsStep handles step 3: beads initialization or skip.
func quickstartBeadsStep(cwd string) error {
	return quickstartBeadsStepWithApp(cwd, NewApp())
}

func quickstartBeadsStepWithApp(cwd string, app *App) error {
	return quickstartBeadsStepVerbose(cwd, app, true)
}

// quickstartBeadsStepVerbose runs the beads init/skip side effects. When
// verbose is false (the diet default) the STEP divider chatter is suppressed;
// the substantive tracker readiness is reported by the readiness checklist
// instead.
func quickstartBeadsStepVerbose(cwd string, app *App, verbose bool) error {
	if !noBeads {
		if verbose {
			fmt.Println("\n━━━ STEP 3: Beads initialization ━━━")
		}
		if err := initBeadsWithApp(cwd, app); err != nil {
			return fmt.Errorf("tracker initialization failed: %w", err)
		}
	} else {
		if verbose {
			fmt.Println("\n━━━ STEP 3: Skipping beads (--no-beads) ━━━")
			fmt.Println("  → Issues will be tracked in .agents/tasks.json instead")
		}
		createTasksFile(cwd)
	}
	return nil
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

	// The multi-line ASCII banner is long-form only. The diet default opens on
	// a single completion header printed once the work is done (a fresh install
	// should read as one screen, not three).
	if !jsonMode && quickstartVerbose {
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

	return runQuickstartFull(cwd, opts, jsonMode, AppFromContext(cmd.Context()))
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
	if !jsonMode && quickstartVerbose {
		fmt.Println("━━━ STEP 1: Creating .agents/ structure ━━━")
	}
	if err := createQuickstartDirs(cwd); err != nil {
		return err
	}
	report, err := lifecycle.InspectRepoReadiness(cwd, opts)
	if err != nil {
		return err
	}
	firstVerdict := prepareFirstVerdict()
	if jsonMode {
		return outputQuickstartResult(quickstartResult{
			Path:         cwd,
			Minimal:      true,
			NoBeads:      noBeads,
			Beads:        "skipped-minimal",
			Readiness:    report,
			FirstVerdict: firstVerdict,
		})
	}
	fmt.Println("\n✓ Minimal setup complete!")
	if quickstartVerbose {
		printReadinessSummary(report)
		showNextSteps(false)
		printFirstVerdictStep(firstVerdict)
		return nil
	}
	printQuickstartDietBody(cwd, []createdItem{
		{Name: ".agents/", Note: "agent workspace (dirs + storage)"},
	}, report, firstVerdict)
	return nil
}

func runQuickstartFull(cwd string, opts lifecycle.ReadinessOptions, jsonMode bool, app *App) error {
	if !jsonMode && quickstartVerbose {
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
	if !jsonMode && quickstartVerbose {
		fmt.Println("  ✓ Core readiness seed applied")
		fmt.Println("\n━━━ STEP 2: Creating starter knowledge pack ━━━")
	}
	if err := createStarterPack(cwd); err != nil {
		if !jsonMode {
			fmt.Printf("  ⚠ Warning: %v\n", err)
		}
	}

	beadsStatus := beadsReadinessStatus(cwd, noBeads)
	firstVerdict := prepareFirstVerdict()
	if jsonMode {
		return outputQuickstartResult(quickstartResult{
			Path:         cwd,
			Minimal:      false,
			NoBeads:      noBeads,
			Beads:        beadsStatus,
			Readiness:    report,
			FirstVerdict: firstVerdict,
		})
	}
	if quickstartVerbose {
		return finalizeQuickstartFull(cwd, claudePath, claudeAlreadyExisted, report, firstVerdict, app)
	}
	return finalizeQuickstartDiet(cwd, claudeAlreadyExisted, report, firstVerdict, app)
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

func finalizeQuickstartFull(cwd, claudePath string, claudeAlreadyExisted bool, report *lifecycle.ReadinessReport, firstVerdict *firstVerdictInfo, app *App) error {
	if err := quickstartBeadsStepWithApp(cwd, app); err != nil {
		return err
	}
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
	showNextSteps(beadsReadinessStatus(cwd, noBeads) == "ready")
	printFirstVerdictStep(firstVerdict)
	return nil
}

// createdItem is one line in the diet "Created:" summary — the artifact name
// and a short note (template files are marked "template — edit me").
type createdItem struct {
	Name string
	Note string
}

// finalizeQuickstartDiet renders the one-screen default output: a completion
// header, a created-files summary, a readiness checklist, the environment-
// tailored golden paths, exactly one Next action, and the tightened first-
// verdict close. Long form lives behind --verbose (finalizeQuickstartFull).
func finalizeQuickstartDiet(cwd string, claudeAlreadyExisted bool, report *lifecycle.ReadinessReport, firstVerdict *firstVerdictInfo, app *App) error {
	if err := quickstartBeadsStepVerbose(cwd, app, false); err != nil {
		return err
	}
	fmt.Println("\n━━━ SETUP COMPLETE ━━━")
	claudeNote := "project instructions (+ AgentOps section)"
	if claudeAlreadyExisted {
		claudeNote = "AgentOps section appended"
	}
	printQuickstartDietBody(cwd, []createdItem{
		{Name: ".agents/", Note: "agent workspace (dirs + storage)"},
		{Name: "GOALS.md", Note: "template — edit me"},
		{Name: "CLAUDE.md", Note: claudeNote},
		{Name: ".agents/patterns/", Note: "3 starter patterns"},
	}, report, firstVerdict)
	return nil
}

// printQuickstartDietBody prints the shared one-screen body (created summary,
// readiness checklist, golden paths, single Next action, first verdict). The
// caller prints its own completion header first.
func printQuickstartDietBody(cwd string, created []createdItem, report *lifecycle.ReadinessReport, firstVerdict *firstVerdictInfo) {
	fmt.Printf("Project: %s\n\n", cwd)
	printCreatedSummary(created)
	printReadinessChecklist(report)
	// Tracker paths render only when a ledger actually exists on disk — the
	// --no-beads flag default says nothing about whether init ran (--minimal
	// skips it), and a resolvable br binary with no ledger makes `br ready`
	// a dead command, not a golden path.
	renderGoldenPaths(beadsReadinessStatus(cwd, noBeads) == "ready")
	fmt.Printf("\nNext: run %s to shape your first capability  ·  %s\n", quickstartNextSkill, quickstartDocsLink)
	printFirstVerdictStep(firstVerdict)
}

func printCreatedSummary(items []createdItem) {
	if len(items) == 0 {
		return
	}
	fmt.Println("Created:")
	for _, it := range items {
		fmt.Printf("  %-18s %s\n", it.Name, it.Note)
	}
	fmt.Println()
}

// readinessLayerLabel is the short human label for a readiness layer in the
// diet checklist.
func readinessLayerLabel(layer lifecycle.ReadinessLayer) string {
	switch layer {
	case lifecycle.LayerCore:
		return "core scaffolding"
	case lifecycle.LayerGoals:
		return "GOALS.md"
	case lifecycle.LayerInstructions:
		return "CLAUDE.md AgentOps section"
	case lifecycle.LayerTracking:
		return "beads tracker"
	case lifecycle.LayerProduct:
		return "PRODUCT.md"
	default:
		return string(layer)
	}
}

func printReadinessChecklist(report *lifecycle.ReadinessReport) {
	fmt.Println("Readiness:")
	for _, layer := range []lifecycle.ReadinessLayer{
		lifecycle.LayerCore,
		lifecycle.LayerGoals,
		lifecycle.LayerInstructions,
		lifecycle.LayerTracking,
		lifecycle.LayerProduct,
	} {
		present, total, action := readinessLayerStatus(report, layer)
		if total == 0 {
			continue
		}
		if present >= total {
			fmt.Printf("  [x] %s\n", readinessLayerLabel(layer))
		} else {
			fmt.Printf("  [ ] %s — %s\n", readinessLayerLabel(layer), action)
		}
	}
	fmt.Println()
}

// goldenPath is one environment-tailored "run this" path. It is printed only
// when its Binary resolves on PATH (skill paths carry an empty Binary and are
// always fine). When an absent tool has an Enable hint, at most one short line
// is printed instead of the full path.
type goldenPath struct {
	Run           string // the run-this command, e.g. "br ready"
	Binary        string // binary that must resolve on PATH; "" = always shown
	Desc          string
	RequiresBeads bool   // only participate when tracking is enabled
	Enable        string // one-line enable hint when Binary is absent; "" = omit silently
}

// quickstartGoldenPaths is the environment-tailored path catalog. Each entry
// is filtered by exec.LookPath at render time, so a fresh install only ever
// sees the paths that actually work in its shell.
func quickstartGoldenPaths() []goldenPath {
	return []goldenPath{
		{
			Run:           "br ready",
			Binary:        "br",
			Desc:          "find unblocked tracked work",
			RequiresBeads: true,
			Enable:        "br not found — install beads_rust to track work: https://github.com/Dicklesworthstone/beads_rust",
		},
		{Run: "gt log", Binary: "gt", Desc: "inspect your stacked branches"},
		{Run: `codex exec "<task>"`, Binary: "codex", Desc: "dispatch a Codex worker"},
		{Run: `agy -p "<task>"`, Binary: "agy", Desc: "run an AGY (Gemini) review"},
	}
}

// binaryResolves reports whether name is an executable on the current PATH.
func binaryResolves(name string) bool {
	if name == "" {
		return true
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// renderGoldenPaths prints only the golden paths whose first command resolves
// in this environment; absent tools collapse to at most one enable line.
func renderGoldenPaths(hasBeads bool) {
	var runLines, enableLines []string
	for _, p := range quickstartGoldenPaths() {
		if p.RequiresBeads && !hasBeads {
			continue
		}
		if binaryResolves(p.Binary) {
			runLines = append(runLines, fmt.Sprintf("  $ %-20s %s", p.Run, p.Desc))
			continue
		}
		if p.Enable != "" {
			enableLines = append(enableLines, "  · "+p.Enable)
		}
	}
	if len(runLines) == 0 && len(enableLines) == 0 {
		return
	}
	fmt.Println("Paths that work in this environment:")
	for _, l := range runLines {
		fmt.Println(l)
	}
	for _, l := range enableLines {
		fmt.Println(l)
	}
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
		if quickstartVerbose && GetOutput() != "json" {
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
	fmt.Println("\nNext: follow the live operating-loop path below.")
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
	if _, err := os.Stat(filepath.Join(cwd, "_beads")); err == nil {
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

Use a fresh context for each operating-loop phase and persist the handoff on disk.

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

Run a pre-mortem on P0/P1 work before implementation:

` + "```bash" + `
ao session bootstrap
# Review findings
# Then implement through the declared skill contract
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

2. **Run a post-mortem after epics**
   - Captures learnings for the flywheel
   - Creates patterns from experience

3. **Check Smart Connections before starting**
   - Search for prior art: ` + "`mcp__smart-connections-work__lookup`" + `
   - Don't reinvent what exists

4. **Use beads for state**
   - ` + "`br ready`" + ` shows unblocked work
   - br is git-JSONL-backed (` + "`_beads/issues.jsonl`" + `); run ` + "`br sync --flush-only`" + ` to flush a snapshot
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
		if quickstartVerbose && GetOutput() != "json" {
			fmt.Printf("  ✓ %s\n", path)
		}
	}

	return nil
}

func initBeads(cwd string) error {
	return initBeadsWithApp(cwd, NewApp())
}

func initBeadsWithApp(cwd string, app *App) error {
	resolution, err := resolveTracker(cwd, os.Environ())
	if err != nil {
		return err
	}
	binary, err := app.LookPath(resolution.Tracker)
	if err != nil {
		return fmt.Errorf("selected tracker %s command not found: %w", resolution.Tracker, err)
	}
	resolution.Binary = binary

	// Check if already initialized
	if _, err := os.Stat(resolution.LedgerDir); err == nil {
		fmt.Printf("  ✓ %s tracker already initialized\n", resolution.Tracker)
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

	// Run the selected tracker. Availability and execution use the same resolved
	// backend so an explicit bd selection can never be preflighted as br.
	cmd := app.ExecCommand(resolution.Binary, "init", "--prefix", prefix) // #nosec G204 -- selected br|bd binary.
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s init failed: %s", resolution.Tracker, string(output))
	}

	fmt.Printf("  ✓ %s tracker initialized with prefix '%s'\n", resolution.Tracker, prefix)
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
  "note": "Beads-optional mode. Use 'br init' to enable full git-native issues."
}
`
	// ag-chvc: surface write failures instead of silently dropping them; only claim
	// success after the write actually lands.
	if err := os.WriteFile(tasksPath, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ could not write %s: %v\n", tasksPath, err)
	} else if quickstartVerbose && GetOutput() != "json" {
		fmt.Println("  ✓ Created .agents/tasks.json (beads-optional mode)")
	}
}

func createProjectClaudeMd(cwd string) error {
	dirName := filepath.Base(cwd)
	content := fmt.Sprintf(`# %s

## Quick Start

`+"```bash"+`
ao quick-start        # Repair or inspect the repo seed
ao session bootstrap  # Orient the agent in this repository
ao beads exec ready   # See unblocked issues when tracking is enabled
`+"```"+`

## Session Protocol

`+"```bash"+`
# Start
ao status             # Check AgentOps state
ao beads exec ready   # Find available work through the selected tracker

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

type quickstartJourneyStep struct {
	Title    string
	Commands []string
}

func quickstartJourney(hasBeads bool) []quickstartJourneyStep {
	steps := []quickstartJourneyStep{{
		Title:    "Orient the agent",
		Commands: []string{"ao session bootstrap"},
	}}
	if hasBeads {
		steps = append(steps, quickstartJourneyStep{
			Title:    "Select tracked work",
			Commands: []string{"ao beads tracker", "ao beads exec ready"},
		})
	} else {
		steps = append(steps, quickstartJourneyStep{
			Title:    "Inspect repository readiness",
			Commands: []string{"ao status"},
		})
	}
	steps = append(steps, quickstartJourneyStep{
		Title:    "Prove the committed change",
		Commands: []string{firstVerdictCommand},
	})
	return steps
}

func showNextSteps(hasBeads bool) {
	fmt.Print(`
═══════════════════════════════════════════════════════════════════
                           LIVE PATH
═══════════════════════════════════════════════════════════════════
`)
	for i, step := range quickstartJourney(hasBeads) {
		fmt.Printf("  %d. %s:\n", i+1, step.Title)
		for _, command := range step.Commands {
			// The final verdict is rendered once, with readiness information, by
			// printFirstVerdictStep. Keeping it in the typed journey makes the
			// terminal contract explicit without printing two competing paths.
			if command == firstVerdictCommand {
				fmt.Println("     (the final step below)")
				continue
			}
			fmt.Printf("     $ %s\n", command)
		}
		fmt.Println()
	}

	fmt.Print(`
  Success signal: the run leaves validation evidence and reusable context in .agents/

═══════════════════════════════════════════════════════════════════

  "Stateful environment. Stateless agents. One explicit operator lane."

═══════════════════════════════════════════════════════════════════
`)
}
