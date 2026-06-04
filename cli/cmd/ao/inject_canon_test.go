// practices: [wiki-knowledge-surface, knowledge-flywheel]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeCanonInjectFixture builds a workspace with one local learning and one
// canon learning of equal utility, returning the workspace root.
func writeCanonInjectFixture(t *testing.T, localUtility, canonUtility string) string {
	t.Helper()
	root := t.TempDir()

	localDir := filepath.Join(root, ".agents", "learnings")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	local := "---\nutility: " + localUtility + "\nmaturity: provisional\n---\n# Local Note\n\nLocal content describing a provisional learning used for canon-tier inject tests.\n"
	if err := os.WriteFile(filepath.Join(localDir, "local.md"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}

	canonDir := filepath.Join(root, ".agents", "canon", "learnings")
	if err := os.MkdirAll(canonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canon := "---\nutility: " + canonUtility + "\nmaturity: provisional\n---\n# Canon Note\n\nEarned team-canon content describing a verified learning surfaced by inject.\n"
	if err := os.WriteFile(filepath.Join(canonDir, "canon.md"), []byte(canon), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestCollectLearnings_CanonTierSurfaces verifies canon entries are collected
// and flagged distinctly from local entries.
func TestCollectLearnings_CanonTierSurfaces(t *testing.T) {
	root := writeCanonInjectFixture(t, "0.8", "0.8")

	learnings, err := collectLearnings(root, "", 10, "", 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings (local + canon), got %d", len(learnings))
	}

	var foundLocal, foundCanon bool
	for _, l := range learnings {
		if l.Canon {
			foundCanon = true
		} else {
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Error("expected a local (non-canon) learning")
	}
	if !foundCanon {
		t.Error("expected a canon-flagged learning")
	}
}

// TestCollectLearnings_CanonBoost verifies that a canon entry outranks a local
// entry of identical utility — canon is the verification-earned trusted tier,
// so it is boosted, not penalized like the global mirror.
func TestCollectLearnings_CanonBoost(t *testing.T) {
	root := writeCanonInjectFixture(t, "0.8", "0.8")

	learnings, err := collectLearnings(root, "", 10, "", 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}

	var localScore, canonScore float64
	for _, l := range learnings {
		if l.Canon {
			canonScore = l.CompositeScore
		} else {
			localScore = l.CompositeScore
		}
	}
	if canonScore <= localScore {
		t.Errorf("canon score %.4f should exceed equal-utility local score %.4f (boost not applied)", canonScore, localScore)
	}
	// And the boosted canon entry must rank first.
	if !learnings[0].Canon {
		t.Errorf("expected canon entry ranked first, got %q (canon=%v)", learnings[0].Title, learnings[0].Canon)
	}
}

// TestCollectLearnings_CanonAbsentDirNoop verifies inject works unchanged when
// no canon tier exists.
func TestCollectLearnings_CanonAbsentDirNoop(t *testing.T) {
	root := t.TempDir()
	localDir := filepath.Join(root, ".agents", "learnings")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nutility: 0.8\nmaturity: provisional\n---\n# Only Local\n\nLocal content present without any canon tier directory on disk at all.\n"
	if err := os.WriteFile(filepath.Join(localDir, "only.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(root, "", 10, "", 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}
	if len(learnings) != 1 || learnings[0].Canon {
		t.Fatalf("expected 1 non-canon learning, got %d (canon=%v)", len(learnings), len(learnings) > 0 && learnings[0].Canon)
	}
}

func TestCollectLearnings_ComputesAlwaysOnlyForEstablishedCanon(t *testing.T) {
	root := t.TempDir()
	localDir := filepath.Join(root, ".agents", "learnings")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	local := "---\nutility: 0.8\nmaturity: established\nreach: always\n---\n# Local Established\n\nLocal established content tries to self-author always reach without canon.\n"
	if err := os.WriteFile(filepath.Join(localDir, "local.md"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}

	canonDir := filepath.Join(root, ".agents", "canon", "learnings")
	if err := os.MkdirAll(canonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonEstablished := "---\nutility: 0.8\nmaturity: established\n---\n# Canon Established\n\nCanon established content earned the computed always reach projection.\n"
	if err := os.WriteFile(filepath.Join(canonDir, "canon-established.md"), []byte(canonEstablished), 0o644); err != nil {
		t.Fatal(err)
	}
	canonCandidate := "---\nutility: 0.8\nmaturity: candidate\nreach: always\n---\n# Canon Candidate\n\nCanon candidate content cannot self-author always reach before establishment.\n"
	if err := os.WriteFile(filepath.Join(canonDir, "canon-candidate.md"), []byte(canonCandidate), 0o644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(root, "", 10, "", 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}
	if len(learnings) != 3 {
		t.Fatalf("expected 3 learnings, got %d", len(learnings))
	}

	byTitle := map[string]learning{}
	for _, l := range learnings {
		byTitle[l.Title] = l
	}
	if got := byTitle["Local Established"].Reach; got != "pull" {
		t.Fatalf("local authored reach=always computed to %q, want pull", got)
	}
	if got := byTitle["Canon Established"].Reach; got != "always" {
		t.Fatalf("canon established reach = %q, want always", got)
	}
	if got := byTitle["Canon Candidate"].Reach; got != "pull" {
		t.Fatalf("canon candidate authored reach=always computed to %q, want pull", got)
	}
}
