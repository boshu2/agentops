package main

// `ao converge` — bounded fix -> re-run-judge-panel loop to terminal agreement or
// a 3-consecutive-fail BLOCK. It COMPOSES existing AgentOps Go: the non-mutating
// `ao codex dispatch` judge leg, the markdown verdict identity parser, the
// rpi_loop KILL-switch convention, and a vote-agreement convergence criterion.
//
// KERNEL INVARIANTS:
//   - The FIX step is applied by the ORCHESTRATING agent, never by the dispatched
//     judge. The judge leg is non-mutating (Dispatch.MutatesRepo == false).
//   - The independence axis is FRESH CONTEXT: convergence needs >=2 distinct
//     non-author judge contexts that PASS.
//   - LAW 0: no path ever shells a headless claude (the print/non-interactive
//     mode). The Claude->Codex leg uses the Go headless `ao codex dispatch`
//     (Codex Pro sub); the Codex->Claude leg is DELEGATED to a codex-approval /
//     NTM pane (no Go transport, never a headless claude call). UsesClaudePrint
//     is false in every transport branch.
//   - Fail-closed: a round with zero usable verdicts is never a vacuous PASS.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/liveness"
	"github.com/spf13/cobra"
)

// --- convergence criterion (vote-agreement) -------------------------------

// convergeContextVerdict is one judge's verdict in a converge round, keyed by the
// judge's distinct CONTEXT (the independence axis).
type convergeContextVerdict struct {
	ContextID   string
	ModelFamily string
	Pass        bool
}

// convergeRoundResult is the set of judge verdicts produced in one round.
type convergeRoundResult struct {
	ContextVerdicts []convergeContextVerdict
}

// convergeJudgeAgreement is the vote-agreement convergence criterion (distinct
// from the CI-streak ports.ConvergenceCriteria). The round converges iff the
// number of distinct non-empty PASS contexts is >= minContexts AND there are zero
// FAIL contexts. On convergence reasons is empty; otherwise reasons names the
// PASS/FAIL split.
func convergeJudgeAgreement(result convergeRoundResult, minContexts int) (bool, []string) {
	passCtx := map[string]struct{}{}
	failCtx := map[string]struct{}{}
	for _, v := range result.ContextVerdicts {
		if v.ContextID == "" {
			continue
		}
		if v.Pass {
			passCtx[v.ContextID] = struct{}{}
		} else {
			failCtx[v.ContextID] = struct{}{}
		}
	}
	// A context that both passed and failed (contradiction) is not a clean PASS.
	for id := range failCtx {
		delete(passCtx, id)
	}
	nPass := len(passCtx)
	nFail := len(failCtx)
	if nPass >= minContexts && nFail == 0 {
		return true, nil
	}
	reasons := []string{
		fmt.Sprintf("not converged: %d distinct PASS context(s) (need >=%d), %d FAIL context(s)", nPass, minContexts, nFail),
	}
	return false, reasons
}

// --- per-round disposition (fail-closed) ----------------------------------

// convergeRoundDisposition records whether a round counts as progress.
type convergeRoundDisposition struct {
	Pass                  bool
	IncrementsFailCounter bool
}

// convergeEvaluateRound maps a round to a disposition, failing closed: a round
// with zero usable verdicts (nil or empty ContextVerdicts) is NEVER a vacuous
// PASS — absence of evidence is treated as a failing round. The optional
// minContexts arg lets a caller (e.g. convergeRunBounded) evaluate against the
// SAME floor it uses; omitted/<=0 falls back to convergeDefaultMinContexts so the
// two paths cannot silently disagree on the floor.
func convergeEvaluateRound(result convergeRoundResult, minContexts ...int) convergeRoundDisposition {
	floor := convergeDefaultMinContexts
	if len(minContexts) > 0 && minContexts[0] > 0 {
		floor = minContexts[0]
	}
	usable := 0
	for _, v := range result.ContextVerdicts {
		if v.ContextID != "" {
			usable++
		}
	}
	if usable == 0 {
		return convergeRoundDisposition{Pass: false, IncrementsFailCounter: true}
	}
	// A round is "passing progress" only when it converges on the given floor.
	converged, _ := convergeJudgeAgreement(result, floor)
	if converged {
		return convergeRoundDisposition{Pass: true, IncrementsFailCounter: false}
	}
	return convergeRoundDisposition{Pass: false, IncrementsFailCounter: true}
}

// --- consecutive-fail BLOCK tracker ---------------------------------------

const (
	convergeStatusBlock            = "BLOCK"
	convergeStatusProgress         = "PROGRESS"
	convergeStatusConverged        = "CONVERGED"
	convergeStatusBoundedExhausted = "NOT-CONVERGED"
	convergeStatusKilled           = "KILLED"
	convergeConsecutiveFailLimit   = 3
	convergeDefaultMinContexts     = 2
	convergeDefaultMaxRounds       = 5
)

// convergeFailTracker counts CONSECUTIVE (not cumulative) failing rounds and
// blocks once the limit is reached.
type convergeFailTracker struct {
	consecutiveFails int
}

func newConvergeFailTracker() *convergeFailTracker {
	return &convergeFailTracker{}
}

// Observe records a round outcome: a converged round resets the consecutive
// counter; a failing round increments it.
func (t *convergeFailTracker) Observe(roundConverged bool) {
	if roundConverged {
		t.consecutiveFails = 0
		return
	}
	t.consecutiveFails++
}

// Blocked reports whether the consecutive-fail limit has been reached.
func (t *convergeFailTracker) Blocked() bool {
	return t.consecutiveFails >= convergeConsecutiveFailLimit
}

// Status returns the terminal BLOCK status when blocked, else a non-block status.
func (t *convergeFailTracker) Status() string {
	if t.Blocked() {
		return convergeStatusBlock
	}
	if t.consecutiveFails == 0 {
		return convergeStatusConverged
	}
	return convergeStatusProgress
}

// BlockReason explains the block.
func (t *convergeFailTracker) BlockReason() string {
	return fmt.Sprintf("BLOCK: %d consecutive failing rounds reached the limit", t.consecutiveFails)
}

// --- runtime / transport detection (LAW 0 guard) --------------------------

// convergeEnv carries the runtime-detection inputs (the session/thread ids that
// canonical_identity.go resolves from CLAUDE_SESSION_ID / CODEX_THREAD_ID).
type convergeEnv struct {
	ClaudeSessionID string
	CodexThreadID   string
}

// convergeTransport names how the judge panel leg is dispatched. UsesClaudePrint
// is hard-wired false in every branch (LAW 0).
type convergeTransport struct {
	Kind            string
	UsesClaudePrint bool
	Delegated       bool
}

// convergeResolveTransport selects the judge transport from the runtime context.
// From a Claude context the Claude->Codex leg uses the Go headless `ao codex
// dispatch`; from a Codex context the Codex->Claude leg has NO Go transport and
// is delegated to a codex-approval / NTM pane. UsesClaudePrint is false in every
// branch — there is never a headless claude (print-mode) call.
func convergeResolveTransport(env convergeEnv) convergeTransport {
	switch {
	case env.CodexThreadID != "" && env.ClaudeSessionID == "":
		// Codex context: the cross-leg judge (Claude) is reached via a pane, not Go.
		return convergeTransport{Kind: "pane-delegation", UsesClaudePrint: false, Delegated: true}
	default:
		// Claude context (or ambiguous/empty): dispatch a Codex judge via Go.
		return convergeTransport{Kind: "codex-dispatch", UsesClaudePrint: false, Delegated: false}
	}
}

// --- non-mutating judge packet + author_neq_validator ---------------------

// convergeBuildJudgePacket builds a non-mutating, judge-only dispatch packet
// carrying the author identity so author_neq_validator can be enforced. converge
// never applies a fix itself — the packet only judges.
func convergeBuildJudgePacket(authorID string) codexTaskPacket {
	return codexTaskPacket{
		Role:           "independent judge (non-mutating, verdict-only)",
		AuthorIdentity: authorID,
		Dispatch: codexTaskDispatchPolicy{
			Mode:        "non-mutating",
			MutatesRepo: false,
			Notes:       "judge-only: emit a verdict + evidence; never edit the repo",
		},
	}
}

// convergeVerifyReceipt enforces author_neq_validator by routing the no-self-grade
// decision through liveness.Disjoint (the single canonical author!=judge guard —
// which also fails closed on an empty author or judge id, the DRIFT #149 bypass).
// A receipt whose judge is not Disjoint from the author is unverified (self-judge).
func convergeVerifyReceipt(authorID string, receipt codexRunReceipt) error {
	if liveness.Disjoint(authorID, receipt.Verdict.JudgeName) != liveness.Allowed {
		return fmt.Errorf("converge: author_neq_validator violated: judge %q is not disjoint from author %q", receipt.Verdict.JudgeName, authorID)
	}
	return nil
}

// --- bounded loop (max-rounds + KILL switch) ------------------------------

// convergeLoopConfig bounds the loop.
type convergeLoopConfig struct {
	MaxRounds   int
	MinContexts int
	// KillDir is the directory under which the <KillDir>/.agents/rpi/KILL switch
	// is checked at each round boundary (reusing the rpi_loop convention).
	KillDir string
}

// convergeOutcome is the terminal result of a bounded run.
type convergeOutcome struct {
	Status    string
	RoundsRun int
}

// convergeKillSwitchPath returns the KILL switch path for a KillDir (the rpi_loop
// convention: <dir>/.agents/rpi/KILL).
func convergeKillSwitchPath(killDir string) string {
	return filepath.Join(killDir, ".agents", "rpi", "KILL")
}

// convergeRunBounded drives the supplied rounds to a terminal outcome. At each
// round boundary it checks the KILL switch first (-> KILLED), then evaluates the
// round: convergence stops CONVERGED, 3 consecutive failing rounds stop BLOCK,
// otherwise it continues. If MaxRounds elapse with neither terminal condition it
// stops NOT-CONVERGED after exactly MaxRounds.
func convergeRunBounded(cfg convergeLoopConfig, rounds []convergeRoundResult) convergeOutcome {
	minContexts := cfg.MinContexts
	if minContexts <= 0 {
		minContexts = convergeDefaultMinContexts
	}
	tracker := newConvergeFailTracker()
	run := 0
	for run < cfg.MaxRounds {
		// KILL check at the round boundary.
		if cfg.KillDir != "" {
			if _, err := os.Stat(convergeKillSwitchPath(cfg.KillDir)); err == nil {
				return convergeOutcome{Status: convergeStatusKilled, RoundsRun: run}
			}
		}
		var round convergeRoundResult
		if run < len(rounds) {
			round = rounds[run]
		}
		run++

		converged, _ := convergeJudgeAgreement(round, minContexts)
		tracker.Observe(converged)
		if converged {
			return convergeOutcome{Status: convergeStatusConverged, RoundsRun: run}
		}
		if tracker.Blocked() {
			return convergeOutcome{Status: convergeStatusBlock, RoundsRun: run}
		}
	}
	return convergeOutcome{Status: convergeStatusBoundedExhausted, RoundsRun: run}
}

// --- command wiring -------------------------------------------------------

var convergeCmd = &cobra.Command{
	Use:   "converge",
	Short: "Bounded fix -> re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK",
	Long: strings.TrimSpace(`
Run a bounded convergence loop: dispatch a non-mutating judge panel, apply the
orchestrating agent's fix between rounds, and re-run until the judges agree
(>=2 fresh non-author contexts PASS) or 3 consecutive rounds fail (BLOCK).

Composes the existing non-mutating ao codex dispatch judge leg and the rpi_loop
KILL switch. The Claude->Codex leg dispatches a Codex judge via Go; the
Codex->Claude leg is delegated to a codex-approval / NTM pane. Never a headless
claude (print-mode) call (LAW 0).
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		maxRounds, _ := cmd.Flags().GetInt("max-rounds")
		minContexts, _ := cmd.Flags().GetInt("min-contexts")
		requireCrossFamily, _ := cmd.Flags().GetBool("require-cross-family")

		// Mandatory ENTRY canary: prove the gate can FAIL on a planted positive
		// (and accepts a known-good) before trusting any PASS. An empty/PASS result
		// is a lie until proven to bite. A failed canary aborts before any dispatch.
		canary := convergeRunCanary(convergeProductionCanaryGate)
		if !canary.Proceed {
			return fmt.Errorf("%s", canary.Message)
		}

		// Resolve the author identity + transport from the runtime context. This
		// exercises the LAW-0 transport selector (UsesClaudePrint is false in every
		// branch) and builds the non-mutating, author_neq_validator judge packet —
		// the kernel the loop drives, surfaced here so the command is not vacuous.
		authorID := resolveSessionID("")
		transport := convergeResolveTransport(convergeEnv{
			ClaudeSessionID: os.Getenv("CLAUDE_SESSION_ID"),
			CodexThreadID:   os.Getenv("CODEX_THREAD_ID"),
		})
		if transport.UsesClaudePrint { // structural LAW-0 assertion, can never be true
			return fmt.Errorf("converge: refusing transport that uses a headless claude print call (LAW 0)")
		}
		packet := convergeBuildJudgePacket(authorID)
		if packet.Dispatch.MutatesRepo {
			return fmt.Errorf("converge: judge packet must be non-mutating")
		}
		cfg := convergeLoopConfig{MaxRounds: maxRounds, MinContexts: minContexts, KillDir: cmd.Flags().Lookup("kill-dir").Value.String()}

		// The loop is agent-driven: converge dispatches a non-mutating judge each
		// round and the ORCHESTRATING agent applies the fix between rounds — converge
		// never applies a fix itself, and AgentOps never writes a binding verdict
		// (MTO is the sole writer). The command surfaces the resolved plan/CLAIM.
		fmt.Fprintf(cmd.OutOrStdout(),
			"ao converge: %s\n  author=%q transport=%s (delegated=%v) judge-role=%q\n  bounded loop: max-rounds=%d min-contexts=%d require-cross-family=%v kill-dir=%q\n  the fix step is the orchestrating agent's; the judge leg is non-mutating; MTO writes the binding verdict.\n",
			canary.Message, authorID, transport.Kind, transport.Delegated, packet.Role,
			cfg.MaxRounds, cfg.MinContexts, requireCrossFamily, cfg.KillDir)
		return nil
	},
}

func init() {
	convergeCmd.Flags().Int("max-rounds", convergeDefaultMaxRounds, "maximum fix->re-run rounds before NOT-CONVERGED")
	convergeCmd.Flags().Int("min-contexts", convergeDefaultMinContexts, "distinct non-author judge contexts required to converge")
	convergeCmd.Flags().Bool("require-cross-family", false, "optional strengthener: additionally require >=2 model families in the PASS quorum")
	convergeCmd.Flags().String("kill-dir", ".", "directory whose .agents/rpi/KILL switch aborts the bounded loop at a round boundary")
	rootCmd.AddCommand(convergeCmd)
}
