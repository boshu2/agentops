#!/usr/bin/env bash
# pawl.sh — the standing cross-family pawl service (age-standing-pawl-service-ml8).
#
# Encodes the manual opus+codex "duel" dogfooded on 2026-06-18 (landing age-1vy +
# age-uqj) into a deterministic lifecycle so the cross-family pawl is a SERVICE you
# route requests to, not a hand-spun codex-exec per bead. `ao pawl` wraps this;
# the deterministic core lives here (CLI-for-deterministic, skills-are-instructions).
#
# Subcommands:
#   up [--dual|--tri|--models a,b,c]   spawn the standing session + readiness-gate the
#                                      ENABLED panes (idempotent). Default: probe what's
#                                      installed and stand up the strongest membrane possible.
#   down                     kill the standing session (no orphan panes)
#   reap                     tear down the session iff idle longer than PAWL_IDLE_TTL (no-op otherwise)
#   health [--json]          per-pane liveness/readiness probe + the session's membrane tier
#   route <bead> <packet> [pr]   route a review packet to the ENABLED panes, require tier-
#                                appropriate agreement, capture evidence, record the verdict
#                                (pr default 0 = push-to-main)
#
# Capability-adaptive (age-4o33): the warm membrane stands up the STRONGEST reviewer panel the
# host's installed CLIs can form, over the three paid cross-family families (one pane each):
#   cc = claude/opus   cod = codex/gpt   agy = antigravity/Gemini 3.5 Flash
# (Local llama is eval-only by decision — never probed into the warm service.) Each verdict is
# stamped with the TIER it achieved, so a door decides sufficiency, not the service:
#   tier=multi  >=2 families  -> the real cross-family gate (mode=multi-model)
#   tier=fresh  1 family      -> a single fresh-context refuter (mode=fresh-context) — valid for
#                                normal doors (pawls.md fresh-context default); a high-irreversibility
#                                door (e.g. push-to-main) still demands multi-model and refuses it.
#
# Hard rules learned in the dogfood (do not regress):
#  - codex /goal caps the objective at ~4000 chars -> ALWAYS send the packet as a FILE
#    reference, never inline (an 8373-char inline paste was rejected).
#  - gate readiness before the first route (boot race): spawn returns before panes boot.
#  - codex 'codex exec --model gpt-5.3-codex' is rejected on a ChatGPT account; the
#    interactive ATM codex pane resolves gpt-5.5 and works.
#  - agreement is fail-closed & ALL-CONFIRM over the ENABLED panes: any REFUTED -> REFUTED;
#    a pass needs every replier to CONFIRM and >= the tier's min (2 for multi, 1 for fresh);
#    a model unavailable after retries degrades to the remaining repliers (never below the min).
set -euo pipefail

SESSION="${PAWL_SESSION:-agentops--pawl-service}"
LABEL="${PAWL_LABEL:-pawl-service}"
PROJECT="${PAWL_PROJECT:-agentops}"
CC_PANE="${PAWL_CC_PANE:-1}"     # claude/opus pane (fresh-context refuter)
COD_PANE="${PAWL_COD_PANE:-2}"   # codex pane (cross-family refuter)
AGY_PANE="${PAWL_AGY_PANE:-3}"   # AGY/Antigravity pane (3rd cross-family refuter, Gemini 3.5 Flash)
EVID_DIR="${PAWL_EVID_DIR:-/tmp/pawl-evidence}"
STATE_DIR="${PAWL_STATE_DIR:-.agents/pawl}"
ROUTE_TIMEOUT="${PAWL_ROUTE_TIMEOUT:-320}"   # seconds to wait per pane for a VERDICT
PAWL_IDLE_TTL="${PAWL_IDLE_TTL:-1800}"       # idle seconds before `reap` tears the session down
PAWL_STALL_GIVEUP="${PAWL_STALL_GIVEUP:-150}"  # age-djfo: a pane showing NO new output for this
                                               # long is given up (alive-but-stuck) -> degrade fast
                                               # instead of burning the full ROUTE_TIMEOUT. 0 disables.
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ENABLED="${ENABLED:-}"           # resolved family list (space-sep, canonical order); set by up/load_session
TIER="${TIER:-}"                 # multi (>=2 families) | fresh (1) | "" (none)

die() { echo "pawl: $*" >&2; exit 1; }
log() { echo "pawl: $*" >&2; }

# ── Capability layer (age-4o33): turn "what's installed" into "which panes, tier, decision" ──
# All pure (no tmux/atm) — locked by tests/scripts/pawl-adaptive.bats.
PAWL_CANON_FAMILIES="cc cod agy"

# family token -> its CLI binary name (for the install probe).
_family_bin() { case "$1" in cc) echo claude ;; cod) echo codex ;; agy) echo agy ;; *) echo "$1" ;; esac; }

# Indirection so tests can stub presence without touching PATH.
_cli_present() { command -v "$1" >/dev/null 2>&1; }

# Installed families, canonical order, one per line. A family is "available" iff its CLI binary
# is on PATH — the dominant real signal for "I don't have that account"; a present-but-logged-out
# CLI is caught later by the readiness gate, not here.
probe_families() {
  local f
  for f in $PAWL_CANON_FAMILIES; do _cli_present "$(_family_bin "$f")" && echo "$f"; done
}

# Normalize a pin (--dual/--tri or a --models csv of family tokens/aliases) to canonical family
# tokens (space-sep, canonical order). Empty input -> empty (caller uses the probe). An unknown
# token -> exit 2 (fail-fast on a typo'd pin, never a silent drop).
parse_pin() {
  local raw="${1:-}" req="" f tok
  [ -z "$raw" ] && return 0
  case "$raw" in dual) raw="cc,cod" ;; tri) raw="cc,cod,agy" ;; esac
  local _toks; IFS=',' read -ra _toks <<< "$raw"
  for tok in "${_toks[@]}"; do
    case "$(printf '%s' "$tok" | tr 'A-Z' 'a-z' | tr -d ' ')" in
      cc|claude|opus|sonnet)   f=cc ;;
      cod|codex|gpt|openai)    f=cod ;;
      agy|gemini|antigravity)  f=agy ;;
      "") continue ;;
      *) echo "pawl: unknown model '$tok' (use cc/cod/agy, dual, or tri)" >&2; return 2 ;;
    esac
    case " $req " in *" $f "*) : ;; *) req="$req $f" ;; esac
  done
  for f in $PAWL_CANON_FAMILIES; do case " $req " in *" $f "*) echo "$f" ;; esac; done
}

# 1-based pane index of a family within the ORDERED enabled set (panes are spawned in canonical
# order, so a missing family shifts the tail up). Empty if the family is absent.
pane_index() {
  local target="$1"; shift
  local i=0 f
  for f in "$@"; do i=$((i + 1)); [ "$f" = "$target" ] && { echo "$i"; return 0; }; done
  return 0
}

# Membrane tier from the enabled family count: multi (>=2, real cross-family) | fresh (1,
# single-family fresh-context) | "" (0, cannot run).
tier_of() {
  if [ "$#" -ge 2 ]; then echo multi; elif [ "$#" -eq 1 ]; then echo fresh; else echo ""; fi
}

# Min confirmers for a tier to PASS: multi needs >=2 (cross-family), fresh needs 1. An empty tier
# returns an unreachable-high threshold so it can never pass.
min_confirm_for_tier() { case "$1" in multi) echo 2 ;; fresh) echo 1 ;; *) echo 99 ;; esac; }

# Resolve the per-family pane vars + ENABLED + TIER from an ordered family list. A disabled
# family's pane var is "" so every send/poll/recovery/refuter site skips it.
_set_panes_from_enabled() {
  ENABLED="$*"
  CC_PANE="$(pane_index cc "$@")"
  COD_PANE="$(pane_index cod "$@")"
  AGY_PANE="$(pane_index agy "$@")"
  TIER="$(tier_of "$@")"
}

_now() { date +%s; }

# Persist the resolved session so health/route/reap (fresh processes) operate over the same set.
_write_session_json() {
  mkdir -p "$ROOT/$STATE_DIR"
  local now; now="$(_now)"
  printf '{"session":"%s","families":"%s","tier":"%s","cc_pane":"%s","cod_pane":"%s","agy_pane":"%s","ready":true,"up_ts":%s,"last_route_ts":%s}\n' \
    "$SESSION" "$ENABLED" "$TIER" "$CC_PANE" "$COD_PANE" "$AGY_PANE" "$now" "$now" > "$ROOT/$STATE_DIR/session.json"
}

# Load families/tier/panes from session.json into the globals. A missing/legacy file defaults to
# the full 3-family layout (back-compat with pre-age-4o33 sessions).
load_session() {
  local sj="$ROOT/$STATE_DIR/session.json" fams=""
  if [ -f "$sj" ]; then
    # Tolerate optional whitespace after the colon (age-nomq): a reader must parse the file
    # regardless of which writer (compact printf or any JSON formatter) last touched it.
    fams="$(grep -oE '"families": *"[^"]*"' "$sj" 2>/dev/null | sed -E 's/.*: *"([^"]*)".*/\1/')"
  fi
  if [ -n "$fams" ]; then _set_panes_from_enabled $fams; else _set_panes_from_enabled cc cod agy; fi
}

# Idle seconds since the last route (-1 if no session.json or no timestamp).
_session_idle() {
  local sj="$ROOT/$STATE_DIR/session.json" last
  [ -f "$sj" ] || { echo -1; return 0; }
  last="$(grep -oE '"last_route_ts": *[0-9]+' "$sj" 2>/dev/null | grep -oE '[0-9]+' | tail -1)"
  [ -n "$last" ] || { echo -1; return 0; }
  echo $(( $(_now) - last ))
}

# Reset the idle clock (best-effort; never affects the verdict).
_touch_route_ts() {
  local sj="$ROOT/$STATE_DIR/session.json"
  [ -f "$sj" ] || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  python3 - "$sj" "$(_now)" <<'PY' 2>/dev/null || true
import json,sys
sj,now=sys.argv[1],int(sys.argv[2])
try:
    d=json.load(open(sj)); d["last_route_ts"]=now
    # COMPACT separators (age-nomq): match _write_session_json's printf format exactly. Python's
    # default json.dump emits "key": "val" (space after colon), which the grep readers in
    # load_session / _session_idle (compact "key":"val") then fail to parse — silently reverting
    # the session to the 3-family default + a permanently-stale idle clock after the first route.
    json.dump(d,open(sj,"w"),separators=(",",":"))
except Exception:
    pass
PY
}

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
    elif [ "$kind" = "agy" ]; then
      clear_known_prompts "$AGY_PANE" || true; agy_ready && return 0
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

# --- AGY / Antigravity (Gemini 3.5 Flash) pane: 3rd cross-family refuter ---

# agy pane is DEAD iff its FOREGROUND PROCESS is a shell (mirrors cod_dead's
# deterministic pane_current_command check — immune to scrollback contents). A
# live agy pane runs the `agy` binary as the foreground command.
agy_dead() {
  local cmd
  cmd="$(tmux display-message -p -t "${SESSION}.${AGY_PANE}" '#{pane_current_command}' 2>/dev/null)" || return 1
  case "$cmd" in
    zsh|-zsh|bash|-bash|sh|-sh|fish|-fish|tcsh|-tcsh|csh|ksh|dash|login) return 0 ;; # shell => DEAD
    *) return 1 ;;  # agy binary (or empty/unknown read) => treat as alive (conservative)
  esac
}

# --- age-djfo: detect + dismiss the known CLI interruption prompts that BLOCK a warm pane ---
# A warm pane can be ALIVE (foreground = the binary) yet stuck on a periodic prompt that is NOT
# the trust gate: codex's "Update available … Press enter to continue" dialog and agy's "How's
# the CLI experience? [0] Skip" survey. Unhandled, they leave health falsely green and make the
# route burn the full ROUTE_TIMEOUT then degrade. These three helpers (the first two PURE) detect
# the known blockers and clear them with the default-accept / skip key.

# Classify a known blocking prompt from a pane's captured text. Echoes the type, or "" if none.
detect_blocking_prompt() {
  local t="$1"
  if printf '%s' "$t" | grep -qiE "trust this folder|trust this directory|requires permission to read"; then
    echo trust-gate
  elif printf '%s' "$t" | grep -qiE "update now|skip until next version"; then
    # MENU form of codex's update prompt: "› 1. Update now (runs brew upgrade --cask codex) /
    # 2. Skip / 3. Skip until next version". The default-highlighted option is "Update now", so a
    # plain Enter (the codex-update key below) SELECTS it -> runs the brew upgrade and stalls the
    # warm pane (the exact degradation this guards against; observed 2026-06-26). Classify it
    # distinctly so it gets a navigate-to-Skip key, not Enter. Checked BEFORE codex-update because
    # the menu also contains "update available".
    echo codex-update-menu
  elif printf '%s' "$t" | grep -qiE "update available|press enter to continue|a new version|update & restart"; then
    echo codex-update
  elif printf '%s' "$t" | grep -qiE "how('?s| is| was) (the|your) cli experience|\[0\][[:space:]]*skip|rate (the|your) experience"; then
    echo agy-survey
  else
    echo ""
  fi
}

# The tmux send-keys argument(s) that dismiss a given prompt type (default-accept / skip).
prompt_dismiss_key() {
  case "$1" in
    trust-gate)        echo "Enter" ;;        # Enter = "Yes, I trust this folder"
    codex-update)      echo "Enter" ;;        # plain "Press enter to continue" form
    codex-update-menu) echo "Down Enter" ;;   # arrow-select menu: Down to "Skip", Enter to take it
                                              # (NOT Enter on the default, which is "Update now")
    agy-survey)        echo "0 Enter" ;;      # "[0] Skip"
    *)            echo "" ;;
  esac
}

# Detect + dismiss any known blocking prompt on a pane (generalizes the old agy trust-gate clear
# to every pane + every known prompt). Idempotent: sends keys ONLY when a prompt is showing, so it
# cannot perturb a ready pane. Returns 0 if it dismissed something, 1 if none (or unreadable).
clear_known_prompts() {
  local pane="$1" txt typ keys
  # BOTTOM-ANCHOR (cross-family review catch): an interactive prompt is the ACTIVE bottom UI of the
  # pane; the diff/packet a reviewer is reading sits in the scrollback BODY above it. Inspecting the
  # whole capture let a reviewed diff that merely CONTAINS "press enter to continue" / "[0] Skip" /
  # "trust this folder" trigger a key-injection into a working reviewer pane (a lost-verdict fail-open).
  # Only the last few lines (the live prompt region) are inspected. The route also gates this on STALL
  # (it only calls clear on a pane producing NO new output), so keys can never hit a producing pane.
  txt="$(tmux capture-pane -p -t "${SESSION}.${pane}" 2>/dev/null | tail -n 10)" || return 1
  typ="$(detect_blocking_prompt "$txt")"
  [ -n "$typ" ] || return 1
  keys="$(prompt_dismiss_key "$typ")"
  [ -n "$keys" ] || return 1
  log "clearing '$typ' prompt on pane $pane"
  # $keys is a deliberate multi-key sequence (e.g. "0 Enter") — word-splitting is intended.
  # shellcheck disable=SC2086
  tmux send-keys -t "${SESSION}.${pane}" $keys 2>/dev/null || true
  sleep 2
  return 0
}

# Back-compat wrapper: the agy trust-gate clear is now the general prompt clear on the agy pane.
agy_clear_trust_gate() { clear_known_prompts "$AGY_PANE"; }

# age-djfo (c): PURE — a pane is "given up" once it has shown NO new output for the stall budget
# (it is alive but stuck on an unclearable hang). Returns 0 (give up) iff budget>0 and the stall
# seconds have reached it. A pane that is making progress (or whose known prompt got cleared, which
# CHANGES its output) resets its stall counter and is never given up.
_stall_over_budget() { [ "${2:-0}" -gt 0 ] && [ "${1:-0}" -ge "${2}" ]; }

# agy pane is READY when the agy binary is POSITIVELY the foreground process AND it is not
# sitting on the trust gate. Fail-CLOSED on every uncertainty so a missing/unreadable pane, a
# shell, or any other program is NEVER reported ready — otherwise health/up would green-light a
# two-pane or wrong-program session as tri-model and route packets to the wrong place.
agy_ready() {
  local cmd pane_txt
  # Positive foreground check: require exactly the agy binary. A display-message failure
  # (pane absent/unreadable) returns non-zero -> NOT ready. (agy_dead's "unknown => alive"
  # bias is right for respawn decisions but wrong for a readiness gate, so check directly.)
  cmd="$(tmux display-message -p -t "${SESSION}.${AGY_PANE}" '#{pane_current_command}' 2>/dev/null)" || return 1
  [ "$cmd" = "agy" ] || return 1
  # READ-ONLY predicate (cross-family review catch, round 2): agy_ready must NOT send keys. It is
  # called by cmd_health/_enabled_ready/respawn, and cmd_health can run WHILE the agy pane is mid-
  # review — an unconditional clear here could inject dismiss keys into a producing reviewer pane
  # (the same lost-verdict fail-open, off the route's stall protection). Detect-only here; the
  # CLEARING happens at the safe action sites (the readiness gate _enabled_ready, respawn_pane, and
  # the stall-gated route poll) — never in this predicate.
  # Require a SUCCESSFUL capture that shows NO known blocking prompt (trust-gate OR the agy survey,
  # age-djfo) in the live bottom region; a capture failure -> NOT ready (fail-closed).
  pane_txt="$(tmux capture-pane -p -t "${SESSION}.${AGY_PANE}" 2>/dev/null | tail -n 10)" || return 1
  [ -z "$(detect_blocking_prompt "$pane_txt")" ]
}

# Robust agy send: deliver the packet file; if not delivered or the pane is dead,
# respawn + retry (mirrors cc_send). agy reads the file and emits the verdict line.
agy_send() {
  local rp="$1" out
  out="$(atm send "$SESSION" --pane="$AGY_PANE" --file "$rp" --no-cass-check --force-non-interactive --json 2>/dev/null || true)"
  printf '%s' "$out" | grep -q '"delivered":1' && return 0
  log "agy send not delivered — respawning + retry"
  respawn_pane "$AGY_PANE" agy || true
  out="$(atm send "$SESSION" --pane="$AGY_PANE" --file "$rp" --no-cass-check --force-non-interactive --json 2>/dev/null || true)"
  printf '%s' "$out" | grep -q '"delivered":1'
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

# True iff every ENABLED family's pane is route-ready.
_enabled_ready() {
  # age-djfo: dismiss any known blocking prompt during boot too (a codex update dialog at launch
  # would otherwise keep the pane un-ready for the full gate). agy_ready already clears its pane.
  case " $ENABLED " in *" cc "*) clear_known_prompts "$CC_PANE" || true; cc_ready || return 1 ;; esac
  case " $ENABLED " in *" cod "*) clear_known_prompts "$COD_PANE" || true; local cs; cs="$(codex_state || true)"; { [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; } || return 1 ;; esac
  case " $ENABLED " in *" agy "*) clear_known_prompts "$AGY_PANE" || true; agy_ready || return 1 ;; esac
  return 0
}

# Per-enabled-family readiness summary for the timeout diagnostic.
_ready_debug() {
  local out=""
  case " $ENABLED " in *" cc "*) out="$out cc=$(cc_ready && echo yes || echo no)" ;; esac
  case " $ENABLED " in *" cod "*) out="$out codex=$(codex_state || echo '?')" ;; esac
  case " $ENABLED " in *" agy "*) out="$out agy=$(agy_ready && echo yes || echo no)" ;; esac
  printf '%s' "${out# }"
}

# Human phrase for the tier achieved (printed on `up`).
_tier_phrase() {
  case "$TIER" in
    multi) echo "cross-family gate" ;;
    fresh) echo "single-family fresh-context — weaker; add codex or agy for the cross-family gate" ;;
    *) echo "no reviewers" ;;
  esac
}

cmd_up() {
  # Optional pin: --dual (cc,cod) | --tri (all) | --models <csv>. Default: probe what's installed.
  local pin=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dual) pin="dual" ;;
      --tri)  pin="tri" ;;
      --models) shift; pin="${1:-}" ;;
      --models=*) pin="${1#--models=}" ;;
      *) ;;
    esac
    shift || true
  done

  # Resolve the enabled family set: an explicit pin (validated against what's installed) or the
  # install probe. Fail-fast on a pin that names a CLI the host doesn't have.
  local probe enabled f
  probe="$(probe_families | tr '\n' ' ')"
  if [ -n "$pin" ]; then
    enabled="$(parse_pin "$pin")" || die "bad model pin '$pin'"
    [ -n "$(printf '%s' "$enabled" | tr -d '[:space:]')" ] || die "pin '$pin' resolved to no families"
    for f in $enabled; do
      case " $probe " in *" $f "*) : ;; *) die "pinned family '$f' (CLI '$(_family_bin "$f")') is not installed — drop it from --models or install it" ;; esac
    done
  else
    enabled="$probe"
  fi
  set -- $enabled
  [ "$#" -ge 1 ] || die "no pawl families installed — need at least one of: claude, codex, agy"
  _set_panes_from_enabled "$@"

  # Build the atm spawn flags from the enabled set (canonical order: cc, then cod, then agy).
  local -a spawn_flags=(--no-user)
  case " $ENABLED " in *" cc "*) spawn_flags+=(--cc=1:opus) ;; esac
  case " $ENABLED " in *" cod "*) spawn_flags+=(--cod=1) ;; esac
  case " $ENABLED " in *" agy "*) spawn_flags+=(--agy=1) ;; esac

  if session_exists; then
    log "session $SESSION already exists — gating readiness (idempotent up)"
  else
    log "spawning standing pawl session $SESSION (families: $ENABLED, tier=$TIER, no-user)"
    atm spawn "$PROJECT" --label "$LABEL" "${spawn_flags[@]}" \
      --no-cass-context --ready-timeout=2m --json >/dev/null 2>&1 \
      || die "atm spawn failed"
  fi

  # Readiness gate over ONLY the enabled panes (boot race). agy boots slower; keep 45 ticks.
  for _ in $(seq 1 45); do
    if _enabled_ready; then
      _write_session_json
      log "UP: ready — families: $ENABLED, tier=$TIER ($(_tier_phrase))"
      return 0
    fi
    sleep 4
  done
  die "readiness gate timed out (families=$ENABLED; $(_ready_debug))"
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

# Idle reaper (Bo's "idle-TTL auto-down"): tear the session down iff it has been idle longer than
# PAWL_IDLE_TTL. No-op if no session, not-idle, or unmeasurable. AgentOps ships no in-repo daemon
# (ADR-0009), so reaping is schedule/event-driven: a substrate (NTM/cron) calls `ao pawl reap` on
# a cadence, and the shared lazy-auto-up re-ups the service on the next review.
cmd_reap() {
  session_exists || { log "REAP: no session (no-op)"; return 0; }
  local idle; idle="$(_session_idle)"
  if [ "$idle" -ge 0 ] && [ "$idle" -gt "$PAWL_IDLE_TTL" ]; then
    log "REAP: session idle ${idle}s > TTL ${PAWL_IDLE_TTL}s — tearing down"
    cmd_down
  else
    log "REAP: session active (idle=${idle}s, TTL=${PAWL_IDLE_TTL}s) — keeping warm"
  fi
}

cmd_health() {
  load_session
  local json="${1:-}"
  local cc="n/a" cs="n/a" agy="n/a"
  case " $ENABLED " in *" cc "*) cc_ready && cc="ready" || cc="not-ready" ;; esac
  case " $ENABLED " in *" cod "*) cs="$(codex_state || echo absent)" ;; esac
  case " $ENABLED " in *" agy "*) agy_ready && agy="ready" || agy="not-ready" ;; esac
  # age-djfo: codex preflight can report 'codex-live' while the pane is actually BLOCKED on the
  # update dialog (falsely green). Detect the known blocker directly so health is honest + the
  # verdict below (which only accepts codex-live|goal-completed) refuses a stuck pane.
  case " $ENABLED " in *" cod "*)
    if [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; then
      local _ct _cb
      _ct="$(tmux capture-pane -p -t "${SESSION}.${COD_PANE}" 2>/dev/null || true)"
      _cb="$(detect_blocking_prompt "$_ct")"
      [ -n "$_cb" ] && cs="stuck:$_cb"
    fi ;;
  esac
  if [ "$json" = "--json" ]; then
    printf '{"session":"%s","exists":%s,"families":"%s","tier":"%s","cc_pane":{"pane":"%s","state":"%s"},"cod_pane":{"pane":"%s","state":"%s"},"agy_pane":{"pane":"%s","state":"%s"}}\n' \
      "$SESSION" "$(session_exists && echo true || echo false)" "$ENABLED" "$TIER" "$CC_PANE" "$cc" "$COD_PANE" "$cs" "$AGY_PANE" "$agy"
  else
    echo "session=$SESSION exists=$(session_exists && echo yes || echo no) tier=$TIER cc[${CC_PANE:-–}]=$cc codex[${COD_PANE:-–}]=$cs agy[${AGY_PANE:-–}]=$agy"
  fi
  # Healthy iff the session exists AND every ENABLED family is ready.
  session_exists || return 1
  case " $ENABLED " in *" cc "*) [ "$cc" = "ready" ] || return 1 ;; esac
  case " $ENABLED " in *" cod "*) { [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; } || return 1 ;; esac
  case " $ENABLED " in *" agy "*) [ "$agy" = "ready" ] || return 1 ;; esac
  return 0
}

# Parse THIS route's verdict from a pane capture. Scoped by a per-route nonce so a
# prior route's verdict still in the scrollback can never be read as this one's
# (the stale-scrollback bug the self-review caught). The reviewer is asked to tag
# its verdict line `PAWL <nonce> <CONFIRMED|REFUTED>`; the nonce makes the staleness
# false-positive impossible (the prompt never contains the reviewer's actual
# nonce+verdict pair).
#
# WHOLE-LINE ANCHOR (age-nomq): the nonce alone does NOT defeat a reviewer that NARRATES the
# format with the concrete words — codex wrote "the final line must be PAWL <nonce> CONFIRMED or
# PAWL <nonce> REFUTED. I'm keeping notes…" mid-review, and an un-anchored regex matched "PAWL
# <nonce> REFUTED" inside that prose → a FALSE verdict. The symmetric narrated-CONFIRMED case is a
# FAIL-OPEN. An END-anchor alone is ALSO insufficient: a sentence that ENDS at the token (e.g.
# "so my answer is PAWL <nonce> CONFIRMED") still false-matches (the cross-family review caught
# this). The instruction is "End your reply with ONE line exactly", so we require the WHOLE LINE
# to be the verdict: only non-alphanumeric chrome (whitespace + an optional TUI prefix like "• "
# or "│ ") may precede PAWL, and only whitespace may follow the verdict. Any PROSE before PAWL
# (letters/digits) or after the token rejects the line. Strictly safer — a miss fails CLOSED (no
# verdict → timeout → re-route), and narration can never produce a verdict in EITHER direction.
verdict_of() {
  local pane="$1" nonce="$2"
  # Must NEVER fail (returns empty until the verdict appears): a non-zero grep under the caller's
  # `set -euo pipefail` would abort the whole route on the first empty poll. `|| true` keeps the
  # substitution status 0 while still emitting any match. awk prints the LAST field (the verdict),
  # which is correct regardless of any leading TUI prefix shifting the column positions.
  { tmux capture-pane -p -t "${SESSION}.${pane}" -S -120 2>/dev/null \
    | grep -E "^[^[:alnum:]]*PAWL ${nonce} (CONFIRMED|REFUTED)[[:space:]]*$" || true; } | tail -1 | awk '{print $NF}'
}

# Pure agreement decision over the ENABLED panes' verdicts (each "CONFIRMED", "REFUTED", or
# ""=timeout/unavailable). <min> is the tier's minimum confirmers (2 multi, 1 fresh). Echoes
# "<DISPOSITION>:<detail>:<confirmed-count>":
#   CONFIRMED:full:N       every enabled pane CONFIRMED
#   CONFIRMED:degraded:N   >=min CONFIRMED but a pane was unavailable (still meets the tier min)
#   REFUTED:refuted:N      at least one pane REFUTED (a defect ANY model catches blocks)
#   REFUTED:insufficient:N fewer than <min> CONFIRMED — cannot form a tier-valid pass (fail-closed)
# ALL-CONFIRM + recall-biased: any REFUTE blocks; a pass needs every REPLIER to CONFIRM and >=min
# of them. The enabled panes are one-per-family, so >=2 confirmers are always cross-family.
pawl_decide() {
  local min="$1"; shift
  local total="$#" confirmed=0 refuted=0 replied=0 _v
  for _v in "$@"; do
    [ -n "$_v" ] && replied=$((replied + 1))
    [ "$_v" = "CONFIRMED" ] && confirmed=$((confirmed + 1))
    [ "$_v" = "REFUTED" ] && refuted=$((refuted + 1))
  done
  if [ "$refuted" -ge 1 ]; then
    echo "REFUTED:refuted:$confirmed"
  elif [ "$replied" -ge 1 ] && [ "$confirmed" -ge "$min" ] && [ "$confirmed" -eq "$replied" ]; then
    if [ "$confirmed" -eq "$total" ]; then echo "CONFIRMED:full:$confirmed"; else echo "CONFIRMED:degraded:$confirmed"; fi
  else
    echo "REFUTED:insufficient:$confirmed"
  fi
}

# Back-compat: the original 3-pane all-CONFIRM rule == pawl_decide with min 2.
pawl_decide_agreement() { pawl_decide 2 "$@"; }

cmd_route() {
  load_session
  local bead="${1:?route needs <bead>}" packet="${2:?route needs <packet-file>}" pr="${3:-0}"
  [ -f "$packet" ] || die "packet file not found: $packet"
  session_exists || die "no standing session — run 'pawl up' first"
  [ -n "$ENABLED" ] || die "session has no enabled families — re-run 'pawl up'"
  mkdir -p "$EVID_DIR"
  local ev_cc="$EVID_DIR/${bead}-opus.txt" ev_cod="$EVID_DIR/${bead}-codex.txt" ev_agy="$EVID_DIR/${bead}-agy.txt"
  # Per-route nonce scopes verdict parsing to THIS route (kills stale-scrollback +
  # echoed-instruction false positives).
  local nonce; nonce="r$(printf '%x' "$$")$(date +%s | tail -c 6)"
  local _route_t0; _route_t0="$(date +%s)"   # route latency clock
  # Single source of truth for every pane: a per-route packet copy with the nonce-tag
  # appended (so the verdict line carries this route's nonce). The tag deliberately
  # avoids a bare "<nonce> CONFIRMED/REFUTED" pair, so an echo of it can't match the parser.
  local rp="$EVID_DIR/${bead}.packet.md"
  { cat "$packet"; printf '\n\n--- VERDICT FORMAT (required) ---\nEnd your reply with ONE line exactly:\n  PAWL %s <the single word CONFIRMED or REFUTED>\n' "$nonce"; } > "$rp"

  local minc; minc="$(min_confirm_for_tier "$TIER")"
  log "route $bead -> [$ENABLED] tier=$TIER min=$minc (packet=$packet, pr=$pr, nonce=$nonce)"

  # Send to ONLY the enabled panes. Robust sends (retry + respawn); never `die` on a flaky send.
  case " $ENABLED " in *" cc "*) cc_send "$rp"  || log "claude pane did not engage on send — poll/reroute will recover" ;; esac
  case " $ENABLED " in *" cod "*) cod_send "$rp" || log "codex pane did not engage after retries — poll/reroute will recover" ;; esac
  case " $ENABLED " in *" agy "*) agy_send "$rp" || log "agy pane did not engage on send — poll/reroute will recover" ;; esac

  # A disabled family is the sentinel "n/a" (never polled, never counted). An enabled family
  # starts "" (awaiting a verdict) and is filled by verdict_of.
  local vc="n/a" vd="n/a" va="n/a"
  case " $ENABLED " in *" cc "*) vc="" ;; esac
  case " $ENABLED " in *" cod "*) vd="" ;; esac
  case " $ENABLED " in *" agy "*) va="" ;; esac
  local waited=0 cc_rr=0 cod_rr=0 agy_rr=0 cs=""
  # age-djfo (c): per-pane stall give-up. cksum (POSIX, always present — a change-detector, not
  # crypto) of recent output; unchanged for PAWL_STALL_GIVEUP seconds => alive-but-stuck => give
  # up (degrade) instead of burning the full ROUTE_TIMEOUT. Clearing a known prompt CHANGES the
  # output, so a CLEARABLE block resets the stall (the pane unblocks) rather than being given up.
  local cc_h="" cod_h="" agy_h="" cc_st=0 cod_st=0 agy_st=0 cc_gu=0 cod_gu=0 agy_gu=0 _h
  while [ "$waited" -lt "$ROUTE_TIMEOUT" ]; do
    [ -z "$vc" ] && vc="$(verdict_of "$CC_PANE" "$nonce")"
    [ -z "$vd" ] && vd="$(verdict_of "$COD_PANE" "$nonce")"
    [ -z "$va" ] && va="$(verdict_of "$AGY_PANE" "$nonce")"
    # age-djfo (a)+(c): for each enabled, still-awaiting, not-given-up pane — track output stall and
    # ONLY when STALLED (no new output this tick) try to dismiss a known blocking prompt + count the
    # stall toward give-up. Gating the clear on stall is the cross-family-review fix: a pane that is
    # actively producing review output changes its cksum every tick, so it is NEVER stalled and thus
    # NEVER gets keys injected — keys can only reach a pane that has genuinely stopped (maybe on a
    # prompt). A clearable prompt CHANGES the output next tick, resetting the stall (unblock, not give up).
    # STALL-TRACKING ONLY — the route NEVER sends keys to a pane (cross-family review, rounds 1-4).
    # Content-pattern prompt-dismissal CANNOT be made safe on a pane that might be reviewing: the
    # reviewed diff/output can itself contain a trigger phrase, so injecting Enter/"0 Enter" risks
    # derailing a producing reviewer and LOSING its verdict (a degraded false-pass). Keys are sent
    # ONLY where the pane is provably idle: the boot readiness gate (_enabled_ready) and respawn_pane.
    # Mid-route, a pane that genuinely stops (a real blocking dialog, an unknown hang) shows no new
    # output -> stalls -> is GIVEN UP (degrade, fail-closed) at PAWL_STALL_GIVEUP, never dismissed in
    # place. The cksum subshell is `|| true`-guarded so a DISAPPEARING pane (capture-pane exits
    # non-zero under pipefail) can never abort the whole route before recovery/give-up.
    if [ -z "$vc" ] && [ "$cc_gu" -eq 0 ]; then
      _h="$(tmux capture-pane -p -t "${SESSION}.${CC_PANE}" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true)"
      if [ -n "$_h" ] && [ "$_h" = "$cc_h" ]; then
        cc_st=$((cc_st + 5))
        if _stall_over_budget "$cc_st" "$PAWL_STALL_GIVEUP"; then log "claude pane stalled ${cc_st}s (no new output) — giving up (degrade)"; cc_gu=1; fi
      else cc_h="$_h"; cc_st=0; fi
    fi
    if [ -z "$vd" ] && [ "$cod_gu" -eq 0 ]; then
      _h="$(tmux capture-pane -p -t "${SESSION}.${COD_PANE}" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true)"
      if [ -n "$_h" ] && [ "$_h" = "$cod_h" ]; then
        cod_st=$((cod_st + 5))
        if _stall_over_budget "$cod_st" "$PAWL_STALL_GIVEUP"; then log "codex pane stalled ${cod_st}s (no new output) — giving up (degrade)"; cod_gu=1; fi
      else cod_h="$_h"; cod_st=0; fi
    fi
    if [ -z "$va" ] && [ "$agy_gu" -eq 0 ]; then
      _h="$(tmux capture-pane -p -t "${SESSION}.${AGY_PANE}" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true)"
      if [ -n "$_h" ] && [ "$_h" = "$agy_h" ]; then
        agy_st=$((agy_st + 5))
        if _stall_over_budget "$agy_st" "$PAWL_STALL_GIVEUP"; then log "agy pane stalled ${agy_st}s (no new output) — giving up (degrade)"; agy_gu=1; fi
      else agy_h="$_h"; agy_st=0; fi
    fi
    # Done when every enabled pane is RESOLVED — a verdict (n/a sentinel counts) OR given-up; OR
    # short-circuit the moment ANY pane REFUTES (a single refute decides the all-CONFIRM verdict).
    if { [ -n "$vc" ] || [ "$cc_gu" -eq 1 ]; } && { [ -n "$vd" ] || [ "$cod_gu" -eq 1 ]; } && { [ -n "$va" ] || [ "$agy_gu" -eq 1 ]; }; then break; fi
    { [ "$vc" = "REFUTED" ] || [ "$vd" = "REFUTED" ] || [ "$va" = "REFUTED" ]; } && break
    # Per-family mid-route recovery — ONLY for enabled, still-awaiting, NOT-given-up panes.
    if [ -z "$vd" ] && [ "$cod_gu" -eq 0 ] && [ "$cod_rr" -lt 1 ]; then
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
    if [ -z "$vc" ] && [ "$cc_gu" -eq 0 ] && [ "$cc_rr" -lt 1 ] && ! cc_alive; then
      log "claude degraded mid-route (dropped to shell) — respawn + reroute"
      respawn_pane "$CC_PANE" cc || true; cc_send "$rp" || true; cc_rr=1
    fi
    if [ -z "$va" ] && [ "$agy_gu" -eq 0 ] && [ "$agy_rr" -lt 1 ] && agy_dead; then
      log "agy degraded mid-route (dropped to shell) — respawn + reroute"
      respawn_pane "$AGY_PANE" agy || true; agy_send "$rp" || true; agy_rr=1
    fi
    sleep 5; waited=$((waited + 5))
  done
  [ -n "$CC_PANE" ]  && tmux capture-pane -p -t "${SESSION}.${CC_PANE}"  -S -60 > "$ev_cc"  2>&1 || true
  [ -n "$COD_PANE" ] && tmux capture-pane -p -t "${SESSION}.${COD_PANE}" -S -80 > "$ev_cod" 2>&1 || true
  [ -n "$AGY_PANE" ] && tmux capture-pane -p -t "${SESSION}.${AGY_PANE}" -S -80 > "$ev_agy" 2>&1 || true

  log "opus=${vc:-<timeout>} codex=${vd:-<timeout>} agy=${va:-<timeout>}"

  # --- agreement over the ENABLED panes: ALL-CONFIRM, recall-biased, degrade-on-outage ---
  # Build the verdict list from ONLY the enabled panes (drop the "n/a" sentinels), then delegate
  # to the pure pawl_decide with the tier's min confirmers.
  local -a verds=()
  [ "$vc" != "n/a" ] && verds+=("$vc")
  [ "$vd" != "n/a" ] && verds+=("$vd")
  [ "$va" != "n/a" ] && verds+=("$va")
  local total="${#verds[@]}"
  local _decision disposition detail confirmed degraded=""
  _decision="$(pawl_decide "$minc" "${verds[@]}")"
  disposition="${_decision%%:*}"; detail="$(printf '%s' "$_decision" | cut -d: -f2)"; confirmed="${_decision##*:}"
  case "$detail" in
    degraded)     degraded="degraded: ${confirmed}/${total} families CONFIRMED (tier=$TIER min=${minc} still met)" ;;
    insufficient) degraded="insufficient reviewers: ${confirmed}/${total} CONFIRMED (tier=$TIER needs >=${minc})" ;;
  esac

  # One SLO datapoint per route — non-blocking + fail-safe (must NEVER affect the verdict).
  { _lat=$(( $(date +%s) - _route_t0 ))
    _agree="disagree"; { [ "$disposition" = "CONFIRMED" ] && [ "$confirmed" -eq "$total" ]; } && _agree="agree"
    mkdir -p "$ROOT/$STATE_DIR"
    printf '{"ts":"%s","bead":"%s","tier":"%s","families":"%s","latency_s":%d,"opus":"%s","codex":"%s","agy":"%s","confirmed":%d,"total":%d,"disposition":"%s","agreement":"%s"}\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$bead" "$TIER" "$ENABLED" "$_lat" "${vc:-timeout}" "${vd:-timeout}" "${va:-timeout}" "$confirmed" "$total" "$disposition" "$_agree" \
      >> "$ROOT/$STATE_DIR/metrics.jsonl"
  } 2>/dev/null || true

  _touch_route_ts   # reset the idle-TTL clock: this route is real use

  # mode reflects the tier achieved: multi-model (>=2 families) vs fresh-context (single family).
  # A fresh-context verdict is recorded honestly; a high-irreversibility door (pawl-verdict.sh /
  # the push gate) decides whether that tier is sufficient.
  local mode="multi-model"; [ "$TIER" = "fresh" ] && mode="fresh-context"
  local head; head="$(git rev-parse HEAD)"
  if [ "$disposition" = "CONFIRMED" ]; then
    # Record ONLY the CONFIRMED enabled refuters; an unavailable/disabled pane is omitted, never
    # recorded as a false CONFIRM.
    local -a rf=()
    [ "$vc" = "CONFIRMED" ] && rf+=(--refuter "claude:CONFIRMED:opus-pawl-pane-fresh:${ev_cc}")
    [ "$vd" = "CONFIRMED" ] && rf+=(--refuter "gpt:CONFIRMED:codex-pawl-pane-gpt55:${ev_cod}")
    [ "$va" = "CONFIRMED" ] && rf+=(--refuter "gemini:CONFIRMED:agy-pawl-pane-flash35:${ev_agy}")
    bash "$ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
      --disposition CONFIRMED --head "$head" \
      --author-context "pawl-route-author-${bead}" --mode "$mode" \
      "${rf[@]}" >&2
    log "ROUTE $bead: CONFIRMED (${confirmed}/${total} agree, tier=$TIER${degraded:+; $degraded}) — verdict recorded for head $head"
    echo "CONFIRMED"
    return 0
  fi
  # Fail-closed REFUTED/HOLD: STILL record the verdict (age-uxva — the membrane catch we MOST
  # want logged; the chokepoint emit lives in pawl-verdict.sh write). Record ONLY the enabled
  # panes' actual results (a timeout maps to the REFUTED token).
  local -a rf=()
  [ "$vc" != "n/a" ] && rf+=(--refuter "claude:${vc:-REFUTED}:opus-pawl-pane-fresh:${ev_cc}")
  [ "$vd" != "n/a" ] && rf+=(--refuter "gpt:${vd:-REFUTED}:codex-pawl-pane-gpt55:${ev_cod}")
  [ "$va" != "n/a" ] && rf+=(--refuter "gemini:${va:-REFUTED}:agy-pawl-pane-flash35:${ev_agy}")
  bash "$ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
    --disposition REFUTED --head "$head" \
    --author-context "pawl-route-author-${bead}" --mode "$mode" \
    "${rf[@]}" \
    --reason "standing-pawl route: ${degraded:-no agreement} (tier=$TIER; opus=${vc:-timeout} codex=${vd:-timeout} agy=${va:-timeout})" >&2 || true
  log "ROUTE $bead: REFUTED/HOLD — tier=$TIER ${degraded:-no agreement} (evidence in $EVID_DIR)"
  echo "REFUTED"
  return 1
}

# SLO surface over the recorded routes — p50/p95 round-trip latency + agreement rate (all-enabled
# CONFIRMED vs disagreement). Reads the append-only metrics.jsonl cmd_route writes.
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
# the pure helpers (probe_families, pawl_decide, …) without running a command.
[ "${BASH_SOURCE[0]:-$0}" = "${0}" ] || return 0

case "${1:-}" in
  up)     shift; cmd_up "$@" ;;
  down)   shift; cmd_down "$@" ;;
  reap)   shift; cmd_reap "$@" ;;
  health) shift; cmd_health "$@" ;;
  route)  shift; cmd_route "$@" ;;
  metrics) shift; cmd_metrics "$@" ;;
  *) cat >&2 <<'H'
Usage: pawl.sh <up|down|reap|health|route|metrics>
  up [--dual|--tri|--models a,b,c]  spawn + readiness-gate the standing pawl session (idempotent).
                                    Default: probe installed CLIs (claude/codex/agy) and stand up
                                    the STRONGEST membrane possible. --dual=cc,cod; --tri=all;
                                    --models is an explicit family list (cc/cod/agy or aliases).
  down                              tear down the standing session
  reap                              tear down iff idle > PAWL_IDLE_TTL (substrate/cron schedules it)
  health [--json]                   per-pane liveness/readiness + the membrane tier
  route <bead> <packet> [pr]        route to the enabled panes, require tier-appropriate agreement,
                                    record the verdict (mode=multi-model for >=2 families, else fresh-context)
  metrics [--json]                  SLO surface over recorded routes: p50/p95 latency + agreement rate

The membrane is capability-adaptive: a host with only Claude gets a single fresh-context refuter
(tier=fresh, valid for normal doors); >=2 families give the cross-family gate (tier=multi). A
high-irreversibility door (push-to-main) still demands multi-model and refuses a fresh-context verdict.

route is self-healing (S3): sends retry with engagement verification (never aborts on a flaky
codex send); a pane that goes degraded mid-route (dead shell, usage-limit, stuck dialog) is
respawned and re-routed once, so it cannot silently time out into a false REFUTED. On a
usage-limit set PAWL_AUTO_ROTATE=1 to rotate the account (caam / claude-acct) before respawn.
Tunables: PAWL_ROUTE_TIMEOUT (default 320s), PAWL_IDLE_TTL (default 1800s),
PAWL_STALL_GIVEUP (default 150s; 0 disables), PAWL_AUTO_ROTATE.

Scheduling the idle reaper (age-mc3s): `reap` is the TEARDOWN half of the lazy-auto-up lifecycle,
but AgentOps ships NO in-repo daemon/scheduler (ADR-0009) — the schedule lives in your substrate:
  cron:    */30 * * * * cd /path/to/agentops && ao pawl reap >> /tmp/pawl-reap.log 2>&1
  launchd: a StartInterval=1800 agent running `ao pawl reap` in the repo dir
  NTM:     call `ao pawl reap` on a tending tick
Without a schedule the warm panes stay up until an explicit `ao pawl down`; the next review's
lazy-auto-up brings the service back. Operator pattern + rationale: docs/contracts/pawls.md.
H
    exit 2 ;;
esac
