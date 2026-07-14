package demo

import (
	"encoding/json"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/spf13/cobra"
)

func contract() clicontract.CommandContract {
	return clicontract.CommandContract{Output: clicontract.OutputStructured}
}

func command() *cobra.Command {
	return &cobra.Command{RunE: func(command *cobra.Command, _ []string) error {
		return json.NewEncoder(command.OutOrStdout()).Encode(map[string]bool{"ok": true})
	}}
}
