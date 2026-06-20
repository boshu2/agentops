package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/corpusscan"
)

// TestCompilePublishCandidate_StableContentDigest is the core of the
// `ao wiki publish --dry-run` slice (age-port-openkb-into-agentops-go-5qw.9):
// the same corpus must always yield the SAME digest (the key the verdict gate
// binds to), independent of the day it is compiled.
func TestCompilePublishCandidate_StableContentDigest(t *testing.T) {
	base := t.TempDir()
	agents := filepath.Join(base, ".agents")
	// A durable, undated finding — the date would default to now() and drift the
	// digest if the clock weren't pinned.
	writeAgent(t, filepath.Join(agents, "findings"), "keep.md",
		"---\ntype: finding\nid: keep\nmaturity: established\nconfidence: 0.9\n---\n\nGates must fail closed: an unprovable condition is treated as a failure.\n")

	c1, err := CompilePublishCandidate(agents, 0)
	if err != nil {
		t.Fatalf("compile 1: %v", err)
	}
	defer c1.Cleanup()
	c2, err := CompilePublishCandidate(agents, 0)
	if err != nil {
		t.Fatalf("compile 2: %v", err)
	}
	defer c2.Cleanup()

	if c1.Digest == "" || len(c1.Digest) != 64 {
		t.Errorf("digest %q is not a 64-char sha256 hex", c1.Digest)
	}
	if c1.Digest != c2.Digest {
		t.Errorf("digest not stable across runs:\n  %s\n  %s", c1.Digest, c2.Digest)
	}
	if c1.Stats.Promoted < 1 {
		t.Errorf("expected the durable finding to promote, Promoted=%d", c1.Stats.Promoted)
	}
	// The candidate tree must exist and be leak-clean for this safe fixture.
	rep, err := corpusscan.Scan(c1.OutDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !rep.Clean() {
		t.Errorf("safe fixture scanned dirty: %d hit(s)", rep.HitCount())
	}
}

// TestCompilePublishCandidate_DigestTracksContent confirms the digest changes
// when the promoted content changes (it is a real content identity, not a const).
func TestCompilePublishCandidate_DigestTracksContent(t *testing.T) {
	mk := func(body string) string {
		base := t.TempDir()
		agents := filepath.Join(base, ".agents")
		writeAgent(t, filepath.Join(agents, "findings"), "f.md",
			"---\ntype: finding\nid: f\nmaturity: established\nconfidence: 0.9\n---\n\n"+body+"\n")
		c, err := CompilePublishCandidate(agents, 0)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		defer c.Cleanup()
		return c.Digest
	}
	a := mk("Gates must fail closed when a condition cannot be proven true at all.")
	b := mk("Retrieval ranking must weight utility so reused findings surface first.")
	if a == b {
		t.Error("digest did not change when promoted content changed")
	}
}

// TestCompilePublishCandidate_LeakSurvivesToScan proves the second-layer leak
// scan catches a REAL private span that sanitize() does NOT handle: sanitize
// only scrubs secrets/$HOME/UUIDs, but the corpusscan registry guards
// fleet/brand/myth markers (e.g. "shield", "AgentOps"). A durable finding whose
// body carries such a marker promotes, survives sanitize, and lands in the gold
// tree — where the publish scan must flag it. (bead risk note: test the leak
// scan with a real private span, not a synthetic one.)
func TestCompilePublishCandidate_LeakSurvivesToScan(t *testing.T) {
	base := t.TempDir()
	agents := filepath.Join(base, ".agents")
	writeAgent(t, filepath.Join(agents, "findings"), "leaky.md",
		"---\ntype: finding\nid: leaky\nmaturity: established\nconfidence: 0.9\n---\n\nThe shield cluster topology must never appear in published gold output.\n")

	cand, err := CompilePublishCandidate(agents, 0)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer cand.Cleanup()

	rep, err := corpusscan.Scan(cand.OutDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if rep.Clean() {
		t.Fatal("leak scan reported CLEAN, but the gold tree carries a fleet marker — fail-closed broken")
	}
}

// TestCompilePublishCandidate_Cleanup confirms the temp tree is removed.
func TestCompilePublishCandidate_Cleanup(t *testing.T) {
	base := t.TempDir()
	agents := filepath.Join(base, ".agents")
	writeAgent(t, filepath.Join(agents, "findings"), "f.md",
		"---\ntype: finding\nid: f\nmaturity: established\nconfidence: 0.9\n---\n\nA durable finding worth keeping in the gold layer for reuse.\n")
	c, err := CompilePublishCandidate(agents, 0)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.Contains(c.OutDir, "ao-wiki-publish-") {
		t.Errorf("unexpected temp dir %q", c.OutDir)
	}
	c.Cleanup()
	if _, err := os.Stat(c.OutDir); !os.IsNotExist(err) {
		t.Errorf("Cleanup did not remove %s", c.OutDir)
	}
}
