// practices: [dora-metrics, lean-startup]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/resteer"
	"github.com/boshu2/agentops/cli/internal/verdictledger"
	"github.com/spf13/cobra"
)

// steerAutoYes / steerAutoAuto pre-confirm an apply for non-interactive or
// scripted use. Without either, `ao goals steer apply` reads an interactive
// y/N prompt before mutating GOALS.md (ADR-0006 I-2: no GOALS.md change
// without explicit human consent). The two flags are equivalent: --auto is
// the documented re-steer name, --yes the conventional scripted-consent name.
var (
	steerAutoYes  bool
	steerAutoAuto bool
)

// steerPreConfirmed reports whether the operator gave explicit non-interactive
// consent via either --yes or --auto.
func steerPreConfirmed() bool { return steerAutoYes || steerAutoAuto }

// steerAutoPolicyPath overrides the re-steer policy path; empty means the
// ADR-0006 default (docs/re-steer-policy.json, falling back to safe defaults).
var steerAutoPolicyPath string

// goalsSteerRecommendCmd is the recommendation-only re-steer surface. It runs
// the F5.2 engine and prints recommendations + skip reasons. It NEVER touches
// GOALS.md (ADR-0006 I-2: default is recommendation-only).
var goalsSteerRecommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Show re-steer recommendations without modifying GOALS.md",
	Long: "Run the re-steer policy engine over the verdict ledger and print " +
		"recommended directive mutations and skip reasons. GOALS.md is never " +
		"modified. Use `ao goals steer apply` to apply a recommendation.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSteerRecommend(cmd.OutOrStdout(), goalsJSONOutput())
	},
}

// goalsSteerApplyCmd applies a re-steer recommendation to GOALS.md, but only
// after an explicit human-on-loop confirmation: an interactive y/N prompt, or
// --auto --yes for scripted explicit consent. It additionally requires the
// policy's auto_apply to be true.
var goalsSteerApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a re-steer recommendation to GOALS.md (human-gated)",
	Long: "Apply the top re-steer recommendation to GOALS.md via the non-lossy " +
		"directive-block patcher. Requires policy auto_apply:true AND explicit " +
		"human confirmation (interactive prompt, or --auto --yes for scripts). " +
		"A run without confirmation never changes GOALS.md.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSteerApply(cmd.InOrStdin(), cmd.OutOrStdout(), goalsJSONOutput())
	},
}

// steerEvalContext bundles the resolved inputs a re-steer run needs.
type steerEvalContext struct {
	goalsPath   string
	projectRoot string
	policy      resteer.Policy
	ledger      *verdictledger.Ledger
	directives  []goals.ParsedDirective
	result      resteer.Result
}

// loadSteerContext resolves GOALS.md, the verdict ledger, and the re-steer
// policy, then runs the F5.2 engine. Errors name the corrective command.
func loadSteerContext() (*steerEvalContext, error) {
	patcher, goalsPath, err := goals.LoadGoalsPatcher(resolveGoalsFile())
	if err != nil {
		return nil, fmt.Errorf("loading goals file (run `ao goals init` to create one): %w", err)
	}
	root := steerProjectRoot()
	policy, err := resteer.LoadPolicy(steerPolicyPath(root))
	if err != nil {
		return nil, fmt.Errorf("%w (fix or remove %s)", err, steerPolicyPath(root))
	}
	ledger, err := verdictledger.Load(root)
	if err != nil {
		return nil, fmt.Errorf("loading verdict ledger (run `ao goals measure` to populate it): %w", err)
	}
	directives := patcher.Directives()
	ids := make([]string, 0, len(directives))
	for _, d := range directives {
		if d.StableID != "" {
			ids = append(ids, d.StableID)
		}
	}
	return &steerEvalContext{
		goalsPath:   goalsPath,
		projectRoot: root,
		policy:      policy,
		ledger:      ledger,
		directives:  directives,
		result:      resteer.Evaluate(ledger, policy, ids),
	}, nil
}

// runSteerRecommend prints recommendations and skip reasons; never mutates.
func runSteerRecommend(w io.Writer, asJSON bool) error {
	ctx, err := loadSteerContext()
	if err != nil {
		return err
	}
	if asJSON {
		return emitSteerJSON(w, ctx, nil)
	}
	printSteerRecommendations(w, ctx)
	fmt.Fprintln(w, "\nGOALS.md not modified (recommendation-only). Run `ao goals steer apply` to apply.")
	return nil
}

// runSteerApply applies the top recommendation after the human-gated checks.
func runSteerApply(in io.Reader, w io.Writer, asJSON bool) error {
	ctx, err := loadSteerContext()
	if err != nil {
		return err
	}
	if len(ctx.result.Recommendations) == 0 {
		if asJSON {
			return emitSteerJSON(w, ctx, nil)
		}
		printSteerRecommendations(w, ctx)
		return nil
	}
	rec := ctx.result.Recommendations[0]
	if err := guardAutoApply(ctx.policy); err != nil {
		return err
	}
	if err := confirmApply(in, w, rec, asJSON); err != nil {
		return err
	}
	return applySteerRecommendation(w, ctx, rec, asJSON)
}

// guardAutoApply enforces ADR-0006 I-2: GOALS.md is not modified unless the
// policy explicitly sets auto_apply:true.
func guardAutoApply(policy resteer.Policy) error {
	if policy.AutoApply {
		return nil
	}
	return fmt.Errorf(
		"re-steer apply blocked: policy auto_apply is false (recommendation-only). "+
			"Set \"auto_apply\": true in %s to permit applied mutations",
		steerPolicyPath(steerProjectRoot()))
}

// confirmApply enforces the human-on-loop gate. With --yes the explicit
// scripted consent stands in for the prompt; otherwise an interactive y/N
// prompt must be answered affirmatively. A declined or empty answer aborts
// without touching GOALS.md.
func confirmApply(in io.Reader, w io.Writer, rec resteer.Recommendation, asJSON bool) error {
	if steerPreConfirmed() {
		return nil
	}
	if asJSON {
		return fmt.Errorf(
			"re-steer apply needs confirmation: re-run with --yes for non-interactive consent " +
				"(JSON output mode cannot prompt interactively)")
	}
	fmt.Fprintf(w, "Apply %s to directive %q (failure streak %d)? [y/N]: ",
		rec.MutationType, rec.DirectiveID, rec.FailureStreak)
	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("re-steer apply aborted: confirmation declined; GOALS.md not modified")
	}
	return nil
}

// applySteerRecommendation mutates GOALS.md via the non-lossy resteer/apply
// helper, then records a cooldown so the gate engages next iteration.
func applySteerRecommendation(w io.Writer, ctx *steerEvalContext, rec resteer.Recommendation, asJSON bool) error {
	data, err := os.ReadFile(ctx.goalsPath)
	if err != nil {
		return fmt.Errorf("reading %s for re-steer apply: %w", ctx.goalsPath, err)
	}
	patched, outcome, err := resteer.Apply(data, ctx.policy, rec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(ctx.goalsPath, patched, 0o644); err != nil {
		return fmt.Errorf("writing patched %s: %w", ctx.goalsPath, err)
	}
	if err := recordSteerCooldown(ctx.projectRoot, rec, verdictledger.CooldownApplied,
		"applied "+outcome.Detail); err != nil {
		return err
	}
	if asJSON {
		return emitSteerJSON(w, ctx, &outcome)
	}
	fmt.Fprintf(w, "Applied: %s\n", outcome.Detail)
	fmt.Fprintf(w, "Cooldown recorded for %q; it is skipped for the next %d iterations.\n",
		rec.DirectiveID, ctx.policy.CooldownIterations)
	return nil
}

// recordSteerCooldown appends a cooldown record so the F5.2 cooldown gate
// engages on the next iteration (ADR-0006 §COOLDOWN: a record is written on
// proposal AND on application).
func recordSteerCooldown(root string, rec resteer.Recommendation, kind, note string) error {
	writer := verdictledger.Writer{}
	if _, err := writer.AppendCooldown(root, verdictledger.CooldownInput{
		DirectiveID:  rec.DirectiveID,
		RunTime:      time.Now(),
		CooldownKind: kind,
		MutationType: rec.MutationType,
		Note:         note,
	}); err != nil {
		return fmt.Errorf("recording re-steer cooldown: %w", err)
	}
	return nil
}

// printSteerRecommendations renders the recommendations and skip reasons as a
// human-readable table-ish listing.
func printSteerRecommendations(w io.Writer, ctx *steerEvalContext) {
	if len(ctx.result.Recommendations) == 0 {
		fmt.Fprintln(w, "No re-steer recommendations: every directive is healthy or below the evidence/cooldown gate.")
	} else {
		fmt.Fprintf(w, "Re-steer recommendations (%d):\n", len(ctx.result.Recommendations))
		for _, r := range ctx.result.Recommendations {
			fmt.Fprintf(w, "  %s  %s  streak=%d iterations=%d\n      %s\n",
				r.DirectiveID, r.MutationType, r.FailureStreak, r.IterationCount, r.Rationale)
		}
	}
	if len(ctx.result.Skipped) > 0 {
		fmt.Fprintf(w, "Skipped (%d):\n", len(ctx.result.Skipped))
		for _, s := range ctx.result.Skipped {
			fmt.Fprintf(w, "  %s  %s  streak=%d\n", s.DirectiveID, s.Reason, s.FailureStreak)
		}
	}
}

// steerJSONOut is the -o json shape for `ao goals steer recommend|apply`.
type steerJSONOut struct {
	AutoApply       bool                     `json:"auto_apply"`
	Recommendations []resteer.Recommendation `json:"recommendations"`
	Skipped         []resteer.Skip           `json:"skipped"`
	Applied         *resteer.ApplyOutcome    `json:"applied,omitempty"`
}

// emitSteerJSON writes the recommendation result (and any applied outcome) as
// JSON, honoring the global -o json convention.
func emitSteerJSON(w io.Writer, ctx *steerEvalContext, applied *resteer.ApplyOutcome) error {
	out := steerJSONOut{
		AutoApply:       ctx.policy.AutoApply,
		Recommendations: ctx.result.Recommendations,
		Skipped:         ctx.result.Skipped,
		Applied:         applied,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encoding re-steer JSON: %w", err)
	}
	return nil
}

// steerProjectRoot returns the project root the verdict ledger and policy
// resolve against. The current working directory is the project root.
func steerProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// steerPolicyPath resolves the re-steer policy path: the --policy override if
// set, else the ADR-0006 default under the project root.
func steerPolicyPath(root string) string {
	if steerAutoPolicyPath != "" {
		return steerAutoPolicyPath
	}
	return root + string(os.PathSeparator) + resteer.DefaultPolicyRelPath
}

func init() {
	goalsSteerApplyCmd.Flags().BoolVar(&steerAutoYes, "yes", false,
		"Pre-confirm the apply for non-interactive/scripted use (explicit consent)")
	goalsSteerApplyCmd.Flags().BoolVar(&steerAutoAuto, "auto", false,
		"Equivalent to --yes: explicit non-interactive consent to apply")
	goalsSteerApplyCmd.Flags().StringVar(&steerAutoPolicyPath, "policy", "",
		"Re-steer policy path (default: docs/re-steer-policy.json)")
	goalsSteerRecommendCmd.Flags().StringVar(&steerAutoPolicyPath, "policy", "",
		"Re-steer policy path (default: docs/re-steer-policy.json)")

	goalsSteerCmd.AddCommand(goalsSteerRecommendCmd)
	goalsSteerCmd.AddCommand(goalsSteerApplyCmd)
}
