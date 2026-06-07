#!/usr/bin/env bash
# convergence-check.sh — test the 3 OC-016 convergence conditions
#
# Usage: convergence-check.sh <session> <repo_path>
# Exit code: 0 if all three conditions hold for >=2 ticks; 1 otherwise.
#
# Condition 1: git log --since="1 hour ago" = 0 commits
# Condition 2: every pane's recent tail contains convergence language
# Condition 3: br ready = 0 AND (in_progress + claimed) unchanged since last tick
#
# Keeps state in /tmp/vibing-convergence-$SESSION.state (counter + prev inflight).
# Per OC-016 / OBSERVABILITY.md "Deterministic Convergence-Termination".

set -u
SESSION="${1:?usage: convergence-check.sh <session> <repo_path>}"
REPO="${2:?usage: convergence-check.sh <session> <repo_path>}"
STATE_KEY=$(printf '%s' "$SESSION" | tr -c 'A-Za-z0-9_.-' '_')
STATE="/tmp/vibing-convergence-$STATE_KEY.state"

# Canonical convergence phrases (keep in sync with ANTI-PATTERNS.md dictionary).
CONV_REGEX='exemplary|already complete|no fixes needed|ready to ship|no changes required|the implementation is solid|code is clean|nothing to add|looks good to me|\bLGTM\b|tests are passing|all conditions met'

git_commit_count_since() {
  local log_output

  log_output=$(git -C "$REPO" log --since="1 hour ago" --oneline 2>/dev/null) || {
    echo -1
    return
  }

  printf '%s\n' "$log_output" | sed '/^$/d' | wc -l | tr -d ' '
}

COMMITS=$(git_commit_count_since)

fmt_count() {
  local value="${1:--1}"
  if [ "$value" -lt 0 ]; then
    printf 'unknown'
  else
    printf '%s' "$value"
  fi
}

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

READY=$(br_count ready)
IP=$(br_count in_progress)
CLAIMED=$(br_count claimed)
READY=${READY:--1}
IP=${IP:--1}
CLAIMED=${CLAIMED:--1}
if [ "$IP" -ge 0 ] && [ "$CLAIMED" -ge 0 ]; then
  INFLIGHT=$((IP + CLAIMED))
else
  INFLIGHT=-1
fi

# How many panes' last 30 lines contain convergence language?
read -r TOTAL_PANES CONV_PANES < <(
  ntm --robot-tail="$SESSION" --lines=30 2>/dev/null \
    | jq -r --arg re "$CONV_REGEX" '
        [ .panes[]? | ((.lines // []) | join("\n")) ] as $tails
        | [($tails | length), ($tails | map(test($re; "i")) | map(select(.)) | length)]
        | @tsv
      ' 2>/dev/null || printf '0\t0\n'
)
TOTAL_PANES=${TOTAL_PANES:-0}
CONV_PANES=${CONV_PANES:-0}

# Read prior state
PREV_STREAK=0
PREV_INFLIGHT=-1
if [ -r "$STATE" ]; then
  # shellcheck disable=SC1090
  . "$STATE"
fi

# Conditions
C1=0; [ "$COMMITS" -eq 0 ] && C1=1
C2=0; [ "$TOTAL_PANES" -gt 0 ] && [ "$CONV_PANES" -eq "$TOTAL_PANES" ] && C2=1
C3=0
if [ "$READY" -ge 0 ] && [ "$INFLIGHT" -ge 0 ] && [ "$READY" -eq 0 ] && [ "$INFLIGHT" -eq "$PREV_INFLIGHT" ]; then
  C3=1
fi

cat <<EOF
convergence-check for session=$SESSION repo=$REPO
  C1 (commits_1h=0)           = $C1  (actual: $(fmt_count "$COMMITS"))
  C2 (all panes converged)    = $C2  ($CONV_PANES / $TOTAL_PANES panes match)
  C3 (ready=0, inflight flat) = $C3  (ready=$(fmt_count "$READY"), inflight=$(fmt_count "$INFLIGHT"), prev=$(fmt_count "$PREV_INFLIGHT"))
  previous streak             = $PREV_STREAK tick(s)
EOF

if [ "$C1" = 1 ] && [ "$C2" = 1 ] && [ "$C3" = 1 ]; then
  STREAK=$((PREV_STREAK + 1))
else
  STREAK=0
fi

# Persist
cat > "$STATE" <<EOF
PREV_STREAK=$STREAK
PREV_INFLIGHT=$INFLIGHT
EOF

if [ "$STREAK" -ge 2 ]; then
  echo "VERDICT: CONVERGED (streak=$STREAK) — STOP the orchestrator loop; report and exit."
  exit 0
else
  echo "VERDICT: continue (streak=$STREAK, need 2)."
  exit 1
fi
