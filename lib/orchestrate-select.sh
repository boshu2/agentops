#!/usr/bin/env bash
# orchestrate-select.sh — dual-runtime SELECTOR SEAM (spike keeper)
#
# Resolves which orchestration backend to use for a task and degrades safely:
#     NTM swarm  ->  Claude-native  ->  beads (floor)
#
# This is the prototype of the SDK's OrchestrationPort.Select(). It only DECIDES
# + emits a trace; actual dispatch is the adapter's job. The whole point of the
# spike is to prove the *degradation* is clean and explicit.
#
# Resolution order (first match wins):
#   1. AGENTOPS_ORCHESTRATION pin:  ntm | claude | beads | off   (off => beads)
#      (AGENTOPS_ORCH accepted as a back-compat alias)
#   2. NTM available?      -> ntm     (preferred: persistent, attachable, multi-vendor)
#   3. Claude-native?      -> claude  (in-session/headless; the "worse NTM")
#   4. always              -> beads   (sequential floor; always works)
set -euo pipefail

NTM_HOST="${AGENTOPS_NTM_HOST:-bushido}"

log() { printf '  [select] %s\n' "$*" >&2; }

ntm_available() {
  # Honors a test override so we can prove degradation without killing bushido.
  [[ "${AGENTOPS_NTM_FORCE_DOWN:-0}" == "1" ]] && { log "NTM forced down (test)"; return 1; }
  timeout 10 ssh -o ConnectTimeout=6 -o BatchMode=yes "$NTM_HOST" \
    'command -v ntm >/dev/null && command -v tmux >/dev/null' >/dev/null 2>&1
}

claude_available() {
  # In-session Claude / CI agent context. Disable-able for the floor test.
  [[ "${AGENTOPS_NO_CLAUDE:-0}" == "1" ]] && return 1
  return 0
}

select_backend() {
  local pin="${AGENTOPS_ORCHESTRATION:-${AGENTOPS_ORCH:-auto}}"
  case "$pin" in
    off|beads) echo "beads"; log "reason: explicit opt-out / pin=$pin"; return ;;
    ntm)       echo "ntm";    log "reason: explicit pin=ntm"; return ;;
    claude)    echo "claude"; log "reason: explicit pin=claude"; return ;;
    auto)      : ;;
    *)         log "unknown AGENTOPS_ORCH='$pin' -> auto"; ;;
  esac
  if ntm_available;    then echo "ntm";    log "reason: NTM reachable on $NTM_HOST"; return; fi
  if claude_available; then echo "claude"; log "reason: NTM absent -> Claude-native"; return; fi
  echo "beads"; log "reason: no orchestration runtime -> beads floor"
}

self_test() {
  local fail=0
  check() { # desc expected; runs select_backend with current env
    local desc="$1" expected="$2" got
    got="$(select_backend 2>/dev/null)"
    if [[ "$got" == "$expected" ]]; then printf 'PASS  %-46s -> %s\n' "$desc" "$got"
    else printf 'FAIL  %-46s -> got %s, want %s\n' "$desc" "$got" "$expected"; fail=1; fi
  }
  echo "== degradation-ladder self-test =="
  ( unset AGENTOPS_ORCH AGENTOPS_NTM_FORCE_DOWN AGENTOPS_NO_CLAUDE
    check "default (NTM up)" "ntm" )
  ( export AGENTOPS_NTM_FORCE_DOWN=1; unset AGENTOPS_ORCH AGENTOPS_NO_CLAUDE
    check "NTM down -> degrade to Claude" "claude" )
  ( export AGENTOPS_NTM_FORCE_DOWN=1 AGENTOPS_NO_CLAUDE=1; unset AGENTOPS_ORCH
    check "NTM down + Claude off -> beads floor" "beads" )
  ( export AGENTOPS_ORCH=off
    check "explicit opt-out (AGENTOPS_ORCH=off)" "beads" )
  ( export AGENTOPS_ORCH=claude
    check "explicit pin=claude" "claude" )
  ( export AGENTOPS_ORCH=beads
    check "explicit pin=beads" "beads" )
  echo
  [[ $fail -eq 0 ]] && echo "ALL PASS — ladder degrades cleanly" || echo "FAILURES present"
  return $fail
}

case "${1:-select}" in
  --self-test) self_test ;;
  select|"")   b="$(select_backend)"; echo "selected backend: $b" ;;
  *) echo "usage: $0 [select|--self-test]" >&2; exit 2 ;;
esac
