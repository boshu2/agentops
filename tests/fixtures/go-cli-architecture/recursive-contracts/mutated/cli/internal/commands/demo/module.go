package demo

import (
	"github.com/spf13/cobra"
	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func command() *cobra.Command {
	root := &cobra.Command{Use: "demo", RunE: run}
	child := &cobra.Command{Use: "child", RunE: run}
	root.AddCommand(child)
	_ = clicontract.Attach(root, clicontract.CommandContract{})
	return root
}

func run(*cobra.Command, []string) error { return nil }
