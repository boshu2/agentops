package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	evalsub "github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

var (
	evalSuiteVerdictBootstrapInputs string
	evalSuiteVerdictMDE             float64
	evalSuiteVerdictB               int
	evalSuiteVerdictNRequired       int
)

var evalSuiteCmd = &cobra.Command{
	Use:   "suite",
	Short: "Suite-level operations (verdict, n-required)",
	Long: `Suite-level operations against the §6.5 statistical contract.

Subcommands:

  ao eval suite verdict <suite-id> --arms a,b --inputs <bootstrap-inputs.json>
       Compute the §6.5 paired cluster-bootstrap verdict for two Run arms.
       Reads canonical-JSON inputs (one row per (sample_id, seed) pair with
       both arm scores), derives bootstrap_seed deterministically per §6.5,
       runs B=10000 resamples (override via --B), maps to the §6.5 5-verdict
       enum, and emits all manifest fields the run writer needs.

  ao eval suite n-required --baseline-rate <p> --mde <delta> --alpha <a>
       Compute power-derived n_required (Day-3 graduates gate #6 to use this
       instead of Task.stats.min_n_samples).

The actual bootstrap math + RNG (numpy.random.PCG64) lives in
~/.agents/evals/_stats/. The Go CLI shells out so both Go and shell-script
callers go through the same code path — bit-exact reproducibility is a
property of the Python module, not the CLI surface.`,
}

var evalSuiteVerdictCmd = &cobra.Command{
	Use:   "verdict <suite-id> --arms a,b --inputs <bootstrap-inputs.json>",
	Short: "Compute the §6.5 paired cluster-bootstrap verdict",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalSuiteVerdict,
}

func runEvalSuiteVerdict(cmd *cobra.Command, args []string) error {
	suiteID := args[0]
	if evalSuiteVerdictBootstrapInputs == "" {
		return fmt.Errorf("eval suite verdict: --inputs is required")
	}
	suite, err := loadSuite(suiteID)
	if err != nil {
		// Suite ID may not be on disk yet (ad-hoc verdict); skip lookup if so.
		suite = nil
	}

	armsCSV := strings.Join(extractArmIDs(suite, evalSuiteVerdictArmsRaw), ",")
	if strings.TrimSpace(armsCSV) == "" {
		return fmt.Errorf("eval suite verdict: --arms required when suite has no varied_axis on disk")
	}

	rule := decisionRuleJSON(suite)
	nReq := evalSuiteVerdictNRequired
	if nReq <= 0 {
		nReq = derivedNRequired(suite)
	}

	pyArgs := []string{
		"-m", "_stats.cli", "verdict",
		"--suite-id", suiteID,
		"--arms", armsCSV,
		"--inputs", evalSuiteVerdictBootstrapInputs,
		"--decision-rule", rule,
		"--n-required", fmt.Sprintf("%d", nReq),
		"--B", fmt.Sprintf("%d", evalSuiteVerdictB),
	}
	if evalSuiteVerdictMDE > 0 {
		pyArgs = append(pyArgs, "--mde", fmt.Sprintf("%g", evalSuiteVerdictMDE))
	}

	out, err := runStatsCLI(pyArgs)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		_, _ = cmd.OutOrStdout().Write(out)
		_, _ = cmd.OutOrStdout().Write([]byte("\n"))
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("eval suite verdict: parse stats output: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"Suite verdict: %v\n  delta_point: %v\n  ci: [%v, %v]\n  n_clusters: %v\n  n_required: %v\n  bootstrap_seed: %v\n  bootstrap_inputs_hash: %v\n",
		result["verdict"], result["delta_point"], result["ci_low"], result["ci_high"],
		result["n_clusters"], result["n_required"], result["bootstrap_seed"],
		result["bootstrap_inputs_hash"])
	return nil
}

var evalSuiteNRequiredCmd = &cobra.Command{
	Use:   "n-required",
	Short: "Compute power-derived n_required (gate #6 input on Day 3+)",
	RunE:  runEvalSuiteNRequired,
}

var (
	evalSuiteNRReqBaselineRate float64
	evalSuiteNRReqMDE          float64
	evalSuiteNRReqAlpha        float64
	evalSuiteNRReqPower        float64
	evalSuiteNRReqPaired       bool
)

func runEvalSuiteNRequired(cmd *cobra.Command, args []string) error {
	pyArgs := []string{
		"-m", "_stats.cli", "n-required",
		"--baseline-rate", fmt.Sprintf("%g", evalSuiteNRReqBaselineRate),
		"--mde", fmt.Sprintf("%g", evalSuiteNRReqMDE),
		"--alpha", fmt.Sprintf("%g", evalSuiteNRReqAlpha),
		"--power", fmt.Sprintf("%g", evalSuiteNRReqPower),
		"--paired", fmt.Sprintf("%v", evalSuiteNRReqPaired),
	}
	out, err := runStatsCLI(pyArgs)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		_, _ = cmd.OutOrStdout().Write(out)
		_, _ = cmd.OutOrStdout().Write([]byte("\n"))
		return nil
	}
	var got struct {
		NRequired int `json:"n_required"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		return fmt.Errorf("eval suite n-required: parse: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "n_required: %d  (baseline_rate=%g MDE=%g alpha=%g power=%g paired=%v)\n",
		got.NRequired, evalSuiteNRReqBaselineRate, evalSuiteNRReqMDE,
		evalSuiteNRReqAlpha, evalSuiteNRReqPower, evalSuiteNRReqPaired)
	return nil
}

var evalSuiteVerdictArmsRaw string

func registerEvalSuiteCmd() {
	evalSuiteVerdictCmd.Flags().StringVar(&evalSuiteVerdictArmsRaw, "arms", "", "Comma-separated arm ids (default: from suite varied_axis)")
	evalSuiteVerdictCmd.Flags().StringVar(&evalSuiteVerdictBootstrapInputs, "inputs", "", "Path to canonical bootstrap-inputs JSON (REQUIRED)")
	evalSuiteVerdictCmd.Flags().Float64Var(&evalSuiteVerdictMDE, "mde", 0, "Minimum detectable effect (used for inconclusive_high_variance)")
	evalSuiteVerdictCmd.Flags().IntVar(&evalSuiteVerdictB, "B", 10000, "Bootstrap resamples")
	evalSuiteVerdictCmd.Flags().IntVar(&evalSuiteVerdictNRequired, "n-required", 0, "Override n_required (default: derived from suite power block)")

	evalSuiteNRequiredCmd.Flags().Float64Var(&evalSuiteNRReqBaselineRate, "baseline-rate", 0.5, "Baseline rate (binomial worst-case fallback)")
	evalSuiteNRequiredCmd.Flags().Float64Var(&evalSuiteNRReqMDE, "mde", 0.05, "Minimum detectable effect")
	evalSuiteNRequiredCmd.Flags().Float64Var(&evalSuiteNRReqAlpha, "alpha", 0.05, "Type-I error rate")
	evalSuiteNRequiredCmd.Flags().Float64Var(&evalSuiteNRReqPower, "power", 0.80, "Statistical power (1-beta)")
	evalSuiteNRequiredCmd.Flags().BoolVar(&evalSuiteNRReqPaired, "paired", true, "Paired comparison")

	evalSuiteCmd.AddCommand(evalSuiteVerdictCmd, evalSuiteNRequiredCmd)
}

func extractArmIDs(suite *evalsub.Suite, override string) []string {
	if override != "" {
		parts := strings.Split(override, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	if suite != nil && len(suite.VariedAxis.Values) > 0 {
		return suite.VariedAxis.Values
	}
	return nil
}

func decisionRuleJSON(suite *evalsub.Suite) string {
	rule := map[string]interface{}{"kind": "ci_excludes_zero", "confidence": 0.95}
	if suite != nil {
		if suite.Stats.DecisionRule.Kind != "" {
			rule["kind"] = suite.Stats.DecisionRule.Kind
		}
		if suite.Stats.DecisionRule.Confidence > 0 {
			rule["confidence"] = suite.Stats.DecisionRule.Confidence
		}
		if suite.Stats.DecisionRule.MinDelta > 0 {
			rule["min_delta"] = suite.Stats.DecisionRule.MinDelta
		}
	}
	bs, _ := json.Marshal(rule)
	return string(bs)
}

func derivedNRequired(suite *evalsub.Suite) int {
	if suite == nil || suite.Stats.Power == nil {
		return 0
	}
	pyArgs := []string{
		"-m", "_stats.cli", "n-required",
		"--baseline-rate", "0.5",
		"--mde", fmt.Sprintf("%g", suite.Stats.Power.MinimumDetectableEffect),
		"--alpha", fmt.Sprintf("%g", suite.Stats.Power.Alpha),
		"--power", "0.80",
		"--paired", "true",
	}
	out, err := runStatsCLI(pyArgs)
	if err != nil {
		return 0
	}
	var got struct {
		NRequired int `json:"n_required"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		return 0
	}
	return got.NRequired
}

// runStatsCLI shells out to the substrate venv's `python -m _stats.cli ...`.
// Looks for AGENTOPS_EVALS_VENV (override) -> $AGENTOPS_EVALS_ROOT/.venv/bin/python ->
// ~/.agents/evals/.venv/bin/python.
func runStatsCLI(args []string) ([]byte, error) {
	py := pythonBinary()
	if py == "" {
		return nil, fmt.Errorf("eval suite: substrate venv not found; provision via `python3 -m venv ~/.agents/evals/.venv && pip install numpy scipy`")
	}
	root := evalsRoot()
	cmd := exec.Command(py, args...)
	// PYTHONPATH so `_stats` is importable.
	cmd.Env = append(os.Environ(), "PYTHONPATH="+root)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return nil, fmt.Errorf("eval suite: stats CLI failed: %w (stderr: %s)", err, stderr)
	}
	return out, nil
}

func pythonBinary() string {
	if v := strings.TrimSpace(os.Getenv("AGENTOPS_EVALS_VENV")); v != "" {
		return v
	}
	candidates := []string{
		filepath.Join(evalsRoot(), ".venv", "bin", "python"),
		filepath.Join(evalsRoot(), ".venv", "bin", "python3"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".agents", "evals", ".venv", "bin", "python"),
			filepath.Join(home, ".agents", "evals", ".venv", "bin", "python3"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

