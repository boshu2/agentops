package main

import (
	"github.com/spf13/cobra"
	"os"
)

func prepare(*cobra.Command, []string) error {
	_, err := os.Getwd()
	return err
}

var root = &cobra.Command{Use: "ao", PersistentPreRunE: prepare}
