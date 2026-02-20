package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	specSubCmd := &cobra.Command{
		Use:     "spec",
		GroupID: "inspection",
		Short:   "Get current spec path",
		Long: `Find and output the current spec artifact path.

Searches for specs in priority order: crew → rig → town.

Examples:
  ao ratchet spec
  SPEC=$(ol ratchet spec) && echo $SPEC`,
		RunE: runRatchetSpec,
	}
	ratchetCmd.AddCommand(specSubCmd)
}

// runRatchetSpec finds the current spec path.
func runRatchetSpec(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	locator, err := ratchet.NewLocator(cwd)
	if err != nil {
		return fmt.Errorf("create locator: %w", err)
	}

	// Search for specs in order
	patterns := []string{
		"specs/*-v*.md",
		"synthesis/*.md",
	}

	for _, pattern := range patterns {
		path, loc, err := locator.FindFirst(pattern)
		if err == nil {
			w := cmd.OutOrStdout()
			switch GetOutput() {
			case "json":
				result := map[string]string{
					"path":     path,
					"location": string(loc),
				}
				enc := json.NewEncoder(w)
				return enc.Encode(result)

			default:
				fmt.Fprintln(w, path)
			}
			return nil
		}
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "No spec found")
	return fmt.Errorf("no spec found")
}
