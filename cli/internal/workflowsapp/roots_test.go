// practices: [design-by-contract, code-complete]
package workflowsapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mkCheckout builds an agentops-shaped checkout fixture: the identity markers
// (skills/, skills-codex/, registry.json, PRODUCT.md) plus, when withWorkflows
// is set, a workflows/ dir carrying the named scripts. Tests never depend on
// the real repo's workflows/ content.
func mkCheckout(t *testing.T, withWorkflows bool, scripts ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"skills", "skills-codex"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for _, f := range []string{"registry.json", "PRODUCT.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("marker"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	if withWorkflows {
		for _, s := range scripts {
			mkWorkflow(t, filepath.Join(root, "workflows"), s)
		}
		if len(scripts) == 0 {
			if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestResolveRepoWorkflowsDir_InsideCheckoutResolvesAbsolute(t *testing.T) {
	root := mkCheckout(t, true, "a.js")
	sub := filepath.Join(root, "skills") // resolution must walk UP from a subdir
	t.Chdir(sub)

	got, err := ResolveRepoWorkflowsDir()
	if err != nil {
		t.Fatalf("inside fixture checkout: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("resolved workflows dir = %q, want an absolute path", got)
	}
	// os.Getwd may canonicalize the tempdir path; compare canonical forms.
	wantCanon, _ := filepath.EvalSymlinks(filepath.Join(root, "workflows"))
	gotCanon, _ := filepath.EvalSymlinks(got)
	if gotCanon != wantCanon {
		t.Fatalf("resolved %q, want %q", gotCanon, wantCanon)
	}
}

func TestResolveRepoWorkflowsDir_NoWorkflowsDirErrorsClearly(t *testing.T) {
	root := mkCheckout(t, false)
	t.Chdir(root)

	got, err := ResolveRepoWorkflowsDir()
	if err == nil {
		t.Fatalf("checkout without workflows/ must error, got dir=%q nil error", got)
	}
	if !strings.Contains(err.Error(), "workflows/") {
		t.Fatalf("error must name the missing workflows/ dir, got: %v", err)
	}
}

func TestResolveRepoWorkflowsDir_OutsideCheckoutFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	// A stray workflows/ dir without the checkout identity markers must NOT count.
	if err := os.MkdirAll(filepath.Join(tmp, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)
	if got, err := ResolveRepoWorkflowsDir(); err == nil {
		t.Fatalf("outside the checkout must fail closed, got dir=%q nil error", got)
	}
}

func TestResolveTargetDir_ExplicitIntoWins(t *testing.T) {
	got, err := ResolveTargetDir("/custom/workflows")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/custom/workflows" {
		t.Fatalf("got %q, want /custom/workflows", got)
	}
}

func TestResolveTargetDir_GitRootClaudeWorkflows(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	sub := filepath.Join(repo, "nested", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub) // target must be the git ROOT's .claude/workflows, not cwd's

	got, err := ResolveTargetDir("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantCanon, _ := filepath.EvalSymlinks(repo)
	want := filepath.Join(wantCanon, ".claude", "workflows")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTargetDir_OutsideGitRepoErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := ResolveTargetDir("")
	if err == nil {
		t.Fatalf("outside a git repo must error, got %q", got)
	}
	if !strings.Contains(err.Error(), "--into") {
		t.Fatalf("error must point at the --into escape hatch, got: %v", err)
	}
}

// gitInit initializes a real git repo at dir with cmd.Dir scoped to it, never
// touching the enclosing checkout (TestMain scrubs GIT_DIR et al).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	// Per-command scrub (test-isolation ratchet): drop git discovery vars so a
	// hook-injected GIT_DIR can never point this fixture at the real checkout.
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "GIT_DIR="),
			strings.HasPrefix(entry, "GIT_WORK_TREE="),
			strings.HasPrefix(entry, "GIT_COMMON_DIR="),
			strings.HasPrefix(entry, "GIT_INDEX_FILE="):
			continue
		default:
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}
