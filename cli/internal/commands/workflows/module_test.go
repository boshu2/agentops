package workflows

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain scrubs git discovery env vars before any test shells out to git
// (fixture `git init`), per .claude/rules/go.md: a leaked GIT_DIR pointing at a
// linked worktree's gitdir would let a fixture init rewrite the SHARED
// .git/config (ek8v).
func TestMain(m *testing.M) {
	testsupport.ScrubGitDiscoveryEnv()
	os.Exit(m.Run())
}

// newTestModule wires the module exactly as the cmd/ao composition does, with
// a controllable --dry-run seam.
func newTestModule(dry bool) *Module {
	return NewModule(clicontract.HostOptions{DryRun: func() bool { return dry }})
}

func TestCommandTreeShape(t *testing.T) {
	root := newTestModule(false).Command()
	if root.Name() != "workflows" {
		t.Fatalf("root name = %q, want workflows", root.Name())
	}
	if root.GroupID != "knowledge" {
		t.Fatalf("GroupID = %q, want knowledge (next to skills)", root.GroupID)
	}
	children := root.Commands()
	if len(children) != 2 {
		t.Fatalf("workflows exposes %d children, want exactly link+unlink", len(children))
	}
	for _, name := range []string{"link", "unlink"} {
		child, _, err := root.Find([]string{name})
		if err != nil || child == root {
			t.Fatalf("missing workflows child %q: %v", name, err)
		}
		for _, flag := range []string{"into", "json"} {
			if child.Flags().Lookup(flag) == nil {
				t.Errorf("workflows %s is missing the --%s flag", name, flag)
			}
		}
		if child.Args == nil {
			t.Errorf("workflows %s declares no Args policy", name)
		} else if err := child.Args(child, []string{"stray"}); err == nil {
			t.Errorf("workflows %s accepted a positional arg; want NoArgs", name)
		}
	}
}

// mkFixtureCheckout builds an agentops-shaped checkout that is also a git repo,
// so both source resolution (identity markers) and target resolution (git
// top-level) work against it. Tests never depend on the real repo's workflows/.
func mkFixtureCheckout(t *testing.T, scripts ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"skills", "skills-codex", "workflows"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"registry.json", "PRODUCT.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("marker"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range scripts {
		if err := os.WriteFile(filepath.Join(root, "workflows", s), []byte("// "+s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
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
	return root
}

// runModule executes one subcommand through the module's own command tree —
// the L2 entry point below the cmd/ao composition.
func runModule(t *testing.T, dry bool, args ...string) (string, error) {
	t.Helper()
	root := newTestModule(dry).Command()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestLinkThenUnlinkThroughModule(t *testing.T) {
	root := mkFixtureCheckout(t, "a.js", "b.js")
	t.Chdir(root)

	// Dry-run --json previews both scripts without writing anything.
	out, err := runModule(t, true, "link", "--json")
	if err != nil {
		t.Fatalf("link --dry-run --json: %v\n%s", err, out)
	}
	var preview struct {
		Dest      string   `json:"dest"`
		DryRun    bool     `json:"dry_run"`
		Linked    []string `json:"linked"`
		Conflicts []string `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out)
	}
	if !preview.DryRun {
		t.Fatal("dry_run = false in preview")
	}
	if len(preview.Linked) != 2 || preview.Linked[0] != "a.js" || preview.Linked[1] != "b.js" {
		t.Fatalf("preview linked = %v, want [a.js b.js]", preview.Linked)
	}
	if filepath.Base(preview.Dest) != "workflows" || filepath.Base(filepath.Dir(preview.Dest)) != ".claude" {
		t.Fatalf("preview dest = %q, want .../.claude/workflows", preview.Dest)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .claude/; err = %v, want IsNotExist", err)
	}

	// Real link mints the symlinks.
	if out, err := runModule(t, false, "link"); err != nil {
		t.Fatalf("link: %v\n%s", err, out)
	}
	for _, s := range []string{"a.js", "b.js"} {
		link := filepath.Join(root, ".claude", "workflows", s)
		if _, err := os.Readlink(link); err != nil {
			t.Fatalf("expected symlink at %s: %v", link, err)
		}
	}

	// Unlink removes exactly those links.
	if out, err := runModule(t, false, "unlink"); err != nil {
		t.Fatalf("unlink: %v\n%s", err, out)
	}
	for _, s := range []string{"a.js", "b.js"} {
		if _, err := os.Lstat(filepath.Join(root, ".claude", "workflows", s)); !os.IsNotExist(err) {
			t.Fatalf("unlink left %s behind; err = %v", s, err)
		}
	}
}

func TestLinkErrorsOutsideCheckout(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := runModule(t, false, "link")
	if err == nil {
		t.Fatalf("link outside the checkout must error, got:\n%s", out)
	}
}
