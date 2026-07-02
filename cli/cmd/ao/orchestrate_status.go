//go:build legacy

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var orchestrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "List active ATM sessions with degraded hints",
	RunE:  runOrchestrateStatus,
}

func runOrchestrateStatus(cmd *cobra.Command, _ []string) error {
	out, err := exec.CommandContext(cmd.Context(), "atm", "list").Output()
	if err != nil {
		return fmt.Errorf("atm list: %w", err)
	}
	if orchestrateJSON {
		type row struct {
			Line string `json:"line"`
		}
		var rows []row
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			rows = append(rows, row{Line: line})
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"schema_version": 1,
			"command":        "status",
			"sessions":       rows,
		})
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return err
}

func initOrchestrateStatus() {
	orchestrateStatusCmd.Flags().BoolVar(&orchestrateJSON, "json", false, "Emit JSON")
	orchestrateCmd.AddCommand(orchestrateStatusCmd)
}
