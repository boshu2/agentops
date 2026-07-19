// practices: [design-by-contract, code-complete]
package skillsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// ResolveTargetDests must fan out to every INSTALLED runtime (Codex/AGY too, not
// just Claude), honor an explicit dest, and always include the portable root.
func TestResolveTargetDests(t *testing.T) {
	t.Run("explicit dest wins", func(t *testing.T) {
		got, err := ResolveTargetDests("/custom/skills")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 1 || got[0] != "/custom/skills" {
			t.Fatalf("got %v, want [/custom/skills]", got)
		}
	})

	t.Run("fans out to installed runtimes", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		for _, rt := range []string{".codex", ".gemini", ".pi"} {
			if err := os.MkdirAll(filepath.Join(home, rt), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		got, err := ResolveTargetDests("")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := []string{
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".codex", "skills"),
			filepath.Join(home, ".gemini", "skills"),
			filepath.Join(home, ".pi", "skills"),
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("portable root exists even when no runtime is present", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		got, err := ResolveTargetDests("")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := filepath.Join(home, ".agents", "skills")
		if len(got) != 1 || got[0] != want {
			t.Fatalf("got %v, want [%s]", got, want)
		}
	})
}

func TestLinkMissingSkills_WrongSymlinkIsConflict(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkSkill(t, src, "plan")
	wrong := filepath.Join(t.TempDir(), "plan")
	if err := os.Symlink(wrong, filepath.Join(dest, "plan")); err != nil {
		t.Fatal(err)
	}

	res, err := linkMissingSkills(src, dest, false)
	if err != nil {
		t.Fatalf("linkMissingSkills: %v", err)
	}
	if len(res.Present) != 0 || len(res.Conflicts) != 1 || res.Conflicts[0] != "plan" {
		t.Fatalf("wrong link classified as %+v", res)
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
	if _, werr := linkMissingSkills("   ", dest, false); werr == nil {
		t.Fatal("whitespace-only srcDir must fail closed with an error, got nil")
	}
}

// Cross-family refuter regression, round 2 (codex-fresh-review, age-u031): the
// identity check must reject a directory that merely CONTAINS a stray skills/
// subdir but is not the agentops repo (no skills-codex/ sibling).
func TestResolveRepoSkillsDir_OutsideRepoFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir stray skills/: %v", err)
	}
	t.Chdir(tmp)
	if got, err := ResolveRepoSkillsDir(); err == nil {
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
	if got, err := ResolveRepoSkillsDir(); err == nil {
		t.Fatalf("a skills/+skills-codex/ look-alike without agentops root markers must fail closed, got dir=%q nil error", got)
	}
}

// Inside the real repo (the test binary runs under cli/internal/skillsapp), the
// resolver locates the skills/+skills-codex/ pair and returns an absolute path.
func TestResolveRepoSkillsDir_InsideRepoResolvesAbsolute(t *testing.T) {
	dir, err := ResolveRepoSkillsDir()
	if err != nil {
		t.Fatalf("inside the agentops repo: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("resolved skills dir = %q, want an absolute path", dir)
	}
}

// Cross-family refuter regression (codex-fresh-review, age-d686g): the fan-out
// must be RESILIENT — a per-dest failure records an error on that dest but must
// NOT abort the loop.
func TestLinkAllDests_ResilientAcrossDests(t *testing.T) {
	src := t.TempDir()
	mkSkill(t, src, "alpha")

	good := t.TempDir()
	badParent := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(badParent, []byte("x"), 0o644); err != nil {
		t.Fatalf("write bad parent: %v", err)
	}
	bad := filepath.Join(badParent, "skills")

	results, anyErr := LinkAllDests(src, []string{bad, good}, false)

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
	if len(results[1].Linked) != 1 || results[1].Linked[0] != "alpha" {
		t.Fatalf("good dest Linked = %v, want [alpha]", results[1].Linked)
	}
	if _, err := os.Lstat(filepath.Join(good, "alpha")); err != nil {
		t.Fatalf("good dest was skipped after the earlier failure: %v", err)
	}
}

// RenderLinkResult is the human-facing per-dest summary: the ERROR line must
// promise the fan-out continued, and dry-run must never claim links were made.
func TestRenderLinkResult_ErrorDryRunAndConflictLines(t *testing.T) {
	var errBuf strings.Builder
	RenderLinkResult(&errBuf, LinkResult{Dest: "/d1", Err: "boom"})
	out := errBuf.String()
	if !strings.Contains(out, "ERROR: boom") || !strings.Contains(out, "other runtimes still attempted") {
		t.Errorf("error rendering must name the error and the continued fan-out, got:\n%s", out)
	}
	if strings.Contains(out, "linked:") {
		t.Errorf("error rendering must not print link counts, got:\n%s", out)
	}

	var dryBuf strings.Builder
	RenderLinkResult(&dryBuf, LinkResult{Dest: "/d2", DryRun: true, Linked: []string{"alpha"}, Conflicts: []string{"gamma"}})
	out = dryBuf.String()
	if !strings.Contains(out, "missing (dry-run, not linked): 1") {
		t.Errorf("dry-run rendering must label links as not created, got:\n%s", out)
	}
	if !strings.Contains(out, "? alpha") {
		t.Errorf("dry-run link mark must be '?', got:\n%s", out)
	}
	if !strings.Contains(out, "! gamma (real dir — foreign corpus, not clobbered)") {
		t.Errorf("conflict line must state the non-clobber guarantee, got:\n%s", out)
	}
}
