package demo

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func contract() clicontract.CommandContract {
	return clicontract.CommandContract{Output: clicontract.OutputStructured}
}

func command() *cobra.Command {
	return &cobra.Command{RunE: func(command *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(command.OutOrStdout(), "human text")
		return err
	}}
}
