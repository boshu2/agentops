package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

type phaseSessionOutcome struct {
	SessionID      string   `json:"session_id"`
	TranscriptPath string   `json:"transcript_path,omitempty"`
	Reward         float64  `json:"reward,omitempty"`
	Signals        []Signal `json:"signals,omitempty"`
	AnalyzedAt     string   `json:"analyzed_at,omitempty"`
}

type phaseEvaluatorArtifact struct {
	SchemaVersion  int                  `json:"schema_version"`
	RunID          string               `json:"run_id"`
	Phase          int                  `json:"phase"`
	PhaseName      string               `json:"phase_name"`
	GateVerdict    string               `json:"gate_verdict,omitempty"`
	Verdict        string               `json:"verdict"`
	Summary        string               `json:"summary"`
	Findings       []finding            `json:"findings,omitempty"`
	Evidence       []string             `json:"evidence,omitempty"`
	SessionOutcome *phaseSessionOutcome `json:"session_outcome,omitempty"`
	GeneratedAt    string               `json:"generated_at"`
}

func emitPhaseEvaluatorArtifact(cwd string, state *phasedState, phaseNum int, gateVerdict string, gateFindings []finding, evidence ...string) (*phaseEvaluatorArtifact, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}

	outcome := collectPhaseSessionOutcome(cwd, state.RunID, phaseNum)
	evidence = collectPhaseEvaluatorEvidence(cwd, phaseNum, evidence...)
	findings := append([]finding{}, gateFindings...)
	findings = append(findings, defaultEvaluatorFindings(phaseNum, state, gateVerdict, outcome, evidence)...)
	findings = uniqueFindings(findings)

	artifact := &phaseEvaluatorArtifact{
		SchemaVersion:  1,
		RunID:          state.RunID,
		Phase:          phaseNum,
		PhaseName:      phaseNameForNumber(phaseNum),
		GateVerdict:    strings.ToUpper(strings.TrimSpace(gateVerdict)),
		Verdict:        phaseEvaluatorVerdict(phaseNum, state, gateVerdict, outcome),
		Summary:        phaseEvaluatorSummary(phaseNum, state, gateVerdict, outcome, findings),
		Findings:       findings,
		Evidence:       evidence,
		SessionOutcome: outcome,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := writePhaseEvaluatorArtifact(cwd, artifact); err != nil {
		return nil, err
	}

	if state.Verdicts == nil {
		state.Verdicts = make(map[string]string)
	}
	state.Verdicts[fmt.Sprintf("%s_evaluator", artifact.PhaseName)] = artifact.Verdict
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   phaseNum,
		Backend: state.Backend,
		Source:  "orchestrator",
		Type:    "phase.evaluator.updated",
		Message: fmt.Sprintf("Phase %d evaluator verdict: %s", phaseNum, artifact.Verdict),
		Details: map[string]any{
			"verdict":      artifact.Verdict,
			"gate_verdict": artifact.GateVerdict,
			"artifact":     filepath.ToSlash(filepath.Join(".agents", "rpi", fmt.Sprintf(phaseEvaluatorFileFmt, phaseNum))),
		},
	}); err != nil {
		VerbosePrintf("Warning: could not emit phase.evaluator.updated event: %v\n", err)
	}

	return artifact, nil
}

func writePhaseEvaluatorArtifact(cwd string, artifact *phaseEvaluatorArtifact) error {
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return fmt.Errorf("create evaluator directory: %w", err)
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evaluator artifact: %w", err)
	}
	data = append(data, '\n')

	path := filepath.Join(stateDir, fmt.Sprintf(phaseEvaluatorFileFmt, artifact.Phase))
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write evaluator tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename evaluator artifact: %w", err)
	}
	return nil
}

func collectPhaseEvaluatorEvidence(cwd string, phaseNum int, extra ...string) []string {
	var evidence []string
	evidence = append(evidence, latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, phaseNum))))
	evidence = append(evidence, latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", fmt.Sprintf("phase-%d-summary*.md", phaseNum))))
	evidence = append(evidence, latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "execution-packet.json")))
	for _, item := range extra {
		item = pathClean(item)
		if !isSafeArtifactRelPath(item) {
			continue
		}
		full := filepath.Join(cwd, filepath.FromSlash(item))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			evidence = append(evidence, item)
		}
	}
	return uniqueStringsPreserveOrder(evidence)
}

func collectPhaseSessionOutcome(cwd, runID string, phaseNum int) *phaseSessionOutcome {
	sessionID := latestPhaseSessionID(cwd, runID, phaseNum)
	if sessionID == "" {
		return nil
	}

	// Structured evidence from the execution packet's validation_lanes_results
	// can override regex-based test detection in the transcript: a clean run
	// that did not print "PASSED" still credits tests_pass when a lane PASSed.
	packetPath := filepath.Join(cwd, ".agents", "rpi", rpi.ExecutionPacketFile)
	laneResults, err := rpi.ParseValidationLanesResults(packetPath)
	if err != nil {
		VerbosePrintf("Warning: could not parse validation_lanes_results from %s: %v\n", packetPath, err)
	}
	testsPassEvidence := rpi.AnyLaneResultPassed(laneResults)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &phaseSessionOutcome{SessionID: sessionID}
	}
	transcriptPath := findTranscriptForSession(filepath.Join(homeDir, ".claude", "projects"), sessionID)
	if transcriptPath == "" {
		return &phaseSessionOutcome{SessionID: sessionID}
	}

	outcome, err := analyzeTranscriptWithEvidence(transcriptPath, sessionID, testsPassEvidence)
	if err != nil {
		return &phaseSessionOutcome{
			SessionID:      sessionID,
			TranscriptPath: transcriptPath,
		}
	}
	return &phaseSessionOutcome{
		SessionID:      outcome.SessionID,
		TranscriptPath: transcriptPath,
		Reward:         outcome.Reward,
		Signals:        outcome.Signals,
		AnalyzedAt:     outcome.AnalyzedAt.UTC().Format(time.RFC3339),
	}
}

func latestPhaseSessionID(cwd, runID string, phaseNum int) string {
	if strings.TrimSpace(runID) == "" {
		return ""
	}
	events, err := loadRPIC2Events(cwd, runID)
	if err != nil {
		return ""
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Phase != phaseNum {
			continue
		}
		if sessionID := sessionIDFromRPIEvent(events[i]); sessionID != "" {
			return sessionID
		}
	}
	return ""
}

func sessionIDFromRPIEvent(ev RPIC2Event) string {
	return rpi.SessionIDFromEventDetails(ev.Details)
}

func phaseEvaluatorVerdict(phaseNum int, state *phasedState, gateVerdict string, outcome *phaseSessionOutcome) string {
	trackerMode := ""
	if state != nil {
		trackerMode = state.TrackerMode
	}
	hasTranscript := outcome != nil && outcome.TranscriptPath != ""
	reward := 0.0
	if outcome != nil {
		reward = outcome.Reward
	}
	return rpi.PhaseEvaluatorVerdict(phaseNum, trackerMode, gateVerdict, hasTranscript, reward)
}

func phaseEvaluatorSummary(phaseNum int, state *phasedState, gateVerdict string, outcome *phaseSessionOutcome, findings []finding) string {
	trackerMode := ""
	if state != nil {
		trackerMode = state.TrackerMode
	}
	hasTranscript := outcome != nil && outcome.TranscriptPath != ""
	reward := 0.0
	if outcome != nil {
		reward = outcome.Reward
	}
	return rpi.PhaseEvaluatorSummary(phaseNum, trackerMode, gateVerdict, hasTranscript, reward, len(findings))
}

func defaultEvaluatorFindings(phaseNum int, state *phasedState, gateVerdict string, outcome *phaseSessionOutcome, evidence []string) []finding {
	trackerMode := ""
	if state != nil {
		trackerMode = state.TrackerMode
	}
	hasTranscript := outcome != nil && outcome.TranscriptPath != ""
	reward := 0.0
	transcriptPath := ""
	if outcome != nil {
		reward = outcome.Reward
		transcriptPath = outcome.TranscriptPath
	}
	ref := ""
	if len(evidence) > 0 {
		ref = evidence[0]
	}
	rpiFindings := rpi.DefaultEvaluatorFindings(phaseNum, trackerMode, gateVerdict, hasTranscript, reward, transcriptPath, ref)
	out := make([]finding, len(rpiFindings))
	for i, f := range rpiFindings {
		out[i] = finding(f)
	}
	return out
}

func uniqueFindings(items []finding) []finding {
	rpiItems := make([]rpi.Finding, len(items))
	for i, f := range items {
		rpiItems[i] = rpi.Finding(f)
	}
	deduped := rpi.UniqueFindings(rpiItems)
	out := make([]finding, len(deduped))
	for i, f := range deduped {
		out[i] = finding(f)
	}
	return out
}

func phaseNameForNumber(phaseNum int) string {
	return rpi.PhaseNameForNumber(phaseNum)
}
