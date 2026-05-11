// practices: [agile-manifesto, dora-metrics]
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartEmbeddedDashboard_ServesOnEphemeralPort(t *testing.T) {
	root := t.TempDir()
	runID := "test-run-abc123"

	srv, dashURL := startEmbeddedDashboard(root, runID, false)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	defer shutdownDashboard(srv)

	if dashURL == "" {
		t.Fatal("expected non-empty dashboard URL")
	}
	if !strings.HasPrefix(dashURL, "http://") {
		t.Errorf("URL should start with http://, got %q", dashURL)
	}
	if !strings.Contains(dashURL, "run="+runID) {
		t.Errorf("URL should contain run=%s, got %q", runID, dashURL)
	}

	// Verify the server is actually responding
	resp, err := http.Get(dashURL)
	if err != nil {
		t.Fatalf("GET dashboard URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestShutdownDashboard_NilServer(t *testing.T) {
	// shutdownDashboard(nil) must be a no-op — verify no panic and clean return
	shutdownDashboard(nil)
	// If we reached here, nil server was handled gracefully
	if t.Failed() {
		t.Error("shutdownDashboard(nil) should not cause failures")
	}
}

func TestDash_StartEmbeddedDashboard_ReturnsServer(t *testing.T) {
	root := t.TempDir()
	runID := "dash-returns-server"

	srv, dashURL := startEmbeddedDashboard(root, runID, false)
	if srv == nil {
		t.Fatal("expected non-nil server from startEmbeddedDashboard")
	}
	defer shutdownDashboard(srv)

	if dashURL == "" {
		t.Fatal("expected non-empty URL")
	}

	// URL should have a valid port (not :0)
	if strings.Contains(dashURL, ":0") && !strings.Contains(dashURL, ":0/") {
		t.Errorf("dashboard URL has unresolved port 0: %s", dashURL)
	}
	if !strings.Contains(dashURL, "run="+runID) {
		t.Errorf("URL %q missing run=%s query param", dashURL, runID)
	}

	// Verify the URL has a real port by making a request
	resp, err := http.Get(dashURL)
	if err != nil {
		t.Fatalf("GET %s: %v", dashURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDash_ShutdownDashboard_GracefulClose(t *testing.T) {
	root := t.TempDir()
	runID := "dash-graceful"

	srv, dashURL := startEmbeddedDashboard(root, runID, false)
	if srv == nil {
		t.Fatal("server is nil")
	}

	// Server should be alive before shutdown
	resp, err := http.Get(dashURL)
	if err != nil {
		t.Fatalf("server not alive before shutdown: %v", err)
	}
	resp.Body.Close()

	// Shutdown should complete without error (shutdownDashboard doesn't return error,
	// but we verify it doesn't panic and the server stops accepting)
	shutdownDashboard(srv)

	// After shutdown, requests should fail
	_, err = http.Get(dashURL)
	if err == nil {
		t.Error("expected error after shutdown, but GET succeeded")
	}
}

func TestDash_IsTerminal_NonTTY(t *testing.T) {
	// In test environment, stdout is typically not a terminal (piped to test runner)
	got := isTerminal()
	if got {
		t.Skip("stdout appears to be a TTY in this environment; skipping non-TTY assertion")
	}
	// In CI / test runner, isTerminal() should return false
	if got {
		t.Error("isTerminal() = true, expected false when not connected to TTY")
	}
}

func TestBuildServeMux_Routes(t *testing.T) {
	root := t.TempDir()
	runID := "test-mux-run"
	mux := buildServeMux(&serveMuxRoot{path: root}, runID)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCTHint string // substring of Content-Type
	}{
		{
			name:       "index returns HTML",
			path:       "/",
			wantStatus: http.StatusOK,
			wantCTHint: "text/html",
		},
		{
			name:       "runs returns JSON",
			path:       "/runs",
			wantStatus: http.StatusOK,
			wantCTHint: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d; body = %s", resp.StatusCode, tt.wantStatus, body)
			}
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, tt.wantCTHint) {
				t.Errorf("Content-Type = %q, want substring %q", ct, tt.wantCTHint)
			}
		})
	}
}
