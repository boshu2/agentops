package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var sessionBootstrapJSON bool

type sessionBootstrapStatus struct {
	Workspace        string   `json:"workspace"`
	OrientationFiles []string `json:"orientation_files"`
}

var sessionBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Report available local orientation files",
	Long: `Report local orientation files without starting runtimes, probing
trackers, selecting work, inspecting queues, or installing hooks.`,
	Args: cobra.NoArgs,
	RunE: runSessionBootstrap,
}

func init() {
	sessionCmd.AddCommand(sessionBootstrapCmd)
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapJSON, "json", false, "Emit JSON")
}

func runSessionBootstrap(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	status := sessionBootstrapStatus{Workspace: cwd, OrientationFiles: []string{}}
	for _, relative := range []string{"AGENTS.md", "README.md", "PRODUCT.md", "GOALS.md", "PROGRAM.md"} {
		if info, err := os.Stat(filepath.Join(cwd, relative)); err == nil && info.Mode().IsRegular() {
			status.OrientationFiles = append(status.OrientationFiles, relative)
		}
	}
	if sessionBootstrapJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "workspace: %s\n", status.Workspace)
	for _, relative := range status.OrientationFiles {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", relative)
	}
	return nil
}
