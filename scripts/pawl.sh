#!/usr/bin/env bash
# pawl.sh — the standing cross-family pawl service (age-standing-pawl-service-ml8).
#
# Encodes the manual opus+codex "duel" dogfooded on 2026-06-18 (landing age-1vy +
# age-uqj) into a deterministic lifecycle so the cross-family pawl is a SERVICE you
# route requests to, not a hand-spun codex-exec per bead. `ao pawl` will wrap this;
# the deterministic core lives here (CLI-for-deterministic, skills-are-instructions).
#
# Subcommands:
#   up                       spawn the standing session + readiness-gate both panes (idempotent)
#   down                     kill the standing session (no orphan panes)
#   health [--json]          per-pane liveness/readiness probe
#   route <bead> <packet> [pr]   route a review packet to BOTH panes, require opus+codex
#                                agreement, capture evidence, record the verdict (pr default 0 = push-to-main)
#
# Hard rules learned in the dogfood (do not regress):
#  - codex /goal caps the objective at ~4000 chars -> ALWAYS send the packet as a FILE
#    reference, never inline (an 8373-char inline paste was rejected).
#  - gate readiness before the first route (boot race): spawn returns before panes boot.
#  - codex 'codex exec --model gpt-5.3-codex' is rejected on a ChatGPT account; the
#    interactive ATM codex pane resolves gpt-5.5 and works.
#  - agreement is fail-closed: BOTH panes must CONFIRM; any REFUTED/timeout -> REFUTED.
set -euo pipefail

SESSION="${PAWL_SESSION:-agentops--pawl-service}"
LABEL="${PAWL_LABEL:-pawl-service}"
PROJECT="${PAWL_PROJECT:-agentops}"
CC_PANE="${PAWL_CC_PANE:-1}"     # claude/opus pane (fresh-context refuter)
COD_PANE="${PAWL_COD_PANE:-2}"   # codex pane (cross-family refuter)
EVID_DIR="${PAWL_EVID_DIR:-/tmp/pawl-evidence}"
STATE_DIR="${PAWL_STATE_DIR:-.agents/pawl}"
ROUTE_TIMEOUT="${PAWL_ROUTE_TIMEOUT:-200}"   # seconds to wait per pane for a VERDICT
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

die() { echo "pawl: $*" >&2; exit 1; }
log() { echo "pawl: $*" >&2; }

session_exists() { tmux has-session -t "$SESSION" 2>/dev/null; }

codex_state() {
  atm codex preflight --session "$SESSION" --pane "$COD_PANE" --json 2>/dev/null \
    | grep '"state"' | head -1 | sed -E 's/.*"state": *"([^"]*)".*/\1/'
}

cc_ready() {
  # claude pane is route-ready when its input box is present (the ❯ prompt line)
  tmux capture-pane -p -t "${SESSION}.${CC_PANE}" 2>/dev/null | grep -qE '❯|Try "'
}

cmd_up() {
  if session_exists; then
    log "session $SESSION already exists — gating readiness (idempotent up)"
  else
    log "spawning standing pawl session $SESSION (opus + codex, no-user)"
    atm spawn "$PROJECT" --label "$LABEL" --no-user --cc=1:opus --cod=1 \
      --no-cass-context --ready-timeout=2m --json >/dev/null 2>&1 \
      || die "atm spawn failed"
  fi
  # Readiness gate (boot race): both panes must reach a route-ready state.
  local cs
  for _ in $(seq 1 30); do
    cs="$(codex_state || true)"
    if cc_ready && { [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; }; then
      mkdir -p "$ROOT/$STATE_DIR"
      printf '{"session":"%s","cc_pane":%s,"cod_pane":%s,"ready":true}\n' \
        "$SESSION" "$CC_PANE" "$COD_PANE" > "$ROOT/$STATE_DIR/session.json"
      log "UP: both panes route-ready (codex=$cs)"
      return 0
    fi
    sleep 4
  done
  die "readiness gate timed out (codex=$(codex_state || echo '?'), cc_ready=$(cc_ready && echo yes || echo no))"
}

cmd_down() {
  if session_exists; then
    atm kill "$SESSION" --json >/dev/null 2>&1 || tmux kill-session -t "$SESSION" 2>/dev/null || true
    rm -f "$ROOT/$STATE_DIR/session.json" 2>/dev/null || true
    log "DOWN: killed $SESSION"
  else
    log "DOWN: no session $SESSION (no-op)"
  fi
}

cmd_health() {
  local json="${1:-}"
  local cs cc
  cs="$(codex_state || echo absent)"
  if cc_ready; then cc="ready"; else cc="not-ready"; fi
  if [ "$json" = "--json" ]; then
    printf '{"session":"%s","exists":%s,"cc_pane":{"pane":%s,"state":"%s"},"cod_pane":{"pane":%s,"state":"%s"}}\n' \
      "$SESSION" "$(session_exists && echo true || echo false)" "$CC_PANE" "$cc" "$COD_PANE" "$cs"
  else
    echo "session=$SESSION exists=$(session_exists && echo yes || echo no) cc[$CC_PANE]=$cc codex[$COD_PANE]=$cs"
  fi
  session_exists && [ "$cc" = "ready" ] && { [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; }
}

# Parse THIS route's verdict from a pane capture. Scoped by a per-route nonce so a
# prior route's verdict still in the scrollback can never be read as this one's
# (the stale-scrollback bug the self-review caught). The reviewer is asked to tag
# its verdict line `PAWL <nonce> <CONFIRMED|REFUTED>`; the nonce makes both the
# staleness AND the echoed-instruction false-positive impossible (the prompt never
# contains the reviewer's actual nonce+verdict pair).
verdict_of() {
  local pane="$1" nonce="$2"
  # Must NEVER fail (returns empty until the verdict appears): a non-zero grep under
  # the caller's `set -euo pipefail` would abort the whole route on the first empty
  # poll. The `|| true` keeps the substitution status 0 while still emitting any match.
  { tmux capture-pane -p -t "${SESSION}.${pane}" -S -120 2>/dev/null \
    | grep -oE "PAWL ${nonce} (CONFIRMED|REFUTED)" || true; } | tail -1 | awk '{print $3}'
}

cmd_route() {
  local bead="${1:?route needs <bead>}" packet="${2:?route needs <packet-file>}" pr="${3:-0}"
  [ -f "$packet" ] || die "packet file not found: $packet"
  session_exists || die "no standing session — run 'pawl up' first"
  mkdir -p "$EVID_DIR"
  local ev_cc="$EVID_DIR/${bead}-opus.txt" ev_cod="$EVID_DIR/${bead}-codex.txt"
  # Per-route nonce scopes verdict parsing to THIS route (kills stale-scrollback +
  # echoed-instruction false positives).
  local nonce; nonce="r$(printf '%x' "$$")$(date +%s | tail -c 6)"
  # Single source of truth for both panes: a per-route packet copy with the nonce-tag
  # appended (so the verdict line carries this route's nonce). The tag deliberately
  # avoids a bare "<nonce> CONFIRMED/REFUTED" pair, so an echo of it can't match the parser.
  local rp="$EVID_DIR/${bead}.packet.md"
  { cat "$packet"; printf '\n\n--- VERDICT FORMAT (required) ---\nEnd your reply with ONE line exactly:\n  PAWL %s <the single word CONFIRMED or REFUTED>\n' "$nonce"; } > "$rp"

  log "route $bead -> both panes (packet=$packet, pr=$pr, nonce=$nonce)"
  # Claude pane: file send. Codex pane: short /goal referencing the file (NEVER inline
  # — the 4000-char /goal limit). Both read the SAME nonce-tagged packet.
  atm send "$SESSION" --pane="$CC_PANE" --file "$rp" \
    --no-cass-check --force-non-interactive --json >/dev/null 2>&1 || die "send to cc pane failed"
  atm send "$SESSION" --pane="$COD_PANE" --codex-goal \
    "Follow the adversarial review instructions in the file $rp and obey its final VERDICT FORMAT line. Read the file now." \
    --no-cass-check --force-non-interactive --json >/dev/null 2>&1 || die "send to codex pane failed"

  # Poll both panes for THIS route's nonce-tagged verdict (bounded).
  local waited=0 vc="" vd=""
  while [ "$waited" -lt "$ROUTE_TIMEOUT" ]; do
    [ -z "$vc" ] && vc="$(verdict_of "$CC_PANE" "$nonce")"
    [ -z "$vd" ] && vd="$(verdict_of "$COD_PANE" "$nonce")"
    [ -n "$vc" ] && [ -n "$vd" ] && break
    sleep 5; waited=$((waited + 5))
  done
  tmux capture-pane -p -t "${SESSION}.${CC_PANE}" -S -60 > "$ev_cc" 2>&1 || true
  tmux capture-pane -p -t "${SESSION}.${COD_PANE}" -S -80 > "$ev_cod" 2>&1 || true

  log "opus=${vc:-<timeout>} codex=${vd:-<timeout>}"
  if [ "$vc" = "CONFIRMED" ] && [ "$vd" = "CONFIRMED" ]; then
    local head; head="$(git rev-parse HEAD)"
    bash "$ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
      --disposition CONFIRMED --head "$head" \
      --author-context "pawl-route-author-${bead}" --mode multi-model \
      --refuter "claude:CONFIRMED:opus-pawl-pane-fresh:${ev_cc}" \
      --refuter "gpt:CONFIRMED:codex-pawl-pane-gpt55:${ev_cod}" >&2
    log "ROUTE $bead: CONFIRMED (opus+codex agree) — verdict recorded for head $head"
    echo "CONFIRMED"
    return 0
  fi
  # Fail-closed: any non-CONFIRMED or timeout.
  log "ROUTE $bead: REFUTED/HOLD — opus=${vc:-timeout} codex=${vd:-timeout} (no agreement; evidence in $EVID_DIR)"
  echo "REFUTED"
  return 1
}

case "${1:-}" in
  up)     shift; cmd_up "$@" ;;
  down)   shift; cmd_down "$@" ;;
  health) shift; cmd_health "$@" ;;
  route)  shift; cmd_route "$@" ;;
  *) cat >&2 <<'H'
Usage: pawl.sh <up|down|health|route>
  up                          spawn + readiness-gate the standing pawl session (idempotent)
  down                        tear down the standing session
  health [--json]             per-pane liveness/readiness
  route <bead> <packet> [pr]  route a review packet to opus+codex, require agreement, record verdict
H
    exit 2 ;;
esac
