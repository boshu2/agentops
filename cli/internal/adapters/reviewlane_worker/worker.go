package reviewlane_worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type Adapter struct {
	worker       agentworker.AgentWorker
	kind         agentworker.WorkerKind
	provider     agentworker.Provider
	pollInterval time.Duration
	waitTimeout  time.Duration
}

func New(worker agentworker.AgentWorker, kind agentworker.WorkerKind, provider agentworker.Provider) *Adapter {
	return &Adapter{
		worker: worker, kind: kind, provider: provider,
		pollInterval: 250 * time.Millisecond,
		waitTimeout:  10 * time.Minute,
	}
}

func (a *Adapter) Run(ctx context.Context, request ports.ReviewRequestV1) (ports.ReviewLaneResultV1, error) {
	if err := request.Validate(); err != nil {
		return ports.ReviewLaneResultV1{}, err
	}
	evidencePath := reviewEvidencePath(request)
	session, err := a.worker.Start(ctx, agentworker.StartRequest{
		WorkerKind: a.kind, Provider: a.provider, JobID: "pawl-" + request.SubjectID,
		AttemptID: request.Nonce, RequestID: request.Nonce, CWD: request.EvidenceDir,
		Prompt: reviewPrompt(request, evidencePath),
		Metadata: map[string]string{
			"review_nonce": request.Nonce, "head_sha": request.HeadSHA, "read_only": "true",
			"review_evidence_path": evidencePath,
		},
	})
	if err != nil {
		return transportResult("start", familyFor(a.kind), request.Nonce, err.Error()), nil
	}
	state, err := a.waitForTerminal(ctx, session)
	if err != nil {
		return transportResult(session.Ref().SessionID, familyFor(a.kind), request.Nonce, err.Error()), nil
	}
	if state.Status == agentworker.StatusProviderUnreachable || state.Status == agentworker.StatusLost || !state.Successful() {
		return transportResult(session.Ref().SessionID, familyFor(a.kind), request.Nonce, state.Reason), nil
	}
	transcript, err := session.Transcript(ctx)
	if err != nil {
		return transportResult(session.Ref().SessionID, familyFor(a.kind), request.Nonce, err.Error()), nil
	}
	artifacts, err := session.Artifacts(ctx)
	if err != nil {
		return transportResult(session.Ref().SessionID, familyFor(a.kind), request.Nonce, err.Error()), nil
	}
	result, err := parseSemanticResult(session.Ref().SessionID, familyFor(a.kind), transcript.Text, artifacts)
	if err != nil {
		return ports.ReviewLaneResultV1{}, err
	}
	if err := result.ValidateAgainst(request); err != nil {
		return ports.ReviewLaneResultV1{}, err
	}
	return result, nil
}

func (a *Adapter) waitForTerminal(ctx context.Context, session agentworker.AgentSession) (agentworker.TerminalState, error) {
	waitCtx := ctx
	cancel := func() {}
	if _, bounded := ctx.Deadline(); !bounded {
		waitCtx, cancel = context.WithTimeout(ctx, a.waitTimeout)
	}
	defer cancel()

	for {
		state, err := session.TerminalState(waitCtx)
		if err != nil {
			return agentworker.TerminalState{}, err
		}
		if state.Terminal() {
			return state, nil
		}

		timer := time.NewTimer(a.pollInterval)
		select {
		case <-waitCtx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return agentworker.TerminalState{}, fmt.Errorf("wait for reviewer terminal state: %w", waitCtx.Err())
		case <-timer.C:
		}
	}
}

func reviewEvidencePath(r ports.ReviewRequestV1) string {
	digest := sha256.Sum256([]byte(r.SubjectID + "\x00" + r.HeadSHA + "\x00" + r.Nonce))
	return filepath.Join(r.EvidenceDir, fmt.Sprintf("review-evidence-%x.txt", digest[:8]))
}

func reviewPrompt(r ports.ReviewRequestV1, evidencePath string) string {
	return fmt.Sprintf("PAWL REVIEW\nSUBJECT: %s\nHEAD: %s\nCONTRACT: %s\nCONTRACT_SHA256: %s\nDIFF: %s\nDIFF_SHA256: %s\nNONCE: %s\nREAD_ONLY: true\nEVIDENCE_DIR: %s\nEVIDENCE_PATH: %s\nReturn VERDICT, NONCE, CONTEXT, and READ_ONLY markers. Write nonempty review evidence to EVIDENCE_PATH; the adapter will durably materialize the transcript there if the runtime does not write a richer artifact.", r.SubjectID, r.HeadSHA, r.AcceptanceContract, r.AcceptanceContractSHA256, r.DiffPath, r.DiffSHA256, r.Nonce, r.EvidenceDir, evidencePath)
}

func familyFor(kind agentworker.WorkerKind) string {
	if kind == agentworker.WorkerKindClaude {
		return "claude"
	}
	return "gpt"
}

func transportResult(lane, family, nonce, reason string) ports.ReviewLaneResultV1 {
	if strings.TrimSpace(reason) == "" {
		reason = "worker ended without usable reviewer evidence"
	}
	return ports.ReviewLaneResultV1{SchemaVersion: "review-lane-result.v1", LaneID: lane, Family: family, ContextID: lane, FailureClass: ports.ReviewFailureTransport, FailureReason: reason, Nonce: nonce}
}

func parseSemanticResult(lane, family, transcript string, artifacts []agentworker.Artifact) (ports.ReviewLaneResultV1, error) {
	markers := map[string]string{}
	findings := make([]ports.ReviewFinding, 0)
	for _, line := range strings.Split(transcript, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.TrimSpace(strings.ToUpper(parts[0])), strings.TrimSpace(parts[1])
		markers[key] = value
		if key == "FINDING" && value != "" {
			findings = append(findings, ports.ReviewFinding{Title: value, Evidence: value})
		}
	}
	var disposition ports.ReviewDisposition
	switch strings.ToUpper(markers["VERDICT"]) {
	case "CONFIRMED":
		disposition = ports.ReviewConfirmed
	case "REFUTED":
		disposition = ports.ReviewRefuted
	default:
		return ports.ReviewLaneResultV1{}, fmt.Errorf("review transcript lacks a semantic VERDICT marker")
	}
	evidence := ""
	for _, artifact := range artifacts {
		if artifact.Kind == "review-evidence" && artifact.ValidationStatus == "valid" && artifact.Path != "" {
			evidence = artifact.Path
			break
		}
	}
	return ports.ReviewLaneResultV1{
		SchemaVersion: "review-lane-result.v1", LaneID: lane, Family: family,
		ContextID: markers["CONTEXT"], Disposition: disposition, FailureClass: ports.ReviewFailureSemantic,
		Findings: findings, EvidencePath: evidence, Nonce: markers["NONCE"],
		ReadOnly: strings.EqualFold(markers["READ_ONLY"], "true"),
	}, nil
}
