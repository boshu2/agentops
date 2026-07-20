package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	configShow bool
	configCmd  = newConfigCommand()
)

func runConfig(command *cobra.Command, args []string) error {
	fresh := newConfigCommand()
	if configShow {
		_ = fresh.Flags().Set("show", "true")
	}
	fresh.SetOut(os.Stdout)
	return fresh.RunE(command, args)
}
