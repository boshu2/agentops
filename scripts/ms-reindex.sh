#!/usr/bin/env bash
# ms-reindex.sh — mechanize the ms reindex law (age-22g0).
#
# WHY: a running `ms mcp serve` NEVER reopens its SQLite/tantivy handles after an
# on-disk rebuild. It follows renamed inodes into the backup dir, serving
# pre-wipe ORPHAN ids and mis-landing writes while returning recorded:true
# (lsof-verified 2026-07-02: 63 fds into ms.bak-.../wipe2/). The prior
# countermeasure was a prose footgun row in skills/ms/SKILL.md — proven INERT (a
# doc instruction to change behavior does not change behavior; graphify A/B
# 2026-06-30). This script encodes the law as executable tooling.
#
# ORDERING (measured — DELIBERATELY not the bead's literal a=index,b=sweep,c=probe):
#   A live `ms mcp serve` holds the tantivy WRITER lock, so `ms index` fails with
#   "Index opened in read-only mode" BEFORE any post-index sweep could run
#   (measured live age-22g0). And a signal-killed server leaves a STALE writer
#   lock that blocks the next index too. So the correct, law-honoring sequence is:
#
#     (1) SWEEP + TERM every `ms mcp serve` FIRST — releases their handles and
#         removes the inode-followers so none survive the rebuild (the law's
#         whole point). Discovery uses `ps -Ao pid,lstart,command | grep -F`
#         because plain `ps aux | grep` false-negatives on these (measured).
#     (2) CLEAR the stale lock a killed/crashed writer leaves (a dead-pid lock is
#         safe to delete — SKILL.md footgun table); refuse if a LIVE pid holds it.
#     (3) REBUILD the index and PROVE it took (indexed >= MIN, errors <= MAX).
#     (4) SWEEP again — belt-and-suspenders: kill any server that raced in during
#         the rebuild (literal "kill after rebuild").
#     (5) PROBE a FRESH one-shot server over stdio JSON-RPC and assert it serves a
#         real skill id and NO orphan-class id.
#
# Any step failing exits nonzero (fail loud). This is THE way to reindex ms —
# never run bare `ms index` and leave servers up.
#
# Portability: Mac-first (Bo's orchestration node) but pure bash + POSIX `ps`;
# no BSD-only flags. `timeout`/`gtimeout` used when present, else the probe still
# terminates on stdin EOF (ms mcp serve exits when its stdin closes).

set -euo pipefail

MS_BIN="${MS_BIN:-ms}"
MIN_INDEXED="${MS_REINDEX_MIN_INDEXED:-170}"
MAX_ERRORS="${MS_REINDEX_MAX_ERRORS:-1}"          # the intentional _fixtures/bad-skill
SERVE_PATTERN="${MS_REINDEX_SERVE_PATTERN:-ms mcp serve}"
PROBE_QUERY="${MS_REINDEX_PROBE_QUERY:-flaky concurrent test}"
PROBE_EXPECT_ID="${MS_REINDEX_PROBE_EXPECT_ID:-deadlock-finder-and-fixer}"
# orphan-class ids that only a stale / pre-wipe server would surface:
ORPHAN_IDS="${MS_REINDEX_ORPHAN_IDS:-expected-all-pass node-env-is-not-production}"

log()  { printf '[ms-reindex] %s\n' "$*" >&2; }
die()  { printf '[ms-reindex] FATAL: %s\n' "$*" >&2; exit 1; }

# Snapshot of running processes. Overridable via MS_REINDEX_PS_FIXTURE for tests.
ps_snapshot() {
  if [ -n "${MS_REINDEX_PS_FIXTURE:-}" ]; then
    cat "$MS_REINDEX_PS_FIXTURE"
  else
    ps -Ao pid,lstart,command
  fi
}

# Pids of live `ms mcp serve` processes. `grep -v ' grep '` drops the matcher
# itself; the leading pid is field 1 of `pid,lstart,command` (lstart is 5 fields).
ms_serve_pids() {
  ps_snapshot | grep -F "$SERVE_PATTERN" | grep -v ' grep ' | awk '{print $1}' | grep -E '^[0-9]+$' || true
}

# Resolve ms's data dir (holds ms.db + index/). MS_DATA_DIR overrides; else the
# platform default (macOS Application Support, else XDG). Prefer a candidate that
# actually contains ms.db.
ms_data_dir() {
  if [ -n "${MS_DATA_DIR:-}" ]; then printf '%s' "$MS_DATA_DIR"; return; fi
  local candidates=() d
  case "$(uname -s)" in
    Darwin) candidates+=("$HOME/Library/Application Support/ms") ;;
  esac
  candidates+=("${XDG_DATA_HOME:-$HOME/.local/share}/ms")
  for d in "${candidates[@]}"; do
    if [ -f "$d/ms.db" ]; then printf '%s' "$d"; return; fi
  done
  printf '%s' "${candidates[0]}"
}

run_with_timeout() {
  local secs="$1"; shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$secs" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$secs" "$@"
  else
    "$@"
  fi
}

# --- Sweep: TERM (then KILL) every ms mcp serve, confirm death -----------------
step_sweep() {
  local pids pid alive
  pids="$(ms_serve_pids)"
  if [ -z "$pids" ]; then
    log "sweep OK — no live '$SERVE_PATTERN' processes"
    return 0
  fi
  log "sweep — TERMing '$SERVE_PATTERN' pids: $(printf '%s' "$pids" | tr '\n' ' ')"
  for pid in $pids; do
    kill -TERM "$pid" 2>/dev/null || true
  done
  sleep 1
  for pid in $pids; do
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  done
  sleep 1
  alive=""
  for pid in $pids; do
    if kill -0 "$pid" 2>/dev/null; then
      alive="$alive $pid"
    fi
  done
  [ -z "$alive" ] || die "sweep failed — server(s) still alive after TERM+KILL:$alive"
  log "sweep OK — all '$SERVE_PATTERN' processes terminated"
}

# --- Clear a stale writer lock left by a killed/crashed server ------------------
# Safe ONLY after step_sweep confirmed no live server. Refuses if ms.lock is held
# by a still-LIVE pid (a concurrent ms op we must not stomp).
step_clear_stale_locks() {
  local data lock pid
  data="$(ms_data_dir)"
  lock="$data/ms.lock"
  if [ -f "$lock" ]; then
    pid="$(jq -r '.pid // empty' "$lock" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      die "ms.lock held by LIVE pid $pid (concurrent ms op?) — refusing to clear; resolve manually"
    fi
  fi
  rm -f "$lock" "$data/index/.tantivy-writer.lock" "$data/index/.tantivy-meta.lock" 2>/dev/null || true
  log "cleared stale locks under $data (if any)"
}

# --- Rebuild the index, assert it took -----------------------------------------
step_index() {
  local out indexed errors
  if ! out="$("$MS_BIN" index -O json 2>/dev/null)"; then
    die "ms index exited nonzero even after sweep+lock-clear — check: ms doctor"
  fi
  [ -n "$out" ] || die "ms index produced no JSON on stdout"
  indexed="$(printf '%s' "$out" | jq -r '.indexed // empty')"
  errors="$(printf '%s' "$out" | jq -r '(.errors // []) | length')"
  [ -n "$indexed" ] || die "ms index JSON missing .indexed: $out"
  case "$indexed" in ''|*[!0-9]*) die "ms index .indexed not numeric: '$indexed'";; esac
  if [ "$indexed" -lt "$MIN_INDEXED" ]; then
    die "indexed=$indexed < required $MIN_INDEXED — index looks empty/broken"
  fi
  if [ "$errors" -gt "$MAX_ERRORS" ]; then
    die "errors=$errors > allowed $MAX_ERRORS: $(printf '%s' "$out" | jq -c '.errors')"
  fi
  log "index OK — indexed=$indexed errors=$errors (min=$MIN_INDEXED max_err=$MAX_ERRORS)"
}

# --- Probe a fresh one-shot server over stdio JSON-RPC -------------------------
step_probe() {
  local resp orphan
  resp="$(printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"ms-reindex-probe","version":"1.0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"search\",\"arguments\":{\"query\":\"${PROBE_QUERY}\"}}}" \
    | run_with_timeout 30 "$MS_BIN" mcp serve 2>/dev/null)" || true
  [ -n "$resp" ] || die "probe — ms mcp serve returned no output over stdio"
  printf '%s' "$resp" | grep -Fq "$PROBE_EXPECT_ID" \
    || die "probe — expected real id '$PROBE_EXPECT_ID' NOT in results for query '$PROBE_QUERY' (index empty/stale?)"
  for orphan in $ORPHAN_IDS; do
    if printf '%s' "$resp" | grep -Fq "$orphan"; then
      die "probe — ORPHAN id '$orphan' present: a stale/pre-wipe index is being served (the law's exact failure)"
    fi
  done
  log "probe OK — '$PROBE_EXPECT_ID' served, no orphan ids ($ORPHAN_IDS)"
}

usage() {
  cat >&2 <<'EOF'
ms-reindex.sh — sweep every ms mcp serve, reindex ms, probe a fresh server.

Usage:
  ms-reindex.sh                      Full law: sweep -> clear-locks -> index -> sweep -> probe.
  ms-reindex.sh --print-serve-pids   Print discovered 'ms mcp serve' pids, exit.
  ms-reindex.sh --sweep-only         Run only the sweep step (no index/probe).
  ms-reindex.sh -h | --help

Env overrides: MS_BIN, MS_DATA_DIR, MS_REINDEX_MIN_INDEXED (170),
  MS_REINDEX_MAX_ERRORS (1), MS_REINDEX_SERVE_PATTERN, MS_REINDEX_PROBE_QUERY,
  MS_REINDEX_PROBE_EXPECT_ID, MS_REINDEX_ORPHAN_IDS, MS_REINDEX_PS_FIXTURE (test hook).
EOF
}

main() {
  case "${1:-run}" in
    run)
      step_sweep              # (1) kill servers first — they hold the writer lock
      step_clear_stale_locks  # (2) remove the stale lock a killed writer leaves
      step_index              # (3) clean rebuild + assert it took
      step_sweep              # (4) kill anything that raced in during the rebuild
      step_probe              # (5) fresh server serves real ids, no orphans
      log "DONE — servers swept, index rebuilt, fresh server verified."
      ;;
    --print-serve-pids) ms_serve_pids ;;
    --sweep-only)       step_sweep ;;
    -h|--help)          usage ;;
    *)                  usage; exit 2 ;;
  esac
}

main "$@"
