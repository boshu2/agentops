#!/usr/bin/env bash
# orchestrator-tick.sh — terse, one-screen snapshot for deciding a tick's action
#
# Usage: orchestrator-tick.sh <session> [repo_path]
# Example: orchestrator-tick.sh asupersync /data/projects/asupersync
#
# Prints:
#   - sources / degraded_sources (from --robot-snapshot)
#   - per-pane is_working + is_rate_limited + is_context_low
#   - rate-limit / OAuth truth
#   - stuck-pane dry-run
#   - triage quick_ref
#   - commits in last hour (productivity ground truth)
#   - in-flight + claimed bead counts (including the often-missed `claimed`)
#
# Per /vibing-with-ntm Operator Loop + OC-012 (source freshness first) + OC-005 (track claimed).

set -u
SESSION="${1:?usage: orchestrator-tick.sh <session> [repo_path]}"
REPO="${2:-$PWD}"

hr() { printf '── %s ──\n' "$*"; }

br_count() {
  local query="$1"
  local json

  if ! command -v br >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
    echo -1
    return
  fi

  case "$query" in
    ready)
      json=$(cd "$REPO" 2>/dev/null && br ready --json 2>/dev/null) || {
        echo -1
        return
      }
      ;;
    *)
      json=$(cd "$REPO" 2>/dev/null && br list --status="$query" --json 2>/dev/null) || {
        echo -1
        return
      }
      ;;
  esac

  printf '%s\n' "$json" | jq 'if type=="object" then (.issues // []) else . end | length' 2>/dev/null || echo -1
}

fmt_count() {
  local value="${1:--1}"
  if [ "$value" -lt 0 ]; then
    printf 'unknown'
  else
    printf '%s' "$value"
  fi
}

git_commit_count_since() {
  local log_output

  log_output=$(git -C "$REPO" log --since="1 hour ago" --oneline 2>/dev/null) || {
    echo -1
    return
  }

  printf '%s\n' "$log_output" | sed '/^$/d' | wc -l | tr -d ' '
}

hr "SOURCE HEALTH"
ntm --robot-snapshot 2>/dev/null \
  | jq -r '(.sources.sources // {}) | to_entries[] | (.value.fresh // false) as $fresh | (.value.degraded // false) as $degraded | (.value.age_ms // "?") as $age | (.value.degraded_reason // .value.reason_code // "-") as $reason | "  \(.key): fresh=\($fresh) degraded=\($degraded) age_ms=\($age) reason=\($reason)"' \
  || echo "  (snapshot unavailable — cursor may be expired; rerun --robot-snapshot)"

hr "PANES · is_working / is_rate_limited / is_context_low"
ntm --robot-is-working="$SESSION" 2>/dev/null \
  | jq -r '.panes[] | "  p\(.pane): working=\(.is_working // false) rate_limited=\(.is_rate_limited // false) ctx_low=\(.is_context_low // false) conf=\(.confidence // "?")"' \
  || echo "  (--robot-is-working unavailable)"

hr "OAUTH / RATE-LIMIT TRUTH"
ntm --robot-health-oauth="$SESSION" 2>/dev/null \
  | jq -r '.panes[] | "  p\(.pane): provider=\(.provider) rate_limited=\(.rate_limited) resets=\(.resets_at // "-")"' \
  || echo "  (oauth health unavailable)"

hr "STUCK-PANE DRY-RUN (10m threshold)"
ntm --robot-health-restart-stuck="$SESSION" --stuck-threshold=10m --dry-run 2>/dev/null \
  | jq -r '.stuck_panes[]? | "  p\(.pane): stuck=\(.stuck_for)s reason=\(.reason // "-")"' \
  || true

hr "BV QUICK-REF (from --robot-triage)"
if BV_JSON=$(cd "$REPO" 2>/dev/null && bv --robot-triage 2>/dev/null); then
  printf '%s\n' "$BV_JSON" | jq -r '.quick_ref // "  (bv --robot-triage failed)"'
else
  echo "  (bv --robot-triage failed)"
fi

hr "PRODUCTIVITY · git log last 1h"
COMMITS=$(git_commit_count_since)
echo "  commits_1h=$(fmt_count "$COMMITS")"
if [ "$COMMITS" -ge 0 ]; then
  git -C "$REPO" log --since="1 hour ago" --format='  %ar %an %h %s' 2>/dev/null | head -5
else
  echo "  (git log unavailable for repo=$REPO)"
fi

hr "BEAD STATE · open + claimed + in_progress"
OPEN=$(br_count open)
CLAIMED=$(br_count claimed)
IP=$(br_count in_progress)
READY=$(br_count ready)
OPEN=${OPEN:--1}
CLAIMED=${CLAIMED:--1}
IP=${IP:--1}
READY=${READY:--1}
if [ "$OPEN" -ge 0 ] && [ "$CLAIMED" -ge 0 ] && [ "$IP" -ge 0 ]; then
  TOTAL_BACKLOG=$((OPEN + CLAIMED + IP))
else
  TOTAL_BACKLOG=-1
fi
echo "  open=$(fmt_count "$OPEN")  claimed=$(fmt_count "$CLAIMED")  in_progress=$(fmt_count "$IP")  ready=$(fmt_count "$READY")  (total backlog=$(fmt_count "$TOTAL_BACKLOG"))"

hr "SUGGESTED NEXT ACTION"
if [ "$COMMITS" -lt 0 ]; then
  echo "  ⮕ Repo unavailable — fix repo_path before making orchestration decisions"
elif [ "$COMMITS" -eq 0 ] && [ "$READY" -eq 0 ] && [ "$IP" -eq 0 ] && [ "$CLAIMED" -eq 0 ]; then
  echo "  ⮕ CONVERGED candidate — verify with convergence-check.sh then STOP"
elif [ "$TOTAL_BACKLOG" -gt 100 ]; then
  echo "  ⮕ BACKLOG >100 — dispatch close-the-backlog prompt; block new review beads"
elif [ "$COMMITS" -eq 0 ]; then
  echo "  ⮕ No commits in 1h — scan for prose-without-commits (OC-004) or handoff-failure (OC-036)"
else
  echo "  ⮕ Healthy pace — specific-terse nudges to idle panes only (OC-010)"
fi
