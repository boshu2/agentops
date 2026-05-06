#!/usr/bin/env bash
# test-evolve-cycle-smoke.sh — End-to-end /evolve cycle smoke (soc-k3fa).
#
# Asserts that one bounded `ao evolve` cycle lands a real commit on the current
# branch AND creates no new orphaned commits. Designed to catch the failure
# mode from the 2026-05-06 overnight run, where 4 cycles produced 0 commits on
# main while orphaning 3 real commits in detached worktrees that were
# auto-cleaned.
#
# Pass criteria (both required):
#   1. `git rev-parse HEAD` advances by at least one commit during the run.
#   2. `git fsck --unreachable | grep -c "^unreachable commit"` does not grow.
#
# This script is opt-in. It is wired into `scripts/pre-push-gate.sh` behind
# the `--smoke-evolve` flag because a single bounded cycle takes 15-30 minutes;
# normal push gates skip it. Run manually:
#
#   scripts/test-evolve-cycle-smoke.sh
#   scripts/test-evolve-cycle-smoke.sh --max-cycles 2 --timeout 3600
#
# Environment:
#   AO_BIN           Path to ao binary (default: $(command -v ao)).
#   SMOKE_GOAL       Optional explicit goal string to pass to ao evolve.
#   SMOKE_BRANCH     Branch to track HEAD on (default: current branch).
#   SMOKE_KEEP_LOG   1 to keep the run log on success (default: 0 = delete).
#
# Exit codes:
#   0  smoke passed (commit landed, no new orphans)
#   1  smoke failed (no commit landed, or new orphans appeared, or command failed)
#   2  argument parse error or environment misconfiguration

set -euo pipefail

usage() {
    cat <<'EOF'
Usage: scripts/test-evolve-cycle-smoke.sh [--max-cycles N] [--timeout SECONDS] [--goal GOAL]

Run one bounded `ao evolve` cycle and assert a commit lands with no new
orphaned commits. Intended for opt-in pre-push validation of the cycle
lifecycle (soc-k3fa / mc-m3.5-pre4).

Options:
  --max-cycles N       Cycles to run (default: 1).
  --timeout SECONDS    Hard timeout for the evolve invocation (default: 1800).
  --goal STRING        Explicit goal string for the cycle (default: queue-driven).
  -h, --help           Show this message and exit.

Environment overrides: AO_BIN, SMOKE_GOAL, SMOKE_BRANCH, SMOKE_KEEP_LOG.
EOF
}

MAX_CYCLES=1
TIMEOUT_SECONDS=1800
GOAL="${SMOKE_GOAL:-}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --max-cycles)
            MAX_CYCLES="${2:-}"
            shift 2
            ;;
        --timeout)
            TIMEOUT_SECONDS="${2:-}"
            shift 2
            ;;
        --goal)
            GOAL="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown arg: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if ! [[ "$MAX_CYCLES" =~ ^[0-9]+$ ]] || [[ "$MAX_CYCLES" -lt 1 ]]; then
    echo "test-evolve-cycle-smoke: --max-cycles must be a positive integer (got: $MAX_CYCLES)" >&2
    exit 2
fi

if ! [[ "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT_SECONDS" -lt 60 ]]; then
    echo "test-evolve-cycle-smoke: --timeout must be an integer >= 60 (got: $TIMEOUT_SECONDS)" >&2
    exit 2
fi

AO_BIN="${AO_BIN:-$(command -v ao || true)}"
if [[ -z "$AO_BIN" ]]; then
    echo "test-evolve-cycle-smoke: ao binary not found on PATH (set AO_BIN to override)" >&2
    exit 2
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "test-evolve-cycle-smoke: not inside a git work tree" >&2
    exit 2
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

SMOKE_BRANCH="${SMOKE_BRANCH:-$(git branch --show-current)}"
if [[ -z "$SMOKE_BRANCH" ]]; then
    echo "test-evolve-cycle-smoke: refusing to run on a detached HEAD; checkout a branch first" >&2
    exit 2
fi

count_orphans() {
    git fsck --unreachable 2>/dev/null | grep -c '^unreachable commit' || true
}

HEAD_BEFORE="$(git rev-parse HEAD)"
ORPHANS_BEFORE="$(count_orphans)"
LOG_DIR="$REPO_ROOT/.agents/evolve/smoke"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/$(date -u +%Y%m%dT%H%M%SZ)-cycle.log"

echo "test-evolve-cycle-smoke: starting"
echo "  branch:           $SMOKE_BRANCH"
echo "  HEAD before:      $HEAD_BEFORE"
echo "  orphan commits:   $ORPHANS_BEFORE"
echo "  max-cycles:       $MAX_CYCLES"
echo "  timeout (sec):    $TIMEOUT_SECONDS"
echo "  log file:         $LOG_FILE"

set +e
if [[ -n "$GOAL" ]]; then
    timeout "$TIMEOUT_SECONDS" "$AO_BIN" evolve "$GOAL" \
        --max-cycles="$MAX_CYCLES" \
        --landing-policy=commit \
        --auto-clean \
        --gate-policy=best-effort \
        2>&1 | tee "$LOG_FILE"
else
    timeout "$TIMEOUT_SECONDS" "$AO_BIN" evolve \
        --max-cycles="$MAX_CYCLES" \
        --landing-policy=commit \
        --auto-clean \
        --gate-policy=best-effort \
        2>&1 | tee "$LOG_FILE"
fi
EVOLVE_RC=${PIPESTATUS[0]}
set -e

HEAD_AFTER="$(git rev-parse HEAD)"
ORPHANS_AFTER="$(count_orphans)"

echo ""
echo "test-evolve-cycle-smoke: results"
echo "  HEAD after:       $HEAD_AFTER"
echo "  orphan commits:   $ORPHANS_AFTER"
echo "  evolve exit:      $EVOLVE_RC"

FAIL=0

if [[ "$HEAD_BEFORE" == "$HEAD_AFTER" ]]; then
    echo "FAIL: HEAD did not advance — no commit landed during the cycle"
    FAIL=1
fi

if [[ "$ORPHANS_AFTER" -gt "$ORPHANS_BEFORE" ]]; then
    DELTA=$((ORPHANS_AFTER - ORPHANS_BEFORE))
    echo "FAIL: $DELTA new orphaned commit(s) appeared during the cycle"
    echo "  inspect with: git fsck --unreachable | head -20"
    FAIL=1
fi

if [[ "$EVOLVE_RC" -ne 0 ]] && [[ "$FAIL" -eq 0 ]]; then
    # Cycle reported non-zero exit but invariants hold (commit landed, no orphans).
    # Surface as a warning rather than a hard fail — the supervisor may exit
    # non-zero on timer/queue-empty even when cycle work succeeded.
    echo "WARN: ao evolve exited $EVOLVE_RC but invariants hold; treating as PASS"
fi

if [[ "$FAIL" -eq 0 ]]; then
    echo "PASS: cycle landed a commit and created no new orphans"
    if [[ "${SMOKE_KEEP_LOG:-0}" != "1" ]]; then
        rm -f "$LOG_FILE"
    fi
    exit 0
fi

echo ""
echo "test-evolve-cycle-smoke: FAILED — log retained at $LOG_FILE"
exit 1
