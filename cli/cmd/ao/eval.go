package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

var (
	evalRunOutput      string
	evalRunID          string
	evalRunRuntime     string
	evalRunBaseline    string
	evalCompareOutput  string
	evalCompareMaxAgg  float64
	evalCompareMaxDim  float64
	evalBaselineOutput string
	evalBaselineBy     string
	evalBaselineReason string
	evalConfigured     bool
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run deterministic local evaluation suites",
	Long: `Run deterministic AgentOps evaluation suites and compare run records.

The eval surface intentionally supports only offline deterministic runs in this
release. Live Claude and Codex adapters are evaluated by a later runtime tier.`,
}

var evalRunCmd = &cobra.Command{
	Use:   "run <suite.json>",
	Short: "Run a deterministic eval suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runtimeName, err := parseEvalRuntime(evalRunRuntime)
		if err != nil {
			return err
		}
		run, err := aoeval.RunSuite(aoeval.RunOptions{
			SuitePath:    args[0],
			RunID:        evalRunID,
			Runtime:      runtimeName,
			OutputPath:   evalRunOutput,
			BaselinePath: evalRunBaseline,
		})
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, run)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval %s: %s (aggregate %.4f, cases %d)\n", run.RunID, run.Status, run.AggregateScore, len(run.CaseResults))
		if evalRunOutput != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Run record: %s\n", evalRunOutput)
		}
		return nil
	},
}

var evalCompareCmd = &cobra.Command{
	Use:   "compare <candidate-run.json> <baseline-run.json>",
	Short: "Compare an eval run against a baseline",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidate, err := aoeval.LoadRun(args[0])
		if err != nil {
			return err
		}
		baseline, err := aoeval.LoadRun(args[1])
		if err != nil {
			return err
		}
		compared, err := aoeval.CompareRuns(candidate, baseline, aoeval.CompareOptions{
			MaxAggregateRegression: evalCompareMaxAgg,
			MaxDimensionRegression: evalCompareMaxDim,
			OutputPath:             evalCompareOutput,
		})
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, compared)
		}
		delta := 0.0
		if compared.BaselineComparison != nil {
			delta = compared.BaselineComparison.AggregateDelta
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval compare %s vs %s: %s (aggregate delta %.4f)\n", compared.RunID, baseline.RunID, compared.Verdict, delta)
		if evalCompareOutput != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Comparison record: %s\n", evalCompareOutput)
		}
		return nil
	},
}

var evalBaselineCmd = &cobra.Command{
	Use:   "baseline <run.json>",
	Short: "Promote an eval run record as a baseline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		run, err := aoeval.LoadRun(args[0])
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		promoted, err := aoeval.PromoteBaseline(run, aoeval.BaselineOptions{
			OutputPath: evalBaselineOutput,
			PromotedBy: evalBaselineBy,
			Rationale:  evalBaselineReason,
			WorkDir:    cwd,
		})
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, promoted)
		}
		path := ""
		if promoted.Baseline != nil {
			path = promoted.Baseline.BaselinePath
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval baseline promoted: %s\n", path)
		return nil
	},
}

func init() {
	registerEvalCommand()
}

func registerEvalCommand() {
	if !evalConfigured {
		configureEvalCommand()
		evalConfigured = true
	}
	if evalCmd.Parent() == nil {
		rootCmd.AddCommand(evalCmd)
	}
}

func configureEvalCommand() {
	evalCmd.GroupID = "workflow"

	evalRunCmd.Flags().StringVar(&evalRunOutput, "out", "", "write eval run record to path")
	evalRunCmd.Flags().StringVar(&evalRunID, "run-id", "", "stable run id to use in the run record")
	evalRunCmd.Flags().StringVar(&evalRunRuntime, "runtime", "", "deterministic runtime override (static, mock, shell)")
	evalRunCmd.Flags().StringVar(&evalRunBaseline, "baseline", "", "compare the run against a baseline run record")
	_ = evalRunCmd.RegisterFlagCompletionFunc("runtime", staticCompletionFunc("static", "mock", "shell"))

	evalCompareCmd.Flags().StringVar(&evalCompareOutput, "out", "", "write compared eval run record to path")
	evalCompareCmd.Flags().Float64Var(&evalCompareMaxAgg, "max-aggregate-regression", 0, "allowed aggregate regression before verdict becomes regression")
	evalCompareCmd.Flags().Float64Var(&evalCompareMaxDim, "max-dimension-regression", 0, "allowed per-dimension regression before verdict becomes regression")

	evalBaselineCmd.Flags().StringVar(&evalBaselineOutput, "out", "", "write promoted baseline run record to path")
	evalBaselineCmd.Flags().StringVar(&evalBaselineBy, "promoted-by", "", "identity promoting the baseline")
	evalBaselineCmd.Flags().StringVar(&evalBaselineReason, "rationale", "", "rationale for promoting the baseline")

	evalCmd.AddCommand(evalRunCmd, evalCompareCmd, evalBaselineCmd)
}

func parseEvalRuntime(value string) (aoeval.Runtime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	switch aoeval.Runtime(value) {
	case aoeval.RuntimeStatic, aoeval.RuntimeMock, aoeval.RuntimeShell:
		return aoeval.Runtime(value), nil
	default:
		return "", fmt.Errorf("runtime %q is out of deterministic scope (use static, mock, or shell)", value)
	}
}

func writeEvalJSON(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
