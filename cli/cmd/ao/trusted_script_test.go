package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planTrustedRepo builds a temp "repo" containing script at rel (relative,
// slash-separated), makes it executable, and returns (repoRoot, sentinelPath).
// The planted script `touch`es the sentinel when executed, so a missing sentinel
// after a call proves the script did NOT run.
func planTrustedRepo(t *testing.T, rel string) (string, string) {
	t.Helper()
	repo := t.TempDir()
	sentinel := filepath.Join(repo, "SCRIPT_RAN")
	script := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// #nosec G306 -- test fixture script must be executable.
	body := "#!/usr/bin/env bash\ntouch " + sentinel + "\necho ran-on-stderr 1>&2\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, sentinel
}

// useTrustedSelfAo simulates a GENUINE checkout: the running ao binary lives
// INSIDE repo (repo/cli/bin/ao), so aoBinaryInside(repo) is true. Restores on
// cleanup.
func useTrustedSelfAo(t *testing.T, repo string) {
	t.Helper()
	binDir := filepath.Join(repo, "cli", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	selfAo := filepath.Join(binDir, "ao")
	if err := os.WriteFile(selfAo, []byte("dummy"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := pawlSelfBinary
	pawlSelfBinary = func() (string, error) { return selfAo, nil }
	t.Cleanup(func() { pawlSelfBinary = prev })
}

// captureStderrDuring runs fn with os.Stderr redirected to a pipe and returns
// everything written. Restores os.Stderr before returning.
func captureStderrDuring(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, _ := io.ReadAll(r)
	return string(data)
}

// TestRunTrustedRepoScript_UntrustedRepoNotExecuted is the core planted-script
// regression: an INSTALLED ao (binary outside the repo) pointed at a repo that
// planted hooks/finding-compiler.sh must NOT execute it (aoBinaryInside false,
// no escape hatch) — the marker file must be absent and errUntrustedRepoScript
// returned.
func TestRunTrustedRepoScript_UntrustedRepoNotExecuted(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "hooks/finding-compiler.sh")
	useFakeSelfAo(t) // installed-ao: trusted binary lives OUTSIDE the repo.
	t.Setenv(trustRepoEnvVar, "")

	executed, err := runTrustedRepoScript(repo, "hooks/finding-compiler.sh", "--quiet")

	if executed {
		t.Fatal("SECURITY: untrusted repo's planted script was EXECUTED")
	}
	if !errors.Is(err, errUntrustedRepoScript) {
		t.Fatalf("want errUntrustedRepoScript, got %v", err)
	}
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Fatal("SECURITY: sentinel exists — the planted script ran despite untrusted checkout (RCE)")
	}
}

// TestRunTrustedRepoScript_TrustedRepoExecuted proves the guard is not
// vacuously blocking: a GENUINE checkout (ao binary inside the repo) DOES run
// the script and its stderr is surfaced, not discarded.
func TestRunTrustedRepoScript_TrustedRepoExecuted(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "hooks/finding-compiler.sh")
	useTrustedSelfAo(t, repo)
	t.Setenv(trustRepoEnvVar, "")

	var executed bool
	var runErr error
	stderr := captureStderrDuring(t, func() {
		executed, runErr = runTrustedRepoScript(repo, "hooks/finding-compiler.sh", "--quiet")
	})

	if runErr != nil {
		t.Fatalf("trusted repo script should run cleanly, got %v", runErr)
	}
	if !executed {
		t.Fatal("trusted repo script should have executed")
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("sentinel absent — trusted script did not run: %v", statErr)
	}
	if !strings.Contains(stderr, "ran-on-stderr") {
		t.Fatalf("script stderr was discarded; want it surfaced, got %q", stderr)
	}
}

// TestRunTrustedRepoScript_EscapeHatch proves AGENTOPS_TRUST_REPO=1 lets an
// installed ao (outside the repo) run the repo script anyway — for the
// installed-ao-inside-its-own-checkout workflow — with stderr captured.
func TestRunTrustedRepoScript_EscapeHatch(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "hooks/finding-compiler.sh")
	useFakeSelfAo(t) // outside the repo → aoBinaryInside false...
	t.Setenv(trustRepoEnvVar, "1")

	var executed bool
	var runErr error
	stderr := captureStderrDuring(t, func() {
		executed, runErr = runTrustedRepoScript(repo, "hooks/finding-compiler.sh")
	})

	if runErr != nil {
		t.Fatalf("escape hatch should run the script cleanly, got %v", runErr)
	}
	if !executed {
		t.Fatal("escape hatch should have executed the script")
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("sentinel absent — escape hatch did not run the script: %v", statErr)
	}
	if !strings.Contains(stderr, "ran-on-stderr") {
		t.Fatalf("escape-hatch run discarded stderr; got %q", stderr)
	}
}

// TestRunTrustedRepoScript_MissingScript proves an absent script is a silent
// no-op, not an error (executed=false, err=nil).
func TestRunTrustedRepoScript_MissingScript(t *testing.T) {
	repo := t.TempDir()
	useTrustedSelfAo(t, repo)
	executed, err := runTrustedRepoScript(repo, "hooks/finding-compiler.sh")
	if err != nil {
		t.Fatalf("missing script should be a no-op, got err %v", err)
	}
	if executed {
		t.Fatal("missing script should report executed=false")
	}
}

// TestBestEffortRefreshFindingCompiler_UntrustedSkipsWithNote covers the
// findings pull/retire best-effort path: an untrusted repo must SKIP the planted
// hooks/finding-compiler.sh (marker absent) and emit an observable debug note.
func TestBestEffortRefreshFindingCompiler_UntrustedSkipsWithNote(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "hooks/finding-compiler.sh")
	useFakeSelfAo(t)
	t.Setenv(trustRepoEnvVar, "")

	note := captureStderrDuring(t, func() {
		bestEffortRefreshFindingCompiler(repo)
	})

	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Fatal("SECURITY: bestEffortRefreshFindingCompiler executed a planted script on an untrusted repo")
	}
	if !strings.Contains(note, "hooks/finding-compiler.sh") || !strings.Contains(note, "untrusted") {
		t.Fatalf("want an observable skip note naming the script + untrusted reason, got %q", note)
	}
}

// TestBestEffortPruneAgents_UntrustedSkipsWithNote covers the session-end
// best-effort path: an untrusted repo must SKIP the planted scripts/prune-agents.sh
// (marker absent) and emit an observable debug note.
func TestBestEffortPruneAgents_UntrustedSkipsWithNote(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "scripts/prune-agents.sh")
	useFakeSelfAo(t)
	t.Setenv(trustRepoEnvVar, "")
	t.Setenv("AGENTOPS_AUTO_PRUNE", "") // ensure not disabled

	note := captureStderrDuring(t, func() {
		bestEffortPruneAgents(repo)
	})

	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Fatal("SECURITY: bestEffortPruneAgents executed a planted script on an untrusted repo")
	}
	if !strings.Contains(note, "scripts/prune-agents.sh") || !strings.Contains(note, "untrusted") {
		t.Fatalf("want an observable skip note naming the script + untrusted reason, got %q", note)
	}
}

// TestBestEffortPruneAgents_TrustedExecutes proves the session-end path DOES run
// the prune script on a genuine checkout (guard not vacuous).
func TestBestEffortPruneAgents_TrustedExecutes(t *testing.T) {
	repo, sentinel := planTrustedRepo(t, "scripts/prune-agents.sh")
	useTrustedSelfAo(t, repo)
	t.Setenv(trustRepoEnvVar, "")
	t.Setenv("AGENTOPS_AUTO_PRUNE", "")

	_ = captureStderrDuring(t, func() {
		bestEffortPruneAgents(repo)
	})

	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("trusted session-end path did not run scripts/prune-agents.sh: %v", statErr)
	}
}

// TestUntrustedRepoScriptError_NamesReasonAndHatch proves the command-site hard
// error surfaces both the script and the escape hatch so the operator can opt in.
func TestUntrustedRepoScriptError_NamesReasonAndHatch(t *testing.T) {
	err := untrustedRepoScriptError("/some/repo", "scripts/pawl-verdict.sh")
	if err == nil {
		t.Fatal("want a non-nil hard error")
	}
	msg := err.Error()
	for _, want := range []string{"scripts/pawl-verdict.sh", "/some/repo", trustRepoEnvVar} {
		if !strings.Contains(msg, want) {
			t.Errorf("hard error missing %q; got %q", want, msg)
		}
	}
}

// TestRepoScriptTrusted_Policy tests the trust predicate directly across the
// three inputs (untrusted, escape hatch, trusted-inside).
func TestRepoScriptTrusted_Policy(t *testing.T) {
	repo := t.TempDir()

	// Untrusted: installed ao outside the repo, no escape hatch.
	useFakeSelfAo(t)
	t.Setenv(trustRepoEnvVar, "")
	if repoScriptTrusted(repo) {
		t.Error("installed ao outside repo without escape hatch must be untrusted")
	}

	// Escape hatch flips it to trusted.
	t.Setenv(trustRepoEnvVar, "1")
	if !repoScriptTrusted(repo) {
		t.Error("AGENTOPS_TRUST_REPO=1 must trust the repo")
	}

	// Non-"1" values do not trust.
	t.Setenv(trustRepoEnvVar, "yes")
	if repoScriptTrusted(repo) {
		t.Error("only AGENTOPS_TRUST_REPO=1 (exactly) should trust; \"yes\" must not")
	}
}

// TestNoUngatedRepoScriptExec is the fail-closed, per-site AST guard mirroring
// the canon sh -c trust-boundary guard (verifier_trust_boundary_test.go,
// age-b6al). It parses every production .go file in cmd/ao and FAILS if any
// `exec.Command("bash"|"sh", ...)` call site — outside trusted_script.go and the
// scoped exemptions — passes a repo/cwd-relative script path (a filepath.Join /
// string literal referencing scripts|hooks). Every cwd-relative script exec MUST
// route through runTrustedRepoScript so the aoBinaryInside RCE boundary is
// enforced. File-level string matching was explicitly refuted before; this is
// per-call-site via go/ast.
func TestNoUngatedRepoScriptExec(t *testing.T) {
	// Files allowed to construct exec.Command("bash"/"sh", ...) with a
	// repo-relative script, because they ARE the trust boundary:
	//   - trusted_script.go: the chokepoint itself.
	//   - pawl.go: another lane's surface; runs repo scripts behind its own
	//     aoBinaryInside gate (runForwardedPawlScript / trustedLookPath).
	exemptFiles := map[string]bool{
		"trusted_script.go": true,
		"pawl.go":           true,
	}

	root := "."
	sitesChecked := 0
	var violations []string

	// isBashOrSh reports whether expr is a string literal "bash" or "sh".
	isBashOrSh := func(expr ast.Expr) bool {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		v := strings.Trim(lit.Value, "`\"")
		return v == "bash" || v == "sh"
	}

	// mentionsRepoScript reports whether any of the remaining args references a
	// repo/cwd-relative script path — a "scripts"/"hooks" string literal, or a
	// filepath.Join(...) whose args include such a literal. Conservative: if we
	// can't prove it's NOT a repo script, we treat a bash/sh exec as a repo-script
	// exec (fail closed).
	referencesScriptLiteral := func(n ast.Node) bool {
		found := false
		ast.Inspect(n, func(m ast.Node) bool {
			lit, ok := m.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v := strings.Trim(lit.Value, "`\"")
			if strings.Contains(v, "scripts") || strings.Contains(v, "hooks") || strings.HasSuffix(v, ".sh") {
				found = true
				return false
			}
			return true
		})
		return found
	}

	scan := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err // fail closed
		}
		if !strings.Contains(string(data), "exec.Command") {
			return nil
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, data, 0)
		if perr != nil {
			return perr // fail closed on unparseable production file
		}
		base := filepath.Base(path)
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Command" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "exec" {
				return true
			}
			if len(call.Args) == 0 || !isBashOrSh(call.Args[0]) {
				return true // not a bash/sh invocation
			}
			sitesChecked++
			if exemptFiles[base] {
				return true
			}
			// A bash/sh exec in a non-exempt file that references a repo-relative
			// script path is a violation — it must route through the chokepoint.
			for _, arg := range call.Args[1:] {
				if referencesScriptLiteral(arg) {
					violations = append(violations,
						base+":"+itoa(fset.Position(call.Pos()).Line))
					break
				}
			}
			return true
		})
		return nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return scan(path)
	})
	if err != nil {
		t.Fatalf("scanning cmd/ao production sources: %v", err)
	}

	// Non-vacuous: the exempt trust-boundary files DO construct bash/sh execs, so
	// at least one site must have been seen. Zero means the guard is scanning
	// nothing (moved package, renamed API) and must fail rather than pass empty.
	if sitesChecked == 0 {
		t.Fatal("no exec.Command(\"bash\"/\"sh\", ...) site found in cmd/ao — the guard is covering nothing")
	}

	for _, v := range violations {
		t.Errorf("ungated repo-script exec at %s — a cwd-relative script must route "+
			"through runTrustedRepoScript (trusted_script.go) so the aoBinaryInside RCE "+
			"boundary applies; do not exec.Command(\"bash\", <repo script>) directly.", v)
	}
}

// itoa is a tiny int->string helper local to this test (avoids importing strconv
// only for the AST guard's line-number formatting).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
