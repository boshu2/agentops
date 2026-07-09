// practices: [design-by-contract, code-complete]
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// landReexecEnv marks a re-exec'd child of `ao land` so it (1) does NOT rebuild the
// fresh binary again (the parent just built it) and (2) is a belt-and-suspenders
// guard against a re-exec loop (the realpath check below is the primary guard).
const landReexecEnv = "AO_LAND_REEXECED"

// landExitError carries the process exit code `ao land` maps to, so the exit code
// is meaningful on any failure (a re-exec'd child's code, a pawl-land / gate / push
// failure). Mirrors pawlReviewExitError; wired into root.go's Execute.
type landExitError struct{ code int }

func (e *landExitError) Error() string { return "" }

// ExitCode returns the process exit code this land outcome maps to.
func (e *landExitError) ExitCode() int { return e.code }

var landCmd = &cobra.Command{
	Use:     "land <bead-id>",
	GroupID: "workflow",
	Short:   "Land a bead through the trusted pawl path with a fresh in-checkout binary, then atomic pawl-land",
	Long: `Land one reviewed bead the trusted way, in one verb. 'ao land' builds NOTHING
new — it turns ON shipped-but-off-path optimizations that the INSTALLED ao skips.

The recurring land-friction it fixes: an installed ~/.local/bin/ao fails
aoBinaryInside(repoRoot), so 'ao pawl review' on your own checkout takes the
stranger/UNTRUSTED path (cold review, PAWL_NO_SERVICE=1, no verdict auto-bind).
'ao land' closes that gap deterministically:

  0. Build a fresh in-checkout binary (cli/bin/ao) so the review runs under a binary
     that is BOTH HEAD-fresh AND physically inside the checkout — aoBinaryInside()
     passes, so the review takes the LIVE (trusted) path: warm auto-up + deterministic
     preflight + verdict AUTO-BIND. Re-exec the whole verb through that fresh binary.
  1. Pin AO_BIN to the fresh binary for every child step (the live path passes
     extraEnv=nil, so it does NOT pin AO_BIN itself — pin it here so preflight + the
     pre-push gate never fall back to a stale in-checkout binary).
  2. Warm-service liveness — bring the standing pawl-service up once if it is down
     (best-effort; a cold review still works, never a hard fail on warm-up).
  3. Run 'ao pawl review <bead> --scope head' on the LIVE path — auto-bind fires on
     CONFIRM (emits the single #trivial verdict commit). REFUTED / NO-VERDICT stops
     here (exit non-zero, no land).
  4. On CONFIRM, hand off to scripts/pawl-land.sh (fetch → rebase origin/main →
     restamp the verdict onto the post-rebase feat → single push through the gate),
     then scripts/post-land-provenance-emit.sh for the trunk-bound landed edge.

RCE SAFETY: the same aoBinaryInside(repoRoot) trust test the pawl already uses gates
every repo-script exec (never forgeable marker files). 'ao land' requires a genuine
AgentOps checkout; from a stale installed ao, bootstrap once with
'cd <checkout> && make build && ./cli/bin/ao land <bead>'.`,
	Args: cobra.ExactArgs(1),
	RunE: runLand,
}

func init() {
	rootCmd.AddCommand(landCmd)
}

// --- seams (overridable in tests so the unit test never runs a real go build, a
// real re-exec, a live codex pawl, or a real push) ---

// landBuildFreshBinary builds a fresh cli/bin/ao inside the checkout (Step 0). ~1s.
var landBuildFreshBinary = func(repoRoot, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("preparing %s: %w", filepath.Dir(dest), err)
	}
	c := exec.Command("go", "build", "-o", dest, "./cmd/ao") // #nosec G204 -- fixed argv; dest is repoRoot/cli/bin/ao (a path we control), never user input.
	c.Dir = filepath.Join(repoRoot, "cli")
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
}

// landReexec re-runs the whole `land` verb under the fresh binary (Step 1). Returns
// the child's exit code (0 on success).
var landReexec = func(cmd *cobra.Command, freshBin string, args []string) (int, error) {
	childArgs := append([]string{"land"}, args...)
	c := exec.Command(freshBin, childArgs...) // #nosec G204 -- freshBin is the cli/bin/ao we just built inside the resolved checkout; args carry the operator's bead id.
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Env = append(os.Environ(), landReexecEnv+"=1")
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

// landEnsureWarmService brings the standing pawl-service up once if it is down
// (Step 2, best-effort). Never fails the land — a cold review still works, and
// pawl-review lazy-auto-ups on the LIVE path anyway; this just makes the warm
// intent explicit. Routes through the trusted-script boundary (aoBinaryInside).
var landEnsureWarmService = func(cmd *cobra.Command, repoRoot, aoBin string) {
	// Already healthy? Nothing to do (a non-zero health exit means down/absent).
	if _, err := runTrustedRepoScriptStreaming(repoRoot, defaultPawlServiceScript,
		nil, nil, nil, []string{"AO_BIN=" + aoBin}, "health"); err == nil {
		return
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "ao land: standing pawl-service not up — bringing it up once (best-effort; cold review still works)…")
	if _, err := runTrustedRepoScriptStreaming(repoRoot, defaultPawlServiceScript,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), []string{"AO_BIN=" + aoBin}, "up"); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "ao land: pawl up failed (best-effort, ignored — review will run cold): %v\n", err)
	}
}

// landRunReview runs the existing `ao pawl review <bead> --scope head` machinery
// in-process (Step 3). Because the caller is the fresh in-checkout binary,
// runPawlReview takes the LIVE (trusted) path — auto-bind fires on CONFIRM. It
// returns nil on CONFIRM (exit 0) and a *pawlReviewExitError on REFUTED/NO-VERDICT.
var landRunReview = func(cmd *cobra.Command, bead string) error {
	return runPawlReview(cmd, []string{bead, "--scope", "head"})
}

// landRunScript runs a repo land script (scripts/pawl-land.sh / post-land-provenance)
// with stdio streamed and AO_BIN pinned, behind the aoBinaryInside RCE boundary
// (Step 4). Returns (executed, err) — executed=false means the script is absent.
var landRunScript = func(cmd *cobra.Command, repoRoot, aoBin, rel string, args ...string) (bool, error) {
	return runTrustedRepoScriptStreaming(repoRoot, rel,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
		[]string{"AO_BIN=" + aoBin}, args...)
}

// runningAoIsFresh reports whether the currently-running ao binary IS the fresh
// in-checkout binary (realpath-compared, so a symlinked build compares correctly).
// When true, we are on the LIVE/trusted path and need not re-exec.
func runningAoIsFresh(freshBin string) bool {
	self, err := pawlSelfBinary()
	if err != nil || self == "" {
		return false
	}
	return realpathOrSelf(self) == realpathOrSelf(freshBin)
}

func runLand(cmd *cobra.Command, args []string) error {
	bead := strings.TrimSpace(args[0])
	if bead == "" {
		return fmt.Errorf("ao land: <bead-id> must not be empty")
	}

	// 'ao land' drives the trusted pawl path from YOUR OWN build, so it requires a
	// genuine AgentOps checkout. An installed ao pointed at a foreign repo fails here.
	repoRoot, err := resolveAgentsRepoRoot()
	if err != nil {
		return fmt.Errorf("ao land must run inside the AgentOps checkout (it drives the trusted pawl path from your own build): %w", err)
	}
	freshBin := filepath.Join(repoRoot, "cli", "bin", "ao")
	reexeced := os.Getenv(landReexecEnv) == "1"

	// Step 0: build a fresh in-checkout binary so the review runs the LIVE (trusted)
	// path — HEAD-fresh AND physically inside the checkout ⇒ aoBinaryInside() passes ⇒
	// warm auto-up + preflight + auto-bind all enabled. Skipped in the re-exec'd child
	// (its parent just built it).
	if !reexeced {
		fmt.Fprintf(cmd.ErrOrStderr(), "ao land: building a fresh in-checkout ao (→ %s) so the review takes the trusted path (warm auto-up + auto-bind)…\n", freshBin)
		if buildErr := landBuildFreshBinary(repoRoot, freshBin); buildErr != nil {
			return fmt.Errorf("ao land: building the fresh in-checkout ao binary failed (fix the build, then re-run): %w", buildErr)
		}
	}

	// Step 1: re-exec the whole verb through the fresh binary if we are not it, so
	// aoBinaryInside(repoRoot) passes for every step that follows (the trusted path).
	if !runningAoIsFresh(freshBin) {
		code, reErr := landReexec(cmd, freshBin, args)
		if reErr != nil {
			return fmt.Errorf("ao land: re-exec through the fresh binary %s failed: %w", freshBin, reErr)
		}
		if code != 0 {
			return &landExitError{code: code}
		}
		return nil
	}

	// Step 2: pin AO_BIN so preflight + verdict emit + the pre-push gate all share the
	// ONE fresh binary. VERIFIED GAP: the live path passes extraEnv=nil, so unlike the
	// cold path it does NOT pin AO_BIN — without this pin a stale in-checkout binary
	// could resolve for those child steps.
	if err := os.Setenv("AO_BIN", freshBin); err != nil {
		return fmt.Errorf("ao land: pinning AO_BIN=%s: %w", freshBin, err)
	}

	// Step 3 (warm-service liveness, best-effort — never hard-fails on warm-up).
	landEnsureWarmService(cmd, repoRoot, freshBin)

	// Step 4: run the pawl review on the LIVE path. auto-bind fires on CONFIRM; a
	// REFUTED / NO-VERDICT stops the land here (the review already printed the verdict
	// + defects and carries its own exit code).
	fmt.Fprintf(cmd.ErrOrStderr(), "ao land: running the cross-family pawl review for %s (scope head, trusted path)…\n", bead)
	if reviewErr := landRunReview(cmd, bead); reviewErr != nil {
		return reviewErr
	}

	// Step 5: CONFIRM — the auto-bind emitted the single #trivial verdict commit. Hand
	// off to the atomic land machinery (rebase → restamp under PAWL_AUTOBIND=0 → single
	// push through the gate).
	fmt.Fprintf(cmd.ErrOrStderr(), "ao land: CONFIRMED — handing off to scripts/pawl-land.sh (rebase → restamp → single push)…\n")
	executed, landErr := landRunScript(cmd, repoRoot, freshBin, "scripts/pawl-land.sh", bead)
	if landErr != nil {
		var exitErr *exec.ExitError
		if errors.As(landErr, &exitErr) {
			// A pawl-land failure (rebase conflict, gate red, push race) already printed
			// its reason via streamed stdio; propagate the exit code, no double-wrap.
			return &landExitError{code: exitErr.ExitCode()}
		}
		return fmt.Errorf("ao land: scripts/pawl-land.sh failed: %w", landErr)
	}
	if !executed {
		return fmt.Errorf("ao land: scripts/pawl-land.sh is missing from %s — cannot complete the land", repoRoot)
	}

	// Reconcile trunk-bound provenance for the just-landed range (best-effort,
	// non-blocking, idempotent — the pre-push hook also emits it; emit-landed dedups
	// by edge identity so a second pass records nothing new).
	if _, provErr := landRunScript(cmd, repoRoot, freshBin, "scripts/post-land-provenance-emit.sh"); provErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "ao land: post-land provenance emit failed (best-effort, ignored): %v\n", provErr)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "ao land: LANDED %s.\n", bead)
	return nil
}
