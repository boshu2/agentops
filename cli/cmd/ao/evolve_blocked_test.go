// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withFixedBlockedClock pins the timestamp clock used by the blocked subcommand
// for deterministic assertions.
func withFixedBlockedClock(t *testing.T, ts time.Time) {
	t.Helper()
	prev := evolveBlockedTimestampClock
	evolveBlockedTimestampClock = func() time.Time { return ts }
	t.Cleanup(func() { evolveBlockedTimestampClock = prev })
}

// TestEvolveBlocked_WriteThenListRoundTrip writes a blocked event, reads it
// back via --list --json, and asserts structural equality on the parsed JSON.
func TestEvolveBlocked_WriteThenListRoundTrip(t *testing.T) {
	dir := chdirTemp(t)
	fixed := time.Date(2026, 5, 21, 16, 30, 0, 0, time.UTC)
	withFixedBlockedClock(t, fixed)

	// Write.
	writeOut, err := executeCommand(
		"evolve", "blocked",
		"--reason", "ladder exhausted",
		"--bead", "soc-mlbm",
		"--needed-context", "undefined ladder step 4 semantics",
		"--ladder-step-failed", "4",
		"--cycle", "2026-05-21-cycle-42",
	)
	if err != nil {
		t.Fatalf("write: %v\nout=%s", err, writeOut)
	}
	if !strings.Contains(writeOut, "Logged blocked event for cycle 2026-05-21-cycle-42") {
		t.Errorf("write stdout: want confirmation, got %q", writeOut)
	}

	logPath := filepath.Join(dir, ".agents/evolve/blocked.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 record, got %d", len(lines))
	}

	listOut, err := executeCommand("evolve", "blocked", "--list", "--json")
	if err != nil {
		t.Fatalf("list: %v\nout=%s", err, listOut)
	}
	// Strip leading non-JSON noise; output starts at first '['.
	jsonStart := strings.Index(listOut, "[")
	if jsonStart < 0 {
		t.Fatalf("list output missing JSON array: %q", listOut)
	}
	var got []BlockedEvent
	if err := json.Unmarshal([]byte(listOut[jsonStart:]), &got); err != nil {
		t.Fatalf("decode list json: %v\noutput=%s", err, listOut)
	}
	want := BlockedEvent{
		Cycle:            "2026-05-21-cycle-42",
		Timestamp:        fixed.Format(time.RFC3339),
		Bead:             "soc-mlbm",
		Reason:           "ladder exhausted",
		NeededContext:    "undefined ladder step 4 semantics",
		LadderStepFailed: 4,
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("round trip mismatch:\n got = %+v\nwant = %+v", got, want)
	}
}

// TestEvolveBlocked_ClearRemovesMatchingCycle seeds two events, clears one,
// then re-reads.
func TestEvolveBlocked_ClearRemovesMatchingCycle(t *testing.T) {
	dir := chdirTemp(t)
	fixed := time.Date(2026, 5, 21, 16, 30, 0, 0, time.UTC)

	path := filepath.Join(dir, ".agents/evolve/blocked.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	a := BlockedEvent{Cycle: "cycle-A", Timestamp: fixed.Format(time.RFC3339), Reason: "alpha"}
	b := BlockedEvent{Cycle: "cycle-B", Timestamp: fixed.Format(time.RFC3339), Reason: "beta"}
	for _, ev := range []BlockedEvent{a, b} {
		raw, _ := json.Marshal(ev)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, err := f.Write(append(raw, '\n')); err != nil {
			t.Fatalf("write seed: %v", err)
		}
		f.Close()
	}

	out, err := executeCommand("evolve", "blocked", "--clear", "cycle-A")
	if err != nil {
		t.Fatalf("clear: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "Cleared 1 blocked event(s) for cycle cycle-A") {
		t.Errorf("clear stdout: %q", out)
	}

	remaining, err := readBlockedEvents(path)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Cycle != "cycle-B" {
		t.Errorf("after clear: %+v", remaining)
	}
}

// TestEvolveBlocked_MalformedJSONLRejected confirms readBlockedEvents returns
// a typed error referencing the line number for malformed rows.
func TestEvolveBlocked_MalformedJSONLRejected(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blocked.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := readBlockedEvents(path)
	if err == nil {
		t.Fatalf("expected error for malformed line")
	}
	if !strings.Contains(err.Error(), ":1:") {
		t.Errorf("error missing line context: %v", err)
	}
}

// TestEvolveBlocked_MissingRequiredFieldsRejected confirms records that parse
// as JSON but omit required fields fail readBlockedEvents.
func TestEvolveBlocked_MissingRequiredFieldsRejected(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blocked.jsonl")
	row := `{"bead":"x"}` + "\n"
	if err := os.WriteFile(path, []byte(row), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := readBlockedEvents(path)
	if err == nil {
		t.Fatalf("expected error for missing fields")
	}
	if !strings.Contains(err.Error(), "missing required field") {
		t.Errorf("error message: %v", err)
	}
}

// TestEvolveBlocked_MutuallyExclusiveFlags confirms resolveBlockedMode rejects
// combinations of --reason + --list.
func TestEvolveBlocked_MutuallyExclusiveFlags(t *testing.T) {
	chdirTemp(t)

	out, err := executeCommand("evolve", "blocked", "--reason", "x", "--list")
	if err == nil {
		t.Fatalf("expected mutual-exclusion error\nout=%s", out)
	}
	combined := err.Error() + "\n" + out
	if !strings.Contains(combined, "mutually exclusive") {
		t.Errorf("error: %v\nout=%s", err, out)
	}
}

// TestEvolveBlocked_DefaultCycleIDUsesCronHistoryCounter confirms the default
// cycle id is derived as <date>-cycle-<count of cron-history rows>.
func TestEvolveBlocked_DefaultCycleIDUsesCronHistoryCounter(t *testing.T) {
	dir := chdirTemp(t)
	fixed := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	withFixedBlockedClock(t, fixed)

	historyPath := filepath.Join(dir, evolveCronHistoryRel)
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(historyPath, []byte("{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, err := executeCommand("evolve", "blocked", "--reason", "test")
	if err != nil {
		t.Fatalf("write: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "2026-05-21-cycle-3") {
		t.Errorf("default cycle: %q", out)
	}
}

// TestEvolveBlocked_RegisteredOnEvolve confirms the subcommand is reachable via
// `ao evolve blocked`.
func TestEvolveBlocked_RegisteredOnEvolve(t *testing.T) {
	var found bool
	for _, sub := range evolveCmd.Commands() {
		if sub.Name() == "blocked" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evolve blocked subcommand should be registered on evolveCmd")
	}
}

// TestEvolveBlocked_NextCycleIDFormat covers the cycle id derivation helper.
func TestEvolveBlocked_NextCycleIDFormat(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	got := nextBlockedCycleID(tmp, now)
	if got != "2026-05-21-cycle-0" {
		t.Errorf("nextBlockedCycleID() = %q, want %q", got, "2026-05-21-cycle-0")
	}
}
