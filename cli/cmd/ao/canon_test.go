// practices: [ddd-bounded-context, knowledge-flywheel]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/canon"
	"github.com/spf13/cobra"
)

// TestRunCanonStatus_TierAware drives the canon status command and asserts the
// tier-aware gate is surfaced: a heuristic learning earns on 3 cross-engineer
// citations alone, and the tier is shown.
func TestRunCanonStatus_TierAware(t *testing.T) {
	dir := t.TempDir()
	learnDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hPath := filepath.Join(learnDir, "h.md")
	if err := os.WriteFile(hPath, []byte("---\nauthor: Alice\nauthor_email: alice@example.com\ncanon_tier: heuristic\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cl, _ := canonLedgers(dir)
	ts := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	for _, who := range []canon.Identity{
		{Name: "Bob", Email: "bob@x"},
		{Name: "Carol", Email: "carol@x"},
		{Name: "Dave", Email: "dave@x"},
	} {
		if _, err := cl.Record("h", hPath, "q", "s", who, ts); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(dir)
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runCanonStatus(cmd, []string{"h"}); err != nil {
		t.Fatalf("runCanonStatus: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "EARNED") {
		t.Errorf("heuristic with 3 cross-engineer citations should be EARNED; got %q", out)
	}
	if !strings.Contains(out, "[heuristic]") {
		t.Errorf("status should surface the tier; got %q", out)
	}
}
