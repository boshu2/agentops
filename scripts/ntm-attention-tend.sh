#!/usr/bin/env bash
# ntm-attention-tend.sh — single-shot wake bridge (Direction D, council-ratified).
#
# Closes the live-path propulsion gap: an idle agent pane that nobody re-drives.
# One invocation = ONE reconcile pass over the CURRENT attention state — wake the
# idle agents that have `action_required` attention (e.g. urgent unread mail),
# then EXIT. The external trigger (operator, a completion event, or — once
# ag-egpu lands — a real watch surface) re-invokes it; this script never loops.
#
# DESIGN INVARIANTS (council 2026-06-15; verdict
# .agents/council/2026-06-15-propulsion-one-way-door-verdict.md):
#   1. EVENT-TRIGGERED + BOUNDED, NOT a daemon. No internal loop/ticker/sleep,
#      no backgrounded long-runner. One pass, then exit.
#   2. NO ORPHAN WATCHER (Navi #955). A bounded `ntm --robot-attention` probe can
#      fork a watcher child; if the wrapper kills only the parent, that child is
#      orphaned (survives under launchd). Every ntm probe here runs in its OWN
#      process group, reaped after a synchronous wait, and a trap reaps any
#      lingering child on exit — so no watcher survives this script.
#   3. Any always-on form (cron/launchd/systemd wrapper around this) is BLOCKED
#      until ag-egpu (the workload-object/watch schema) is council-ratified —
#      enforced by the ag-cjruu guardrail.
#
# Usage:
#   ntm-attention-tend.sh <session> [<session>...]   # wake idle action_required panes
#   ntm-attention-tend.sh --dry-run <session>        # print intended sends, send nothing
#
# Env:
#   NTM_BIN      ntm binary (default: ntm) — overridable for tests
#   WAKE_PROMPT  the nudge text delivered to a woken pane
#
# Exit codes:
#   0  ran a bounded pass (0+ wakes; nothing-to-do is success)
#   2  usage error / missing dependency (ntm or jq)
#
# Portability: bash 3.2+ (macOS default) and Linux. No `mapfile`. Each probe is
# launched as a SESSION LEADER (`setsid` on Linux; `python3`/`perl` os.setsid on
# macOS, which has no `setsid`) so child PGID == PID and any watcher the probe
# forks is reaped as a group. NEVER `set -m` job control — it can leave the child
# in our group and SIGTERM the bridge itself.

set -euo pipefail

NTM_BIN="${NTM_BIN:-ntm}"
WAKE_PROMPT="${WAKE_PROMPT:-Hey! Listen! — you have action_required attention. Check your inbox and continue your DAG.}"
DRY_RUN=0

log() { printf '[ntm-attention-tend] %s\n' "$*" >&2; }

# --- no-orphan cleanup (invariant 2) -----------------------------------------
# On ANY exit path, reap any background job THIS shell still owns. The probe path
# already manages its children (waited + group-reaped), so this is
# belt-and-suspenders. Uses `jobs -pr` (ONLY this shell's own running jobs) —
# never a broad name-based process kill, which can match unrelated processes.
# PGID of the probe currently in flight (== its PID; set by ntm_run). Lets an
# INT/TERM mid-probe group-reap the watcher too, not just our own jobs.
CURRENT_PROBE_PGID=""

# shellcheck disable=SC2329  # invoked indirectly via trap
cleanup() {
  trap - EXIT INT TERM                      # clear traps first: no recursion under TERM
  # reap the in-flight probe's whole group on an interrupted exit (guarded)
  [ -n "$CURRENT_PROBE_PGID" ] && kill -TERM -- -"$CURRENT_PROBE_PGID" 2>/dev/null || true
  local p
  for p in $(jobs -pr 2>/dev/null); do kill "$p" 2>/dev/null || true; done
}
trap cleanup EXIT INT TERM

# Run an ntm invocation with a no-orphan guarantee: the probe gets its OWN
# process group; we wait synchronously, then kill the whole group — so a watcher
# child the probe forked cannot be orphaned (Navi #955).
ntm_run() {
  local out_file pid rc self_pgid deadline g
  out_file="$(mktemp)"
  # Launch the probe as a SESSION LEADER so child PGID == PID; then a watcher the
  # probe itself forks shares that PGID and is reaped as a group. setsid uses no
  # racy setpgrp; python3/perl os.setsid cover macOS (no setsid binary).
  if command -v setsid >/dev/null 2>&1; then
    setsid "$NTM_BIN" "$@" >"$out_file" 2>/dev/null &
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c 'import os,sys; os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' "$NTM_BIN" "$@" >"$out_file" 2>/dev/null &
  elif command -v perl >/dev/null 2>&1; then
    perl -MPOSIX=setsid -e 'setsid or die; exec @ARGV' -- "$NTM_BIN" "$@" >"$out_file" 2>/dev/null &
  else
    # Fail CLOSED: without a session-leader tool we cannot guarantee no-orphan.
    log "no session-leader tool (setsid/python3/perl); cannot guarantee no-orphan — refusing"
    rm -f "$out_file"; exit 2
  fi
  pid=$!
  self_pgid="$(ps -o pgid= -p "$$" 2>/dev/null | tr -d ' ')"
  [ "$pid" != "$self_pgid" ] && CURRENT_PROBE_PGID="$pid"   # publish in-flight group for cleanup
  # BOUNDED: poll until the probe exits or NTM_PROBE_TIMEOUT passes — never a bare
  # `wait`, which would hang forever if the probe (or its watcher) never returns.
  deadline=$(( SECONDS + ${NTM_PROBE_TIMEOUT:-15} ))
  while kill -0 "$pid" 2>/dev/null && [ "$SECONDS" -lt "$deadline" ]; do sleep 0.2; done
  # Reap the probe's whole session group (hung probe AND any watcher): TERM, brief
  # grace, then KILL whatever lingers. GUARD: never target our own group, so a
  # broken-isolation path fails the no-orphan test cleanly instead of self-TERM.
  if [ -n "$pid" ] && [ "$pid" != "$self_pgid" ]; then
    kill -TERM -- -"$pid" 2>/dev/null || true
    g=0; while kill -0 "$pid" 2>/dev/null && [ "$g" -lt 10 ]; do sleep 0.1; g=$((g + 1)); done
    kill -KILL -- -"$pid" 2>/dev/null || true
  fi
  set +e; wait "$pid" 2>/dev/null; rc=$?; set -e
  CURRENT_PROBE_PGID=""
  cat "$out_file"; rm -f "$out_file"; return "$rc"
}

# --- args --------------------------------------------------------------------
sessions=()
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    --*) log "unknown flag: $arg"; exit 2 ;;
    *) sessions+=("$arg") ;;
  esac
done
[ "${#sessions[@]}" -eq 0 ] && { log "usage: ntm-attention-tend.sh <session>... [--dry-run]"; exit 2; }
command -v jq >/dev/null 2>&1 || { log "jq is required"; exit 2; }

# --- single-shot reconcile pass ----------------------------------------------
attn="$(ntm_run --robot-attention --robot-format=json || true)"
if [ -z "$attn" ] || ! printf '%s' "$attn" | jq -e . >/dev/null 2>&1; then
  log "no parseable attention state; nothing to do"
  exit 0
fi

actionability="$(printf '%s' "$attn" | jq -r '.trigger_event.actionability // ""')"
if [ "$actionability" != "action_required" ]; then
  log "attention actionability='${actionability:-none}' (not action_required); nothing to do"
  exit 0
fi

# Affected agents (only these get woken, and only if idle). bash-3.2-safe read loop.
affected=()
while IFS= read -r a; do
  [ -n "$a" ] && affected+=("$a")
done < <(printf '%s' "$attn" | jq -r '.trigger_event.details.agents[]? // empty')

summary="$(printf '%s' "$attn" | jq -r '.trigger_event.summary // "action_required"')"
log "action_required: ${summary} (affected: ${affected[*]:-none})"

is_affected() {
  local want="$1" a
  [ "${#affected[@]}" -eq 0 ] && return 1
  for a in "${affected[@]}"; do [ "$a" = "$want" ] && return 0; done
  return 1
}

sent=0
for session in "${sessions[@]}"; do
  act="$(ntm_run --robot-activity="$session" --robot-format=json || true)"
  if ! printf '%s' "$act" | jq -e . >/dev/null 2>&1; then
    log "session '$session': no parseable activity; skipping"
    continue
  fi
  while IFS=$'\t' read -r name pane; do
    [ -n "$name" ] || continue
    is_affected "$name" || continue          # only wake idle agents that are in the action_required set
    if [ "$DRY_RUN" -eq 1 ]; then
      log "DRY-RUN would wake: session=$session pane=$pane agent=$name"
    else
      log "waking: session=$session pane=$pane agent=$name"
      ntm_run --robot-send="$session" --pane="$pane" "$WAKE_PROMPT" >/dev/null 2>&1 || log "send failed: $session pane $pane"
    fi
    sent=$((sent + 1))
  done < <(printf '%s' "$act" | jq -r '.agents[]? | select(.state=="idle") | "\(.name)\t\(.pane)"')
done

log "bounded pass complete; ${sent} wake(s)$([ "$DRY_RUN" -eq 1 ] && echo ' (dry-run)')"
exit 0
