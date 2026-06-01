package warmind

import (
	"os"
	"runtime"
	"testing"
)

// devFull is a Linux character device whose every write fails with ENOSPC,
// while open/create succeeds. It lets us deterministically exercise the
// write-error paths that ag-dnpk fixed (previously these errors were swallowed,
// silently losing team-knowledge records).
const devFull = "/dev/full"

func requireDevFull(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("write-failure regression test requires /dev/full (Linux only)")
	}
	if fi, err := os.Stat(devFull); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("/dev/full not available")
	}
}

func TestAppendCitation_PropagatesWriteError(t *testing.T) {
	requireDevFull(t)
	ct := &CitationTracker{CitationsFile: devFull}
	if err := ct.appendCitation(Citation{ArtifactID: "a", CitedBy: "tester"}); err == nil {
		t.Fatal("expected appendCitation to return an error on a failed write, got nil")
	}
}

func TestCitationsRewriteAll_PropagatesWriteError(t *testing.T) {
	requireDevFull(t)
	ct := &CitationTracker{CitationsFile: devFull}
	if err := ct.rewriteAll([]Citation{{ArtifactID: "a"}}); err == nil {
		t.Fatal("expected rewriteAll to return an error on a failed write, got nil")
	}
}

func TestAppendContradiction_PropagatesWriteError(t *testing.T) {
	requireDevFull(t)
	cd := &ContradictionDetector{ContradictionsFile: devFull}
	if err := cd.appendContradiction(Contradiction{ID: "c1"}); err == nil {
		t.Fatal("expected appendContradiction to return an error on a failed write, got nil")
	}
}

func TestContradictionsRewriteAll_PropagatesWriteError(t *testing.T) {
	requireDevFull(t)
	cd := &ContradictionDetector{ContradictionsFile: devFull}
	if err := cd.rewriteAll([]Contradiction{{ID: "c1"}}); err == nil {
		t.Fatal("expected rewriteAll to return an error on a failed write, got nil")
	}
}
