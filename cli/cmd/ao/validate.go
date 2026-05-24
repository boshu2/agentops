// practices: [continuous-delivery, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

// Verdict is the AgentOps validation verdict vocabulary (same as the
// `agentops:validate` skill and `ao ratchet`).
type verdict string

const (
	verdictPass verdict = "PASS"
	verdictWarn verdict = "WARN"
	verdictFail verdict = "FAIL"
)

// Gate exit codes. The exit code IS the verdict in --gate mode, deliberately
// distinguishing a clean FAIL (1) from a broken gate (2) so retry loops
// (GC check.max_attempts, CI, ao rpi's internal validate phase) never read a
// setup error as a passing or failing gate.
const (
	gateExitPass     = 0 // PASS or WARN (gate passes)
	gateExitFail     = 1 // FAIL (gate fails)
	gateExitInternal = 2 // internal/usage error (could not run the gate)
)

// gateExitError carries a gate exit code out through cobra's RunE so Execute()
// can map it to os.Exit. Reuses the typed-error + errors.As pattern
// (see doctorExitError, AgentsLintError).
type gateExitError struct {
	code int
	msg  string
}

func (e *gateExitError) Error() string { return e.msg }

// ExitCode returns the process exit code this error maps to.
func (e *gateExitError) ExitCode() int { return e.code }

// validateGateResult is the aggregated, machine-readable gate verdict.
type validateGateResult struct {
	Verdict  verdict  `json:"verdict"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
	GateExit int      `json:"gate_exit"`
}

// validate command flags.
var (
	validateGate          bool
	validateBead          string
	validateChanges       []string
	validateStrict        bool
	validateWarnAsFail    bool
	validateJSONOut       bool
	validateLenient       bool
	validateLenientExpiry int
)

func init() {
	validateCmd := &cobra.Command{
		Use:     "validate",
		GroupID: "workflow",
		Short:   "Umbrella validation gate (exit-code-as-verdict with --gate)",
		Long: `Run a deterministic validation gate over RPI artifacts and emit a single
PASS / WARN / FAIL verdict.

This is the umbrella gate; the existing sub-surfaces (ao ratchet validate,
ao scenario validate, ao goals validate) stay and can be delegated to. ao validate
composes the ratchet validator, aggregates to one verdict, and (with --gate) maps
that verdict onto the process exit code.

Deterministic: no network, no LLM. Safe as a GC check script, a CI step, or
ao rpi's internal validate phase.

Exit codes with --gate:
  0  PASS or WARN (gate passes)
  1  FAIL (gate fails)
  2  internal/usage error (could not run the gate — distinct from a clean FAIL)

WARN exits 0 by default (advisory). --strict / --warn-as-fail promotes WARN to
exit 1 for callers that want zero-warning gates.

Examples:
  ao validate --gate                       # gate the RPI artifacts in cwd
  ao validate --gate --changes plan.md     # gate explicit files
  ao validate --gate --bead soc-123        # gate a bead's artifacts
  ao validate --gate --strict              # WARN -> FAIL (exit 1)
  ao validate --json                       # structured verdict, exit 0`,
		Args: cobra.NoArgs,
		// The exit code carries the verdict; we map gateExitError in Execute().
		// Silence cobra's own error print so a FAIL/internal verdict doesn't emit
		// a spurious "Error:" line on top of our verdict output.
		SilenceErrors: true,
		RunE:          runValidate,
	}
	f := validateCmd.Flags()
	f.BoolVar(&validateGate, "gate", false, "Exit-code mode: 0=PASS/WARN, 1=FAIL, 2=error")
	f.StringVar(&validateBead, "bead", "", "Validate artifacts bound to a bead id")
	f.StringSliceVar(&validateChanges, "changes", nil, "Explicit files to validate")
	f.BoolVar(&validateStrict, "strict", false, "Promote WARN to FAIL (exit 1)")
	f.BoolVar(&validateWarnAsFail, "warn-as-fail", false, "Alias for --strict")
	f.BoolVar(&validateJSONOut, "json", false, "Structured verdict (honored in both modes)")
	f.BoolVar(&validateLenient, "lenient", false, "Allow legacy artifacts without schema_version")
	f.IntVar(&validateLenientExpiry, "lenient-expiry", 90, "Days until lenient bypass expires")
	rootCmd.AddCommand(validateCmd)
}

// runValidate executes the umbrella validation gate.
func runValidate(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return &gateExitError{code: gateExitInternal, msg: fmt.Sprintf("get working directory: %v", err)}
	}

	validator, err := ratchet.NewValidator(cwd)
	if err != nil {
		return &gateExitError{code: gateExitInternal, msg: fmt.Sprintf("create validator: %v", err)}
	}

	targets, err := resolveGateTargets(cwd)
	if err != nil {
		return &gateExitError{code: gateExitInternal, msg: err.Error()}
	}
	if len(targets) == 0 {
		return &gateExitError{code: gateExitInternal,
			msg: "no artifacts to validate (use --changes, --bead, or run from a directory with RPI outputs)"}
	}

	result := runGateValidation(validator, targets)
	return emitGateResult(cmd, result)
}

// gateTarget pairs a file path with the ratchet step that validates it.
type gateTarget struct {
	step ratchet.Step
	path string
}

// resolveGateTargets determines which (step, file) pairs to validate.
// Precedence: explicit --changes, then --bead, then auto-locate RPI outputs.
func resolveGateTargets(cwd string) ([]gateTarget, error) {
	if len(validateChanges) > 0 {
		return targetsFromChanges(cwd, validateChanges), nil
	}
	// --bead and the auto-locate path both walk the standard RPI steps and keep
	// whichever expected outputs exist on disk. --bead is reserved for future
	// bead-scoped narrowing; today both resolve via the ratchet locator, which is
	// deterministic and network-free.
	return locateRPITargets(cwd), nil
}

// targetsFromChanges maps explicit files onto inferred steps.
func targetsFromChanges(cwd string, files []string) []gateTarget {
	targets := make([]gateTarget, 0, len(files))
	for _, f := range files {
		targets = append(targets, gateTarget{step: inferStepForPath(cwd, f), path: f})
	}
	return targets
}

// locateRPITargets finds the expected RPI artifacts that exist on disk.
func locateRPITargets(cwd string) []gateTarget {
	locator, _ := ratchet.NewLocator(cwd)
	var targets []gateTarget
	for _, step := range ratchet.AllSteps() {
		pattern := ratchet.GetExpectedOutput(step)
		if strings.HasPrefix(pattern, "epic:") || strings.HasPrefix(pattern, "issue:") {
			continue
		}
		if path, _, err := locator.FindFirst(pattern); err == nil {
			targets = append(targets, gateTarget{step: step, path: path})
		}
	}
	return targets
}

// inferStepForPath picks the ratchet step whose expected-output pattern best
// matches a given path; falls back to the research step.
func inferStepForPath(cwd, path string) ratchet.Step {
	locator, _ := ratchet.NewLocator(cwd)
	for _, step := range ratchet.AllSteps() {
		pattern := ratchet.GetExpectedOutput(step)
		if strings.HasPrefix(pattern, "epic:") || strings.HasPrefix(pattern, "issue:") {
			continue
		}
		if found, _, err := locator.FindFirst(pattern); err == nil && found == path {
			return step
		}
	}
	return ratchet.StepResearch
}

// runGateValidation validates each target and aggregates to a single verdict.
func runGateValidation(validator *ratchet.Validator, targets []gateTarget) validateGateResult {
	opts := buildGateValidateOptions()
	// Non-nil slices so --json emits [] not null for a clean machine contract.
	issues := []string{}
	warnings := []string{}
	hasFail := false

	for _, t := range targets {
		res, err := validator.ValidateWithOptions(t.step, t.path, opts)
		if err != nil {
			hasFail = true
			issues = append(issues, fmt.Sprintf("%s: validate error: %v", t.path, err))
			continue
		}
		if !res.Valid {
			hasFail = true
		}
		for _, i := range res.Issues {
			issues = append(issues, t.path+": "+i)
		}
		for _, w := range res.Warnings {
			warnings = append(warnings, t.path+": "+w)
		}
	}

	v := aggregateVerdict(hasFail, len(warnings) > 0)
	return validateGateResult{
		Verdict:  v,
		Issues:   issues,
		Warnings: warnings,
		GateExit: gateExitForVerdict(v, validateStrict || validateWarnAsFail),
	}
}

// aggregateVerdict maps (hasFail, hasWarn) onto the PASS/WARN/FAIL vocabulary.
func aggregateVerdict(hasFail, hasWarn bool) verdict {
	switch {
	case hasFail:
		return verdictFail
	case hasWarn:
		return verdictWarn
	default:
		return verdictPass
	}
}

// gateExitForVerdict maps a verdict onto a gate exit code. In strict mode WARN
// is promoted to FAIL (exit 1).
func gateExitForVerdict(v verdict, strict bool) int {
	switch v {
	case verdictFail:
		return gateExitFail
	case verdictWarn:
		if strict {
			return gateExitFail
		}
		return gateExitPass
	default: // PASS
		return gateExitPass
	}
}

// buildGateValidateOptions builds ratchet options from the validate flags.
func buildGateValidateOptions() *ratchet.ValidateOptions {
	opts := &ratchet.ValidateOptions{Lenient: validateLenient}
	if validateLenient && validateLenientExpiry > 0 {
		t := time.Now().AddDate(0, 0, validateLenientExpiry)
		opts.LenientExpiryDate = &t
	}
	return opts
}

// emitGateResult writes output and, in --gate mode, returns a gateExitError so
// the exit code carries the verdict.
func emitGateResult(cmd *cobra.Command, result validateGateResult) error {
	wantJSON := validateJSONOut || GetOutput() == "json"
	if wantJSON {
		writeGateJSON(cmd.OutOrStdout(), result)
	} else if validateGate {
		writeGateTerse(cmd, result)
	} else {
		writeGateReport(cmd.OutOrStdout(), result)
	}

	if validateGate && result.GateExit != gateExitPass {
		return &gateExitError{code: result.GateExit, msg: ""}
	}
	return nil
}

// writeGateJSON emits the structured verdict.
func writeGateJSON(w io.Writer, result validateGateResult) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

// writeGateTerse emits one verdict line to stdout, detail to stderr (--gate text).
func writeGateTerse(cmd *cobra.Command, result validateGateResult) {
	fmt.Fprintln(cmd.OutOrStdout(), string(result.Verdict))
	writeGateDetail(cmd.ErrOrStderr(), result)
}

// writeGateReport emits the human verdict report (default, non-gate text mode).
func writeGateReport(w io.Writer, result validateGateResult) {
	fmt.Fprintf(w, "Verdict: %s\n", result.Verdict)
	writeGateDetail(w, result)
}

// writeGateDetail prints the issues/warnings blocks.
func writeGateDetail(w io.Writer, result validateGateResult) {
	formatStringList(w, "Issues", result.Issues)
	formatStringList(w, "Warnings", result.Warnings)
}
