//go:build legacy

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrchestrateSelectCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"orchestrate", "select"})
	if err != nil {
		t.Fatalf("orchestrate select command not registered: %v", err)
	}
	if cmd == nil {
		t.Fatal("orchestrate select command not found")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("orchestrate select missing --json flag")
	}
	if cmd.Flags().Lookup("pin") == nil {
		t.Fatal("orchestrate select missing --pin flag")
	}
	if cmd.Flags().Lookup("opt-out") == nil {
		t.Fatal("orchestrate select missing --opt-out flag")
	}
}

func orchestrateTestChdir(t *testing.T) {
	t.Helper()
	t.Chdir(orchestrateTestRepoRoot(t)) // auto-restores at the calling test's cleanup.
}

func orchestrateTestRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "docs/contracts/orchestration-tools.yaml")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("repo root not found")
		}
		wd = parent
	}
}

func TestOrchestrateToolsCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "tools"}); err != nil {
		t.Fatalf("orchestrate tools: %v", err)
	}
}

func TestOrchestratePreflightCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "preflight"}); err != nil {
		t.Fatalf("orchestrate preflight: %v", err)
	}
}

func TestOrchestrateVerifyCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "verify"}); err != nil {
		t.Fatalf("orchestrate verify: %v", err)
	}
}

func TestOrchestrateRouteCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "route"}); err != nil {
		t.Fatalf("orchestrate route: %v", err)
	}
}

func TestOrchestrateStatusCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "status"}); err != nil {
		t.Fatalf("orchestrate status: %v", err)
	}
}

func TestOrchestrateShapeCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"orchestrate", "shape"}); err != nil {
		t.Fatalf("orchestrate shape: %v", err)
	}
}

func TestOrchestrateRouteExecuteJSON(t *testing.T) {
	orchestrateTestChdir(t)
	_, err := executeCommand("orchestrate", "route", "--writers", "2", "--json")
	if err != nil {
		t.Fatalf("orchestrate route --json: %v", err)
	}
}
