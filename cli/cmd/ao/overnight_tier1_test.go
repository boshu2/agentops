// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestCollectRecentSessionJSONL_FindsRecentFiles(t *testing.T) {
	// Create a fake projects dir structure mirroring ~/.claude/projects/
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".claude", "projects", "test-project")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write a recent .jsonl file (>1000 bytes so it passes the size filter).
	recent := filepath.Join(projDir, "recent-session.jsonl")
	data := make([]byte, 2000)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(recent, data, 0644); err != nil {
		t.Fatalf("write recent: %v", err)
	}

	// Write a tiny .jsonl file that should be filtered out.
	tiny := filepath.Join(projDir, "tiny.jsonl")
	if err := os.WriteFile(tiny, []byte("small"), 0644); err != nil {
		t.Fatalf("write tiny: %v", err)
	}

	// Write a non-jsonl file that should be ignored.
	other := filepath.Join(projDir, "notes.md")
	if err := os.WriteFile(other, data, 0644); err != nil {
		t.Fatalf("write other: %v", err)
	}

	paths, err := collectRecentSessionJSONL("/unused-cwd")
	if err != nil {
		t.Fatalf("collectRecentSessionJSONL: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("want 1 path (recent-session.jsonl), got %d: %v", len(paths), paths)
	}
	if len(paths) == 1 && filepath.Base(paths[0]) != "recent-session.jsonl" {
		t.Errorf("wrong file: %s", paths[0])
	}
}

func TestCollectRecentSessionJSONL_NoClaude(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No .claude dir at all — should return nil, nil.
	paths, err := collectRecentSessionJSONL("/unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("want 0 paths, got %d", len(paths))
	}
}

func TestRunPostLoopTier1Forge_SkipsWhenKillSwitchSet(t *testing.T) {
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "1")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(context.TODO(), t.TempDir(), summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Errorf("kill switch should skip silently, got degraded: %v", summary.Degraded)
	}
}

func TestRunPostLoopTier1Forge_SkipsWhenNoModel(t *testing.T) {
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(context.TODO(), t.TempDir(), summary, overnightSettings{})
	// No model configured = skip silently (opt-in feature).
	if len(summary.Degraded) != 0 {
		t.Errorf("no model should skip silently, got degraded: %v", summary.Degraded)
	}
}

func TestRunPostLoopTier1Forge_SkipsModelWithoutLegacyEngine(t *testing.T) {
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "gemma4:e4b")
	t.Setenv("AGENTOPS_DREAM_CURATOR_ENGINE", "")
	t.Setenv(forgeLegacyLocalLLMEnv, "")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(context.TODO(), t.TempDir(), summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Errorf("model without legacy engine should skip silently, got degraded: %v", summary.Degraded)
	}
	if _, ok := summary.CloseLoop["tier1_forge"]; ok {
		t.Fatalf("tier1_forge should not run without explicit legacy config")
	}
}

func TestRunPostLoopTier1Forge_QueuesWhenWorkerConfigured(t *testing.T) {
	tmp := t.TempDir()
	workerDir := filepath.Join(tmp, "dream-worker")
	projDir := filepath.Join(tmp, ".claude", "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sourcePath := filepath.Join(projDir, "recent-session.jsonl")
	data := make([]byte, 2000)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(sourcePath, data, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(tmp, "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", workerDir)

	summary := &overnightSummary{}
	runPostLoopTier1Forge(context.TODO(), filepath.Join(tmp, "repo"), summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Fatalf("expected no degradation, got %v", summary.Degraded)
	}

	queueDir := filepath.Join(workerDir, "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("queue entries = %d, want 1", len(entries))
	}

	tier1, ok := summary.CloseLoop["tier1_forge"].(map[string]any)
	if !ok {
		t.Fatalf("tier1_forge summary = %#v, want map", summary.CloseLoop["tier1_forge"])
	}
	if tier1["mode"] != "dream-worker-queue" {
		t.Fatalf("mode = %#v, want dream-worker-queue", tier1["mode"])
	}
	if tier1["queued"] != 1 {
		t.Fatalf("queued = %#v, want 1", tier1["queued"])
	}
	if tier1["queue_dir"] != queueDir {
		t.Fatalf("queue_dir = %#v, want %q", tier1["queue_dir"], queueDir)
	}

	jobData, err := os.ReadFile(filepath.Join(queueDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	var job curatorJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		t.Fatalf("parse job: %v\n%s", err, string(jobData))
	}
	if job.Kind != "ingest-claude-session" {
		t.Fatalf("job kind = %q, want ingest-claude-session", job.Kind)
	}
	if job.Source == nil || job.Source.Path != sourcePath {
		t.Fatalf("job source = %+v, want path %q", job.Source, sourcePath)
	}
}

func TestRunPostLoopTier1Forge_QueuesDaemonWikiForgeWhenReady(t *testing.T) {
	fixture := setupTier1DaemonWikiForgeFixture(t)
	configureTier1DaemonWikiForgeEnv(t, fixture.tmp)

	summary := runTier1DaemonWikiForge(t, fixture.repo)
	tier1 := assertTier1DaemonWikiForgeSummary(t, summary)
	assertTier1DaemonWikiForgeAgentWorker(t, tier1)
	if fixture.activation.URL == "" {
		t.Fatal("activation URL should be set")
	}
	assertTier1DaemonWikiForgeLedger(t, fixture.repo, fixture.sourcePath)
}

type tier1DaemonWikiForgeFixture struct {
	tmp        string
	repo       string
	sourcePath string
	activation agentopsDaemonActivation
}

func setupTier1DaemonWikiForgeFixture(t *testing.T) tier1DaemonWikiForgeFixture {
	t.Helper()
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	sourcePath := writeTier1RecentSession(t, tmp)
	activation := startTier1DaemonWikiForgeDaemon(t, repo)
	return tier1DaemonWikiForgeFixture{
		tmp:        tmp,
		repo:       repo,
		sourcePath: sourcePath,
		activation: activation,
	}
}

func writeTier1RecentSession(t *testing.T, tmp string) string {
	t.Helper()
	projDir := filepath.Join(tmp, ".claude", "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	sourcePath := filepath.Join(projDir, "recent-session.jsonl")
	data := make([]byte, 2000)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(sourcePath, data, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	return sourcePath
}

func startTier1DaemonWikiForgeDaemon(t *testing.T, repo string) agentopsDaemonActivation {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, repo, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
	})
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	t.Cleanup(func() {
		cancel()
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("daemon serve returned unexpected error: %v", err)
		}
	})
	return activation
}

func configureTier1DaemonWikiForgeEnv(t *testing.T, tmp string) {
	t.Helper()
	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_DAEMON_TOKEN", "secret-token")
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(tmp, "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
}

func runTier1DaemonWikiForge(t *testing.T, repo string) *overnightSummary {
	t.Helper()
	summary := &overnightSummary{RunID: "dream-wiki-daemon", OutputDir: filepath.Join(repo, ".agents", "overnight", "dream-wiki-daemon")}
	runPostLoopTier1Forge(context.Background(), repo, summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Fatalf("expected no degradation, got %v", summary.Degraded)
	}
	return summary
}

func assertTier1DaemonWikiForgeSummary(t *testing.T, summary *overnightSummary) map[string]any {
	t.Helper()
	tier1, ok := summary.CloseLoop["tier1_forge"].(map[string]any)
	if !ok {
		t.Fatalf("tier1_forge summary = %#v", summary.CloseLoop["tier1_forge"])
	}
	if tier1["mode"] != "daemon-wiki-forge" || tier1["queued"] != 1 {
		t.Fatalf("tier1 summary = %#v", tier1)
	}
	if tier1["daemon_job_id"] != "job-wiki-dream-wiki-daemon" {
		t.Fatalf("daemon job id = %#v", tier1["daemon_job_id"])
	}
	return tier1
}

func assertTier1DaemonWikiForgeAgentWorker(t *testing.T, tier1 map[string]any) {
	t.Helper()
	agentWorker, ok := tier1["agent_worker"].(map[string]any)
	if !ok || agentWorker["provider"] != "gascity" || agentWorker["worker_kind"] != "codex" {
		t.Fatalf("agent_worker = %#v", tier1["agent_worker"])
	}
	if refs, ok := tier1["worker_session_refs"].([]string); !ok || len(refs) != 0 {
		t.Fatalf("worker_session_refs = %#v", tier1["worker_session_refs"])
	}
}

func assertTier1DaemonWikiForgeLedger(t *testing.T, repo, sourcePath string) {
	t.Helper()
	events, err := daemonpkg.NewStore(repo).ReadLedger()
	if err != nil {
		t.Fatalf("read daemon ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != daemonpkg.EventJobAccepted {
		t.Fatalf("ledger events = %#v", events)
	}
	payload, ok := events[0].Payload["job_payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload = %#v", events[0].Payload)
	}
	spec, err := daemonpkg.WikiForgeJobSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("WikiForgeJobSpecFromPayload: %v", err)
	}
	if spec.DreamRunID != "dream-wiki-daemon" || len(spec.SourcePaths) != 1 || spec.SourcePaths[0] != sourcePath {
		t.Fatalf("wiki forge spec = %#v", spec)
	}
	if spec.OutputDir != filepath.Join(repo, ".agents", "wiki", "sources") {
		t.Fatalf("output dir = %q", spec.OutputDir)
	}
}
