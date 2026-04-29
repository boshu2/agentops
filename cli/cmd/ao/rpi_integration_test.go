package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func TestRPIDaemonL3SmokeFakeGasCity(t *testing.T) {
	cwd := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
		Now:   func() time.Time { return time.Now().UTC() },
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

	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cwd
	opts.DaemonSubmit = true
	opts.DaemonURL = activation.URL
	opts.DaemonToken = "secret-token"
	opts.RunID = "l3-run"
	handled, err := maybeSubmitRPIPhasedDaemon(context.Background(), opts, []string{"exercise daemon RPI smoke"})
	if err != nil {
		t.Fatalf("daemon submit: %v", err)
	}
	if !handled {
		t.Fatal("daemon submit was not handled")
	}

	gasCityServer := newFakeGasCityRPIServer(t)
	defer gasCityServer.Close()
	gasCityClient, err := gascity.NewClient(gascity.Config{
		Endpoint:      gasCityServer.URL,
		MutationToken: "agentops-test",
	})
	if err != nil {
		t.Fatalf("New GasCity client: %v", err)
	}
	store := daemonpkg.NewStore(cwd)
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	runner, err := daemonpkg.NewRPIRunner(store, daemonpkg.RPIRunnerOptions{
		Queue: queue,
		Executor: daemonpkg.GasCityRPIPhaseExecutor{
			Client:       daemonpkg.GasCityClientAdapter{Client: gasCityClient},
			CityName:     "agentops",
			PhaseTimeout: time.Second,
		},
		Actor: "l3-rpi-runner",
	})
	if err != nil {
		t.Fatalf("NewRPIRunner: %v", err)
	}
	result, err := runner.RunNextRPIJob(context.Background())
	if err != nil {
		t.Fatalf("RunNextRPIJob: %v", err)
	}
	if result.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("runner result = %#v", result)
	}

	status, err := fetchDaemonStatus(context.Background(), activation.URL)
	if err != nil {
		t.Fatalf("fetch daemon status: %v", err)
	}
	output := buildRPIStatusOutputFromDaemon(status)
	if output.Count != 1 || len(output.Historical) != 1 {
		t.Fatalf("daemon status output = %#v", output)
	}
	if output.Historical[0].RunID != "l3-run" || output.Historical[0].Status != string(daemonpkg.JobStatusCompleted) {
		t.Fatalf("historical daemon run = %#v", output.Historical[0])
	}

	statePath := filepath.Join(cwd, ".agents", "rpi", "runs", "l3-run", cliRPI.PhasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read RPI run artifact: %v", err)
	}
	if !strings.Contains(string(data), `"terminal_status": "completed"`) {
		t.Fatalf("run artifact missing completed terminal status:\n%s", string(data))
	}
	for phase := 1; phase <= 3; phase++ {
		if _, err := os.Stat(cliRPI.GasCityPhaseEvidencePath(cwd, "l3-run", phase)); err != nil {
			t.Fatalf("phase %d evidence not written: %v", phase, err)
		}
	}
}

func newFakeGasCityRPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	sessions := map[string]string{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(gascity.RequestIDHeader, "req-"+strings.Trim(strings.ReplaceAll(r.URL.Path, "/", "-"), "-"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/readiness":
			_ = json.NewEncoder(w).Encode(gascity.ReadinessResponse{Ready: true, Status: "ready"})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/sessions":
			if got := r.Header.Get(gascity.MutationHeader); got == "" {
				t.Fatalf("create missing %s", gascity.MutationHeader)
			}
			var req gascity.SessionCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			if req.Alias == "" || req.Name != "worker" || !req.Async {
				t.Fatalf("create request = %#v", req)
			}
			sessionID := "sess_" + strings.ReplaceAll(req.Alias, "-", "_")
			mu.Lock()
			sessions[sessionID] = req.Alias
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(gascity.Session{ID: sessionID, Alias: req.Alias, Running: true})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v0/city/agentops/session/") && strings.HasSuffix(r.URL.Path, "/submit"):
			if got := r.Header.Get(gascity.MutationHeader); got == "" {
				t.Fatalf("submit missing %s", gascity.MutationHeader)
			}
			var req gascity.SessionSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submit: %v", err)
			}
			if req.Message == "" || req.Intent != "follow_up" {
				t.Fatalf("submit request = %#v", req)
			}
			_ = json.NewEncoder(w).Encode(gascity.SessionSubmitResponse{Queued: true, Intent: req.Intent})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			mu.Lock()
			defer mu.Unlock()
			seq := 0
			for sessionID := range sessions {
				seq++
				writeSSEFrame(t, w, sessionID+"-done", gascity.EventStreamEnvelope{
					Seq:     int64(seq),
					Type:    "session.completed",
					Subject: sessionID,
					Payload: map[string]any{"session_id": sessionID, "status": "completed"},
				})
			}
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v0/city/agentops/session/") && strings.HasSuffix(r.URL.Path, "/transcript"):
			if got := r.URL.Query().Get("format"); got != "conversation" {
				t.Fatalf("transcript format = %q, want conversation", got)
			}
			sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v0/city/agentops/session/"), "/transcript")
			_ = json.NewEncoder(w).Encode(gascity.TranscriptResponse{
				ID:        "tx_" + sessionID,
				SessionID: sessionID,
				Format:    "conversation",
				Turns:     []gascity.TranscriptEntry{{Role: "assistant", Text: "done"}},
				Messages:  []map[string]any{{"role": "assistant", "content": "done"}},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
}
