// practices: [microservices, team-topologies]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/bridge"
	"github.com/spf13/cobra"
)

var (
	factoryStartGoal          string
	factoryStartLimit         int
	factoryStartNoMaintenance bool
)

type factoryStartResult struct {
	Workspace       string           `json:"workspace"`
	Goal            string           `json:"goal,omitempty"`
	Briefing        string           `json:"briefing,omitempty"`
	BriefingWarning string           `json:"briefing_warning,omitempty"`
	Codex           codexStartResult `json:"codex"`
	Recommended     []string         `json:"recommended,omitempty"`
}

var factoryCmd = &cobra.Command{
	Use:   "factory",
	Short: "Software-factory operator surface for briefing-first agent work",
	Long: `Software-factory operator surface for AgentOps.

This surface keeps the operator lane explicit:
  1. Compile a bounded goal-time briefing when the knowledge corpus can support it
  2. Start the runtime with explicit startup context
  3. Run the delivery lane through /rpi or ao rpi phased
  4. Close the loop with ao codex stop and ao knowledge activate

Use lower-level commands like ao knowledge, ao codex, ao context assemble, and
ao rpi when you need substrate-level control.`,
}

var factoryStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Compile a briefing when possible, then run explicit Codex startup",
	Long: `Start the software-factory operator lane for a concrete goal.

When --goal is provided, AgentOps attempts to build a goal-time briefing from
the local .agents corpus before surfacing Codex startup context. The startup
path then retrieves ranked briefings, learnings, patterns, findings, research,
and next work for the same goal so the runtime starts from a bounded work
packet instead of a broad recency dump.`,
	Args: cobra.NoArgs,
	RunE: runFactoryStart,
}

func init() {
	factoryCmd.GroupID = "workflow"
	rootCmd.AddCommand(factoryCmd)
	factoryCmd.AddCommand(factoryStartCmd)

	factoryStartCmd.Flags().StringVar(&factoryStartGoal, "goal", "", "Goal to brief and use as the startup query")
	factoryStartCmd.Flags().IntVar(&factoryStartLimit, "limit", 3, "Maximum artifacts to surface per category during startup")
	factoryStartCmd.Flags().BoolVar(&factoryStartNoMaintenance, "no-maintenance", false, "Skip safe close-loop maintenance on start")
}

func runFactoryStart(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result, err := performFactoryStart(cwd, strings.TrimSpace(factoryStartGoal), factoryStartLimit, factoryStartNoMaintenance)
	if err != nil {
		return err
	}
	return outputFactoryStartResult(result)
}

func performFactoryStart(cwd, goal string, limit int, noMaintenance bool) (factoryStartResult, error) {
	result := factoryStartResult{
		Workspace:   cwd,
		Goal:        goal,
		Recommended: factoryRecommendedCommands(goal),
	}

	if goal != "" {
		agentsRoot := filepath.Join(cwd, ".agents")
		run, err := runKnowledgeNativeBuilder(cwd, agentsRoot, knowledgeBuilderInvocation{
			Step:           "briefing",
			Implementation: knowledgeBuilderImplementationAONative,
			Args:           []string{"--goal", goal},
		})
		if err != nil {
			result.BriefingWarning = err.Error()
		} else {
			result.Briefing = firstNonEmptyTrimmed(run.Metadata["briefing"], run.Path)
		}
	}

	origQuery := codexStartQuery
	origLimit := codexStartLimit
	origNoMaintenance := codexStartNoMaintenance
	codexStartQuery = goal
	codexStartLimit = limit
	codexStartNoMaintenance = noMaintenance
	defer func() {
		codexStartQuery = origQuery
		codexStartLimit = origLimit
		codexStartNoMaintenance = origNoMaintenance
	}()

	codexResult, err := performCodexStart(cwd)
	if err != nil {
		return factoryStartResult{}, err
	}
	result.Codex = codexResult
	if result.Briefing == "" && len(codexResult.Briefings) > 0 {
		result.Briefing = codexResult.Briefings[0].Path
	}

	return result, nil
}

func outputFactoryStartResult(result factoryStartResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Software Factory Start")
	fmt.Println("======================")
	fmt.Printf("Workspace: %s\n", result.Workspace)
	if result.Goal != "" {
		fmt.Printf("Goal: %s\n", result.Goal)
	}
	if result.Briefing != "" {
		fmt.Printf("Briefing: %s\n", result.Briefing)
	} else if result.Goal != "" {
		fmt.Println("Briefing: not built")
	} else {
		fmt.Println("Briefing: skipped (no --goal provided)")
	}
	if result.BriefingWarning != "" {
		fmt.Printf("Briefing note: %s\n", result.BriefingWarning)
	}
	fmt.Printf("Startup context: %s\n", result.Codex.StartupContextPath)
	if result.Codex.MemoryPath != "" {
		fmt.Printf("Memory: %s\n", result.Codex.MemoryPath)
	}
	fmt.Println()
	fmt.Println("Factory lane:")
	for _, step := range result.Recommended {
		fmt.Printf("  - %s\n", step)
	}
	return nil
}

func factoryRecommendedCommands(goal string) []string {
	return bridge.FactoryRecommendedCommands(goal)
}
