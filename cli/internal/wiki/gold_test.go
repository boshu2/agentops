package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAgent(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestCompiler(t *testing.T) (*GoldCompiler, string) {
	t.Helper()
	base := t.TempDir()
	agents := filepath.Join(base, ".agents")
	out := filepath.Join(base, ".ao", "wiki")
	gc := &GoldCompiler{
		AgentsDir: agents,
		OutDir:    out,
		Now:       func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) },
	}
	return gc, agents
}

func TestGold_PromotionGate(t *testing.T) {
	gc, agents := newTestCompiler(t)
	// durable: established maturity
	writeAgent(t, filepath.Join(agents, "learnings"), "good.md",
		"---\ntype: learning\nid: keep-me\nmaturity: established\nconfidence: 0.9\n---\n\nThis is a durable, reviewed lesson worth keeping in the gold layer forever.\n")
	// noise: provisional
	writeAgent(t, filepath.Join(agents, "learnings"), "noise.md",
		"---\ntype: learning\nid: drop-me\nmaturity: provisional\nconfidence: 0.5\n---\n\nThis provisional capture should be gated out of the gold layer entirely.\n")
	// below floor: no maturity, low confidence
	writeAgent(t, filepath.Join(agents, "findings"), "weak.md",
		"---\ntype: finding\nid: weak\nconfidence: 0.4\n---\n\nLow-confidence finding that has not earned promotion into the durable wiki.\n")

	stats, err := gc.Compile(false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Promoted != 1 {
		t.Errorf("promoted = %d, want 1", stats.Promoted)
	}
	if stats.Rejected != 2 {
		t.Errorf("rejected = %d, want 2", stats.Rejected)
	}
	if !fileExists(filepath.Join(gc.OutDir, "learnings", "keep-me.md")) {
		t.Error("durable entry not written to gold")
	}
	if fileExists(filepath.Join(gc.OutDir, "learnings", "drop-me.md")) {
		t.Error("provisional noise leaked into gold")
	}
}

func TestGold_Sanitize(t *testing.T) {
	gc, agents := newTestCompiler(t)
	// realistic-length anthropic key (matches the canonical sk-ant-{40,} rule)
	key := "sk-ant-" + strings.Repeat("a", 48)
	body := "ran from /Users/secretuser/dev and session 12345678-1234-1234-1234-123456789abc with key " + key
	writeAgent(t, filepath.Join(agents, "findings"), "leak.md",
		"---\ntype: finding\nid: leak\ntier: gold\n---\n\n"+body+"\n")

	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(gc.OutDir, "findings", "leak.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	for _, leak := range []string{"secretuser", key, "12345678-1234"} {
		if strings.Contains(out, leak) {
			t.Errorf("sanitize failed to scrub %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "[session]") {
		t.Error("session UUID not scrubbed to [session]")
	}
}

func TestGold_OKFConformance(t *testing.T) {
	gc, agents := newTestCompiler(t)
	// a title with a colon — the case that corrupted frontmatter before quoting
	writeAgent(t, filepath.Join(agents, "findings"), "colon.md",
		"---\ntype: finding\nid: null\nmaturity: canonical\n---\n\nFinding: gates must fail closed: an unprovable condition is a failure.\n")

	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	// must NOT be slugged "null"
	if fileExists(filepath.Join(gc.OutDir, "findings", "null.md")) {
		t.Error("null id sentinel was used as a slug")
	}
	// reserved OKF files present
	for _, f := range []string{"index.md", "log.md"} {
		if !fileExists(filepath.Join(gc.OutDir, f)) {
			t.Errorf("missing OKF reserved file %s", f)
		}
	}
	// every emitted doc must parse and carry the required `type`
	codec := NewFrontmatterCodec()
	mds, _ := collectMarkdown(filepath.Join(gc.OutDir, "findings"))
	for _, md := range mds {
		if filepath.Base(md) == "index.md" {
			continue
		}
		raw, _ := os.ReadFile(md)
		doc := codec.Decode(string(raw))
		if !doc.HasFrontmatter {
			t.Errorf("%s: emitted frontmatter does not parse (invalid YAML)", md)
		}
		if fieldStr(doc.Fields, "type") == "" {
			t.Errorf("%s: missing required OKF `type` field", md)
		}
	}
	// lint must be clean
	stats, _ := gc.Compile(false)
	if len(stats.Lint) != 0 {
		t.Errorf("lint not clean: %v", stats.Lint)
	}
}

func TestGold_Idempotent(t *testing.T) {
	gc, agents := newTestCompiler(t)
	writeAgent(t, filepath.Join(agents, "patterns"), "p.md",
		"---\ntype: pattern\nid: keeper\ntier: gold\n---\n\nA durable reusable pattern that belongs in the gold layer across rebuilds.\n")
	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	// stale file from a "previous" run that no longer has a source
	stale := filepath.Join(gc.OutDir, "patterns", "stale.md")
	_ = os.WriteFile(stale, []byte("stale"), 0o644)
	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	if fileExists(stale) {
		t.Error("idempotent rebuild did not clear stale output")
	}
}
