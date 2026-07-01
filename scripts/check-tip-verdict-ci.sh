#!/usr/bin/env bash
# check-tip-verdict-ci.sh — CI verdict backstop (age-wedge-all-in-dyr0.9).
#
# The local pre-push cockpit gate (scripts/check-pawl-pre-push.sh) is the
# release AUTHORITY; this is the remote BACKSTOP: given a pushed ref range
# BASE..TIP it answers "does this push carry its proof?" by verifying RECORDS
# only — it never produces verdicts (no reviewer calls, no secrets, no
# subscription auth; posture per docs: no hosted control plane).
#
# Checks:
#   (a) the committed provenance ledger (docs/provenance/ledger.jsonl) is an
#       intact, tamper-evident hash chain (`ao provenance verify`, same surface
#       validate.yml uses via scripts/validate-provenance-ledger.sh).
#       TAMPER TRUMPS REPORT-ONLY: a broken chain exits nonzero in BOTH modes.
#   (b) every commit in BASE..TIP has a bound verdict edge in the ledger
#       (from_type=verdict, to_type=commit, disposition=CONFIRMED — the shape
#       `ao provenance emit-verdict` writes) OR satisfies the #trivial
#       provenance-only waiver. The waiver is the SAME implementation the
#       pre-push gate runs: scripts/lib/trivial-waiver.sh (single source —
#       never re-inline it here; see check-pawl-pre-push.sh).
#
# Modes:
#   default    report-only — always exit 0 on missing verdicts; emit GitHub
#              annotations (::warning:: per unverified sha, ::notice:: per
#              verified/waived one). Chain tamper still exits nonzero.
#   --enforce  exit nonzero when any commit in the range lacks proof.
#
# Usage:
#   scripts/check-tip-verdict-ci.sh [--base <sha>] [--head <ref>] [--repo <dir>] [--enforce]
#     --base   range base (exclusive). Empty, all-zeros (new branch/force
#              push), or unknown-locally → falls back to checking the tip
#              commit only.
#     --head   range tip (default: HEAD).
#     --repo   repository to check (default: git toplevel of cwd).
#   Env: AO_BIN — path to the ao binary (else <repo>/cli/bin/ao, else PATH).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die() { echo "tip-verdict-ci: ERROR: $*" >&2; exit 2; }

# SINGLE-IMPLEMENTATION (age-wedge-all-in-dyr0.9): the #trivial waiver is
# sourced from the SAME file the pre-push gate sources.
# shellcheck source=lib/trivial-waiver.sh
if ! source "$SCRIPT_DIR/lib/trivial-waiver.sh"; then
  die "shared waiver lib missing/unreadable at $SCRIPT_DIR/lib/trivial-waiver.sh"
fi
declare -F pawl_trivial_waiver >/dev/null || die "pawl_trivial_waiver not defined by $SCRIPT_DIR/lib/trivial-waiver.sh"

BASE=""
HEAD_REF="HEAD"
REPO=""
ENFORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base)    BASE="${2:-}"; shift 2 ;;
    --head)    HEAD_REF="${2:-}"; shift 2 ;;
    --repo)    REPO="${2:-}"; shift 2 ;;
    --enforce) ENFORCE=1; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

if [[ -z "$REPO" ]]; then
  REPO="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not inside a git repository and no --repo given"
fi
[[ -d "$REPO/.git" || -f "$REPO/.git" ]] || die "--repo $REPO is not a git repository"

TIP="$(git -C "$REPO" rev-parse --verify "${HEAD_REF}^{commit}" 2>/dev/null)" || die "cannot resolve --head '$HEAD_REF' in $REPO"

LEDGER="$REPO/docs/provenance/ledger.jsonl"

# ── (a) Ledger hash-chain verification — tamper trumps report-only ─────────
resolve_ao() {
  if [[ -n "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
  if [[ -x "$REPO/cli/bin/ao" ]]; then printf '%s\n' "$REPO/cli/bin/ao"; return 0; fi
  if command -v ao >/dev/null 2>&1; then printf 'ao\n'; return 0; fi
  return 1
}

if AO="$(resolve_ao)"; then
  # `ao provenance verify` resolves the ledger by walking up from cwd, so run
  # it FROM the repo under check (mirrors validate-provenance-ledger.sh --gate).
  if ( cd "$REPO" && "$AO" provenance verify ); then
    echo "tip-verdict-ci: provenance ledger hash chain intact" >&2
  else
    echo "::error::provenance ledger hash chain BROKEN/TAMPERED in $LEDGER — failing regardless of mode (tamper trumps report-only)"
    exit 1
  fi
else
  # No binary means the chain CANNOT be verified. Report-only degrades to a
  # loud warning; enforce mode is fail-closed (an unverifiable chain must not
  # pass an enforcing gate).
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::ao binary unavailable — cannot verify the provenance ledger hash chain (fail-closed under --enforce)"
    exit 1
  fi
  echo "::warning::ao binary unavailable — provenance ledger hash-chain verification SKIPPED (report-only)"
fi

command -v jq >/dev/null 2>&1 || {
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::jq unavailable — cannot resolve verdict edges (fail-closed under --enforce)"
    exit 1
  fi
  echo "::warning::jq unavailable — verdict-edge resolution SKIPPED (report-only)"
  exit 0
}

# ── build the commit list ───────────────────────────────────────────────────
ZEROS="0000000000000000000000000000000000000000"
declare -a COMMITS=()
if [[ -z "$BASE" || "$BASE" == "$ZEROS" ]] \
   || ! git -C "$REPO" rev-parse --verify --quiet "${BASE}^{commit}" >/dev/null 2>&1; then
  # New branch, force push, or shallow history without the base: fall back to
  # the tip commit — the same single-commit surface the pre-push gate checks.
  COMMITS=("$TIP")
  echo "tip-verdict-ci: base unavailable — checking tip commit only (${TIP:0:12})" >&2
else
  while IFS= read -r _sha; do
    [[ -n "$_sha" ]] && COMMITS+=("$_sha")
  done < <(git -C "$REPO" rev-list --reverse "${BASE}..${TIP}")
fi

if [[ "${#COMMITS[@]}" -eq 0 ]]; then
  echo "::notice::tip-verdict-ci: no commits in range ${BASE:0:12}..${TIP:0:12} — nothing to verify"
  exit 0
fi

# verdict_event_for <sha> — print "from_id<TAB>evidence_ref" of the first
# CONFIRMED verdict edge bound to the commit, if any. Edge shape is exactly
# what `ao provenance emit-verdict` writes (verdict --wasDerivedFrom--> commit,
# evidence_ref "pawl-verdict <bead> disposition=<D>"). A >=7-char to_id prefix
# of the sha is accepted (verdict files may carry short heads).
verdict_event_for() {
  local sha="$1"
  [[ -f "$LEDGER" ]] || return 1
  local hit
  hit="$(jq -r --arg sha "$sha" '
    select(.from_type == "verdict" and .to_type == "commit")
    | select(.to_id == $sha or ((.to_id | length) >= 7 and ($sha | startswith(.to_id))))
    | select(.evidence_ref // "" | contains("disposition=CONFIRMED"))
    | [.from_id, .evidence_ref] | @tsv' "$LEDGER" 2>/dev/null | head -1)"
  [[ -n "$hit" ]] || return 1
  printf '%s\n' "$hit"
}

# ── (b) per-commit verdict-or-waiver check ──────────────────────────────────
violations=0
for sha in "${COMMITS[@]}"; do
  subject="$(git -C "$REPO" log -1 --format=%s "$sha" 2>/dev/null || true)"
  if event="$(verdict_event_for "$sha")"; then
    from_id="${event%%$'\t'*}"
    echo "::notice::commit ${sha:0:12} verified — bound verdict event ${from_id} (${subject})"
    continue
  fi
  waiver_rc=0
  pawl_trivial_waiver "$REPO" "$sha" "tip-verdict-ci" || waiver_rc=$?
  if [[ "$waiver_rc" -eq 0 ]]; then
    echo "::notice::commit ${sha:0:12} waived — #trivial provenance-ledger-only commit (${subject})"
    continue
  fi
  echo "::warning::commit ${sha:0:12} lacks a bound verdict — no CONFIRMED verdict edge in docs/provenance/ledger.jsonl and no #trivial provenance-only waiver (${subject})"
  violations=$((violations + 1))
done

if [[ "$violations" -gt 0 ]]; then
  if [[ "$ENFORCE" -eq 1 ]]; then
    echo "::error::tip-verdict-ci: $violations commit(s) in ${#COMMITS[@]}-commit range lack proof (no verdict = not done) — failing (--enforce)"
    exit 1
  fi
  echo "::warning::tip-verdict-ci: $violations commit(s) in ${#COMMITS[@]}-commit range lack proof — report-only, not failing (run with --enforce to gate)"
  exit 0
fi

echo "tip-verdict-ci: all ${#COMMITS[@]} commit(s) in range carry proof (verdict edge or #trivial waiver)" >&2
exit 0
