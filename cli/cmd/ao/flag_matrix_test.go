package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// aoBinary resolves the path to the ao binary built by `make build`.
// Tests that need the binary should call this and skip if it's missing.
func aoBinary(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find cli/bin/ao
	_, thisFile, _, _ := runtime.Caller(0)
	binPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "bin", "ao")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("ao binary not found at %s — run 'cd cli && make build' first", binPath)
	}
	return binPath
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_JSONOutput
//
// For every command that supports --json and produces parseable JSON,
// verify exit code 0 and valid JSON output.
// ---------------------------------------------------------------------------

func TestFlagMatrix_JSONOutput(t *testing.T) {
	bin := aoBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{"search", []string{"search", "--json", "test"}},
		{"ratchet-status", []string{"ratchet", "status", "--json"}},
		{"flywheel-status", []string{"flywheel", "status", "--json"}},
		{"pool-list", []string{"pool", "list", "--json"}},
		{"status", []string{"status", "--json"}},
		{"doctor", []string{"doctor", "--json"}},
		{"metrics-report", []string{"metrics", "report", "--json"}},
		{"vibe-check", []string{"vibe-check", "--json"}},
		{"knowledge-gaps", []string{"knowledge", "gaps", "--json"}},
		// NOTE: goals measure --json requires GOALS.md to exist; excluded from
		// this matrix because it fails in repos without one.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			cmd.Dir = findRepoRoot(t)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command %v failed (exit error: %v):\n%s", tt.args, err, string(out))
			}

			trimmed := strings.TrimSpace(string(out))
			if len(trimmed) == 0 {
				t.Fatalf("command %v produced empty output", tt.args)
			}
			if !json.Valid([]byte(trimmed)) {
				// Show first 500 chars for debugging
				snippet := trimmed
				if len(snippet) > 500 {
					snippet = snippet[:500] + "..."
				}
				t.Errorf("command %v produced invalid JSON:\n%s", tt.args, snippet)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_QuietMode
//
// For commands with --quiet, verify they exit 0 and produce less or no output.
// ---------------------------------------------------------------------------

func TestFlagMatrix_QuietMode(t *testing.T) {
	bin := aoBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{"memory-sync", []string{"memory", "sync", "--quiet"}},
		{"notebook-update", []string{"notebook", "update", "--quiet"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			cmd.Dir = findRepoRoot(t)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command %v failed (exit error: %v):\n%s", tt.args, err, string(out))
			}
			// Quiet mode should succeed — output may be empty or minimal, both are fine.
			// We only assert exit code 0 (already checked via err == nil).
		})
	}
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_InvalidFlags
//
// Verify that passing an unknown flag produces a non-zero exit and stderr
// contains an error message about the unknown flag.
// ---------------------------------------------------------------------------

func TestFlagMatrix_InvalidFlags(t *testing.T) {
	bin := aoBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version", "--nonexistent-flag"}},
		{"status", []string{"status", "--nonexistent-flag"}},
		{"doctor", []string{"doctor", "--nonexistent-flag"}},
		{"search", []string{"search", "--nonexistent-flag"}},
		{"inject", []string{"inject", "--nonexistent-flag"}},
		{"knowledge", []string{"knowledge", "--nonexistent-flag"}},
		{"badge", []string{"badge", "--nonexistent-flag"}},
		{"ratchet-status", []string{"ratchet", "status", "--nonexistent-flag"}},
		{"pool-list", []string{"pool", "list", "--nonexistent-flag"}},
		{"flywheel-status", []string{"flywheel", "status", "--nonexistent-flag"}},
		{"metrics-report", []string{"metrics", "report", "--nonexistent-flag"}},
		{"compile", []string{"compile", "--nonexistent-flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			cmd.Dir = findRepoRoot(t)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("command %v should have failed with unknown flag, but exited 0:\n%s",
					tt.args, string(out))
			}

			combined := strings.ToLower(string(out))
			if !strings.Contains(combined, "unknown flag") &&
				!strings.Contains(combined, "unknown shorthand") &&
				!strings.Contains(combined, "bad flag syntax") {
				t.Errorf("command %v error output does not mention unknown flag:\n%s",
					tt.args, string(out))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_HelpConsistency
//
// For every top-level command, verify --help exits 0 and produces non-empty
// output. This catches commands that were registered but crash on --help.
// ---------------------------------------------------------------------------

func TestFlagMatrix_HelpConsistency(t *testing.T) {
	bin := aoBinary(t)

	commands := []string{
		"version",
		"status",
		"doctor",
		"seed",
		"hooks",
		"completion",
		// Deprecated aliases (still show help)
		"search",
		"inject",
		"badge",
		"compile",
		"constraint",
		"contradict",
		"curate",
		"dedup",
		"lookup",
		"knowledge",
		"memory",
		"notebook",
		"metrics",
		"goals",
		"ratchet",
		"retrieval-bench",
		"rpi",
		"pool",
		"flywheel",
		"forge",
		"session",
		"config",
		"trace",
		"maturity",
		"anti-patterns",
		"plans",
		"gate",
		"init",
		"demo",
		"vibe-check",
		"quick-start",
		// Former namespace commands now top-level
		"forge",
		"pool",
		"ratchet",
		"memory",
		"seed",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			c := exec.Command(bin, cmd, "--help")
			c.Dir = findRepoRoot(t)
			out, err := c.CombinedOutput()
			if err != nil {
				t.Fatalf("%s --help failed (exit error: %v):\n%s", cmd, err, string(out))
			}

			trimmed := strings.TrimSpace(string(out))
			if len(trimmed) == 0 {
				t.Errorf("%s --help produced empty output", cmd)
			}

			// Every help output should contain "Usage:" (cobra standard)
			if !strings.Contains(trimmed, "Usage:") && !strings.Contains(trimmed, "usage:") {
				t.Errorf("%s --help output missing 'Usage:' section:\n%s", cmd, trimmed)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_MutuallyExclusiveFormats
//
// Verify that -o json and --json produce equivalent output for a command.
// ---------------------------------------------------------------------------

func TestFlagMatrix_MutuallyExclusiveFormats(t *testing.T) {
	bin := aoBinary(t)

	t.Run("status-json-equivalence", func(t *testing.T) {
		// Run with --json
		cmd1 := exec.Command(bin, "status", "--json")
		cmd1.Dir = findRepoRoot(t)
		out1, err1 := cmd1.CombinedOutput()
		if err1 != nil {
			t.Fatalf("status --json failed: %v\n%s", err1, string(out1))
		}

		// Run with -o json
		cmd2 := exec.Command(bin, "status", "-o", "json")
		cmd2.Dir = findRepoRoot(t)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			t.Fatalf("status -o json failed: %v\n%s", err2, string(out2))
		}

		// Both should be valid JSON
		s1 := strings.TrimSpace(string(out1))
		s2 := strings.TrimSpace(string(out2))

		if !json.Valid([]byte(s1)) {
			t.Errorf("status --json produced invalid JSON")
		}
		if !json.Valid([]byte(s2)) {
			t.Errorf("status -o json produced invalid JSON")
		}

		// Both should parse to the same structure (same keys)
		var m1, m2 map[string]interface{}
		if err := json.Unmarshal([]byte(s1), &m1); err != nil {
			t.Fatalf("parse --json output: %v", err)
		}
		if err := json.Unmarshal([]byte(s2), &m2); err != nil {
			t.Fatalf("parse -o json output: %v", err)
		}

		// Verify same top-level keys exist
		for k := range m1 {
			if _, ok := m2[k]; !ok {
				t.Errorf("key %q present in --json output but missing from -o json output", k)
			}
		}
		for k := range m2 {
			if _, ok := m1[k]; !ok {
				t.Errorf("key %q present in -o json output but missing from --json output", k)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestFlagMatrix_NoCommandShowsHelp
//
// Running `ao` with no arguments should exit 0 and show usage info.
// ---------------------------------------------------------------------------

func TestFlagMatrix_NoCommandShowsHelp(t *testing.T) {
	bin := aoBinary(t)

	cmd := exec.Command(bin)
	cmd.Dir = findRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ao (no args) failed: %v\n%s", err, string(out))
	}

	s := string(out)
	if !strings.Contains(s, "Usage:") && !strings.Contains(s, "ao [command]") {
		t.Errorf("ao (no args) should show usage info, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// findRepoRoot walks up from the test to find the repo root (has .agents/ dir).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	// cli/cmd/ao/ -> walk up 3 levels to get repo root
	dir := filepath.Dir(thisFile)
	for range 5 {
		if _, err := os.Stat(filepath.Join(dir, ".agents")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	// Fallback: use current working directory
	wd, _ := os.Getwd()
	return wd
}
