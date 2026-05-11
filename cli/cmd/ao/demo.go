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
	Short: "Show the council-first AgentOps 3.0 value path",
	Long: `Run a demonstration of AgentOps as the engineering operating system
for agent teams.

This command walks you through the launch path:
  1. Install AgentOps beside your existing agent runtime
  2. Make the domain and engineering practices visible
  3. Assemble bounded context for the task
  4. Run a mixed-model council and record its verdict
  5. Track the follow-up work and optionally schedule compounding

No hosted control plane required. AgentOps runs on your machine, your repo,
your subscriptions, and your chosen agent harness.

Examples:
  ao demo              # Interactive walkthrough
  ao demo --quick      # 2-minute council-first overview
  ao demo --concepts   # Explain the product model`,
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
	demoCmd.Flags().BoolVar(&demoQuick, "quick", false, "2-minute council-first overview")
	demoCmd.Flags().BoolVar(&demoConcepts, "concepts", false, "Explain product model")
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
║                    AGENTOPS 3.0 PRODUCT MODEL                    ║
╚══════════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────────────────────────────┐
│  1. ENGINEERING OS FOR AGENT TEAMS                               │
│                                                                  │
│     Humans already learned how to coordinate work in complex     │
│     codebases: domain language, specs, tests, review, evidence,  │
│     release gates, and retros. AgentOps encodes that discipline  │
│     for agents.                                                  │
│                                                                  │
│     You bring the agent and harness. AgentOps supplies the       │
│     operating layer around the work.                             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  2. DOMAIN AND PRACTICE PACKETS                                  │
│                                                                  │
│     A packet tells the agent what domain it is operating in,     │
│     which engineering practices apply, what evidence matters,    │
│     and which claims are off limits.                             │
│                                                                  │
│     The packet is small enough to inspect, commit, and reuse.    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  3. COUNCIL VERDICTS                                             │
│                                                                  │
│     /council --mixed validate this PR                            │
│       -> Claude and Codex judge the same evidence packet         │
│       -> consolidated verdict lands in .agents/council/          │
│                                                                  │
│     From agent opinions to engineering verdicts.                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  4. LOCAL CORPUS AND SCHEDULED COMPOUNDING                       │
│                                                                  │
│     .agents/ records attempts, citations, decisions, verdicts,  │
│     handoffs, and learnings. The daemon can run approved         │
│     compounding jobs when you want an always-on lane.            │
│                                                                  │
│     In-the-loop for high-rigor work. On-the-loop for scheduled   │
│     maintenance and knowledge compounding.                       │
└─────────────────────────────────────────────────────────────────┘

Next steps:
  ao demo --quick    # See the council-first path
  ao quick-start     # Set up your first project`)
	return nil
}

func quickDemo() error {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                    AGENTOPS QUICK DEMO (2 min)                   ║
╚══════════════════════════════════════════════════════════════════╝`)

	steps := []struct {
		title   string
		cmd     string
		explain string
	}{
		{
			"1. Install beside the agent you already use",
			"curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash",
			"AgentOps adds skills, hooks, and the ao CLI. Your repo, runtime, and subscriptions stay yours.",
		},
		{
			"2. Make the operating domain visible",
			"cp docs/examples/agentops-3-domain-practice-packet.md .agents/packets/checkout-hardening.md",
			"The domain/practice packet makes language, engineering practices, evidence rules, and non-claims reviewable.",
		},
		{
			"3. Assemble bounded task context",
			"ao context assemble --task \"validate checkout retry plan\" --phase pre-mortem --output-file .agents/rpi/briefing-current.md",
			"The agent works from the packet and repo evidence, not from a vague prompt.",
		},
		{
			"4. Ask for an engineering verdict",
			"/council --mixed validate .agents/rpi/briefing-current.md",
			"Claude and Codex judge the same evidence and produce one consolidated verdict.",
		},
		{
			"5. Track the work the verdict creates",
			"bd create \"Add checkout retry jitter\" --description \"From .agents/council/<run-id>/verdict.md\" --json",
			"The decision turns into tracked work that cites the verdict and survives agent/session boundaries.",
		},
		{
			"6. Optional: move trusted loops to the daemon",
			"ao daemon run & ao schedule add --file ./examples/schedules/dream-nightly.yaml",
			"Approved compounding can run on a schedule while humans keep authority over what mutates code.",
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

THE PRODUCT:

  The engineering operating system for agent teams.
  A disciplined engineering layer for agentic software development.

WHAT YOU CAN SEE:

  .agents/packets/checkout-hardening.md       # shared domain and practice context
  .agents/rpi/briefing-current.md             # bounded task context
  .agents/council/<run-id>/verdict.md         # mixed-model engineering verdict
  .beads/issues.jsonl                         # tracked follow-up work
  examples/schedules/dream-nightly.yaml       # optional always-on lane

Next: ao quick-start`)
	return nil
}

func interactiveDemo() error {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                AGENTOPS INTERACTIVE DEMO                         ║
║           Engineering discipline for agent teams                  ║
╚══════════════════════════════════════════════════════════════════╝

This demo will:
  ✓ Create a sample AgentOps 3.0 packet workspace
  ✓ Show how bounded context becomes a council verdict
  ✓ Record the artifacts a human can inspect
  ✓ Show where scheduled compounding fits

Press Enter to continue...`)

	//nolint:errcheck // demo interactive prompt, ignore input errors
	fmt.Scanln() // #nosec G104

	// Create demo directories
	homeDir, _ := os.UserHomeDir()
	demoDir := filepath.Join(homeDir, ".agentops-demo")

	dirs := []string{
		filepath.Join(demoDir, ".agents/packets"),
		filepath.Join(demoDir, ".agents/rpi"),
		filepath.Join(demoDir, ".agents/council/demo-run"),
		filepath.Join(demoDir, ".agents/schedules"),
	}

	fmt.Println("\n━━━ STEP 1: Creating the operating workspace ━━━")
	for _, dir := range dirs {
		//nolint:errcheck // demo code, errors shown implicitly by missing output
		os.MkdirAll(dir, 0750) // #nosec G104
		fmt.Printf("  ✓ Created %s\n", dir)
	}

	packetPath := filepath.Join(demoDir, ".agents/packets/checkout-hardening.md")
	packetContent := `# Domain and Practice Packet: Checkout Hardening

**Date:** ` + time.Now().Format("2006-01-02") + `
**Source:** AgentOps Demo

## Domain

Checkout retry behavior must protect customer conversion, payment correctness,
and operational visibility.

## Engineering Practices

- Domain-driven language for retry, idempotency, and payment boundaries
- Test-first validation for backoff and duplicate-submit behavior
- Council review before shipping risky checkout changes

## Evidence Required

- Relevant code paths and tests
- Risk list for duplicate charge and stuck checkout states
- Consolidated council verdict
`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(packetPath, []byte(packetContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created domain/practice packet: %s\n", packetPath)

	briefingPath := filepath.Join(demoDir, ".agents/rpi/briefing-current.md")
	briefingContent := `# Context Briefing: Validate Checkout Retry Plan

## Task

Validate the implementation plan before code changes begin.

## Loaded Packet

` + packetPath + `

## Expected Gate

Run a mixed-model council and require a consolidated PASS/WARN/BLOCK verdict.

`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(briefingPath, []byte(briefingContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created context briefing: %s\n", briefingPath)

	verdictPath := filepath.Join(demoDir, ".agents/council/demo-run/verdict.md")
	verdictContent := `# Council Verdict: Checkout Retry Plan

**Status:** WARN
**Judges:** Claude Code, Codex CLI

## Consensus

Proceed after adding jitter bounds and duplicate-submit tests.

## Required Follow-up

- Add retry jitter lower and upper bounds
- Add idempotency regression tests
- Capture post-merge learning for future checkout work
`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(verdictPath, []byte(verdictContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created council verdict: %s\n", verdictPath)

	schedulePath := filepath.Join(demoDir, ".agents/schedules/nightly-dream.yaml")
	scheduleContent := `version: 1
schedules:
  - name: checkout-dream-nightly
    cron: "0 2 * * *"
    command: "ao overnight run --goal 'find stale checkout learnings'"
`
	//nolint:errcheck // demo code, errors shown implicitly by missing output
	os.WriteFile(schedulePath, []byte(scheduleContent), 0600) // #nosec G104
	fmt.Printf("  ✓ Created optional schedule: %s\n", schedulePath)

	fmt.Println("\n━━━ STEP 2: The Product Path ━━━")
	fmt.Print(`
  INSTALL -> PACKET -> CONTEXT -> COUNCIL -> WORK -> OPTIONAL SCHEDULE

  You can inspect every handoff:
  - the domain and practice packet
  - the assembled task briefing
  - the mixed-model verdict
  - the tracked issue that follows
  - the scheduled compounding lane when you choose it
`)

	fmt.Println("\n━━━ STEP 3: The Commands ━━━")
	fmt.Print(`
  ao quick-start
  ao context assemble --task "validate checkout retry plan" --phase pre-mortem
  /council --mixed validate .agents/rpi/briefing-current.md
  bd create "Add checkout retry jitter"
  ao daemon run & ao schedule add --file .agents/schedules/nightly-dream.yaml
`)

	fmt.Printf("\n✓ Demo files created in: %s\n", demoDir)
	fmt.Print(`
═══════════════════════════════════════════════════════════════════

NEXT STEPS:

  1. Try in your project:
     $ cd your-project
     $ ao quick-start

  2. Or explore the demo files:
     $ ls ` + demoDir + `/.agents/

  3. Learn more:
     $ ao demo --concepts

═══════════════════════════════════════════════════════════════════
`)
	return nil
}
