// practices: [design-by-contract, code-complete]
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/boshu2/agentops/cli/internal/verifycfg"
)

// hookMarker is the unique substring both marker lines of the installed hook
// carry; its presence identifies a hook this command manages (idempotency +
// safe --remove). It must match scripts/hooks/pre-push-verify.template.
const hookMarker = "AGENTOPS-VERIFY-RATCHET"

// aoBinPlaceholder is the token the embedded template carries where the absolute
// path of the installing ao binary is substituted at install time.
const aoBinPlaceholder = "@@AO_BIN@@"

// embeddedHookTemplatePath is the location of the hook template inside PawlFS.
const embeddedHookTemplatePath = "pawl/hooks/pre-push-verify.template"

// origHookName is the sidecar filename a pre-existing pre-push hook is set aside
// as, so it can be chained (run first) and restored byte-identically on --remove.
const origHookName = "pre-push.agentops-orig"

var verifyInitRemove bool

// verifyInitCmd installs (or --remove uninstalls) the portable pre-push verdict
// ratchet in the current repository: the local, sovereign complement to CI that
// makes "no verdict = not done" MECHANICAL in any repo (age-rk3r.6).
var verifyInitCmd = &cobra.Command{
	Use:   "init [--remove]",
	Short: "Install the portable pre-push verdict ratchet (a local hook enforcing no verdict = not done)",
	Long: `Install a pre-push hook into THIS git repository that refuses any push to
main/master whose commits lack proof — a commit-bound CONFIRMED cross-family
verdict in docs/provenance/ledger.jsonl, or the provenance-only #trivial waiver —
and that also verifies the ledger's tamper-evident hash chain. It is the sovereign
LOCAL complement to the CI verdict backstop: the same shape that held this repo
honest through its own lands, portable to any repo, enforced with NO repo-local
scripts (the hook body is shipped from the ao binary and delegates the whole gate
to ao, so a repo cannot subvert its own gate).

The hook is idempotent (re-running refreshes it in place), respects a pre-existing
pre-push hook (it is set aside, chained to run first, and restored byte-identically
on --remove), and bakes in an ao-version floor so a binary too old to read the
current ledger refuses-with-upgrade instead of reporting a false broken chain.

Nothing else is owed: 'ao verify' already bootstraps the ledger + genesis on first
append, so init only installs the ratchet.

SCOPE: this ratchet defends against honest mistakes and misconfiguration, NOT an
adversarial repository actively subverting its own gate. A repo-relative
core.hooksPath, a repo that rewrites its own installed hook, or an operator who
runs 'git push --no-verify' can bypass it by design — that is the parked
adversarial-multi-tenant threat (PRODUCT.md), out of scope for the portable
ratchet.

  ao verify init            # install / refresh the pre-push ratchet
  ao verify init --remove   # uninstall, restoring any pre-existing hook

After init, produce a verdict with:  ao verify <change-id>`,
	Args: cobra.NoArgs,
	RunE: runVerifyInit,
}

func init() {
	verifyInitCmd.Flags().BoolVar(&verifyInitRemove, "remove", false,
		"Uninstall the ratchet, restoring any pre-existing pre-push hook byte-identically")
	verifyCmd.AddCommand(verifyInitCmd)
}

func runVerifyInit(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	out := cmd.OutOrStdout()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	repo, err := gitToplevel(cwd)
	if err != nil {
		return fmt.Errorf("ao verify init must run inside a git repository: %w", err)
	}
	hooksDir, err := resolveHooksDir(repo)
	if err != nil {
		return err
	}
	hookPath := filepath.Join(hooksDir, "pre-push")
	origPath := filepath.Join(hooksDir, origHookName)

	if verifyInitRemove {
		return removeRatchet(out, hookPath, origPath)
	}
	return installRatchet(out, repo, hooksDir, hookPath, origPath)
}

// resolveHooksDir returns the ABSOLUTE hooks directory git ITSELF will run hooks
// from, by asking git to resolve it: `git rev-parse --path-format=absolute
// --git-path hooks`. Delegating is load-bearing (honest-mistakes correctness):
// git honors core.hooksPath's OWN resolution — ~-expansion (core.hooksPath =
// ~/ao-hooks → $HOME/ao-hooks), absolute values, repo-relative values, AND the
// common-dir/worktree cases — all at once. The prior hand-rolled
// absolute-vs-repo-relative split installed a ~-value into <repo>/~/ao-hooks
// while git looked in $HOME/ao-hooks, so `ao verify init` reported success but
// the ratchet never ran (a silent fail-open for a legitimate operator config).
//
// This is DISTINCT from the parked adversarial core.hooksPath threat (see the
// SCOPE note near loadHookTemplate): that is a repo attacking itself; THIS is
// honoring a legitimate operator config exactly as git does. trustedGit backs
// gitStdout, so the round-4 trusted-git invariant (absolute git resolved outside
// the repo) is preserved. Fail-closed if git returns no usable absolute path.
func resolveHooksDir(repo string) (string, error) {
	out, err := gitStdout(repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return "", fmt.Errorf("ao verify init: cannot determine the hooks directory git uses "+
			"(git rev-parse --path-format=absolute --git-path hooks) — cannot install the ratchet (fail-closed): %w", err)
	}
	// Strip ONLY git's trailing line terminator, never TrimSpace: git preserves a
	// trailing-space core.hooksPath verbatim (`git -c core.hooksPath='/x ' rev-parse
	// --git-path hooks` emits `/x \n`), so trimming the space would install the hook
	// at `/x/pre-push` while git runs `/x /pre-push` — a silent fail-open. Trailing
	// spaces belong to the path; only the `\n` (or `\r\n`) delimiter is ours to drop.
	dir := strings.TrimSuffix(strings.TrimSuffix(out, "\n"), "\r")
	if dir == "" || !filepath.IsAbs(dir) {
		return "", fmt.Errorf("ao verify init: git returned an unusable hooks path %q — cannot install the ratchet (fail-closed)", dir)
	}
	return dir, nil
}

// installRatchet writes the ratchet hook, chaining a pre-existing foreign hook to
// a sidecar. Idempotent: re-running when our hook is already present refreshes it
// in place without re-chaining (never turning our own hook into the "original").
func installRatchet(out io.Writer, repo, hooksDir, hookPath, origPath string) error {
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir %s: %w", hooksDir, err)
	}
	body, err := loadHookTemplate(repo)
	if err != nil {
		return err
	}

	existing, existed := readFileIfExists(hookPath)
	alreadyOurs := existed && isAgentopsHook(existing)

	if existed && !alreadyOurs {
		// A foreign hook is present — set it aside for chaining. Never clobber a
		// sidecar that already holds a real original.
		if _, sideExists := readFileIfExists(origPath); sideExists {
			return fmt.Errorf("a chained-original sidecar already exists at %s but %s is a foreign hook — "+
				"resolve manually or run `ao verify init --remove` first", origPath, hookPath)
		}
		if err := os.Rename(hookPath, origPath); err != nil {
			return fmt.Errorf("set aside existing pre-push hook: %w", err)
		}
		fmt.Fprintf(out, "chained the pre-existing pre-push hook → %s (it will run first)\n", origHookName)
	}

	if err := os.WriteFile(hookPath, []byte(body), 0o755); err != nil { // #nosec G306 -- a git hook must be executable.
		return fmt.Errorf("write pre-push hook: %w", err)
	}
	// WriteFile honors umask; force the exec bits so the hook always runs.
	if err := os.Chmod(hookPath, 0o755); err != nil { // #nosec G302 -- a git hook must be executable.
		return fmt.Errorf("chmod pre-push hook: %w", err)
	}

	if alreadyOurs {
		fmt.Fprintf(out, "refreshed the AgentOps pre-push verdict ratchet at %s\n", hookPath)
	} else {
		fmt.Fprintf(out, "installed the AgentOps pre-push verdict ratchet at %s\n", hookPath)
	}
	if _, sideExists := readFileIfExists(origPath); sideExists {
		fmt.Fprintf(out, "  a pre-existing hook is chained (runs first); `ao verify init --remove` restores it byte-identically\n")
	}
	fmt.Fprintln(out, "  pushes to main/master now require a CONFIRMED verdict per commit — no verdict = not done")

	// Surface the effective verify policy the subsequent `ao verify` runs will
	// use, read via the age-rk3r.5 per-repo config (env > .aoverify.yaml > default).
	cfg := verifycfg.LoadDir(repo)
	if cfg.FileFound {
		fmt.Fprintf(out, "  verify policy: %s (autobind=%v, strict=%v)\n", cfg.ConfigPath, cfg.Autobind, cfg.Strict)
	} else {
		fmt.Fprintf(out, "  verify policy: defaults (autobind=%v, strict=%v) — add a .aoverify.yaml to pin per-repo\n", cfg.Autobind, cfg.Strict)
	}
	return nil
}

// removeRatchet uninstalls our hook, restoring any chained original
// byte-identically. It never touches a foreign (non-ours) hook.
func removeRatchet(out io.Writer, hookPath, origPath string) error {
	existing, existed := readFileIfExists(hookPath)
	if !existed {
		fmt.Fprintln(out, "no pre-push hook present — nothing to remove")
		return nil
	}
	if !isAgentopsHook(existing) {
		fmt.Fprintf(out, "the pre-push hook at %s is not managed by AgentOps — leaving it untouched\n", hookPath)
		return nil
	}
	if _, sideExists := readFileIfExists(origPath); sideExists {
		// Byte-identical restore: rename preserves the original bytes + mode.
		if err := os.Rename(origPath, hookPath); err != nil {
			return fmt.Errorf("restore chained original hook: %w", err)
		}
		fmt.Fprintf(out, "removed the ratchet and restored the pre-existing pre-push hook from %s\n", origHookName)
		return nil
	}
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("remove pre-push hook: %w", err)
	}
	fmt.Fprintf(out, "removed the AgentOps pre-push verdict ratchet (%s)\n", hookPath)
	return nil
}

// SCOPE (parked adversarial boundary, do NOT "fix" without a scope change): this
// ratchet defends against honest mistakes and misconfiguration, NOT an
// adversarial repository actively subverting its own gate. A repo-relative
// core.hooksPath (which resolveHooksDir honors, so an install can land in a
// repo-controlled location), a repo that rewrites its own installed hook, or an
// operator who runs `git push --no-verify` can bypass it by design — that is the
// parked adversarial-multi-tenant threat (PRODUCT.md), out of scope for the
// portable ratchet. The repo-internal-BINARY invariant below is still enforced
// (a swapped baked ao is an honest-mistake footgun, not the multi-tenant threat).
//
// loadHookTemplate reads the embedded hook template, CRLF-normalizes it, and
// substitutes the placeholder with the absolute path of the running ao binary so
// the installed hook always invokes THIS binary — the ONLY ao the runtime hook
// will trust (baked-only; no PATH fallback).
//
// INSTALL-SIDE HALF of the "no repo-internal-absolute binary is ever trusted"
// invariant (the runtime git twin is trustedGit): the runtime hook trusts the
// baked path ABSOLUTELY, so the baked path must never be repo-internal — a
// repo-local ao ($REPO/bin/ao, cli/bin/ao) could later be swapped for a fake
// that fakes the version + exits 0 for `verify pre-push` to authorize an
// unverified push. So before baking, the resolved ao path must be (a) absolute
// AND (b) outside repo's tree; otherwise REFUSE the install (never silently fall
// back), so the operator reinstalls from a trusted ao outside the repo.
func loadHookTemplate(repo string) (string, error) {
	raw, err := fs.ReadFile(embedded.PawlFS, embeddedHookTemplatePath)
	if err != nil {
		return "", fmt.Errorf("read embedded hook template %s: %w", embeddedHookTemplatePath, err)
	}
	self := resolveSelfBinaryPath()
	if !filepath.IsAbs(self) {
		return "", fmt.Errorf("ao verify init: cannot determine an absolute path for the running ao (%q) — "+
			"install from an installed ao on PATH so the hook can bake a trusted absolute path", self)
	}
	if pathInsideRepo(self, repo) {
		return "", fmt.Errorf("ao verify init: refusing to bake a repo-internal ao path (%s) into the hook — "+
			"install from an ao outside the repo (e.g. an installed ao on PATH); a repo-local binary could later be "+
			"swapped to bypass the gate", self)
	}
	body := string(normalizeShellScript(raw))
	return strings.ReplaceAll(body, aoBinPlaceholder, self), nil
}

// verifyInitSelfBinary is the seam for resolving the running ao binary path
// (production: os.Executable), so a test can point the install-time baked-ao
// validation at a repo-internal path without moving the test binary.
var verifyInitSelfBinary = os.Executable

// resolveSelfBinaryPath returns the absolute, symlink-resolved path of the
// running ao binary, or "" when it cannot be resolved (loadHookTemplate then
// refuses the install rather than baking an untrusted/relative path).
func resolveSelfBinaryPath() string {
	self, err := verifyInitSelfBinary()
	if err != nil || self == "" {
		return ""
	}
	if resolved, rerr := filepath.EvalSymlinks(self); rerr == nil && resolved != "" {
		return resolved
	}
	return self
}

// isAgentopsHook reports whether content is a hook this command manages.
func isAgentopsHook(content []byte) bool {
	return strings.Contains(string(content), hookMarker)
}

// readFileIfExists returns a file's contents and whether it exists (a read error
// other than not-exist is reported as "exists" with nil content so callers
// fail-closed rather than silently overwrite).
func readFileIfExists(path string) ([]byte, bool) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a resolved hook path under the repo's git dir.
	if err == nil {
		return data, true
	}
	if os.IsNotExist(err) {
		return nil, false
	}
	return nil, true
}
