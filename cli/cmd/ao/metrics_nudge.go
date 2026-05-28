// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func init() {
	flywheelCmd.AddCommand(nudgeCmd)
}

// nudgeCmd provides hook-consumable flywheel + ratchet status in a single call.
var nudgeCmd = &cobra.Command{
	Use:   "nudge",
	Short: "Combined flywheel + ratchet + pool status for hooks",
	Long: `Returns structured JSON combining:
  - Flywheel status (velocity, escape velocity, counts)
  - Ratchet chain state (last step, next step, artifact, skill)
  - Pool pending counts (pending, approaching threshold)
  - Suggested next action

Designed for session-start.sh to provide contextual nudges in a single call.

Examples:
  ao flywheel nudge
  ao flywheel nudge --json
  ao flywheel nudge -o table`,
	RunE: runNudge,
}

// NudgeResult is the structured output for hooks.
type NudgeResult struct {
	Status          string   `json:"status"`
	Velocity        float64  `json:"velocity"`
	EscapeVelocity  bool     `json:"escape_velocity"`
	SessionsCount   int      `json:"sessions_count"`
	LearningsCount  int      `json:"learnings_count"`
	RPIState        RPIState `json:"rpi_state"`
	PoolPending     int      `json:"pool_pending"`
	PoolApproaching int      `json:"pool_approaching"`
	Suggestion      string   `json:"suggestion"`
}

// RPIState represents the current RPI workflow state.
type RPIState struct {
	LastStep string `json:"last_step"`
	NextStep string `json:"next_step"`
	Artifact string `json:"artifact"`
	Skill    string `json:"skill"`
}

// runNudge combines flywheel, ratchet, and pool data into a single response.
func runNudge(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Compute flywheel metrics
	metrics, err := computeMetrics(cwd, metricsDays)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	populateGoldenSignals(cwd, metricsDays, metrics)

	// Load ratchet chain state
	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		VerbosePrintf("Warning: load chain: %v\n", err)
		// Continue with empty chain - this is non-fatal
		chain = &ratchet.Chain{}
	}

	// Get pool counts
	p := pool.NewPool(cwd)
	pendingEntries, err := p.List(pool.ListOptions{
		Status: types.PoolStatusPending,
	})
	if err != nil {
		VerbosePrintf("Warning: list pool: %v\n", err)
	}

	// Count pending and approaching
	poolPending := len(pendingEntries)
	poolApproaching := 0
	for _, entry := range pendingEntries {
		if entry.ApproachingAutoPromote {
			poolApproaching++
		}
	}

	// Build RPI state
	rpiState := buildRPIState(chain)

	// Build suggestion
	suggestion := buildSuggestion(metrics, rpiState, poolPending, poolApproaching)

	// Build result
	result := NudgeResult{
		Status:          metrics.HealthStatus(),
		Velocity:        metrics.Velocity,
		EscapeVelocity:  metrics.AboveEscapeVelocity,
		SessionsCount:   metrics.TotalArtifacts, // Approximation - actual sessions are in storage
		LearningsCount:  metrics.TierCounts["learning"],
		RPIState:        rpiState,
		PoolPending:     poolPending,
		PoolApproaching: poolApproaching,
		Suggestion:      suggestion,
	}

	return outputNudge(&result)
}

// buildRPIState determines the current RPI workflow state from the chain.
func buildRPIState(chain *ratchet.Chain) RPIState {
	state := RPIState{
		LastStep: "",
		NextStep: "",
		Artifact: "",
		Skill:    "",
	}

	if chain == nil || len(chain.Entries) == 0 {
		state.NextStep = "research"
		state.Skill = "/research"
		return state
	}

	// Get all step statuses
	allStatus := chain.GetAllStatus()

	// Find last completed step
	for _, step := range ratchet.AllSteps() {
		status := allStatus[step]
		if status == "locked" || status == "in_progress" {
			state.LastStep = string(step)
			// Get artifact from latest entry
			if entry := chain.GetLatest(step); entry != nil {
				state.Artifact = entry.Output
			}
		}
	}

	// Determine next step based on last step
	state.NextStep = determineNextStep(state.LastStep)
	state.Skill = stepToSkill(state.NextStep)

	return state
}

// determineNextStep returns the next logical step in the RPI workflow.
func determineNextStep(lastStep string) string {
	switch lastStep {
	case "":
		return "research"
	case "research":
		return "pre-mortem"
	case "pre-mortem":
		return "plan"
	case "plan":
		return "implement"
	case "implement":
		return "vibe"
	case "crank":
		return "vibe"
	case "vibe":
		return "post-mortem"
	case "post-mortem":
		return "research" // Loop back
	default:
		return "research"
	}
}

// stepToSkill maps a step name to its corresponding skill.
func stepToSkill(step string) string {
	switch step {
	case "research":
		return "/research"
	case "pre-mortem":
		return "/pre-mortem"
	case "plan":
		return "/plan"
	case "implement":
		return "/implement"
	case "crank":
		return "/crank"
	case "vibe":
		return "/vibe"
	case "post-mortem":
		return "/post-mortem"
	default:
		return ""
	}
}

// buildSuggestion generates a contextual suggestion based on current state.
func buildSuggestion(metrics *types.FlywheelMetrics, rpiState RPIState, poolPending, poolApproaching int) string {
	// Priority 1: Pool candidates approaching threshold
	if poolApproaching > 0 {
		return fmt.Sprintf("Review %d candidates approaching auto-promote threshold", poolApproaching)
	}

	// Priority 2: Pool pending
	if poolPending > 5 {
		return fmt.Sprintf("Review %d pending pool candidates", poolPending)
	}

	// Priority 3: RPI workflow continuation
	if rpiState.LastStep != "" && rpiState.NextStep != "" {
		return fmt.Sprintf("Resume %s — %s complete", rpiState.Skill, rpiState.LastStep)
	}

	// Priority 4: Flywheel health
	if !metrics.HealthCompounding() {
		if metrics.GoldenSignals != nil {
			switch metrics.GoldenSignals.ClosureVerdict {
			case "unmined", "hoarding":
				return "Mine research into learnings before calling the flywheel healthy"
			}
			switch metrics.GoldenSignals.PipelineVerdict {
			case "degrading":
				return "Favor applied citations over retrieved-only references"
			}
			switch metrics.GoldenSignals.ConcentrationVerdict {
			case "dormant":
				return "Broaden reuse: retrieve and apply more than the same few artifacts"
			}
		}
		if metrics.Sigma < 0.3 {
			return "Improve retrieval: use 'ao lookup' for on-demand knowledge"
		}
		if metrics.Rho < 0.5 {
			return "Cite more learnings: reference artifacts in your work"
		}
		return "Start new work: run /research to build knowledge"
	}

	// Default: flywheel is healthy
	return "Knowledge compounding. Continue your work."
}

// outputNudge formats the nudge result according to output format.
func outputNudge(result *NudgeResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default: // table
		fmt.Println()
		fmt.Println("AgentOps Status Nudge")
		fmt.Println("=====================")
		fmt.Println()

		// Flywheel status
		fmt.Printf("Flywheel: [%s] velocity=%.2f escape=%v\n",
			result.Status, result.Velocity, result.EscapeVelocity)

		// RPI state
		if result.RPIState.LastStep != "" {
			fmt.Printf("RPI: last=%s next=%s skill=%s\n",
				result.RPIState.LastStep, result.RPIState.NextStep, result.RPIState.Skill)
		} else {
			fmt.Printf("RPI: next=%s skill=%s\n",
				result.RPIState.NextStep, result.RPIState.Skill)
		}

		// Pool status
		fmt.Printf("Pool: pending=%d approaching=%d\n",
			result.PoolPending, result.PoolApproaching)

		// Suggestion
		fmt.Println()
		fmt.Printf("Suggestion: %s\n", result.Suggestion)
		fmt.Println()

		return nil
	}
}
