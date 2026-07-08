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
  echo "check-corpus-freshness: SKIP (no snapshot dir at $SNAPSHOT_DIR — 'make build-flywheel' restores 'ao corpus snapshot' to initialize)"
  exit 0
fi

# Probe whether the ONLY repair for a stale snapshot -- `ao corpus snapshot` --
# is actually reachable in the ao binary on this box. That command is compiled
# behind `//go:build flywheel` (cli/cmd/ao/corpus_snapshot.go), so it is ABSENT
# from the default shipped binary agents build. A >7d FAIL there is unfixable by
# design, so it must degrade to an HONEST structural SKIP (exit 75), not a hard
# FAIL that forces AGENTOPS_CORPUS_FRESHNESS_SKIP=1 on every land. This is a D11
# fitness gate on an ADR-0012-archived surface (off the pawl/provenance
# membrane), so SKIP-when-unfixable is honest, not a silenced pass.
#
# GUARD: report "absent" (skip) ONLY on genuine tool-absence -- a nonzero exit
# PLUS a cobra unknown/removed-command hint. A transient ao-not-on-PATH or a
# broken binary reports "present" so we fall through to the real FAIL. And a
# `flywheel` build (where the tool IS present; `--help` exits 0) also reports
# "present", so a genuine >7d staleness there still FAILs.
repair_tool_absent() {
  local ao=""
  if [ -n "${AO_BIN:-}" ]; then
    ao="$AO_BIN"
  elif command -v ao >/dev/null 2>&1; then
    ao="$(command -v ao)"
  else
    local repo_root
    repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
    [ -x "$repo_root/cli/bin/ao" ] && ao="$repo_root/cli/bin/ao"
  fi
  # Unresolvable ao (transient / not built) is NOT genuine tool-absence -> FAIL.
  [ -n "$ao" ] || return 1

  local out rc
  out="$("$ao" corpus snapshot --help 2>&1)"
  rc=$?
  # Tool present (`--help` exits 0 under a flywheel build) -> keep FAIL-on-stale.
  [ "$rc" -eq 0 ] && return 1
  # Nonzero + a cobra unknown/removed-command hint -> genuinely absent from binary.
  printf '%s' "$out" | grep -Eqi 'unknown command|removed from ao|removed command'
}

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
  # A stale snapshot is only actionable if the repair tool is in the binary.
  # When it is not (default non-flywheel build), degrade to a structural SKIP
  # (exit 75 -> GateStatusSkip in cli/internal/gates/scriptrunner.go) instead of
  # a hard FAIL that would force AGENTOPS_CORPUS_FRESHNESS_SKIP=1 on every land.
  if repair_tool_absent; then
    echo "check-corpus-freshness: SKIP — newest snapshot is ${AGE_DAYS}d old (>${THRESHOLD_DAYS}d) but the only repair ('ao corpus snapshot') is absent from this binary (archived behind the 'flywheel' build tag). 'make build-flywheel' restores it."
    echo "  path: $LATEST"
    exit 75
  fi
  echo "check-corpus-freshness: FAIL — newest snapshot is ${AGE_DAYS}d old (>${THRESHOLD_DAYS}d threshold)"
  echo "  path: $LATEST"
  echo "  fix:  make build-flywheel to restore 'ao corpus snapshot', or set AGENTOPS_CORPUS_FRESHNESS_SKIP=1"
  exit 1
fi

echo "check-corpus-freshness: PASS (newest snapshot ${AGE_DAYS}d old, threshold ${THRESHOLD_DAYS}d)"
exit 0
