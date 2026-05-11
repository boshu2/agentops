// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// startEmbeddedDashboard launches the mission control web dashboard on an
// OS-assigned port (avoids collision with standalone `ao rpi serve` on 7799).
// It reuses the existing buildServeMux and openBrowserURL from rpi_serve.go.
// Returns the server handle (for deferred shutdown) and the dashboard URL.
// Returns (nil, "") if the listener cannot bind.
func startEmbeddedDashboard(root, runID string, noDashboard bool) (*http.Server, string) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		VerbosePrintf("Warning: could not start dashboard server: %v\n", err)
		return nil, ""
	}

	addr := ln.Addr().String()
	dashURL := fmt.Sprintf("http://%s?run=%s", addr, runID)

	mux := buildServeMux(&serveMuxRoot{path: root}, runID)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			VerbosePrintf("Warning: dashboard server error: %v\n", err)
		}
	}()

	if !noDashboard {
		openBrowserURL(dashURL)
	}
	return srv, dashURL
}

// shutdownDashboard gracefully shuts down the embedded dashboard server.
func shutdownDashboard(srv *http.Server) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// isTerminal reports whether stdout is connected to a terminal (TTY).
// Uses stdlib only — no golang.org/x/term dependency.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
