// practices: [resilience-patterns, design-by-contract]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// errorTestdataDir returns the absolute path to testdata/errors/ relative to
// this test file's source location. This ensures golden file I/O works even
// when chdirTemp has changed the working directory.
func errorTestdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", "errors")
}

// errorGoldenTest compares error output against a golden file in testdata/errors/.
// Uses the same update mechanism as goldenTest but with an absolute path so it
// works even when chdirTemp has changed the cwd.
func errorGoldenTest(t *testing.T, name string, got []byte) {
	t.Helper()
	golden := filepath.Join(errorTestdataDir(), name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatalf("mkdir for golden: %v", err)
		}
		if err := os.WriteFile(golden, got, 0644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update-golden to create)", golden, err)
	}
	if diff := cmp.Diff(string(expected), string(got)); diff != "" {
		t.Errorf("error output mismatch (-want +got):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// Test 1: ratchet status in empty directory (no .agents)
// This test validates JSON structure rather than exact bytes because the output
// contains a generated chain_id and timestamp that change each run.
// ---------------------------------------------------------------------------

func TestErrorRatchetStatusNoAgents(t *testing.T) {
	tmp := chdirTemp(t)
	_ = tmp

	out, err := executeCommand("ratchet", "status", "--json")
	if err != nil {
		t.Fatalf("ratchet status --json in empty dir: %v", err)
	}

	// Validate it's valid JSON with expected structure
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	// Must have chain_id, started, steps, path
	for _, key := range []string{"chain_id", "started", "steps", "path"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected key %q in JSON output", key)
		}
	}

	// chain_id should start with "chain-"
	if chainID, ok := parsed["chain_id"].(string); !ok || !strings.HasPrefix(chainID, "chain-") {
		t.Errorf("expected chain_id starting with 'chain-', got %v", parsed["chain_id"])
	}

	// steps should be an array with 7 entries (all RPI steps)
	if steps, ok := parsed["steps"].([]interface{}); !ok || len(steps) != 7 {
		t.Errorf("expected 7 steps, got %v", parsed["steps"])
	}
}

// ---------------------------------------------------------------------------
// Test 2: ratchet next with non-existent epic filter
// ---------------------------------------------------------------------------

func TestErrorRatchetNextBadEpic(t *testing.T) {
	tmp := chdirTemp(t)
	// Create a minimal chain so LoadChain finds something
	setupAgentsDir(t, tmp)
	chainDir := filepath.Join(tmp, ".agents", "ao")
	chainContent := `{"id":"err-test-001","started":"2025-06-15T08:00:00Z"}
{"step":"research","timestamp":"2025-06-15T08:10:00Z","output":"findings.md","locked":true}
`
	writeFile(t, filepath.Join(chainDir, "chain.jsonl"), chainContent)

	_, err := executeCommand("ratchet", "next", "--epic", "nonexistent-epic-xyz")
	if err == nil {
		t.Fatal("expected error for non-existent epic, got nil")
	}

	errorGoldenTest(t, "ratchet-next-bad-epic.txt", []byte(err.Error()+"\n"))
}

// ---------------------------------------------------------------------------
// Test 3: status command in uninitialized directory
// Validates JSON structure rather than exact bytes because the output
// contains the temp directory path which changes each run.
// ---------------------------------------------------------------------------

func TestErrorStatusUninitialized(t *testing.T) {
	_ = chdirTemp(t)

	out, err := executeCommand("status", "--json")
	if err != nil {
		t.Fatalf("status --json should not error in uninitialized dir, got: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	// Must report initialized=false
	if init, ok := parsed["initialized"].(bool); !ok || init {
		t.Errorf("expected initialized=false, got %v", parsed["initialized"])
	}

	// Must have base_dir containing .agents/ao
	if baseDir, ok := parsed["base_dir"].(string); !ok || !strings.Contains(baseDir, ".agents/ao") {
		t.Errorf("expected base_dir containing .agents/ao, got %v", parsed["base_dir"])
	}

	// session_count should be 0
	if count, ok := parsed["session_count"].(float64); !ok || count != 0 {
		t.Errorf("expected session_count=0, got %v", parsed["session_count"])
	}
}

// ---------------------------------------------------------------------------
// Test 4: unknown subcommand
// ---------------------------------------------------------------------------

func TestErrorUnknownSubcommand(t *testing.T) {
	out, err := executeCommand("nonexistent-command-xyz")
	var result string
	if err != nil {
		result = out + err.Error() + "\n"
	} else {
		result = out
	}

	errorGoldenTest(t, "unknown-subcommand.txt", []byte(result))
}

// ---------------------------------------------------------------------------
// Test 5: forge without required argument
// ---------------------------------------------------------------------------

func TestErrorForgeNoArgs(t *testing.T) {
	_ = chdirTemp(t)

	out, err := executeCommand("forge")
	var result string
	if err != nil {
		result = out + err.Error() + "\n"
	} else {
		result = out
	}

	errorGoldenTest(t, "forge-no-args.txt", []byte(result))
}

// ---------------------------------------------------------------------------
// Test 6: ratchet record with missing --output flag
// ---------------------------------------------------------------------------

func TestErrorRatchetRecordMissingOutput(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("ratchet", "record", "--step", "research")
	var result string
	if err != nil {
		result = out + err.Error() + "\n"
	} else {
		result = out
	}

	errorGoldenTest(t, "ratchet-record-missing-output.txt", []byte(result))
}

// ---------------------------------------------------------------------------
// Test 7: doctor in empty directory (various checks should fail/warn)
// ---------------------------------------------------------------------------

func TestErrorDoctorEmptyDir(t *testing.T) {
	_ = chdirTemp(t)

	// Use the computeResult + renderDoctorTable path to get deterministic output.
	// gatherDoctorChecks depends on filesystem state. For a controlled test,
	// synthesize checks matching what an empty dir produces.
	checks := []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: "vdev", Required: true},
		{Name: "Knowledge Base", Status: "fail", Detail: ".agents/ao not initialized", Required: true},
		{Name: "Flywheel Health", Status: "warn", Detail: "No learnings found \u2014 the flywheel hasn't started", Required: false},
	}
	result := computeResult(checks)

	var buf bytes.Buffer
	renderDoctorTable(&buf, result)

	errorGoldenTest(t, "doctor-empty-dir.txt", buf.Bytes())
}
