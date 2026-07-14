package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
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
// can drive executeBeadsExec and read what the child wrote, without mutating the
// shared legacyBeadsExecCommand global.
func newBeadsExecCmd(out, errbuf *strings.Builder) *cobra.Command {
	c := &cobra.Command{Use: "exec"}
	c.SetOut(out)
	c.SetErr(errbuf)
	c.SetIn(strings.NewReader(""))
	return c
}

func TestBeadsProductionPathsUseSharedResolvedCommand(t *testing.T) {
	root := makeGitRepoForTracker(t)
	workDir := filepath.Join(root, "nested")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(root, "ledger")
	script := filepath.Join(root, "tracker")
	scriptBody := `#!/bin/sh
printf 'cwd=%s beads=%s args=%s\n' "$PWD" "${BEADS_DIR-unset}" "$*" >> "$BEADS_PRODUCTION_LOG"
printf 'tracker-stdout\n'
printf 'tracker-stderr\n' >&2
exit 23
`
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatal(err)
	}
	originalLookPath := trackerLookPath
	t.Cleanup(func() { trackerLookPath = originalLookPath })
	trackerLookPath = func(name string) (string, error) {
		if name == trackerBR || name == trackerBD {
			return script, nil
		}
		return "", exec.ErrNotFound
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(workDir)

	for _, testCase := range []struct {
		name      string
		tracker   string
		wantDir   string
		wantBeads string
	}{
		{name: "br", tracker: trackerBR, wantDir: workDir, wantBeads: ledger},
		{name: "bd", tracker: trackerBD, wantDir: root, wantBeads: "unset"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			logPath := filepath.Join(root, testCase.name+".log")
			t.Setenv("AGENTOPS_TRACKER", testCase.tracker)
			t.Setenv("BEADS_DIR", ledger)
			t.Setenv("BEADS_PRODUCTION_LOG", logPath)

			tracker := currentBeadsTracker()
			output, outputErr := tracker.Output(context.Background(), "ready", "--json")
			var sharedExit *trackerexec.ExitError
			if !errors.As(outputErr, &sharedExit) || sharedExit.ExitCode() != 23 {
				t.Fatalf("tracker output error = %T %v, want shared *trackerexec.ExitError(23)", outputErr, outputErr)
			}
			if string(output) != "tracker-stdout\n" {
				t.Fatalf("tracker output = %q, want captured stdout", output)
			}

			var stdout, stderr bytes.Buffer
			executeErr := beadsadapter.NewExecutor(tracker).Execute(
				context.Background(),
				[]string{"close", "age-x", "-r", "done"},
				beadsapp.ExecStreams{Stdout: &stdout, Stderr: &stderr},
			)
			var familyExit *beadsapp.ExitError
			if !errors.As(executeErr, &familyExit) || familyExit.ExitCode() != 23 {
				t.Fatalf("executor error = %T %v, want beads ExitError(23)", executeErr, executeErr)
			}
			if stdout.String() != "tracker-stdout\n" || stderr.String() != "tracker-stderr\n" {
				t.Fatalf("executor streams = stdout %q stderr %q", stdout.String(), stderr.String())
			}
			logBody, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			wantContext := "cwd=" + testCase.wantDir + " beads=" + testCase.wantBeads
			if !strings.Contains(string(logBody), wantContext+" args=ready --json") ||
				!strings.Contains(string(logBody), wantContext+" args=close age-x -r done") {
				t.Fatalf("tracker log = %q, want canonical context and exact argv", logBody)
			}
		})
	}

	cancelLog := filepath.Join(root, "canceled.log")
	t.Setenv("AGENTOPS_TRACKER", trackerBR)
	t.Setenv("BEADS_DIR", ledger)
	t.Setenv("BEADS_PRODUCTION_LOG", cancelLog)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cancelErr := beadsadapter.NewExecutor(currentBeadsTracker()).Execute(
		canceled, []string{"ready"}, beadsapp.ExecStreams{},
	)
	if !errors.Is(cancelErr, context.Canceled) {
		t.Fatalf("canceled executor error = %T %v, want context.Canceled", cancelErr, cancelErr)
	}
	if _, err := os.Stat(cancelLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled executor launched tracker child: stat error %v", err)
	}
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
	if err := executeBeadsExec(cmd, []string{"ready"}); err != nil {
		t.Fatalf("executeBeadsExec: %v (stderr=%s)", err, errbuf.String())
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
	if err := executeBeadsExec(cmd, []string{"ready"}); err != nil {
		t.Fatalf("executeBeadsExec: %v (stderr=%s)", err, errbuf.String())
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
			cmd := legacyBeadsExecCommand
			cmd.SetOut(&out)
			cmd.SetErr(&errbuf)
			t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) })
			if err := executeBeadsExec(cmd, []string{flag}); err != nil {
				t.Fatalf("executeBeadsExec %s = %v, want nil (static help)", flag, err)
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
	if err := executeBeadsExec(cmd, []string{"children", "age-x"}); err != nil {
		t.Fatalf("executeBeadsExec children: %v (stderr=%s)", err, errbuf.String())
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
	if err := executeBeadsExec(cmd, []string{"children", "age-x"}); err != nil {
		t.Fatalf("executeBeadsExec children (bd): %v (stderr=%s)", err, errbuf.String())
	}
	got := strings.TrimSpace(out.String())
	if got != "ARGS=children age-x" {
		t.Errorf("bd children not forwarded verbatim; got %q", got)
	}
}

// TestRunBeadsExec_PropagatesExitCode: a non-zero tracker exit surfaces as a
// beadsVerdictError carrying the same code (Execute maps it to os.Exit).
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
	err := executeBeadsExec(cmd, []string{"close", "age-x", "-r", "nope"})
	if err == nil {
		t.Fatal("executeBeadsExec with failing tracker = nil, want beadsVerdictError")
	}
	var exitErr *beadsVerdictError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *beadsVerdictError", err)
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
	err := executeBeadsExec(cmd, []string{"children", "age-epic"})
	if err == nil {
		t.Fatal("children on a failing br show = nil, want beadsVerdictError")
	}
	var exitErr *beadsVerdictError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *beadsVerdictError", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7 (br show status propagated unchanged)", exitErr.ExitCode())
	}
	if !strings.Contains(errbuf.String(), "boom") {
		t.Errorf("br stderr not surfaced for diagnostics: %q", errbuf.String())
	}
}

// TestRunBeadsExec_BRReadJSONIsCanonicalPassthrough: br's `list --json` output
// is ALREADY canonical ({"issues":[...]} with br's extra envelope keys), so
// ao streams it VERBATIM — no reshape, no field drop, no reorder (age-f07z).
func TestRunBeadsExec_BRReadJSONIsCanonicalPassthrough(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// br's real list envelope: {"issues":[...], total, has_more, limit, offset}.
	writeTrackerShim(t, "br", `if [ "$1" = "list" ]; then
cat <<'JSON'
{"issues":[{"id":"age-1","title":"T","description":"d","priority":1,"status":"open"}],"total":1,"has_more":false,"limit":50,"offset":0}
JSON
exit 0
fi
echo "UNEXPECTED $*" >&2; exit 9
`)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"list", "--json"}); err != nil {
		t.Fatalf("executeBeadsExec list --json (br): %v (stderr=%s)", err, errbuf.String())
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &obj); err != nil {
		t.Fatalf("br list --json output not an object: %v\n%s", err, out.String())
	}
	if _, ok := obj["issues"]; !ok {
		t.Errorf("canonical list output missing .issues:\n%s", out.String())
	}
	// br's extra envelope keys survive verbatim (proves no reshape/strip on br).
	if _, ok := obj["total"]; !ok {
		t.Errorf("br passthrough dropped its .total envelope key:\n%s", out.String())
	}
}

// TestRunBeadsExec_BDListJSONReshapedToCanonical: bd's `list --json` bare array
// is reshaped to the SAME canonical shape as br — {"issues":[...]} — with the
// canonical .description key added and bd's extra fields preserved (age-f07z).
func TestRunBeadsExec_BDListJSONReshapedToCanonical(t *testing.T) {
	root := makeGitRepoForTracker(t)
	// bd list --json: a BARE array, elements lacking `description` (bd omits it
	// when empty), carrying a bd-only field (issue_type) that must survive.
	writeTrackerShim(t, "bd", `if [ "$1" = "list" ] && [ "$2" = "--json" ]; then
cat <<'JSON'
[{"id":"bd-1","title":"Child A","priority":2,"status":"open","issue_type":"task"}]
JSON
exit 0
fi
echo "UNEXPECTED $*" >&2; exit 9
`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"list", "--json"}); err != nil {
		t.Fatalf("executeBeadsExec list --json (bd): %v (stderr=%s)", err, errbuf.String())
	}
	var obj struct {
		Issues []map[string]json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal([]byte(out.String()), &obj); err != nil {
		t.Fatalf("bd list --json NOT reshaped to a {\"issues\":[...]} object: %v\n%s", err, out.String())
	}
	if len(obj.Issues) != 1 {
		t.Fatalf("reshaped .issues length = %d, want 1:\n%s", len(obj.Issues), out.String())
	}
	el := obj.Issues[0]
	if got := string(el["id"]); got != `"bd-1"` {
		t.Errorf("reshaped .issues[0].id = %s, want \"bd-1\"", got)
	}
	if _, ok := el["description"]; !ok {
		t.Errorf("reshape did not guarantee the canonical .description key:\n%s", out.String())
	}
	if _, ok := el["issue_type"]; !ok {
		t.Errorf("reshape dropped bd's extra .issue_type field (must be field-preserving):\n%s", out.String())
	}
}

// TestRunBeadsExec_BDShowJSONReshapedWithDependents: bd's `show --json` bare
// array is reshaped to a canonical bare array whose elements carry the
// guaranteed .description (null) and .dependents ([]) keys — so a
// `jq '.[0].dependents[]?'` / `.[0].description` selector never breaks (age-f07z).
func TestRunBeadsExec_BDShowJSONReshapedWithDependents(t *testing.T) {
	root := makeGitRepoForTracker(t)
	// bd show --json for an epic: bare array, NO description, NO dependents[]
	// (bd's default show reports only a dependent_count).
	writeTrackerShim(t, "bd", `if [ "$1" = "show" ]; then
cat <<'JSON'
[{"id":"bd-ep","title":"Epic","priority":1,"status":"open","issue_type":"epic","dependent_count":1}]
JSON
exit 0
fi
echo "UNEXPECTED $*" >&2; exit 9
`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"show", "bd-ep", "--json"}); err != nil {
		t.Fatalf("executeBeadsExec show --json (bd): %v (stderr=%s)", err, errbuf.String())
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &arr); err != nil {
		t.Fatalf("bd show --json NOT reshaped to a bare array: %v\n%s", err, out.String())
	}
	if len(arr) != 1 {
		t.Fatalf("reshaped show array length = %d, want 1:\n%s", len(arr), out.String())
	}
	el := arr[0]
	if _, ok := el["description"]; !ok {
		t.Errorf("reshape did not guarantee .description on show element:\n%s", out.String())
	}
	dep, ok := el["dependents"]
	if !ok {
		t.Fatalf("reshape did not guarantee .dependents on show element:\n%s", out.String())
	}
	if strings.TrimSpace(string(dep)) != "[]" {
		t.Errorf("bd show missing dependents should reshape to [], got %s", string(dep))
	}
	if _, ok := el["issue_type"]; !ok {
		t.Errorf("reshape dropped bd's extra .issue_type field on show:\n%s", out.String())
	}
}

// TestRunBeadsExec_BDReadyJSONBareArrayPreserved: bd's `ready --json` bare array
// stays a canonical bare array, so `jq length` counts issues (not object keys).
func TestRunBeadsExec_BDReadyJSONBareArrayPreserved(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `if [ "$1" = "ready" ] && [ "$2" = "--json" ]; then
cat <<'JSON'
[{"id":"bd-a","title":"A","priority":1,"status":"open"},{"id":"bd-b","title":"B","priority":2,"status":"open"}]
JSON
exit 0
fi
echo "UNEXPECTED $*" >&2; exit 9
`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"ready", "--json"}); err != nil {
		t.Fatalf("executeBeadsExec ready --json (bd): %v (stderr=%s)", err, errbuf.String())
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &arr); err != nil {
		t.Fatalf("bd ready --json NOT a bare array: %v\n%s", err, out.String())
	}
	if len(arr) != 2 {
		t.Errorf("ready array length = %d, want 2 (so `jq length` counts issues):\n%s", len(arr), out.String())
	}
}

// TestRunBeadsExec_BDReadWithoutJSONStreamsVerbatim: a bd READ verb WITHOUT
// --json is NOT reshaped — it streams the human output verbatim (age-f07z).
func TestRunBeadsExec_BDReadWithoutJSONStreamsVerbatim(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `echo "HUMAN: $*"`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"list"}); err != nil {
		t.Fatalf("executeBeadsExec list (bd, no --json): %v (stderr=%s)", err, errbuf.String())
	}
	if got := strings.TrimSpace(out.String()); got != "HUMAN: list" {
		t.Errorf("bd read without --json must stream verbatim; got %q", got)
	}
}

// TestRunBeadsExec_BDWriteVerbWithJSONStreamsVerbatim: a WRITE verb is never
// reshaped even with --json — only list/ready/show are read verbs (age-f07z).
func TestRunBeadsExec_BDWriteVerbWithJSONStreamsVerbatim(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `echo "RAW: $*"`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	if err := executeBeadsExec(cmd, []string{"create", "title", "--json"}); err != nil {
		t.Fatalf("executeBeadsExec create --json (bd): %v (stderr=%s)", err, errbuf.String())
	}
	if got := strings.TrimSpace(out.String()); got != "RAW: create title --json" {
		t.Errorf("bd write verb must stream verbatim (no reshape); got %q", got)
	}
}

// TestRunBeadsExec_BDReadJSONPropagatesExitCode: the capture+reshape path must
// propagate bd's exit code UNCHANGED and surface its stderr (age-f07z), matching
// the streaming passthrough contract.
func TestRunBeadsExec_BDReadJSONPropagatesExitCode(t *testing.T) {
	root := makeGitRepoForTracker(t)
	writeTrackerShim(t, "bd", `echo "boom" >&2
exit 7
`)
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	var out, errbuf strings.Builder
	cmd := newBeadsExecCmd(&out, &errbuf)
	err := executeBeadsExec(cmd, []string{"list", "--json"})
	if err == nil {
		t.Fatal("bd read+--json with a failing tracker = nil, want beadsVerdictError")
	}
	var exitErr *beadsVerdictError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *beadsVerdictError", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7 (propagated unchanged)", exitErr.ExitCode())
	}
	if !strings.Contains(errbuf.String(), "boom") {
		t.Errorf("bd stderr not surfaced for diagnostics: %q", errbuf.String())
	}
}

// TestCanonicalizeBDReadJSON_Unit: direct unit coverage of the reshape mapping
// for each read verb (L1 regression net under the L2 command tests above).
func TestCanonicalizeBDReadJSON_Unit(t *testing.T) {
	// list: bare array -> {"issues":[...]}
	got, err := canonicalizeBDReadJSON("list", []byte(`[{"id":"x","title":"t","priority":1,"status":"open"}]`))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(got)), `{"issues":`) {
		t.Errorf("list canonical must be an {\"issues\":...} object, got %s", got)
	}
	// empty list -> {"issues":[]}, never null (so `.issues | length` == 0)
	got, err = canonicalizeBDReadJSON("list", []byte(`[]`))
	if err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if strings.TrimSpace(string(got)) != `{"issues":[]}` {
		t.Errorf("empty list canonical = %s, want {\"issues\":[]}", got)
	}
	// ready: bare array stays a bare array
	got, err = canonicalizeBDReadJSON("ready", []byte(`[{"id":"y","title":"t","priority":1,"status":"open"}]`))
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(got)), "[") {
		t.Errorf("ready canonical must stay a bare array, got %s", got)
	}
	// show: missing dependents/description are injected as []/null
	got, err = canonicalizeBDReadJSON("show", []byte(`[{"id":"z","title":"t","priority":1,"status":"open"}]`))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(got, &arr); err != nil {
		t.Fatalf("show output parse: %v", err)
	}
	if strings.TrimSpace(string(arr[0]["dependents"])) != "[]" {
		t.Errorf("show must inject dependents=[], got %s", arr[0]["dependents"])
	}
	if strings.TrimSpace(string(arr[0]["description"])) != "null" {
		t.Errorf("show must inject description=null, got %s", arr[0]["description"])
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
