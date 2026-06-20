#!/usr/bin/env bash
# Intentionally NOT `set -e`: a nudge must never break a caller. Every path
# validates its inputs and exits 0.
set -uo pipefail

# check-release-due.sh — a 'release due' NUDGE, not a gate and not an
# auto-release mechanism (age-push-equals-ci-0ua.5 / IMP-4). It converts
# "N commits / M days on main since the last vX.Y.Z tag" into a one-line
# notification so a release that is overdue becomes visible. Distribution stays
# pull-based by design; this only surfaces the signal, never acts on it.
#
# Usage:
#   ./scripts/check-release-due.sh [ref]      # ref defaults to HEAD
#
# Thresholds (override via env; non-numeric values fall back to the defaults):
#   RELEASE_DUE_COMMITS  commits since last tag that flag "due" (default 50)
#   RELEASE_DUE_DAYS     days since last tag that flag "due"    (default 14)
#   RELEASE_DUE_NOW_EPOCH  override "now" (unix seconds) — for deterministic tests
#
# Always exits 0.

is_uint() { [[ "${1:-}" =~ ^[0-9]+$ ]]; }

# Normalize an env/computed value to a base-10 non-negative integer, falling back
# to a default. Forcing base 10 (10#) makes leading-zero values like "08"/"09"
# safe in the (( )) comparisons below (bash would otherwise read them as invalid
# octal). The <=15-digit bound rejects absurd values that would overflow bash's
# signed 64-bit arithmetic and wrap negative — a commit count, day count, or unix
# epoch never approaches 15 digits, so this never clips a real value.
norm_uint() {
  local v="${1:-}" def="${2:-0}"
  if is_uint "$v" && (( ${#v} <= 15 )); then printf '%d' "$((10#$v))"; else printf '%d' "$def"; fi
}

COMMITS_THRESHOLD="$(norm_uint "${RELEASE_DUE_COMMITS:-}" 50)"
DAYS_THRESHOLD="$(norm_uint "${RELEASE_DUE_DAYS:-}" 14)"
REF="${1:-HEAD}"

run_git_clean() { env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR git "$@"; }

if ! run_git_clean rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "release-due: not a git repository — skipping."
  exit 0
fi

last_tag="$(run_git_clean tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname 2>/dev/null | head -1 || true)"

if [[ -z "$last_tag" ]]; then
  echo "release-due: no vX.Y.Z release tag found — first release pending."
  exit 0
fi

commits="$(norm_uint "$(run_git_clean rev-list "${last_tag}..${REF}" --count 2>/dev/null || true)" 0)"
tag_epoch="$(norm_uint "$(run_git_clean log -1 --format=%ct "$last_tag" 2>/dev/null || true)" 0)"

now_epoch="${RELEASE_DUE_NOW_EPOCH:-}"
is_uint "$now_epoch" || now_epoch="$(date +%s 2>/dev/null || echo 0)"
now_epoch="$(norm_uint "$now_epoch" 0)"

if (( tag_epoch > 0 && now_epoch >= tag_epoch )); then
  days=$(( (now_epoch - tag_epoch) / 86400 ))
else
  days=0
fi

reasons=()
(( commits >= COMMITS_THRESHOLD )) && reasons+=("${commits} commits ≥ ${COMMITS_THRESHOLD}")
(( days >= DAYS_THRESHOLD )) && reasons+=("${days}d ≥ ${DAYS_THRESHOLD}d")

if (( ${#reasons[@]} > 0 )); then
  reason_str=""; printf -v reason_str '%s, ' "${reasons[@]}"; reason_str="${reason_str%, }"
  printf 'release-due: YES — %s commits / %sd since %s (%s). Consider cutting a release: scripts/ci-local-release.sh, then tag.\n' \
    "$commits" "$days" "$last_tag" "$reason_str"
else
  printf 'release-due: no — %s commits / %sd since %s (thresholds: %s commits / %sd).\n' \
    "$commits" "$days" "$last_tag" "$COMMITS_THRESHOLD" "$DAYS_THRESHOLD"
fi
exit 0
