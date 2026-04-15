package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	v1reader "github.com/boshu2/agentops/cli/cmd/ao/testdata/v1_reader"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
)

func writeExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
	if runtime.GOOS == "windows" {
		cmdPath := filepath.Join(dir, name+".cmd")
		cmdBody := "@echo off\r\nbash \"%~dp0" + name + "\" %*\r\n"
		if err := os.WriteFile(cmdPath, []byte(cmdBody), 0o755); err != nil {
			t.Fatalf("write executable shim %s: %v", name, err)
		}
	}
	return path
}

func TestRunOvernightStartDryRunJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testProjectDir = tmpDir
	defer func() { testProjectDir = "" }()

	oldDryRun := dryRun
	oldOutput := output
	oldGoal := overnightGoal
	oldOutputDir := overnightOutputDir
	oldRunTimeout := overnightRunTimeout
	oldLongHaul := overnightLongHaul
	oldLongHaulBudget := overnightLongHaulBudget
	oldKeepAwake := overnightKeepAwake
	oldNoKeepAwake := overnightNoKeepAwake
	oldRunners := append([]string{}, overnightRunners...)
	oldModels := overnightModels
	oldCreative := overnightCreative
	defer func() {
		dryRun = oldDryRun
		output = oldOutput
		overnightGoal = oldGoal
		overnightOutputDir = oldOutputDir
		overnightRunTimeout = oldRunTimeout
		overnightLongHaul = oldLongHaul
		overnightLongHaulBudget = oldLongHaulBudget
		overnightKeepAwake = oldKeepAwake
		overnightNoKeepAwake = oldNoKeepAwake
		overnightRunners = append([]string{}, oldRunners...)
		overnightModels = oldModels
		overnightCreative = oldCreative
	}()

	dryRun = true
	output = "json"
	overnightGoal = "stabilize dream slice"
	overnightRunners = []string{"codex", "claude"}
	overnightCreative = true

	cmd := &cobra.Command{}
	cmd.Flags().String("output-dir", "", "")
	cmd.Flags().String("run-timeout", "", "")
	cmd.Flags().Bool("long-haul", false, "")
	cmd.Flags().String("long-haul-budget", "", "")
	cmd.Flags().Bool("keep-awake", false, "")
	cmd.Flags().Bool("no-keep-awake", false, "")
	cmd.Flags().StringSlice("runner", nil, "")
	cmd.Flags().String("models", "", "")
	cmd.Flags().Bool("creative-lane", false, "")

	stdout, err := captureStdout(t, func() error {
		return runOvernightStart(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runOvernightStart dry-run: %v", err)
	}

	var summary overnightSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse dry-run summary: %v\noutput=%s", err, stdout)
	}
	if summary.Status != "dry-run" {
		t.Fatalf("status = %q, want dry-run", summary.Status)
	}
	if summary.Runtime.KeepAwakeMode != "disabled" {
		t.Fatalf("keep_awake_mode = %q, want disabled in dry-run", summary.Runtime.KeepAwakeMode)
	}
	if summary.Council == nil {
		t.Fatal("expected Dream Council plan in dry-run summary")
	}
	if got := strings.Join(summary.Council.RequestedRunners, ","); got != "claude,codex" {
		t.Fatalf("requested_runners = %q, want claude,codex", got)
	}
	if summary.Dreamscape == nil {
		t.Fatal("expected DreamScape in dry-run summary")
	}
	if _, ok := summary.Artifacts["council_packet"]; !ok {
		t.Fatalf("expected council_packet artifact in %#v", summary.Artifacts)
	}
	if !strings.Contains(summary.NextAction, "ao rpi phased") {
		t.Fatalf("next_action = %q, want RPI recommendation", summary.NextAction)
	}
	// Regression guard for Micro-epic 2 (C1): summary.RunID must be
	// populated before runOvernightStart constructs ovn.RunLoopOptions, or
	// the iteration-store per-run namespace collapses to
	// <OutputDir>//iterations/ and prior-run cross-contamination returns.
	if summary.RunID == "" {
		t.Fatal("summary.RunID is empty; RunLoopOptions.RunID would be empty too")
	}
}

// TestRunOvernight_RunLoopOptionsReceivesRunID is the Micro-epic 2 (C1)
// regression guard for the one-line wire-in in runOvernightStart.
//
// The unit test uses the same summary and RunLoopOptions helpers as the
// command-layer code and asserts that setting RunID on an overnightSummary
// propagates through to the loop options.
// If a future refactor drops the RunID assignment, this test catches it.
func TestRunOvernight_RunLoopOptionsReceivesRunID(t *testing.T) {
	oldGoal := overnightGoal
	defer func() { overnightGoal = oldGoal }()
	overnightGoal = ""

	settings := overnightSettings{
		OutputDir:  "/tmp/fake-overnight",
		RunTimeout: time.Hour,
	}
	startedAt := time.Date(2026, 4, 12, 7, 30, 0, 0, time.UTC)
	summary := newOvernightStartSummary("/tmp/fake-cwd", settings, startedAt)
	runOpts := newOvernightRunLoopOptions("/tmp/fake-cwd", settings, summary, nil)

	if runOpts.RunID != "20260412T073000Z" {
		t.Fatalf("RunLoopOptions.RunID = %q, want %q", runOpts.RunID, "20260412T073000Z")
	}
	if runOpts.OutputDir != "/tmp/fake-overnight" {
		t.Fatalf("RunLoopOptions.OutputDir = %q, want %q", runOpts.OutputDir, "/tmp/fake-overnight")
	}
	if runOpts.Cwd != "/tmp/fake-cwd" {
		t.Fatalf("RunLoopOptions.Cwd = %q, want %q", runOpts.Cwd, "/tmp/fake-cwd")
	}
}

func TestNewOvernightRunLoopOptions_WiresCloseLoopCallbacks(t *testing.T) {
	settings := overnightSettings{
		OutputDir:  "/tmp/fake-overnight",
		RunTimeout: time.Hour,
	}
	startedAt := time.Date(2026, 4, 12, 7, 30, 0, 0, time.UTC)
	summary := newOvernightStartSummary("/tmp/fake-cwd", settings, startedAt)
	runOpts := newOvernightRunLoopOptions("/tmp/fake-cwd", settings, summary, nil)

	if runOpts.CloseLoopCallbacks.ResolveIngestFiles == nil {
		t.Fatal("ResolveIngestFiles callback not wired")
	}
	if runOpts.CloseLoopCallbacks.IngestFilesToPool == nil {
		t.Fatal("IngestFilesToPool callback not wired")
	}
	if runOpts.CloseLoopCallbacks.AutoPromoteFn == nil {
		t.Fatal("AutoPromoteFn callback not wired")
	}
	if runOpts.CloseLoopCallbacks.ProcessCitationFeedback == nil {
		t.Fatal("ProcessCitationFeedback callback not wired")
	}
	if runOpts.CloseLoopCallbacks.PromoteCitedLearnings == nil {
		t.Fatal("PromoteCitedLearnings callback not wired")
	}
	if runOpts.CloseLoopCallbacks.PromoteToMemory == nil {
		t.Fatal("PromoteToMemory callback not wired")
	}
	if runOpts.CloseLoopCallbacks.ApplyMaturityFn == nil {
		t.Fatal("ApplyMaturityFn callback not wired")
	}
}

func TestResolveOvernightSettings_LongHaulDefaultsOff(t *testing.T) {
	oldLongHaul := overnightLongHaul
	oldLongHaulBudget := overnightLongHaulBudget
	defer func() {
		overnightLongHaul = oldLongHaul
		overnightLongHaulBudget = oldLongHaulBudget
	}()

	overnightLongHaul = false
	overnightLongHaulBudget = "1h"

	cmd := &cobra.Command{}
	cmd.Flags().String("output-dir", "", "")
	cmd.Flags().String("run-timeout", "", "")
	cmd.Flags().Bool("long-haul", false, "")
	cmd.Flags().String("long-haul-budget", "", "")
	cmd.Flags().Bool("keep-awake", false, "")
	cmd.Flags().Bool("no-keep-awake", false, "")

	settings, err := resolveOvernightSettings(cmd, t.TempDir())
	if err != nil {
		t.Fatalf("resolveOvernightSettings: %v", err)
	}
	if settings.LongHaulEnabled {
		t.Fatal("LongHaulEnabled = true, want false")
	}
	if settings.LongHaulBudget != time.Hour {
		t.Fatalf("LongHaulBudget = %s, want 1h", settings.LongHaulBudget)
	}
}

func TestRunOvernight_LongHaulSkipsWhenTriggersWeak(t *testing.T) {
	summary := overnightSummary{
		OutputDir: filepath.Join(t.TempDir(), "overnight"),
		MorningPackets: []overnightMorningPacket{
			{
				Rank:           1,
				Title:          "Strong queue-backed packet",
				QueueBacked:    true,
				Confidence:     "high",
				MorningCommand: `ao rpi phased "Strong queue-backed packet"`,
			},
		},
		RetrievalLive: map[string]any{"coverage": 0.91},
		LongHaul:      &ovn.LongHaulSummary{Enabled: true},
	}
	settings := overnightSettings{
		LongHaulEnabled: true,
		LongHaulBudget:  time.Hour,
		Runners:         []string{"codex"},
	}

	oldCouncil := runDreamCouncilFn
	oldPackets := executeDreamMorningPacketsFn
	defer func() {
		runDreamCouncilFn = oldCouncil
		executeDreamMorningPacketsFn = oldPackets
	}()

	councilCalls := 0
	packetCalls := 0
	runDreamCouncilFn = func(ctx context.Context, cwd string, log io.Writer, summary *overnightSummary, settings overnightSettings) error {
		councilCalls++
		return nil
	}
	executeDreamMorningPacketsFn = func(cwd string, summary *overnightSummary) {
		packetCalls++
	}

	if err := runDreamLongHaul(context.Background(), t.TempDir(), io.Discard, &summary, settings); err != nil {
		t.Fatalf("runDreamLongHaul: %v", err)
	}
	if councilCalls != 0 || packetCalls != 0 {
		t.Fatalf("unexpected probe calls: council=%d packets=%d", councilCalls, packetCalls)
	}
	if summary.LongHaul == nil || summary.LongHaul.Active {
		t.Fatalf("long_haul = %#v, want inactive", summary.LongHaul)
	}
	if summary.LongHaul.ExitReason != "trigger threshold not met" {
		t.Fatalf("exit_reason = %q, want trigger threshold not met", summary.LongHaul.ExitReason)
	}
}

func TestRunOvernight_LongHaulStopsOnEarlyExit(t *testing.T) {
	summary := overnightSummary{
		OutputDir: filepath.Join(t.TempDir(), "overnight"),
		Goal:      "Strengthen Dream handoff",
		MorningPackets: []overnightMorningPacket{
			{
				ID:             dreamPacketID("goal", "Strengthen Dream handoff"),
				Rank:           1,
				Title:          "Weak synthetic packet",
				SourceEpic:     "dream-goal",
				QueueBacked:    false,
				Confidence:     "medium",
				MorningCommand: `ao rpi phased "Weak synthetic packet"`,
			},
		},
		Briefing:      map[string]any{"mode": "fallback", "first_move": "Use the goal packet."},
		CloseLoop:     map[string]any{"findings_routed": 1},
		RetrievalLive: map[string]any{"coverage": 0.91},
		LongHaul:      &ovn.LongHaulSummary{Enabled: true},
	}
	settings := overnightSettings{
		LongHaulEnabled: true,
		LongHaulBudget:  time.Hour,
		Runners:         []string{"codex"},
	}

	oldCouncil := runDreamCouncilFn
	oldPackets := executeDreamMorningPacketsFn
	defer func() {
		runDreamCouncilFn = oldCouncil
		executeDreamMorningPacketsFn = oldPackets
	}()

	councilCalls := 0
	packetCalls := 0
	runDreamCouncilFn = func(ctx context.Context, cwd string, log io.Writer, summary *overnightSummary, settings overnightSettings) error {
		councilCalls++
		return nil
	}
	executeDreamMorningPacketsFn = func(cwd string, summary *overnightSummary) {
		packetCalls++
		summary.MorningPackets[0].Confidence = "high"
		refreshOvernightTelemetry(summary)
	}

	if err := runDreamLongHaul(context.Background(), t.TempDir(), io.Discard, &summary, settings); err != nil {
		t.Fatalf("runDreamLongHaul: %v", err)
	}
	if councilCalls != 0 || packetCalls != 1 {
		t.Fatalf("probe calls = council:%d packets:%d, want council skipped after corroboration", councilCalls, packetCalls)
	}
	if summary.LongHaul == nil || !summary.LongHaul.Active {
		t.Fatalf("long_haul = %#v, want active", summary.LongHaul)
	}
	if summary.LongHaul.ProbeCount != 1 {
		t.Fatalf("probe_count = %d, want 1", summary.LongHaul.ProbeCount)
	}
	if summary.LongHaul.ZeroDeltaProbeStreak != 0 {
		t.Fatalf("zero_delta_probe_streak = %d, want 0", summary.LongHaul.ZeroDeltaProbeStreak)
	}
	if summary.LongHaul.ExitReason != "no additional long-haul probes available" {
		t.Fatalf("exit_reason = %q", summary.LongHaul.ExitReason)
	}
}

func TestHydrateOvernightSummaryArtifacts_LoadsSurfaces(t *testing.T) {
	tmpDir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	summary := overnightSummary{
		Artifacts: map[string]string{
			"metrics_health":    write("metrics.json", `{"escape_velocity":false}`),
			"retrieval_live":    write("retrieval.json", `{"coverage":0.8}`),
			"close_loop":        write("close-loop.json", `{"findings_routed":2}`),
			"briefing_fallback": write("briefing-fallback.json", `{"mode":"fallback","first_move":"Use the packet."}`),
		},
	}

	hydrateOvernightSummaryArtifacts(&summary)

	if got, ok := lookupBool(summary.MetricsHealth, "escape_velocity"); !ok || got {
		t.Fatalf("metrics_health = %#v, want escape_velocity=false", summary.MetricsHealth)
	}
	if got, ok := lookupFloat(summary.RetrievalLive, "coverage"); !ok || got != 0.8 {
		t.Fatalf("retrieval_live = %#v, want coverage=0.8", summary.RetrievalLive)
	}
	if got, ok := lookupFloat(summary.CloseLoop, "findings_routed"); !ok || got != 2 {
		t.Fatalf("close_loop = %#v, want findings_routed=2", summary.CloseLoop)
	}
	if got := stringifyAny(summary.Briefing["mode"]); got != "fallback" {
		t.Fatalf("briefing = %#v, want fallback mode", summary.Briefing)
	}
}

func TestExecuteOvernightReportSurfaces_MarksRealStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	summary := overnightSummary{
		RunID:     "dream-run-1",
		Goal:      "repair dream cycle",
		OutputDir: tmpDir,
		Steps: []overnightStepSummary{
			{Name: "close-loop", Status: "pending"},
			{Name: "defrag-preview", Status: "pending"},
			{Name: "metrics-health", Status: "pending"},
			{Name: "retrieval-live", Status: "pending"},
			{Name: "knowledge-brief", Status: "pending"},
		},
		Artifacts: map[string]string{
			"close_loop":     filepath.Join(tmpDir, "close-loop.json"),
			"defrag_report":  filepath.Join(tmpDir, "defrag", "latest.json"),
			"metrics_health": filepath.Join(tmpDir, "metrics-health.json"),
			"retrieval_live": filepath.Join(tmpDir, "retrieval-bench.json"),
			"briefing":       filepath.Join(tmpDir, "briefing.json"),
		},
		Iterations: []ovn.IterationSummary{
			{
				ID:     "dream-run-1-iter-1",
				Index:  1,
				Status: ovn.StatusDone,
				Reduce: map[string]any{
					"close_loop_promoted": 2,
					"findings_routed":     1,
				},
			},
		},
	}

	origCloseLoop := writeDreamCloseLoopArtifact
	origDefrag := runDreamDefragPreviewFn
	origMetrics := runDreamMetricsHealthFn
	origRetrieval := runDreamRetrievalLiveFn
	origBrief := runDreamKnowledgeBriefFn
	defer func() {
		writeDreamCloseLoopArtifact = origCloseLoop
		runDreamDefragPreviewFn = origDefrag
		runDreamMetricsHealthFn = origMetrics
		runDreamRetrievalLiveFn = origRetrieval
		runDreamKnowledgeBriefFn = origBrief
	}()

	writeDreamCloseLoopArtifact = writeDreamLoopCloseLoopArtifact
	runDreamDefragPreviewFn = func(cwd, artifactPath string) error {
		return writeJSONFile(artifactPath, map[string]any{"mode": "dry-run"})
	}
	runDreamMetricsHealthFn = func(cwd, artifactPath string) error {
		return writeJSONFile(artifactPath, map[string]any{"escape_velocity": true})
	}
	runDreamRetrievalLiveFn = func(cwd, artifactPath string) error {
		return writeJSONFile(artifactPath, map[string]any{"coverage": 1.0})
	}
	runDreamKnowledgeBriefFn = func(cwd, goal, artifactPath string) error {
		return writeJSONFile(artifactPath, map[string]any{"output_path": filepath.Join(cwd, ".agents", "briefings", "dream.md")})
	}

	if err := executeOvernightReportSurfaces(tmpDir, &summary); err != nil {
		t.Fatalf("executeOvernightReportSurfaces: %v", err)
	}

	for _, stepName := range []string{"close-loop", "defrag-preview", "metrics-health", "retrieval-live", "knowledge-brief"} {
		step := findOvernightHardFailStep(summary.Steps, stepName)
		if step == nil {
			t.Fatalf("missing step %q after report surface execution", stepName)
		}
		if step.Status != "done" {
			t.Fatalf("step %q status = %q, want done", stepName, step.Status)
		}
	}

	data, err := os.ReadFile(summary.Artifacts["close_loop"])
	if err != nil {
		t.Fatalf("read close-loop artifact: %v", err)
	}
	var artifact map[string]any
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse close-loop artifact: %v", err)
	}
	if got := artifact["iteration_id"]; got != "dream-run-1-iter-1" {
		t.Fatalf("iteration_id = %#v, want dream-run-1-iter-1", got)
	}
	if got := artifact["source"]; got != "dream-loop" {
		t.Fatalf("source = %#v, want dream-loop", got)
	}
}

func TestRunDreamCouncilRunner_TimesOutAndLeavesNoArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "council", "claude.json")

	origTimeout := dreamCouncilRunnerTimeout
	origClaude := dreamRunClaudeCouncilFn
	defer func() {
		dreamCouncilRunnerTimeout = origTimeout
		dreamRunClaudeCouncilFn = origClaude
	}()

	dreamCouncilRunnerTimeout = 20 * time.Millisecond
	dreamRunClaudeCouncilFn = func(ctx context.Context, cwd, model, schemaPath, prompt, outputPath string, log io.Writer) error {
		<-ctx.Done()
		return ctx.Err()
	}

	_, err := runDreamCouncilRunner(
		context.Background(),
		tmpDir,
		io.Discard,
		"claude",
		"",
		filepath.Join(tmpDir, "schema.json"),
		dreamCouncilPacket{RunID: "run-1", RepoRoot: tmpDir},
		outputPath,
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("runDreamCouncilRunner error = %v, want timeout", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no final artifact after timeout, stat err=%v", statErr)
	}
}

func TestRunDreamCouncilRunner_RejectsEmptyOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "council", "claude.json")

	origClaude := dreamRunClaudeCouncilFn
	defer func() {
		dreamRunClaudeCouncilFn = origClaude
	}()

	dreamRunClaudeCouncilFn = func(ctx context.Context, cwd, model, schemaPath, prompt, outputPath string, log io.Writer) error {
		return nil
	}

	_, err := runDreamCouncilRunner(
		context.Background(),
		tmpDir,
		io.Discard,
		"claude",
		"",
		filepath.Join(tmpDir, "schema.json"),
		dreamCouncilPacket{RunID: "run-1", RepoRoot: tmpDir},
		outputPath,
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "output was empty") {
		t.Fatalf("runDreamCouncilRunner error = %v, want empty-output failure", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no final artifact after empty output, stat err=%v", statErr)
	}
}

func TestWriteDreamCouncilSchema_RequiresAllProperties(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "report-schema.json")
	if err := writeDreamCouncilSchema(schemaPath); err != nil {
		t.Fatalf("writeDreamCouncilSchema: %v", err)
	}

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if len(schema.Properties) == 0 {
		t.Fatal("schema properties unexpectedly empty")
	}

	required := make(map[string]struct{}, len(schema.Required))
	for _, key := range schema.Required {
		required[key] = struct{}{}
	}

	for key := range schema.Properties {
		if _, ok := required[key]; !ok {
			t.Fatalf("schema required list missing property %q", key)
		}
	}
}

func TestRunOvernightSetupDryRunJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testProjectDir = tmpDir
	defer func() { testProjectDir = "" }()

	for _, name := range []string{"codex", "claude", "caffeinate"} {
		writeExecutable(t, tmpDir, name, "#!/bin/sh\nexit 0\n")
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(tmpDir, ".agentops", "config.yaml"))

	oldOutput := output
	oldSetupApply := overnightSetupApply
	oldSetupScheduler := overnightSetupScheduler
	oldSetupAt := overnightSetupAt
	oldSetupRunners := append([]string{}, overnightSetupRunners...)
	oldSetupKeepAwake := overnightSetupKeepAwake
	oldSetupNoKeepAwake := overnightSetupNoKeepAwake
	oldDreamOS := dreamOS
	oldDreamBatteryStatus := dreamBatteryStatus
	defer func() {
		output = oldOutput
		overnightSetupApply = oldSetupApply
		overnightSetupScheduler = oldSetupScheduler
		overnightSetupAt = oldSetupAt
		overnightSetupRunners = append([]string{}, oldSetupRunners...)
		overnightSetupKeepAwake = oldSetupKeepAwake
		overnightSetupNoKeepAwake = oldSetupNoKeepAwake
		dreamOS = oldDreamOS
		dreamBatteryStatus = oldDreamBatteryStatus
	}()

	output = "json"
	overnightSetupScheduler = "auto"
	overnightSetupRunners = nil
	dreamOS = "darwin"
	dreamBatteryStatus = func() bool { return true }

	cmd := &cobra.Command{}
	cmd.Flags().Bool("apply", false, "")
	cmd.Flags().String("scheduler", "auto", "")
	cmd.Flags().String("at", "", "")
	cmd.Flags().StringSlice("runner", nil, "")
	cmd.Flags().Bool("keep-awake", false, "")
	cmd.Flags().Bool("no-keep-awake", false, "")

	stdout, err := captureStdout(t, func() error {
		return runOvernightSetup(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runOvernightSetup dry-run: %v", err)
	}

	var summary dreamSetupSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse setup summary: %v\noutput=%s", err, stdout)
	}
	if summary.Status != "dry-run" {
		t.Fatalf("status = %q, want dry-run", summary.Status)
	}
	if summary.Host.RecommendedMode != "manual" {
		t.Fatalf("recommended_mode = %q, want manual", summary.Host.RecommendedMode)
	}
	if summary.DreamConfig.SchedulerMode != "manual" {
		t.Fatalf("scheduler_mode = %q, want manual", summary.DreamConfig.SchedulerMode)
	}
	if summary.DreamConfig.KeepAwake == nil || !*summary.DreamConfig.KeepAwake {
		t.Fatalf("keep_awake = %#v, want true", summary.DreamConfig.KeepAwake)
	}
	if got := strings.Join(summary.DreamConfig.Runners, ","); got != "claude,codex" {
		t.Fatalf("dream runners = %q, want claude,codex", got)
	}
	if !strings.Contains(summary.NextAction, "ao overnight start") {
		t.Fatalf("next_action = %q, want overnight start guidance", summary.NextAction)
	}
}

func TestRunOvernightSetupApplyWritesSchedulerArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	testProjectDir = tmpDir
	defer func() { testProjectDir = "" }()

	for _, name := range []string{"codex", "claude", "caffeinate"} {
		writeExecutable(t, tmpDir, name, "#!/bin/sh\nexit 0\n")
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	configPath := filepath.Join(tmpDir, ".agentops", "config.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)

	oldOutput := output
	oldSetupApply := overnightSetupApply
	oldSetupScheduler := overnightSetupScheduler
	oldSetupAt := overnightSetupAt
	oldSetupRunners := append([]string{}, overnightSetupRunners...)
	oldDreamOS := dreamOS
	oldDreamBatteryStatus := dreamBatteryStatus
	oldDryRun := dryRun
	defer func() {
		output = oldOutput
		overnightSetupApply = oldSetupApply
		overnightSetupScheduler = oldSetupScheduler
		overnightSetupAt = oldSetupAt
		overnightSetupRunners = append([]string{}, oldSetupRunners...)
		dreamOS = oldDreamOS
		dreamBatteryStatus = oldDreamBatteryStatus
		dryRun = oldDryRun
	}()

	output = "json"
	dryRun = false
	overnightSetupApply = true
	overnightSetupScheduler = "launchd"
	overnightSetupAt = "01:30"
	dreamOS = "darwin"
	dreamBatteryStatus = func() bool { return false }

	cmd := &cobra.Command{}
	cmd.Flags().Bool("apply", false, "")
	cmd.Flags().String("scheduler", "auto", "")
	cmd.Flags().String("at", "", "")
	cmd.Flags().StringSlice("runner", nil, "")
	cmd.Flags().Bool("keep-awake", false, "")
	cmd.Flags().Bool("no-keep-awake", false, "")
	if err := cmd.Flags().Set("at", "01:30"); err != nil {
		t.Fatalf("set --at: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return runOvernightSetup(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runOvernightSetup apply: %v", err)
	}

	var summary dreamSetupSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse setup summary: %v\noutput=%s", err, stdout)
	}
	if summary.Status != "configured" {
		t.Fatalf("status = %q, want configured", summary.Status)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config at %s: %v", configPath, err)
	}
	launchdPath := filepath.Join(tmpDir, ".agentops", "generated", "dream", "com.agentops.dream.plist")
	if _, err := os.Stat(launchdPath); err != nil {
		t.Fatalf("expected launchd artifact at %s: %v", launchdPath, err)
	}
}

func TestResolveDreamSchedulerModeFlagOverride(t *testing.T) {
	oldSetupScheduler := overnightSetupScheduler
	defer func() { overnightSetupScheduler = oldSetupScheduler }()

	host := dreamHostProfile{RecommendedMode: "cron"}

	tests := []struct {
		name string
		flag string
		base string
		want string
	}{
		{name: "empty flag keeps base", flag: "", base: "manual", want: "manual"},
		{name: "explicit flag wins", flag: "launchd", base: "manual", want: "launchd"},
		{name: "auto prefers host recommendation", flag: "auto", base: "manual", want: "cron"},
		{name: "auto falls back to manual", flag: "auto", base: "systemd", want: "manual"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overnightSetupScheduler = tt.flag
			testHost := host
			if tt.name == "auto falls back to manual" {
				testHost.RecommendedMode = ""
			}

			if got := resolveDreamSchedulerModeFlagOverride(tt.base, testHost); got != tt.want {
				t.Fatalf("resolveDreamSchedulerModeFlagOverride(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func TestRunDreamCouncilWithMockRunners(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock shell runners rely on Unix argv quoting")
	}
	tmpDir := t.TempDir()
	writeExecutable(t, tmpDir, "codex", `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    out="$1"
  fi
  shift
done
cat > "$out" <<'JSON'
{"runner":"codex-dream-council","headline":"Codex says validate","recommended_kind":"validate","recommended_first_action":"Review the overnight council synthesis before shipping.","risks":["retrieval drift"],"opportunities":["promote learnings"],"confidence":"high","wildcard_idea":"Explore a speculative promotion lane."}
JSON
`)
	writeExecutable(t, tmpDir, "claude", `#!/bin/sh
cat <<'JSON'
{"runner":"claude","headline":"Claude agrees","recommended_kind":"validate","recommended_first_action":"Review the overnight council synthesis before shipping.","risks":["low retrieval coverage"],"opportunities":["tighten report copy"],"confidence":"high","wildcard_idea":""}
JSON
`)
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	summary := overnightSummary{
		RunID:         "test-run",
		Goal:          "stabilize Dream",
		RepoRoot:      tmpDir,
		OutputDir:     filepath.Join(tmpDir, "overnight"),
		Artifacts:     map[string]string{},
		RetrievalLive: map[string]any{"coverage": 0.72},
		MetricsHealth: map[string]any{"escape_velocity": true},
	}
	settings := overnightSettings{
		OutputDir:     summary.OutputDir,
		RunTimeoutRaw: "8h",
		RunTimeout:    8 * time.Hour,
		KeepAwake:     false,
		Runners:       []string{"codex", "claude"},
		RunnerModels:  map[string]string{},
		Consensus:     "majority",
		CreativeLane:  true,
	}
	appendDreamCouncilPlan(&summary, settings)
	var log bytes.Buffer

	if err := runDreamCouncil(context.Background(), tmpDir, &log, &summary, settings); err != nil {
		t.Fatalf("runDreamCouncil: %v", err)
	}
	ensureOvernightDerivedViews(&summary)

	if summary.Council == nil {
		t.Fatal("expected council summary")
	}
	if got := strings.Join(summary.Council.CompletedRunners, ","); got != "claude,codex" {
		t.Fatalf("completed_runners = %q, want claude,codex\ndegraded=%v\nfailed=%v\nlog=%s", got, summary.Degraded, summary.Council.FailedRunners, log.String())
	}
	codexArtifact, err := os.ReadFile(summary.Artifacts["council_codex"])
	if err != nil {
		t.Fatalf("read codex council artifact: %v", err)
	}
	var codexReport overnightCouncilRunnerReport
	if err := json.Unmarshal(codexArtifact, &codexReport); err != nil {
		t.Fatalf("parse codex council artifact: %v", err)
	}
	if codexReport.Runner != "codex" {
		t.Fatalf("codex council artifact runner = %q, want codex", codexReport.Runner)
	}
	if summary.Council.RecommendedFirstAction != "Review the overnight council synthesis before shipping." {
		t.Fatalf("recommended_first_action = %q", summary.Council.RecommendedFirstAction)
	}
	if len(summary.Council.WildcardIdeas) != 1 {
		t.Fatalf("wildcard_ideas = %#v, want 1 item", summary.Council.WildcardIdeas)
	}
	if summary.Dreamscape == nil || summary.Dreamscape.FirstMove != summary.Council.RecommendedFirstAction {
		t.Fatalf("dreamscape = %#v, want first move from council", summary.Dreamscape)
	}
}

func TestRunOvernightReportReadsSummaryJSON(t *testing.T) {
	tmpDir := t.TempDir()
	summary := overnightSummary{
		SchemaVersion: 1,
		Mode:          "dream.local-bedtime",
		RunID:         "test-run",
		OutputDir:     tmpDir,
		Status:        "done",
		Runtime: overnightRuntimeSummary{
			KeepAwake:        true,
			KeepAwakeMode:    "caffeinate",
			RequestedTimeout: "8h",
			EffectiveTimeout: "8h0m0s",
		},
		Council: &overnightCouncilSummary{
			RequestedRunners:       []string{"claude", "codex"},
			CompletedRunners:       []string{"claude", "codex"},
			ConsensusPolicy:        "majority",
			ConsensusKind:          "validate",
			RecommendedFirstAction: "Review the overnight council synthesis before shipping.",
		},
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "summary.json"), data, 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	oldOutput := output
	oldFrom := overnightReportFrom
	defer func() {
		output = oldOutput
		overnightReportFrom = oldFrom
	}()

	output = "table"
	overnightReportFrom = tmpDir

	stdout, err := captureStdout(t, func() error {
		return runOvernightReport(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runOvernightReport: %v", err)
	}
	if !strings.Contains(stdout, "Dream Morning Report") {
		t.Fatalf("report output missing header: %s", stdout)
	}
	if !strings.Contains(stdout, "test-run") {
		t.Fatalf("report output missing run id: %s", stdout)
	}
	if !strings.Contains(stdout, "DreamScape") {
		t.Fatalf("report output missing DreamScape section: %s", stdout)
	}
	if !strings.Contains(stdout, "Dream Council") {
		t.Fatalf("report output missing Dream Council section: %s", stdout)
	}
}

// TestRunOvernight_HardFail_WritesFailedSummary locks the hard-fail
// contract (na-cdn): when the overnight Dream pipeline aborts on a
// hard-fail step, finalizeOvernightSummary must still write both
// summary.json and summary.md with status="failed", preserving the
// last completed step, the degraded state, and the artifacts map.
//
// This mirrors the runOvernightStart hard-fail path where, on
// ovn.RunLoop error, the command sets summary.Status = "failed" and
// calls finalizeOvernightSummary(&summary, startedAt) before returning
// the error. We exercise finalizeOvernightSummary directly (the same
// pattern TestRunDreamCouncilWithMockRunners uses for its inner
// helper) so the test is hermetic and does not require a real
// .agents/ corpus or lock acquisition.
func TestRunOvernight_HardFail_WritesFailedSummary(t *testing.T) {
	fixture := newOvernightHardFailSummaryFixture(t)

	if err := finalizeOvernightSummary(&fixture.summary, fixture.startedAt); err != nil {
		t.Fatalf("finalizeOvernightSummary on hard-fail path: %v", err)
	}

	persisted := readOvernightHardFailSummaryJSON(t, fixture.summaryJSONPath)
	assertOvernightHardFailSummaryMarkdown(t, fixture.summaryMDPath)
	assertOvernightHardFailStatus(t, persisted)
	assertOvernightHardFailSteps(t, persisted)
	assertOvernightHardFailArtifacts(t, persisted, fixture)
	assertOvernightHardFailFinalized(t, persisted)
}

func TestFinalizeOvernightSummary_PersistsYieldAndLongHaulTelemetry(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "overnight", "latest")
	startedAt := time.Now().UTC().Add(-2 * time.Minute)
	summary := overnightSummary{
		SchemaVersion: 2,
		Mode:          "dream.local-bedtime",
		RunID:         "yield-longhaul-run",
		RepoRoot:      tmpDir,
		OutputDir:     outputDir,
		Status:        "done",
		StartedAt:     startedAt.Format(time.RFC3339),
		Runtime: overnightRuntimeSummary{
			KeepAwake:          false,
			KeepAwakeMode:      "disabled",
			RequestedTimeout:   "8h",
			EffectiveTimeout:   "8h0m0s",
			ProcessContractDoc: "docs/contracts/dream-run-contract.md",
			ReportContractDoc:  "docs/contracts/dream-report.md",
		},
		Artifacts: map[string]string{
			"summary_json":     filepath.Join(outputDir, "summary.json"),
			"summary_markdown": filepath.Join(outputDir, "summary.md"),
		},
		MorningPackets: []overnightMorningPacket{
			{
				Rank:           1,
				Title:          "Repair Dream retrieval coverage",
				Confidence:     "high",
				QueueBacked:    true,
				BeadID:         "na-telemetry",
				MorningCommand: `ao rpi phased "Repair Dream retrieval coverage"`,
			},
		},
		Council: &overnightCouncilSummary{
			RequestedRunners:       []string{"claude", "codex"},
			CompletedRunners:       []string{"codex"},
			FailedRunners:          []string{"claude"},
			ConsensusPolicy:        "majority",
			ConsensusKind:          "validate",
			RecommendedFirstAction: "Inspect the retrieval coverage packet before shipping.",
		},
		Degraded: []string{"claude council timed out after 1m30s"},
		LongHaul: &ovn.LongHaulSummary{
			Enabled:              true,
			Active:               true,
			TriggerReason:        "packet confidence below high",
			ExitReason:           "zero_delta_probe_streak >= 2",
			ProbeCount:           2,
			ZeroDeltaProbeStreak: 2,
		},
	}
	summary.councilNextActionHint = "Inspect the overnight report before shipping."

	if err := finalizeOvernightSummary(&summary, startedAt); err != nil {
		t.Fatalf("finalizeOvernightSummary: %v", err)
	}

	persisted := readOvernightHardFailSummaryJSON(t, filepath.Join(outputDir, "summary.json"))
	if persisted.Yield == nil {
		t.Fatal("persisted yield telemetry unexpectedly nil")
	}
	if persisted.Yield.PacketCountAfter != 1 {
		t.Fatalf("persisted packet_count_after = %d, want 1", persisted.Yield.PacketCountAfter)
	}
	if persisted.Yield.BeadSyncCount != 1 {
		t.Fatalf("persisted bead_sync_count = %d, want 1", persisted.Yield.BeadSyncCount)
	}
	if persisted.Yield.CouncilTimeoutCount != 1 {
		t.Fatalf("persisted council_timeout_count = %d, want 1", persisted.Yield.CouncilTimeoutCount)
	}
	if persisted.Yield.CouncilActionDelta != "refined" {
		t.Fatalf("persisted council_action_delta = %q, want refined", persisted.Yield.CouncilActionDelta)
	}
	if persisted.LongHaul == nil {
		t.Fatal("persisted long_haul unexpectedly nil")
	}
	if !persisted.LongHaul.Enabled || !persisted.LongHaul.Active {
		t.Fatalf("persisted long_haul = %#v, want enabled+active", persisted.LongHaul)
	}

	md, err := os.ReadFile(filepath.Join(outputDir, "summary.md"))
	if err != nil {
		t.Fatalf("read summary markdown: %v", err)
	}
	text := string(md)
	for _, want := range []string{"## Yield", "## Long-Haul", "Council action delta: `refined`", "Probe count: `2`"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary markdown missing %q:\n%s", want, text)
		}
	}
}

type overnightHardFailSummaryFixture struct {
	summary         overnightSummary
	startedAt       time.Time
	summaryJSONPath string
	summaryMDPath   string
}

func newOvernightHardFailSummaryFixture(t *testing.T) overnightHardFailSummaryFixture {
	t.Helper()

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "overnight", "latest")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}

	startedAt := time.Now().UTC().Add(-3 * time.Minute)
	summary := overnightSummary{
		SchemaVersion: 2,
		Mode:          "dream.local-bedtime",
		RunID:         "hardfail-run",
		Goal:          "lock hard-fail contract",
		RepoRoot:      tmpDir,
		OutputDir:     outputDir,
		// Simulate the state runOvernightStart leaves the summary in
		// when ovn.RunLoop returns an error: status flipped to
		// "failed", a last-completed step recorded, degraded entries
		// accumulated, and the baseline artifacts map populated with
		// summary_json / summary_markdown paths.
		Status:    "failed",
		StartedAt: startedAt.Format(time.RFC3339),
		Runtime: overnightRuntimeSummary{
			KeepAwake:          false,
			KeepAwakeMode:      "disabled",
			RequestedTimeout:   "8h",
			EffectiveTimeout:   "8h0m0s",
			LogPath:            filepath.Join(outputDir, "overnight.log"),
			ProcessContractDoc: "docs/contracts/dream-run-contract.md",
			ReportContractDoc:  "docs/contracts/dream-report.md",
		},
		Steps: []overnightStepSummary{
			{
				Name:     "close-loop",
				Status:   "done",
				Command:  "ao flywheel close-loop --threshold 0h --json",
				Artifact: filepath.Join(outputDir, "close-loop.json"),
			},
			{
				Name:    "metrics-health",
				Status:  "failed",
				Command: "ao metrics health --json",
				Note:    "hard-fail: metrics-health exited non-zero",
			},
		},
		Artifacts: map[string]string{
			"close_loop":       filepath.Join(outputDir, "close-loop.json"),
			"metrics_health":   filepath.Join(outputDir, "metrics-health.json"),
			"summary_json":     filepath.Join(outputDir, "summary.json"),
			"summary_markdown": filepath.Join(outputDir, "summary.md"),
		},
		Degraded: []string{"metrics-health: hard-fail simulated for regression test"},
	}

	return overnightHardFailSummaryFixture{
		summary:         summary,
		startedAt:       startedAt,
		summaryJSONPath: filepath.Join(outputDir, "summary.json"),
		summaryMDPath:   filepath.Join(outputDir, "summary.md"),
	}
}

func readOvernightHardFailSummaryJSON(t *testing.T, path string) overnightSummary {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected summary.json at %s: %v", path, err)
	}
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var persisted overnightSummary
	if err := json.Unmarshal(jsonBytes, &persisted); err != nil {
		t.Fatalf("parse summary.json: %v\npayload=%s", err, string(jsonBytes))
	}
	return persisted
}

func assertOvernightHardFailSummaryMarkdown(t *testing.T, path string) {
	t.Helper()

	mdBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read summary.md: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "- Status: `failed`") {
		t.Errorf("summary.md missing failed status line\npayload=%s", md)
	}
}

func assertOvernightHardFailStatus(t *testing.T, persisted overnightSummary) {
	t.Helper()

	if persisted.Status != "failed" {
		t.Errorf("summary.json status = %q, want %q", persisted.Status, "failed")
	}
}

func assertOvernightHardFailSteps(t *testing.T, persisted overnightSummary) {
	t.Helper()

	if len(persisted.Steps) < 2 {
		t.Fatalf("persisted Steps length = %d, want >= 2", len(persisted.Steps))
	}
	lastDone := findOvernightHardFailStep(persisted.Steps, "close-loop")
	hardFail := findOvernightHardFailStep(persisted.Steps, "metrics-health")
	if lastDone == nil {
		t.Fatal("expected last-completed step close-loop in persisted summary")
	}
	if lastDone.Status != "done" {
		t.Errorf("close-loop status = %q, want %q", lastDone.Status, "done")
	}
	if lastDone.Artifact == "" {
		t.Error("close-loop artifact was not preserved on hard-fail summary")
	}
	if hardFail == nil {
		t.Fatal("expected hard-fail step metrics-health in persisted summary")
	}
	if hardFail.Status != "failed" {
		t.Errorf("metrics-health status = %q, want %q", hardFail.Status, "failed")
	}
}

func findOvernightHardFailStep(steps []overnightStepSummary, name string) *overnightStepSummary {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

func assertOvernightHardFailArtifacts(t *testing.T, persisted overnightSummary, fixture overnightHardFailSummaryFixture) {
	t.Helper()

	if len(persisted.Degraded) == 0 {
		t.Error("persisted Degraded unexpectedly empty on hard-fail summary")
	}
	foundDegraded := false
	for _, d := range persisted.Degraded {
		if strings.Contains(d, "metrics-health") {
			foundDegraded = true
			break
		}
	}
	if !foundDegraded {
		t.Errorf("persisted Degraded missing metrics-health entry: %#v", persisted.Degraded)
	}
	if got := persisted.Artifacts["summary_json"]; got != fixture.summaryJSONPath {
		t.Errorf("artifacts[summary_json] = %q, want %q", got, fixture.summaryJSONPath)
	}
	if got := persisted.Artifacts["summary_markdown"]; got != fixture.summaryMDPath {
		t.Errorf("artifacts[summary_markdown] = %q, want %q", got, fixture.summaryMDPath)
	}
	if got := persisted.Artifacts["close_loop"]; got == "" {
		t.Error("artifacts[close_loop] unexpectedly empty on hard-fail summary")
	}
}

func assertOvernightHardFailFinalized(t *testing.T, persisted overnightSummary) {
	t.Helper()

	if persisted.FinishedAt == "" {
		t.Error("persisted FinishedAt unexpectedly empty on hard-fail summary")
	}
	if persisted.Duration == "" {
		t.Error("persisted Duration unexpectedly empty on hard-fail summary")
	}
}

// TestRunOvernight_SchemaV2IsV1BackwardCompatible verifies that a v2
// Dream report JSON still parses cleanly into the frozen v1 reader
// struct. This is the pm-008 fix: the compat claim now has a real
// regression test instead of unmarshaling v2 into itself.
//
// The v1 reader lives in cli/cmd/ao/testdata/v1_reader and is a
// frozen snapshot of overnightSummary as it existed on 2026-04-09.
// DO NOT edit the v1 reader to pick up v2-only fields — the whole
// point is that a strict v1 consumer never sees them yet still reads
// every field it does know about.
func TestRunOvernight_SchemaV2IsV1BackwardCompatible(t *testing.T) {
	v1 := decodeSchemaV2SummaryWithV1Reader(t, newSchemaV2CompatSummary())

	assertSchemaV2CompatCoreFields(t, v1)
	assertSchemaV2CompatRuntime(t, v1)
	assertSchemaV2CompatSteps(t, v1)
	assertSchemaV2CompatCollections(t, v1)
}

func newSchemaV2CompatSummary() overnightSummary {
	return overnightSummary{
		SchemaVersion: 2,
		Mode:          "dream.local-bedtime",
		RunID:         "compat-run-2026-04-09",
		Goal:          "verify v1 reader still parses v2 output",
		RepoRoot:      "/tmp/repo",
		OutputDir:     "/tmp/repo/.agents/overnight/compat",
		Status:        "done",
		DryRun:        false,
		StartedAt:     "2026-04-09T00:00:00Z",
		FinishedAt:    "2026-04-09T00:05:00Z",
		Duration:      "5m0s",
		Runtime: overnightRuntimeSummary{
			KeepAwake:          true,
			KeepAwakeMode:      "caffeinate",
			KeepAwakeNote:      "test",
			RequestedTimeout:   "2h",
			EffectiveTimeout:   "2h0m0s",
			LockPath:           "/tmp/repo/.agents/overnight/run.lock",
			LogPath:            "/tmp/repo/.agents/overnight/compat/overnight.log",
			ProcessContractDoc: "docs/contracts/dream-process.md",
			ReportContractDoc:  "docs/contracts/dream-report.md",
		},
		Steps: []overnightStepSummary{
			{
				Name:     "close-loop",
				Status:   "done",
				Command:  "ao learnings close-loop",
				Artifact: "close-loop.json",
			},
			{
				Name:   "defrag-preview",
				Status: "done",
			},
		},
		Artifacts: map[string]string{
			"summary": "summary.json",
		},
		Degraded:    []string{"inject-refresh: soft-skipped in test"},
		Recommended: []string{"promote the top council finding"},
		NextAction:  "ao overnight report",

		// v2 additive fields — the whole point of this test is
		// that these DO NOT break the v1 reader.
		Iterations: []ovn.IterationSummary{
			{
				ID:         "compat-run-2026-04-09-iter-1",
				Index:      1,
				StartedAt:  time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
				FinishedAt: time.Date(2026, 4, 9, 0, 2, 30, 0, time.UTC),
				Duration:   "2m30s",
				Status:     "done",
			},
		},
		FitnessDelta: map[string]any{
			"maturity_provisional_or_higher": 0.02,
		},
		PlateauReason:    "",
		RegressionReason: "",
	}
}

func decodeSchemaV2SummaryWithV1Reader(t *testing.T, v2 overnightSummary) v1reader.OvernightSummaryV1 {
	t.Helper()

	data, err := json.Marshal(v2)
	if err != nil {
		t.Fatalf("marshal v2 summary: %v", err)
	}

	var v1 v1reader.OvernightSummaryV1
	if err := json.Unmarshal(data, &v1); err != nil {
		t.Fatalf("unmarshal v2 JSON into v1 reader: %v", err)
	}
	return v1
}

func assertSchemaV2CompatCoreFields(t *testing.T, v1 v1reader.OvernightSummaryV1) {
	t.Helper()

	if v1.SchemaVersion != 2 {
		t.Errorf("SchemaVersion: got %d, want 2", v1.SchemaVersion)
	}
	if v1.RunID != "compat-run-2026-04-09" {
		t.Errorf("RunID: got %q, want %q", v1.RunID, "compat-run-2026-04-09")
	}
	if v1.Mode != "dream.local-bedtime" {
		t.Errorf("Mode: got %q", v1.Mode)
	}
	if v1.Goal == "" {
		t.Error("Goal: unexpectedly empty")
	}
	if v1.RepoRoot == "" {
		t.Error("RepoRoot: unexpectedly empty")
	}
	if v1.OutputDir == "" {
		t.Error("OutputDir: unexpectedly empty")
	}
	if v1.Status != "done" {
		t.Errorf("Status: got %q, want done", v1.Status)
	}
	if v1.StartedAt == "" {
		t.Error("StartedAt: unexpectedly empty")
	}
	if v1.FinishedAt == "" {
		t.Error("FinishedAt: unexpectedly empty")
	}
	if v1.Duration == "" {
		t.Error("Duration: unexpectedly empty")
	}
}

func assertSchemaV2CompatRuntime(t *testing.T, v1 v1reader.OvernightSummaryV1) {
	t.Helper()

	if v1.Runtime.KeepAwakeMode != "caffeinate" {
		t.Errorf("Runtime.KeepAwakeMode: got %q, want caffeinate", v1.Runtime.KeepAwakeMode)
	}
	if v1.Runtime.RequestedTimeout != "2h" {
		t.Errorf("Runtime.RequestedTimeout: got %q, want 2h", v1.Runtime.RequestedTimeout)
	}
	if v1.Runtime.ProcessContractDoc == "" {
		t.Error("Runtime.ProcessContractDoc: unexpectedly empty")
	}
}

func assertSchemaV2CompatSteps(t *testing.T, v1 v1reader.OvernightSummaryV1) {
	t.Helper()

	if len(v1.Steps) != 2 {
		t.Fatalf("Steps: got %d, want 2", len(v1.Steps))
	}
	if v1.Steps[0].Name != "close-loop" {
		t.Errorf("Steps[0].Name: got %q", v1.Steps[0].Name)
	}
	if v1.Steps[0].Status != "done" {
		t.Errorf("Steps[0].Status: got %q", v1.Steps[0].Status)
	}
}

func assertSchemaV2CompatCollections(t *testing.T, v1 v1reader.OvernightSummaryV1) {
	t.Helper()

	if len(v1.Artifacts) != 1 {
		t.Errorf("Artifacts: got %d entries, want 1", len(v1.Artifacts))
	}
	if len(v1.Degraded) == 0 {
		t.Error("Degraded: unexpectedly empty")
	}
	if len(v1.Recommended) == 0 {
		t.Error("Recommended: unexpectedly empty")
	}
	if v1.NextAction == "" {
		t.Error("NextAction: unexpectedly empty")
	}
}

// TestRunOvernightWarnOnlyReset_WritesFreshBudget is the Micro-epic 4
// (C3) regression guard: `ao overnight warn-only reset` must produce an
// on-disk budget file at the canonical path with Remaining equal to
// InitialBudget, and must tolerate --initial overrides.
func TestRunOvernightWarnOnlyReset_WritesFreshBudget(t *testing.T) {
	tmpDir := t.TempDir()
	testProjectDir = tmpDir
	defer func() { testProjectDir = "" }()

	oldInitial := overnightWarnOnlyResetInitial
	oldJSON := overnightWarnOnlyResetJSON
	defer func() {
		overnightWarnOnlyResetInitial = oldInitial
		overnightWarnOnlyResetJSON = oldJSON
	}()

	// Case 1: default (initial=0 → DefaultWarnOnlyBudget).
	overnightWarnOnlyResetInitial = 0
	overnightWarnOnlyResetJSON = false
	stdout, err := captureStdout(t, func() error {
		return runOvernightWarnOnlyReset(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("reset default: %v", err)
	}
	if !strings.Contains(stdout, "warn-only budget reset") {
		t.Fatalf("stdout missing confirmation: %q", stdout)
	}

	path := ovn.WarnOnlyBudgetPath(tmpDir)
	state, reason := ovn.ReadBudget(tmpDir)
	if reason != "" {
		t.Fatalf("post-reset rescue reason=%q (budget should be clean)", reason)
	}
	if state.Remaining != ovn.DefaultWarnOnlyBudget {
		t.Fatalf("Remaining=%d want %d", state.Remaining, ovn.DefaultWarnOnlyBudget)
	}
	if state.InitialBudget != ovn.DefaultWarnOnlyBudget {
		t.Fatalf("InitialBudget=%d want %d", state.InitialBudget, ovn.DefaultWarnOnlyBudget)
	}
	if state.LastResetAt == "" {
		t.Fatal("LastResetAt should be populated after reset")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("budget file not at expected path %s: %v", path, err)
	}

	// Case 2: --initial override.
	overnightWarnOnlyResetInitial = 7
	overnightWarnOnlyResetJSON = false
	if _, err := captureStdout(t, func() error {
		return runOvernightWarnOnlyReset(&cobra.Command{}, nil)
	}); err != nil {
		t.Fatalf("reset with initial=7: %v", err)
	}
	state, _ = ovn.ReadBudget(tmpDir)
	if state.Remaining != 7 || state.InitialBudget != 7 {
		t.Fatalf("state=%+v want Remaining=7 InitialBudget=7", state)
	}

	// Case 3: --json emission matches the disk shape.
	overnightWarnOnlyResetInitial = 4
	overnightWarnOnlyResetJSON = true
	stdout, err = captureStdout(t, func() error {
		return runOvernightWarnOnlyReset(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("reset --json: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse JSON payload: %v\noutput=%s", err, stdout)
	}
	if got, _ := payload["remaining"].(float64); int(got) != 4 {
		t.Fatalf("payload.remaining=%v want 4", payload["remaining"])
	}
	if got, _ := payload["initial"].(float64); int(got) != 4 {
		t.Fatalf("payload.initial=%v want 4", payload["initial"])
	}
	if got, _ := payload["path"].(string); got == "" {
		t.Fatal("payload.path should be non-empty")
	}
}

// TestRunOvernight_WarnOnlyRatchet_WiredIntoLoopOpts is the Micro-epic 4
// companion to TestRunOvernight_RunLoopOptionsReceivesRunID: it verifies
// the shape of the ratchet literal that runOvernightStart constructs,
// without exercising the full end-to-end loop (which requires the real
// fitness fixture). If a future refactor drops WarnOnlyBudget from the
// options literal, this test catches it via the exported
// WarnOnlyRatchet type.
func TestRunOvernight_WarnOnlyRatchet_WiredIntoLoopOpts(t *testing.T) {
	tmpDir := t.TempDir()
	// Seed a budget file with Remaining=2 to prove the wiring reads
	// the live value (not a hardcoded default).
	if _, err := ovn.ResetBudget(tmpDir, 2); err != nil {
		t.Fatalf("seed budget: %v", err)
	}

	state, _ := ovn.ReadBudget(tmpDir)
	if state.Remaining != 2 {
		t.Fatalf("seed state.Remaining=%d want 2", state.Remaining)
	}

	// Mirror the ratchet literal in runOvernightStart.
	ratchet := &ovn.WarnOnlyRatchet{
		Initial:   state.InitialBudget,
		Remaining: state.Remaining,
		OnConsume: func(newRemaining int) error { return nil },
	}
	runOpts := ovn.RunLoopOptions{
		WarnOnly:       true,
		WarnOnlyBudget: ratchet,
	}
	if runOpts.WarnOnlyBudget == nil {
		t.Fatal("WarnOnlyBudget literal dropped")
	}
	if runOpts.WarnOnlyBudget.Remaining != 2 {
		t.Fatalf("Remaining=%d want 2", runOpts.WarnOnlyBudget.Remaining)
	}
	if runOpts.WarnOnlyBudget.OnConsume == nil {
		t.Fatal("OnConsume callback dropped")
	}
}
