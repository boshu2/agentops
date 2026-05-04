package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

var (
	evalRunOutput         string
	evalRunID             string
	evalRunRuntime        string
	evalRunBaseline       string
	evalRunBaselineMode   string
	evalRunContextMode    string
	evalRunContextOffDir  string
	evalRunContextOnDir   string
	evalRunDeltaOut       string
	evalCompareOutput     string
	evalCompareMaxAgg     float64
	evalCompareMaxDim     float64
	evalScorecardOutput   string
	evalScorecardKind     string
	evalScorecardMaxCat   float64
	evalBaselineOutput    string
	evalBaselineBy        string
	evalBaselineReason    string
	evalBaselineAuditRoot string
	evalBaselineAuditDir  string
	evalCoverageRoot      string
	evalCoverageDomains   []string
	evalCoverageEvidence  []string
	evalCoverageDims      []string
	evalCoverageRuntimes  []string
	evalConfigured        bool
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
		mode := aoeval.ABBaselineMode(evalRunBaselineMode)
		if !aoeval.IsValidBaselineMode(string(mode)) {
			return fmt.Errorf("invalid --baseline-mode %q (allowed: %s)", evalRunBaselineMode, strings.Join(aoeval.AllBaselineModes(), ", "))
		}
		contextMode := aoeval.ContextMode(evalRunContextMode)
		if !aoeval.IsValidContextMode(string(contextMode)) {
			return fmt.Errorf("invalid --context-mode %q (allowed: %s)", evalRunContextMode, strings.Join(aoeval.AllContextModes(), ", "))
		}
		baseOpts := aoeval.RunOptions{
			SuitePath:    args[0],
			RunID:        evalRunID,
			Runtime:      runtimeName,
			OutputPath:   evalRunOutput,
			BaselinePath: evalRunBaseline,
		}
		if contextMode == aoeval.ContextModeAB {
			if mode != aoeval.BaselineModeSkillOn {
				return fmt.Errorf("--context-mode=ab cannot be combined with --baseline-mode=%s", mode)
			}
			contextOpts := resolveEvalContextABOptions(args[0], evalRunContextOffDir, evalRunContextOnDir)
			scorecard, offRun, onRun, err := aoeval.RunContextAB(baseOpts, contextOpts)
			if err != nil {
				return err
			}
			if err := aoeval.WriteContextDeltaScorecard(scorecard, evalRunDeltaOut); err != nil {
				return err
			}
			if GetOutput() == "json" {
				return writeEvalJSON(cmd, scorecard)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Eval context-AB %s: context-off=%.4f (%s) context-on=%.4f (%s) delta=%+.4f cases=%d\n",
				scorecard.SuiteID,
				offRun.AggregateScore, offRun.Status,
				onRun.AggregateScore, onRun.Status,
				scorecard.AggregateDelta,
				len(scorecard.PerCase),
			)
			if evalRunDeltaOut != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Context delta scorecard: %s\n", evalRunDeltaOut)
			}
			return nil
		}
		if mode == aoeval.BaselineModeBoth {
			scorecard, onRun, offRun, err := aoeval.RunBaselineAB(baseOpts)
			if err != nil {
				return err
			}
			if err := aoeval.WriteDeltaScorecard(scorecard, evalRunDeltaOut); err != nil {
				return err
			}
			if GetOutput() == "json" {
				return writeEvalJSON(cmd, scorecard)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Eval baseline-AB %s: skill-on=%.4f (%s) skill-off=%.4f (%s) delta=%+.4f cases=%d\n",
				scorecard.SuiteID,
				onRun.AggregateScore, onRun.Status,
				offRun.AggregateScore, offRun.Status,
				scorecard.AggregateDelta,
				len(scorecard.PerCase),
			)
			if evalRunDeltaOut != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Delta scorecard: %s\n", evalRunDeltaOut)
			}
			return nil
		}
		opts := baseOpts
		opts.OverrideDisableHooks = (mode == aoeval.BaselineModeSkillOff)
		run, err := aoeval.RunSuite(opts)
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

var evalBaselineAuditCmd = &cobra.Command{
	Use:   "baseline-audit [suite.json ...]",
	Short: "Audit eval suite baseline policy against promoted baselines",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		roots := []string{}
		if len(args) == 0 {
			roots = append(roots, evalBaselineAuditRoot)
		}
		report, err := aoeval.AuditBaselinePolicy(aoeval.BaselineAuditOptions{
			SuitePaths:  args,
			Roots:       roots,
			BaselineDir: evalBaselineAuditDir,
		})
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, report)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval baseline audit: %d suites, %d baselines, %d policy mismatches\n", report.SuiteCount, report.BaselineCount, report.PolicyMismatchCount)
		if len(report.MissingCompareBaselines) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Missing compare baselines: %d\n", len(report.MissingCompareBaselines))
		}
		if len(report.UnexpectedBaselinesForNone) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Unexpected baselines for none policy: %d\n", len(report.UnexpectedBaselinesForNone))
		}
		if len(report.OrphanBaselines) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Orphan baselines: %d\n", len(report.OrphanBaselines))
		}
		if len(report.StaleSuiteHashes) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Stale suite hashes: %d\n", len(report.StaleSuiteHashes))
		}
		return nil
	},
}

var evalScorecardCmd = &cobra.Command{
	Use:   "scorecard <candidate-run.json> [baseline-run.json]",
	Short: "Build an eval scorecard from run records",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, err := parseScorecardKind(evalScorecardKind)
		if err != nil {
			return err
		}
		candidate, err := aoeval.LoadRun(args[0])
		if err != nil {
			return err
		}
		var baseline *aoeval.RunRecord
		if len(args) == 2 {
			baseline, err = aoeval.LoadRun(args[1])
			if err != nil {
				return err
			}
		}
		scorecard, err := aoeval.BuildScorecard(candidate, baseline, aoeval.ScorecardOptions{
			Kind:                  kind,
			MaxCategoryRegression: evalScorecardMaxCat,
		})
		if err != nil {
			return err
		}
		if evalScorecardOutput != "" {
			if err := aoeval.WriteScorecard(evalScorecardOutput, scorecard); err != nil {
				return err
			}
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, scorecard)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval scorecard %s: %s (%s, categories %d)\n", scorecard.CandidateRunID, scorecard.Verdict, scorecard.Kind, len(scorecard.Categories))
		if evalScorecardOutput != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Scorecard: %s\n", evalScorecardOutput)
		}
		return nil
	},
}

var evalCoverageCmd = &cobra.Command{
	Use:   "coverage [suite.json ...]",
	Short: "Summarize eval suite coverage",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		roots := []string{}
		if len(args) == 0 {
			roots = append(roots, evalCoverageRoot)
		}
		report, err := aoeval.BuildCoverageReport(aoeval.CoverageOptions{
			SuitePaths:            args,
			Roots:                 roots,
			RequiredDomains:       evalCoverageDomains,
			RequiredEvidenceKinds: evalCoverageEvidence,
			RequiredDimensions:    evalCoverageDims,
			RequiredRuntimes:      evalCoverageRuntimes,
		})
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, report)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Eval coverage: %d suites, %d cases, %d critical cases\n", report.SuiteCount, report.CaseCount, report.CriticalCaseCount)
		if len(report.MissingRequiredDomains) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Missing required domains: %s\n", strings.Join(report.MissingRequiredDomains, ", "))
		} else if len(report.RequiredDomains) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Required domains covered")
		}
		if len(report.MissingRequiredEvidenceKinds) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Missing required evidence kinds: %s\n", strings.Join(report.MissingRequiredEvidenceKinds, ", "))
		} else if len(report.RequiredEvidenceKinds) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Required evidence kinds covered")
		}
		if len(report.MissingRequiredDimensions) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Missing required dimensions: %s\n", strings.Join(report.MissingRequiredDimensions, ", "))
		} else if len(report.RequiredDimensions) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Required dimensions covered")
		}
		if len(report.MissingRequiredRuntimes) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Missing required runtimes: %s\n", strings.Join(report.MissingRequiredRuntimes, ", "))
		} else if len(report.RequiredRuntimes) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Required runtimes covered")
		}
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
	evalRunCmd.Flags().StringVar(&evalRunRuntime, "runtime", "", "runtime override (static, mock, shell, claude, codex)")
	evalRunCmd.Flags().StringVar(&evalRunBaseline, "baseline", "", "compare the run against a baseline run record")
	evalRunCmd.Flags().StringVar(&evalRunBaselineMode, "baseline-mode", string(aoeval.BaselineModeSkillOn), "skill-on | skill-off | both — runs the suite once with skills loaded, once with hooks suppressed, or both for a delta scorecard")
	evalRunCmd.Flags().StringVar(&evalRunContextMode, "context-mode", string(aoeval.ContextModeNone), "none | ab — run context-off/context-on legs over isolated AO_AGENTS_DIR roots")
	evalRunCmd.Flags().StringVar(&evalRunContextOffDir, "context-off-agents-dir", "", "AO_AGENTS_DIR root for the context-off leg (defaults to suite fixtures)")
	evalRunCmd.Flags().StringVar(&evalRunContextOnDir, "context-on-agents-dir", "", "AO_AGENTS_DIR root for the context-on leg (defaults to suite fixtures)")
	evalRunCmd.Flags().StringVar(&evalRunDeltaOut, "delta-out", "", "write delta scorecard JSON to path (with --baseline-mode=both or --context-mode=ab)")
	_ = evalRunCmd.RegisterFlagCompletionFunc("runtime", staticCompletionFunc("static", "mock", "shell", "claude", "codex"))
	_ = evalRunCmd.RegisterFlagCompletionFunc("baseline-mode", staticCompletionFunc(aoeval.AllBaselineModes()...))
	_ = evalRunCmd.RegisterFlagCompletionFunc("context-mode", staticCompletionFunc(aoeval.AllContextModes()...))

	evalCompareCmd.Flags().StringVar(&evalCompareOutput, "out", "", "write compared eval run record to path")
	evalCompareCmd.Flags().Float64Var(&evalCompareMaxAgg, "max-aggregate-regression", 0, "allowed aggregate regression before verdict becomes regression")
	evalCompareCmd.Flags().Float64Var(&evalCompareMaxDim, "max-dimension-regression", 0, "allowed per-dimension regression before verdict becomes regression")

	evalBaselineCmd.Flags().StringVar(&evalBaselineOutput, "out", "", "write promoted baseline run record to path")
	evalBaselineCmd.Flags().StringVar(&evalBaselineBy, "promoted-by", "", "identity promoting the baseline")
	evalBaselineCmd.Flags().StringVar(&evalBaselineReason, "rationale", "", "rationale for promoting the baseline")

	evalBaselineAuditCmd.Flags().StringVar(&evalBaselineAuditRoot, "root", "evals/agentops-core", "suite root to scan when no suite paths are provided")
	evalBaselineAuditCmd.Flags().StringVar(&evalBaselineAuditDir, "baseline-dir", ".agents/evals/baselines", "promoted baseline directory")

	evalCoverageCmd.Flags().StringVar(&evalCoverageRoot, "root", "evals/agentops-core", "suite root to scan when no suite paths are provided")
	evalCoverageCmd.Flags().StringArrayVar(&evalCoverageDomains, "require-domain", aoeval.DefaultCoverageDomains, "required product domain for missing-domain reporting")
	evalCoverageCmd.Flags().StringArrayVar(&evalCoverageEvidence, "require-evidence-kind", nil, "required evidence kind for missing-evidence-kind reporting")
	evalCoverageCmd.Flags().StringArrayVar(&evalCoverageDims, "require-dimension", aoeval.DefaultCoverageDimensions, "required score dimension for missing-dimension reporting")
	evalCoverageCmd.Flags().StringArrayVar(&evalCoverageRuntimes, "require-runtime", aoeval.DefaultCoverageRuntimes, "required deterministic runtime for missing-runtime reporting")

	evalScorecardCmd.Flags().StringVar(&evalScorecardOutput, "out", "", "write scorecard JSON to path")
	evalScorecardCmd.Flags().StringVar(&evalScorecardKind, "kind", string(aoeval.ScorecardKindRPI), "scorecard kind (rpi, skill-change)")
	evalScorecardCmd.Flags().Float64Var(&evalScorecardMaxCat, "max-category-regression", 0, "allowed per-category regression before verdict becomes regression")
	_ = evalScorecardCmd.RegisterFlagCompletionFunc("kind", staticCompletionFunc(string(aoeval.ScorecardKindRPI), string(aoeval.ScorecardKindSkillChange)))

	evalCmd.AddCommand(evalRunCmd, evalCompareCmd, evalBaselineCmd, evalBaselineAuditCmd, evalScorecardCmd, evalCoverageCmd)

	registerEvalTaskCmd()
	registerEvalCleanupCmd()
	registerEvalSuiteCmd()
	evalCmd.AddCommand(evalTaskCmd, evalCleanupCmd, evalSuiteCmd)
}

func resolveEvalContextABOptions(suitePath, offDir, onDir string) aoeval.ContextABOptions {
	if offDir == "" {
		offDir = defaultEvalContextAgentsDir(suitePath, "context-off")
	}
	if onDir == "" {
		onDir = defaultEvalContextAgentsDir(suitePath, "context-on")
	}
	return aoeval.ContextABOptions{
		ContextOffAgentsDir: absoluteEvalContextAgentsDir(offDir),
		ContextOnAgentsDir:  absoluteEvalContextAgentsDir(onDir),
		ContextOffLabel:     "context-off",
		ContextOnLabel:      "context-on",
	}
}

func defaultEvalContextAgentsDir(suitePath, leg string) string {
	base := filepath.Base(suitePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(suitePath), "fixtures", name, leg, "agents")
}

func absoluteEvalContextAgentsDir(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func parseEvalRuntime(value string) (aoeval.Runtime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	switch aoeval.Runtime(value) {
	case aoeval.RuntimeStatic, aoeval.RuntimeMock, aoeval.RuntimeShell:
		return aoeval.Runtime(value), nil
	case aoeval.RuntimeClaude, aoeval.RuntimeCodex:
		return aoeval.Runtime(value), nil
	default:
		return "", fmt.Errorf("unknown runtime %q (use static, mock, shell, claude, or codex)", value)
	}
}

func parseScorecardKind(value string) (aoeval.ScorecardKind, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return aoeval.ScorecardKindRPI, nil
	}
	switch aoeval.ScorecardKind(value) {
	case aoeval.ScorecardKindRPI, aoeval.ScorecardKindSkillChange:
		return aoeval.ScorecardKind(value), nil
	default:
		return "", fmt.Errorf("unsupported scorecard kind %q (use rpi or skill-change)", value)
	}
}

func writeEvalJSON(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
