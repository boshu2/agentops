package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var scenarioValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate holdout scenarios against schema",
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

		var validationErrors []string
		var validated int

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			filePath := filepath.Join(holdoutDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: read error: %v", entry.Name(), err))
				continue
			}

			var s map[string]interface{}
			if err := json.Unmarshal(data, &s); err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid JSON: %v", entry.Name(), err))
				continue
			}

			// Check required fields
			required := []string{"id", "version", "date", "goal", "narrative", "expected_outcome", "satisfaction_threshold", "status"}
			for _, field := range required {
				if _, ok := s[field]; !ok {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: missing required field '%s'", entry.Name(), field))
				}
			}

			// Validate id pattern
			if id, ok := s["id"].(string); ok {
				if !scenarioIDPattern.MatchString(id) {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: id '%s' does not match pattern s-YYYY-MM-DD-NNN or auto-*", entry.Name(), id))
				}
			}

			// Validate status
			if status, ok := s["status"].(string); ok {
				if !validScenarioStatus(status) {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid status '%s' (must be active, draft, or retired)", entry.Name(), status))
				}
			}

			// Validate source if present
			if source, ok := s["source"].(string); ok {
				if !validScenarioSource(source) {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid source '%s' (must be human, agent, or prod-telemetry)", entry.Name(), source))
				}
			}

			// Validate satisfaction_threshold range
			if threshold, ok := s["satisfaction_threshold"].(float64); ok {
				if threshold < 0 || threshold > 1 {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: satisfaction_threshold %.2f out of range [0, 1]", entry.Name(), threshold))
				}
			}

			validated++
		}

		if len(validationErrors) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "Validation errors:\n%s\n", strings.Join(validationErrors, "\n"))
			return fmt.Errorf("%d validation error(s) found", len(validationErrors))
		}

		if validated == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No scenario files found to validate.")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Validated %d scenario(s): all pass\n", validated)
		}
		return nil
	},
}

func init() {
	scenarioCmd.AddCommand(scenarioValidateCmd)
}
