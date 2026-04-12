package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
)

func TestResolveDreamLocalCuratorConfigDetectsOllamaModel(t *testing.T) {
	server := fakeOllamaServer(t, []string{"gemma4:e4b", "gemma2:9b"})
	workerDir := t.TempDir()
	vaultDir := t.TempDir()

	enabled := true
	got := resolveDreamLocalCuratorConfig(config.DreamLocalCuratorConfig{
		Enabled:   &enabled,
		OllamaURL: server.URL,
		WorkerDir: workerDir,
		VaultDir:  vaultDir,
	}, time.Second)

	if got.Engine != "ollama" {
		t.Fatalf("Engine = %q, want ollama", got.Engine)
	}
	if got.Model != "gemma4:e4b" {
		t.Fatalf("Model = %q, want gemma4:e4b", got.Model)
	}
	if got.HourlyCap != 20 {
		t.Fatalf("HourlyCap = %d, want 20", got.HourlyCap)
	}
	if got.AllowedJobKinds == nil || strings.Join(got.AllowedJobKinds, ",") != "ingest-claude-session,lint-wiki,dream-seed" {
		t.Fatalf("AllowedJobKinds = %#v", got.AllowedJobKinds)
	}
}

func TestRunCuratorEnqueueWritesValidatedJob(t *testing.T) {
	server := fakeOllamaServer(t, []string{"gemma4:e4b"})
	workerDir := t.TempDir()
	vaultDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), ".agentops", "config.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)
	if err := config.Save(&config.Config{Dream: config.DreamConfig{
		LocalCurator: config.DreamLocalCuratorConfig{
			Enabled:         testBoolPtr(true),
			Engine:          "ollama",
			OllamaURL:       server.URL,
			Model:           "gemma4:e4b",
			WorkerDir:       workerDir,
			VaultDir:        vaultDir,
			HourlyCap:       20,
			AllowedJobKinds: []string{"ingest-claude-session", "lint-wiki", "dream-seed"},
		},
	}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	oldKind := curatorEnqueueKind
	oldSource := curatorEnqueueSource
	oldStart := curatorEnqueueChunkStart
	oldEnd := curatorEnqueueChunkEnd
	oldDryRun := dryRun
	oldOutput := output
	defer func() {
		curatorEnqueueKind = oldKind
		curatorEnqueueSource = oldSource
		curatorEnqueueChunkStart = oldStart
		curatorEnqueueChunkEnd = oldEnd
		dryRun = oldDryRun
		output = oldOutput
	}()

	dryRun = false
	output = "json"
	curatorEnqueueKind = "ingest-claude-session"
	curatorEnqueueSource = filepath.Join(vaultDir, "session.jsonl")
	curatorEnqueueChunkStart = 0
	curatorEnqueueChunkEnd = 10

	if _, err := captureStdout(t, func() error { return runCuratorEnqueue(&cobra.Command{}, nil) }); err != nil {
		t.Fatalf("runCuratorEnqueue: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(workerDir, "queue"))
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("queue entries = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(workerDir, "queue", entries[0].Name()))
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	var job curatorJob
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatalf("parse job: %v", err)
	}
	if job.Kind != "ingest-claude-session" || job.Source == nil || job.Source.ChunkEnd != 10 {
		t.Fatalf("job = %#v", job)
	}
}

func TestMaybeWriteDreamSchedulerArtifactsWindowsTaskScheduler(t *testing.T) {
	cwd := t.TempDir()
	generated, warnings, err := maybeWriteDreamSchedulerArtifacts(cwd, dreamHostProfile{OS: "windows"}, config.DreamConfig{
		SchedulerMode: "task-scheduler",
		ScheduleAt:    "02:15",
	})
	if err != nil {
		t.Fatalf("maybeWriteDreamSchedulerArtifacts: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	path := generated["task_scheduler"]
	if path == "" {
		t.Fatalf("task_scheduler artifact missing: %#v", generated)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated artifact: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "Register-ScheduledTask") || !strings.Contains(body, "ao overnight start") || !strings.Contains(body, "02:15") {
		t.Fatalf("unexpected task scheduler artifact:\n%s", body)
	}
}

func fakeOllamaServer(t *testing.T, models []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_, _ = w.Write([]byte(`{"version":"0.20.5"}`))
		case "/api/tags":
			payload := struct {
				Models []struct {
					Name string `json:"name"`
				} `json:"models"`
			}{}
			for _, model := range models {
				payload.Models = append(payload.Models, struct {
					Name string `json:"name"`
				}{Name: model})
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
}

func testBoolPtr(v bool) *bool {
	return &v
}
