package cliapp

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// Module declares behavior separately from presentation. Command must return a
// fresh, side-effect-free Cobra tree on every call.
type Module interface {
	Contract() clicontract.CommandContract
	Command() *cobra.Command
}
