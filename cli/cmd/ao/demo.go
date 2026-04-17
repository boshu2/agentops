package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Interactive demo showing AgentOps value in 5 minutes",
	Long: `Run an interactive demonstration of AgentOps capabilities.

This command walks you through the core concepts:
  1. The Knowledge Flywheel (how sessions compound)
  2. The Brownian Ratchet (chaos + filter = progress)
  3. Git-native state (beads for issue tracking)
  4. Quality gates (/pre-mortem, /vibe)

No setup required - just run and see the value.

Examples:
  ao demo              # Interactive walkthrough
  ao demo --quick      # 2-minute overview
  ao demo --concepts   # Just explain concepts`,
	RunE: runDemo,
}

var (
	demoQuick    bool
	demoConcepts bool
	// demoStepDelay is the pause between terminal "cards" in quickDemo.
	// Exposed at package scope so tests can zero it out — the 500 ms is for
	// human pacing in a live terminal and has no behavioral meaning.
	demoStepDelay = 500 * time.Millisecond
)

func init() {
	demoCmd.GroupID = "start"
	rootCmd.AddCommand(demoCmd)
	demoCmd.Flags().BoolVar(&demoQuick, "quick", false, "2-minute quick overview")
	demoCmd.Flags().BoolVar(&demoConcepts, "concepts", false, "Just explain core concepts")
}

func runDemo(cmd *cobra.Command, args []string) error {
	if demoConcepts {
		return showConcepts()
	}
	if demoQuick {
		return quickDemo()
	}
	return interactiveDemo()
}

func showConcepts() error {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                    AGENTOPS CORE CONCEPTS                          ║
╚══════════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────────┐
│  1. THE KNOWLEDGE FLYWHEEL                                       │
│                                                                  │
│     Session N → /post-mortem → Learnings                         │
│                                    ↓                             │
│     Session N+1 ← Smart Connections ← Indexed                    │
│                                    ↓                             │
│                              COMPOUNDS                           │
│                                                                  │
│  Others start fresh. You get smarter every session.             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  2. THE BROWNIAN RATCHET                                         │
│                                                                  │
│     CHAOS (polecats) → FILTER (/vibe) → RATCHET (merged)        │
│                                                                  │
│  Multiple parallel attempts. Quality gates filter bad ones.      │
│  Progress locks in permanently. Can't go backward.              │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  3. GIT-NATIVE STATE (Beads)                                     │
│                                                                  │
│     bd create "Fix auth bug"     # Create issue                  │
│     bd ready                     # What's unblocked?             │
│     bd close at-1234             # Done                          │
│                                                                  │
│  Issues live in git. Survive any tool failure. No vendor lock.  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  4. QUALITY GATES                                                │
│                                                                  │
│     /pre-mortem  → Simulate failures BEFORE implementing        │
│     /vibe        → 8-aspect validation AFTER implementing       │
│     /crank       → Autonomous execution until epic done         │
│                                                                  │
│  Prevention > reaction. Quality at every step.                  │
└─────────────────────────────────────────────────────────────────┘

Next steps:
  ao demo --quick    # See it in action
  ao quick-start     # Set up your first project`)
	return nil
}

func quickDemo() error {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                    AGENTOPS QUICK DEMO (2 min)                     ║
╚══════════════════════════════════════════════════════════════════╝`)

	steps := []struct {
		title   string
		cmd     string
		explain string
	}{
		{
			"1. Create a task (beads)",
			"bd create \"Add user authentication\"",
			"Issue created in git. No external service needed.",
		},
		{
			"2. See what's ready to work on",
			"bd ready",
			"Shows unblocked issues. Dependencies are automatic.",
		},
		{
			"3. Start autonomous execution",
			"/crank",
			"Claude works until the epic is DONE. No babysitting.",
		},
		{
			"4. Extract learnings",
			"/post-mortem",
			"Captures what worked. Feeds the flywheel.",
		},
		{
			"5. Next session searches prior learnings",
			"mcp__smart-connections-work__lookup",
			"Your knowledge compounds. Others start fresh.",
		},
	}

	for _, step := range steps {
		fmt.Printf("┌─ %s\n", step.title)
		fmt.Printf("│  $ %s\n", step.cmd)
		fmt.Printf("│  → %s\n", step.explain)
		fmt.Print("└─\n")
		if demoStepDelay > 0 {
			time.Sleep(demoStepDelay)
		}
	}

	fmt.Println(`
═══════════════════════════════════════════════════════════════════

THE DIFFERENCE:

  ┌────────────────────┬────────────────────┐
  │   COMPETITORS      │     AGENTOPS              │
  ├────────────────────┼────────────────────┤
  │ Start fresh        │ Knowledge compounds │
  │ Need infrastructure│ Pure git           │
  │ React to failures  │ Prevent failures   │
  │ 0-1 quality gates  │ 4-layer ratchets   │
  └────────────────────┴────────────────────┘

Next: ol quick-start`)
	return nil
}

func interactiveDemo() error {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                AGENTOPS INTERACTIVE DEMO                           ║
║           "Problem in. Value out. Intelligence compounds."        ║
╚══════════════════════════════════════════════════════════════════╝

This demo will:
  ✓ Create a sample .agents/ structure
  ✓ Show the phased RPI workflow (Discovery → Implementation → Validation)
  ✓ Demonstrate the knowledge flywheel
  ✓ Explain quality gates

Press Enter to continue...`)

	//nolint:errcheck // demo interactive prompt, ignore input errors
	fmt.Scanln() // #nosec G104

	// Create demo directories
	homeDir, _ := os.UserHomeDir()
	demoDir := filepath.Join(homeDir, ".agentops-demo")

	dirs := []string{
		filepath.Join(demoDir, ".agents/research"),
		filepath.Join(demoDir, ".agents/learnings"),
		filepath.Join(demoDir, ".agents/patterns"),
		filepath.Join(demoDir, ".agents/specs"),
	}

	fmt.Println("\n━━━ STEP 1: Creating knowledge structure ━━━")
	for _, dir := range dirs {
		//nolint:errcheck // demo code, errors shown implicitly by missing output
		os.MkdirAll(dir, 0750) // #nosec G104
		fmt.Printf("  ✓ Created %s\n", dir)
	}

	// Create sample learning
	learningPath := filepath.Join(demoDir, ".agents/learnings/demo-learning.md")
	learningContent := `# Learning: Context Compounds

**Date:** ` + time.Now().Format("2006-01-02") + `
**Source:** AgentOps Demo
**Tier:** 1 (Learning)

## Insight

Sessions that capture learnings via /post-mortem compound over time.
Smart Connections indexes these automatically.
Future sessions search past learnings before starting work.

## Evidence

- 6 epics completed with 163+ commits
- Knowledge flywheel reduced rework by ~30%
- Pre-mortem prevented 6 critical issues

## Application

Always run /post-mortem after significant work.
`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(learningPath, []byte(learningContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created sample learning: %s\n", learningPath)

	// Create sample pattern
	patternPath := filepath.Join(demoDir, ".agents/patterns/demo-pattern.md")
	patternContent := `# Pattern: Wave-Based Parallel Execution

**Tier:** 2 (Pattern)
**Citations:** 4

## Problem

Sequential execution of issues is slow.
But pure parallel causes merge conflicts.

## Solution

Group issues into waves by dependency:
- Wave 1: No dependencies (parallel)
- Wave 2: Depends on Wave 1 (parallel within wave)
- ...

## Usage

` + "```bash" + `
bd ready --parent=<epic>    # See what's unblocked
gt sling <issue1> <issue2>  # Parallel dispatch
` + "```" + `
`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(patternPath, []byte(patternContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created sample pattern: %s\n", patternPath)

	fmt.Println("\n━━━ STEP 2: The RPI Workflow ━━━")
	fmt.Print(`
  RESEARCH → PLAN → IMPLEMENT → VALIDATE
      │                            │
      └──── Knowledge Flywheel ────┘

  Each phase has:
  • Fresh context (prevents error compounding)
  • Quality gates (filters bad work)
  • Ratchet points (locks progress)
`)

	fmt.Println("\n━━━ STEP 3: The Skills ━━━")
	fmt.Print(`
  /research    Deep codebase exploration
  /plan        Decompose into issues (beads)
  /pre-mortem  Simulate failures before implementing
  /implement   Execute single issue
  /crank       Autonomous epic execution
  /vibe        8-aspect code validation
  /post-mortem Validate + extract learnings
`)

	fmt.Println("\n━━━ STEP 4: The Moat ━━━")
	fmt.Print(`
  ┌─────────────────────────────────────────────────────────────┐
  │                    KNOWLEDGE FLYWHEEL                        │
  │                                                              │
  │   Session 1 → learnings → indexed                            │
  │                              ↓                               │
  │   Session 2 → searches → finds prior learnings → better work │
  │                              ↓                               │
  │   Session 3 → even more context → COMPOUNDS                  │
  │                                                              │
  │   Formula: Value = Quality^time × Knowledge^sessions         │
  └─────────────────────────────────────────────────────────────┘

  Others are O(1) per session.
  AgentOps is O(n) where n = historical sessions.
`)

	fmt.Printf("\n✓ Demo files created in: %s\n", demoDir)
	fmt.Print(`
═══════════════════════════════════════════════════════════════════

NEXT STEPS:

  1. Try in your project:
     $ cd your-project
     $ ol quick-start

  2. Or explore the demo files:
     $ ls ` + demoDir + `/.agents/

  3. Learn more:
     $ ol demo --concepts

═══════════════════════════════════════════════════════════════════
`)
	return nil
}
