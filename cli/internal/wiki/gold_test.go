package wiki

import (
	"context"
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

func TestGold_CrossLinks(t *testing.T) {
	gc, agents := newTestCompiler(t)
	// A links B (resolvable) and a dangling target (unresolvable).
	writeAgent(t, filepath.Join(agents, "learnings"), "a.md",
		"---\ntype: learning\nid: alpha\nmaturity: established\n---\n\nAlpha lesson references [[beta]] and a missing [[ghost-entry]] worth noting here.\n")
	writeAgent(t, filepath.Join(agents, "patterns"), "b.md",
		"---\ntype: pattern\nid: beta\nmaturity: established\n---\n\nBeta is a durable reusable pattern that other entries point to across the wiki.\n")

	stats, err := gc.Compile(false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Links != 1 {
		t.Errorf("links woven = %d, want 1 (alpha->beta)", stats.Links)
	}
	a, err := os.ReadFile(filepath.Join(gc.OutDir, "learnings", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(a)
	// resolved [[beta]] -> cross-category relative markdown link
	if !strings.Contains(out, "(../patterns/beta.md)") {
		t.Errorf("resolved cross-link missing relative path:\n%s", out)
	}
	// unresolved [[ghost-entry]] flattened (no dead wiki-syntax)
	if strings.Contains(out, "[[") {
		t.Errorf("unresolved wikilink not flattened:\n%s", out)
	}
	// a Related section listing the edge
	if !strings.Contains(out, "## Related") {
		t.Errorf("missing Related section:\n%s", out)
	}
}

func TestGold_RelatedFrontmatter(t *testing.T) {
	gc, agents := newTestCompiler(t)
	writeAgent(t, filepath.Join(agents, "learnings"), "x.md",
		"---\ntype: learning\nid: ecks\nmaturity: established\nrelated: why-zed\n---\n\nEcks lesson with no body wikilinks but an explicit related frontmatter ref.\n")
	writeAgent(t, filepath.Join(agents, "learnings"), "z.md",
		"---\ntype: learning\nid: why-zed\nmaturity: established\n---\n\nZed is the durable lesson that ecks points to via a frontmatter cross-ref.\n")

	stats, err := gc.Compile(false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Links != 1 {
		t.Errorf("links = %d, want 1 (related frontmatter ecks->why-zed)", stats.Links)
	}
	out, _ := os.ReadFile(filepath.Join(gc.OutDir, "learnings", "ecks.md"))
	if !strings.Contains(string(out), "## Related") || !strings.Contains(string(out), "why-zed.md") {
		t.Errorf("frontmatter related ref not woven into Related:\n%s", out)
	}
}

func TestGold_TruncateWords(t *testing.T) {
	got := truncateWords("alpha beta gamma delta epsilon zeta", 20)
	if strings.Contains(got, "delt") && !strings.HasSuffix(got, "…") {
		t.Errorf("truncateWords cut mid-word: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateWords should append ellipsis: %q", got)
	}
	if len("alpha beta") > 20 && !strings.HasPrefix(got, "alpha") {
		t.Errorf("truncateWords lost the head: %q", got)
	}
	// short strings pass through untouched
	if truncateWords("short", 20) != "short" {
		t.Error("short string should be unchanged")
	}
}

func TestGold_IndexForRoots(t *testing.T) {
	gc, agents := newTestCompiler(t)
	writeAgent(t, filepath.Join(agents, "learnings"), "g.md",
		"---\ntype: learning\nid: goldidx\nmaturity: established\n---\n\nGold entry about idempotent ratchet behavior that retrieval should find.\n")
	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	// retrieval points at the gold dir, not .agents
	idx, err := NewWikiIndexForRoots(filepath.Join(gc.OutDir, "wiki-index.jsonl"), gc.OutDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Reindex(context.Background()); err != nil {
		t.Fatal(err)
	}
	recs, err := idx.Records()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range recs {
		if strings.Contains(r.Path, "goldidx.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("gold doc not indexed via NewWikiIndexForRoots; got %d records", len(recs))
	}
}

func TestGold_DedupeSections(t *testing.T) {
	gc, agents := newTestCompiler(t)
	body := "# Finding: CLI bridge without version pinning\n\n" +
		"## Summary\nCLI bridge without version pinning\n\n" +
		"## Pattern\nCLI bridge without version pinning\n\n" +
		"## Detection Question\nDoes the plan pin the external CLI schema version?\n\n" +
		"## Source\n- Skill: finding-compiler\n- Artifact: .agents/findings/x.md\n"
	writeAgent(t, filepath.Join(agents, "findings"), "x.md",
		"---\ntype: finding\nid: cli-bridge\nmaturity: established\n---\n\n"+body)

	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(gc.OutDir, "findings", "cli-bridge.md"))
	s := string(out)
	// unique content preserved
	if !strings.Contains(s, "Detection Question") || !strings.Contains(s, "pin the external CLI") {
		t.Errorf("unique section dropped:\n%s", s)
	}
	// redundant echoes + Source block removed
	if strings.Contains(s, "## Summary") || strings.Contains(s, "## Pattern") {
		t.Errorf("title-echo sections not removed:\n%s", s)
	}
	if strings.Contains(s, "## Source") || strings.Contains(s, "finding-compiler") {
		t.Errorf("redundant Source block not removed:\n%s", s)
	}
}

func TestGold_CatalogQuality(t *testing.T) {
	gc, agents := newTestCompiler(t)
	writeAgent(t, filepath.Join(agents, "learnings"), "auth.md",
		"---\ntype: learning\nid: auth-one\nmaturity: canonical\n---\n\nAuthoritative durable lesson that should sort first in the OKF catalog index.\n")
	writeAgent(t, filepath.Join(agents, "learnings"), "draft.md",
		"---\ntype: learning\nid: draft-one\nconfidence: 0.72\n---\n\nA draft-status lesson promoted on confidence floor that should sort after authoritative.\n")
	if _, err := gc.Compile(false); err != nil {
		t.Fatal(err)
	}
	cat, _ := os.ReadFile(filepath.Join(gc.OutDir, "learnings", "index.md"))
	s := string(cat)
	if !strings.Contains(s, "authoritative. OKF catalog") {
		t.Errorf("catalog missing authoritative count:\n%s", s)
	}
	// authoritative entry must appear before the draft entry
	ai := strings.Index(s, "auth-one.md")
	di := strings.Index(s, "draft-one.md")
	if ai < 0 || di < 0 || ai > di {
		t.Errorf("authoritative entry not sorted first (ai=%d di=%d):\n%s", ai, di, s)
	}
	// description line present (progressive disclosure)
	if !strings.Contains(s, "Authoritative durable lesson") {
		t.Errorf("catalog missing description line:\n%s", s)
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
