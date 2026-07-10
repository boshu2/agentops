package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The command's --help is the user-facing contract for the optional track-main
// install path (age-4asp): documenting it there is the whole point, so guard it
// against silent removal.
func TestSkillsLinkHelp_DocumentsTrackMain(t *testing.T) {
	long := skillsLinkCmd.Long
	for _, want := range []string{"Track main", "git pull && ao skills link"} {
		if !strings.Contains(long, want) {
			t.Errorf("`ao skills link --help` no longer documents the track-main workflow: missing %q", want)
		}
	}
}

// mkSkill creates dir/<name>/SKILL.md so linkMissingSkills recognizes it as a skill.
func mkSkill(t *testing.T, dir, name string) {
	t.Helper()
	sd := filepath.Join(dir, name)
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", sd, err)
	}
	if err := os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestLinkMissingSkills_LinksAbsentAndResolves(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "goal-design")

	res, err := linkMissingSkills(src, dest, false)
	if err != nil {
		t.Fatalf("linkMissingSkills: %v", err)
	}
	if len(res.Linked) != 1 || res.Linked[0] != "goal-design" {
		t.Fatalf("Linked = %v, want [goal-design]", res.Linked)
	}

	tgt := filepath.Join(dest, "goal-design")
	got, err := os.Readlink(tgt)
	if err != nil {
		t.Fatalf("expected a symlink at %s: %v", tgt, err)
	}
	want := filepath.Join(src, "goal-design")
	if got != want {
		t.Fatalf("symlink target = %q, want %q", got, want)
	}
	// The linked skill's SKILL.md must be reachable THROUGH the link.
	if _, err := os.Stat(filepath.Join(tgt, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not reachable through link: %v", err)
	}
}

func TestLinkMissingSkills_Idempotent(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "using-gc")

	if _, err := linkMissingSkills(src, dest, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := linkMissingSkills(src, dest, false)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("second run Linked = %v, want []", res.Linked)
	}
	if len(res.Present) != 1 || res.Present[0] != "using-gc" {
		t.Fatalf("second run Present = %v, want [using-gc]", res.Present)
	}
}

func TestLinkMissingSkills_DryRunDoesNotWrite(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "gc-membrane")

	res, err := linkMissingSkills(src, dest, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(res.Linked) != 1 || res.Linked[0] != "gc-membrane" {
		t.Fatalf("dry-run Linked = %v, want [gc-membrane]", res.Linked)
	}
	if !res.DryRun {
		t.Fatalf("res.DryRun = false, want true")
	}
	if _, err := os.Lstat(filepath.Join(dest, "gc-membrane")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a link; Lstat err = %v, want IsNotExist", err)
	}
}

func TestLinkMissingSkills_RealDirIsConflictNotClobbered(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "multi-pass-bug-hunting")
	// Simulate a foreign corpus (jsm) real dir already owning the name.
	foreign := filepath.Join(dest, "multi-pass-bug-hunting")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("mkdir foreign: %v", err)
	}
	sentinel := filepath.Join(foreign, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("jsm"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	res, err := linkMissingSkills(src, dest, false)
	if err != nil {
		t.Fatalf("linkMissingSkills: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "multi-pass-bug-hunting" {
		t.Fatalf("Conflicts = %v, want [multi-pass-bug-hunting]", res.Conflicts)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("Linked = %v, want [] (must not clobber)", res.Linked)
	}
	// The foreign dir + its contents must be untouched.
	info, err := os.Lstat(foreign)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("foreign dir was replaced; Lstat=%v mode=%v", err, info.Mode())
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "jsm" {
		t.Fatalf("sentinel clobbered; b=%q err=%v", b, err)
	}
}

func TestLinkMissingSkills_SkipsNonSkillDirs(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "real-skill")
	// A directory with no SKILL.md must be ignored, not linked.
	if err := os.MkdirAll(filepath.Join(src, "not-a-skill"), 0o755); err != nil {
		t.Fatalf("mkdir not-a-skill: %v", err)
	}

	res, err := linkMissingSkills(src, dest, false)
	if err != nil {
		t.Fatalf("linkMissingSkills: %v", err)
	}
	if len(res.Linked) != 1 || res.Linked[0] != "real-skill" {
		t.Fatalf("Linked = %v, want [real-skill] only", res.Linked)
	}
	if _, err := os.Lstat(filepath.Join(dest, "not-a-skill")); !os.IsNotExist(err) {
		t.Fatalf("non-skill dir was linked; err = %v, want IsNotExist", err)
	}
}

// Cross-family refuter regression (codex-fresh-review, age-u031): an empty
// source dir must fail CLOSED, never fall through to filepath.Abs("")→cwd and
// scan/link an unrelated directory.
func TestLinkMissingSkills_EmptySrcFailsClosed(t *testing.T) {
	dest := t.TempDir()
	res, err := linkMissingSkills("", dest, false)
	if err == nil {
		t.Fatalf("empty srcDir must fail closed with an error, got nil (res=%+v)", res)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("empty srcDir must link nothing, got Linked=%v", res.Linked)
	}
	// A whitespace-only path is equally unresolved and must also fail closed.
	if _, werr := linkMissingSkills("   ", dest, false); werr == nil {
		t.Fatal("whitespace-only srcDir must fail closed with an error, got nil")
	}
}

// Cross-family refuter regression, round 2 (codex-fresh-review, age-u031): the
// identity check must reject a directory that merely CONTAINS a stray skills/
// subdir but is not the agentops repo (no skills-codex/ sibling), rather than
// scanning that unintended tree. t.Chdir auto-restores cwd on cleanup.
func TestResolveRepoSkillsDir_OutsideRepoFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir stray skills/: %v", err)
	}
	t.Chdir(tmp)
	if got, err := resolveRepoSkillsDir(); err == nil {
		t.Fatalf("outside the repo must fail closed, got dir=%q nil error", got)
	}
}

// Cross-family refuter regression, round 3 (codex-fresh-review, age-u031): a
// look-alike directory with BOTH skills/ and skills-codex/ but WITHOUT the
// agentops repo-root markers must still fail closed — shape is not identity.
func TestResolveRepoSkillsDir_LookAlikeWithoutMarkersFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	for _, d := range []string{"skills", "skills-codex"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	t.Chdir(tmp)
	if got, err := resolveRepoSkillsDir(); err == nil {
		t.Fatalf("a skills/+skills-codex/ look-alike without agentops root markers must fail closed, got dir=%q nil error", got)
	}
}

// Inside the real repo (the test binary runs under cli/cmd/ao), the resolver
// locates the skills/+skills-codex/ pair and returns an absolute path.
func TestResolveRepoSkillsDir_InsideRepoResolvesAbsolute(t *testing.T) {
	dir, err := resolveRepoSkillsDir()
	if err != nil {
		t.Fatalf("inside the agentops repo: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("resolved skills dir = %q, want an absolute path", dir)
	}
}
