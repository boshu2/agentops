// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeVerifyTestRepo builds on writePawlTestRepo (the shared pawl fixture) and swaps
// the stub scripts/pawl-review.sh for one that RECORDS its argv to a marker file before
// exiting — proof of which script `ao verify` actually invoked and with what args.
// Restores verifyCmd's shared cobra state via t.Cleanup (runVerify passes verifyCmd
// into runPawlReview, which mutates SilenceUsage/SilenceErrors on error).
func writeVerifyTestRepo(t *testing.T, exitCode int) (markerPath string) {
	t.Helper()
	writePawlTestRepo(t, exitCode)
	repo := testProjectDir
	markerPath = filepath.Join(repo, "pawl-review-invoked-args")
	stub := "#!/usr/bin/env bash\nprintf '%s' \"$*\" > \"" + markerPath + "\"\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(repo, "scripts", "pawl-review.sh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	prevSU, prevSE := verifyCmd.SilenceUsage, verifyCmd.SilenceErrors
	t.Cleanup(func() {
		verifyCmd.SilenceUsage = prevSU
		verifyCmd.SilenceErrors = prevSE
	})
	return markerPath
}

// verdictShape normalizes an error from the review path into (exitCode, isVerdict):
// nil == CONFIRMED (0, true); *pawlReviewExitError == a verdict code; anything else
// is a non-verdict hard error (false).
func verdictShape(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	var exitErr *pawlReviewExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return -1, false
}

// `ao verify` must exist at the ROOT of the ao surface (the front-door verb),
// carrying the wedge headline as its one-line pitch.
func TestVerifyCmd_RegisteredAtRoot(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "verify" {
			if c != verifyCmd {
				t.Fatalf("root command %q is not verifyCmd (a second verify surface exists)", c.Name())
			}
			found = true
		}
	}
	if !found {
		t.Fatal("`ao verify` is not registered on rootCmd")
	}
	wantShort := "Independent cross-family verdict on your change — no verdict = not done"
	if verifyCmd.Short != wantShort {
		t.Fatalf("verifyCmd.Short = %q, want %q", verifyCmd.Short, wantShort)
	}
}

// The help text is product copy: the wedge headline, the visible advanced path
// (`ao pawl review` = same engine), the threat-model boundary, and the `ao doctor`
// pointer for failure paths must all be present.
func TestVerifyCmd_HelpCarriesWedgeCopy(t *testing.T) {
	for _, want := range []string{
		"no verdict = not done",
		"ao pawl review",
		"same engine",
		"single-operator",
		"ao doctor",
		"never a silent pass",
	} {
		if !strings.Contains(strings.ToLower(verifyCmd.Long), want) {
			t.Errorf("verifyCmd.Long is missing the wedge copy %q", want)
		}
	}
}

// KEEP-REF (age-rk3r.19): the REBOUND CI-scoping copy must now describe the SHIPPED
// keep-ref behavior — CI honors a REBOUND when the reviewed commit is reachable OR a
// keep-ref (refs/agentops/rebound/<C>) makes it fetchable, and the keep-ref is only a
// reachability aid re-checked against the ledger's R (a forged keep-ref cannot launder
// authorization). This locks the doc reconciliation so a future edit cannot silently
// revert it to the old "age-rk3r.19 tracks a keep-ref fix" forward-looking wording.
func TestVerifyCmd_HelpDocumentsReboundKeepRef(t *testing.T) {
	long := strings.ToLower(verifyCmd.Long)
	for _, want := range []string{
		"refs/agentops/rebound",   // the keep-ref namespace
		"keep-ref",                // named as the mechanism
		"fetchable",               // CI honors when reachable OR keep-ref makes it fetchable
		"forged keep-ref cannot launder", // the security invariant
	} {
		if !strings.Contains(long, strings.ToLower(want)) {
			t.Errorf("verifyCmd.Long is missing the REBOUND keep-ref copy %q", want)
		}
	}
	// And the OLD forward-looking wording must be gone (reconciled to shipped).
	if strings.Contains(long, "tracks a keep-ref") {
		t.Errorf("verifyCmd.Long still carries the stale 'tracks a keep-ref fix' forward-looking wording; reconcile it to the shipped behavior")
	}
}

// DELEGATION PROOF: `ao verify` must execute the SAME scripts/pawl-review.sh the
// `ao pawl review` path resolves, forward argv verbatim, and produce a verdict
// shape IDENTICAL to runPawlReview for every verdict exit code
// (0 CONFIRMED · 3 REFUTED · 4 advisory · 2 usage · 1 hard error).
// TestRunVerify_BareInvocationDefaultsLabel pins the zero-arg front door: bare
// `ao verify` in a git repo must default the change label to change-<sha12>
// and reach the engine, never exit 2 on missing positional (the README CLI
// block documents the bare form).
func TestRunVerify_BareInvocationDefaultsLabel(t *testing.T) {
	marker := writeVerifyTestRepo(t, 0)
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if err := runVerify(verifyCmd, nil); err != nil {
		t.Fatalf("bare runVerify: %v", err)
	}
	argv, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("stub never invoked: %v", readErr)
	}
	want := "change-" + sha
	if !strings.Contains(string(argv), want) {
		t.Fatalf("bare invocation argv = %q, want default label %q", string(argv), want)
	}
}

func TestRunVerify_DelegatesToPawlReviewEngine(t *testing.T) {
	args := []string{"age-test", "--scope", "head"}
	for _, code := range []int{0, 3, 4, 2, 1} {
		t.Run("exit-"+strconv.Itoa(code), func(t *testing.T) {
			marker := writeVerifyTestRepo(t, code)
			vCode, vIsVerdict := verdictShape(runVerify(verifyCmd, args))
			got, readErr := os.ReadFile(marker)
			if readErr != nil {
				t.Fatalf("ao verify did not invoke the pawl-review script: %v", readErr)
			}
			if string(got) != "age-test --scope head" {
				t.Fatalf("forwarded argv = %q, want %q (must forward verbatim)", got, "age-test --scope head")
			}
			// Same fixture through the advanced surface must yield the identical shape.
			writeVerifyTestRepo(t, code)
			pCode, pIsVerdict := verdictShape(runPawlReview(pawlReviewCmd, args))
			if vIsVerdict != pIsVerdict || vCode != pCode {
				t.Fatalf("ao verify (%d, verdict=%v) diverges from ao pawl review (%d, verdict=%v)",
					vCode, vIsVerdict, pCode, pIsVerdict)
			}
			if !vIsVerdict || vCode != code {
				t.Fatalf("verdict exit code = %d (verdict=%v), want %d propagated verbatim", vCode, vIsVerdict, code)
			}
		})
	}
}

// STRICT (age-rk3r.13): the two-family cold quorum posture must be documented on BOTH
// surfaces' help — the opt-in nature, the refuses-to-degrade invariant, the exit-5 HOLD,
// and the honest-UNAVAILABLE current posture (no active-protection claim). This is the
// documentary half; the behavioral forwarding is proven in TestRunVerify_StrictForwarded.
func TestVerifyCmd_HelpDocumentsStrict(t *testing.T) {
	for _, want := range []string{
		"--strict",
		"two-family",
		"refuses to degrade",
		"strict hold",   // the exit-5 HOLD is named
		"unavailable",   // the honest current posture
		"opt-in",
	} {
		if !strings.Contains(strings.ToLower(verifyCmd.Long), want) {
			t.Errorf("verifyCmd.Long is missing the strict copy %q", want)
		}
	}
	// The advanced surface (`ao pawl review`) documents strict too — same engine, same flag.
	for _, want := range []string{"--strict", "two-family", "refuses"} {
		if !strings.Contains(strings.ToLower(pawlReviewCmd.Long), want) {
			t.Errorf("pawlReviewCmd.Long is missing the strict copy %q", want)
		}
	}
	// Do NOT claim ACTIVE strict cross-family protection (the honesty boundary): the help must
	// state the machinery is built + unavailable, never that strict protection is live today.
	if !strings.Contains(verifyCmd.Long, "claims NO active strict") {
		t.Error("verifyCmd.Long must state it claims NO active strict protection (the honesty boundary)")
	}
}

// STRICT forwarding: `--strict` is a flag the SCRIPT owns (DisableFlagParsing), so `ao verify`
// must forward it verbatim, and the strict HOLD/UNAVAILABLE exit code (5) must propagate as a
// verdict shape — never be swallowed into a hard error. Mirrors TestRunVerify_DelegatesToPawlReviewEngine
// but pins the strict flag + the new non-authorizing exit 5.
func TestRunVerify_StrictForwarded(t *testing.T) {
	args := []string{"age-strict", "--scope", "head", "--strict"}
	marker := writeVerifyTestRepo(t, 5) // stub exits 5 (STRICT HOLD/UNAVAILABLE)
	vCode, vIsVerdict := verdictShape(runVerify(verifyCmd, args))
	got, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("ao verify did not invoke the pawl-review script: %v", readErr)
	}
	if string(got) != "age-strict --scope head --strict" {
		t.Fatalf("forwarded argv = %q, want %q (--strict must forward verbatim)", got, "age-strict --scope head --strict")
	}
	if !vIsVerdict || vCode != 5 {
		t.Fatalf("strict exit code = %d (verdict=%v), want 5 propagated verbatim as a verdict shape", vCode, vIsVerdict)
	}
	// Same fixture through the advanced surface must yield the identical shape.
	writeVerifyTestRepo(t, 5)
	pCode, pIsVerdict := verdictShape(runPawlReview(pawlReviewCmd, args))
	if vIsVerdict != pIsVerdict || vCode != pCode {
		t.Fatalf("ao verify strict (%d, verdict=%v) diverges from ao pawl review (%d, verdict=%v)",
			vCode, vIsVerdict, pCode, pIsVerdict)
	}
}

// --help is for ao's command, not the wrapped script: it must print verify's OWN
// help (the wedge copy) and never forward to the script (the exit-3 stub would fail).
func TestRunVerify_HelpDoesNotForwardToScript(t *testing.T) {
	marker := writeVerifyTestRepo(t, 3)
	var out bytes.Buffer
	verifyCmd.SetOut(&out)
	verifyCmd.SetErr(&out)
	t.Cleanup(func() {
		verifyCmd.SetOut(nil)
		verifyCmd.SetErr(nil)
	})
	if err := runVerify(verifyCmd, []string{"--help"}); err != nil {
		t.Fatalf("--help should print help and return nil, got %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("--help forwarded to the pawl-review script (marker written); it must not")
	}
	if !strings.Contains(strings.ToLower(out.String()), "no verdict = not done") {
		t.Fatalf("--help output is missing the wedge headline; got: %s", out.String())
	}
}

// A pre-verdict environment failure (missing script) must stay a plain error —
// never a fabricated verdict code — and point the user at `ao doctor`.
func TestRunVerify_EnvironmentFailurePointsAtDoctor(t *testing.T) {
	writeVerifyTestRepo(t, 0)
	if err := os.Remove(filepath.Join(testProjectDir, "scripts", "pawl-review.sh")); err != nil {
		t.Fatal(err)
	}
	err := runVerify(verifyCmd, []string{"age-test"})
	if err == nil {
		t.Fatal("missing pawl-review script should error (fail-closed), got nil")
	}
	if _, isVerdict := verdictShape(err); isVerdict {
		t.Fatalf("environment failure must not masquerade as a verdict exit code: %v", err)
	}
	if !strings.Contains(err.Error(), "ao doctor") {
		t.Fatalf("environment failure must point at `ao doctor`, got: %v", err)
	}
}

// verifyCfgEnvVars mirrors the canonical env surface the config reader honors;
// tests clear them so a runner exporting a PAWL_* var cannot poison provenance.
var verifyCfgEnvVars = []string{
	"PAWL_REVIEWER_CHAIN", "PAWL_REVIEW_TIMEOUT", "PAWL_STRICT",
	"PAWL_SMOKE_CMD", "PAWL_AUTOBIND", "PAWL_AUTHOR_FAMILY",
}

func clearVerifyCfgEnv(t *testing.T) {
	t.Helper()
	for _, name := range verifyCfgEnvVars {
		t.Setenv(name, "") // snapshots + auto-restores the prior set/unset state
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
}

// mkVerifyCfgRepo creates a temp git-marked repo (with an optional .aoverify.yaml)
// and chdirs into it so verifycfg.Load() resolves it. cwd auto-restores.
func mkVerifyCfgRepo(t *testing.T, yaml string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if yaml != "" {
		if err := os.WriteFile(filepath.Join(root, ".aoverify.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
}

// `--show-config` must short-circuit BEFORE the review engine (return nil without
// forwarding — a missing pawl script would otherwise error), read the repo's own
// .aoverify.yaml, print effective values with provenance, and route unknown-key
// warnings to stderr.
func TestRunVerify_ShowConfigReadsRepoPolicy(t *testing.T) {
	clearVerifyCfgEnv(t)
	mkVerifyCfgRepo(t, "review_timeout: 600\nauthor_family: gpt5\nbogus_key: 1\n")

	cmd := &cobra.Command{}
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)

	if err := runVerify(cmd, []string{"--show-config"}); err != nil {
		t.Fatalf("--show-config returned error (must not forward to engine): %v", err)
	}
	for _, want := range []string{"config file:", ".aoverify.yaml", "review_timeout", "600", "file", "author_family", "gpt5"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--show-config output missing %q; got:\n%s", want, out.String())
		}
	}
	if !strings.Contains(errb.String(), "unknown key(s) ignored: bogus_key") {
		t.Errorf("unknown-key warning not on stderr; got: %s", errb.String())
	}
}

// Env override wins and --show-config must show the env provenance.
func TestRunVerify_ShowConfigEnvOverride(t *testing.T) {
	clearVerifyCfgEnv(t)
	mkVerifyCfgRepo(t, "review_timeout: 600\n")
	t.Setenv("PAWL_REVIEW_TIMEOUT", "999")

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runVerify(cmd, []string{"--show-config"}); err != nil {
		t.Fatalf("--show-config: %v", err)
	}
	line := findLine(out.String(), "review_timeout")
	if !strings.Contains(line, "999") || !strings.Contains(line, "env") {
		t.Errorf("review_timeout line = %q, want value 999 with source env", line)
	}
}

// Zero-config: --show-config prints all defaults and returns cleanly.
func TestRunVerify_ShowConfigZeroConfig(t *testing.T) {
	clearVerifyCfgEnv(t)
	mkVerifyCfgRepo(t, "") // no .aoverify.yaml

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runVerify(cmd, []string{"--show-config"}); err != nil {
		t.Fatalf("--show-config: %v", err)
	}
	for _, want := range []string{"config file: none", "review_timeout", "300", "default", "autobind", "true"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("zero-config --show-config missing %q; got:\n%s", want, out.String())
		}
	}
}

// `--export-env` emits shell-eval lines for non-default keys only (the bridge
// bead age-rk3r.17 sources); bools render as 0/1.
func TestRunVerify_ExportEnvEmitsShellLines(t *testing.T) {
	clearVerifyCfgEnv(t)
	mkVerifyCfgRepo(t, "review_timeout: 600\nautobind: false\n")

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runVerify(cmd, []string{"--export-env"}); err != nil {
		t.Fatalf("--export-env: %v", err)
	}
	want := "export PAWL_REVIEW_TIMEOUT='600'\nexport PAWL_AUTOBIND='0'\n"
	if out.String() != want {
		t.Errorf("--export-env = %q, want %q", out.String(), want)
	}
}

// findLine returns the first line containing sub, or "".
func findLine(s, sub string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, sub) {
			return ln
		}
	}
	return ""
}
