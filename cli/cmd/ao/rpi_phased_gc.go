// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type rpiGasCityClient interface {
	CityReadiness(context.Context, string) (gascity.ReadinessResponse, error)
	CreateSession(context.Context, string, gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error)
	SubmitSession(context.Context, string, string, gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error)
	GetSession(context.Context, string, string, gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error)
	SessionTranscript(context.Context, string, string, gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error)
	ListCityEvents(context.Context, string, gascity.EventListParams) (gascity.EventListResponse, gascity.ResponseMeta, error)
	StreamCityEvents(context.Context, string, gascity.EventStreamOptions) (rpiGasCityEventStream, gascity.ResponseMeta, error)
	EmitCityEvent(context.Context, string, gascity.EventEmitRequest) (gascity.EventEmitResponse, gascity.ResponseMeta, error)
}

type rpiGasCityEventStream interface {
	NextEvent() (gascity.EventStreamFrame, error)
	Close() error
}

type rpiGasCityAPIAdapter struct {
	client *gascity.Client
}

func (a *rpiGasCityAPIAdapter) CityReadiness(ctx context.Context, cityName string) (gascity.ReadinessResponse, error) {
	return a.client.CityReadiness(ctx, cityName)
}

func (a *rpiGasCityAPIAdapter) CreateSession(ctx context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	return a.client.CreateSession(ctx, cityName, req)
}

func (a *rpiGasCityAPIAdapter) SubmitSession(ctx context.Context, cityName string, id string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	return a.client.SubmitSession(ctx, cityName, id, req)
}

func (a *rpiGasCityAPIAdapter) GetSession(ctx context.Context, cityName string, id string, opts gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	return a.client.GetSession(ctx, cityName, id, opts)
}

func (a *rpiGasCityAPIAdapter) SessionTranscript(ctx context.Context, cityName string, id string, opts gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	return a.client.SessionTranscript(ctx, cityName, id, opts)
}

func (a *rpiGasCityAPIAdapter) ListCityEvents(ctx context.Context, cityName string, params gascity.EventListParams) (gascity.EventListResponse, gascity.ResponseMeta, error) {
	return a.client.ListCityEvents(ctx, cityName, params)
}

func (a *rpiGasCityAPIAdapter) StreamCityEvents(ctx context.Context, cityName string, opts gascity.EventStreamOptions) (rpiGasCityEventStream, gascity.ResponseMeta, error) {
	return a.client.StreamCityEvents(ctx, cityName, opts)
}

func (a *rpiGasCityAPIAdapter) EmitCityEvent(ctx context.Context, cityName string, req gascity.EventEmitRequest) (gascity.EventEmitResponse, gascity.ResponseMeta, error) {
	return a.client.EmitCityEvent(ctx, cityName, req)
}

type rpiGasCityPhaseRecord struct {
	CityName             string
	SessionAlias         string
	SessionID            string
	CreateRequestID      string
	SubmitRequestID      string
	EventStreamRequestID string
	LastEventID          string
	StartedEventSeen     bool
	TerminalStatus       string
	EvidencePath         string
}

// gcExecutor implements PhaseExecutor using Gas City's gc CLI for session management.
// It starts a gc session with the phase prompt via `gc session nudge`, monitors
// progress via `gc session peek`, and emits ao:phase events to the gc event bus.
type gcExecutor struct {
	cityPath     string        // path to city.toml directory; empty = auto-discover
	phaseTimeout time.Duration // max time per phase
	pollInterval time.Duration // how often to check session status
	execCommand  gcExecFn      // nil = exec.Command
	lookPath     gcLookFn      // nil = exec.LookPath
	apiClient    rpiGasCityClient
	apiCityName  string
	apiLastPhase rpiGasCityPhaseRecord
}

func (g *gcExecutor) Name() string { return "gc" }

func (g *gcExecutor) backendMode() string {
	if _, ok := g.gasCityClient(); ok {
		return "gc-api"
	}
	return "gc-cli-fallback"
}

func (g *gcExecutor) gasCityClient() (rpiGasCityClient, bool) {
	if g == nil || g.apiClient == nil {
		return nil, false
	}
	return g.apiClient, true
}

func (g *gcExecutor) Execute(ctx context.Context, prompt, cwd, runID string, phaseNum int) error {
	cityPath := g.resolveCityPath(cwd)
	if cityPath == "" {
		return fmt.Errorf("gc executor: no city.toml found (walk up from %s)", cwd)
	}

	if client, ok := g.gasCityClient(); ok {
		return g.executeAPISession(ctx, client, cityPath, cwd, prompt, runID, phaseNum)
	}

	ready, reason := gcBridgeReady(cityPath, g.execCommand, g.lookPath)
	if !ready {
		return fmt.Errorf("gc executor: not ready: %s", reason)
	}

	_ = gcEmitPhaseEvent(cityPath, phaseNum, "started", runID, g.execCommand, g.lookPath)

	sessionAlias := fmt.Sprintf("rpi-%s-p%d", runID, phaseNum)
	if err := gcRunCommand(g.execCommand, cityPath, gcSessionNewArgs("worker", sessionAlias)...); err != nil {
		return fmt.Errorf("gc executor: create session %q: %w", sessionAlias, err)
	}
	if err := gcRunCommand(g.execCommand, cityPath, gcNudgeArgs(sessionAlias, prompt)...); err != nil {
		return fmt.Errorf("gc executor: nudge session %q: %w", sessionAlias, err)
	}

	return g.pollSessionCompletion(ctx, cityPath, sessionAlias, runID, phaseNum)
}

func (g *gcExecutor) executeAPISession(
	ctx context.Context,
	client rpiGasCityClient,
	cityPath string,
	cwd string,
	prompt string,
	runID string,
	phaseNum int,
) error {
	cityName := g.resolveAPICityName(cityPath)
	if cityName == "" {
		return fmt.Errorf("gc executor: api city name is required")
	}
	ready, err := client.CityReadiness(ctx, cityName)
	if err != nil {
		return fmt.Errorf("gc executor: api city readiness for %q: %w", cityName, err)
	}
	if !ready.Ready {
		return fmt.Errorf("gc executor: api city %q not ready: %s", cityName, ready.Status)
	}

	sessionAlias := fmt.Sprintf("rpi-%s-p%d", runID, phaseNum)
	session, createMeta, err := client.CreateSession(ctx, cityName, gascity.SessionCreateRequest{
		Kind:  "agent",
		Name:  "worker",
		Alias: sessionAlias,
		Async: true,
	})
	if err != nil {
		return fmt.Errorf("gc executor: api create session %q: %w", sessionAlias, err)
	}
	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return fmt.Errorf("gc executor: api create session %q returned empty session ID", sessionAlias)
	}
	g.apiLastPhase = rpiGasCityPhaseRecord{
		CityName:        cityName,
		SessionAlias:    sessionAlias,
		SessionID:       sessionID,
		CreateRequestID: createMeta.RequestID,
	}

	_, submitMeta, err := client.SubmitSession(ctx, cityName, sessionID, gascity.SessionSubmitRequest{
		Message: prompt,
		Intent:  "follow_up",
	})
	if err != nil {
		return fmt.Errorf("gc executor: api submit session %q: %w", sessionID, err)
	}
	g.apiLastPhase.SubmitRequestID = submitMeta.RequestID
	return g.waitAPIEventCompletion(ctx, client, cityName, sessionID, cwd, runID, phaseNum)
}

func (g *gcExecutor) waitAPIEventCompletion(
	ctx context.Context,
	client rpiGasCityClient,
	cityName string,
	sessionID string,
	cwd string,
	runID string,
	phaseNum int,
) error {
	timeout := g.phaseTimeout
	if timeout == 0 {
		timeout = 90 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stream, meta, err := client.StreamCityEvents(waitCtx, cityName, gascity.EventStreamOptions{})
	if err != nil {
		return fmt.Errorf("gc executor: api event stream for %q: %w", sessionID, err)
	}
	g.apiLastPhase.EventStreamRequestID = meta.RequestID
	defer stream.Close()

	for {
		frame, err := stream.NextEvent()
		if err != nil {
			if waitCtx.Err() != nil {
				return fmt.Errorf("gc executor: api event wait for %q: %w", sessionID, waitCtx.Err())
			}
			return fmt.Errorf("gc executor: api event stream ended before terminal state for %q: %w", sessionID, err)
		}
		if cursor := gascity.CursorFromFrame(frame); cursor != "" {
			g.apiLastPhase.LastEventID = cursor
		}
		event := frame.CityEvent
		if event == nil || !apiEventMatchesPhase(event, g.apiLastPhase) {
			continue
		}
		if apiEventLooksStarted(event.Type, event.Payload) {
			g.apiLastPhase.StartedEventSeen = true
		}

		classification := gascity.ClassifyTerminalState(gascity.TerminalStateInput{
			EventType:    event.Type,
			EventPayload: event.Payload,
		})
		if !classification.Terminal {
			continue
		}
		g.apiLastPhase.TerminalStatus = classification.Status
		if classification.Status == gascity.TerminalStatusCompleted && !classification.Degraded {
			return g.captureAPITerminalEvidence(waitCtx, client, cityName, sessionID, cwd, runID, phaseNum)
		}
		if classification.Reason != "" {
			return fmt.Errorf("gc executor: api session %q terminal %s: %s", sessionID, classification.Status, classification.Reason)
		}
		return fmt.Errorf("gc executor: api session %q terminal %s", sessionID, classification.Status)
	}
}

func (g *gcExecutor) captureAPITerminalEvidence(
	ctx context.Context,
	client rpiGasCityClient,
	cityName string,
	sessionID string,
	cwd string,
	runID string,
	phaseNum int,
) error {
	transcript, meta, err := client.SessionTranscript(ctx, cityName, sessionID, gascity.TranscriptOptions{
		Format: "conversation",
	})
	if err != nil {
		return fmt.Errorf("gc executor: api transcript for %q: %w", sessionID, err)
	}
	requestIDs := map[string]string{}
	addRequestID := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			requestIDs[key] = value
		}
	}
	addRequestID("create", g.apiLastPhase.CreateRequestID)
	addRequestID("submit", g.apiLastPhase.SubmitRequestID)
	addRequestID("stream", g.apiLastPhase.EventStreamRequestID)
	addRequestID("transcript", meta.RequestID)

	artifacts := make([]cliRPI.GasCityTranscriptArtifact, 0, len(transcript.Artifacts))
	for _, artifact := range transcript.Artifacts {
		artifacts = append(artifacts, cliRPI.GasCityTranscriptArtifact{
			Path: artifact.Path,
			Kind: artifact.Kind,
		})
	}
	path, err := cliRPI.WriteGasCityPhaseEvidence(cwd, cliRPI.GasCityPhaseEvidence{
		RunID:                runID,
		Phase:                phaseNum,
		PhaseName:            cliRPI.PhaseNameForNumber(phaseNum),
		CityName:             cityName,
		SessionID:            sessionID,
		SessionAlias:         g.apiLastPhase.SessionAlias,
		Status:               g.apiLastPhase.TerminalStatus,
		EventCursor:          g.apiLastPhase.LastEventID,
		RequestIDs:           requestIDs,
		TranscriptID:         firstNonEmpty(transcript.ID, transcript.SessionID, sessionID),
		TranscriptFormat:     transcript.Format,
		TranscriptTurnCount:  len(transcript.Turns),
		TranscriptMsgCount:   len(transcript.Messages),
		TranscriptArtifacts:  artifacts,
		TranscriptCapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	g.apiLastPhase.EvidencePath = path
	return nil
}

// resolveCityPath returns the city path from the executor config or discovers it.
func (g *gcExecutor) resolveCityPath(cwd string) string {
	if g.cityPath != "" {
		return g.cityPath
	}
	return gcBridgeCityPath(cwd)
}

func (g *gcExecutor) resolveAPICityName(cityPath string) string {
	if g.apiCityName != "" {
		return strings.TrimSpace(g.apiCityName)
	}
	if cityPath == "" {
		return ""
	}
	return strings.TrimSpace(filepath.Base(cityPath))
}

func apiEventMatchesPhase(event *gascity.EventStreamEnvelope, phase rpiGasCityPhaseRecord) bool {
	if event == nil {
		return false
	}
	for _, candidate := range []string{event.Subject, apiEventPayloadString(event.Payload, "session_id"), apiEventPayloadString(event.Payload, "sessionId"), apiEventPayloadString(event.Payload, "alias")} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		switch candidate {
		case phase.SessionID, phase.SessionAlias:
			return true
		}
	}
	return false
}

func apiEventLooksStarted(eventType string, payload map[string]any) bool {
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	status := strings.ToLower(strings.TrimSpace(apiEventPayloadString(payload, "status")))
	return strings.Contains(eventType, ".started") ||
		strings.Contains(eventType, ".created") ||
		status == "running" ||
		status == "started"
}

func apiEventPayloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// pollSessionCompletion blocks until the gc session finishes, is cancelled, or times out.
func (g *gcExecutor) pollSessionCompletion(ctx context.Context, cityPath, sessionAlias, runID string, phaseNum int) error {
	pollInterval := g.pollInterval
	if pollInterval == 0 {
		pollInterval = 10 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeout := g.phaseTimeout
	if timeout == 0 {
		timeout = 90 * time.Minute
	}
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			_ = gcEmitPhaseEvent(cityPath, phaseNum, "cancelled", runID, g.execCommand, g.lookPath)
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("gc executor: phase %d timed out after %v", phaseNum, timeout)
		case <-ticker.C:
			done, err := g.checkSessionDone(cityPath, sessionAlias)
			if err != nil {
				var lostErr *cliRPI.ProviderSessionLostError
				if errors.As(err, &lostErr) {
					_ = gcEmitPhaseEvent(cityPath, phaseNum, cliRPI.ProviderSessionLost, runID, g.execCommand, g.lookPath)
					return err
				}
				continue // transient error, retry on next tick
			}
			if done {
				_ = gcEmitPhaseEvent(cityPath, phaseNum, "complete", runID, g.execCommand, g.lookPath)
				return nil
			}
		}
	}
}

// checkSessionDone returns true if the session is closed/completed.
func (g *gcExecutor) checkSessionDone(cityPath, sessionAlias string) (bool, error) {
	out, err := gcDefaultExec(g.execCommand)("gc", "--city", cityPath, "session", "list", "--json").Output()
	if err != nil {
		return false, fmt.Errorf("gc session list: %w", err)
	}
	sessions, err := parseGCSessions(out)
	if err != nil {
		return false, fmt.Errorf("parse sessions: %w", err)
	}
	for _, s := range sessions {
		if s.Alias == sessionAlias {
			return gcSessionDone(s), nil
		}
	}
	return false, &cliRPI.ProviderSessionLostError{SessionAlias: sessionAlias}
}

func gcSessionDone(s GCSession) bool {
	if s.Closed {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(s.State)) {
	case "closed", "completed", "asleep", "suspended", "drained", "archived", "stopped":
		return true
	default:
		return false
	}
}

// gcRunCommand runs a gc CLI command with optional city path prefix.
func gcRunCommand(execCommand gcExecFn, cityPath string, args ...string) error {
	if cityPath != "" {
		// Check if --city is already in args
		hasCity := false
		for _, a := range args {
			if a == "--city" {
				hasCity = true
				break
			}
		}
		if !hasCity {
			args = append([]string{"--city", cityPath}, args...)
		}
	}
	cmd := gcDefaultExec(execCommand)("gc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gcExecutorAvailable returns true if gc bridge is ready for use as a phase executor.
// This is used by selectExecutorFromCaps to determine if the gc backend should be offered.
func gcExecutorAvailable(cwd string, execCommand gcExecFn, lookPath gcLookFn) bool {
	ready, _ := gcExecutorAvailability(cwd, execCommand, lookPath)
	return ready
}

func gcExecutorAvailability(cwd string, execCommand gcExecFn, lookPath gcLookFn) (bool, string) {
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		return false, fmt.Sprintf("no city.toml found (walk up from %s)", cwd)
	}
	ready, reason := gcBridgeReady(cityPath, execCommand, lookPath)
	if ready {
		return true, "gc bridge ready"
	}
	return false, reason
}

func gcExecutorSelectionReason(prefix string, client rpiGasCityClient, degradedReason string) string {
	mode := "gc-cli-fallback"
	if client != nil {
		mode = "gc-api"
	}
	if strings.TrimSpace(degradedReason) != "" {
		return fmt.Sprintf("%s backend=%s degraded=%q", prefix, mode, degradedReason)
	}
	return fmt.Sprintf("%s backend=%s", prefix, mode)
}

// gcCityPathFromOpts extracts the city path from opts or discovers it from cwd.
func gcCityPathFromOpts(opts phasedEngineOptions) string {
	if p := strings.TrimSpace(opts.GCCityPath); p != "" {
		return p
	}
	return gcBridgeCityPath(opts.WorkingDir)
}
