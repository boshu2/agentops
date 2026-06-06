package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// writeCorpusFile creates an .agents/<section>/<name> markdown file under base.
func writeCorpusFile(t *testing.T, base, section, name string) string {
	t.Helper()
	dir := filepath.Join(base, ".agents", section)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("# learning\nbody\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestComputeReuseReceipt(t *testing.T) {
	base := t.TempDir()

	// Corpus: 2 learnings + 1 pattern = 3 durable entries; a stray .txt is ignored.
	pA := writeCorpusFile(t, base, "learnings", "2026-06-06-alpha-decision.md")
	pB := writeCorpusFile(t, base, "learnings", "2026-06-05-beta-pattern.md")
	writeCorpusFile(t, base, "patterns", "gamma.md")
	writeCorpusFile(t, base, "learnings", "notes.txt")

	now := time.Now().UTC()
	events := []types.CitationEvent{
		// Session under test reuses alpha twice (dedup → 1) and beta once.
		{ArtifactPath: pA, SessionID: "sess-1", CitationType: "retrieved", CitedAt: now},
		{ArtifactPath: pA, SessionID: "sess-1", CitationType: "applied", CitedAt: now},
		{ArtifactPath: pB, SessionID: "sess-1", CitationType: "retrieved", CitedAt: now},
		// A "reference" citation is a manual pointer, not a flywheel read-payoff.
		{ArtifactPath: filepath.Join(base, ".agents", "patterns", "gamma.md"), SessionID: "sess-1", CitationType: "reference", CitedAt: now},
		// Another session's reuse must not leak into this receipt.
		{ArtifactPath: pB, SessionID: "sess-2", CitationType: "retrieved", CitedAt: now},
	}
	for _, e := range events {
		if err := ratchet.RecordCitation(base, e); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	got := computeReuseReceipt(base, "sess-1")

	if got.Reused != 2 {
		t.Errorf("Reused = %d, want 2 (alpha deduped, beta; reference excluded)", got.Reused)
	}
	if got.CorpusEntries != 3 {
		t.Errorf("CorpusEntries = %d, want 3 (2 learnings + 1 pattern, .txt ignored)", got.CorpusEntries)
	}
	titles := strings.Join(got.Titles, ",")
	if !strings.Contains(titles, "alpha-decision") || !strings.Contains(titles, "beta-pattern") {
		t.Errorf("Titles = %v, want date-stripped alpha-decision and beta-pattern", got.Titles)
	}
	for _, ti := range got.Titles {
		if strings.HasPrefix(ti, "2026-") {
			t.Errorf("title %q retained ISO date prefix", ti)
		}
	}
}

func TestComputeReuseReceipt_EmptySession(t *testing.T) {
	base := t.TempDir()
	writeCorpusFile(t, base, "learnings", "solo.md")

	got := computeReuseReceipt(base, "")
	if got.Reused != 0 {
		t.Errorf("Reused = %d, want 0 for empty session ID", got.Reused)
	}
	if got.CorpusEntries != 1 {
		t.Errorf("CorpusEntries = %d, want 1 (corpus still counted)", got.CorpusEntries)
	}
}

func TestComputeReuseReceipt_NoLedger(t *testing.T) {
	base := t.TempDir() // no .agents at all
	got := computeReuseReceipt(base, "sess-x")
	if got.Reused != 0 || got.CorpusEntries != 0 {
		t.Errorf("got %+v, want zero receipt when no ledger/corpus exists", got)
	}
}

func TestReuseTitleFromPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/x/.agents/learnings/2026-06-06-foo-bar.md", "foo-bar"},
		{"/x/.agents/patterns/gamma.md", "gamma"},
		{"2026-01-02_underscore.md", "underscore"},
		{"/x/2026-06-06.md", "2026-06-06.md"}, // date-only basename falls back to full name
	}
	for _, c := range cases {
		if got := reuseTitleFromPath(c.in); got != c.want {
			t.Errorf("reuseTitleFromPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrintReuseReceiptLine(t *testing.T) {
	t.Run("loud when reused", func(t *testing.T) {
		out, _ := captureStdout(t, func() error {
			printReuseReceiptLine(SessionCloseResult{
				Reused:        2,
				ReusedTitles:  []string{"alpha-decision", "beta-pattern"},
				CorpusEntries: 42,
			})
			return nil
		})
		if !strings.Contains(out, "♻ Compounding") {
			t.Errorf("missing compounding marker in %q", out)
		}
		if !strings.Contains(out, "reused 2 prior learnings") {
			t.Errorf("missing reuse count phrase in %q", out)
		}
		if !strings.Contains(out, "alpha-decision") || !strings.Contains(out, "beta-pattern") {
			t.Errorf("missing reused titles in %q", out)
		}
		if !strings.Contains(out, "corpus now 42 entries") {
			t.Errorf("missing corpus size in %q", out)
		}
	})

	t.Run("singular noun for one reuse", func(t *testing.T) {
		out, _ := captureStdout(t, func() error {
			printReuseReceiptLine(SessionCloseResult{Reused: 1, CorpusEntries: 5})
			return nil
		})
		if !strings.Contains(out, "reused 1 prior learning;") {
			t.Errorf("expected singular 'learning' in %q", out)
		}
	})

	t.Run("quiet on cold run", func(t *testing.T) {
		out, _ := captureStdout(t, func() error {
			printReuseReceiptLine(SessionCloseResult{Reused: 0, CorpusEntries: 5})
			return nil
		})
		if strings.TrimSpace(out) != "" {
			t.Errorf("expected no output when Reused=0, got %q", out)
		}
	})
}

func TestPrintCloseTable_ShowsReusedLine(t *testing.T) {
	out, _ := captureStdout(t, func() error {
		printCloseTable(SessionCloseResult{
			SessionID:     "sess-abc",
			Transcript:    "/tmp/t.jsonl",
			Knowledge:     1,
			Reused:        3,
			ReusedTitles:  []string{"alpha"},
			CorpusEntries: 10,
			Status:        "compounding",
			Message:       "done",
		})
		return nil
	})
	if !strings.Contains(out, "Reused:        3") {
		t.Errorf("missing Reused table line in %q", out)
	}
	if !strings.Contains(out, "♻ Compounding") {
		t.Errorf("missing compounding receipt in %q", out)
	}
}
