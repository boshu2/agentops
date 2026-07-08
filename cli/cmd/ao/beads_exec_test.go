package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeTrackerShim installs an executable fake tracker binary named `name`
// (br|bd) in its own dir, prepends that dir to PATH for the test, and returns
// the dir. The script body is caller-supplied so each test controls what the
// fake tracker echoes. Using a real PATH shim exercises the REAL resolveTracker
// -> exec.LookPath -> exec path (no trackerLookPath stub), which is the surface
// under test.
func writeTrackerShim(t *testing.T, name, body string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash unavailable: %v", err)
	}
	dir := t.TempDir()
	script := "#!/usr/bin/env bash\n" + body
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil { // #nosec G306 -- test shim must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// newBeadsExecCmd returns a throwaway cobra command wired to buffers so a test
// can drive runBeadsExec and read what the child wrote, without mutating the
// shared beadsExecCmd global.
func newBeadsExecCmd(out, errbuf *strings.Builder) *cobra.Command {
	c := &cobra.Command{Use: "exec"}
	c.SetOut(out)
	c.SetErr(errbuf)
	c.SetIn(strings.NewReader(""))
	return c
}

// TestRunBeadsExec_BRForwardsWithBeadsDir: in a _beads repo, `ao beads exec
// ready` forwards to br with BEADS_DIR pointing at the resolved ledger.
func TestRunBeadsExec_BRForwardsWithBeadsDir(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTrackerShim(t, "br", `echo "ARGS=$*"
echo "BEADS_DIR=${BEADS_DIR:-<unset>}"
echo "CWD=$(/bin/pwd)"
`)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := runBeadsExec(cmd, []string{"ready"}); err != nil {
		t.Fatalf("runBeadsExec: %v (stderr=%s)", err, errbuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "ARGS=ready") {
		t.Errorf("args not forwarded verbatim to br:\n%s", got)
	}
	wantDir := "BEADS_DIR=" + filepath.Join(root, "_beads")
	if !strings.Contains(got, wantDir) {
		t.Errorf("br child missing %q:\n%s", wantDir, got)
	}
}

// TestRunBeadsExec_BDForwardsWithRepoRootCwdNoBeadsDir: AGENTOPS_TRACKER=bd
// forwards to bd with the child cwd set to the repo root and NO BEADS_DIR.
func TestRunBeadsExec_BDForwardsWithRepoRootCwdNoBeadsDir(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `echo "ARGS=$*"
echo "BEADS_DIR=${BEADS_DIR:-<unset>}"
echo "CWD=$(/bin/pwd)"
`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	// A stray inherited BEADS_DIR must NOT leak into the bd child.
	t.Setenv("BEADS_DIR", "/should/be/stripped")
	t.Setenv("HOME", t.TempDir())
	// Run from a subdir to prove the child is repositioned to the repo root.
	sub := filepath.Join(root, "cli")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := runBeadsExec(cmd, []string{"ready"}); err != nil {
		t.Fatalf("runBeadsExec: %v (stderr=%s)", err, errbuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "ARGS=ready") {
		t.Errorf("args not forwarded verbatim to bd:\n%s", got)
	}
	if !strings.Contains(got, "BEADS_DIR=<unset>") {
		t.Errorf("bd child must not carry BEADS_DIR:\n%s", got)
	}
	wantCWD := "CWD=" + root
	if !strings.Contains(got, wantCWD) {
		t.Errorf("bd child cwd not the repo root; want %q:\n%s", wantCWD, got)
	}
}

// TestRunBeadsExec_HelpIsStatic: -h/--help returns static help with no tracker
// resolution and no side effects — even when NO tracker is resolvable (which
// would otherwise error), proving --help short-circuits before resolveTracker.
func TestRunBeadsExec_HelpIsStatic(t *testing.T) {
	// No ledger, no binary on PATH -> resolveTracker would fail if reached.
	setTrackerLookPath(t, false, false)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "") // no br/bd discoverable
	t.Chdir(t.TempDir()) // not a git repo, no ledger

	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			var out, errbuf strings.Builder
			cmd := beadsExecCmd
			cmd.SetOut(&out)
			cmd.SetErr(&errbuf)
			t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) })
			if err := runBeadsExec(cmd, []string{flag}); err != nil {
				t.Fatalf("runBeadsExec %s = %v, want nil (static help)", flag, err)
			}
			help := out.String() + errbuf.String()
			if !strings.Contains(help, "Usage:") {
				t.Errorf("%s did not render usage help:\n%s", flag, help)
			}
			// The land trap: no resolved binary path may leak into help output.
			for _, leak := range []string{"/br", "/bd", "BEADS_DIR="} {
				if strings.Contains(help, leak) {
					t.Errorf("%s leaked a path-dependent string %q:\n%s", flag, leak, help)
				}
			}
		})
	}
}

// TestRunBeadsExec_ChildrenBR: `children <epic>` on br synthesizes the child-id
// list from `br show <epic> --json`, emitting only parent-child dependents.
func TestRunBeadsExec_ChildrenBR(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// br shim: `show <id> --json` returns a real-shaped array with a mix of
	// parent-child and related dependents.
	writeTrackerShim(t, "br", `if [ "$1" = "show" ]; then
cat <<'JSON'
[{"id":"age-x","dependents":[{"id":"age-x.1","dependency_type":"parent-child"},{"id":"age-x.2","dependency_type":"parent-child"},{"id":"age-y","dependency_type":"related"}]}]
JSON
exit 0
fi
echo "UNEXPECTED $*" >&2
exit 9
`)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := runBeadsExec(cmd, []string{"children", "age-x"}); err != nil {
		t.Fatalf("runBeadsExec children: %v (stderr=%s)", err, errbuf.String())
	}
	lines := splitNonEmptyLines(out.String())
	want := []string{"age-x.1", "age-x.2"} // the related dependent is excluded
	if len(lines) != len(want) {
		t.Fatalf("children ids = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("children id[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

// TestRunBeadsExec_ChildrenBD: `children <epic>` on bd is forwarded verbatim to
// bd's native `bd children` (NOT synthesized).
func TestRunBeadsExec_ChildrenBD(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `echo "ARGS=$*"`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := runBeadsExec(cmd, []string{"children", "age-x"}); err != nil {
		t.Fatalf("runBeadsExec children (bd): %v (stderr=%s)", err, errbuf.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "ARGS=children age-x" {
		t.Errorf("bd children not forwarded verbatim; got %q", got)
	}
}

// TestRunBeadsExec_PropagatesExitCode: a non-zero tracker exit surfaces as a
// beadsExitError carrying the same code (Execute maps it to os.Exit).
func TestRunBeadsExec_PropagatesExitCode(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTrackerShim(t, "br", `echo "boom" >&2
exit 7
`)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	err := runBeadsExec(cmd, []string{"close", "age-x", "-r", "nope"})
	if err == nil {
		t.Fatal("runBeadsExec with failing tracker = nil, want beadsExitError")
	}
	var exitErr *beadsExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *beadsExitError", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7 (propagated unchanged)", exitErr.ExitCode())
	}
}

// TestRunBeadsExec_ChildrenBRPropagatesExitCode: the br `children` synthesis path
// (br show <epic> --json) must ALSO propagate br's exit code unchanged, not collapse
// a failing `br show` into the generic ao error path (age-3mdu membrane refute-fix).
func TestRunBeadsExec_ChildrenBRPropagatesExitCode(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// br shim: `show <id> --json` fails with exit 7 + stderr.
	writeTrackerShim(t, "br", `if [ "$1" = "show" ]; then echo "boom" >&2; exit 7; fi
echo "unexpected verb: $*"
`)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	err := runBeadsExec(cmd, []string{"children", "age-epic"})
	if err == nil {
		t.Fatal("children on a failing br show = nil, want beadsExitError")
	}
	var exitErr *beadsExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *beadsExitError", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7 (br show status propagated unchanged)", exitErr.ExitCode())
	}
	if !strings.Contains(errbuf.String(), "boom") {
		t.Errorf("br stderr not surfaced for diagnostics: %q", errbuf.String())
	}
}

// splitNonEmptyLines splits s on newlines and drops empty lines.
func splitNonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
