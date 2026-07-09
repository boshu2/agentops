package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// trustRepoEnvVar is the operator escape hatch: when set to "1", a repo-relative
// script is trusted even if the running ao binary does not physically live inside
// the checkout (aoBinaryInside is false). This covers the legitimate
// installed-ao-inside-its-own-checkout workflow, where the operator has already
// chosen to execute this repo's code. It is a deliberate, documented opt-out of
// the RCE guard — never a silent default.
const trustRepoEnvVar = "AGENTOPS_TRUST_REPO"

// errUntrustedRepoScript signals that a repo-relative script was NOT executed
// because the checkout failed the trust test (aoBinaryInside false and the
// AGENTOPS_TRUST_REPO escape hatch unset). Best-effort callers treat it as a
// skip; command callers surface it as a hard error to the user.
var errUntrustedRepoScript = errors.New("repo script not trusted")

// repoScriptTrusted reports whether a script physically resolved inside repoRoot
// may be executed. The test mirrors the pawl.go RCE boundary (aoBinaryInside):
// the running ao binary must live inside the resolved checkout, so an installed
// ao pointed at a foreign/forged repo never runs that repo's planted scripts.
// The AGENTOPS_TRUST_REPO=1 escape hatch trusts the repo regardless, for the
// installed-ao-inside-its-own-checkout workflow. It fails safe: any doubt → false.
func repoScriptTrusted(repoRoot string) bool {
	if os.Getenv(trustRepoEnvVar) == "1" {
		return true
	}
	return aoBinaryInside(repoRoot)
}

// runTrustedRepoScript executes a cwd/repo-relative bash script ONLY when the
// checkout passes the trust test (repoScriptTrusted). rel is the script path
// relative to repoRoot (e.g. "hooks/finding-compiler.sh"); args are passed to
// the script.
//
// It stats the script first (a missing script is not an error — the caller
// decides whether the script is expected), routes through the same trust
// boundary as pawl.go's live-script path, and NEVER discards stderr: the child's
// combined output is surfaced (returned on failure, or forwarded to os.Stderr on
// success) so an executed script is observable. When the repo is untrusted it
// returns errUntrustedRepoScript WITHOUT executing anything.
//
// Returns (executed, err):
//   - executed=false, err=nil          → script absent (nothing to run)
//   - executed=false, errUntrustedRepoScript → untrusted repo, deliberately skipped
//   - executed=true,  err=nil          → ran cleanly
//   - executed=true,  err!=nil         → ran and failed (err wraps the script's output)
func runTrustedRepoScript(repoRoot, rel string, args ...string) (bool, error) {
	script := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat repo script %s: %w", rel, err)
	}
	if !repoScriptTrusted(repoRoot) {
		return false, errUntrustedRepoScript
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...) // #nosec G204 -- repo-root-relative script gated by repoScriptTrusted (aoBinaryInside boundary); never a user-supplied path.
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("run repo script %s: %w\n%s", rel, err, out)
	}
	// Surface the script's output rather than discarding it (the pre-guard sites
	// silently swallowed stderr). Only forwarded on success; on failure it is in
	// the wrapped error above.
	if len(out) > 0 {
		_, _ = os.Stderr.Write(out)
	}
	return true, nil
}

// runTrustedRepoScriptStreaming is runTrustedRepoScript's STREAMING sibling: it
// wires the gated script's stdio straight to the provided reader/writers (so a
// long-running land script — scripts/pawl-land.sh's rebase → pre-push gate →
// single push — streams live instead of buffering to the very end), and appends
// extraEnv to the child environment (e.g. the AO_BIN pin so preflight + verdict
// emit + the gate share ONE fresh binary). It enforces the EXACT SAME
// repoScriptTrusted (aoBinaryInside) RCE boundary as runTrustedRepoScript — the
// only reason it lives here in the trust-boundary file rather than being written
// at the call site is so every bash-exec-of-a-repo-script stays behind this one
// gate (the TestNoUngatedRepoScriptExec AST guard).
//
// Returns (executed, err) with the same contract as runTrustedRepoScript:
//   - executed=false, err=nil                 → script absent (nothing to run)
//   - executed=false, errUntrustedRepoScript  → untrusted repo, deliberately skipped
//   - executed=true,  err=nil                 → ran cleanly
//   - executed=true,  err!=nil                → ran and failed (err is the raw *exec.ExitError,
//                                               so callers can propagate the exit CODE verbatim)
func runTrustedRepoScriptStreaming(repoRoot, rel string, stdin io.Reader, stdout, stderr io.Writer, extraEnv []string, args ...string) (bool, error) {
	script := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat repo script %s: %w", rel, err)
	}
	if !repoScriptTrusted(repoRoot) {
		return false, errUntrustedRepoScript
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...) // #nosec G204 -- repo-root-relative script gated by repoScriptTrusted (aoBinaryInside boundary); never a user-supplied path.
	cmd.Dir = repoRoot
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if err := cmd.Run(); err != nil {
		// Return the raw error (an *exec.ExitError on a non-zero exit) so the caller
		// can propagate the script's exit code verbatim — the land verb needs it.
		return true, err
	}
	return true, nil
}

// runBestEffortRepoScript is the best-effort policy over runTrustedRepoScript:
// an untrusted repo is SKIPPED with an observable debug note on stderr (never
// executed), and a missing script is a silent no-op. A script that runs and
// fails is also swallowed with a debug note — these are opportunistic
// maintenance hooks whose failure must never break the calling command. Used by
// the finding-compiler and prune-agents refresh sites.
func runBestEffortRepoScript(repoRoot, rel string, args ...string) {
	// executed is intentionally discarded: in best-effort mode an absent script
	// (executed=false, err=nil) is a silent no-op, so only the error cases below
	// emit an observable note.
	_, err := runTrustedRepoScript(repoRoot, rel, args...)
	switch {
	case errors.Is(err, errUntrustedRepoScript):
		fmt.Fprintf(os.Stderr, "ao: skipping repo script %s — untrusted checkout (running ao binary is not inside %s; set %s=1 to override)\n", rel, repoRoot, trustRepoEnvVar)
	case err != nil:
		fmt.Fprintf(os.Stderr, "ao: repo script %s failed (best-effort, ignored): %v\n", rel, err)
	}
}

// untrustedRepoScriptError builds the hard-error a command-site policy returns
// when a repo-relative script cannot be trusted. It names the reason and the
// escape hatch so the operator can opt in for a legitimate
// installed-ao-inside-its-own-checkout workflow.
func untrustedRepoScriptError(repoRoot, rel string) error {
	return fmt.Errorf("refusing to run repo script %s: the running ao binary is not inside the checkout %s, so this repo is untrusted (RCE guard). If this is genuinely your own checkout, set %s=1 to override", rel, repoRoot, trustRepoEnvVar)
}
