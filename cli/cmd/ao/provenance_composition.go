// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"time"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/spf13/cobra"

	provenancecommands "github.com/boshu2/agentops/cli/internal/commands/provenance"
)

func init() {
	rootCmd.AddCommand(newProvenanceCommand())
}

// newProvenanceCommand wires the provenance command module to its host seams.
// The ledger path (filesystem walk) and the clock are host effects injected
// here. The module attaches family and evidence-leaf contracts and delegates
// explicit evidence filesystem operations to internal/evidence.
func newProvenanceCommand() *cobra.Command {
	module := provenancecommands.NewModule(clicontract.HostOptions{
		LedgerPath: resolveLedgerPath,
		Now:        time.Now,
		OutputMode: GetOutput,
		DryRun:     GetDryRun,
	})
	return module.Command()
}
