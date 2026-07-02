//go:build legacy

// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCodexEnsureStartMaintenanceSkipsStalePendingAlreadyPromotedBody exercises
// the `codex ensure-start` maintenance path. Split out of
// flywheel_promoted_body_dedupe_test.go (which keeps the spine close-loop tests +
// shared fixtures untagged) because the codex command is archived behind
// //go:build legacy (age-h4y3). It reuses the untagged shared fixtures.
func TestCodexEnsureStartMaintenanceSkipsStalePendingAlreadyPromotedBody(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_THREAD_ID", "codex-stale-pending-promoted-body")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	tmp, pendingFile := setupStalePendingAlreadyPromotedFixture(t)
	if err := os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("# Test repo\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	t.Chdir(tmp)
	out, err := executeCommand("codex", "ensure-start", "--json", "--query", "stale pending promoted body")
	if err != nil {
		t.Fatalf("codex ensure-start: %v\noutput: %s", err, out)
	}
	var first codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &first); err != nil {
		t.Fatalf("parse ensure-start json: %v\noutput: %s", err, out)
	}
	if !first.Performed {
		t.Fatalf("ensure-start Performed=false, want true: %+v", first)
	}

	assertSinglePromotedArtifact(t, tmp)
	assertPendingMovedToProcessed(t, tmp, pendingFile)
	assertDuplicateSkipAudited(t, tmp)

	before, err := os.ReadFile(filepath.Join(tmp, ".agents", "ao", "codex", "state.json"))
	if err != nil {
		t.Fatalf("read codex state: %v", err)
	}
	time.Sleep(time.Millisecond)
	secondOut, err := executeCommand("codex", "ensure-start", "--json", "--query", "stale pending promoted body")
	if err != nil {
		t.Fatalf("second codex ensure-start: %v\noutput: %s", err, secondOut)
	}
	var second codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(secondOut)), &second); err != nil {
		t.Fatalf("parse second ensure-start json: %v\noutput: %s", err, secondOut)
	}
	if second.Performed {
		t.Fatalf("second ensure-start Performed=true, want false: %+v", second)
	}
	after, err := os.ReadFile(filepath.Join(tmp, ".agents", "ao", "codex", "state.json"))
	if err != nil {
		t.Fatalf("read codex state after second ensure-start: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("ensure-start idempotency changed state\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
	assertSinglePromotedArtifact(t, tmp)
}
