#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/verify-config.sh — the SHELL half of the per-repo verify config layer
# (bead age-rk3r.17; the Go half is age-rk3r.5, cli/internal/verifycfg + `ao verify
# --export-env`). Sourced ONCE at pawl-review.sh's entry, it resolves the checked-in
# .aoverify.yaml at the REVIEWED repo's root into exported PAWL_* env vars so the
# review engine below honors a committed, reviewable policy without the operator
# exporting ~30 PAWL_* vars by hand.
#
# Unlike lib/codex-exec.sh (a pure-function library — "source it, do NOT execute it"),
# this file is a HOOK: sourcing it RUNS the resolution as a side effect, exactly once.
#
# ---------------------------------------------------------------------------
# THE .5 BRIDGE — why the shell does NOT parse YAML
# ---------------------------------------------------------------------------
# `ao verify --export-env` resolves precedence (env > file > default) in ONE place (Go)
# and emits `export PAWL_*=...` lines for NON-DEFAULT keys ONLY. The shell just evals
# them. That "non-default keys only" contract is LOAD-BEARING:
#   - Under ZERO config the bridge emits NOTHING, so this hook is a no-op and
#     pawl-review.sh stays BYTE-IDENTICAL to its pre-hook behavior.
#   - Emitting a default (e.g. PAWL_REVIEW_TIMEOUT=300) would make pawl-review believe
#     the operator PINNED the timeout and disable its own diff-size auto-scaling — the
#     exact regression this contract prevents. See verifycfg.Resolved.ExportEnv.
#
# ---------------------------------------------------------------------------
# TARGET-REPO RESOLUTION — the config is read from the repo UNDER REVIEW
# ---------------------------------------------------------------------------
# The .5 parser (verifycfg.LoadDir) walks up from ao's CWD to the nearest ancestor with
# a .git entry (the `git rev-parse --show-toplevel` equivalent, pure-Go, worktree-safe)
# and reads .aoverify.yaml there. pawl-review.sh always runs — and this hook therefore
# always invokes ao — with CWD = the repo being reviewed:
#   - in-checkout dogfood: ao (from cli.go's runForwardedPawlScript) runs with Dir =
#     the AgentOps checkout, so the checkout's own .aoverify.yaml is read;
#   - stranger/embedded path: Dir = the USER's git repo (pawlReviewColdEnv re-roots
#     onto it), so the USER's .aoverify.yaml is read — never the AgentOps checkout the
#     embedded bundle was extracted from.
# So the target repo is resolved FOR FREE by running ao with the right cwd; this hook
# passes NO path/root argument and must not, or it would defeat that guarantee.
#
# ---------------------------------------------------------------------------
# THE ao BINARY — three contexts, one resolution
# ---------------------------------------------------------------------------
# Resolution order (deliberately simpler than pawl-review's resolve_ao — the config
# bridge needs any ao that has `verify --export-env`, not the freshest repo build):
#   1. $AO_BIN if set + executable  — the embedded/stranger path pins the TRUSTED
#      invoking binary here (pawlReviewColdEnv AO_BIN=<os.Executable()>), so a repo
#      under review can never substitute its own ao.
#   2. else `ao` on PATH            — the in-checkout / installed context.
#   3. else NO-OP                   — no ao at all: silent, zero-config byte-identical,
#      never an error and never a warning that changes output.
# In PAWL_UNTRUSTED_REPO=1 mode (the embedded stranger path) ONLY step 1 is honored —
# never a PATH-resolved ao — mirroring resolve_ao's stance (defense-in-depth atop the
# cold path's already-sanitized PATH). AO_BIN is always pinned in that mode, so this is
# a belt, not a gap.
#
# FAIL-SAFE: config is best-effort. The eval of the bridge output is `2>/dev/null ||
# true`-guarded and a non-zero ao exit discards its stdout, so a broken/stale ao can
# NEVER break (or corrupt the environment of) a review. ao's STDERR is passed through
# unchanged, so the Go parser's config warnings (unknown key, unparseable value) still
# reach the operator.

# _verify_config_resolve_ao — print the ao binary to drive the config bridge, or nothing
# (non-zero) when none is usable. See "THE ao BINARY" above for the order + trust rules.
_verify_config_resolve_ao() {
  if [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]]; then
    printf '%s\n' "$AO_BIN"
    return 0
  fi
  # Untrusted repo: ONLY the explicitly-trusted AO_BIN (handled above). Never resolve
  # `ao` from PATH here — with the repo under review as cwd that is the resolve_ao RCE
  # stance. AO_BIN is always pinned in this mode, so reaching here means "no trusted ao".
  [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]] && return 1
  command -v ao 2>/dev/null
}

# _verify_config_apply — resolve ao and eval its `export PAWL_*=...` bridge ONCE. A no-op
# when no ao is available or the bridge emits nothing (zero config). Always returns 0.
_verify_config_apply() {
  local _ao _exports
  _ao="$(_verify_config_resolve_ao)" || return 0
  [[ -n "$_ao" ]] || return 0
  # Capture ONLY stdout (the export lines). ao's STDERR (config warnings) flows through
  # to the operator during the command substitution — NOT suppressed — so an unknown key
  # still surfaces its one warning line. On a non-zero exit (a stale ao without `verify
  # --export-env`, a broken build) DISCARD stdout so a partial/garbage payload is never
  # eval'd, and no-op.
  _exports="$("$_ao" verify --export-env)" || return 0
  [[ -n "$_exports" ]] || return 0
  # Apply the resolved policy. `2>/dev/null || true` guards the eval itself so a malformed
  # bridge line can never break the review; env > file precedence is already resolved in Go.
  eval "$_exports" 2>/dev/null || true
  return 0
}

# Run the hook now (the ONE side effect of sourcing this file).
_verify_config_apply
