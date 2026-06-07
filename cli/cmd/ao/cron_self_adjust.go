// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	cronSelfAdjustRelocatedRoute       = "mto-fleet"
	cronSelfAdjustRelocatedReplacement = "MTO cron-fire driver"
)

var (
	cronSelfAdjustOn         string
	cronSelfAdjustTemplate   string
	cronSelfAdjustShipped    string
	cronSelfAdjustNext       string
	cronSelfAdjustSubBeads   string
	cronSelfAdjustTestsDelta string
)

var cronSelfAdjustCmd = &cobra.Command{
	Use:   "self-adjust",
	Short: "Compatibility shim for relocated cron-fire scheduling",
	Long: `Compatibility shim for the retired local cron self-adjust renderer.

AgentOps no longer renders CronCreate prompts or writes cron-history from the
lean local image. Fleet cadence, cron-fire scheduling, and prompt re-arming now
belong behind the MTO/factory boundary. This command remains so older scripts
can discover the relocation route without losing the public command surface.`,
	Args: cobra.NoArgs,
	RunE: runCronSelfAdjust,
}

type cronSelfAdjustRelocationNotice struct {
	Status      string            `json:"status"`
	Route       string            `json:"route"`
	Replacement string            `json:"replacement"`
	Message     string            `json:"message"`
	Accepted    map[string]string `json:"accepted_flags,omitempty"`
}

func init() {
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustOn, "on", "cycle-close", "Accepted for compatibility; MTO owns cron scheduling")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustTemplate, "template", "", "Accepted for compatibility; MTO owns prompt rendering")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustShipped, "shipped", "", "Accepted for compatibility; MTO owns shipped-cycle context")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustNext, "next", "", "Accepted for compatibility; MTO owns next-cycle selection")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustSubBeads, "sub-beads", "", "Accepted for compatibility; MTO owns sub-bead fan-out context")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustTestsDelta, "tests-delta", "", "Accepted for compatibility; MTO owns test-delta context")
	cronCmd.AddCommand(cronSelfAdjustCmd)
}

func runCronSelfAdjust(cmd *cobra.Command, _ []string) error {
	notice := cronSelfAdjustRelocationNotice{
		Status:      "relocated",
		Route:       cronSelfAdjustRelocatedRoute,
		Replacement: cronSelfAdjustRelocatedReplacement,
		Message:     "ao cron self-adjust has moved behind the MTO/factory boundary; AO keeps only this compatibility shim.",
		Accepted: map[string]string{
			"on":          cronSelfAdjustOn,
			"template":    cronSelfAdjustTemplate,
			"shipped":     cronSelfAdjustShipped,
			"next":        cronSelfAdjustNext,
			"sub_beads":   cronSelfAdjustSubBeads,
			"tests_delta": cronSelfAdjustTestsDelta,
		},
	}
	return writeCronSelfAdjustNotice(cmd.OutOrStdout(), notice)
}

func writeCronSelfAdjustNotice(w io.Writer, notice cronSelfAdjustRelocationNotice) error {
	if w == nil {
		return fmt.Errorf("cron self-adjust notice requires a writer")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(notice); err != nil {
		return fmt.Errorf("encode cron relocation notice: %w", err)
	}
	return nil
}
