#!/usr/bin/env bash
# lane-keepalive.sh — re-nudge membrane reviewer lanes idle past budget.
#
# Residual gap 1 (RESIDUAL-GAPS.md): an idle interactive pane in `draining`
# is NOT recoverable by `gc session kill`, `gc session reset`, or raw tmux
# send-keys — `gc session submit` (semantic delivery) is the ONLY verb that
# advances it. This helper is the shipped form of the doc's illustrative
# keepalive: for each membrane lane template, if its session is draining, or
# awake-but-inactive past the budget, re-submit a no-op keepalive line.
#
# CONSERVATIVE by design:
#   - touches ONLY the membrane lane templates (never builders, never the
#     dispatcher — a builder mid-build must not be poked);
#   - the recovery verb is `gc session submit` and NOTHING else;
#   - a busy (recently-active awake) lane is left alone;
#   - all failures are swallowed (|| true) — a keepalive must never take the
#     order lane down with it. Failures still surface via gc events on the
#     order's own exit code if the script itself is broken.
#
# Env (order exec provides GC_CITY_PATH; overridable):
#   MEMBRANE_LANE_TEMPLATES  comma list (default the pack's two lanes)
#   MEMBRANE_IDLE_BUDGET_S   inactivity budget in seconds (default 300)
set -u

CITY="${GC_CITY_PATH:-$PWD}"
GC_BIN="${GC_BIN:-$(command -v gc || echo gc)}"
TEMPLATES="${MEMBRANE_LANE_TEMPLATES:-agentops-membrane.verifier,agentops-membrane.agy-verifier}"
BUDGET="${MEMBRANE_IDLE_BUDGET_S:-300}"
NOW="$(date +%s)"

SESSIONS_JSON="$("$GC_BIN" --city "$CITY" session list --json 2>/dev/null)" || exit 0
[ -n "$SESSIONS_JSON" ] || exit 0

nudged=0
IFS=',' read -ra WANT <<< "$TEMPLATES"
for tpl in "${WANT[@]}"; do
  # newest session per lane template
  row="$(printf '%s' "$SESSIONS_JSON" | jq -c --arg t "$tpl" \
    '[.sessions[]? | select(.template == $t)] | last // empty' 2>/dev/null)"
  [ -n "$row" ] || continue
  name="$(printf '%s' "$row" | jq -r '.name // .id')"
  state="$(printf '%s' "$row" | jq -r '.state // ""')"
  # last activity: tolerate either an epoch or an RFC3339 field name drifting
  last="$(printf '%s' "$row" | jq -r '.last_active_at // .last_active // empty')"
  idle_s="$BUDGET"
  if [ -n "$last" ]; then
    last_epoch="$(date -j -f '%Y-%m-%dT%H:%M:%S' "${last%%.*}" +%s 2>/dev/null \
      || date -d "$last" +%s 2>/dev/null || echo 0)"
    [ "$last_epoch" -gt 0 ] && idle_s=$((NOW - last_epoch))
    [ "$idle_s" -lt 0 ] && idle_s=0   # clock/format skew — treat as just-active
  fi
  case "$state" in
    draining) ;;                                   # always eligible — the known wedge
    active|awake) [ "$idle_s" -ge "$BUDGET" ] || continue ;;  # busy lanes are left alone
    *) continue ;;                                 # asleep/absent: reconciler's job, not ours
  esac
  "$GC_BIN" --city "$CITY" session submit "$name" \
    "membrane keepalive: reply READY if idle (auto re-nudge; state=$state idle=${idle_s}s)" \
    >/dev/null 2>&1 || true
  nudged=$((nudged+1))
done

echo "membrane-lane-keepalive: nudged=$nudged (budget=${BUDGET}s templates=$TEMPLATES)"
exit 0
