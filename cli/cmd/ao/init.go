package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create local AgentOps evidence directories",
	Long: `Create local evidence and verdict directories. This command does not
initialize Git, edit ignore files, install hooks, select work, or start a runtime.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.GroupID = "start"
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	paths := []string{
		filepath.Join(".agents", "ao", "verdicts", "sha256"),
		filepath.Join(".agents", "ao", "provenance"),
		filepath.Join(".agents", "handoff"),
	}
	for _, relative := range paths {
		if GetDryRun() {
			fmt.Fprintf(cmd.OutOrStdout(), "would create %s\n", relative)
			continue
		}
		if err := os.MkdirAll(filepath.Join(cwd, relative), 0o700); err != nil {
			return fmt.Errorf("create %s: %w", relative, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", relative)
	}
	return nil
}
