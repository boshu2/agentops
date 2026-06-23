#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, resilience-patterns, ai-assisted-dev]
# Fail if the newest corpus snapshot under $AGENTOPS_CORPUS_SNAPSHOT_DIR
# (or ~/.agentops/corpus-snapshots/) is older than $AGENTOPS_CORPUS_FRESHNESS_DAYS
# days (default 7). Skip cleanly when no snapshots exist (greenfield boxes).
#
# Pair: GOALS.md gate id corpus-freshness (weight 4).
# Companion CLI: ao corpus snapshot, ao corpus restore.

set -uo pipefail

THRESHOLD_DAYS="${AGENTOPS_CORPUS_FRESHNESS_DAYS:-7}"
SNAPSHOT_DIR="${AGENTOPS_CORPUS_SNAPSHOT_DIR:-$HOME/.agentops/corpus-snapshots}"

# Operator override: SKIP=1 short-circuits with PASS (used by CI on fresh boxes
# or in pre-flight environments that don't carry a snapshot dir).
if [ "${AGENTOPS_CORPUS_FRESHNESS_SKIP:-0}" = "1" ]; then
  echo "check-corpus-freshness: SKIP (AGENTOPS_CORPUS_FRESHNESS_SKIP=1)"
  exit 0
fi

if [ ! -d "$SNAPSHOT_DIR" ]; then
  echo "check-corpus-freshness: SKIP (no snapshot dir at $SNAPSHOT_DIR — run 'ao corpus snapshot' to initialize)"
  exit 0
fi

# `-printf` is GNU-only; on BSD/macOS find errors ("unknown primary") -> empty
# LATEST -> this check silently false-SKIPs even when snapshots exist. Replace
# with a portable, TRUE-global mtime sort: emit "<mtime>\t<path>" per match (BSD
# `stat -f %m` / GNU `stat -c %Y` fallback), then a single global `sort -n | tail`
# (ascending + tail = newest; tail consumes all input, avoiding the head+pipefail
# SIGPIPE trap; no per-batch mis-sort). No `-type f` — match the original so
# symlinked snapshots still count; zero matches -> empty LATEST (correct SKIP).
LATEST=$(find "$SNAPSHOT_DIR" -maxdepth 1 -name '*.tar.gz' 2>/dev/null \
  | while IFS= read -r _f; do
      printf '%s\t%s\n' "$(stat -f %m "$_f" 2>/dev/null || stat -c %Y "$_f" 2>/dev/null)" "$_f"
    done | sort -n | tail -n 1 | cut -f2-)

if [ -z "$LATEST" ]; then
  echo "check-corpus-freshness: SKIP (no *.tar.gz snapshots under $SNAPSHOT_DIR)"
  exit 0
fi

NOW=$(date +%s)
MTIME=$(stat -c %Y "$LATEST" 2>/dev/null || stat -f %m "$LATEST" 2>/dev/null)
AGE_SECS=$(( NOW - MTIME ))
AGE_DAYS=$(( AGE_SECS / 86400 ))
THRESHOLD_SECS=$(( THRESHOLD_DAYS * 86400 ))

if [ "$AGE_SECS" -gt "$THRESHOLD_SECS" ]; then
  echo "check-corpus-freshness: FAIL — newest snapshot is ${AGE_DAYS}d old (>${THRESHOLD_DAYS}d threshold)"
  echo "  path: $LATEST"
  echo "  fix:  ao corpus snapshot"
  exit 1
fi

echo "check-corpus-freshness: PASS (newest snapshot ${AGE_DAYS}d old, threshold ${THRESHOLD_DAYS}d)"
exit 0
