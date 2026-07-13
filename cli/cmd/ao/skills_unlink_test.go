package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The command's --help is the user-facing contract for the documented rollback
// path: it must name the inverse relationship, the multi-runtime coverage, and
// the dry-run rehearsal. Guard against silent removal.
func TestSkillsUnlinkHelp_DocumentsRollbackAndRuntimes(t *testing.T) {
	long := skillsUnlinkCmd.Long
	for _, want := range []string{"inverse", "track main", "~/.codex/skills", "~/.gemini/skills", "--dry-run"} {
		if !strings.Contains(long, want) {
			t.Errorf("`ao skills unlink --help` no longer documents %q", want)
		}
	}
}

// TestSkillsUnlink_RemovesOnlyOwnLinks is the core acceptance test: after a
// round-trip (link → unlink) the runtime is restored to its pre-link state,
// while a foreign symlink pointing elsewhere and a real foreign-corpus directory
// both survive untouched.
func TestSkillsUnlink_RemovesOnlyOwnLinks(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "goal-design")
	mkSkill(t, src, "using-gc")

	// Mint our own live-tier links exactly as `ao skills link` would.
	if _, err := linkMissingSkills(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}

	// A foreign symlink pointing OUTSIDE the repo (e.g. a hand-linked skill from
	// another corpus) must be left alone.
	otherSrc := t.TempDir()
	mkSkill(t, otherSrc, "foreign-skill")
	foreignLink := filepath.Join(dest, "foreign-skill")
	if err := os.Symlink(filepath.Join(otherSrc, "foreign-skill"), foreignLink); err != nil {
		t.Fatalf("make foreign symlink: %v", err)
	}

	// A real directory (a foreign corpus such as jsm) must be left alone.
	realDir := filepath.Join(dest, "jsm-corpus")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir foreign corpus: %v", err)
	}
	sentinel := filepath.Join(realDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("jsm"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	res, err := unlinkOwnedSkills(src, dest, false)
	if err != nil {
		t.Fatalf("unlinkOwnedSkills: %v", err)
	}

	// Exactly our two links were removed.
	if len(res.Removed) != 2 || res.Removed[0] != "goal-design" || res.Removed[1] != "using-gc" {
		t.Fatalf("Removed = %v, want [goal-design using-gc]", res.Removed)
	}
	// The foreign symlink and the real dir are both reported as foreign.
	wantForeign := map[string]bool{"foreign-skill": true, "jsm-corpus": true}
	if len(res.Foreign) != 2 {
		t.Fatalf("Foreign = %v, want the 2 foreign entries", res.Foreign)
	}
	for _, f := range res.Foreign {
		if !wantForeign[f] {
			t.Fatalf("unexpected foreign entry %q (Foreign=%v)", f, res.Foreign)
		}
	}

	// Our links are gone from disk.
	for _, name := range []string{"goal-design", "using-gc"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Fatalf("owned link %q not removed; Lstat err = %v, want IsNotExist", name, err)
		}
	}
	// The foreign symlink survives, still a symlink.
	fi, err := os.Lstat(foreignLink)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("foreign symlink was removed or changed; Lstat=%v mode=%v", err, fi.Mode())
	}
	// The real dir + its sentinel survive untouched.
	di, err := os.Lstat(realDir)
	if err != nil || di.Mode()&os.ModeSymlink != 0 || !di.IsDir() {
		t.Fatalf("foreign dir was replaced; Lstat=%v mode=%v", err, di.Mode())
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "jsm" {
		t.Fatalf("sentinel clobbered; b=%q err=%v", b, err)
	}
}

// TestSkillsUnlink_DryRunWritesNothing: dry-run reports the would-be removals but
// leaves every link on disk.
func TestSkillsUnlink_DryRunWritesNothing(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "gc-membrane")
	if _, err := linkMissingSkills(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}

	res, err := unlinkOwnedSkills(src, dest, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(res.Removed) != 1 || res.Removed[0] != "gc-membrane" {
		t.Fatalf("dry-run Removed = %v, want [gc-membrane]", res.Removed)
	}
	if !res.DryRun {
		t.Fatalf("res.DryRun = false, want true")
	}
	// The link is still on disk.
	if _, err := os.Lstat(filepath.Join(dest, "gc-membrane")); err != nil {
		t.Fatalf("dry-run removed the link; Lstat err = %v, want nil", err)
	}
}

// TestSkillsUnlink_Idempotent: a second sweep removes nothing and does not error.
func TestSkillsUnlink_Idempotent(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "using-gc")
	if _, err := linkMissingSkills(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}

	if _, err := unlinkOwnedSkills(src, dest, false); err != nil {
		t.Fatalf("first unlink: %v", err)
	}
	res, err := unlinkOwnedSkills(src, dest, false)
	if err != nil {
		t.Fatalf("second unlink: %v", err)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("second run Removed = %v, want [] (idempotent)", res.Removed)
	}
	if len(res.Foreign) != 0 {
		t.Fatalf("second run Foreign = %v, want []", res.Foreign)
	}
}

// A missing destination dir (runtime never installed) is a clean no-op, not an
// error — unlink is safe to run against any subset of runtimes.
func TestSkillsUnlink_MissingDestIsNoop(t *testing.T) {
	src := t.TempDir()
	mkSkill(t, src, "goal-design")
	dest := filepath.Join(t.TempDir(), "does-not-exist")

	res, err := unlinkOwnedSkills(src, dest, false)
	if err != nil {
		t.Fatalf("missing dest should be a no-op, got err: %v", err)
	}
	if len(res.Removed) != 0 || len(res.Foreign) != 0 {
		t.Fatalf("missing dest should report nothing, got %+v", res)
	}
}

// A stale link — one pointing into the repo skills/ tree at a skill that no
// longer exists there — is still ours to remove. The target need not resolve.
func TestSkillsUnlink_RemovesStaleOwnedLink(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	// Link points into src but the target skill dir was never created (removed
	// from the repo since it was linked).
	stale := filepath.Join(dest, "retired-skill")
	if err := os.Symlink(filepath.Join(src, "retired-skill"), stale); err != nil {
		t.Fatalf("make stale link: %v", err)
	}

	res, err := unlinkOwnedSkills(src, dest, false)
	if err != nil {
		t.Fatalf("unlinkOwnedSkills: %v", err)
	}
	if len(res.Removed) != 1 || res.Removed[0] != "retired-skill" {
		t.Fatalf("Removed = %v, want [retired-skill]", res.Removed)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale owned link not removed; err = %v, want IsNotExist", err)
	}
}

// Cross-family refuter regression, mirror of the link guard (age-u031): an empty
// source dir must fail CLOSED — never fall through to filepath.Abs("")→cwd and
// wrongly claim links pointing into cwd as owned, then delete them.
func TestSkillsUnlink_EmptySrcFailsClosed(t *testing.T) {
	dest := t.TempDir()
	res, err := unlinkOwnedSkills("", dest, false)
	if err == nil {
		t.Fatalf("empty srcDir must fail closed with an error, got nil (res=%+v)", res)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("empty srcDir must remove nothing, got Removed=%v", res.Removed)
	}
	if _, werr := unlinkOwnedSkills("   ", dest, false); werr == nil {
		t.Fatal("whitespace-only srcDir must fail closed with an error, got nil")
	}
}

// The fan-out must be RESILIENT — a per-dest failure records an error on that
// dest but must NOT abort the loop, so a failing runtime (listed first) never
// skips the ones after it. Mirror of TestLinkAllDests_ResilientAcrossDests.
func TestUnlinkAllDests_ResilientAcrossDests(t *testing.T) {
	src := t.TempDir()
	mkSkill(t, src, "alpha")

	good := t.TempDir()
	if _, err := linkMissingSkills(src, good, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}

	// A bad dest that is a regular FILE (not a dir) → os.ReadDir fails with a
	// non-IsNotExist error.
	badFile := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(badFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write bad dest file: %v", err)
	}

	// bad FIRST: the good dest after it must still be swept.
	results, anyErr := unlinkAllDests(src, []string{badFile, good}, false)

	if !anyErr {
		t.Fatal("anyErr should be true when a dest fails")
	}
	if len(results) != 2 {
		t.Fatalf("want 2 per-dest results (never skip), got %d", len(results))
	}
	if results[0].Err == "" {
		t.Fatalf("the failing dest should carry Err, got %+v", results[0])
	}
	if results[1].Err != "" {
		t.Fatalf("the good dest should succeed despite the earlier failure, got Err=%q", results[1].Err)
	}
	if len(results[1].Removed) != 1 || results[1].Removed[0] != "alpha" {
		t.Fatalf("good dest Removed = %v, want [alpha]", results[1].Removed)
	}
	if _, err := os.Lstat(filepath.Join(good, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("good dest was skipped after the earlier failure (link still present): %v", err)
	}
}
