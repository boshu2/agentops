// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func writeCycleHistoryFixture(t *testing.T, dir string, lines []string) string {
	t.Helper()
	evolveDir := filepath.Join(dir, ".agents", "evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(evolveDir, "cycle-history.jsonl")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoopHistory_Latest(t *testing.T) {
	dir := t.TempDir()
	writeCycleHistoryFixture(t, dir, []string{
		`{"cycle":1,"mode":"a","result":"improved"}`,
		`{"cycle":2,"mode":"b","result":"improved"}`,
		`{"cycle":3,"mode":"c","result":"unchanged"}`,
	})
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		latest: true,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], `"Number":3`) {
		t.Fatalf("latest entry wrong: %s", lines[0])
	}
}

func TestLoopHistory_LimitTrimsToLast(t *testing.T) {
	dir := t.TempDir()
	writeCycleHistoryFixture(t, dir, []string{
		`{"cycle":1}`, `{"cycle":2}`, `{"cycle":3}`, `{"cycle":4}`, `{"cycle":5}`,
	})
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		limit:  2,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("limit=2 → %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Number":4`) || !strings.Contains(lines[1], `"Number":5`) {
		t.Fatalf("limit should give last 2, got:\n%s", buf.String())
	}
}

func TestLoopHistory_Range(t *testing.T) {
	dir := t.TempDir()
	writeCycleHistoryFixture(t, dir, []string{
		`{"cycle":1}`, `{"cycle":2}`, `{"cycle":3}`, `{"cycle":4}`,
	})
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		start:  2,
		end:    3,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("range 2-3 → %d lines, want 2", len(lines))
	}
}

func TestLoopHistory_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// no fixture written — file missing
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		latest: true,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("missing file should emit 0 entries; got %q", buf.String())
	}
}

func TestLoopHistory_HistoryFnInjectable(t *testing.T) {
	// Verify that callers can substitute the loader (for tests that
	// don't want to cd or write fixtures).
	stub := func(_ context.Context, _ loopHistoryOptions) ([]ports.CycleEntry, error) {
		return []ports.CycleEntry{
			{Number: 7, Mode: "x", Result: "improved"},
		}, nil
	}
	var buf bytes.Buffer
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		writer:    &buf,
		historyFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"Number":7`) {
		t.Fatalf("stub not used: %q", buf.String())
	}
}

func TestLoopHistory_HistoryFnErrorPropagates(t *testing.T) {
	stub := func(_ context.Context, _ loopHistoryOptions) ([]ports.CycleEntry, error) {
		return nil, errors.New("boom")
	}
	err := loopHistoryRun(context.Background(), loopHistoryOptions{
		historyFn: stub,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "loop history:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

// Cycle 163: tests for ao loop verify — first NEW consumer of the
// cycle-161 CycleEntry widening. Audits cycle-history integrity
// using the typed LoopReaderPort surface.

func TestCheckLoopIntegrity_CleanLedger(t *testing.T) {
	entries := []ports.CycleEntry{
		{Number: 1, StartedAt: "2026-05-13T00:00:00Z", Result: "improved"},
		{Number: 2, StartedAt: "2026-05-13T01:00:00Z", Result: "improved"},
		{Number: 3, StartedAt: "2026-05-13T02:00:00Z", Result: "improved"},
	}
	issues := checkLoopIntegrity(entries, 0, 5)
	if len(issues) != 0 {
		t.Fatalf("clean ledger should have no issues, got %v", issues)
	}
}

func TestCheckLoopIntegrity_NonMonotonicNumber(t *testing.T) {
	entries := []ports.CycleEntry{
		{Number: 1, StartedAt: "2026-05-13T00:00:00Z"},
		{Number: 3, StartedAt: "2026-05-13T01:00:00Z"},
		{Number: 2, StartedAt: "2026-05-13T02:00:00Z"},
	}
	issues := checkLoopIntegrity(entries, 0, 5)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "non-monotonic: cycle 2 follows cycle 3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected non-monotonic issue, got %v", issues)
	}
}

func TestCheckLoopIntegrity_DuplicateNumber(t *testing.T) {
	entries := []ports.CycleEntry{
		{Number: 1, StartedAt: "2026-05-13T00:00:00Z"},
		{Number: 2, StartedAt: "2026-05-13T01:00:00Z"},
		{Number: 2, StartedAt: "2026-05-13T02:00:00Z"},
	}
	issues := checkLoopIntegrity(entries, 0, 5)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "duplicate cycle number: 2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate issue, got %v", issues)
	}
}

func TestCheckLoopIntegrity_MissingStartedAt(t *testing.T) {
	entries := []ports.CycleEntry{
		{Number: 1, StartedAt: "2026-05-13T00:00:00Z"},
		{Number: 2},
	}
	issues := checkLoopIntegrity(entries, 0, 5)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "cycle 2 missing StartedAt") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing-StartedAt issue, got %v", issues)
	}
}

func TestCheckLoopIntegrity_IdleStreakExceedsThreshold(t *testing.T) {
	entries := []ports.CycleEntry{
		{Number: 1, StartedAt: "2026-05-13T00:00:00Z"},
	}
	issues := checkLoopIntegrity(entries, 10, 5)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "IdleStreak=10 exceeds max-idle=5") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected idle-streak issue, got %v", issues)
	}
}

func TestLoopVerifyRun_StubPASS(t *testing.T) {
	stub := func(_ context.Context, _ loopVerifyOptions) ([]string, error) {
		return nil, nil
	}
	var buf bytes.Buffer
	err := loopVerifyRun(context.Background(), loopVerifyOptions{
		writer:   &buf,
		verifyFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PASS") {
		t.Fatalf("expected PASS in output, got %q", buf.String())
	}
}

func TestLoopVerifyRun_StubFAIL(t *testing.T) {
	stub := func(_ context.Context, _ loopVerifyOptions) ([]string, error) {
		return []string{"sample issue"}, nil
	}
	var buf bytes.Buffer
	err := loopVerifyRun(context.Background(), loopVerifyOptions{
		writer:   &buf,
		verifyFn: stub,
	})
	if err == nil {
		t.Fatal("expected non-nil error when issues found")
	}
	if !strings.Contains(buf.String(), "FAIL") || !strings.Contains(buf.String(), "sample issue") {
		t.Fatalf("expected FAIL + issue in output, got %q", buf.String())
	}
}
