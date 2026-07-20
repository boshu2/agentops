// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/boshu2/agentops/cli/internal/clicontract"

	provenancecommands "github.com/boshu2/agentops/cli/internal/commands/provenance"
)

func init() {
	rootCmd.AddCommand(newProvenanceCommand())
}

// newProvenanceCommand wires the provenance command module to its host seams.
// The ledger path (filesystem walk) and the clock are host effects injected
// here; the module itself performs neither. Provenance had no attached
// capabilities contract before the carve-out, so this composition does not
// attach the module's contract either — the capabilities surface is unchanged.
func newProvenanceCommand() *cobra.Command {
	module := provenancecommands.NewModule(clicontract.HostOptions{
		LedgerPath: resolveLedgerPath,
		Now:        time.Now,
		OutputMode: GetOutput,
	})
	return module.Command()
}
