#!/usr/bin/env bash
# Mechanical pre-cycle halt gate for the autonomous /evolve loop.
#
# Externalizes the marker + regression checks that previously lived as inline
# prose in skills/evolve/SKILL.md Step 1 / Step 1.5. Prose the agent might skip
# becomes a script the agent MUST run: the skill calls this before every cycle
# and exits the cycle when it returns non-zero. Adapted from the mt-olympus
# unbounded-evolve substrate (scripts/evolve/halt-check.sh) to agentops marker
# semantics (soc-5qit: non-sticky DORMANT, TTL on KILL/STOP) plus the
# goal-regression gate agentops lacked.
#
# Usage:
#   scripts/evolve/halt-check.sh          # human-readable (1 line on stdout)
#   scripts/evolve/halt-check.sh --json   # {"halt":bool,"halt_reason":str|null}
#
# Env:
#   EVOLVE_DIR             evolve state dir (default <repo>/.agents/evolve)
#   EVOLVE_NEXTWORK        harvested next-work ledger (default <repo>/.agents/rpi/next-work.jsonl)
#   EVOLVE_KILL_TTL_DAYS   KILL/STOP staleness window in days (default 7)
#   EVOLVE_HALT_TIMEOUT    per-probe timeout seconds (default 30)
#
# Halt priority (highest first):
#   1. ~/.config/evolve/KILL (fresh)        -> kill           (operator global)
#   2. .agents/evolve/STOP   (fresh)        -> user_halt      (operator repo)
#   3. .agents/evolve/DORMANT + no ready    -> dormant        (auto-clears if ready>0)
#   4. cycle-history goals_passing dropped  -> goal_regression (last < prior productive)
#   5. cycle-history latest result FAIL     -> prior_cycle_fail (restorative signal)
#
# Non-sticky HANDOFF is always cleared (context-handoff, not a halt).
# Stale KILL/STOP (older than TTL) are surfaced loudly and NOT honored.
#
# Exit: 0 = continue (halt_reason=""), 1 = halt (halt_reason set).

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVOLVE_DIR="${EVOLVE_DIR:-$ROOT/.agents/evolve}"
NEXTWORK="${EVOLVE_NEXTWORK:-$ROOT/.agents/rpi/next-work.jsonl}"
KILL_TTL_DAYS="${EVOLVE_KILL_TTL_DAYS:-7}"
PROBE_TIMEOUT="${EVOLVE_HALT_TIMEOUT:-30}"

json_mode=0
[[ "${1:-}" == "--json" ]] && json_mode=1

emit_result() {
  local halt="$1" reason="$2"
  if [[ "$json_mode" == "1" ]]; then
    if [[ -z "$reason" ]]; then
      printf '{"halt":%s,"halt_reason":null}\n' "$halt"
    else
      printf '{"halt":%s,"halt_reason":"%s"}\n' "$halt" "$reason"
    fi
  elif [[ "$halt" == "true" ]]; then
    echo "HALT: $reason"
  else
    echo "OK: continue"
  fi
}

# Returns 0 if the marker exists AND is fresh (within TTL); 1 otherwise.
# A stale marker is surfaced on stderr and treated as not-blocking.
marker_is_fresh() {
  local path="$1" ttl_days="$2"
  [[ -f "$path" ]] || return 1
  local mtime now age
  mtime="$(stat -c %Y "$path" 2>/dev/null || stat -f %m "$path" 2>/dev/null || echo 0)"
  now="$(date +%s)"
  age=$(( (now - mtime) / 86400 ))
  if (( age > ttl_days )); then
    echo "WARN: ${path} is ${age}d old (> ${ttl_days}d); STALE, proceeding. Re-touch it or raise EVOLVE_KILL_TTL_DAYS to keep blocking." >&2
    return 1
  fi
  return 0
}

# 1. Operator global KILL (fresh)
if marker_is_fresh "$HOME/.config/evolve/KILL" "$KILL_TTL_DAYS"; then
  emit_result true "kill"
  exit 1
fi

# 2. Operator repo STOP (fresh)
if marker_is_fresh "$EVOLVE_DIR/STOP" "$KILL_TTL_DAYS"; then
  emit_result true "user_halt"
  exit 1
fi

# 3. DORMANT — non-sticky (soc-5qit). Auto-clear if real work exists; else halt.
if [[ -f "$EVOLVE_DIR/DORMANT" ]]; then
  ready="$(timeout "$PROBE_TIMEOUT" bd ready --json 2>/dev/null | jq -r 'length // 0' 2>/dev/null || echo 0)"
  harvested="$(jq -r 'select(.consumed==false) | .severity' "$NEXTWORK" 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "${ready:-0}" -gt 0 || "${harvested:-0}" -gt 0 ]]; then
    rm -f "$EVOLVE_DIR/DORMANT"  # stale: ready beads exist, loop resumes
  else
    emit_result true "dormant"
    exit 1
  fi
fi

# HANDOFF is non-sticky context-handoff, never a halt — always clear it.
[[ -f "$EVOLVE_DIR/HANDOFF" ]] && rm -f "$EVOLVE_DIR/HANDOFF"

# The canonical cycle ledger written by scripts/evolve-log-cycle.sh / `ao loop
# append`. Each line carries `result` and, on productive cycles, `goals_passing`
# + `goals_total`. We read the last two productive entries to detect regression
# and the last entry's `result` for the restorative signal — no parallel report
# writer needed.
history="$EVOLVE_DIR/cycle-history.jsonl"
if [[ ! -r "$history" ]]; then
  emit_result false ""   # bootstrap: no ledger yet → nothing to regress against
  exit 0
fi

# 4. Goal regression: latest productive goals_passing < the prior productive one.
mapfile -t gp < <(
  timeout "$PROBE_TIMEOUT" jq -r 'select(.goals_passing != null) | .goals_passing' "$history" 2>/dev/null \
    | tail -2
)
if [[ ${#gp[@]} -eq 2 && "${gp[1]}" =~ ^[0-9]+$ && "${gp[0]}" =~ ^[0-9]+$ ]] && (( gp[1] < gp[0] )); then
  emit_result true "goal_regression"
  exit 1
fi

# 5. Prior-cycle FAIL → restorative signal (next cycle must reduce red, not add
#    features). The skill reads this and sets restorative mode; not terminal.
last_result="$(timeout "$PROBE_TIMEOUT" jq -r 'select(.result != null) | .result' "$history" 2>/dev/null | tail -1 || echo "")"
case "$last_result" in
  *FAIL*|*BLOCKED*) emit_result true "prior_cycle_fail"; exit 1 ;;  # *FAIL* also matches FAILED
esac

emit_result false ""
exit 0
