package provenance

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/provenanceapp"
	"github.com/spf13/cobra"
)

// runMineView keeps the existing event/checkpoint path unchanged. Excerpts are
// a separate read-only application operation with explicit serialization bounds.
func (m *Module) runMineView(cmd *cobra.Command, view string, opts provenanceapp.ExcerptOptions) error {
	switch view {
	case "events":
		for _, flag := range []string{"target", "start-byte", "max-bytes", "max-records", "max-output-bytes"} {
			if cmd.Flags().Changed(flag) {
				cmd.SilenceUsage = true
				return fmt.Errorf("--%s requires --view excerpts", flag)
			}
		}
		return m.runMineSession(cmd, nil)
	case "excerpts":
		cmd.SilenceUsage = true
		if cmd.Flags().Changed("state") {
			return fmt.Errorf("excerpt view does not use --state")
		}
		if !m.mineJSON || m.outputMode() == "yaml" {
			return fmt.Errorf("excerpt view requires JSON output")
		}
		opts.File = m.mineFile
		return provenanceapp.ExcerptSession(opts, cmd.OutOrStdout())
	default:
		cmd.SilenceUsage = true
		return fmt.Errorf("unknown mine-session view %q: choose events or excerpts", view)
	}
}
