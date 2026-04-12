package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/types"
)

// maxGateRetryDepth is the hard ceiling for gate retry attempts.
const maxGateRetryDepth = rpi.MaxGateRetryDepth

// shouldForceEscalation returns true when the attempt count exceeds the hard ceiling.
func shouldForceEscalation(attempt int) bool { return rpi.ShouldForceEscalation(attempt) }

// gateFailError signals a gate check failure that may be retried.
type gateFailError struct {
	Phase    int
	Verdict  string
	Findings []finding
	Report   string
}

func (e *gateFailError) Error() string {
	return fmt.Sprintf("gate FAIL at phase %d: %s (report: %s)", e.Phase, e.Verdict, e.Report)
}

// postPhaseProcessing handles phase-specific post-processing.
func postPhaseProcessing(cwd string, state *phasedState, phaseNum int, logPath string) error {
	switch phaseNum {
	case 1:
		return processDiscoveryPhase(cwd, state, logPath)
	case 2:
		return processImplementationPhase(cwd, state, phaseNum, logPath)
	case 3:
		return processValidationPhase(cwd, state, phaseNum, logPath)
	}
	return nil
}

// processDiscoveryPhase handles post-processing for the discovery phase.
// It extracts the epic ID, detects fast path, and checks the pre-mortem verdict.
func processDiscoveryPhase(cwd string, state *phasedState, logPath string) error {
	tracker := detectTrackerHealth(state.Opts.BDCommand, state.Opts.LookPath)
	state.TrackerMode = tracker.Mode
	state.TrackerReason = tracker.Reason

	epicID, err := resolveDiscoveryEpicID(cwd, state, tracker, logPath)
	if err != nil {
		return err
	}
	state.EpicID = epicID
	recordDiscoveryEpicID(state, logPath)
	if err := writeExecutionPacketSeed(cwd, state); err != nil {
		return fmt.Errorf("refresh execution packet after discovery: %w", err)
	}

	maybeEnableDiscoveryFastPath(state)

	report, err := findLatestCouncilReport(cwd, "pre-mortem", time.Time{}, discoveryCouncilEpicID(state))
	if err != nil {
		return lockDiscoveryGateForMissingReport(state, logPath, err)
	}
	verdict, err := extractCouncilVerdict(report)
	if err != nil {
		return lockDiscoveryGateForParseError(state, logPath, report, err)
	}
	return recordDiscoveryVerdict(cwd, state, logPath, report, verdict)
}

func resolveDiscoveryEpicID(cwd string, state *phasedState, tracker trackerHealth, logPath string) (string, error) {
	if tracker.Healthy {
		if epicID, err := extractEpicID(state.Opts.BDCommand); err == nil {
			return epicID, nil
		} else {
			return resolveDiscoveryFallback(cwd, state, tracker, logPath, err)
		}
	}
	return resolveDiscoveryFallback(cwd, state, tracker, logPath, nil)
}

func resolveDiscoveryFallback(cwd string, state *phasedState, tracker trackerHealth, logPath string, epicErr error) (string, error) {
	if !tracker.Healthy {
		fmt.Printf("Tracker degraded: %s (%s)\n", tracker.Reason, tracker.Error)
		logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("tracker degraded: mode=%s reason=%s error=%s", tracker.Mode, tracker.Reason, tracker.Error))
	}
	planPath, planErr := discoverPlanFile(cwd)
	if planErr == nil {
		fmt.Printf("Tasklist fallback: using execution packet + plan %s\n", planPath)
		return "", nil
	}
	if tracker.Healthy {
		issueID, issueErr := extractAnyOpenIssueID(state.Opts.BDCommand)
		if issueErr != nil {
			return "", fmt.Errorf("discovery phase: could not find epic, plan file, or open issue: %w", epicErr)
		}
		fmt.Printf("Single-issue fallback: using %s (not an epic)\n", issueID)
		return issueID, nil
	}
	return "", fmt.Errorf("discovery phase: tracker degraded and no plan file found: %w", planErr)
}

func recordDiscoveryEpicID(state *phasedState, logPath string) {
	if state.EpicID != "" {
		fmt.Printf("Epic ID: %s\n", state.EpicID)
		logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("extracted epic: %s", state.EpicID))
		return
	}
	fmt.Println("Epic ID: none (tasklist fallback)")
	logPhaseTransition(logPath, state.RunID, "discovery", "extracted objective: tasklist fallback via execution packet")
}

func maybeEnableDiscoveryFastPath(state *phasedState) {
	if state.Opts.FastPath || state.EpicID == "" || isPlanFileEpic(state.EpicID) {
		return
	}
	fast, err := detectFastPath(state.EpicID, state.Opts.BDCommand)
	if err != nil {
		VerbosePrintf("Warning: fast-path detection failed (continuing without): %v\n", err)
		return
	}
	if fast {
		state.FastPath = true
		fmt.Println("Micro-epic detected — using fast path (--quick for gates)")
	}
}

func discoveryCouncilEpicID(state *phasedState) string {
	if isPlanFileEpic(state.EpicID) {
		return ""
	}
	return state.EpicID
}

func lockDiscoveryGateForMissingReport(state *phasedState, logPath string, err error) error {
	fmt.Printf("Pre-mortem verdict: LOCKED (report unavailable: %v)\n", err)
	logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("pre-mortem verdict: LOCKED (report unavailable: %v)", err))
	state.Verdicts["pre_mortem"] = "LOCKED"
	return &gateFailError{Phase: 1, Verdict: "LOCKED", Findings: []finding{{
		Description: "Pre-mortem council report not found — gate is locked",
		Fix:         "Run the pre-mortem council before proceeding past discovery.",
	}}, Report: "unavailable"}
}

func lockDiscoveryGateForParseError(state *phasedState, logPath, report string, err error) error {
	fmt.Printf("Pre-mortem verdict: LOCKED (parse error: %v)\n", err)
	logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("pre-mortem verdict: LOCKED (parse error: %v)", err))
	state.Verdicts["pre_mortem"] = "LOCKED"
	return &gateFailError{Phase: 1, Verdict: "LOCKED", Findings: []finding{{
		Description: "Could not extract verdict from pre-mortem report",
		Fix:         "Re-run the pre-mortem council to produce a valid report.",
		Ref:         pathClean(report),
	}}, Report: report}
}

func recordDiscoveryVerdict(cwd string, state *phasedState, logPath, report, verdict string) error {
	findings, _ := extractCouncilFindings(report, 5)
	state.Verdicts["pre_mortem"] = verdict
	fmt.Printf("Pre-mortem verdict: %s\n", verdict)
	_, _ = appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   1,
		Type:    "gate.discovery.verdict",
		Message: fmt.Sprintf("Pre-mortem verdict: %s", verdict),
		Details: map[string]any{"verdict": verdict, "report": report},
	})
	logPhaseTransition(logPath, state.RunID, "discovery", fmt.Sprintf("pre-mortem verdict: %s report=%s", verdict, report))
	if artifact, evalErr := emitPhaseEvaluatorArtifact(cwd, state, 1, verdict, findings, pathClean(report)); evalErr != nil {
		VerbosePrintf("Warning: could not write discovery evaluator artifact: %v\n", evalErr)
	} else if artifact != nil {
		state.Verdicts["discovery_evaluator"] = artifact.Verdict
	}

	if verdict == "FAIL" {
		return &gateFailError{Phase: 1, Verdict: verdict, Findings: findings, Report: report}
	}
	return nil
}

// processImplementationPhase handles post-processing for the implementation phase.
// It validates the prior phase result and checks crank completion status.
func processImplementationPhase(cwd string, state *phasedState, phaseNum int, logPath string) error {
	if state.StartPhase <= 1 {
		if err := validatePriorPhaseResult(cwd, 1); err != nil {
			return fmt.Errorf("phase %d prerequisite not met: %w", phaseNum, err)
		}
	}
	if state.EpicID == "" {
		gateVerdict := "PASS"
		if state.TrackerMode == "tasklist" {
			gateVerdict = "WARN"
		}
		recordImplementationEvaluator(cwd, state, gateVerdict, nil)
		return nil
	}
	// Plan-file mode: skip bd-dependent completion check
	if isPlanFileEpic(state.EpicID) {
		recordImplementationEvaluator(cwd, state, "PASS", nil)
		return nil
	}
	if isEpic, err := isEpicIssue(state.EpicID, state.Opts.BDCommand); err == nil && !isEpic {
		fmt.Printf("Crank status: SKIP (non-epic issue %s)\n", state.EpicID)
		logPhaseTransition(logPath, state.RunID, "implementation", fmt.Sprintf("crank status: SKIP (non-epic issue %s)", state.EpicID))
		recordImplementationEvaluator(cwd, state, "SKIP", nil)
		return nil
	} else if err != nil {
		VerbosePrintf("Warning: could not determine issue type for %s (continuing with crank completion check): %v\n", state.EpicID, err)
	}
	status, err := checkCrankCompletion(state.EpicID, state.Opts.BDCommand)
	if err != nil {
		VerbosePrintf("Warning: could not check crank completion (continuing to validation): %v\n", err)
		findings := []finding{{
			Description: "Unable to mechanically confirm crank completion",
			Fix:         "Inspect the implementation artifacts and tracker state before treating implementation as finished.",
			Ref:         latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "execution-packet.json")),
		}}
		recordImplementationEvaluator(cwd, state, "WARN", findings)
		return nil
	}
	fmt.Printf("Crank status: %s\n", status)
	_, _ = appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   2,
		Type:    "gate.implementation.verdict",
		Message: fmt.Sprintf("Crank status: %s", status),
		Details: map[string]any{"status": status, "epic_id": state.EpicID},
	})
	logPhaseTransition(logPath, state.RunID, "implementation", fmt.Sprintf("crank status: %s", status))
	findings := []finding{}
	if implementationStatusBlocksValidation(status) {
		findings = append(findings, finding{
			Description: fmt.Sprintf("Crank reported %s for %s", status, state.EpicID),
			Fix:         "Resolve the remaining blocked or incomplete work before validation continues.",
			Ref:         latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "execution-packet.json")),
		})
	}
	findings = recordImplementationEvaluator(cwd, state, status, findings)
	if implementationStatusBlocksValidation(status) {
		return &gateFailError{Phase: 2, Verdict: status, Findings: findings, Report: "bd children " + state.EpicID}
	}
	return nil
}

func recordImplementationEvaluator(cwd string, state *phasedState, verdict string, findings []finding) []finding {
	artifact, err := emitPhaseEvaluatorArtifact(cwd, state, 2, verdict, findings)
	if err != nil {
		VerbosePrintf("Warning: could not write implementation evaluator artifact: %v\n", err)
		return findings
	}
	if artifact == nil {
		return findings
	}
	state.Verdicts["implementation_evaluator"] = artifact.Verdict
	if implementationStatusBlocksValidation(verdict) {
		return artifact.Findings
	}
	return findings
}

func implementationStatusBlocksValidation(status string) bool {
	return status == "BLOCKED" || status == "PARTIAL"
}

// processValidationPhase handles post-processing for the validation phase.
// It validates the prior phase result, checks the vibe verdict, and optionally extracts
// the post-mortem verdict.
func processValidationPhase(cwd string, state *phasedState, phaseNum int, logPath string) error {
	if state.StartPhase <= 2 {
		if err := validatePriorPhaseResult(cwd, 2); err != nil {
			return fmt.Errorf("phase %d prerequisite not met: %w", phaseNum, err)
		}
	}
	report, err := findLatestCouncilReport(cwd, "vibe", time.Time{}, state.EpicID)
	if err != nil {
		return fmt.Errorf("validation phase: vibe report not found (phase may not have completed): %w", err)
	}
	verdict, err := extractCouncilVerdict(report)
	if err != nil {
		return fmt.Errorf("validation phase: could not extract vibe verdict from %s: %w", report, err)
	}
	findings, _ := extractCouncilFindings(report, 5)
	state.Verdicts["vibe"] = verdict
	fmt.Printf("Vibe verdict: %s\n", verdict)
	_, _ = appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   3,
		Type:    "gate.validation.verdict",
		Message: fmt.Sprintf("Vibe verdict: %s", verdict),
		Details: map[string]any{"verdict": verdict, "report": report},
	})
	logPhaseTransition(logPath, state.RunID, "validation", fmt.Sprintf("vibe verdict: %s report=%s", verdict, report))

	// Also extract post-mortem verdict if available (non-blocking)
	evidence := []string{pathClean(report)}
	pmReport, err := findLatestCouncilReport(cwd, "post-mortem", time.Time{}, state.EpicID)
	if err == nil {
		pmVerdict, err := extractCouncilVerdict(pmReport)
		if err == nil {
			state.Verdicts["post_mortem"] = pmVerdict
			fmt.Printf("Post-mortem verdict: %s\n", pmVerdict)
			logPhaseTransition(logPath, state.RunID, "validation", fmt.Sprintf("post-mortem verdict: %s report=%s", pmVerdict, pmReport))
			evidence = append(evidence, pathClean(pmReport))
		}
	}
	if artifact, evalErr := emitPhaseEvaluatorArtifact(cwd, state, 3, verdict, findings, evidence...); evalErr != nil {
		VerbosePrintf("Warning: could not write validation evaluator artifact: %v\n", evalErr)
	} else if artifact != nil {
		state.Verdicts["validation_evaluator"] = artifact.Verdict
		if verdict == "FAIL" {
			findings = artifact.Findings
		}
	}
	if verdict == "FAIL" {
		return &gateFailError{Phase: 3, Verdict: verdict, Findings: findings, Report: report}
	}
	return nil
}

func legacyGateAction(attempt, maxRetries int) types.MemRLAction {
	return rpi.LegacyGateAction(attempt, maxRetries)
}

func classifyGateFailureClass(phaseNum int, gateErr *gateFailError) types.MemRLFailureClass {
	if gateErr == nil {
		return ""
	}
	return rpi.ClassifyGateFailureClass(phaseNum, gateErr.Verdict)
}

func resolveGateRetryAction(state *phasedState, phaseNum int, gateErr *gateFailError, attempt int) (types.MemRLAction, types.MemRLPolicyDecision) {
	mode := types.GetMemRLMode()
	failureClass := classifyGateFailureClass(phaseNum, gateErr)
	metadataPresent := gateErr != nil && strings.TrimSpace(gateErr.Verdict) != ""

	decision := types.EvaluateDefaultMemRLPolicy(types.MemRLPolicyInput{
		Mode:            mode,
		FailureClass:    failureClass,
		Attempt:         attempt,
		MaxAttempts:     state.Opts.MaxRetries,
		MetadataPresent: metadataPresent,
	})

	legacy := rpi.LegacyGateAction(attempt, state.Opts.MaxRetries)
	if mode == types.MemRLModeEnforce {
		return decision.Action, decision
	}
	return legacy, decision
}

// handleGateRetry manages retry logic for failed gates.
// spawnCwd is the working directory for spawned claude sessions (may be worktree).
func handleGateRetry(ctx context.Context, cwd string, state *phasedState, phaseNum int, gateErr *gateFailError, logPath string, spawnCwd string, statusPath string, allPhases []PhaseProgress, executor PhaseExecutor) (bool, error) {
	phaseName := phases[phaseNum-1].Name
	attemptKey := fmt.Sprintf("phase_%d", phaseNum)

	state.Attempts[attemptKey]++
	attempt := state.Attempts[attemptKey]

	// Hard ceiling: force escalation regardless of MemRL policy
	if shouldForceEscalation(attempt) {
		return performGateEscalation(state, phaseNum, attempt, gateErr,
			types.MemRLPolicyDecision{}, types.MemRLActionEscalate,
			phaseName, logPath, statusPath, allPhases)
	}

	maybeUpdateLiveStatus(state, statusPath, allPhases, phaseNum, "retrying after "+gateErr.Verdict, attempt, "")

	action, decision := resolveGateRetryAction(state, phaseNum, gateErr, attempt)
	logGateRetryMemRL(logPath, state.RunID, phaseName, decision, action)

	if action == types.MemRLActionEscalate {
		return performGateEscalation(state, phaseNum, attempt, gateErr, decision, action, phaseName, logPath, statusPath, allPhases)
	}

	fmt.Printf("%s: %s (attempt %d/%d) — retrying\n", phaseName, gateErr.Verdict, attempt, state.Opts.MaxRetries)
	_, _ = appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   phaseNum,
		Type:    "gate.retry.attempt",
		Message: fmt.Sprintf("%s retry attempt %d/%d", phaseName, attempt, state.Opts.MaxRetries),
		Details: map[string]any{"attempt": attempt, "max_retries": state.Opts.MaxRetries, "verdict": gateErr.Verdict},
	})
	logPhaseTransition(logPath, state.RunID, phaseName, fmt.Sprintf("RETRY attempt %d/%d verdict=%s report=%s", attempt, state.Opts.MaxRetries, gateErr.Verdict, gateErr.Report))

	retryCtx := &retryContext{
		Attempt:  attempt,
		Findings: gateErr.Findings,
		Verdict:  gateErr.Verdict,
	}

	retryPrompt, err := buildRetryPrompt(cwd, phaseNum, state, retryCtx)
	if err != nil {
		return false, fmt.Errorf("build retry prompt: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would spawn retry: %s\n", formatRuntimePromptInvocation(effectiveRuntimeCommand(state.Opts.RuntimeCommand), retryPrompt))
		return false, nil
	}

	if err := executeWithStatus(ctx, executor, state, statusPath, allPhases, phaseNum, attempt, retryPrompt, spawnCwd, "running retry prompt", "retry failed"); err != nil {
		return false, fmt.Errorf("retry failed: %w", err)
	}

	rerunPrompt, err := buildPromptForPhase(cwd, phaseNum, state, nil)
	if err != nil {
		return false, fmt.Errorf("build rerun prompt: %w", err)
	}

	fmt.Printf("Re-running phase %d after retry\n", phaseNum)
	if err := executeWithStatus(ctx, executor, state, statusPath, allPhases, phaseNum, attempt, rerunPrompt, spawnCwd, "re-running phase", "rerun failed"); err != nil {
		return false, fmt.Errorf("rerun failed: %w", err)
	}

	return verifyGateAfterRetry(ctx, cwd, state, phaseNum, logPath, spawnCwd, statusPath, allPhases, executor, attempt)
}

func maybeUpdateLiveStatus(state *phasedState, statusPath string, allPhases []PhaseProgress, phaseNum int, status string, attempt int, errMsg string) {
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, status, attempt, errMsg)
	}
}

func executeWithStatus(ctx context.Context, executor PhaseExecutor, state *phasedState, statusPath string, allPhases []PhaseProgress, phaseNum, attempt int, prompt, spawnCwd, runningMsg, failedMsg string) error {
	maybeUpdateLiveStatus(state, statusPath, allPhases, phaseNum, runningMsg, attempt, "")
	if err := executor.Execute(ctx, prompt, spawnCwd, state.RunID, phaseNum); err != nil {
		maybeUpdateLiveStatus(state, statusPath, allPhases, phaseNum, failedMsg, attempt, err.Error())
		return err
	}
	return nil
}

// logGateRetryMemRL logs the MemRL policy decision for a gate retry, if mode is not off.
func logGateRetryMemRL(logPath, runID, phaseName string, decision types.MemRLPolicyDecision, action types.MemRLAction) {
	if decision.Mode == types.MemRLModeOff {
		return
	}
	logPhaseTransition(
		logPath,
		runID,
		phaseName,
		fmt.Sprintf(
			"memrl policy mode=%s failure_class=%s attempt_bucket=%s policy_action=%s selected_action=%s rule=%s",
			decision.Mode,
			decision.FailureClass,
			decision.AttemptBucket,
			decision.Action,
			action,
			decision.RuleID,
		),
	)
}

// performGateEscalation handles the escalation path when the retry action is escalate.
// Returns (false, nil) to signal escalation without error (caller will handle reporting).
func performGateEscalation(state *phasedState, phaseNum, attempt int, gateErr *gateFailError, decision types.MemRLPolicyDecision, action types.MemRLAction, phaseName, logPath, statusPath string, allPhases []PhaseProgress) (bool, error) {
	msg := fmt.Sprintf(
		"%s escalated (mode=%s, action=%s, rule=%s, attempt=%d/%d). Last report: %s. Manual intervention needed.",
		phaseName,
		decision.Mode,
		action,
		decision.RuleID,
		attempt,
		state.Opts.MaxRetries,
		gateErr.Report,
	)
	fmt.Println(msg)
	escalationRoot := filepath.Dir(filepath.Dir(filepath.Dir(logPath)))
	_, _ = appendRPIC2Event(escalationRoot, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   phaseNum,
		Type:    "gate.escalation",
		Message: msg,
		Details: map[string]any{"attempt": attempt, "report": gateErr.Report},
	})
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "failed after retries", attempt, gateErr.Report)
	}
	logPhaseTransition(logPath, state.RunID, phaseName, msg)
	return false, nil
}

// verifyGateAfterRetry re-checks the gate after a retry session completes.
// If the gate still fails, it recurses into handleGateRetry.
func verifyGateAfterRetry(ctx context.Context, cwd string, state *phasedState, phaseNum int, logPath, spawnCwd, statusPath string, allPhases []PhaseProgress, executor PhaseExecutor, attempt int) (bool, error) {
	if err := postPhaseProcessing(cwd, state, phaseNum, logPath); err != nil {
		var gateErr *gateFailError
		if errors.As(err, &gateErr) {
			// Still failing — recurse
			return handleGateRetry(ctx, cwd, state, phaseNum, gateErr, logPath, spawnCwd, statusPath, allPhases, executor)
		}
		return false, err
	}
	if state.Opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, "retry succeeded", attempt, "")
	}
	return true, nil
}
