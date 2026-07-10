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
#   doctor|smoke [--json]    read-only preflight: assert model/cwd/trust/atm/evidence readiness
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

# age-l3xj (cross-family refuter catch): SESSION is resolved BELOW (once PROJECT is known) as
# ${PROJECT}--${LABEL} — the exact name `atm spawn "$PROJECT" --label "$LABEL"` creates. Hardcoding
# it "agentops--pawl-service" while PROJECT derived from the repo made `up` from personal-site spawn
# 'personal-site--pawl-service' but check readiness on 'agentops--pawl-service' — an orphaned pane +
# a lifecycle that never sees it. PAWL_SESSION still overrides (the operator's live session).
SESSION="${PAWL_SESSION:-}"
LABEL="${PAWL_LABEL:-pawl-service}"
# PROJECT was hardcoded "agentops", but `up` from an ordinary repo resolves ROOT to THAT repo — so
# atm would spawn panes in the wrong project / fail when no agentops checkout exists. Default it to
# the resolved repo (set once ROOT is known, below); PAWL_PROJECT still overrides.
PROJECT="${PAWL_PROJECT:-}"
PAWL_CLAUDE_MODEL="${PAWL_CLAUDE_MODEL:-claude-opus-4-8}"
PAWL_CODEX_MODEL="${PAWL_CODEX_MODEL:-}"
CC_PANE="${PAWL_CC_PANE:-1}"     # claude/opus pane (fresh-context refuter)
COD_PANE="${PAWL_COD_PANE:-2}"   # codex pane (cross-family refuter)
AGY_PANE="${PAWL_AGY_PANE:-3}"   # AGY/Antigravity pane (3rd cross-family refuter, Gemini 3.5 Flash)
# age-l3xj (cross-family refuter catch): the per-route packet + evidence files are named by BEAD
# ($EVID_DIR/${bead}.packet.md, ${bead}-codex.txt); a GLOBAL /tmp/pawl-evidence meant two sessions
# (two repos) routing the SAME bead id wrote identical paths — one overwrote the other's evidence,
# and a verdict could be validated against the wrong review. Scope EVID_DIR by SESSION (resolved
# below, once SESSION is final) — the same keying the lease uses. PAWL_EVID_DIR still overrides.
EVID_DIR="${PAWL_EVID_DIR:-}"
STATE_DIR="${PAWL_STATE_DIR:-.agents/pawl}"
ROUTE_TIMEOUT="${PAWL_ROUTE_TIMEOUT:-320}"   # seconds to wait per pane for a VERDICT
PAWL_IDLE_TTL="${PAWL_IDLE_TTL:-1800}"       # idle seconds before `reap` tears the session down
PAWL_STALL_GIVEUP="${PAWL_STALL_GIVEUP:-150}"  # age-djfo: a pane showing NO new output for this
                                               # long is given up (alive-but-stuck) -> degrade fast
                                               # instead of burning the full ROUTE_TIMEOUT. 0 disables.
# age-55qz.10: ABSOLUTE per-pane deadline — a pane with NO verdict by this many seconds is
# given up (degrade), even while it KEEPS changing output (the cksum-stall heuristic misses a
# compacting opus pane that re-renders every tick). verification-surface-honesty S3: an
# EXPLICIT env value is an operator override and wins as-is (0 disables); left unset,
# cmd_route derives the effective deadline from recorded route metrics via
# resolve_engage_deadline (>= measured p95, capped at ROUTE_TIMEOUT) — a 240s static default
# under a 261s measured panel p50 degraded two live routes into give-ups on 2026-07-10.
PAWL_ENGAGE_DEADLINE_EXPLICIT="${PAWL_ENGAGE_DEADLINE:+1}"
PAWL_ENGAGE_DEADLINE="${PAWL_ENGAGE_DEADLINE:-240}"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
# age-l3xj: default the atm project to the resolved repo (basename), so `up` from any repo
# spawns panes in THAT repo rather than a hardcoded "agentops". PAWL_PROJECT overrides.
PROJECT="${PROJECT:-$(basename "$ROOT")}"
# age-l3xj: SESSION must be the name atm actually creates — ${PROJECT}--${LABEL} — so spawn,
# readiness, route, health, and teardown all target ONE session. From the agentops repo this is
# 'agentops--pawl-service' (unchanged; the operator's live session); from any other repo it tracks
# that repo. PAWL_SESSION (set above) still wins.
SESSION="${SESSION:-${PROJECT}--${LABEL}}"
# age-l3xj (cross-family refuter catch): the sibling verdict writer must resolve
# SCRIPT-RELATIVE, never under the caller-derived ROOT. ROOT is the CALLER's repo — on the
# embedded/stranger path (an installed ao in an untrusted repo) a ROOT-relative exec would
# run the caller's planted verdict writer (the very RCE the trust split closes), and in an
# ordinary repo that file does not exist at all. PAWL_SCRIPT_DIR follows the trust decision
# the Go side already made: repo/scripts/ in-checkout, the extracted trusted bundle's
# scripts/ on the embedded path (which carries pawl-verdict.sh + the schemas/ sibling it
# needs). Same idiom pawl-review.sh uses for its lib/ sources.
PAWL_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAWL_VERDICT_SH="${PAWL_VERDICT_SH:-$PAWL_SCRIPT_DIR/pawl-verdict.sh}"
# …but the verdict DATA belongs to the CALLER's repo, not to the trusted script's directory
# (cross-family refuter catch). pawl-verdict.sh defaults its output dir to $SCRIPT_DIR/../.agents/
# pawl-verdicts — on the embedded path that is the extracted temp bundle, which the Go wrapper
# DELETES on cleanup, so a CONFIRMED route would report success and leave no durable verdict.
# Pass --dir explicitly: code from the trusted bundle, verdicts into the caller's repo
# (AGENTOPS_REPO_ROOT is the seam the Go side already sets on the embedded path).
PAWL_VERDICT_DIR="${PAWL_VERDICT_DIR:-${AGENTOPS_REPO_ROOT:-$ROOT}/.agents/pawl-verdicts}"
# age-yvrp/age-l3xj: the route LEASE — an atomic exclusive lock DIRECTORY (mkdir wins once) with
# owner metadata in $ROUTE_LOCK/owner. Exactly one route owns the service at a time; down/reap
# acquire the same lease before teardown (no check-then-kill race); a stale lease is reclaimed
# once, bounded. Legacy timestamp-FILE locks from older writers are still honored + cleaned.
#
# Keyed by SESSION, NOT by repo (cross-family refuter catch): the protected resource is the GLOBAL
# tmux session, so a repo-local lease let two different repos driving the SAME session both acquire
# "their own" lock — concurrent routes into one pane set, or repo A's `down` killing repo B's live
# review. The default therefore lives outside any repo and is named for the session.
# _pawl_hex is a REVERSIBLE (perfectly injective) encoding of a string as contiguous hex bytes —
# distinct inputs ALWAYS produce distinct output, unlike any hash. `od` is POSIX; `-v` keeps
# duplicate lines. This is the injective disambiguator for lease/evidence paths.
_pawl_hex() { printf '%s' "${1:-}" | od -An -v -tx1 2>/dev/null | tr -d ' \n'; }

# _pawl_lease_slug maps a session to a filesystem-safe, path-DISTINCT slug: a readable sanitized
# hint PLUS the reversible hex of the ORIGINAL name. The sanitized charset alone is not injective
# (refuter: `sess+a` and `sess=a` both → `sess_a`), and a CRC/cksum is not either (refuter
# CONSTRUCTED two distinct sessions with the same cksum). The hex suffix is the ONLY perfectly
# injective part; the hint is cosmetic. Because the hex is pure [0-9a-f] with no `-`, it is the
# last `-`-delimited segment, so the slug is unambiguously parseable back to the session.
_pawl_lease_slug() {
  local s="${1:-}" hint
  hint="$(printf '%s' "$s" | tr -c 'A-Za-z0-9._-' '_' | cut -c1-24)"
  printf '%s-%s' "$hint" "$(_pawl_hex "$s")"
}
ROUTE_LOCK="${PAWL_ROUTE_LOCK:-${TMPDIR:-/tmp}/pawl-lease-$(_pawl_lease_slug "$SESSION").lock}"
ROUTE_LOCK="${ROUTE_LOCK//\/\//\/}"   # collapse the double slash a trailing-slash TMPDIR leaves
# age-l3xj: scope the per-route evidence dir by SESSION *and* REPO (refuter: the cross-repo contract
# lets TWO repos target one existing PAWL_SESSION; scoping evidence by session alone then let both
# repos' same-bead routes share `${bead}.packet.md` — one overwrites the other and a verdict could
# validate against the wrong repo's review). Appending hex(ROOT) makes the pair (session, repo)
# injective: two repos → distinct evidence dirs even under one session; the same repo re-reviewing a
# bead reuses its own path (fine — the verdict is head-bound and rewritten per route). Because the
# trailing hex segments are pure [0-9a-f], the (session, repo) pair is recoverable from the path.
# PAWL_EVID_DIR still overrides; the verdict records the resolved (scoped) paths, so `check` stays coherent.
EVID_DIR="${EVID_DIR:-${TMPDIR:-/tmp}/pawl-evidence-$(_pawl_lease_slug "$SESSION")-$(_pawl_hex "$ROOT")}"
EVID_DIR="${EVID_DIR//\/\//\/}"
# age-l3xj (cross-family refuter catch): the session's family/pane LAYOUT + idle clock is a property
# of the GLOBAL tmux session, not of any repo — so it must live in a SESSION-scoped SHARED location
# (like the lease), NOT repo-local `.agents/pawl/session.json`. Repo-local state meant a second repo
# routing to one existing PAWL_SESSION had no layout and defaulted to `cc cod agy`, mis-labeling a
# real `cc agy` session's AGY pane as codex. Session-scoping makes cross-repo route read the SAME
# layout `up` wrote, and (injective slug ⇒) never another session's. PAWL_SESSION_JSON overrides.
SESSION_JSON="${PAWL_SESSION_JSON:-${TMPDIR:-/tmp}/pawl-session-$(_pawl_lease_slug "$SESSION").json}"
SESSION_JSON="${SESSION_JSON//\/\//\/}"
ENABLED="${ENABLED:-}"           # resolved family list (space-sep, canonical order); set by up/load_session
TIER="${TIER:-}"                 # multi (>=2 families) | fresh (1) | "" (none)

die() { echo "pawl: $*" >&2; exit 1; }
log() { echo "pawl: $*" >&2; }

# age-l3xj (cross-family refuter catch): the repo-local state dir ($ROOT/$STATE_DIR = .agents/pawl)
# is written/deleted by up/route/down. On the installed-binary path in an UNTRUSTED repo, that repo
# can commit `.agents` or `.agents/pawl` as a SYMLINK escaping the repo — then session.json/metrics
# writes land OUTSIDE it and `down` deletes the target's session.json (data loss). Refuse a symlink
# anywhere in the ROOT→STATE_DIR chain, so state can never escape the repo. Called before any state
# write. (mkdir -p would FOLLOW a symlinked ancestor, so this must run BEFORE the first mkdir.)
_pawl_verify_state_dir() {
  local base="$ROOT" comp
  local IFS=/
  for comp in $STATE_DIR; do
    [ -z "$comp" ] && continue
    base="$base/$comp"
    [ -L "$base" ] && return 1     # a symlink anywhere in the chain => escape risk => refuse
  done
  return 0
}

# _pawl_verdict_dir_safe: the route writes verdicts into PAWL_VERDICT_DIR (default
# <repo>/.agents/pawl-verdicts) via pawl-verdict.sh (mkdir/mktemp/mv). On the untrusted-repo path
# a committed `.agents/pawl-verdicts` (or its `.agents` parent) SYMLINK would be followed, writing
# <bead>.json outside the repo (refuter catch). Refuse a symlink at the verdict dir or its parent.
_pawl_verdict_dir_safe() {
  [ -L "$PAWL_VERDICT_DIR" ] && return 1
  [ -L "$(dirname "$PAWL_VERDICT_DIR")" ] && return 1
  return 0
}
# _pawl_require_safe_state_dir dies with an actionable message when the state dir chain is unsafe.
_pawl_require_safe_state_dir() {
  _pawl_verify_state_dir || die "refusing to write pawl state: $ROOT/$STATE_DIR (or an ancestor) is a symlink — state must stay inside the repo. Remove the symlink, or set PAWL_STATE_DIR to a real in-repo path."
}

# _pawl_unlink_if_symlink neutralizes a LEAF state file that a repo committed as a symlink: the
# directory-chain guard above only covers ancestors, so `.agents/pawl/metrics.jsonl` (or session.json)
# could itself be a symlink that a `>> metrics.jsonl` append or a write would FOLLOW, corrupting an
# external target (refuter catch). Unlinking a symlink removes only the LINK — the target is
# untouched — so the subsequent write/append creates a real in-repo file. A real file is left as-is.
_pawl_unlink_if_symlink() {
  [ -L "$1" ] && rm -f "$1" 2>/dev/null
  return 0
}

# age-l3xj: resolve the ATM projects_base (the dir `atm spawn <name>` roots panes under). Prefer the
# live atm config, then NTM_PROJECTS_BASE, then the conventional ~/dev. Pure read (no spawn).
_pawl_projects_base() {
  local pb=""
  pb="$(atm config get projects_base 2>/dev/null | tr -d '[:space:]')"
  [ -n "$pb" ] || pb="${NTM_PROJECTS_BASE:-}"
  [ -n "$pb" ] || pb="$HOME/dev"
  printf '%s' "$pb"
}

# _pawl_verify_spawn_target: true iff `atm spawn "$PROJECT"` would land in THIS repo — i.e.
# projects_base/$PROJECT resolves (realpath) to $ROOT. Guards `up` from spawning into the wrong or
# a missing directory when the repo is not a direct child of projects_base (e.g. a nested worktree).
# An explicit PAWL_PROJECT is trusted as the operator's deliberate choice (they named the project).
_pawl_verify_spawn_target() {
  [ -n "${PAWL_PROJECT:-}" ] && return 0
  local pb target
  pb="$(_pawl_projects_base)"
  target="$pb/$PROJECT"
  [ -d "$target" ] || return 1
  [ "$(_realpath_or_self "$target")" = "$(_realpath_or_self "$ROOT")" ]
}

# age-l3xj (D5): route identifier containment. A route id is interpolated into evidence and
# state paths; only [A-Za-z0-9._-], 1-64 chars, leading alphanumeric, is accepted — no '/',
# no whitespace/control chars, no flag-shaped or dotfile ids. Pure; POSIX case-pattern.
_valid_route_id() {
  case "$1" in
    ''|*[!A-Za-z0-9._-]*|[!A-Za-z0-9]*) return 1 ;;
  esac
  [ "${#1}" -le 64 ]
}

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
  return 0
}

# Strict-benched families (A7 ruling; ebec.7): excluded from the DEFAULT route probe — a
# benched family reviews only via an explicit pin (--tri/--models). NOT a rigor change:
# quorum/tier math is untouched. Override: PAWL_BENCHED_FAMILIES (space-sep; empty = none).
PAWL_BENCHED_FAMILIES="${PAWL_BENCHED_FAMILIES-agy}"

# Default-route set: the install probe minus benched families. If benching would empty the
# set (only benched CLIs installed) fall back to the raw probe — degraded beats none, and
# the no-families die stays in cmd_up.
resolve_default_families() {
  local f keep=""
  for f in $(probe_families); do case " $PAWL_BENCHED_FAMILIES " in *" $f "*) : ;; *) keep="$keep$f"$'\n' ;; esac; done
  if [ -n "$keep" ]; then printf '%s' "$keep"; else probe_families; fi
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

# Persist the resolved session so health/route/reap — from ANY repo — operate over the same set.
# SESSION-scoped shared path (age-l3xj): the layout is a session property, so `up` in repo A and
# `route` in repo B read the same file. ATOMIC (tmp + rename) so a reader never sees a partial file
# and a crash leaves the previous intact; the leaf is unlinked first in case it is a planted symlink.
_write_session_json() {
  local now tmp; now="$(_now)"; tmp="$SESSION_JSON.tmp.$$"
  mkdir -p "$(dirname "$SESSION_JSON")" 2>/dev/null || true
  _pawl_unlink_if_symlink "$SESSION_JSON"
  printf '{"session":"%s","families":"%s","tier":"%s","cc_pane":"%s","cod_pane":"%s","agy_pane":"%s","ready":true,"up_ts":%s,"last_route_ts":%s}\n' \
    "$SESSION" "$ENABLED" "$TIER" "$CC_PANE" "$COD_PANE" "$AGY_PANE" "$now" "$now" > "$tmp" \
    && mv -f "$tmp" "$SESSION_JSON"
}

# Load families/tier/panes from the session-scoped session.json. A missing file (no `up` yet, from
# any repo) defaults to the full 3-family layout (back-compat). Cross-repo route to an existing
# session reads the SAME file `up` wrote, so the real layout is preserved (refuter round 21).
load_session() {
  local fams=""
  if _session_json_matches; then
    # Tolerate optional whitespace after the colon (age-nomq); `|| true` so a truncated file falls
    # back to the default (age-l3xj D5) instead of killing the command under set -e.
    fams="$(grep -oE '"families": *"[^"]*"' "$SESSION_JSON" 2>/dev/null | sed -E 's/.*: *"([^"]*)".*/\1/' || true)"
  fi
  if [ -n "$fams" ]; then _set_panes_from_enabled $fams; else _set_panes_from_enabled cc cod agy; fi
}

# _session_json_matches: true iff the session-scoped session.json is present AND its stored session
# equals $SESSION. The filename is already the session key (injective slug), so this is a defensive
# corruption/legacy check — never loads another session's metadata.
_session_json_matches() {
  local stored
  [ -f "$SESSION_JSON" ] || return 1
  stored="$(grep -oE '"session": *"[^"]*"' "$SESSION_JSON" 2>/dev/null | sed -E 's/.*: *"([^"]*)".*/\1/' || true)"
  [ "$stored" = "$SESSION" ]
}

# Idle seconds since the last route (-1 if no session.json, wrong session, or no timestamp).
_session_idle() {
  local last
  _session_json_matches || { echo -1; return 0; }
  last="$(grep -oE '"last_route_ts": *[0-9]+' "$SESSION_JSON" 2>/dev/null | grep -oE '[0-9]+' | tail -1 || true)"
  [ -n "$last" ] || { echo -1; return 0; }
  echo $(( $(_now) - last ))
}

# Reset the idle clock (best-effort; never affects the verdict). No-op unless the file is OURS —
# never rewrite another session's state.
_touch_route_ts() {
  local sj="$SESSION_JSON"
  _session_json_matches || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  # -I (isolated): drop cwd from sys.path so a repo-planted json.py is never imported (RCE guard on
  # the untrusted-repo path) and ignore PYTHONPATH/user-site injection.
  python3 -I - "$sj" "$(_now)" <<'PY' 2>/dev/null || true
import json,os,sys
sj,now=sys.argv[1],int(sys.argv[2])
try:
    d=json.load(open(sj)); d["last_route_ts"]=now
    # COMPACT separators (age-nomq): match _write_session_json's printf format exactly. Python's
    # default json.dump emits "key": "val" (space after colon), which the grep readers in
    # load_session / _session_idle (compact "key":"val") then fail to parse — silently reverting
    # the session to the 3-family default + a permanently-stale idle clock after the first route.
    # ATOMIC (age-l3xj D5): tmp + os.replace so an interrupted rewrite never leaves a partial file.
    tmp=sj+".tmp."+str(os.getpid())
    with open(tmp,"w") as f:
        json.dump(d,f,separators=(",",":"))
    os.replace(tmp,sj)
except Exception:
    pass
PY
}

session_exists() { tmux has-session -t "$SESSION" 2>/dev/null; }

codex_state() {
  atm codex preflight --session "$SESSION" --pane "$COD_PANE" --json 2>/dev/null \
    | grep '"state"' | head -1 | sed -E 's/.*"state": *"([^"]*)".*/\1/'
}

_pane_live_text() {
  local pane="$1" lines="${2:-14}"
  tmux capture-pane -p -t "${SESSION}.${pane}" 2>/dev/null | sed '/^[[:space:]]*$/d' | tail -n "$lines"
}

cc_ready() {
  # claude pane is route-ready when its input box is present (the ❯ prompt line)
  local pane_txt
  pane_txt="$(_pane_live_text "$CC_PANE")" || return 1
  [ -z "$(detect_blocking_prompt "$pane_txt")" ] || return 1
  printf '%s\n' "$pane_txt" | grep -qE '❯|Try "'
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
  if printf '%s' "$t" | grep -qiE "trust this folder|trust this directory|trust the contents of this directory|requires permission to read"; then
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
  # age-l3xj (D1): under `ao --dry-run` even prompt-clearing key sends are a pane mutation —
  # the Go wrapper exports PAWL_DRY_RUN=1 on the read-only inspect path; report "not cleared".
  [ "${PAWL_DRY_RUN:-0}" = "1" ] && return 1
  # BOTTOM-ANCHOR (cross-family review catch): an interactive prompt is the ACTIVE bottom UI of the
  # pane; the diff/packet a reviewer is reading sits in the scrollback BODY above it. Inspecting the
  # whole capture let a reviewed diff that merely CONTAINS "press enter to continue" / "[0] Skip" /
  # "trust this folder" trigger a key-injection into a working reviewer pane (a lost-verdict fail-open).
  # Only the last few lines (the live prompt region) are inspected. The route also gates this on STALL
  # (it only calls clear on a pane producing NO new output), so keys can never hit a producing pane.
  txt="$(_pane_live_text "$pane")" || return 1
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

# age-55qz.10: PURE — a pane has blown its ABSOLUTE engagement deadline once the route has waited
# PAWL_ENGAGE_DEADLINE seconds and it still has no verdict. Unlike _stall_over_budget (which needs
# OUTPUT to go quiet), this fires on wall-clock alone, so it catches a pane that keeps re-rendering
# (compacting opus) yet never produces a verdict. Arg1=seconds waited. 0 (default) disables.
_engage_over_deadline() { [ "${PAWL_ENGAGE_DEADLINE:-0}" -gt 0 ] && [ "${1:-0}" -ge "${PAWL_ENGAGE_DEADLINE}" ]; }

# resolve_engage_deadline <metrics-file>: echo the EFFECTIVE per-pane engage-deadline
# (verification-surface-honesty S3). Pure over its inputs (metrics file + env) — locked by
# tests/scripts/pawl-engage-deadline.bats.
#   - PAWL_ENGAGE_DEADLINE_EXPLICIT=1 (operator set the env var) -> the override wins as-is;
#   - else max(static default, measured p95 over the recorded route latencies — the same
#     formula as cmd_metrics), capped at ROUTE_TIMEOUT: the hard ceiling, so low-n or
#     timeout-truncated samples can never ratchet the deadline past the route's own budget;
#   - missing/empty/unparseable metrics -> the static default unchanged (corrupt lines skip).
resolve_engage_deadline() {
  local mf="${1:-}"
  if [ "${PAWL_ENGAGE_DEADLINE_EXPLICIT:-}" = "1" ]; then echo "$PAWL_ENGAGE_DEADLINE"; return 0; fi
  local eff="${PAWL_ENGAGE_DEADLINE:-240}" ceil="${ROUTE_TIMEOUT:-320}" p95=""
  if [ -n "$mf" ] && [ -s "$mf" ]; then
    # -I (isolated): drop cwd from sys.path so a repo-planted json.py is never imported (RCE guard
    # on the untrusted-repo path — this runs with the caller's repo as cwd).
    p95="$(python3 -I - "$mf" <<'PY' 2>/dev/null || true
import json,sys
lat=[]
for l in open(sys.argv[1]):
    l=l.strip()
    if not l: continue
    try: lat.append(int(json.loads(l).get("latency_s",0)))
    except Exception: continue
lat.sort()
print(lat[min(len(lat)-1,int(round(0.95*(len(lat)-1))))] if lat else "")
PY
)"
  fi
  case "$p95" in
    ""|*[!0-9]*) : ;;
    *) if [ "$p95" -gt "$eff" ]; then eff="$p95"; fi ;;
  esac
  if [ "$eff" -gt "$ceil" ]; then eff="$ceil"; fi
  echo "$eff"
}

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
  pane_txt="$(_pane_live_text "$AGY_PANE")" || return 1
  [ -z "$(detect_blocking_prompt "$pane_txt")" ]
}

# age-55qz.11-followup (agy+cc engagement reliability): sends are DELIVERY-based. The original .11
# gated cc/agy on `atm wait --until=generating`, but that primitive does NOT reliably detect a pane
# actually generating — verified live 2026-07-01: both the opus pane ("ack") and the agy pane
# ("pong") responded to a trivial task while `atm wait` TIMED OUT the whole window, and `--type
# gemini` does not even match the Antigravity pane. So we do NOT gate on atm-wait. Instead we verify
# engagement the RELIABLE, type-agnostic way: after a delivery, the pane must actually START PRODUCING
# OUTPUT (its recent-scrollback cksum changes) within a short window — a real review shows a spinner /
# text immediately. Not-delivered -> respawn + re-send. A genuinely stuck/compacting pane still
# degrades downstream via the route's stall-detection + the .10 engage-deadline (age-yvrp).
#
# age-9rmh (2026-07-03): deliver the packet by its file PATH + a "read it now" instruction (mirroring
# cod_send), NOT by PASTING the content via `atm send --file`. On large diffs (~14KB+) the Antigravity
# (agy) TUI intermittently DROPS a pasted packet (`"delivered":1` but an empty pane, no review), and a
# re-send just re-pastes + re-drops. A tiny path-pointer message the pane reads itself is immune to the
# paste-size limit, so it is safe at EVERY diff size — hence ONE uniform path-based delivery for both cc
# and agy, no size branch. The packet file ($rp) is the SAME self-contained artifact codex reads.
_pane_activity() { tmux capture-pane -p -t "${SESSION}.$1" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true; }
_family_send() {   # $1=pane $2=cc|agy (respawn kind) $3=packet-file
  local pane="$1" kind="$2" rp="$3" try out before _
  for try in 1 2 3; do
    before="$(_pane_activity "$pane")"
    # age-9rmh: PATH + read-now instruction (see header) — never a `--file` paste that the agy TUI drops.
    out="$(atm send "$SESSION" --pane="$pane" --no-cass-check --force-non-interactive --json \
      "Follow the adversarial review instructions in the file $rp and obey its final VERDICT FORMAT line. Read the file now." \
      2>/dev/null || true)"
    if printf '%s' "$out" | grep -q '"delivered":1'; then
      # engaged iff the pane began producing output (started reviewing) within a short window
      for _ in $(seq 1 "${PAWL_SEND_ENGAGE_POLLS:-6}"); do
        sleep "${PAWL_SEND_ENGAGE_TICK:-3}"
        [ "$(_pane_activity "$pane")" != "$before" ] && return 0
      done
      log "$kind send try $try: delivered but pane produced no output (input likely dropped) — re-send"
    else
      log "$kind send try $try: not delivered — respawn + re-send"
      respawn_pane "$pane" "$kind" || true
    fi
  done
  return 1
}
agy_send() { _family_send "$AGY_PANE" agy "$1"; }
cc_send()  { _family_send "$CC_PANE"  cc  "$1"; }

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

# age-yvrp: echo the subset of ENABLED families whose pane is INDIVIDUALLY route-ready. cmd_up uses
# this to DEGRADE a boot where not every enabled family came up (e.g. AGY down in a tri-spawn) to the
# ready subset — reusing the tier door (tier_of, via _set_panes_from_enabled) for sufficiency —
# instead of dying. Never fabricates readiness (each family runs its real fail-closed ready check).
_ready_subset() {
  local out="" f cs
  for f in $ENABLED; do
    case "$f" in
      cc)  clear_known_prompts "$CC_PANE" >/dev/null 2>&1 || true; cc_ready && out="$out cc" ;;
      cod) clear_known_prompts "$COD_PANE" >/dev/null 2>&1 || true; cs="$(codex_state || true)"; { [ "$cs" = "codex-live" ] || [ "$cs" = "goal-completed" ]; } && out="$out cod" ;;
      agy) clear_known_prompts "$AGY_PANE" >/dev/null 2>&1 || true; agy_ready && out="$out agy" ;;
    esac
  done
  printf '%s' "${out# }"
}

# ── Route lease (age-yvrp, hardened age-l3xj D5) ─────────────────────────────────────────
# The lease is an ATOMIC EXCLUSIVE primitive: `mkdir $ROUTE_LOCK` succeeds for exactly one
# caller (atomic on macOS + Linux), replacing the old check-then-write timestamp file that
# let two concurrent routes both "hold" the lock. Owner metadata (pid/started/session) lives
# in $ROUTE_LOCK/owner. Freshness window: ROUTE_TIMEOUT + 60s slack; a stale lease (crashed
# route) is reclaimed ONCE per acquire attempt — bounded recovery, never a permanent wedge.

# _route_lock_mtime echoes a directory's mtime epoch (0 if unavailable). Portable across
# macOS (stat -f %m) and Linux (stat -c %Y).
_route_lock_mtime() {
  local m
  m="$(stat -f %m "$1" 2>/dev/null || stat -c %Y "$1" 2>/dev/null || echo 0)"
  case "$m" in ''|*[!0-9]*) m=0 ;; esac
  printf '%s' "$m"
}

# _route_lock_started echoes the lease's start epoch (0 if unreadable). Handles the lock-DIR
# format ($ROUTE_LOCK/owner JSON) and the legacy timestamp-FILE format.
#
# A lock DIR whose owner file is MISSING or unparseable falls back to the DIRECTORY's OWN
# mtime — set atomically by mkdir (refuter catch). Reading it as epoch 0 (=> instantly stale)
# opened a publication race: a contender arriving in the window between another process's
# mkdir and its owner write would declare the fresh lease stale, reclaim it, and both would
# hold the lease. The directory's mtime exists from the instant the lease does.
_route_lock_started() {
  local started=""
  if [ -d "$ROUTE_LOCK" ]; then
    started="$(grep -oE '"started": *[0-9]+' "$ROUTE_LOCK/owner" 2>/dev/null | grep -oE '[0-9]+' | tail -1 || true)"
    case "$started" in ''|*[!0-9]*) started="$(_route_lock_mtime "$ROUTE_LOCK")" ;; esac
  elif [ -f "$ROUTE_LOCK" ]; then
    started="$(cat "$ROUTE_LOCK" 2>/dev/null || echo 0)"
  fi
  case "$started" in ''|*[!0-9]*) started=0 ;; esac
  printf '%s' "$started"
}

# _route_lock_window is the single freshness/heartbeat window (seconds): 2*ROUTE_TIMEOUT, FLOORED to
# 2 so a degenerate ROUTE_TIMEOUT (the supported PAWL_ROUTE_TIMEOUT=0 "don't poll" tunable) can't
# collapse it to 0 — which made `age < 0` impossible and marked every LIVE lease instantly stale
# (refuter catch). Shared by _route_lock_fresh AND _route_heartbeat_interval so the freshness bound
# and the heartbeat cadence can never disagree.
_route_lock_window() {
  local w=$(( ROUTE_TIMEOUT * 2 ))
  [ "$w" -lt 2 ] && w=2
  printf '%s' "$w"
}

# _route_lock_fresh: true iff a lease exists and was started within the window a real route could
# still be running (2*ROUTE_TIMEOUT, floored). The send/respawn phase runs BEFORE the first poll
# tick, so a too-tight window let a live lease go stale before its first heartbeat and be reclaimed
# mid-run; the BACKGROUND heartbeat refreshes within this window, so it only bounds CRASHED-route
# recovery, not a live route's real duration.
_route_lock_fresh() {
  [ -e "$ROUTE_LOCK" ] || return 1
  local started age
  started="$(_route_lock_started)"
  age=$(( $(date +%s) - started ))
  [ "$age" -ge 0 ] && [ "$age" -lt "$(_route_lock_window)" ]
}

# _route_lock_is_lease: the path is safe to treat (and remove) as OUR lease. PAWL_ROUTE_LOCK is
# an operator override, so an arbitrary directory can be pointed at it — a lease directory holds
# NOTHING but an optional `owner` file. Anything else is refused rather than deleted (refuter
# catch: the earlier code recursively deleted the unvalidated override path — a direct
# data-loss route whenever it named an existing directory without a fresh owner).
_route_lock_is_lease() {
  # A SYMLINK is NEVER our lease: we only ever create a real dir via mkdir, and `[ -d ]` /
  # mv / rmdir all FOLLOW a symlink — so a PAWL_ROUTE_LOCK pointed at (or a planted) symlink to a
  # populated dir would otherwise be "reclaimed", deleting the TARGET's owner (data loss). Refuse it.
  [ -L "$ROUTE_LOCK" ] && return 1
  if [ -d "$ROUTE_LOCK" ]; then
    local extra
    extra="$(find "$ROUTE_LOCK" -mindepth 1 -maxdepth 1 ! -name owner -print 2>/dev/null | head -1)"
    [ -z "$extra" ]
    return
  fi
  if [ -f "$ROUTE_LOCK" ]; then
    # A LEGACY lock is a file holding ONLY a numeric epoch (what old `date +%s > lock` wrote). A
    # regular file with any other content (e.g. an operator's /etc/hosts pointed at by
    # PAWL_ROUTE_LOCK) is NOT our lock — refuse it, so stale reclaim never rm's an unrelated file.
    local content; content="$(cat "$ROUTE_LOCK" 2>/dev/null)"
    case "$content" in
      ''|*[!0-9[:space:]]*) return 1 ;;   # empty or non-numeric => not a legacy lock
    esac
    return 0
  fi
  return 0   # absent: nothing to over-delete
}

# _route_lock_break_stale removes a STALE lease of generation $1 (its start epoch), serialized by
# a GENERATION-SCOPED break-token so exactly one breaker acts per stale generation. It removes the
# lease ONLY after re-confirming, while holding the token, that the lease is STILL that same stale
# generation — so a peer's FRESH lease (a different generation) is NEVER touched.
#
# Refuter round 3+5 catch: an unconditional "rename whatever is at ROUTE_LOCK aside" is NOT
# single-winner. Interleaving: A and B both see the stale lease; A fully reclaims + recreates a
# FRESH lease Y; B, still holding the OLD generation it captured, then renames Y aside (its rename
# never re-checked that the lease was still stale) and creates its own — A and B both "hold" it.
# The generation token closes this: B can only break generation T while holding ${ROUTE_LOCK}.break.T;
# A cannot recreate over a lease it did not itself break (it too needs the token); and B's post-token
# re-read of the generation shows Y's NEW epoch (!= T), so B refuses to touch Y and fails closed.
# Wedge-free: a crashed breaker's token ages out (older than any break can take) and is reclaimed.
_route_lock_break_stale() {
  local gen="$1"
  local btok="${ROUTE_LOCK}.break.${gen}"
  if ! mkdir "$btok" 2>/dev/null; then
    local bt; bt="$(_route_lock_mtime "$btok")"
    if [ "$bt" -gt 0 ] && [ "$(( $(date +%s) - bt ))" -lt 60 ]; then
      return 1                       # a LIVE breaker owns this generation — do not interfere
    fi
    rmdir "$btok" 2>/dev/null || return 1   # crashed breaker's token — reclaim it, then retry once
    mkdir "$btok" 2>/dev/null || return 1
  fi
  # We hold the break-token for THIS generation. Remove the lease ONLY if it is STILL that stale
  # generation (a peer may already have broken + recreated a fresh one of a different generation).
  if [ "$(_route_lock_started)" = "$gen" ] && ! _route_lock_fresh; then
    if [ -d "$ROUTE_LOCK" ]; then
      if _route_lock_is_lease; then
        rm -f "$ROUTE_LOCK/owner" 2>/dev/null || true
        rmdir "$ROUTE_LOCK" 2>/dev/null || true
      else
        rmdir "$btok" 2>/dev/null || true
        die "refusing to reclaim $ROUTE_LOCK — it is not a pawl lease directory (contains entries other than 'owner'). Point PAWL_ROUTE_LOCK at a dedicated path."
      fi
    elif [ -f "$ROUTE_LOCK" ]; then
      # Only remove a file that is a genuine legacy lock (numeric epoch content) — never an
      # unrelated regular file an operator pointed PAWL_ROUTE_LOCK at (e.g. /etc/hosts).
      if _route_lock_is_lease; then
        rm -f "$ROUTE_LOCK" 2>/dev/null || true
      else
        rmdir "$btok" 2>/dev/null || true
        die "refusing to reclaim $ROUTE_LOCK — it is not a pawl lease (a regular file with non-lock content). Point PAWL_ROUTE_LOCK at a dedicated path."
      fi
    fi
  fi
  rmdir "$btok" 2>/dev/null || true
  return 0
}

# _route_lock_owner_pid echoes the pid recorded in the lease owner metadata (empty if none).
_route_lock_owner_pid() {
  [ -d "$ROUTE_LOCK" ] || return 0
  grep -oE '"pid": *[0-9]+' "$ROUTE_LOCK/owner" 2>/dev/null | grep -oE '[0-9]+' | tail -1 || true
}

# _route_lock_owned_by_me: true iff the lease exists and its owner pid is THIS process.
_route_lock_owned_by_me() {
  [ -d "$ROUTE_LOCK" ] || return 1
  [ "$(_route_lock_owner_pid)" = "$$" ]
}

# _route_lock_publish writes the owner metadata into a lease we already own — ATOMICALLY
# (write a sibling temp, rename in). A truncate-in-place `> owner` would let a concurrent
# freshness reader see a half-written owner and fall back to the dir's (old) mtime — reading a
# LIVE, heartbeating lease as stale. The temp is a SIBLING of the lease dir (not inside it) so
# _route_lock_is_lease never trips on it. Also bumps `started` to now — this is the heartbeat.
_route_lock_publish() {
  local tmp="${ROUTE_LOCK}.owner.$$"
  printf '{"pid":%s,"started":%s,"session":"%s"}\n' "$$" "$(date +%s)" "$SESSION" > "$tmp" 2>/dev/null \
    && mv -f "$tmp" "$ROUTE_LOCK/owner" 2>/dev/null || rm -f "$tmp" 2>/dev/null || true
}

# _route_lock_touch refreshes OUR lease once (the heartbeat unit). ALWAYS returns 0 so it is safe
# as a bare statement under `set -e` — a non-zero return (we no longer own the lease) must not
# abort the caller. Only refreshes a lease we still own (a successor's lease is never touched).
_route_lock_touch() {
  if _route_lock_owned_by_me; then _route_lock_publish; fi
  return 0
}

# _route_heartbeat_interval DERIVES the heartbeat interval from the freshness window so the two can
# never drift apart (refuter catch: a fixed 20s heartbeat with PAWL_ROUTE_TIMEOUT=2 left the 4s
# lease expiring BETWEEN beats — a live owner's lease went stale and a successor reclaimed it).
# window = 2*ROUTE_TIMEOUT; default interval = window/4 (>=4 beats per window), capped at 20s so a
# huge window doesn't drift too long. An env override (PAWL_HEARTBEAT_INTERVAL) is HARD-CLAMPED to
# at most window/2, so NO configuration can break the interval < window invariant. Floors at 1s.
_route_heartbeat_interval() {
  local window; window="$(_route_lock_window)"
  local cap=$(( window / 2 )); [ "$cap" -lt 1 ] && cap=1
  local iv
  if [ -n "${PAWL_HEARTBEAT_INTERVAL:-}" ] && printf '%s' "$PAWL_HEARTBEAT_INTERVAL" | grep -qE '^[0-9]+$' && [ "$PAWL_HEARTBEAT_INTERVAL" -ge 1 ]; then
    iv="$PAWL_HEARTBEAT_INTERVAL"
  else
    iv=$(( window / 4 )); [ "$iv" -lt 1 ] && iv=1
    [ "$iv" -gt 20 ] && iv=20
  fi
  [ "$iv" -gt "$cap" ] && iv="$cap"      # hard invariant: >= 2 beats within the freshness window
  printf '%s' "$iv"
}

# _route_lock_heartbeat_bg is the BACKGROUND heartbeat, tied to the route PROCESS lifetime. It
# refreshes our lease every $interval for the WHOLE route — the pre-poll send/respawn phase AND the
# poll loop — closing the gap a poll-loop-only heartbeat left (default send+respawn can run ~700s+
# before the first poll tick, longer than any static TTL; widening the window is whack-a-mole). It
# self-terminates the instant the route process ($1) is gone (kill -0 fails), so a CRASHED route —
# however it died — stops heartbeating and its lease goes stale + reclaimable; an orphaned heartbeat
# that kept a dead route's lease alive forever would be the worse bug.
_route_lock_heartbeat_bg() {
  local parent="$1" interval="$2"
  while kill -0 "$parent" 2>/dev/null; do
    sleep "$interval"
    kill -0 "$parent" 2>/dev/null || break     # route process gone → stop (lease may age out)
    _route_lock_owned_by_me || break           # lease lost/released → stop
    _route_lock_publish
  done
}

# _route_lock_heartbeat_start forks the background heartbeat for the current process and records
# its pid in _ROUTE_HEARTBEAT_PID. _route_lock_heartbeat_stop kills it (idempotent).
_route_lock_heartbeat_start() {
  _route_lock_heartbeat_bg "$$" "$(_route_heartbeat_interval)" &
  _ROUTE_HEARTBEAT_PID=$!
}
_route_lock_heartbeat_stop() {
  [ -n "${_ROUTE_HEARTBEAT_PID:-}" ] || return 0
  kill "$_ROUTE_HEARTBEAT_PID" 2>/dev/null || true
  # WAIT for the heartbeat to actually die BEFORE the caller releases the lease (refuter catch:
  # the heartbeat's ownership-check and owner-write are not atomic — a delayed heartbeat could pass
  # its check, then after release + successor-acquire overwrite the SUCCESSOR's owner with our PID,
  # dispossessing a live successor). `wait` guarantees no in-flight heartbeat write survives; any
  # last write it did land in OUR lease dir, which _route_lock_release then removes, so the
  # successor's fresh dir is never touched. The RETURN trap runs stop (kill+wait) THEN release.
  wait "$_ROUTE_HEARTBEAT_PID" 2>/dev/null || true
  _ROUTE_HEARTBEAT_PID=""
}

# _route_lock_acquire: mkdir is the SOLE ownership barrier (atomic, single-winner). A FRESH lease
# held by anyone fails closed. A STALE lease is removed via _route_lock_break_stale (generation
# token: one breaker per generation, fresh leases never touched), then the slot is re-raced — and
# whoever wins the mkdir owns it; a caller that loses fails closed and NEVER assumes ownership.
_route_lock_acquire() {
  mkdir -p "$(dirname "$ROUTE_LOCK")" 2>/dev/null || true
  if mkdir "$ROUTE_LOCK" 2>/dev/null; then
    _route_lock_publish
    return 0
  fi
  _route_lock_fresh && return 1              # held & fresh: fail closed, never break
  local gen; gen="$(_route_lock_started)"    # the stale generation to break
  _route_lock_break_stale "$gen" || true     # best-effort; another breaker may act, or it aged out
  if mkdir "$ROUTE_LOCK" 2>/dev/null; then   # the ONE ownership barrier — exactly one winner
    _route_lock_publish
    return 0
  fi
  return 1                                   # someone else holds/took the slot — fail closed
}

# _route_lock_release removes ONLY a lease THIS process still owns. Refuter catch: the RETURN
# trap released unconditionally, so a long route whose lease had gone stale and been reclaimed by
# a SUCCESSOR would, on exit, delete the successor's live lease — concurrent routes. The owner-pid
# check makes release a no-op once we no longer own the lease; combined with the heartbeat (which
# stops a live route's lease going stale in the first place), the lease is exclusive for real.
_route_lock_release() {
  if [ -d "$ROUTE_LOCK" ]; then
    _route_lock_is_lease || return 0         # never recursively delete a non-lease path
    _route_lock_owned_by_me || return 0      # a successor owns it now — do not dispossess them
    rm -f "$ROUTE_LOCK/owner" 2>/dev/null || true
    rmdir "$ROUTE_LOCK" 2>/dev/null || true
  fi
  # A legacy timestamp-FILE lock carries no owner; we never create one, so at release time a file
  # here is not ours to remove — leave it (a stale one ages out via _route_lock_break_stale).
}

# age-yvrp: true iff a route is IN PROGRESS — a FRESH lease exists. A STALE lease (crashed
# route) is broken + treated as not-in-progress so it can never wedge teardown forever.
_route_in_progress() {
  [ -e "$ROUTE_LOCK" ] || return 1
  if _route_lock_fresh; then return 0; fi
  _route_lock_break_stale "$(_route_lock_started)" || true
  return 1
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
    enabled="$(resolve_default_families | tr '\n' ' ')"
    [ "$enabled" = "$probe" ] || log "default route excludes benched families [$PAWL_BENCHED_FAMILIES] — opt in via --models/--tri"
  fi
  set -- $enabled
  [ "$#" -ge 1 ] || die "no pawl families installed — need at least one of: claude, codex, agy"
  _set_panes_from_enabled "$@"

  # Build the atm spawn flags from the enabled set (canonical order: cc, then cod, then agy).
  local -a spawn_flags=(--no-user)
  case " $ENABLED " in *" cc "*) spawn_flags+=(--cc=1:"$PAWL_CLAUDE_MODEL") ;; esac
  case " $ENABLED " in *" cod "*) if [ -n "$PAWL_CODEX_MODEL" ]; then spawn_flags+=(--cod=1:"$PAWL_CODEX_MODEL"); else spawn_flags+=(--cod=1); fi ;; esac
  case " $ENABLED " in *" agy "*) spawn_flags+=(--agy=1) ;; esac

  if session_exists; then
    log "session $SESSION already exists — gating readiness (idempotent up)"
  else
    # age-l3xj (cross-family refuter catch): `atm spawn <project>` resolves the pane cwd as
    # projects_base/<project> (there is NO cwd flag — atm's own help: "the session name must match
    # an actual directory under projects_base"). So spawning is only correct when THIS repo is a
    # direct child of projects_base; for a nested worktree or a repo elsewhere, basename(ROOT) would
    # resolve to a DIFFERENT (or missing) directory. Verify before mutating — never spawn into the
    # wrong repo — and fail closed with an actionable message naming the requirement (the spec's
    # cross-repo fallback: work when safe, else fail before mutation, never ambiguous).
    _pawl_verify_spawn_target || die "ao pawl up: this repo ($ROOT) is not a direct child of the ATM projects_base ($(_pawl_projects_base)), so 'atm spawn $PROJECT' would target '$(_pawl_projects_base)/$PROJECT' (the wrong/missing directory). Run 'ao pawl up' from a repo directly under $(_pawl_projects_base); OR set PAWL_PROJECT to a valid atm project name; OR set PAWL_SESSION to route to an existing standing session (e.g. the operator's 'agentops--pawl-service')."
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
  # age-yvrp: the full enabled set never all came ready — DEGRADE to the subset that IS ready
  # (reuse the tier door) rather than dying, so e.g. a tri-spawn with AGY down still serves the
  # cross-family gate on cc+cod. Fail-safe: _ready_subset runs each family's real fail-closed check
  # (no fabricated readiness), and a degrade to a single family honestly records TIER=fresh (weaker;
  # a high-irreversibility door decides sufficiency). Only a fully-empty ready subset is fatal.
  local ready_subset; ready_subset="$(_ready_subset)"
  if [ -n "$ready_subset" ]; then
    log "readiness gate: not all of [$ENABLED] came ready ($(_ready_debug)) — degrading to ready subset [$ready_subset]"
    # shellcheck disable=SC2086
    _set_panes_from_enabled $ready_subset
    _write_session_json
    log "UP: ready (degraded) — families: $ENABLED, tier=$TIER ($(_tier_phrase))"
    return 0
  fi
  die "readiness gate timed out — NO enabled family became ready (families=$ENABLED; $(_ready_debug))"
}

cmd_down() {
  # age-yvrp/age-l3xj: teardown ACQUIRES the route lease itself (not check-then-kill — that
  # TOCTOU let a route start between the check and the kill). While down holds the lease no
  # route can start; while a route holds it, down fails closed (exit 3). --force overrides;
  # a STALE lease (crashed route) never blocks (acquire reclaims it, bounded).
  local force=0; [ "${1:-}" = "--force" ] && force=1
  if [ "$force" -eq 0 ]; then
    if ! _route_lock_acquire; then
      log "DOWN: refused — a route is in progress (route lease fresh). Use 'down --force' to override."
      return 3
    fi
    trap '_route_lock_release' RETURN
  fi
  if session_exists; then
    atm kill "$SESSION" --json >/dev/null 2>&1 || tmux kill-session -t "$SESSION" 2>/dev/null || true
    # Remove the session-scoped state (never follow a symlink at that leaf).
    _pawl_unlink_if_symlink "$SESSION_JSON"; rm -f "$SESSION_JSON" 2>/dev/null || true
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
      _ct="$(_pane_live_text "$COD_PANE" 14 2>/dev/null || true)"
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

# --- Read-only standing-pawl doctor / smoke preflight ---

_json_escape() {
  printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e ':a' -e 'N' -e '$!ba' -e 's/\n/\\n/g'
}

_realpath_or_self() {
  realpath "$1" 2>/dev/null || printf '%s' "$1"
}

_pane_current_path() {
  tmux display-message -p -t "${SESSION}.$1" '#{pane_current_path}' 2>/dev/null
}

_pane_model_text() {
  tmux capture-pane -p -t "${SESSION}.$1" -S -80 2>/dev/null
}

_text_has_model() {
  local text="$1" family="$2" expected="$3"
  [ -n "$expected" ] || return 0
  case "$family:$expected" in
    cc:claude-opus-4-8)
      printf '%s\n' "$text" | grep -qiE 'claude-opus-4-8|Opus[[:space:]]*4\.8'
      ;;
    *)
      printf '%s\n' "$text" | grep -qiF -- "$expected"
      ;;
  esac
}

_standing_evidence_marker_policy_ready() {
  local vf="$PAWL_VERDICT_SH"
  [ -f "$vf" ] || return 1
  grep -qF '_standing_pawl_context' "$vf" || return 1
  case " $ENABLED " in
    *" cod "*) grep -qF 'gpt:codex-pawl-pane-*' "$vf" || return 1 ;;
  esac
  case " $ENABLED " in
    *" agy "*) grep -qF 'gemini:agy-pawl-pane-*' "$vf" || return 1 ;;
  esac
  return 0
}

DOCTOR_NAMES=()
DOCTOR_OKS=()
DOCTOR_DETAILS=()

_doctor_reset() { DOCTOR_NAMES=(); DOCTOR_OKS=(); DOCTOR_DETAILS=(); }
_doctor_add() {
  DOCTOR_NAMES+=("$1")
  DOCTOR_OKS+=("$2")
  DOCTOR_DETAILS+=("$3")
}
_doctor_fail_count() {
  local n=0 ok
  for ok in "${DOCTOR_OKS[@]}"; do [ "$ok" = "true" ] || n=$((n + 1)); done
  echo "$n"
}
_doctor_emit() {
  local json="$1" fails="$2" i sep="" status="pass"
  [ "$fails" -eq 0 ] || status="fail"
  if [ "$json" -eq 1 ]; then
    printf '{"status":"%s","checks":[' "$status"
    for i in "${!DOCTOR_NAMES[@]}"; do
      printf '%s{"name":"%s","ok":%s,"detail":"%s"}' \
        "$sep" "$(_json_escape "${DOCTOR_NAMES[$i]}")" "${DOCTOR_OKS[$i]}" "$(_json_escape "${DOCTOR_DETAILS[$i]}")"
      sep=","
    done
    printf ']}\n'
  else
    echo "pawl doctor: $(printf '%s' "$status" | tr '[:lower:]' '[:upper:]')"
    for i in "${!DOCTOR_NAMES[@]}"; do
      if [ "${DOCTOR_OKS[$i]}" = "true" ]; then
        printf '  %-28s PASS  %s\n' "${DOCTOR_NAMES[$i]}" "${DOCTOR_DETAILS[$i]}"
      else
        printf '  %-28s FAIL  %s\n' "${DOCTOR_NAMES[$i]}" "${DOCTOR_DETAILS[$i]}"
      fi
    done
  fi
  [ "$fails" -eq 0 ]
}

cmd_doctor() {
  load_session
  local json=0
  local expected_cwd="${PAWL_EXPECT_CWD:-$ROOT}"
  local expect_cc="${PAWL_EXPECT_CLAUDE_MODEL:-$PAWL_CLAUDE_MODEL}"
  local expect_cod="${PAWL_EXPECT_CODEX_MODEL:-${PAWL_CODEX_MODEL:-gpt-5.5}}"
  local expect_agy="${PAWL_EXPECT_AGY_MODEL:-Gemini}"
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --json) json=1 ;;
      --expected-cwd) shift; expected_cwd="${1:-}" ;;
      --expected-cwd=*) expected_cwd="${1#--expected-cwd=}" ;;
      --expected-claude-model) shift; expect_cc="${1:-}" ;;
      --expected-claude-model=*) expect_cc="${1#--expected-claude-model=}" ;;
      --expected-codex-model) shift; expect_cod="${1:-}" ;;
      --expected-codex-model=*) expect_cod="${1#--expected-codex-model=}" ;;
      --expected-agy-model) shift; expect_agy="${1:-}" ;;
      --expected-agy-model=*) expect_agy="${1#--expected-agy-model=}" ;;
      *) die "doctor: unknown flag $1" ;;
    esac
    shift || true
  done

  _doctor_reset

  local atm_path ntm_path atm_real ntm_real
  atm_path="$(command -v atm 2>/dev/null || true)"
  ntm_path="$(command -v ntm 2>/dev/null || true)"
  if [ -z "$atm_path" ]; then
    _doctor_add atm-alias false "atm not found on PATH"
  elif [ -n "$ntm_path" ]; then
    atm_real="$(_realpath_or_self "$atm_path")"; ntm_real="$(_realpath_or_self "$ntm_path")"
    if [ "$atm_real" = "$ntm_real" ]; then
      _doctor_add atm-alias true "atm -> $atm_real"
    else
      _doctor_add atm-alias false "atm=$atm_real differs from ntm=$ntm_real"
    fi
  else
    _doctor_add atm-alias true "atm=$atm_path (ntm not present)"
  fi

  if session_exists; then
    _doctor_add session true "$SESSION exists; families=[$ENABLED]; tier=$TIER"
  else
    _doctor_add session false "$SESSION does not exist"
  fi

  if [ -n "$ENABLED" ]; then
    _doctor_add families true "$ENABLED"
  else
    _doctor_add families false "no enabled families in session state"
  fi

  if session_exists; then
    local expected_real cur cur_real txt block state model_txt f pane label expected
    expected_real="$(_realpath_or_self "$expected_cwd")"
    for f in $ENABLED; do
      case "$f" in
        cc)  pane="$CC_PANE";  label="claude"; expected="$expect_cc" ;;
        cod) pane="$COD_PANE"; label="codex";  expected="$expect_cod" ;;
        agy) pane="$AGY_PANE"; label="agy";    expected="$expect_agy" ;;
        *) continue ;;
      esac

      cur="$(_pane_current_path "$pane" || true)"
      cur_real="$(_realpath_or_self "$cur")"
      if [ -n "$cur" ] && [ "$cur_real" = "$expected_real" ]; then
        _doctor_add "$label-cwd" true "$cur"
      else
        _doctor_add "$label-cwd" false "got ${cur:-<unreadable>}; expected $expected_cwd"
      fi

      txt="$(_pane_live_text "$pane" 14 2>/dev/null || true)"
      block="$(detect_blocking_prompt "$txt")"
      if [ -z "$block" ]; then
        _doctor_add "$label-trust-prompt" true "no known blocking prompt"
      else
        _doctor_add "$label-trust-prompt" false "blocked on $block"
      fi

      case "$f" in
        cc)
          if cc_ready; then
            _doctor_add "$label-ready" true "input prompt present"
          else
            _doctor_add "$label-ready" false "claude pane not route-ready"
          fi
          ;;
        cod)
          state="$(codex_state || echo absent)"
          case "$state" in
            codex-live|goal-completed) _doctor_add "$label-ready" true "$state" ;;
            *) _doctor_add "$label-ready" false "$state" ;;
          esac
          ;;
        agy)
          if agy_ready; then
            _doctor_add "$label-ready" true "agy foreground and unblocked"
          else
            _doctor_add "$label-ready" false "agy pane not route-ready"
          fi
          ;;
      esac

      model_txt="$(_pane_model_text "$pane" 2>/dev/null || true)"
      if _text_has_model "$model_txt" "$f" "$expected"; then
        _doctor_add "$label-model" true "expected $expected"
      else
        _doctor_add "$label-model" false "expected $expected not visible in pane"
      fi
    done
  fi

  if _standing_evidence_marker_policy_ready; then
    _doctor_add evidence-marker-policy true "standing pawl-pane contexts do not require cold-adapter footers"
  else
    _doctor_add evidence-marker-policy false "standing pane evidence policy is not ready"
  fi

  local fails; fails="$(_doctor_fail_count)"
  _doctor_emit "$json" "$fails"
}

cmd_smoke() { cmd_doctor "$@"; }

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

# route_outcome <decision> <session-lost 0|1>: map pawl_decide's pure quorum decision plus
# the standing session's availability onto the OUTCOME the route records and binds
# (verification-surface-honesty S3). The quorum itself is UNTOUCHED — what counts as
# CONFIRMED is identical. The honesty split: a REFUTED bind is a DEFECT CLAIM, so only a
# pane that actually voted REFUTED may produce one; a no-substantive-refuter degrade binds
# HOLD (schema-valid; fail-closed at every door — pawl-verdict.sh check authorizes only
# CONFIRMED, so nothing that blocked before passes now). Pure — locked by
# tests/scripts/pawl-engage-deadline.bats.
#   CONFIRMED:<detail>:N                  -> CONFIRMED:<detail>  (a met quorum survives session loss)
#   REFUTED:refuted:N                     -> REFUTED:refuted     (>=1 real refutation, even on loss)
#   REFUTED:insufficient:N + session up   -> HOLD:insufficient-reviewers (deadline/stall give-ups)
#   REFUTED:insufficient:N + session lost -> HOLD:service-unavailable   (panel vanished mid-route)
route_outcome() {
  local decision="$1" lost="${2:-0}"
  local disp="${decision%%:*}" detail
  detail="$(printf '%s' "$decision" | cut -d: -f2)"
  if [ "$disp" = "CONFIRMED" ]; then echo "CONFIRMED:${detail}"; return 0; fi
  if [ "$detail" = "refuted" ]; then echo "REFUTED:refuted"; return 0; fi
  if [ "$lost" = "1" ]; then echo "HOLD:service-unavailable"; else echo "HOLD:insufficient-reviewers"; fi
}

# age-pawl-good-bar #4: PURE — build the REFUTED/HOLD-path refuter list from the panes' ACTUAL
# emitted verdicts. Echoes one "family:verdict:context:evidence" spec per line for every pane that
# really voted (CONFIRMED or REFUTED). A timed-out pane (empty verdict) is OMITTED — recording it as
# REFUTED fabricates a refutation it never made. The verdict schema requires >=1 refuter, so if NO
# pane voted (all timed out -> REFUTED:insufficient) fall back to the timed-out enabled panes with a
# *-timeout context, keeping the membrane catch logged AND honest that it was a timeout, not a
# refute. Args: vc vd va ev_cc ev_cod ev_agy. ("n/a" = a disabled/unavailable pane -> never recorded.)
_refuted_refuters() {
  local vc="$1" vd="$2" va="$3" ecc="$4" ecod="$5" eagy="$6" out=""
  case "$vc" in CONFIRMED|REFUTED) out="${out}claude:${vc}:opus-pawl-pane-fresh:${ecc}"$'\n' ;; esac
  case "$vd" in CONFIRMED|REFUTED) out="${out}gpt:${vd}:codex-pawl-pane-gpt55:${ecod}"$'\n' ;; esac
  case "$va" in CONFIRMED|REFUTED) out="${out}gemini:${va}:agy-pawl-pane-flash35:${eagy}"$'\n' ;; esac
  if [ -z "$out" ]; then
    [ "$vc" != "n/a" ] && out="${out}claude:REFUTED:opus-pawl-pane-timeout:${ecc}"$'\n'
    [ "$vd" != "n/a" ] && out="${out}gpt:REFUTED:codex-pawl-pane-timeout:${ecod}"$'\n'
    [ "$va" != "n/a" ] && out="${out}gemini:REFUTED:agy-pawl-pane-timeout:${eagy}"$'\n'
  fi
  printf '%s' "$out"
}

# age-2yh2: PURE — pick the evidence capture carrying the refuting reviewer's REAL
# finding for the routed-REFUTED membrane catch: the FIRST pane (canonical cc->cod->agy
# order) that actually voted REFUTED. Echoes NOTHING when no pane refuted (an
# insufficient/all-timeout REFUTED carries no reviewer finding to salvage — the caller
# skips the catch; the verdict write still logs the route outcome). Args: vc vd va
# ev_cc ev_cod ev_agy (verdicts as in _refuted_refuters: CONFIRMED/REFUTED/""=timeout/n/a).
_refuting_evidence() {
  local vc="$1" vd="$2" va="$3" ecc="$4" ecod="$5" eagy="$6"
  [ "$vc" = "REFUTED" ] && { printf '%s' "$ecc"; return 0; }
  [ "$vd" = "REFUTED" ] && { printf '%s' "$ecod"; return 0; }
  [ "$va" = "REFUTED" ] && { printf '%s' "$eagy"; return 0; }
  return 0
}

# age-2yh2: resolve the trusted ao binary for the route path's membrane catch, preferring
# the repo's build over a possibly-stale installed ao (a stale ao lacking `membrane catch`
# would silently no-op the emit). Order: $AO_BIN, repo build ($ROOT/cli/bin/ao then
# $ROOT/cli/ao), then PATH — pawl-review.sh's resolve_ao trusted-checkout order. No
# untrusted-repo branch here: the standing service only ever runs against the trusted
# checkout it lives in ($ROOT), never a stranger repo. Prints nothing when none is
# executable (callers treat that as skip).
_route_ao_bin() {
  local c
  for c in "${AO_BIN:-}" "$ROOT/cli/bin/ao" "$ROOT/cli/ao"; do
    if [ -n "$c" ] && [ -x "$c" ]; then printf '%s\n' "$c"; return 0; fi
  done
  command -v ao 2>/dev/null
}

# age-2yh2: record a routed REFUTE as a STRUCTURED membrane catch carrying the refuting
# reviewer's REAL finding — `ao membrane catch --evidence` runs the two-tier REFUTED-reason
# salvage Go-side (age-ulab: the `REFUTED: <finding>` prose line, skipping the bare routed
# `PAWL <nonce> REFUTED` sentinel) and resolves domain + affected paths from git, so the
# catch is producer-actionable instead of the generic "standing-pawl route: …" reason the
# verdict write carries. Mirrors emit_pawl_catch's contract (pawl-review.sh): fail-safe +
# NON-BLOCKING — a missing evidence file, missing ao, or any error never affects the
# REFUTED exit (the catch is observability, not a gate). PAWL_CATCH_CLASS/PAWL_CATCH_DETECTOR
# pass through when set. Runs from $ROOT so the catch lands in the repo-root yield ledger.
# Args: <bead> <head> <mode> <evidence-file> (empty/absent evidence => silent skip).
_route_emit_catch() {
  local bead="$1" head="$2" mode="$3" evfile="$4" ao_bin
  [ -n "$evfile" ] && [ -s "$evfile" ] || return 0
  ao_bin="$(_route_ao_bin)" && [ -n "$ao_bin" ] || return 0
  local -a catch_args=()
  [ -n "${PAWL_CATCH_CLASS:-}" ]    && catch_args+=(--class "$PAWL_CATCH_CLASS")
  [ -n "${PAWL_CATCH_DETECTOR:-}" ] && catch_args+=(--detector-pattern "$PAWL_CATCH_DETECTOR")
  ( cd "$ROOT" && "$ao_bin" membrane catch --bead "$bead" \
      --evidence "$evfile" --head "$head" --mode "$mode" \
      --scope head "${catch_args[@]}" ) >/dev/null 2>&1 || true
}

cmd_route() {
  # age-l3xj (D5): identifier containment FIRST — the bead id is interpolated into
  # evidence/state paths, so traversal/separators/control chars/flag-shape/over-length
  # must be rejected BEFORE any lock or file write. Charset [A-Za-z0-9._-], 1-64 chars,
  # leading alnum (kills "-flag", ".hidden", ".."); no '/' means no path can escape
  # $EVID_DIR / $STATE_DIR.
  local bead="${1:?route needs <bead>}" packet="${2:?route needs <packet-file>}" pr="${3:-0}"
  _valid_route_id "$bead" || die "invalid bead id '$bead' — allowed: [A-Za-z0-9._-], 1-64 chars, leading alphanumeric (path/flag containment)"
  _pawl_require_safe_state_dir   # refuse a symlinked state-dir chain before writing metrics/state
  _pawl_verdict_dir_safe || die "refusing to write the route verdict: $PAWL_VERDICT_DIR (or its parent) is a symlink — verdicts must stay inside the repo. Remove the symlink, or set PAWL_VERDICT_DIR to a real in-repo path."
  load_session
  [ -f "$packet" ] || die "packet file not found: $packet"
  session_exists || die "no standing session — run 'pawl up' first"
  [ -n "$ENABLED" ] || die "session has no enabled families — re-run 'pawl up'"
  # age-yvrp/age-l3xj: take the EXCLUSIVE route lease so (a) down/reap can't tear the session
  # out from under this route and (b) a second concurrent route fails closed BEFORE sending
  # to any pane or writing evidence. The RETURN trap releases it on the normal
  # CONFIRMED/REFUTED exits; a die/crash leaves a lease that acquire/_route_in_progress
  # reclaims once it goes stale (> ROUTE_TIMEOUT+slack) — never a permanent wedge.
  _route_lock_acquire || die "another route holds the route lease ($ROUTE_LOCK) — exactly one route owns the service at a time; retry after it finishes"
  # Start the background heartbeat BEFORE the long send/respawn phase (which can run past any
  # static TTL). It refreshes our lease across every phase for as long as THIS process lives, and
  # dies with it (so a crashed route's lease still ages out). The RETURN trap stops it + releases.
  _route_lock_heartbeat_start
  trap '_route_lock_heartbeat_stop; _route_lock_release' RETURN
  # Meter (ebec.1): route wall-clock anchor; both verdict writes pass it on.
  local _route_t0; _route_t0="$(date +%s)"
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
  # S3: derive the effective engage-deadline from the recorded route metrics (an explicit
  # operator PAWL_ENGAGE_DEADLINE wins inside resolve_engage_deadline).
  PAWL_ENGAGE_DEADLINE="$(resolve_engage_deadline "$ROOT/$STATE_DIR/metrics.jsonl")"
  log "route $bead -> [$ENABLED] tier=$TIER min=$minc engage-deadline=${PAWL_ENGAGE_DEADLINE}s (packet=$packet, pr=$pr, nonce=$nonce)"

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
  local waited=0 cc_rr=0 cod_rr=0 agy_rr=0 cs="" session_lost=0
  # age-djfo (c): per-pane stall give-up. cksum (POSIX, always present — a change-detector, not
  # crypto) of recent output; unchanged for PAWL_STALL_GIVEUP seconds => alive-but-stuck => give
  # up (degrade) instead of burning the full ROUTE_TIMEOUT. Clearing a known prompt CHANGES the
  # output, so a CLEARABLE block resets the stall (the pane unblocks) rather than being given up.
  local cc_h="" cod_h="" agy_h="" cc_st=0 cod_st=0 agy_st=0 cc_gu=0 cod_gu=0 agy_gu=0 _h
  while [ "$waited" -lt "$ROUTE_TIMEOUT" ]; do
    # S3: a reaped/vanished standing session must surface as HOLD:service-unavailable, never
    # burn down into a defect-claiming REFUTED — break to the outcome mapping below.
    if ! session_exists; then
      log "standing session $SESSION disappeared mid-route — degrading to HOLD (service-unavailable)"
      session_lost=1
      break
    fi
    # (the lease is kept fresh by the background heartbeat started at acquire — it covers the
    # send/respawn phase too, so no per-tick touch is needed here.)
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
      if [ "$cc_gu" -eq 0 ] && _engage_over_deadline "$waited"; then log "claude pane no verdict by ${PAWL_ENGAGE_DEADLINE}s engage-deadline — giving up (degrade)"; cc_gu=1; fi
    fi
    if [ -z "$vd" ] && [ "$cod_gu" -eq 0 ]; then
      _h="$(tmux capture-pane -p -t "${SESSION}.${COD_PANE}" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true)"
      if [ -n "$_h" ] && [ "$_h" = "$cod_h" ]; then
        cod_st=$((cod_st + 5))
        if _stall_over_budget "$cod_st" "$PAWL_STALL_GIVEUP"; then log "codex pane stalled ${cod_st}s (no new output) — giving up (degrade)"; cod_gu=1; fi
      else cod_h="$_h"; cod_st=0; fi
      if [ "$cod_gu" -eq 0 ] && _engage_over_deadline "$waited"; then log "codex pane no verdict by ${PAWL_ENGAGE_DEADLINE}s engage-deadline — giving up (degrade)"; cod_gu=1; fi
    fi
    if [ -z "$va" ] && [ "$agy_gu" -eq 0 ]; then
      _h="$(tmux capture-pane -p -t "${SESSION}.${AGY_PANE}" -S -25 2>/dev/null | cksum 2>/dev/null | cut -d' ' -f1 || true)"
      if [ -n "$_h" ] && [ "$_h" = "$agy_h" ]; then
        agy_st=$((agy_st + 5))
        if _stall_over_budget "$agy_st" "$PAWL_STALL_GIVEUP"; then log "agy pane stalled ${agy_st}s (no new output) — giving up (degrade)"; agy_gu=1; fi
      else agy_h="$_h"; agy_st=0; fi
      if [ "$agy_gu" -eq 0 ] && _engage_over_deadline "$waited"; then log "agy pane no verdict by ${PAWL_ENGAGE_DEADLINE}s engage-deadline — giving up (degrade)"; agy_gu=1; fi
    fi
    # Done when every enabled pane is RESOLVED — a verdict (n/a sentinel counts) OR given-up.
    if { [ -n "$vc" ] || [ "$cc_gu" -eq 1 ]; } && { [ -n "$vd" ] || [ "$cod_gu" -eq 1 ]; } && { [ -n "$va" ] || [ "$agy_gu" -eq 1 ]; }; then break; fi
    # age-55qz.10: do NOT short-circuit the loop the instant ANY pane REFUTES. codex is the only
    # engagement-verified + fast family (~43s), so a break-on-any-REFUTED let its verdict GUILLOTINE
    # the still-pending opus/agy panes — they mapped to <timeout> and the route fail-closed REFUTED
    # even when opus was out-voting a codex false-positive (confirmed by live probe). The single-
    # refute-blocks rule is UNCHANGED — it lives in pawl_decide (refuted>=1 -> REFUTED) and runs AFTER
    # the loop, so the disposition is identical; only WHEN we stop waiting changes, and there is no
    # path to a false-CONFIRM (fail-safe). The all-resolved exit above + the per-pane engage-deadline
    # give-up bound how long a slow/compacting pane can hold the route.
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
    # age-agy-reliability: recover an agy pane that has produced NO verdict — whether it dropped to a
    # shell (dead) OR is alive but stalled (Antigravity intermittently drops the input or starts then
    # stalls mid-review). Respawn to a fresh pane + re-send once (bounded by agy_rr<1, never thrashes).
    if [ -z "$va" ] && [ "$agy_gu" -eq 0 ] && [ "$agy_rr" -lt 1 ] \
       && { agy_dead || [ "$agy_st" -ge "${PAWL_AGY_RESEND_STALL:-60}" ]; }; then
      # Trigger on STALL (agy_st = seconds with no new output), NOT elapsed time: a slow-but-working
      # agy on a big diff keeps producing output (agy_st stays ~0) and must be left to finish — only a
      # DROPPED input (empty pane) or a START-THEN-STALL (silent >= threshold) gets respawned + re-sent.
      log "agy no verdict$(agy_dead && echo ' (dropped to shell)' || echo " — stalled ${agy_st}s (dropped/stuck)") — respawn + re-send"
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
  confirmed="${_decision##*:}"
  # S3 honesty split: record and bind the OUTCOME (route_outcome over the quorum decision +
  # session availability), never the raw quorum token — a no-substantive-refuter degrade is
  # HOLD (insufficient-reviewers / service-unavailable), not a defect-claiming REFUTED. The
  # quorum decision itself (pawl_decide) is untouched.
  local _outcome; _outcome="$(route_outcome "$_decision" "$session_lost")"
  disposition="${_outcome%%:*}"; detail="${_outcome##*:}"
  case "$detail" in
    degraded)               degraded="degraded: ${confirmed}/${total} families CONFIRMED (tier=$TIER min=${minc} still met)" ;;
    insufficient-reviewers) degraded="insufficient reviewers: ${confirmed}/${total} CONFIRMED (tier=$TIER needs >=${minc}) — deadline/stall give-ups, no substantive refutation" ;;
    service-unavailable)    degraded="standing pawl session disappeared mid-route (${confirmed}/${total} CONFIRMED before loss)" ;;
  esac

  # One SLO datapoint per route — non-blocking + fail-safe (must NEVER affect the verdict).
  { _lat=$(( $(date +%s) - _route_t0 ))
    _agree="disagree"; { [ "$disposition" = "CONFIRMED" ] && [ "$confirmed" -eq "$total" ]; } && _agree="agree"
    _pawl_verify_state_dir && mkdir -p "$ROOT/$STATE_DIR"
    _pawl_unlink_if_symlink "$ROOT/$STATE_DIR/metrics.jsonl"   # never `>>`-append THROUGH a symlink
    printf '{"ts":"%s","bead":"%s","tier":"%s","families":"%s","latency_s":%d,"opus":"%s","codex":"%s","agy":"%s","confirmed":%d,"total":%d,"disposition":"%s","agreement":"%s"}\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$bead" "$TIER" "$ENABLED" "$_lat" "${vc:-timeout}" "${vd:-timeout}" "${va:-timeout}" "$confirmed" "$total" "$disposition" "$_agree" \
      >> "$ROOT/$STATE_DIR/metrics.jsonl"
  } 2>/dev/null || true

  _touch_route_ts   # reset the idle-TTL clock: this route is real use

  # mode reflects the tier achieved: multi-model (>=2 families) vs fresh-context (single family).
  # A fresh-context verdict is recorded honestly; a high-irreversibility door (pawl-verdict.sh /
  # the push gate) decides whether that tier is sufficient.
  local mode="multi-model"; [ "$TIER" = "fresh" ] && mode="fresh-context"
  # age-33nx: pawl-review exports PAWL_ROUTE_HEAD as the commit its packet was
  # built from (walked back over #trivial provenance-bind commits); bind the routed
  # verdict to THAT commit so packet and binding can never disagree. Absent env
  # (direct route callers) keeps the live-HEAD behavior.
  local head; head="${PAWL_ROUTE_HEAD:-$(git rev-parse HEAD)}"
  if [ "$disposition" = "CONFIRMED" ]; then
    # Record ONLY the CONFIRMED enabled refuters; an unavailable/disabled pane is omitted, never
    # recorded as a false CONFIRM.
    local -a rf=()
    [ "$vc" = "CONFIRMED" ] && rf+=(--refuter "claude:CONFIRMED:opus-pawl-pane-fresh:${ev_cc}")
    [ "$vd" = "CONFIRMED" ] && rf+=(--refuter "gpt:CONFIRMED:codex-pawl-pane-gpt55:${ev_cod}")
    [ "$va" = "CONFIRMED" ] && rf+=(--refuter "gemini:CONFIRMED:agy-pawl-pane-flash35:${ev_agy}")
    bash "$PAWL_VERDICT_SH" write "$bead" "$pr" --dir "$PAWL_VERDICT_DIR" \
      --disposition CONFIRMED --head "$head" \
      --author-context "pawl-route-author-${bead}" --mode "$mode" \
      --wall-seconds "$(( $(date +%s) - _route_t0 ))" \
      "${rf[@]}" >&2
    log "ROUTE $bead: CONFIRMED (${confirmed}/${total} agree, tier=$TIER${degraded:+; $degraded}) — verdict recorded for head $head"
    echo "CONFIRMED"
    return 0
  fi
  # Fail-closed REFUTED/HOLD: STILL record the verdict (age-uxva — the membrane catch we MOST
  # want logged; the chokepoint emit lives in pawl-verdict.sh write). Record ONLY the enabled
  # panes' actual results (a timeout maps to the REFUTED token).
  # age-pawl-good-bar #4: record each enabled pane's ACTUAL emitted verdict (via the pure
  # _refuted_refuters helper). A timed-out pane has NO verdict — the old `${vc:-REFUTED}` FABRICATED
  # a refutation the pane never made, corrupting the membrane's own evidence (metrics.jsonl already
  # logs these as "timeout", so the two disagreed). Disposition is unchanged (pawl_decide decided).
  local -a rf=() _spec
  while IFS= read -r _spec; do [ -n "$_spec" ] && rf+=(--refuter "$_spec"); done \
    < <(_refuted_refuters "$vc" "$vd" "$va" "$ev_cc" "$ev_cod" "$ev_agy")
  bash "$PAWL_VERDICT_SH" write "$bead" "$pr" --dir "$PAWL_VERDICT_DIR" \
    --disposition "$disposition" --head "$head" \
    --author-context "pawl-route-author-${bead}" --mode "$mode" \
    "${rf[@]}" \
    --wall-seconds "$(( $(date +%s) - _route_t0 ))" \
    --reason "standing-pawl route: ${degraded:-no agreement} (tier=$TIER; opus=${vc:-timeout} codex=${vd:-timeout} agy=${va:-timeout})" >&2 || true
  # age-2yh2: ALSO record a structured membrane catch with the refuting reviewer's REAL
  # finding (the generic route --reason above is a non-actionable pseudo-class). The
  # first-refuter lane's capture is the evidence; no substantive refuter (all-timeout/
  # insufficient) => _refuting_evidence echoes nothing and the emit skips — a timeout has
  # no finding to salvage. SINGLE-RECORD: this is the only catch emit for a routed REFUTE —
  # pawl-review.sh's routed-REFUTED branch deliberately does NOT call emit_pawl_catch
  # (its $evidence file is never written on the routed path; see the guard comment there).
  # Non-blocking + fail-safe: never affects the REFUTED exit below.
  _route_emit_catch "$bead" "$head" "$mode" \
    "$(_refuting_evidence "$vc" "$vd" "$va" "$ev_cc" "$ev_cod" "$ev_agy")" || true
  log "ROUTE $bead: $disposition — tier=$TIER ${degraded:-no agreement} (evidence in $EVID_DIR)"
  echo "$disposition"
  return 1
}

# SLO surface over the recorded routes — p50/p95 round-trip latency + agreement rate (all-enabled
# CONFIRMED vs disagreement). Reads the append-only metrics.jsonl cmd_route writes.
cmd_metrics() {
  local mf="$ROOT/$STATE_DIR/metrics.jsonl" json=0
  [ "${1:-}" = "--json" ] && json=1
  if [ ! -s "$mf" ]; then
    [ "$json" = 1 ] && echo '{"routes":0,"skipped_corrupt":0}' || echo "pawl metrics: no routed beads recorded yet ($mf)"
    return 0
  fi
  # -I (isolated): drop cwd from sys.path so a repo-planted json.py is never imported (RCE guard on
  # the untrusted-repo path) — `ao pawl metrics` runs with the untrusted repo as cwd.
  python3 -I - "$mf" "$json" <<'PY'
import json,sys
mf,asjson=sys.argv[1],sys.argv[2]=="1"
# Fail-SOFT: a corrupt/partial append (e.g. a route killed mid-write) must not crash the
# SLO surface — skip unparseable lines, report on the rest. age-l3xj (D5): the skip is
# EXPLICIT (skipped_corrupt), never silent — a torn append is an operational signal.
rows=[]; skipped=0
for l in open(mf):
    l=l.strip()
    if not l: continue
    try: rows.append(json.loads(l))
    except Exception:
        skipped+=1
        continue
n=len(rows)
lat=sorted(int(r.get("latency_s",0)) for r in rows)
def pct(p):
    if not lat: return 0
    return lat[min(len(lat)-1,int(round((p/100.0)*(len(lat)-1))))]
agree=sum(1 for r in rows if r.get("agreement")=="agree")
dis=n-agree
out={"routes":n,"latency_p50_s":pct(50),"latency_p95_s":pct(95),
     "agreement_rate":round(agree/n,3) if n else 0,"agree":agree,"disagreements":dis,
     "skipped_corrupt":skipped}
if asjson:
    print(json.dumps(out))
else:
    print(f"pawl metrics: {n} routed beads")
    print(f"  latency p50={out['latency_p50_s']}s p95={out['latency_p95_s']}s")
    print(f"  agreement {agree}/{n} ({out['agreement_rate']}); disagreements={dis}")
    if skipped:
        print(f"  skipped {skipped} corrupt row(s) in {mf}")
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
  doctor) shift; cmd_doctor "$@" ;;
  smoke)  shift; cmd_smoke "$@" ;;
  route)  shift; cmd_route "$@" ;;
  metrics) shift; cmd_metrics "$@" ;;
  *) cat >&2 <<'H'
Usage: pawl.sh <up|down|reap|health|doctor|smoke|route|metrics>
  up [--dual|--tri|--models a,b,c]  spawn + readiness-gate the standing pawl session (idempotent).
                                    Default: probe installed CLIs (claude/codex/agy) and stand up
                                    the STRONGEST membrane possible. --dual=cc,cod; --tri=all;
                                    --models is an explicit family list (cc/cod/agy or aliases).
  down                              tear down the standing session
  reap                              tear down iff idle > PAWL_IDLE_TTL (substrate/cron schedules it)
  health [--json]                   per-pane liveness/readiness + the membrane tier
  doctor|smoke [--json]             read-only preflight: assert atm alias, session, cwd, model,
                                    trust-prompt, readiness, and standing evidence policy
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
