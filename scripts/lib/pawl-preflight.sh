#!/usr/bin/env bash
# shellcheck shell=bash
# pawl-preflight.sh — sourceable: the DETERMINISTIC pre-reviewer gate
# (age-verification-economics-ebec.9).
#
# THE bottleneck fix (2026-07-07): closing a change took 4-7 model-review rounds
# because ~90% of the defects were DETERMINISTIC (format/link/twin/gate-hint)
# that a local gate run catches for free — but the operator requested the
# expensive cross-family verdict BEFORE the deterministic checks were green, so
# the membrane re-discovered them SERIALLY, each round a full re-review + re-land.
# This runs the reviewed repo's deterministic battery FIRST and fails fast on red,
# WITHOUT spending a reviewer round — mirroring the --smoke fail-first seam
# (age-rk3r.7). It converts the inert prose lesson "run the battery first" into
# MECHANISM (the skills audit proved prose lessons are inert).
#
# SAFETY STANCE — pure ACCELERATOR, never a false blocker: a nonzero that cannot
# be CONFIRMED as a real gate failure (the RCE trust-guard, a missing/oustale ao,
# a build error) is treated as SKIP-and-proceed, not RED. Worst case the preflight
# no-ops and behavior is byte-identical to today (the pre-push gate is still the
# backstop). It can only ever SAVE a wasted review, never wrongly block a good one.

# pawl_preflight <scope> <repo_root>
#   returns 0  -> proceed to the reviewer (passed / skipped / no battery / can't-run)
#   returns 3  -> the deterministic battery is CONFIRMED RED; caller MUST NOT dispatch
#                 the reviewer (0 reviewer tokens spent) — fix and re-run.
#
# Skips (returns 0) when: PAWL_NO_PREFLIGHT=1; PAWL_UNTRUSTED_REPO=1 (never run a
#   repo's own battery over an untrusted checkout — mirrors the smoke stance);
#   scope != head (staged has no committed HEAD to gate); no battery resolves.
#
# Command precedence:
#   1. PAWL_PREFLIGHT_CMD (explicit; the operator owns its exit semantics: nonzero = RED).
#   2. else the default `<ao> gate check --fast --scope head` when an ao binary
#      resolves (AO_BIN -> repo cli/bin/ao -> PATH ao, mirroring check-pawl-pre-push.sh).
#      Here a nonzero is RED only when the gate DEMONSTRABLY ran (its summary marker
#      "checks —" is present); otherwise SKIP.
pawl_preflight() {
  local scope="${1:-head}" repo="${2:-$PWD}"
  [[ "${PAWL_NO_PREFLIGHT:-0}" == "1" ]] && return 0
  [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]] && return 0
  [[ "$scope" == "head" ]] || return 0

  local out rc=0
  if [[ -n "${PAWL_PREFLIGHT_CMD:-}" ]]; then
    # Explicit operator battery — trust its exit code directly.
    out="$(cd "$repo" && eval "$PAWL_PREFLIGHT_CMD" 2>&1)" || rc=$?
    if [[ "$rc" -ne 0 ]]; then
      _pawl_preflight_report "$PAWL_PREFLIGHT_CMD" "$rc" "$out"
      return 3
    fi
    echo "pawl-review: PREFLIGHT passed (PAWL_PREFLIGHT_CMD green) — dispatching the reviewer." >&2
    return 0
  fi

  # Default battery: resolve ao like the pre-push gate does.
  local ao
  ao="$(_pawl_preflight_resolve_ao "$repo")" || return 0   # no ao -> skip, proceed
  local cmd="$ao gate check --fast --scope head"
  out="$(cd "$repo" && $ao gate check --fast --scope head 2>&1)" || rc=$?
  if ! grep -q 'checks —' <<<"$out"; then
    # The gate did NOT demonstrably run (trust-guard, unknown subcommand, crash) —
    # SKIP-and-proceed. The pre-push gate remains the backstop; never false-block.
    echo "pawl-review: PREFLIGHT skipped — the deterministic gate could not run (env/trust/build); proceeding to the reviewer (pre-push gate still backstops). ${cmd} exit ${rc}." >&2
    return 0
  fi
  if [[ "$rc" -ne 0 ]]; then
    _pawl_preflight_report "$cmd" "$rc" "$out"
    return 3
  fi
  echo "pawl-review: PREFLIGHT passed (deterministic battery green) — dispatching the reviewer." >&2
  return 0
}

# _pawl_preflight_resolve_ao <repo> — echo a runnable ao path or return nonzero.
# Order mirrors check-pawl-pre-push.sh: AO_BIN -> repo cli/bin/ao -> PATH ao.
# Never PATH-resolves under PAWL_UNTRUSTED_REPO (the caller already returned 0 there;
# this is defense-in-depth).
_pawl_preflight_resolve_ao() {
  local repo="${1:-$PWD}"
  if [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
  [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]] && return 1
  if [[ -x "$repo/cli/bin/ao" ]]; then printf '%s\n' "$repo/cli/bin/ao"; return 0; fi
  command -v ao 2>/dev/null
}

# _pawl_preflight_report <cmd> <rc> <output> — the RED banner to stderr.
_pawl_preflight_report() {
  local cmd="$1" rc="$2" out="$3"
  {
    echo "=== PAWL-PREFLIGHT: the deterministic battery is RED — the model reviewer was NOT dispatched (0 reviewer tokens). ==="
    echo "command: ${cmd}  (exit ${rc})"
    echo "Fix the failing deterministic check(s) below, then re-run pawl-review. Opt out with PAWL_NO_PREFLIGHT=1."
    printf '%s\n' "$out" | tail -40
    echo "=== end preflight (fix these locally — a model review would only re-discover them at 100x the cost) ==="
  } >&2
}
