package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowsCompositionOwnsCompleteCommandTree(t *testing.T) {
	command := newWorkflowsCommand()
	for _, path := range []string{"ao workflows link", "ao workflows unlink"} {
		args := strings.Fields(strings.TrimPrefix(path, "ao workflows "))
		child, remaining, err := command.Find(args)
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("missing workflows child %q: child=%v remaining=%v err=%v", path, child, remaining, err)
		}
	}
	if len(command.Commands()) != 2 {
		t.Fatalf("workflows exposes %d children, want only link+unlink", len(command.Commands()))
	}
	if command.GroupID != "knowledge" {
		t.Fatalf("workflows GroupID = %q, want knowledge (next to skills)", command.GroupID)
	}
}

func TestWorkflowsCompositionRegistersExactlyOneRootOwner(t *testing.T) {
	owners := 0
	for _, command := range rootCmd.Commands() {
		if command.Name() == "workflows" {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("workflows root owners = %d, want 1", owners)
	}
}

// mkWorkflowsFixtureCheckout builds an agentops-shaped checkout that is also a
// git repo, so source resolution (identity markers) and target resolution (git
// top-level of cwd) both work. Tests never depend on the real workflows/ tree.
func mkWorkflowsFixtureCheckout(t *testing.T, scripts ...string) string {
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
	cmd.Dir = root // TestMain scrubbed GIT_DIR et al; never touches the real checkout
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

type workflowsLinkDoc struct {
	Source    string   `json:"source"`
	Dest      string   `json:"dest"`
	DryRun    bool     `json:"dry_run"`
	Linked    []string `json:"linked"`
	Present   []string `json:"present"`
	Conflicts []string `json:"conflicts"`
}

// TestWorkflowsLinkEndToEnd drives the full acceptance flow through the real
// root command tree (global --dry-run, then a real link, conflict safety, and
// unlink) against a fixture checkout.
func TestWorkflowsLinkEndToEnd(t *testing.T) {
	root := mkWorkflowsFixtureCheckout(t, "a.js", "b.js")
	t.Chdir(root)

	// 1. Dry-run --json lists both targets under <git-root>/.claude/workflows
	//    without writing anything.
	out, err := executeCommand("workflows", "link", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("workflows link --dry-run --json: %v\n%s", err, out)
	}
	var preview workflowsLinkDoc
	if jerr := json.Unmarshal([]byte(out), &preview); jerr != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", jerr, out)
	}
	if !preview.DryRun {
		t.Fatal("dry_run = false; global --dry-run did not reach the module")
	}
	if len(preview.Linked) != 2 || preview.Linked[0] != "a.js" || preview.Linked[1] != "b.js" {
		t.Fatalf("preview linked = %v, want [a.js b.js]", preview.Linked)
	}
	canonRoot, _ := filepath.EvalSymlinks(root)
	wantDest := filepath.Join(canonRoot, ".claude", "workflows")
	if preview.Dest != wantDest {
		t.Fatalf("preview dest = %q, want %q", preview.Dest, wantDest)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote .claude/; err = %v, want IsNotExist", err)
	}

	// 2. Plain link creates exactly those symlinks resolving to the sources.
	if out, err = executeCommand("workflows", "link"); err != nil {
		t.Fatalf("workflows link: %v\n%s", err, out)
	}
	for _, s := range []string{"a.js", "b.js"} {
		got, rerr := os.Readlink(filepath.Join(wantDest, s))
		if rerr != nil {
			t.Fatalf("expected symlink for %s: %v", s, rerr)
		}
		// macOS: the module records the $PWD-form source while git reports the
		// /private-canonicalized root — compare canonical paths, then prove the
		// link actually reaches the canonical source bytes.
		gotCanon, _ := filepath.EvalSymlinks(got)
		wantCanon, _ := filepath.EvalSymlinks(filepath.Join(canonRoot, "workflows", s))
		if gotCanon != wantCanon {
			t.Fatalf("%s resolves to %q (canon %q), want canon %q", s, got, gotCanon, wantCanon)
		}
		body, rerr := os.ReadFile(filepath.Join(wantDest, s))
		if rerr != nil || string(body) != "// "+s {
			t.Fatalf("%s not readable through the link: body=%q err=%v", s, body, rerr)
		}
	}

	// 3. A pre-existing real file is a conflict, never replaced, exit zero
	//    (operator judgment, mirroring skills link conflict semantics).
	if err := os.Remove(filepath.Join(wantDest, "a.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wantDest, "a.js"), []byte("operator-owned"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = executeCommand("workflows", "link", "--json")
	if err != nil {
		t.Fatalf("conflicts must not fail the exit (skills link semantics): %v\n%s", err, out)
	}
	var again workflowsLinkDoc
	if jerr := json.Unmarshal([]byte(out), &again); jerr != nil {
		t.Fatalf("link --json output is not JSON: %v\n%s", jerr, out)
	}
	if len(again.Conflicts) != 1 || again.Conflicts[0] != "a.js" {
		t.Fatalf("conflicts = %v, want [a.js]", again.Conflicts)
	}
	if len(again.Present) != 1 || again.Present[0] != "b.js" {
		t.Fatalf("present = %v, want [b.js]", again.Present)
	}
	if body, rerr := os.ReadFile(filepath.Join(wantDest, "a.js")); rerr != nil || string(body) != "operator-owned" {
		t.Fatalf("real file was clobbered: body=%q err=%v", body, rerr)
	}

	// 4. Unlink removes only checkout-owned links, keeps the operator's file.
	if out, err = executeCommand("workflows", "unlink"); err != nil {
		t.Fatalf("workflows unlink: %v\n%s", err, out)
	}
	if _, err := os.Lstat(filepath.Join(wantDest, "b.js")); !os.IsNotExist(err) {
		t.Fatalf("owned link b.js not removed; err = %v", err)
	}
	if body, rerr := os.ReadFile(filepath.Join(wantDest, "a.js")); rerr != nil || string(body) != "operator-owned" {
		t.Fatalf("unlink touched the operator's real file: body=%q err=%v", body, rerr)
	}
}

func TestWorkflowsLinkIntoOverridesTarget(t *testing.T) {
	root := mkWorkflowsFixtureCheckout(t, "only.js")
	t.Chdir(root)
	into := filepath.Join(t.TempDir(), "custom-workflows")

	out, err := executeCommand("workflows", "link", "--into", into)
	if err != nil {
		t.Fatalf("workflows link --into: %v\n%s", err, out)
	}
	if _, err := os.Readlink(filepath.Join(into, "only.js")); err != nil {
		t.Fatalf("--into target missing the link: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("--into still wrote the default target; err = %v", err)
	}
}

func TestWorkflowsLinkWithoutWorkflowsDirErrorsClearly(t *testing.T) {
	root := mkWorkflowsFixtureCheckout(t) // markers, but delete workflows/
	if err := os.Remove(filepath.Join(root, "workflows")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	out, err := executeCommand("workflows", "link")
	if err == nil {
		t.Fatalf("link without workflows/ must error, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "workflows/") {
		t.Fatalf("error must name the missing workflows/ dir, got: %v", err)
	}
}
