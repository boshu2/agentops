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
ROUTE_TIMEOUT="${PAWL_ROUTE_TIMEOUT:-320}"   # seconds to wait per pane for a VERDICT
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

# claude pane is ALIVE (running the TUI) if its chrome is present — at the input box,
# actively working, or showing the permissions/interrupt footer. If none of these, the
# pane likely dropped to a bare shell (dead) and must be respawned. (A claude pane mid-work
# is NOT at the input box, so cc_ready alone is not a liveness check.)
cc_alive() {
  tmux capture-pane -p -t "${SESSION}.${CC_PANE}" 2>/dev/null \
    | grep -qE '❯|esc to interrupt|bypass permissions|⏵|Try "'
}

# codex pane is DEAD iff it has dropped to a bare shell. THIS IS LOAD-BEARING: `atm codex
# preflight` reads the slash-palette from scrollback and reports a STALE "goal-completed" for a
# pane that has actually dropped to a shell (proven live 2026-06-18 — the failure that aborted
# routes all session), so liveness can't come from preflight. Death is detected POSITIVELY (a
# shell prompt as the last non-empty line) rather than by ABSENCE of TUI chrome: a live pane
# that transiently shows no chrome must NOT be misclassified dead and needlessly respawned
# mid-review (the "false-dead recovery failure" the pawl refuter flagged on the absence-based
# check). So: dead == no codex chrome ANYWHERE *and* the tail is a shell prompt.
cod_dead() {
  # DETERMINISTIC liveness via the pane's FOREGROUND PROCESS — not scraped scrollback text.
  # Text-scraping is fundamentally fragile: a dropped pane retains the codex TUI's scrollback
  # (splash, footer, "gpt-5.x" footer) ABOVE the new shell prompt, and a shell's cwd/output can
  # contain any marker — the refuter demonstrated false-alives from both. `pane_current_command`
  # is the real foreground command: the codex binary (codex-*) when the TUI is up, a shell
  # (zsh/bash/…) when it has dropped. Immune to scrollback and path/output contents.
  local cmd
  cmd="$(tmux display-message -p -t "${SESSION}.${COD_PANE}" '#{pane_current_command}' 2>/dev/null)" || return 1
  case "$cmd" in
    zsh|-zsh|bash|-bash|sh|-sh|fish|-fish|tcsh|-tcsh|csh|ksh|dash|login) return 0 ;; # foreground is a shell => DEAD
    *) return 1 ;;  # codex binary (or empty/unknown read) => treat as alive (conservative)
  esac
}

# --- S3 reliability: robust sends + degraded-pane respawn + reroute ---

# Best-effort account rotation on a rate/usage limit. OPT-IN (PAWL_AUTO_ROTATE=1): rotating
# switches the host's active account — a real side effect — so it is never silent. Routes by
# vendor per TOOLS-TRUTH (macOS+Claude -> claude-acct, NEVER caam; codex -> caam).
rotate_account() {
  if [ "${PAWL_AUTO_ROTATE:-0}" != "1" ]; then
    log "rate/usage limit on $1 — set PAWL_AUTO_ROTATE=1 to auto-rotate the account"
    return 1
  fi
  case "$1" in
    cod|codex) command -v caam        >/dev/null 2>&1 && caam rotate        >/dev/null 2>&1 && log "rotated codex account (caam)" || true ;;
    cc|claude) command -v claude-acct >/dev/null 2>&1 && claude-acct rotate >/dev/null 2>&1 && log "rotated claude account (claude-acct)" || true ;;
  esac
}

# Respawn a single degraded pane and re-gate its readiness (never the user pane).
respawn_pane() {
  local pane="$1" kind="$2"
  log "respawning $kind pane $pane (degraded)"
  atm respawn "$SESSION" --panes="$pane" --force >/dev/null 2>&1 || true
  for _ in $(seq 1 20); do
    if [ "$kind" = "cod" ]; then
      case "$(codex_state || true)" in codex-live|goal-completed) return 0 ;; esac
    else
      cc_ready && return 0
    fi
    sleep 4
  done
  return 1
}

# Robust codex goal send: send, verify ENGAGEMENT (atm codex wait-goal-engaged), retry up to
# 3x; on usage-limit rotate, on a dead/unknown/dialog pane respawn. Returns 0 once engaged.
# Replaces the old `... || die "send to codex pane failed"` that aborted a whole route on a
# benign flaky send (the bug S3 fixes).
cod_send() {
  local rp="$1" try st
  for try in 1 2 3; do
    atm send "$SESSION" --pane="$COD_PANE" --codex-goal \
      "Follow the adversarial review instructions in the file $rp and obey its final VERDICT FORMAT line. Read the file now." \
      --no-cass-check --force-non-interactive --json >/dev/null 2>&1 || true
    sleep 3
    if atm codex wait-goal-engaged --session "$SESSION" --pane "$COD_PANE" --json 2>/dev/null \
         | grep -q '"outcome": *"engaged"'; then
      return 0
    fi
    st="$(codex_state || echo unknown)"
    log "codex send try $try did not engage (state=$st, dead=$(cod_dead && echo yes || echo no))"
    if cod_dead; then
      # Positively dropped to a shell — preflight may still say goal-completed; trust the shell check.
      respawn_pane "$COD_PANE" cod || true
    else
      case "$st" in
        # `|| true` is LOAD-BEARING: rotate_account intentionally returns 1 when PAWL_AUTO_ROTATE
        # is off (the default), and under set -e an unprotected non-zero would abort the whole send.
        usage-limit) rotate_account cod || true; respawn_pane "$COD_PANE" cod || true ;;
        unknown|absent|stale-scrollback|replace-goal-dialog) respawn_pane "$COD_PANE" cod || true ;;
      esac
    fi
  done
  return 1
}

# Robust claude send: deliver the file; if not delivered or the pane is dead, respawn + retry.
cc_send() {
  local rp="$1" out
  out="$(atm send "$SESSION" --pane="$CC_PANE" --file "$rp" --no-cass-check --force-non-interactive --json 2>/dev/null || true)"
  printf '%s' "$out" | grep -q '"delivered":1' && return 0
  log "claude send not delivered — respawning + retry"
  respawn_pane "$CC_PANE" cc || true
  out="$(atm send "$SESSION" --pane="$CC_PANE" --file "$rp" --no-cass-check --force-non-interactive --json 2>/dev/null || true)"
  printf '%s' "$out" | grep -q '"delivered":1'
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
  local _route_t0; _route_t0="$(date +%s)"   # ml8.6: route latency clock
  # Single source of truth for both panes: a per-route packet copy with the nonce-tag
  # appended (so the verdict line carries this route's nonce). The tag deliberately
  # avoids a bare "<nonce> CONFIRMED/REFUTED" pair, so an echo of it can't match the parser.
  local rp="$EVID_DIR/${bead}.packet.md"
  { cat "$packet"; printf '\n\n--- VERDICT FORMAT (required) ---\nEnd your reply with ONE line exactly:\n  PAWL %s <the single word CONFIRMED or REFUTED>\n' "$nonce"; } > "$rp"

  log "route $bead -> both panes (packet=$packet, pr=$pr, nonce=$nonce)"
  # Both read the SAME nonce-tagged packet. Robust sends (retry + respawn), never `die` on a
  # flaky send — a failed send is recovered, not fatal.
  cc_send "$rp"  || log "claude pane did not engage on send — poll/reroute will recover"
  cod_send "$rp" || log "codex pane did not engage after retries — poll/reroute will recover"

  # Poll both panes for THIS route's nonce-tagged verdict (bounded). A pane that goes
  # DEGRADED mid-route (dead shell, usage-limit, stuck dialog) is respawned + re-routed once
  # — otherwise it would silently time out into a false REFUTED (a fail-OPEN of the gate's
  # intent). The reroute is the heart of S3's auto-respawn-and-reroute.
  local waited=0 vc="" vd="" cc_rr=0 cod_rr=0 cs=""
  while [ "$waited" -lt "$ROUTE_TIMEOUT" ]; do
    [ -z "$vc" ] && vc="$(verdict_of "$CC_PANE" "$nonce")"
    [ -z "$vd" ] && vd="$(verdict_of "$COD_PANE" "$nonce")"
    [ -n "$vc" ] && [ -n "$vd" ] && break
    if [ -z "$vd" ] && [ "$cod_rr" -lt 1 ]; then
      cs="$(codex_state || echo unknown)"
      if cod_dead; then
        # Positively dropped to a shell — preflight misreads this as goal-completed, so the
        # shell check is the only reliable signal. Respawn FIRST (cod_send alone would re-send
        # into a shell). Positive detection avoids respawning a live-but-chrome-less pane.
        log "codex pane dropped to a shell mid-route (preflight=$cs, misclassified) — respawn + reroute"
        respawn_pane "$COD_PANE" cod || true; cod_send "$rp" || true; cod_rr=1
      else
        case "$cs" in
          # `|| true` on rotate_account is LOAD-BEARING: it returns 1 when PAWL_AUTO_ROTATE is off
          # (default), and an unprotected non-zero would abort the whole route under set -e before
          # the respawn/reroute ever runs (proven: the route would die on a usage-limit).
          usage-limit) log "codex usage-limit mid-route — rotate + respawn + reroute"; rotate_account cod || true; respawn_pane "$COD_PANE" cod || true; cod_send "$rp" || true; cod_rr=1 ;;
          unknown|absent|stale-scrollback|replace-goal-dialog) log "codex degraded mid-route ($cs) — respawn + reroute"; respawn_pane "$COD_PANE" cod || true; cod_send "$rp" || true; cod_rr=1 ;;
        esac
      fi
    fi
    if [ -z "$vc" ] && [ "$cc_rr" -lt 1 ] && ! cc_alive; then
      log "claude degraded mid-route (dropped to shell) — respawn + reroute"
      respawn_pane "$CC_PANE" cc || true; cc_send "$rp" || true; cc_rr=1
    fi
    sleep 5; waited=$((waited + 5))
  done
  tmux capture-pane -p -t "${SESSION}.${CC_PANE}" -S -60 > "$ev_cc" 2>&1 || true
  tmux capture-pane -p -t "${SESSION}.${COD_PANE}" -S -80 > "$ev_cod" 2>&1 || true

  log "opus=${vc:-<timeout>} codex=${vd:-<timeout>}"
  # ml8.6: record one SLO datapoint per route (latency + agreement) for `pawl metrics`.
  # Non-blocking + fail-safe — metrics must NEVER affect the verdict (the `|| true`).
  { _lat=$(( $(date +%s) - _route_t0 ))
    _mdisp="REFUTED"; [ "$vc" = "CONFIRMED" ] && [ "$vd" = "CONFIRMED" ] && _mdisp="CONFIRMED"
    _magree="disagree"; [ -n "$vc" ] && [ "$vc" = "$vd" ] && _magree="agree"
    mkdir -p "$ROOT/$STATE_DIR"
    printf '{"ts":"%s","bead":"%s","latency_s":%d,"opus":"%s","codex":"%s","disposition":"%s","agreement":"%s"}\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$bead" "$_lat" "${vc:-timeout}" "${vd:-timeout}" "$_mdisp" "$_magree" \
      >> "$ROOT/$STATE_DIR/metrics.jsonl"
  } 2>/dev/null || true
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
  # Fail-closed: any non-CONFIRMED or timeout. STILL record the REFUTED verdict
  # (age-uxva) — a REFUTE is the membrane catch we MOST want in the yield ledger,
  # and the log emit lives in `pawl-verdict.sh write`. Without this, every
  # standing-pawl REFUTE bypassed the chokepoint and was never logged. Mirrors
  # the CONFIRMED branch; non-blocking (the gate signal is still `return 1`).
  # Per-pane verdicts carry their actual result; a timeout maps to REFUTED.
  local rhead; rhead="$(git rev-parse HEAD)"
  bash "$ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
    --disposition REFUTED --head "$rhead" \
    --author-context "pawl-route-author-${bead}" --mode multi-model \
    --refuter "claude:${vc:-REFUTED}:opus-pawl-pane-fresh:${ev_cc}" \
    --refuter "gpt:${vd:-REFUTED}:codex-pawl-pane-gpt55:${ev_cod}" \
    --reason "standing-pawl route: no agreement (opus=${vc:-timeout} codex=${vd:-timeout})" >&2 || true
  log "ROUTE $bead: REFUTED/HOLD — opus=${vc:-timeout} codex=${vd:-timeout} (no agreement; evidence in $EVID_DIR)"
  echo "REFUTED"
  return 1
}

# ml8.6: SLO surface over the recorded routes — p50/p95 round-trip latency + agreement rate
# (both-CONFIRMED vs disagreement). Reads the append-only metrics.jsonl cmd_route writes.
cmd_metrics() {
  local mf="$ROOT/$STATE_DIR/metrics.jsonl" json=0
  [ "${1:-}" = "--json" ] && json=1
  if [ ! -s "$mf" ]; then
    [ "$json" = 1 ] && echo '{"routes":0}' || echo "pawl metrics: no routed beads recorded yet ($mf)"
    return 0
  fi
  python3 - "$mf" "$json" <<'PY'
import json,sys
mf,asjson=sys.argv[1],sys.argv[2]=="1"
# Fail-SOFT: a corrupt/partial append (e.g. a route killed mid-write) must not crash the
# SLO surface — skip unparseable lines, report on the rest.
rows=[]
for l in open(mf):
    l=l.strip()
    if not l: continue
    try: rows.append(json.loads(l))
    except Exception: continue
n=len(rows)
lat=sorted(int(r.get("latency_s",0)) for r in rows)
def pct(p):
    if not lat: return 0
    return lat[min(len(lat)-1,int(round((p/100.0)*(len(lat)-1))))]
agree=sum(1 for r in rows if r.get("agreement")=="agree")
dis=n-agree
out={"routes":n,"latency_p50_s":pct(50),"latency_p95_s":pct(95),
     "agreement_rate":round(agree/n,3) if n else 0,"agree":agree,"disagreements":dis}
if asjson:
    print(json.dumps(out))
else:
    print(f"pawl metrics: {n} routed beads")
    print(f"  latency p50={out['latency_p50_s']}s p95={out['latency_p95_s']}s")
    print(f"  agreement {agree}/{n} ({out['agreement_rate']}); disagreements={dis}")
PY
}

# Dispatch only when EXECUTED, not when SOURCED — so tests can source this file to exercise
# the pure helpers (cod_dead, verdict_of, …) without running a command.
[ "${BASH_SOURCE[0]:-$0}" = "${0}" ] || return 0

case "${1:-}" in
  up)     shift; cmd_up "$@" ;;
  down)   shift; cmd_down "$@" ;;
  health) shift; cmd_health "$@" ;;
  route)  shift; cmd_route "$@" ;;
  metrics) shift; cmd_metrics "$@" ;;
  *) cat >&2 <<'H'
Usage: pawl.sh <up|down|health|route|metrics>
  up                          spawn + readiness-gate the standing pawl session (idempotent)
  down                        tear down the standing session
  health [--json]             per-pane liveness/readiness
  route <bead> <packet> [pr]  route a review packet to opus+codex, require agreement, record verdict
  metrics [--json]            SLO surface over recorded routes: p50/p95 latency + agreement rate

route is self-healing (S3): sends retry with engagement verification (never aborts on a
flaky codex send); a pane that goes degraded mid-route (dead shell, usage-limit, stuck
dialog) is respawned and re-routed once, so it cannot silently time out into a false
REFUTED. On a usage-limit set PAWL_AUTO_ROTATE=1 to rotate the account (caam / claude-acct)
before respawn. Tunables: PAWL_ROUTE_TIMEOUT (default 320s), PAWL_AUTO_ROTATE.
H
    exit 2 ;;
esac
