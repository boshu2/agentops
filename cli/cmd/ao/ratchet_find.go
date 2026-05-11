// practices: [dora-metrics, continuous-delivery]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	findSubCmd := &cobra.Command{
		Use:     "find <pattern>",
		GroupID: "search",
		Short:   "Search for artifacts",
		Long: `Search for artifacts across all locations.

Searches in order: crew → rig → town → plugins.
Warns about duplicates found in multiple locations.

Examples:
  ao ratchet find "research/*.md"
  ao ratchet find "specs/*-v2.md"
  ao ratchet find "learnings/*.md" --json`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetFind,
	}
	ratchetCmd.AddCommand(findSubCmd)
}

// runRatchetFind searches for artifacts.
func runRatchetFind(cmd *cobra.Command, args []string) error {
	pattern := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	locator, err := ratchet.NewLocator(cwd)
	if err != nil {
		return fmt.Errorf("create locator: %w", err)
	}

	result, err := locator.Find(pattern)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default:
		if len(result.Matches) == 0 {
			fmt.Println("No matches found")
			return nil
		}

		fmt.Printf("Found %d match(es) for: %s\n\n", len(result.Matches), pattern)

		for _, match := range result.Matches {
			fmt.Printf("[%s] %s\n", match.Location, match.Path)
		}

		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warn := range result.Warnings {
				fmt.Printf("  ! %s\n", warn)
			}
		}
	}

	return nil
}
