#!/usr/bin/env bash
# mine-all-sessions.sh — E6.4 (age-membrane-memory-arch-tz2s.6.4): run the BUILT
# `ao provenance mine-session` miner over the REAL session corpus and populate the
# local provenance event store. Anti-cathedral: the incremental miner (E6.2) + its
# e2e test (E6.3) are landed, but the miner had never RUN on real logs — a miner
# that never runs makes no contact. This makes contact.
#
# Design (cross-family council, 2026-06-22): the mined stream is PER-INFERENCE,
# high-volume (1951 events from one session), schema agentops-provenance-mine-
# event.v1 — distinct from the curated committed PROV-O graph (docs/provenance/
# ledger.jsonl). So it lands in a LOCAL, gitignored EVENT STORE (the membrane-
# memory plan's event memory), NOT the committed graph (which it would bloat +
# churn). A thin script first; promote to `ao provenance mine-all` only if the
# real-data loop proves useful.
#
# Soundness (council footguns): deterministic session ordering; explicit store +
# per-session state paths; mine each session to a TEMP file, then append DURABLY,
# rolling back that session's incremental state if the durable append fails (so a
# crash never advances state past unstored events); a single-writer lock; a summary
# counts artifact.
#
# Usage: mine-all-sessions.sh [<sessions-dir>]   (default: this project's transcripts)
# Env:   AGENTOPS_AO_BIN  ao binary (default: ao on PATH)
# Exit:  0 = every session mined cleanly (or nothing to mine) · 1 = a hard error OR
#        >=1 session failed to mine (see the summary's .failed) · 2 = lock held.
set -uo pipefail

AO="${AGENTOPS_AO_BIN:-ao}"
SESSIONS_DIR="${1:-$HOME/.claude/projects/-Users-bo-dev-agentops}"
STORE_DIR=".agents/provenance"
STORE="$STORE_DIR/mine-events.jsonl"
STATE_DIR="$STORE_DIR/state"
SUMMARY="$STORE_DIR/mine-summary.json"
LOCK="$STORE_DIR/.mine.lock"

command -v "$AO" >/dev/null 2>&1 || { echo "mine-all: ao not found ($AO)" >&2; exit 1; }
[ -d "$SESSIONS_DIR" ] || { echo "mine-all: no sessions dir $SESSIONS_DIR" >&2; exit 1; }
mkdir -p "$STORE_DIR" "$STATE_DIR"

# Single-writer lock (atomic mkdir; never two miners appending the same store).
if ! mkdir "$LOCK" 2>/dev/null; then
  echo "mine-all: another miner holds the lock ($LOCK) — skipping" >&2
  exit 2
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

sessions=0 events=0 failed=0
# rollback_state() restores a session's incremental --state after a failed mine/append
# so a re-run re-mines the unstored events. TWO cases (the cross-family pawl caught the
# second, 2026-06-22): (a) state PRE-EXISTED this run -> restore its backup; (b) state was
# NEWLY created by the miner this run -> DELETE it. Leaving an advanced, never-backed-up
# state would silently skip the events that never reached the store = data loss.
rollback_state() { # $1=state $2=bak $3=had_state
  if [ "$3" -eq 1 ]; then mv "$2" "$1"; else rm -f "$1"; fi
}
# Deterministic ordering so the store + summary are reproducible.
while IFS= read -r sess; do
  [ -n "$sess" ] || continue
  sid="$(basename "$sess" .jsonl)"
  state="$STATE_DIR/$sid.json"
  bak="$state.bak"
  rm -f "$bak" # clear any stale backup left by a crashed prior run before snapshotting
  had_state=0
  [ -f "$state" ] && { cp "$state" "$bak"; had_state=1; }
  tmp="$(mktemp "${TMPDIR:-/tmp}/mine-all.XXXXXX")" || { failed=$((failed+1)); continue; }
  # mine NEW events (incremental via --state); state advances + events go to stdout.
  if "$AO" provenance mine-session --file "$sess" --state "$state" --json >"$tmp" 2>/dev/null; then
    # The --json miner emits JSONL: one event object per line, each carrying exactly one
    # "schema_version" -> matching-line count == event count for this producer's contract.
    n="$(grep -c '"schema_version"' "$tmp" 2>/dev/null)"; n="${n:-0}"
    appended=1
    [ "$n" -gt 0 ] && { cat "$tmp" >>"$STORE" || appended=0; } # durable append
    if [ "$appended" -eq 1 ]; then
      events=$((events + n))
      sessions=$((sessions+1))
    else
      rollback_state "$state" "$bak" "$had_state"
      failed=$((failed+1))
    fi
  else
    rollback_state "$state" "$bak" "$had_state"
    failed=$((failed+1))
  fi
  rm -f "$tmp" "$bak" # bak is already gone if a rollback restored it; no-op otherwise
done < <(find "$SESSIONS_DIR" -maxdepth 1 -name '*.jsonl' 2>/dev/null | sort)

store_total="$(grep -c '"schema_version"' "$STORE" 2>/dev/null)"; store_total="${store_total:-0}"
# Build the summary with jq when present so paths with quotes/backslashes can't produce
# invalid JSON (pawl finding); a printf fallback keeps the script dependency-free.
if command -v jq >/dev/null 2>&1; then
  jq -n --arg dir "$SESSIONS_DIR" --arg store "$STORE" \
    --argjson sessions "$sessions" --argjson new "$events" \
    --argjson total "$store_total" --argjson failed "$failed" \
    '{sessions_dir:$dir,sessions_mined:$sessions,new_events:$new,store_total:$total,failed:$failed,store:$store}' \
    >"$SUMMARY"
else
  printf '{"sessions_dir":"%s","sessions_mined":%d,"new_events":%d,"store_total":%d,"failed":%d,"store":"%s"}\n' \
    "$SESSIONS_DIR" "$sessions" "$events" "$store_total" "$failed" "$STORE" >"$SUMMARY"
fi
echo "E6.4 mine-all: $sessions session(s) mined, +$events new event(s); store now holds $store_total event(s) ($failed failed) -> $STORE"
# Fail LOUD if any session failed to mine: the exit code is the automation signal, not
# just the summary's .failed count (pawl finding — an unconditional exit 0 false-greens).
[ "$failed" -eq 0 ] || exit 1
exit 0
