// practices: [dora-metrics, continuous-delivery]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

// NextResult holds the result of ratchet next command.
type NextResult struct {
	Next         string `json:"next" yaml:"next"`
	Reason       string `json:"reason" yaml:"reason"`
	LastStep     string `json:"last_step" yaml:"last_step"`
	LastArtifact string `json:"last_artifact" yaml:"last_artifact"`
	Skill        string `json:"skill" yaml:"skill"`
	Complete     bool   `json:"complete" yaml:"complete"`
}

// stepSkillMap maps ratchet steps to their corresponding skill commands.
var stepSkillMap = map[string]string{
	"research":    "/research",
	"pre-mortem":  "/pre-mortem",
	"plan":        "/plan",
	"implement":   "/implement or /crank",
	"crank":       "/implement or /crank",
	"vibe":        "/vibe",
	"post-mortem": "/post-mortem",
}

func init() {
	ratchetNextCmd := &cobra.Command{
		Use:     "next",
		Aliases: []string{"n"},
		GroupID: "inspection",
		Short:   "Show next pending RPI step",
		Long: `Show the next pending step in the RPI workflow.

Returns structured output indicating what to do next based on the current
ratchet chain state. Returns "complete" if all steps are locked.

Examples:
  ao ratchet next
  ao ratchet next --json
  ao ratchet next --epic ol-0001`,
		RunE: runRatchetNext,
	}
	ratchetNextCmd.Flags().StringVar(&ratchetEpicID, "epic", "", "Filter by epic ID")
	ratchetCmd.AddCommand(ratchetNextCmd)
}

// runRatchetNext determines the next pending RPI step.
func runRatchetNext(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	// Filter by epic if requested
	if ratchetEpicID != "" && chain.EpicID != ratchetEpicID {
		return fmt.Errorf("no chain found for epic %s", ratchetEpicID)
	}

	result := computeNextStep(chain)
	return outputNextResult(&result)
}

// computeNextStep analyzes the chain and determines the next step.
func computeNextStep(chain *ratchet.Chain) NextResult {
	allSteps := ratchet.AllSteps()

	lastStep, lastEntry := findLastLockedEntry(chain)
	if lastEntry == nil {
		reason := "no steps completed yet"
		if len(chain.Entries) > 0 {
			reason = "no steps locked yet"
		}
		return newStartResult(allSteps[0], reason)
	}

	if lastStep == ratchet.StepPostMortem {
		return newCompleteResult(lastStep, lastEntry)
	}

	nextStep := resolveNextStep(allSteps, lastStep)
	return newPendingResult(nextStep, lastStep, lastEntry)
}

// findLastLockedEntry returns the most recent locked or skipped entry.
func findLastLockedEntry(chain *ratchet.Chain) (ratchet.Step, *ratchet.ChainEntry) {
	for i := len(chain.Entries) - 1; i >= 0; i-- {
		entry := &chain.Entries[i]
		if entry.Locked || entry.Skipped {
			return entry.Step, entry
		}
	}
	return "", nil
}

// resolveNextStep determines the successor step, handling implement/crank aliasing.
func resolveNextStep(allSteps []ratchet.Step, lastStep ratchet.Step) ratchet.Step {
	// Either implement or crank satisfies the build position; next is vibe.
	if lastStep == ratchet.StepImplement || lastStep == ratchet.StepCrank {
		return ratchet.StepVibe
	}
	// After plan, suggest implement (skip crank).
	if lastStep == ratchet.StepPlan {
		return ratchet.StepImplement
	}
	// Default: advance to the successor in AllSteps.
	for i, step := range allSteps {
		if step == lastStep && i < len(allSteps)-1 {
			return allSteps[i+1]
		}
	}
	return ""
}

// newStartResult builds a NextResult for the first step.
func newStartResult(first ratchet.Step, reason string) NextResult {
	return NextResult{
		Next:   string(first),
		Reason: reason,
		Skill:  stepSkillMap[string(first)],
	}
}

// newCompleteResult builds a NextResult indicating all steps are done.
func newCompleteResult(lastStep ratchet.Step, lastEntry *ratchet.ChainEntry) NextResult {
	return NextResult{
		Reason:       "all steps completed",
		LastStep:     string(lastStep),
		LastArtifact: lastEntry.Output,
		Complete:     true,
	}
}

// newPendingResult builds a NextResult for the next pending step.
func newPendingResult(nextStep, lastStep ratchet.Step, lastEntry *ratchet.ChainEntry) NextResult {
	reason := "unexpected state"
	if nextStep != "" {
		reason = fmt.Sprintf("%s locked", lastStep)
	}
	return NextResult{
		Next:         string(nextStep),
		Reason:       reason,
		LastStep:     string(lastStep),
		LastArtifact: lastEntry.Output,
		Skill:        stepSkillMap[string(nextStep)],
	}
}

// outputNextResult formats and outputs the result based on output format.
func outputNextResult(result *NextResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(result)

	default: // table
		if result.Complete {
			fmt.Println("✓ All RPI steps complete")
			if result.LastStep != "" {
				fmt.Printf("\nLast step: %s\n", result.LastStep)
			}
			if result.LastArtifact != "" {
				fmt.Printf("Last artifact: %s\n", result.LastArtifact)
			}
			return nil
		}

		fmt.Printf("Next step: %s\n", result.Next)
		fmt.Printf("Reason: %s\n", result.Reason)
		if result.Skill != "" {
			fmt.Printf("Suggested skill: %s\n", result.Skill)
		}
		if result.LastStep != "" {
			fmt.Printf("\nLast step: %s\n", result.LastStep)
		}
		if result.LastArtifact != "" {
			fmt.Printf("Last artifact: %s\n", result.LastArtifact)
		}
		return nil
	}
}
