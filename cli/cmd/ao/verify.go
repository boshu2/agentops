// practices: [design-by-contract, code-complete]
package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/verifycfg"
	"github.com/spf13/cobra"
	"os/exec"
	"strings"
)

// verifyCmd is the canonical front-door verb onto the pawl review engine
// (age-wedge-all-in-dyr0.1). It is a THIN alias: every argument is forwarded
// verbatim into runPawlReview — the exact RunE path `ao pawl review` uses — so
// the review engine, the embedded-bundle stranger path, the aoBinaryInside
// trust split, and the sanitized cold env are REUSED, never re-derived
// (a parallel implementation is a reject condition on this surface).
var verifyCmd = &cobra.Command{
	Use:   "verify <change-id> [--scope head|staged] [--converge] [--author-family <fam>] [--context <s>]",
	Short: "Independent cross-family verdict on your change — no verdict = not done",
	Long: `Run an independent cross-family review of your change and, on CONFIRMED, write
the commit-bound verdict to the provenance ledger. No verdict = not done: the
reviewer is a fresh-context model from a DIFFERENT family than the author
(never a same-model self-review), so "looks good to me" from the agent that
wrote the code never counts as done.

'ao verify' is the front door to the same engine as 'ao pawl review' (the
advanced surface — service panes, converge lineage, gate wiring); every
argument is forwarded verbatim, and the exit code IS the verdict:

  0  CONFIRMED — verdict written and bound to the commit
  3  REFUTED — defects printed; fix and re-run
  4  advisory-only (--converge without adversarial lineage)
  2  usage error
  1  hard error — always fail-closed, never a silent pass

Inside the AgentOps checkout it runs the live repo scripts (dogfood); anywhere
else it runs the embedded review bundle against YOUR git repository with a
sanitized environment, so any repo works zero-config.

THREAT MODEL: single-operator, own-repo. The sanitized stranger path stops a
repo under review from hijacking the reviewer (planted binaries, BASH_ENV,
external diff), but it is NOT a defense for adversarial multi-tenant hosting —
you verify code you chose to check out.

If the run fails before a verdict (no codex/agy reviewer installed, not a git
repository, bash missing), that is an environment problem, not a verdict —
run 'ao doctor' to diagnose.

Per-repo policy (age-rk3r.5): an optional committed .aoverify.yaml at the repo
root sets a durable, reviewable verify policy without exporting PAWL_* by hand.
Precedence is env > file > default; zero config is byte-identical to today.

  --show-config   print the EFFECTIVE config and where each value came from
  --export-env    emit "export PAWL_*=..." lines for the shell to eval (the
                  bridge the pawl shell sources; only non-default values emitted)

--rebind (age-rk3r.9): authorize a rebase that is the SAME change WITHOUT a full re-review.
Instead of paying a fresh cross-family review for a rebase whose diff is unchanged (same
diff bytes, new sha/date — the reviewer would read the same bytes), re-bind the prior
CONFIRMED verdict onto the new tip as a DISTINCT REBOUND verdict with lineage. Permitted
ONLY when ALL THREE hold: (a) git patch-id --stable of the reviewed diff equals the new
tip's (the rebase-stable key), AND (b) the +/- content lines are byte-identical whitespace-
SIGNIFICANT (patch-id is whitespace-insensitive, so (a) alone would pass a whitespace-only
change like Python indentation), AND (c) the full local gate is green on the new tip (ao
gate check --scope head). CAVEAT: a matching patch-id proves the change is rebase-stable but
is whitespace-INSENSITIVE, so it does NOT prove the diff bytes are unchanged, and even byte-
identical content on a new base can still break (semantic conflict). REBOUND therefore
requires (a) the same rebase-stable patch-id AND (b) byte-identical +/- content lines AND
(c) a green full gate on the new tip. A REBOUND never authorizes a merge unless its lineage
is a fully-valid CONFIRMED, its patch-id matches, and its content bytes match — never
forgeable, and no easier to authorize than the CONFIRMED it inherits.

LIMITATION (age-rk3r.18): REBOUND is honored by pawl-verdict check (the reconcile/merge
path); the portable 'ao verify init' pre-push gate + CI honor only CONFIRMED today — a
REBOUND edge is safely refused there (fail-closed) until age-rk3r.18 wires Go-side REBOUND
validation. So --rebind's convenience applies on the operator/reconcile merge path; the
installed pre-push ratchet will still ask for a CONFIRMED until then.

  ao verify <id> --rebind [--head SHA] [--from-verdict PATH] [--dir DIR]

Examples:
  ao verify my-change-123                # review + certify HEAD
  ao verify my-change --scope staged     # review staged work (advisory — no commit to bind)
  ao verify --show-config                # inspect effective verify policy for this repo
  ao verify my-change-123 --rebind       # re-bind the CONFIRMED verdict onto HEAD after a no-op rebase`,
	// The pawl review surface owns the flag contract; forward everything verbatim.
	DisableFlagParsing: true,
	RunE:               runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

// headShortSHA returns the 12-char HEAD sha of the repo at cwd, or "" when
// not in a git repo (the engine then reports its own usage error).
func headShortSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runVerify delegates to runPawlReview — the SAME code path `ao pawl review`
// executes — and only decorates non-verdict failures with the `ao doctor`
// pointer. Verdict exit codes (*pawlReviewExitError) propagate verbatim:
// the exit code IS the verdict.
func runVerify(cmd *cobra.Command, args []string) error {
	// Config-inspection flags short-circuit BEFORE the review engine: they read
	// the per-repo policy and print, never forwarding to runPawlReview. Flag
	// parsing is disabled on this command, so scan the raw args.
	for _, a := range args {
		switch a {
		case "--show-config":
			return runVerifyShowConfig(cmd)
		case "--export-env":
			return runVerifyExportEnv(cmd)
		case "--rebind":
			// The SAFE, patch-id-gated re-bind (age-rk3r.9): authorize a byte-identical
			// rebase WITHOUT a full re-review. Short-circuits BEFORE the review engine —
			// it does not run a fresh review, it REUSES a proven-identical prior one — and
			// forwards to `pawl-verdict.sh rebind-verified`.
			return runVerifyRebind(cmd, args)
		}
	}

	// Bare `ao verify` (the README headline form) defaults the change label to
	// the short HEAD sha instead of erroring: the engine requires a positional
	// label, but the front door should not fail on its documented zero-arg use.
	if len(args) == 0 {
		if sha := headShortSHA(); sha != "" {
			args = []string{"change-" + sha}
		}
	}
	err := runPawlReview(cmd, args)
	var exitErr *pawlReviewExitError
	if err == nil || errors.As(err, &exitErr) {
		return err
	}
	return fmt.Errorf("ao verify: %w\nthis is an environment failure, not a verdict (fail-closed — never a silent pass); run 'ao doctor' to diagnose reviewer/tooling setup", err)
}

// runVerifyShowConfig prints the effective per-repo verify config plus the
// provenance (env/file/default) of every value, for debugging a stranger repo.
func runVerifyShowConfig(cmd *cobra.Command) error {
	cfg := verifycfg.Load()
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "ao verify — effective config (precedence: env > file > default)")
	if cfg.FileFound {
		fmt.Fprintf(out, "config file: %s\n", cfg.ConfigPath)
	} else {
		fmt.Fprintf(out, "config file: none (no %s at repo root; using env + defaults)\n", verifycfg.ConfigFileName)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %-15s %-14s %-8s %s\n", "KEY", "VALUE", "SOURCE", "ENV OVERRIDE")
	for _, e := range cfg.Entries() {
		val := e.Value
		if val == "" {
			val = "(unset)"
		}
		fmt.Fprintf(out, "  %-15s %-14s %-8s %s\n", e.Key, val, e.Source, e.EnvVar)
	}
	printVerifyCfgWarnings(cmd.ErrOrStderr(), cfg.Warnings)
	return nil
}

// runVerifyExportEnv emits the shell bridge (age-rk3r.17) for `eval "$(...)"`.
func runVerifyExportEnv(cmd *cobra.Command) error {
	cfg := verifycfg.Load()
	fmt.Fprint(cmd.OutOrStdout(), cfg.ExportEnv())
	printVerifyCfgWarnings(cmd.ErrOrStderr(), cfg.Warnings)
	return nil
}

// printVerifyCfgWarnings writes each non-fatal config warning once to stderr.
func printVerifyCfgWarnings(w io.Writer, warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(w, "ao verify: %s\n", warning)
	}
}

// trustedHeadSHA resolves `git -C repo rev-parse <spec> HEAD` using the SAME trusted git
// the pawl gate uses (trustedGit → an ABSOLUTE git on a PATH stripped of empty/./relative/
// repo-internal entries), so a planted `$REPO/bin/git` on the ambient PATH can NEVER execute
// (age-rk3r.9 trust-boundary fix, mirroring .1/.6/.12 / runPawlReview / verify_prepush.go).
// spec is a rev-parse flag ("" for the full 40-char sha, "--short=12" for the label form).
// Returns "" on any failure (no trusted git, not a git repo) — the caller fail-closes with
// a clear message rather than silently falling back to ambient git.
func trustedHeadSHA(repo, spec string) string {
	gitBin, err := trustedGit(repo)
	if err != nil {
		return ""
	}
	rpArgs := []string{"-C", repo, "rev-parse"}
	if spec != "" {
		rpArgs = append(rpArgs, spec)
	}
	rpArgs = append(rpArgs, "HEAD")
	out, err := exec.Command(gitBin, rpArgs...).Output() // #nosec G204 -- gitBin is trusted-PATH-resolved (absolute, repo-internal entries stripped); repo is the resolved git toplevel; spec is a fixed literal.
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runVerifyRebind implements `ao verify --rebind` (age-rk3r.9): the SAFE, patch-id-gated
// re-bind that authorizes a BYTE-IDENTICAL rebase WITHOUT a full re-review. It is a thin
// pass-through onto `scripts/pawl-verdict.sh rebind-verified` (resolved live-or-embedded
// under the same trust split as `ao pawl review`), which does the real work: verify
// git patch-id --stable of the reviewed diff equals the new tip's AND the full local gate
// is green on the new tip, then write a DISTINCT REBOUND verdict carrying lineage
// (rebound_from_verdict / rebound_from_sha / patch_id_proof). A REBOUND is never forgeable
// as a fresh CONFIRMED — `pawl-verdict.sh check` authorizes it only when its lineage was
// CONFIRMED and its patch_id_proof matches the new tip.
//
// Usage: ao verify <change-id> --rebind [--head SHA] [--from-verdict PATH] [--dir DIR]
//   change-id  the bead/verdict id (defaults to change-<HEAD-short> like bare `ao verify`)
//   --head     the new tip to re-bind onto (defaults to the repo's current full HEAD sha)
//   --from-verdict  the prior CONFIRMED verdict to descend from (defaults to <dir>/<id>.json)
//   --dir      the verdicts dir (forwarded verbatim; defaults to .agents/pawl-verdicts)
//
// CAVEAT (load-bearing): a matching patch-id proves the change is rebase-stable but is
// WHITESPACE-INSENSITIVE, so it does NOT prove the diff bytes are unchanged, and even byte-
// identical content on a new base can still break (semantic conflict). REBOUND therefore
// requires (a) the same rebase-stable patch-id AND (b) byte-identical +/- content lines
// (whitespace-significant) AND (c) a green full gate on the new tip — all enforced by
// rebind-verified, plus a fully-valid CONFIRMED lineage re-checked at merge time.
func runVerifyRebind(cmd *cobra.Command, args []string) error {
	var changeID, head, fromVerdict, dir string
	// Parse the raw args (flag parsing is disabled on `ao verify`). The first non-flag
	// token is the change-id; --rebind itself is consumed here.
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--rebind":
			// consumed
		case "--head":
			if i+1 < len(args) {
				head = args[i+1]
				i++
			}
		case "--from-verdict":
			if i+1 < len(args) {
				fromVerdict = args[i+1]
				i++
			}
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(a, "-") && changeID == "" {
				changeID = a
			}
		}
	}
	// TRUST-BOUNDARY ORDERING (age-rk3r.9): resolve the target repo (workdir) FIRST, BEFORE
	// resolving any default head/change-id from git. The old code ran ambient exec.Command("git")
	// on the process PATH to default the head — on the stranger/embedded path that PATH still
	// includes the repo under review, so a planted `$REPO/bin/git` early on PATH executed
	// repo-controlled code (the exact planted-binary hole runPawlReview / trustedGit close
	// elsewhere in the flow). resolvePawlVerdictScript establishes workdir = the target repo;
	// the default head/change-id are then resolved via trustedHeadSHA(workdir, …) which uses
	// trustedGit (an absolute git on a PATH stripped of repo-internal entries), so no planted
	// git can run.
	script, workdir, extraEnv, cleanup, err := resolvePawlVerdictScript()
	if err != nil {
		return fmt.Errorf("ao verify --rebind: %w\nrun 'ao doctor' to diagnose reviewer/tooling setup", err)
	}
	defer cleanup()

	if changeID == "" {
		if sha := trustedHeadSHA(workdir, "--short=12"); sha != "" {
			changeID = "change-" + sha
		} else {
			return fmt.Errorf("ao verify --rebind: could not resolve a change-id via trusted git (not in a git repo, or no trusted git on PATH) — pass one explicitly: ao verify <change-id> --rebind")
		}
	}
	if head == "" {
		head = trustedHeadSHA(workdir, "")
		if head == "" {
			return fmt.Errorf("ao verify --rebind: could not resolve HEAD via trusted git (not in a git repo, or no trusted git on PATH) — pass the new tip explicitly with --head <sha>")
		}
	}

	// ZERO-CONFIG verdict-dir resolution (must run against the USER's TARGET REPO, not the
	// extracted bundle). On the stranger/embedded path the extracted pawl-verdict.sh defaults
	// VDIR to $SCRIPT_DIR/../.agents/pawl-verdicts — i.e. the TEMP BUNDLE dir, NOT the user's
	// repo — so without an explicit --dir it would look for the prior verdict in the bundle
	// and falsely report "not found". Mirror how `ao verify receipts` (runVerifyReceiptsEmbedded)
	// roots its work in the target repo: resolvePawlVerdictScript already returns `workdir` =
	// the repo being operated on (userRoot on the stranger path via gitToplevel; the AgentOps
	// checkout root on the in-checkout dogfood path), so default --dir to
	// <workdir>/.agents/pawl-verdicts. An explicit user --dir still wins.
	if dir == "" {
		dir = filepath.Join(workdir, ".agents", "pawl-verdicts")
	}

	// PR is 0 for the push-to-main lane (the same convention pawl-verdict.sh uses for a
	// direct push without a PR). Forward the resolved args verbatim to rebind-verified.
	fwd := []string{"rebind-verified", changeID, "0", "--head", head, "--dir", dir}
	if fromVerdict != "" {
		fwd = append(fwd, "--from-verdict", fromVerdict)
	}
	// --repo-root pins the git ops to the repo being operated on (the in-checkout root or
	// the user's own repo on the embedded path), matching where the review + gate run.
	fwd = append(fwd, "--repo-root", workdir)

	rerr := runForwardedPawlScript(cmd, script, workdir, untrustedRootForEnv(extraEnv, workdir), fwd, extraEnv)
	var exitErr *pawlReviewExitError
	if rerr == nil || errors.As(rerr, &exitErr) {
		return rerr
	}
	return fmt.Errorf("ao verify --rebind: %w\nthis is an environment failure, not a verdict (fail-closed); run 'ao doctor' to diagnose", rerr)
}

// untrustedRootForEnv returns the untrusted-root argument for runForwardedPawlScript:
// "" on the in-checkout path (extraEnv nil → PATH already trusted), or the workdir on the
// stranger path (extraEnv non-nil → the repo under review is untrusted while it is cwd, so
// bash must resolve on a PATH that excludes it). Mirrors runPawlReview's two call sites.
func untrustedRootForEnv(extraEnv []string, workdir string) string {
	if extraEnv == nil {
		return ""
	}
	return workdir
}
