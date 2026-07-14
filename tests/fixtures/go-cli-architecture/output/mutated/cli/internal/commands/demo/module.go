package demo

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/spf13/cobra"
)

func contract() clicontract.CommandContract {
	return clicontract.CommandContract{Output: clicontract.OutputStructured}
}

func command() *cobra.Command {
	command := &cobra.Command{RunE: func(command *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(command.OutOrStdout(), "human text")
		return err
	}}
	_ = clicontract.Attach(command, contract())
	return command
}
