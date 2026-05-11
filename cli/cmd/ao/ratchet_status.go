// practices: [dora-metrics, continuous-delivery]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	statusSubCmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"s"},
		GroupID: "inspection",
		Short:   "Show ratchet chain state",
		Long: `Display the current state of the ratchet chain.

Shows all steps and their status (pending, in_progress, locked, skipped).

Examples:
  ao ratchet status
  ao ratchet status --epic ol-0001
  ao ratchet status --json`,
		RunE: runRatchetStatus,
	}
	statusSubCmd.Flags().StringVar(&ratchetEpicID, "epic", "", "Filter by epic ID")
	statusSubCmd.Flags().StringVar(&ratchetChainID, "chain", "", "Filter by chain ID")
	ratchetCmd.AddCommand(statusSubCmd)
}

// runRatchetStatus displays the ratchet chain state.
func runRatchetStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	// Get status for all steps
	allStatus := chain.GetAllStatus()

	// Build output structure
	output := ratchetStatusOutput{
		ChainID: chain.ID,
		Started: chain.Started.Format(time.RFC3339),
		EpicID:  chain.EpicID,
		Path:    chain.Path(),
		Steps:   make([]ratchetStepInfo, 0),
	}

	for _, step := range ratchet.AllSteps() {
		info := ratchetStepInfo{
			Step:   step,
			Status: allStatus[step],
		}

		// Get details from latest entry
		if entry := chain.GetLatest(step); entry != nil {
			info.Output = entry.Output
			info.Input = entry.Input
			info.Time = entry.Timestamp.Format(time.RFC3339)
			info.Location = entry.Location
			info.Cycle = entry.Cycle
			info.ParentEpic = entry.ParentEpic
		}

		output.Steps = append(output.Steps, info)
	}

	w := cmd.OutOrStdout()
	return outputRatchetStatus(w, &output)
}

func outputRatchetStatus(w io.Writer, data *ratchetStatusOutput) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)

	case "yaml":
		enc := yaml.NewEncoder(w)
		return enc.Encode(data)

	default: // table
		fmt.Fprintln(w, "Ratchet Chain Status")
		fmt.Fprintln(w, "====================")
		fmt.Fprintf(w, "Chain: %s\n", data.ChainID)
		fmt.Fprintf(w, "Started: %s\n", data.Started)
		if data.EpicID != "" {
			fmt.Fprintf(w, "Epic: %s\n", data.EpicID)
		}

		// Show cycle and parent epic from the latest entry if present
		for _, s := range data.Steps {
			if s.Cycle > 0 {
				fmt.Fprintf(w, "Cycle: %d\n", s.Cycle)
				if s.ParentEpic != "" {
					fmt.Fprintf(w, "Parent: %s\n", s.ParentEpic)
				}
				break
			}
		}
		fmt.Fprintln(w)

		fmt.Fprintf(w, "%-15s %-12s %-40s\n", "STEP", "STATUS", "OUTPUT")
		fmt.Fprintf(w, "%-15s %-12s %-40s\n", "----", "------", "------")

		for _, s := range data.Steps {
			icon := statusIcon(s.Status)
			out := truncate(s.Output, 40)
			fmt.Fprintf(w, "%-15s %s %-10s %-40s\n", s.Step, icon, s.Status, out)
		}

		fmt.Fprintf(w, "\nPath: %s\n", data.Path)
		return nil
	}
}
