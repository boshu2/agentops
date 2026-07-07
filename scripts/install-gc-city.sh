#!/usr/bin/env bash
# install-gc-city.sh — stand up a correct membrane Gas City in one command.
#
# Automates skills/using-gc/references/standup.md (the 153-line bootstrap
# contract) in ITS order — isolation → version contract → init + pinned packs →
# providers/LAW-0 → membrane wiring (incl. gap-A materialization) → boot →
# gate on green — and closes the packs/agentops-membrane/RESIDUAL-GAPS.md
# catalog (codex pre-trust, usage provider=local) on the way.
#
# Usage:
#   scripts/install-gc-city.sh <city-dir> [options]
#
# Options:
#   --name <n>           City name (default: basename of <city-dir>). Also the
#                        dedicated tmux socket name (tmux -L <n>).
#   --gc-binary <path>   Explicit gc binary (default: the ONE gc on PATH).
#   --pack-source <dir>  Local agentops-membrane pack dir (default: this
#                        repo's packs/agentops-membrane). Local-path imports
#                        are read-in-place per standup.md §2 (no packs.lock
#                        entry; pin by sha once published).
#   --skip-sessions      Do not create/verify the trinity + AGY sessions.
#   --dry-run            Run read-only preflight (incl. the version contract),
#                        print the plan, write NOTHING.
#
# Every failure exits non-zero with a one-line REMEDY.
# bash 3.2 compatible (macOS): no associative arrays, no mapfile.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info() { printf "${GREEN}✓${NC} %b\n" "$*"; }
warn() { printf "${YELLOW}!${NC} %b\n" "$*"; }
step() { printf "\n== %s ==\n" "$*"; }
# die <message> [remedy] — every failure carries a one-line REMEDY.
die() {
  printf "${RED}✗${NC} %b\n" "$1" >&2
  [ -n "${2:-}" ] && printf "  REMEDY: %b\n" "$2" >&2
  exit 1
}

usage() {
  sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
}

# --- constants ----------------------------------------------------------------
DOLT_FLOOR="${DOLT_FLOOR:-2.1.0}"          # standup.md §1: dolt >= managed floor
DEFAULT_SUPERVISOR_PORT="${GC_SUPERVISOR_PORT:-8372}"
MANAGED_MARK="# managed by install-gc-city.sh — regenerated on re-run"
OPERATOR_MARK="# --- operator additions below this line (preserved on re-run) ---"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- args ---------------------------------------------------------------------
CITY_ARG=""
CITY_NAME=""
GC_BIN_ARG=""
PACK_SOURCE="${SCRIPT_DIR}/../packs/agentops-membrane"
SKIP_SESSIONS=0
DRY_RUN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --name) CITY_NAME="${2:?--name needs a value}"; shift 2 ;;
    --gc-binary) GC_BIN_ARG="${2:?--gc-binary needs a value}"; shift 2 ;;
    --pack-source) PACK_SOURCE="${2:?--pack-source needs a value}"; shift 2 ;;
    --skip-sessions) SKIP_SESSIONS=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -*) echo "Unknown arg: $1" >&2; usage >&2; exit 2 ;;
    *)
      [ -z "$CITY_ARG" ] || { echo "Unexpected extra argument: $1" >&2; usage >&2; exit 2; }
      CITY_ARG="$1"; shift ;;
  esac
done
[ -n "$CITY_ARG" ] || { usage >&2; exit 2; }

# Absolute city path WITHOUT requiring it to exist yet (dry-run must not mkdir).
case "$CITY_ARG" in
  /*) CITY="$CITY_ARG" ;;
  *)  CITY="$PWD/$CITY_ARG" ;;
esac
[ -n "$CITY_NAME" ] || CITY_NAME="$(basename "$CITY")"
GC_HOME="$CITY/.gc-home"

# --- helpers -------------------------------------------------------------------

# extract_semver — first X.Y.Z in stdin.
extract_semver() { grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | head -1; }

# ver_ge <a> <b> — numeric dotted-version compare, a >= b.
ver_ge() {
  local a1 a2 a3 b1 b2 b3
  a1="${1%%.*}"; a3="${1##*.}"; a2="${1#*.}"; a2="${a2%%.*}"
  b1="${2%%.*}"; b3="${2##*.}"; b2="${2#*.}"; b2="${b2%%.*}"
  [ "$a1" -ne "$b1" ] && { [ "$a1" -gt "$b1" ]; return; }
  [ "$a2" -ne "$b2" ] && { [ "$a2" -gt "$b2" ]; return; }
  [ "$a3" -ge "$b3" ]
}

# run_gc <args...> — run gc from the city with the isolated GC_HOME + shim PATH.
# GC_CITY_PATH is PINNED to the target city: an inherited value from another
# city's shell (env.sh sourced earlier, a supervisor env) would otherwise make
# gc resolve the WRONG city while this installer declares success for the
# requested one (wrong-object; pawl converge finding 2026-07-06).
run_gc() {
  (cd "$CITY" && env GC_HOME="$GC_HOME" GC_BIN="$GC_BIN" GC_CITY_PATH="$CITY" \
    PATH="$GC_HOME/bin:$PATH" "$GC_BIN" "$@")
}

# run_in_city <cmd...> — run any command (bd, …) from the city with the shim PATH.
run_in_city() {
  (cd "$CITY" && env GC_HOME="$GC_HOME" GC_CITY_PATH="$CITY" PATH="$GC_HOME/bin:$PATH" "$@")
}

port_busy() {
  # bash /dev/tcp connect probe — read-only, no external deps (bash 3.2 ok).
  ( exec 3<>"/dev/tcp/127.0.0.1/$1" ) 2>/dev/null
}

# --- 0. preflight (read-only) ---------------------------------------------------
step "0. Preflight"

# GC_BEADS=file is a troubleshooting escape hatch, not a real backend
# (standup.md §1) — refuse it outright.
if [ "${GC_BEADS:-}" = "file" ]; then
  die "GC_BEADS=file is set in the environment — the file backend is refused (control dispatcher + core-pack orders die on it)" \
      "unset GC_BEADS and re-run; this city runs the native bd/dolt store"
fi

for cmd in jq git tmux bd dolt; do
  command -v "$cmd" >/dev/null 2>&1 || \
    die "required command not found: $cmd" "install $cmd and re-run (brew install $cmd)"
done

# Resolve the gc binary. Exactly one gc on PATH must win (standup.md §1: stale
# ~/go/bin/gc duplicates have shadowed the real one before).
PATH_GC_LIST=""
OLD_IFS="$IFS"; IFS=:
for dir in $PATH; do
  IFS="$OLD_IFS"
  if [ -n "$dir" ] && [ -x "$dir/gc" ] && [ -f "$dir/gc" ]; then
    # normalize to the physical dir so e.g. ~/.local/share/../bin and
    # ~/.local/bin do not count as two binaries
    phys="$(cd "$dir" 2>/dev/null && pwd -P || printf '%s' "$dir")"
    PATH_GC_LIST="${PATH_GC_LIST}${phys}/gc
"
  fi
  IFS=:
done
IFS="$OLD_IFS"
PATH_GC_UNIQUE="$(printf '%s' "$PATH_GC_LIST" | awk 'NF' | sort -u)"
PATH_GC_COUNT=0
[ -n "$PATH_GC_UNIQUE" ] && PATH_GC_COUNT="$(printf '%s\n' "$PATH_GC_UNIQUE" | wc -l | tr -d ' ')"

if [ -n "$GC_BIN_ARG" ]; then
  [ -x "$GC_BIN_ARG" ] || die "--gc-binary is not an executable file: $GC_BIN_ARG" \
    "build gc from the owned fork: cd ~/dev/gascity && make build, then pass --gc-binary ~/dev/gascity/bin/gc"
  GC_BIN="$(cd "$(dirname "$GC_BIN_ARG")" && pwd)/$(basename "$GC_BIN_ARG")"
  [ "$PATH_GC_COUNT" -gt 1 ] && warn "multiple gc binaries on PATH (using --gc-binary anyway):\n$PATH_GC_UNIQUE"
else
  [ "$PATH_GC_COUNT" -ge 1 ] || die "no gc binary found on PATH" \
    "build from the owned fork (cd ~/dev/gascity && make build) and pass --gc-binary ~/dev/gascity/bin/gc"
  if [ "$PATH_GC_COUNT" -gt 1 ]; then
    die "multiple gc binaries on PATH — 'first wins' is how stale ~/go/bin/gc shadows the real one:\n$PATH_GC_UNIQUE" \
        "remove the stale duplicates (commonly: rm ~/go/bin/gc) or pass --gc-binary <fork>/bin/gc"
  fi
  GC_BIN="$(printf '%s\n' "$PATH_GC_UNIQUE" | head -1)"
fi
info "gc binary: $GC_BIN"

# Pack source must be a local directory (local-path imports are read-in-place;
# gap-A materialization copies files from it).
case "$PACK_SOURCE" in
  http://*|https://*|git@*)
    die "--pack-source must be a LOCAL directory (gap-A materialization copies membrane/ from it)" \
        "clone the repo and pass --pack-source <checkout>/packs/agentops-membrane" ;;
esac
[ -d "$PACK_SOURCE" ] || die "pack source not found: $PACK_SOURCE" \
  "pass --pack-source <path-to>/packs/agentops-membrane"
PACK_SOURCE="$(cd "$PACK_SOURCE" && pwd)"
[ -f "$PACK_SOURCE/pack.toml" ] || die "not a gc pack (no pack.toml): $PACK_SOURCE" \
  "point --pack-source at packs/agentops-membrane"
PACK_NAME="$(sed -n 's/^name[[:space:]]*=[[:space:]]*"\(.*\)"/\1/p' "$PACK_SOURCE/pack.toml" | head -1)"
[ -n "$PACK_NAME" ] || PACK_NAME="agentops-membrane"
info "pack source: $PACK_SOURCE (name: $PACK_NAME)"

# --- 1. version contract (F2 — fail hard, standup.md §1) ------------------------
step "1. Version contract"

GC_VERSION_JSON="$("$GC_BIN" version --json 2>/dev/null || true)"
GC_VERSION="$(printf '%s' "$GC_VERSION_JSON" | jq -r '.version // empty' 2>/dev/null || true)"
GC_COMMIT="$(printf '%s' "$GC_VERSION_JSON" | jq -r '.commit // empty' 2>/dev/null || true)"
[ -n "$GC_VERSION" ] || die "'$GC_BIN version --json' did not report a version" \
  "the binary is not a working gc; rebuild from the fork (cd ~/dev/gascity && make build)"
info "gc $GC_VERSION (commit ${GC_COMMIT:-unknown})"

# bd must EXACTLY match the beads library gc's go.mod pins. Resolve the gc
# source tree: --gc-binary at <fork>/bin/gc puts go.mod one level up; else the
# documented fork checkout; GC_SRC overrides.
BEADS_PIN=""
GC_BIN_DIR="$(dirname "$GC_BIN")"
for cand in "${GC_SRC:-}" "$GC_BIN_DIR/.." "$GC_BIN_DIR" "$HOME/dev/gascity"; do
  [ -n "$cand" ] && [ -f "$cand/go.mod" ] || continue
  BEADS_PIN="$(grep -E 'steveyegge/beads[[:space:]]+v[0-9]+\.[0-9]+\.[0-9]+' "$cand/go.mod" | head -1 | extract_semver || true)"
  [ -n "$BEADS_PIN" ] && { info "beads pin v$BEADS_PIN (from $cand/go.mod)"; break; }
done
if [ -z "$BEADS_PIN" ]; then
  # gc version heuristics carry no beads pin — fail closed rather than guess.
  die "cannot resolve the beads version gc pins (no go.mod with steveyegge/beads near $GC_BIN)" \
      "pass --gc-binary <gascity-fork>/bin/gc (go.mod lives next to bin/) or set GC_SRC=<gascity-fork>"
fi

BD_VERSION="$(bd version 2>/dev/null | extract_semver || true)"
[ -n "$BD_VERSION" ] || die "could not parse 'bd version' output" \
  "reinstall bd v$BEADS_PIN (brew install bd or go install github.com/steveyegge/beads/cmd/bd@v$BEADS_PIN)"
if [ "$BD_VERSION" != "$BEADS_PIN" ]; then
  die "version contract violation: bd $BD_VERSION != beads v$BEADS_PIN pinned by gc's go.mod (silent perf/compat cliff)" \
      "install bd v$BEADS_PIN exactly (go install github.com/steveyegge/beads/cmd/bd@v$BEADS_PIN) or rebuild gc against beads v$BD_VERSION"
fi
info "bd $BD_VERSION == gc's beads pin (exact match)"

DOLT_VERSION="$(dolt version 2>/dev/null | extract_semver || true)"
[ -n "$DOLT_VERSION" ] || die "could not parse 'dolt version' output" \
  "reinstall dolt >= $DOLT_FLOOR (brew install dolt)"
ver_ge "$DOLT_VERSION" "$DOLT_FLOOR" || \
  die "version contract violation: dolt $DOLT_VERSION < floor $DOLT_FLOOR" \
      "upgrade dolt (brew upgrade dolt) to >= $DOLT_FLOOR"
info "dolt $DOLT_VERSION >= floor $DOLT_FLOOR"

# --- supervisor port (chosen read-only; written in step 2) ----------------------
if [ -f "$GC_HOME/supervisor.toml" ]; then
  SUPERVISOR_PORT="$(sed -n 's/^port[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$GC_HOME/supervisor.toml" | head -1)"
  [ -n "$SUPERVISOR_PORT" ] || SUPERVISOR_PORT="$DEFAULT_SUPERVISOR_PORT"
  PORT_ORIGIN="existing $GC_HOME/supervisor.toml"
else
  SUPERVISOR_PORT="$DEFAULT_SUPERVISOR_PORT"
  tries=0
  while port_busy "$SUPERVISOR_PORT"; do
    SUPERVISOR_PORT=$((SUPERVISOR_PORT + 1))
    tries=$((tries + 1))
    [ "$tries" -le 100 ] || die "no free supervisor port in ${DEFAULT_SUPERVISOR_PORT}..$((DEFAULT_SUPERVISOR_PORT + 100))" \
      "set GC_SUPERVISOR_PORT=<free-port> and re-run"
  done
  PORT_ORIGIN="first free from $DEFAULT_SUPERVISOR_PORT"
fi

# --- dry run: plan only, write nothing ------------------------------------------
if [ "$DRY_RUN" -eq 1 ]; then
  step "DRY RUN — plan (nothing written)"
  cat <<EOF
  city dir        : $CITY
  city name       : $CITY_NAME
  GC_HOME         : $GC_HOME (dedicated; legacy ~/.gc untouched)
  tmux socket     : -L $CITY_NAME (dedicated)
  supervisor port : $SUPERVISOR_PORT ($PORT_ORIGIN)
  gc binary       : $GC_BIN ($GC_VERSION, commit ${GC_COMMIT:-unknown})
  bd              : $BD_VERSION (== gc beads pin)
  dolt            : $DOLT_VERSION (floor $DOLT_FLOOR)
  pack            : $PACK_NAME from $PACK_SOURCE (local read-in-place import)
  would do        : git init city; write env.sh + supervisor.toml + git shim;
                    gc init --no-start; write managed city.toml (LAW-0
                    print_args=[], codex CODEX_HOME pre-trust, [usage]
                    provider=local, verifier lanes always-on); import pack;
                    materialize membrane/ into the city (gap A); pretrust
                    codex home; law0 doctor check; gc start; enforce
                    dolt_mode=server + NativeDoltStore; create+verify
                    sessions$( [ "$SKIP_SESSIONS" -eq 1 ] && printf ' (SKIPPED: --skip-sessions)'); gc doctor gate.
EOF
  exit 0
fi

# --- 2. isolation invariants (standup.md §0) -------------------------------------
step "2. Isolation (city dir, GC_HOME, socket, port, env.sh)"

mkdir -p "$CITY"
# Absolute PHYSICAL path: every derived value (GC_HOME, env.sh, plists, the
# GC_CITY_PATH pin) shares this one resolution — relative/symlinked arguments
# must not re-resolve differently later (round-8 pawl class).
CITY="$(cd "$CITY" && pwd -P)"
GC_HOME="$CITY/.gc-home"
if [ ! -e "$CITY/.git" ]; then
  git -C "$CITY" init -q
  info "git repo initialized (own city dir = its own git repo)"
fi

mkdir -p "$GC_HOME/bin"

# Host quirk (standup.md §6): homebrew git can hang ~151s on network ops; gc
# shells out to PATH-resolved git for pack clones — shim /usr/bin/git first.
if [ -x /usr/bin/git ] && [ ! -e "$GC_HOME/bin/git" ]; then
  ln -s /usr/bin/git "$GC_HOME/bin/git"
fi

# Explicit supervisor port — isolated GC_HOMEs do NOT auto-pick (standup.md §0).
if [ ! -f "$GC_HOME/supervisor.toml" ]; then
  printf '[supervisor]\nport = %s\n' "$SUPERVISOR_PORT" > "$GC_HOME/supervisor.toml"
fi
info "supervisor port $SUPERVISOR_PORT ($PORT_ORIGIN)"

# env.sh — the operator entrypoint (fully generated; safe to overwrite).
cat > "$CITY/env.sh" <<EOF
# $CITY_NAME operator environment — source this in every session that touches
# this city:   source $CITY/env.sh
$MANAGED_MARK

# Dedicated GC_HOME (deployment isolation; the legacy ~/.gc is NEVER touched).
export GC_HOME="$GC_HOME"

# The city runs the pinned gc binary, not whatever is on PATH.
export GC_BIN="$GC_BIN"

# Dedicated tmux socket so the city coexists with NTM and other cities:
#   tmux -L $CITY_NAME ls
export GC_TMUX_SOCKET="$CITY_NAME"

# git shim first on PATH (\$GC_HOME/bin/git -> /usr/bin/git): pre-empts the
# homebrew-git network hang on pack clones.
export PATH="\$GC_HOME/bin:\$PATH"

# NATIVE BEADS: never set GC_BEADS=file — this city runs the bd/dolt store.

# Pin the city path: an inherited GC_CITY_PATH from another city's shell would
# silently point gc at the WRONG city (wrong-object hazard).
export GC_CITY_PATH="$CITY"

# Convenience wrapper: gc in an operator shell resolves the right binary+home+city.
gc() { GC_HOME="$GC_HOME" GC_CITY_PATH="$CITY" PATH="$GC_HOME/bin:\$PATH" "$GC_BIN" "\$@"; }
EOF
info "env.sh written (source $CITY/env.sh)"

# --- 3. gc init + pinned packs (standup.md §2) -----------------------------------
step "3. gc init + pack imports"

if [ -f "$CITY/pack.toml" ]; then
  info "pack.toml exists — skipping gc init (idempotent re-run)"
else
  run_gc init --default-provider codex --no-start --name "$CITY_NAME" "$CITY" >/dev/null || \
    die "gc init failed for $CITY" \
        "run manually to see the wizard error: GC_HOME=$GC_HOME $GC_BIN init --default-provider codex --no-start $CITY"
  info "gc init done (pinned core+bd imports, --no-start)"
fi
[ -f "$CITY/pack.toml" ] || die "gc init did not write $CITY/pack.toml" \
  "re-run gc init manually and inspect its output"

# Import agentops-membrane by absolute local path (read-in-place — standup.md
# §2; no packs.lock entry for local paths; pin by sha once published).
if grep -q "imports.${PACK_NAME}" "$CITY/pack.toml"; then
  info "pack.toml already imports $PACK_NAME (idempotent re-run)"
else
  cat >> "$CITY/pack.toml" <<EOF

# $PACK_NAME: local absolute-path import (read-in-place, no clone/lock) —
# added by install-gc-city.sh. Pin by sha via a tree-URL once published.
[imports.${PACK_NAME}]
source = "$PACK_SOURCE"
EOF
  info "pack.toml: [imports.${PACK_NAME}] -> $PACK_SOURCE"
fi

run_gc import install >/dev/null 2>&1 || \
  die "gc import install failed" \
      "run: source $CITY/env.sh && gc import install — if a remote clone hangs, the git shim at $GC_HOME/bin/git should already bypass it"
info "gc import install ok (core+bd pins in packs.lock; $PACK_NAME read-in-place)"

# --- 4. managed city.toml: providers (LAW 0), usage, lanes -----------------------
step "4. city.toml (LAW-0 print sinks dead, codex pre-trust wiring, usage, lanes)"

CODEX_HOME_DIR="$CITY/.gc/codex-home"

# Preserve any operator additions below the marker across re-runs; back up a
# pre-existing unmanaged city.toml once.
OPERATOR_TAIL=""
if [ -f "$CITY/city.toml" ]; then
  if grep -qF "$MANAGED_MARK" "$CITY/city.toml"; then
    OPERATOR_TAIL="$(awk -v m="$OPERATOR_MARK" 'found; $0 == m {found=1}' "$CITY/city.toml")"
  else
    cp "$CITY/city.toml" "$CITY/city.toml.pre-install-gc-city.bak"
    warn "existing unmanaged city.toml backed up to city.toml.pre-install-gc-city.bak"
  fi
fi

# [usage] stanza is single-sourced from the pack's template fragment.
USAGE_STANZA="$(sed '/^[[:space:]]*#/d; /^[[:space:]]*$/d' "$PACK_SOURCE/template-fragments/usage-local.toml" 2>/dev/null || true)"
[ -n "$USAGE_STANZA" ] || USAGE_STANZA='[usage]
provider = "local"'

# Fast orphan-sweep override — same single-source mechanism (recovery memo R1:
# stock 5m cooldown strands dead-agent work; 60s collapses requeue latency).
SWEEP_STANZA="$(sed '/^[[:space:]]*#/d; /^[[:space:]]*$/d' "$PACK_SOURCE/template-fragments/orphan-sweep-fast.toml" 2>/dev/null || true)"
[ -n "$SWEEP_STANZA" ] || SWEEP_STANZA='[[orders.overrides]]
name = "orphan-sweep"
interval = "60s"'

cat > "$CITY/city.toml" <<EOF
$MANAGED_MARK
# Contract: skills/using-gc/references/standup.md §§3-4 + packs/agentops-membrane.

[workspace]
name = "$CITY_NAME"
provider = "codex"            # workspace default: NOT claude (title-gen sink)

[session]
socket = "$CITY_NAME"         # dedicated tmux socket (tmux -L $CITY_NAME)

# LAW 0 structural: empty NON-nil print_args overrides the builtin defaults
# (claude ["-p"], antigravity ["--print"]) — the law0-print-args doctor check
# (exit-2 blocking) asserts this holds.
[providers.claude]
base = "builtin:claude"
print_args = []

[providers.codex]
base = "builtin:codex"
# Pre-trusted city-scoped CODEX_HOME (RESIDUAL-GAPS.md gap 2): seeded by the
# pack's pretrust-codex-home.sh; the operator's global ~/.codex stays untouched.
env = { CODEX_HOME = "$CODEX_HOME_DIR" }

[providers.antigravity]       # AGY = the sanctioned Gemini path (never gemini -p)
base = "builtin:antigravity"
print_args = []

# Cost metering — see packs/agentops-membrane/template-fragments/usage-local.toml
# (sub-backed CLIs emit no usage facts; provider=local populates run rows).
$USAGE_STANZA

# Fast work-requeue — see packs/agentops-membrane/template-fragments/orphan-sweep-fast.toml
# (stock 5m sweep strands dead-agent-assigned work; age-gc-adoption-u0he.14 R1).
$SWEEP_STANZA

# Membrane reviewer lanes: always-on named sessions so the close gate's
# 'gc session submit' delivery is deterministic (standup.md §4).
[[named_session]]
template = "${PACK_NAME}.verifier"
mode = "always"
[[named_session]]
template = "${PACK_NAME}.agy-verifier"
mode = "always"

$OPERATOR_MARK
EOF
[ -n "$OPERATOR_TAIL" ] && printf '%s\n' "$OPERATOR_TAIL" >> "$CITY/city.toml"
info "city.toml written (managed; operator tail preserved)"

# --- 5. gap A: materialize the pack's membrane/ gate scripts ---------------------
# 'gc import' does NOT copy them, and the membrane-quest [steps.check] resolves
# path = "membrane/close-gate.sh" against the CITY root — without this copy the
# dispatcher quarantines every check step (standup.md §4, known gap A).
step "5. Materialize membrane/ into the city (gap A)"

[ -d "$PACK_SOURCE/membrane" ] || die "pack has no membrane/ dir: $PACK_SOURCE/membrane" \
  "point --pack-source at a complete agentops-membrane pack"
mkdir -p "$CITY/membrane"
cp "$PACK_SOURCE"/membrane/* "$CITY/membrane/"
chmod +x "$CITY"/membrane/*.sh
info "copied $(ls "$PACK_SOURCE/membrane" | wc -l | tr -d ' ') files to $CITY/membrane/ (+x on *.sh)"

# Materialize the quest scaffold template too: membrane/scaffold-quest.sh
# resolves quests/_template against the CITY root (same gap-A class — found by
# the first live E2E: scaffold died "template missing").
if [ -d "$PACK_SOURCE/quests/_template" ]; then
  mkdir -p "$CITY/quests/_template"
  cp "$PACK_SOURCE"/quests/_template/* "$CITY/quests/_template/"
  find "$CITY/quests/_template" -name "*.sh" -exec chmod +x {} +
  info "materialized quests/_template ($(ls "$PACK_SOURCE/quests/_template" | wc -l | tr -d ' ') files)"
fi

# Verify every membrane/ path the pack's formulas reference resolves in the city.
FORMULA_REFS="$(grep -ho 'membrane/[A-Za-z0-9._-]*' "$PACK_SOURCE"/formulas/*.toml 2>/dev/null | sort -u || true)"
for ref in $FORMULA_REFS; do
  case "$ref" in membrane/*.sh|membrane/*.jq) ;; *) continue ;; esac
  [ -f "$CITY/$ref" ] || die "formula-referenced script missing after materialization: $ref" \
    "re-run the installer; if it persists the pack's membrane/ dir is incomplete"
  case "$ref" in
    *.sh) [ -x "$CITY/$ref" ] || die "formula-referenced script not executable: $ref" \
      "chmod +x $CITY/$ref" ;;
  esac
  info "resolves: $ref"
done

# --- 6. residual gap B: pre-trust the codex home ---------------------------------
step "6. Pre-trust codex home (residual gap B)"

if [ -x "$PACK_SOURCE/scripts/pretrust-codex-home.sh" ]; then
  bash "$PACK_SOURCE/scripts/pretrust-codex-home.sh" "$CITY" >/dev/null || \
    die "pretrust-codex-home.sh failed" \
        "run: bash $PACK_SOURCE/scripts/pretrust-codex-home.sh $CITY"
else
  mkdir -p "$CODEX_HOME_DIR"
  [ -s "$CODEX_HOME_DIR/hooks.json" ] || printf '{}\n' > "$CODEX_HOME_DIR/hooks.json"
fi
info "CODEX_HOME pre-trusted: $CODEX_HOME_DIR (wired via [providers.codex].env)"
warn "agy has no file seed — run the antigravity provider once interactively and accept its trust prompt (one-time, per host)"

# --- 7. LAW 0 doctor check must be green BEFORE boot ------------------------------
step "7. LAW 0 structural check"

LAW0_CHECK="$PACK_SOURCE/doctor/law0-print-args/run.sh"
[ -f "$LAW0_CHECK" ] || die "pack law0 doctor check missing: $LAW0_CHECK" \
  "point --pack-source at a complete agentops-membrane pack"
if law0_out="$(cd "$CITY" && env GC_CITY_PATH="$CITY" GC_BIN="$GC_BIN" GC_HOME="$GC_HOME" \
    PATH="$GC_HOME/bin:$PATH" bash "$LAW0_CHECK" 2>&1)"; then
  info "law0-print-args: $law0_out"
else
  die "LAW 0 check failed: $law0_out" \
      "city.toml must carry print_args = [] on every claude/antigravity provider — re-run the installer (it writes them) and check for an operator-tail override"
fi

# --- 8. boot + store enforcement (F1: dolt_mode=server, native store) -------------
step "8. gc start + native store enforcement"

run_gc start >/dev/null || die "gc start failed" \
  "source $CITY/env.sh && gc start — then gc events / $GC_HOME/supervisor.log for the boot error"
info "gc start ok (supervisor, managed dolt, controller, named sessions)"

DOLT_MODE="$(run_in_city bd context --json 2>/dev/null | jq -r '.dolt_mode // empty' || true)"
if [ "$DOLT_MODE" != "server" ]; then
  die "bd is NOT in server mode (dolt_mode='${DOLT_MODE:-unset}') — gc would fall back to per-op bd subprocess calls (documented perf cliff) or the file backend" \
      "check the bd pack wiring: source $CITY/env.sh && gc status --json | jq .beads; inspect $CITY/.beads/metadata.json; never set GC_BEADS=file"
fi
info "bd context: dolt_mode=server"

STATUS_JSON="$(run_gc status --json 2>/dev/null || true)"
BEADS_STORE="$(printf '%s' "$STATUS_JSON" | jq -r '.beads.beads_store // empty' 2>/dev/null || true)"
STORE_ELIGIBLE="$(printf '%s' "$STATUS_JSON" | jq -r '.beads.native_store_eligible // false' 2>/dev/null || true)"
if [ "$BEADS_STORE" != "NativeDoltStore" ] || [ "$STORE_ELIGIBLE" != "true" ]; then
  die "gc status shows beads_store='${BEADS_STORE:-unknown}' native_store_eligible='${STORE_ELIGIBLE}' (must be NativeDoltStore + true)" \
      "version-contract or dolt wiring issue: verify bd/dolt versions above, then source $CITY/env.sh && gc doctor"
fi
info "gc status: NativeDoltStore, native_store_eligible=true"

# --- 9. sessions (F3: register AND verify) ----------------------------------------
LANE_TEMPLATES="${PACK_NAME}.verifier ${PACK_NAME}.agy-verifier"
REQUIRED_TEMPLATES=""
for d in "$PACK_SOURCE"/agents/*/; do
  [ -d "$d" ] || continue
  b="$(basename "$d")"
  [ "$b" = "_template" ] && continue
  REQUIRED_TEMPLATES="$REQUIRED_TEMPLATES ${PACK_NAME}.${b}"
done

if [ "$SKIP_SESSIONS" -eq 1 ]; then
  step "9. Sessions — SKIPPED (--skip-sessions)"
else
  step "9. Sessions (trinity + AGY): create + verify"

  session_templates() {
    run_gc session list --json 2>/dev/null | jq -r '.sessions[].template' 2>/dev/null | sort -u
  }

  LISTED="$(session_templates || true)"
  for t in $REQUIRED_TEMPLATES; do
    printf '%s\n' "$LISTED" | grep -qx "$t" && { info "session exists: $t"; continue; }
    case " $LANE_TEMPLATES " in
      *" $t "*)
        # Lanes are always-on [[named_session]]s registered by gc start — never
        # 'session new' them on top (template-as-both is a known gc footgun).
        ;;
      *)
        run_gc session new "$t" --no-attach >/dev/null 2>&1 || \
          die "gc session new $t failed" \
              "source $CITY/env.sh && gc session new $t --no-attach; then gc events for the spawn error"
        info "session created: $t"
        ;;
    esac
  done

  # Verify EVERY required template is registered (fail listing the missing).
  LISTED="$(session_templates || true)"
  MISSING=""
  for t in $REQUIRED_TEMPLATES; do
    printf '%s\n' "$LISTED" | grep -qx "$t" || MISSING="$MISSING $t"
  done
  if [ -n "$MISSING" ]; then
    die "sessions NOT registered:$MISSING" \
        "lanes: check the [[named_session]] blocks in city.toml survived gc start; others: source $CITY/env.sh && gc session new <template> --no-attach; then gc events"
  fi
  info "all required session templates registered:$(printf ' %s' $REQUIRED_TEMPLATES)"
fi

# --- 10. doctor gate: law0 + membrane-health must pass -----------------------------
step "10. gc doctor gate"

DOCTOR_JSON="$(run_gc doctor --json 2>/dev/null || true)"
[ -n "$DOCTOR_JSON" ] || die "'gc doctor --json' produced no report" \
  "source $CITY/env.sh && gc doctor"
doctor_status() {  # $1 = check-name suffix; .results shape per pack e2e.sh
  printf '%s' "$DOCTOR_JSON" | jq -r --arg s "$1" '
    (.results // .checks // [])
    | map(select(.name | endswith($s)))
    | if length == 0 then "MISSING"
      else (map(.status) | if all(. == "ok" or . == "pass") then "ok"
            else (map(select(. != "ok" and . != "pass"))[0]) end)
      end'
}
for suffix in law0-print-args membrane-health; do
  st="$(doctor_status "$suffix")"
  case "$st" in
    ok) info "doctor: $suffix ok" ;;
    MISSING) die "doctor check not discovered: $suffix" \
      "the pack import did not register its doctor checks — source $CITY/env.sh && gc import install && gc doctor" ;;
    *) die "doctor check NOT green: $suffix ($st)" \
      "source $CITY/env.sh && gc doctor — fix the named finding; the membrane is fail-closed until green" ;;
  esac
done

# --- summary -----------------------------------------------------------------------
step "Done — membrane city is up"
cat <<EOF

  city            : $CITY  (name: $CITY_NAME)
  GC_HOME         : $GC_HOME
  supervisor port : $SUPERVISOR_PORT
  tmux socket     : tmux -L $CITY_NAME
  gc binary       : $GC_BIN ($GC_VERSION)
  versions        : bd $BD_VERSION (== beads pin) · dolt $DOLT_VERSION (>= $DOLT_FLOOR)
  store           : NativeDoltStore (dolt_mode=server)
  pack            : $PACK_NAME (read-in-place from $PACK_SOURCE)
  close door      : $CITY/membrane/close-gate.sh (materialized, gap A closed)
  codex home      : $CODEX_HOME_DIR (pre-trusted)
  usage           : [usage] provider="local" (run rows populate; never gate on costs)
  sessions        : $( [ "$SKIP_SESSIONS" -eq 1 ] && printf 'SKIPPED (--skip-sessions)' || printf 'registered + verified:%s' "$(printf ' %s' $REQUIRED_TEMPLATES)" )

  Next commands:
    source $CITY/env.sh
    gc status && gc doctor
    gc sling ${PACK_NAME}.builder <quest-bead-id> --on membrane-quest \\
      --var quest=<slug> --var task="<build task>"     # sling a canary quest

EOF
