// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var scenarioListStatus string

var scenarioListCmd = &cobra.Command{
	Use:   "list",
	Short: "List holdout scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		holdoutDir := filepath.Join(".agents", "holdout")
		entries, err := os.ReadDir(holdoutDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No holdout directory found. Run 'ao scenario init' first.")
				return nil
			}
			return fmt.Errorf("reading holdout directory: %w", err)
		}

		type scenarioSummary struct {
			ID     string `json:"id"`
			Goal   string `json:"goal"`
			Status string `json:"status"`
			Date   string `json:"date"`
		}

		var scenarios []scenarioSummary
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(holdoutDir, entry.Name()))
			if err != nil {
				continue
			}
			var s struct {
				ID     string `json:"id"`
				Goal   string `json:"goal"`
				Status string `json:"status"`
				Date   string `json:"date"`
			}
			if err := json.Unmarshal(data, &s); err != nil {
				continue
			}
			if scenarioListStatus != "" && s.Status != scenarioListStatus {
				continue
			}
			scenarios = append(scenarios, scenarioSummary{
				ID:     s.ID,
				Goal:   s.Goal,
				Status: s.Status,
				Date:   s.Date,
			})
		}

		if len(scenarios) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No scenarios found.")
			return nil
		}

		out, _ := json.MarshalIndent(scenarios, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	},
}

func init() {
	scenarioListCmd.Flags().StringVar(&scenarioListStatus, "status", "", "Filter by status (active, draft, retired)")
	scenarioCmd.AddCommand(scenarioListCmd)
}
