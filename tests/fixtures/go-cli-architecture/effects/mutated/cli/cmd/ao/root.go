package main

import (
	"os"

	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use: "ao",
	PersistentPreRunE: func(*cobra.Command, []string) error {
		_, err := os.Getwd()
		return err
	},
}
