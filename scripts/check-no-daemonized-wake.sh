#!/usr/bin/env bash
# check-no-daemonized-wake.sh — guardrail (ag-cjruu).
#
# The wake bridge (scripts/ntm-attention-tend.sh) is a SINGLE-SHOT, on-demand
# reconcile pass BY DESIGN (Direction D; council verdict
# .agents/council/2026-06-15-propulsion-one-way-door-verdict.md). Wiring it into
# an always-on launcher — cron, launchd, systemd, or a shell loop — turns it into
# the standing daemon the council DEFERRED until ag-egpu (the workload-object /
# watch schema, one-way door #2) is council-ratified. Both judges named this trap:
# D must not sprawl into a sticky pseudo-daemon before ag-egpu lands.
#
# This guard FAILS the build if a daemonized invocation of the bridge appears, so
# the one-way door is never walked through silently. Once ag-egpu ratifies a watch
# surface, the always-on form is allowed and this guard is retired.
#
# Usage: check-no-daemonized-wake.sh [root]    (default: git repo root)
# Exit:  0 = no daemonized wake found (PASS); 1 = a daemonized invocation found.
#
# Portability: bash 3.2+ (no mapfile).

set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
ROOT="${1:-$(resolve_repo_root)}"
BRIDGE="ntm-attention-tend"
note() { printf '[check-no-daemonized-wake] %s\n' "$*" >&2; }

# Files that may legitimately NAME the bridge without daemonizing it: the bridge
# itself, its test, this guard, and this guard's test.
is_self() {
  case "$1" in
    */ntm-attention-tend.sh|*/ntm-attention-tend.bats) return 0 ;;
    */check-no-daemonized-wake.sh|*/check-no-daemonized-wake.bats) return 0 ;;
    *) return 1 ;;
  esac
}

# Collect files that reference the bridge (tracked files when in a repo).
refs=()
if git -C "$ROOT" rev-parse >/dev/null 2>&1; then
  while IFS= read -r f; do [ -n "$f" ] && refs+=("$ROOT/$f"); done \
    < <(git -C "$ROOT" grep -lI "$BRIDGE" -- . 2>/dev/null || true)
else
  while IFS= read -r f; do [ -n "$f" ] && refs+=("$f"); done \
    < <(grep -rlI "$BRIDGE" "$ROOT" 2>/dev/null || true)
fi

violations=0
[ "${#refs[@]}" -eq 0 ] && { note "no references to $BRIDGE — OK"; exit 0; }

for f in "${refs[@]}"; do
  is_self "$f" && continue
  reason=""
  # 1) systemd/launchd units, incl. generated (.in/.tmpl) + drop-in dirs.
  case "$f" in
    *.service|*.service.*|*.timer|*.timer.*|*.plist|*.plist.*) reason="systemd/launchd unit references the bridge" ;;
    */systemd/*) reason="systemd unit directory references the bridge" ;;
  esac
  # 2) cron — numeric schedule OR macro (@reboot/@hourly/...) on a bridge line.
  if [ -z "$reason" ] && grep -Eq '^[[:space:]]*(@(reboot|hourly|daily|weekly|monthly|yearly|annually|midnight)|([0-9*/,-]+[[:space:]]+){4,5}).*'"$BRIDGE" "$f" 2>/dev/null; then
    reason="cron schedule/macro invokes the bridge"
  fi
  # 3) one-line launchers on the SAME line as the bridge (low false-positive).
  if [ -z "$reason" ] && grep -Eq '((watch|nohup)[[:space:]]+.*'"$BRIDGE"'|'"$BRIDGE"'[^&]*&[[:space:]]*$)' "$f" 2>/dev/null; then
    reason="backgrounded/watch launcher invokes the bridge"
  fi
  # 4) INFINITE-loop / process-manager / service-enable wrapping the bridge. Match
  #    only daemon signals (while true, until false, for ((;;)), watch, nohup, pm2,
  #    supervisor, systemctl enable, launchctl load) — NOT a finite `for x in`,
  #    which is a legit on-demand multi-session call. Skip pure docs (*.md).
  if [ -z "$reason" ]; then
    case "$f" in *.md) : ;; *)
      if grep -Eq 'while[[:space:]]+(true|:)|until[[:space:]]+(false|:)|for[[:space:]]*\(\([[:space:]]*;;|watch[[:space:]]|nohup[[:space:]]|disown|pm2|supervisor(d|ctl)?|systemctl[[:space:]]+enable|launchctl[[:space:]]+load|crontab' "$f" 2>/dev/null \
         && grep -q "$BRIDGE" "$f" 2>/dev/null; then
        reason="infinite-loop/process-manager/service-enable wraps the bridge"
      fi ;;
    esac
  fi
  if [ -n "$reason" ]; then
    note "VIOLATION: $f — $reason"
    violations=$((violations + 1))
  fi
done

if [ "$violations" -gt 0 ]; then
  note "FAIL: $violations daemonized wake invocation(s). The wake bridge must stay"
  note "single-shot/on-demand until ag-egpu (one-way door #2) ratifies a watch"
  note "surface. See ag-cjruu / the propulsion council verdict."
  exit 1
fi
note "OK: no daemonized wake invocation found"
exit 0
