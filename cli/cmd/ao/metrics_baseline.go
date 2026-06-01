// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/domain"
	"github.com/boshu2/agentops/cli/internal/format"
	"github.com/spf13/cobra"
)

// runMetricsBaseline captures a baseline snapshot.
func runMetricsBaseline(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would capture baseline for %d day period\n", metricsDays)
		return nil
	}

	metrics, err := computeMetrics(cwd, metricsDays)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	populateGoldenSignals(cwd, metricsDays, metrics)

	// Save baseline
	baselinePath, err := saveBaseline(cwd, metrics)
	if err != nil {
		return fmt.Errorf("save baseline: %w", err)
	}

	// Output
	switch GetOutput() {
	case "json":
		return format.EncodeJSON(os.Stdout, metrics)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(metrics)

	default:
		printMetricsTable(metrics)
		fmt.Printf("\nBaseline saved: %s\n", baselinePath)
	}

	return nil
}

// saveBaseline saves metrics to a baseline file.
func saveBaseline(baseDir string, metrics *domain.FlywheelMetrics) (string, error) {
	metricsDir := filepath.Join(baseDir, ".agents", "ao", "metrics")
	if err := os.MkdirAll(metricsDir, 0700); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("baseline-%s.json", metrics.Timestamp.Format("2006-01-02"))
	path := filepath.Join(metricsDir, filename)

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}

	return path, nil
}
