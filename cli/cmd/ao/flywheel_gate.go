// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const flywheelGateMinRho = 0.55

var flywheelGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Check readiness for retrieval-expansion work",
	Long: `Check the post-structural readiness gate before retrieval-expansion work.

The gate passes only when:
  - Research closure has exited "unmined"
  - rho is at least 0.55
  - Holdout retrieval Precision@K meets the benchmark target

Examples:
  ao flywheel gate
  ao flywheel gate --corpus cli/cmd/ao/testdata/retrieval-bench`,
	RunE: runFlywheelGate,
}

type flywheelGateBenchmarkSummary struct {
	Corpus             string  `json:"corpus"`
	Queries            int     `json:"queries"`
	HoldoutCases       int     `json:"holdout_cases"`
	HoldoutPrecision   float64 `json:"holdout_precision_at_k"`
	HoldoutMRR         float64 `json:"holdout_mrr,omitempty"`
	TargetPrecisionAtK float64 `json:"target_precision_at_k"`
}

type flywheelGateResult struct {
	Pass           bool                         `json:"pass"`
	Reasons        []string                     `json:"reasons,omitempty"`
	ClosureVerdict string                       `json:"closure_verdict"`
	Rho            float64                      `json:"rho"`
	RhoThreshold   float64                      `json:"rho_threshold"`
	Benchmark      flywheelGateBenchmarkSummary `json:"benchmark"`
}

func init() {
	flywheelCmd.AddCommand(flywheelGateCmd)
	flywheelGateCmd.Flags().String("corpus", "", "Benchmark corpus directory (defaults to repo testdata)")
}

func runFlywheelGate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cwd = canonicalFlywheelWorkspacePath(cwd)

	corpusFlag, err := cmd.Flags().GetString("corpus")
	if err != nil {
		return fmt.Errorf("read corpus flag: %w", err)
	}
	corpusDir, err := resolveRetrievalBenchCorpus(cwd, corpusFlag)
	if err != nil {
		return err
	}

	metrics, err := computeMetrics(cwd, 7)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	populateGoldenSignals(cwd, 7, metrics)

	report, err := buildBenchReport(corpusDir, corpusDir, 3)
	if err != nil {
		return fmt.Errorf("build retrieval benchmark: %w", err)
	}

	result := evaluateFlywheelGate(metrics, report, corpusDir)

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	case "yaml":
		enc := yaml.NewEncoder(cmd.OutOrStdout())
		if err := enc.Encode(result); err != nil {
			_ = enc.Close()
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
	default:
		printFlywheelGateResult(cmd.OutOrStdout(), result)
	}

	if result.Pass {
		return nil
	}
	return fmt.Errorf("flywheel gate failed: %s", strings.Join(result.Reasons, "; "))
}

func evaluateFlywheelGate(metrics *types.FlywheelMetrics, report benchReport, corpus string) flywheelGateResult {
	result := flywheelGateResult{
		RhoThreshold: flywheelGateMinRho,
		Benchmark: flywheelGateBenchmarkSummary{
			Corpus:             corpus,
			Queries:            report.Queries,
			TargetPrecisionAtK: report.TargetPAtK,
		},
	}

	if metrics != nil {
		result.Rho = metrics.Rho
		if metrics.GoldenSignals != nil {
			result.ClosureVerdict = metrics.GoldenSignals.ClosureVerdict
		}
	}

	if holdout, ok := report.Splits["holdout"]; ok {
		result.Benchmark.HoldoutCases = holdout.Cases
		result.Benchmark.HoldoutPrecision = holdout.AvgPAtK
		result.Benchmark.HoldoutMRR = holdout.AvgMRR
	}

	switch result.ClosureVerdict {
	case "":
		result.Reasons = append(result.Reasons, "missing research-closure verdict")
	case "unmined", "starved":
		result.Reasons = append(result.Reasons, "research closure still unmined")
	}

	if result.Rho < result.RhoThreshold {
		result.Reasons = append(result.Reasons, fmt.Sprintf("rho %.2f < %.2f", result.Rho, result.RhoThreshold))
	}

	if result.Benchmark.HoldoutCases == 0 {
		result.Reasons = append(result.Reasons, "no holdout benchmark cases")
	} else if result.Benchmark.HoldoutPrecision < result.Benchmark.TargetPrecisionAtK {
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("holdout precision %.2f < baseline %.2f", result.Benchmark.HoldoutPrecision, result.Benchmark.TargetPrecisionAtK))
	}

	result.Pass = len(result.Reasons) == 0
	return result
}

func printFlywheelGateResult(w io.Writer, result flywheelGateResult) {
	fmt.Fprintln(w)
	if result.Pass {
		fmt.Fprintln(w, "  Flywheel Gate: PASS")
	} else {
		fmt.Fprintln(w, "  Flywheel Gate: FAIL")
	}
	fmt.Fprintln(w, "  ─────────────────────")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "  Research closure: %s\n", emptyOrDash(result.ClosureVerdict))
	fmt.Fprintf(w, "  rho:              %.2f (threshold %.2f)\n", result.Rho, result.RhoThreshold)
	fmt.Fprintf(w, "  Holdout P@K:      %.2f (baseline %.2f)\n",
		result.Benchmark.HoldoutPrecision, result.Benchmark.TargetPrecisionAtK)
	fmt.Fprintf(w, "  Holdout cases:    %d\n", result.Benchmark.HoldoutCases)
	fmt.Fprintf(w, "  Corpus:           %s\n", result.Benchmark.Corpus)
	if !result.Pass && len(result.Reasons) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Reasons:")
		for _, reason := range result.Reasons {
			fmt.Fprintf(w, "    - %s\n", reason)
		}
	}
	fmt.Fprintln(w)
}

func emptyOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func canonicalFlywheelWorkspacePath(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS != "darwin" {
		return clean
	}
	if !strings.HasPrefix(clean, "/private/") {
		return clean
	}
	trimmed := strings.TrimPrefix(clean, "/private")
	if _, err := os.Stat(trimmed); err == nil {
		return trimmed
	}
	return clean
}
