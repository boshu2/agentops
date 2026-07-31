package gates

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScopeRange(t *testing.T) {
	cases := []struct {
		name     string
		scope    Scope
		wantSpec string
		wantOK   bool
	}{
		{"range with rev spec", "range:origin/main..HEAD", "origin/main..HEAD", true},
		{"range with shas", "range:abc123..def456", "abc123..def456", true},
		{"range prefix only", "range:", "", true},
		{"plain head is not a range", ScopeHead, "", false},
		{"plain upstream is not a range", ScopeUpstream, "", false},
		{"lookalike without colon", "range", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := ScopeRange(tc.scope)
			if ok != tc.wantOK {
				t.Fatalf("ScopeRange(%q) ok = %v, want %v", tc.scope, ok, tc.wantOK)
			}
			if spec != tc.wantSpec {
				t.Fatalf("ScopeRange(%q) spec = %q, want %q", tc.scope, spec, tc.wantSpec)
			}
		})
	}
}

func TestValidateRangeSpec(t *testing.T) {
	cases := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"well-formed two-dot", "origin/main..HEAD", false},
		{"three-dot symmetric", "main...feature", false},
		{"sha range", "abc123..def456", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"bare rev without dots", "HEAD", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRangeSpec(tc.spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateRangeSpec(%q) err = %v, wantErr %v", tc.spec, err, tc.wantErr)
			}
		})
	}
}

func TestScopeArgs_Range(t *testing.T) {
	args, err := scopeArgs("range:origin/main..HEAD")
	if err != nil {
		t.Fatalf("scopeArgs range: %v", err)
	}
	want := []string{"diff", "--name-only", "origin/main..HEAD"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("scopeArgs range = %v, want %v", args, want)
	}

	if _, err := scopeArgs("range:HEAD"); err == nil {
		t.Fatal("scopeArgs range:HEAD (no ..) should error, got nil")
	}
	if _, err := scopeArgs("bogus-scope"); err == nil {
		t.Fatal("scopeArgs on an unknown scope should error, got nil")
	}
}

// gitCommits builds a real 3-commit fixture repo and returns (repoRoot, c0, c1,
// c2). c1 adds a command file; c2 adds its test file — the c1+c2 train shape a
// landing loop pushes.
func gitCommits(t *testing.T) (root, c0, c1, c2 string) {
	t.Helper()
	root = t.TempDir()
	run := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeCommit := func(rel, body, msg string) string {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		run("add", "-A")
		run("commit", "-q", "-m", msg)
		return run("rev-parse", "HEAD")
	}
	run("init", "-q")
	c0 = writeCommit("README.md", "base\n", "base")
	c1 = writeCommit("cli/cmd/ao/foo.go", "package main\n\nfunc foo() {}\n", "add command foo")
	c2 = writeCommit("cli/cmd/ao/foo_test.go", "package main\n\nimport \"testing\"\n\nfunc TestFoo(t *testing.T) {}\n", "add foo test")
	return root, c0, c1, c2
}

// TestGitChangedFiles_RangeVsHead is the SLICE 1 acceptance at the Go layer: a
// range spanning the whole c1+c2 train sees both the command file and its test,
// while scope=head at commit c1 sees only the command file (which is what
// falsely fails the co-change gate in a detached landing worktree).
func TestGitChangedFiles_RangeVsHead(t *testing.T) {
	root, c0, c1, c2 := gitCommits(t)
	g := NewGitChangedFiles(root)
	ctx := context.Background()

	rangeFiles, err := g.Changed(ctx, Scope(ScopeRangePrefix+c0+".."+c2))
	if err != nil {
		t.Fatalf("Changed range: %v", err)
	}
	wantRange := []string{"cli/cmd/ao/foo.go", "cli/cmd/ao/foo_test.go"}
	if !equalSet(rangeFiles, wantRange) {
		t.Fatalf("range %s..%s changed = %v, want %v", c0, c2, rangeFiles, wantRange)
	}

	// Move HEAD to c1 (detached, as a per-commit landing gate would) and take
	// the head scope: it sees only the command file — the false-fail condition.
	checkout := exec.Command("git", "checkout", "-q", c1)
	checkout.Dir = root
	if out, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout c1: %v\n%s", err, out)
	}
	headFiles, err := g.Changed(ctx, ScopeHead)
	if err != nil {
		t.Fatalf("Changed head: %v", err)
	}
	wantHead := []string{"cli/cmd/ao/foo.go"}
	if !equalSet(headFiles, wantHead) {
		t.Fatalf("head@c1 changed = %v, want %v", headFiles, wantHead)
	}

	// The range still spans the whole train even with HEAD detached at c1.
	rangeAfterCheckout, err := g.Changed(ctx, Scope(ScopeRangePrefix+c0+".."+c2))
	if err != nil {
		t.Fatalf("Changed range after checkout: %v", err)
	}
	if !equalSet(rangeAfterCheckout, wantRange) {
		t.Fatalf("range after checkout = %v, want %v", rangeAfterCheckout, wantRange)
	}
}

// initRepoWithHead builds a temp git repo whose single non-base commit adds
// addFile, returning the repo root. Used to prove a git subprocess reads the
// repo named by cmd.Dir, not a leaked GIT_DIR.
func initRepoWithHead(t *testing.T, addFile string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	write("README.md", "base\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	write(addFile, "x\n")
	run("add", "-A")
	run("commit", "-q", "-m", "add "+addFile)
	return root
}

// TestGitChangedFiles_PollutedGitDirResolvesCorrectRepo is the acceptance for
// the SECURITY-MED fix: git injects GIT_DIR/GIT_WORK_TREE/... into
// hook-launched processes, and a leaked GIT_DIR overrides cwd-based discovery,
// so an unscrubbed subprocess computes the changed set against the WRONG repo
// (silently skipping blocking checks). The exec path must scrub the canonical
// git-discovery env so cmd.Dir wins.
func TestGitChangedFiles_PollutedGitDirResolvesCorrectRepo(t *testing.T) {
	correct := initRepoWithHead(t, "correct_file.go")
	polluted := initRepoWithHead(t, "wrong_file.go")

	// Simulate git's hook-injected discovery env pointing at a DIFFERENT repo.
	t.Setenv("GIT_DIR", filepath.Join(polluted, ".git"))

	g := NewGitChangedFiles(correct)
	got, err := g.Changed(context.Background(), ScopeHead)
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	want := []string{"correct_file.go"}
	if !equalSet(got, want) {
		t.Fatalf("polluted GIT_DIR routed changed-set to %v, want %v (from cmd.Dir repo)", got, want)
	}
}

// initRepoNoCommits builds a temp git repository with an unborn HEAD: `git
// init` ran, a file exists and may even be staged, but no commit was ever
// made. This is the state of every repository between `git init` and the first
// commit — the state a brand-new user of the CLI is in.
func initRepoNoCommits(t *testing.T, stage bool) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stage {
		run("add", "-A")
	}
	return root
}

// TestGitChangedFiles_UnbornHeadReportsFriendlyError is the acceptance for the
// zero-commit crash: `ao gate check` in a freshly `git init`ed repo died with
// the raw plumbing failure "git show --name-only --pretty=format: HEAD: exit
// status 128", which reads as a broken tool rather than the ordinary state it
// is. Every HEAD-resolving scope must instead return ErrUnbornHead with a
// remedy, leaking no git exit status.
func TestGitChangedFiles_UnbornHeadReportsFriendlyError(t *testing.T) {
	root := initRepoNoCommits(t, false)
	g := NewGitChangedFiles(root)

	for _, scope := range []Scope{ScopeHead, ScopeWorktree, ScopeUpstream, Scope(ScopeRangePrefix + "HEAD~1..HEAD")} {
		t.Run(string(scope), func(t *testing.T) {
			_, err := g.Changed(context.Background(), scope)
			if err == nil {
				t.Fatalf("Changed(%q) on an unborn HEAD returned no error", scope)
			}
			if !errors.Is(err, ErrUnbornHead) {
				t.Fatalf("Changed(%q) err = %v, want errors.Is ErrUnbornHead", scope, err)
			}
			for _, banned := range []string{"exit status", "--pretty=format"} {
				if strings.Contains(err.Error(), banned) {
					t.Errorf("Changed(%q) message leaks raw git plumbing %q: %v", scope, banned, err)
				}
			}
			for _, want := range []string{"no commits yet", "initial commit", "--scope staged"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Changed(%q) message missing %q: %v", scope, want, err)
				}
			}
		})
	}
}

// TestGitChangedFiles_UnbornHeadStagedScopeWorks pins the escape hatch the
// friendly message advertises: `--scope staged` compares the index against the
// empty tree and works before the first commit. A message pointing at a scope
// that also failed would be worse than the raw error.
func TestGitChangedFiles_UnbornHeadStagedScopeWorks(t *testing.T) {
	root := initRepoNoCommits(t, true)
	got, err := NewGitChangedFiles(root).Changed(context.Background(), ScopeStaged)
	if err != nil {
		t.Fatalf("Changed(staged) on an unborn HEAD: %v", err)
	}
	if !equalSet(got, []string{"README.md"}) {
		t.Fatalf("Changed(staged) = %v, want [README.md]", got)
	}
}

// TestGitChangedFiles_NonRepoKeepsGitError is the negative witness for the
// unborn-HEAD translation: outside a git repository the cause is different and
// so is the remedy, so git's own error must survive rather than be relabelled
// "no commits yet".
func TestGitChangedFiles_NonRepoKeepsGitError(t *testing.T) {
	_, err := NewGitChangedFiles(t.TempDir()).Changed(context.Background(), ScopeHead)
	if err == nil {
		t.Fatal("Changed outside a git repo returned no error")
	}
	if errors.Is(err, ErrUnbornHead) {
		t.Fatalf("non-repo error was mislabelled as an unborn HEAD: %v", err)
	}
}

// TestGitChangedFiles_BadRangeBaseKeepsGitError is the second negative witness:
// in a repo WITH commits, a range naming a revision that does not exist is a
// bad-scope error, not an unborn HEAD.
func TestGitChangedFiles_BadRangeBaseKeepsGitError(t *testing.T) {
	root, _, _, c2 := gitCommits(t)
	_, err := NewGitChangedFiles(root).Changed(context.Background(), Scope(ScopeRangePrefix+"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef.."+c2))
	if err == nil {
		t.Fatal("Changed with a nonexistent range base returned no error")
	}
	if errors.Is(err, ErrUnbornHead) {
		t.Fatalf("bad-range error was mislabelled as an unborn HEAD: %v", err)
	}
}

// equalSet reports whether a and b hold the same elements (order-independent).
func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
