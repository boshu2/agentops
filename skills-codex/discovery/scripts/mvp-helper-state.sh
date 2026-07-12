#!/usr/bin/env bash
set -euo pipefail

JQ_BIN="${DISCOVERY_JQ_BIN:-jq}"
MV_BIN="${DISCOVERY_MV_BIN:-mv}"
LOCK_ATTEMPTS="${DISCOVERY_LOCK_ATTEMPTS:-40}"
LOCK_DELAY="${DISCOVERY_LOCK_DELAY:-0.025}"
LOCK_DIR=""
[[ "$LOCK_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || { echo "ESCALATE: invalid lock attempt budget" >&2; exit 4; }

usage() {
  echo "usage: mvp-helper-state.sh claim <phased-state.json> | transition <phased-state.json> <UNSTUCK|ESCALATE>" >&2
  exit 2
}

atomic_filter() {
  local state_path="$1"
  shift
  local tmp="${state_path}.tmp.$$"

  if ! "$JQ_BIN" "$@" "$state_path" >"$tmp"; then
    rm -f -- "$tmp"
    return 1
  fi
  if ! "$MV_BIN" -- "$tmp" "$state_path"; then
    rm -f -- "$tmp"
    return 1
  fi
}

acquire_lock() {
  local state_path="$1" attempt=0 owner=""
  LOCK_DIR="${state_path}.mvp-helper.lock"
  while (( attempt < LOCK_ATTEMPTS )); do
    if mkdir -- "$LOCK_DIR" 2>/dev/null; then
      if ! printf 'pid=%s started=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$LOCK_DIR/owner"; then
        rmdir -- "$LOCK_DIR" 2>/dev/null || true
        LOCK_DIR=""
        echo "ESCALATE: helper lock owner write failed" >&2
        return 1
      fi
      return 0
    fi
    attempt=$((attempt + 1))
    sleep "$LOCK_DELAY"
  done
  if [[ -r "$LOCK_DIR/owner" ]]; then
    owner="$(tr '\n' ' ' <"$LOCK_DIR/owner")"
  fi
  echo "ESCALATE: helper lock contention after $LOCK_ATTEMPTS attempts; lock is never auto-broken (${owner:-owner unknown})" >&2
  return 1
}

release_lock() {
  local failed=0
  [[ -n "$LOCK_DIR" ]] || return 0
  rm -f -- "$LOCK_DIR/owner" || failed=1
  rmdir -- "$LOCK_DIR" || failed=1
  LOCK_DIR=""
  (( failed == 0 ))
}

claim_locked() {
  local state_path="$1" attempted
  if [[ ! -s "$state_path" ]]; then
    echo "ESCALATE: missing phased state: $state_path" >&2
    return 4
  fi
  if ! attempted="$("$JQ_BIN" -er '(.attempts.discovery_mvp_helper // 0) | if type == "number" then floor else error("attempt must be numeric") end' "$state_path")"; then
    echo "ESCALATE: cannot read helper attempt state" >&2
    return 4
  fi
  if (( attempted >= 1 )); then
    if ! atomic_filter "$state_path" '.verdicts = (.verdicts // {}) | .verdicts.discovery_mvp_helper = "ESCALATE"'; then
      echo "ESCALATE: helper already consumed and transition write failed" >&2
      return 4
    fi
    echo "ESCALATE: helper already consumed" >&2
    return 4
  fi
  if ! atomic_filter "$state_path" '.attempts = (.attempts // {}) | .attempts.discovery_mvp_helper = 1'; then
    echo "ESCALATE: helper claim write failed" >&2
    return 4
  fi
}

claim() {
  local state_path="$1" rc
  if ! acquire_lock "$state_path"; then
    return 4
  fi
  if claim_locked "$state_path"; then rc=0; else rc=$?; fi
  if ! release_lock; then
    echo "ESCALATE: helper lock release failed" >&2
    return 4
  fi
  if (( rc == 0 )); then
    echo "CLAIMED"
  fi
  return "$rc"
}

transition_locked() {
  local state_path="$1" value="$2"
  if [[ ! -s "$state_path" ]]; then
    echo "ESCALATE: missing phased state: $state_path" >&2
    return 4
  fi
  # shellcheck disable=SC2016 # $transition is a jq variable, not a shell expansion.
  if ! atomic_filter "$state_path" --arg transition "$value" \
    '.verdicts = (.verdicts // {}) | .verdicts.discovery_mvp_helper = $transition'; then
    echo "ESCALATE: helper transition write failed" >&2
    return 4
  fi
}

transition() {
  local state_path="$1" value="$2" rc
  case "$value" in
    UNSTUCK|ESCALATE) ;;
    *) usage ;;
  esac
  if ! acquire_lock "$state_path"; then
    return 4
  fi
  if transition_locked "$state_path" "$value"; then rc=0; else rc=$?; fi
  if ! release_lock; then
    echo "ESCALATE: helper lock release failed" >&2
    return 4
  fi
  if (( rc == 0 )); then
    echo "$value"
  fi
  return "$rc"
}

case "${1:-}" in
  claim)
    [[ $# -eq 2 ]] || usage
    claim "$2"
    ;;
  transition)
    [[ $# -eq 3 ]] || usage
    transition "$2" "$3"
    ;;
  *) usage ;;
esac
