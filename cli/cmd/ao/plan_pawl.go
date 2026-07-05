// practices: [design-by-contract, continuous-delivery]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/planpawl"
)

// Exit codes for `ao plan-pawl decide` — the exit code IS the decision, so the
// skill (dual-pane-atm) can branch on it without parsing.
const (
	planPawlExitPass     = 0 // PASS — the door opens
	planPawlExitUsage    = 2 // bad invocation
	planPawlExitRedo     = 3 // REDO — auto-redo loop (no human)
	planPawlExitBlocked  = 4 // BLOCKED — a circuit breaker tripped (andon)
	planPawlExitDegraded = 5 // DEGRADED — transient lane loss below quorum; re-run the PANEL
)

// planPawlExitError carries a decision exit code up to Execute().
type planPawlExitError struct {
	code int
	msg  string
}

func (e *planPawlExitError) Error() string { return e.msg }
func (e *planPawlExitError) ExitCode() int { return e.code }

var (
	planPawlVerdicts     []string
	planPawlRound        int
	planPawlMaxRounds    int
	planPawlDir          string
	planPawlOscillation  bool
	planPawlJudgmentFlag bool
	planPawlJSON         bool
)

var planPawlCmd = &cobra.Command{
	Use:   "plan-pawl",
	Short: "The plan-pawl duel gate over discovery plans (docs/contracts/pawls.md)",
	Long: `The plan-pawl is the multi-model pawl applied to a discovery PLAN artifact
instead of a code diff. The duel runs >= 2 model-family judge panes over the plan;
the 'decide' subcommand is the DETERMINISTIC core that turns their verdicts into one
of three decisions — PASS / REDO / BLOCKED — with the circuit-breaker governance
inherited verbatim from pawls.md. Pane spawning and the re-judge loop are the skill's
job (dual-pane-atm); this decider is the windshield: deterministic, no model.`,
}

var planPawlDecideCmd = &cobra.Command{
	Use:           "decide",
	Short:         "Decide a duel round: PASS / REDO / BLOCKED (exit 0 / 3 / 4)",
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `Apply the deterministic quorum/round/breaker rules to one round of judge
verdicts. Quorum = no FAIL AND >= 2 distinct roster families ran. A FAIL auto-redoes;
a mechanical WARN auto-applies + re-judges; a judgment WARN is surfaced but does not
block. Breakers (round > max-rounds, an explicit judgment flag, or oscillation) trip
to BLOCKED. The exit code is the decision: 0 PASS, 3 REDO, 4 BLOCKED.

Verdicts (repeatable):
  --verdict <family>:<PASS|FAIL|WARN>[:<mechanical|judgment>]
Or read one JSON JudgeVerdict per *.json file from a directory:
  --dir <verdicts-dir>

Examples:
  ao plan-pawl decide --verdict claude:PASS --verdict gpt:PASS
  ao plan-pawl decide --verdict claude:PASS --verdict gpt:FAIL --round 2
  ao plan-pawl decide --dir .agents/duel/round-3 --round 4 --max-rounds 3 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verdicts, err := collectVerdicts()
		if err != nil {
			return &planPawlExitError{code: planPawlExitUsage, msg: err.Error()}
		}
		if len(verdicts) == 0 {
			return &planPawlExitError{code: planPawlExitUsage, msg: "no verdicts: pass --verdict or --dir"}
		}

		out := planpawl.Decide(planpawl.Input{
			Verdicts:    verdicts,
			Round:       planPawlRound,
			MaxRounds:   planPawlMaxRounds,
			Oscillation: planPawlOscillation,
		})

		if planPawlJSON {
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling plan-pawl decision: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "decision: %s (round %d/%d, families=%s)\n",
				out.Decision, out.Round, out.MaxRounds, strings.Join(out.Families, "+"))
			if out.Reason != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "reason: %s\n", out.Reason)
			}
			if out.BreakerTripped != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "breaker: %s\n", out.BreakerTripped)
			}
			if len(out.AutoApplied) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "auto-applied (mechanical): %s\n", strings.Join(out.AutoApplied, ", "))
			}
			if len(out.SurfacedWarns) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "surfaced (judgment): %s\n", strings.Join(out.SurfacedWarns, ", "))
			}
			if len(out.DegradedFamilies) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "degraded (transient lane loss): %s\n", strings.Join(out.DegradedFamilies, ", "))
			}
		}

		switch out.Decision {
		case planpawl.DecisionPass:
			return nil
		case planpawl.DecisionRedo:
			return &planPawlExitError{code: planPawlExitRedo, msg: ""}
		case planpawl.DecisionBlocked:
			return &planPawlExitError{code: planPawlExitBlocked, msg: ""}
		case planpawl.DecisionDegraded:
			// Retryable: re-run the panel, not the work. Its own exit code so the
			// caller never conflates an infra outage with a genuine REDO.
			return &planPawlExitError{code: planPawlExitDegraded, msg: ""}
		default:
			return &planPawlExitError{code: planPawlExitUsage, msg: "unknown decision " + string(out.Decision)}
		}
	},
}

// collectVerdicts merges --verdict tokens and --dir JSON files into one slice.
func collectVerdicts() ([]planpawl.JudgeVerdict, error) {
	var vs []planpawl.JudgeVerdict
	for _, tok := range planPawlVerdicts {
		v, err := parseVerdictToken(tok)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	if planPawlDir != "" {
		dirVs, err := readVerdictDir(planPawlDir)
		if err != nil {
			return nil, err
		}
		vs = append(vs, dirVs...)
	}
	// The --judgment-flag operator override is global: it raises the hard breaker
	// regardless of how the verdicts were collected (--verdict OR --dir). Applying
	// it here — after BOTH sources are merged — closes a fail-open where
	// `--dir --judgment-flag` would silently ignore the breaker. (A judge can also
	// set judgment_flag per-pane inside a --dir JSON file; that path is unaffected.)
	if planPawlJudgmentFlag {
		for i := range vs {
			vs[i].JudgmentFlag = true
		}
	}
	return vs, nil
}

// parseVerdictToken parses "<family>:<disposition>[:<warnclass>]".
func parseVerdictToken(tok string) (planpawl.JudgeVerdict, error) {
	parts := strings.Split(tok, ":")
	if len(parts) < 2 {
		return planpawl.JudgeVerdict{}, fmt.Errorf("bad --verdict %q: need family:disposition[:warnclass]", tok)
	}
	v := planpawl.JudgeVerdict{Family: parts[0]}
	switch strings.ToUpper(parts[1]) {
	case "PASS":
		v.Disposition = planpawl.PASS
	case "FAIL":
		v.Disposition = planpawl.FAIL
	case "WARN":
		v.Disposition = planpawl.WARN
	default:
		return planpawl.JudgeVerdict{}, fmt.Errorf("bad disposition %q in %q (PASS|FAIL|WARN)", parts[1], tok)
	}
	if len(parts) >= 3 && parts[2] != "" {
		switch strings.ToLower(parts[2]) {
		case "mechanical":
			v.WarnClass = planpawl.Mechanical
		case "judgment":
			v.WarnClass = planpawl.Judgment
		default:
			return planpawl.JudgeVerdict{}, fmt.Errorf("bad warnclass %q in %q (mechanical|judgment)", parts[2], tok)
		}
	}
	return v, nil
}

// readVerdictDir reads one JudgeVerdict per *.json file (sorted for determinism).
func readVerdictDir(dir string) ([]planpawl.JudgeVerdict, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read verdict dir %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var vs []planpawl.JudgeVerdict
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", n, err)
		}
		var v planpawl.JudgeVerdict
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("parse %q: %w", n, err)
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func init() {
	planPawlDecideCmd.Flags().StringArrayVar(&planPawlVerdicts, "verdict", nil, "judge verdict: family:disposition[:warnclass] (repeatable)")
	planPawlDecideCmd.Flags().StringVar(&planPawlDir, "dir", "", "directory of judge verdict *.json files")
	planPawlDecideCmd.Flags().IntVar(&planPawlRound, "round", 1, "current round (1-based)")
	planPawlDecideCmd.Flags().IntVar(&planPawlMaxRounds, "max-rounds", 3, "max rounds before the max-attempts breaker trips (<=0 = unbounded)")
	planPawlDecideCmd.Flags().BoolVar(&planPawlOscillation, "oscillation", false, "the same failure has repeated (hard breaker)")
	planPawlDecideCmd.Flags().BoolVar(&planPawlJudgmentFlag, "judgment-flag", false, "a reviewer raised an explicit value/irreversibility judgment (hard breaker)")
	planPawlDecideCmd.Flags().BoolVar(&planPawlJSON, "json", false, "emit the decision as JSON")
	planPawlCmd.AddCommand(planPawlDecideCmd)
	rootCmd.AddCommand(planPawlCmd)
}
