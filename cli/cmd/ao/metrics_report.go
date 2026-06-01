// practices: [dora-metrics, sre]
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/format"
	"github.com/spf13/cobra"
)

// runMetricsReport shows the metrics report.
func runMetricsReport(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	metrics, err := computeMetrics(cwd, metricsDays)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	populateGoldenSignals(cwd, metricsDays, metrics)

	switch GetOutput() {
	case "json":
		return format.EncodeJSON(os.Stdout, metrics)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(metrics)

	default:
		printMetricsTable(metrics)
	}

	return nil
}
