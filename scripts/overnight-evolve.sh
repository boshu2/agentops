#!/usr/bin/env bash
# Overnight evolve loop — kicked off manually before bed.
#
# Runs `ao evolve --quality --max-cycles=10 --test-first` under an 8h timeout,
# tees output to ~/.agents/overnight/<DATE>/run.log, writes a morning
# summary.md on exit (any cause).
#
# Plan: ~/dev/agentops/.agents/plans/2026-05-01-overnight-evolve-loop.md
#
# Usage:
#   bash scripts/overnight-evolve.sh
#
# Kill switch from another terminal:
#   touch ~/.agents/overnight/<DATE>/kill-switch
#
# Each evolve cycle's pre-flight checks AGENTOPS_OVERNIGHT_KILL_SWITCH; if the
# file exists, the cycle exits cleanly with <promise>BLOCKED</promise>.

set -uo pipefail

DATE="$(date +%Y-%m-%d-%H%M)"
DIR="$HOME/.agents/overnight/$DATE"
KILL_SWITCH="$DIR/kill-switch"
LOG="$DIR/run.log"
SUMMARY="$DIR/summary.md"

mkdir -p "$DIR"

# Pre-flight: must run from agentops repo, clean tree
cd "$HOME/dev/agentops" || { echo "no agentops repo at ~/dev/agentops" >&2; exit 1; }

# Working tree clean — IGNORE tool-auto-mutated paths (matches preflight policy).
# .agents/ao/last-processed mutates whenever any ao command runs.
# .agents/findings/registry.jsonl mutates on bd ready / ao lookup.
# .agents/ao/citations.jsonl mutates on ao metrics cite.
DIRTY="$(git status --porcelain | \
    grep -vE '\.agents/ao/last-processed|\.agents/findings/registry\.jsonl|\.agents/ao/citations\.jsonl' \
    || true)"
if [ -n "$DIRTY" ]; then
    {
        echo "# Overnight Evolve — REFUSED"
        echo
        echo "Working tree not clean (excluding tool-auto-mutated paths)."
        echo "Stash or commit before launching."
        echo
        printf '%s\n' "$DIRTY"
    } | tee "$SUMMARY"
    exit 1
fi

START_SHA="$(git rev-parse HEAD)"
START_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
START_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Export kill-switch path so each evolve cycle's hooks can check it
export AGENTOPS_OVERNIGHT_KILL_SWITCH="$KILL_SWITCH"
export AGENTOPS_OVERNIGHT_LOG_DIR="$DIR"

# 8h timeout — gtimeout from coreutils on Mac, falls back to timeout on Linux
TIMEOUT_BIN=gtimeout
command -v "$TIMEOUT_BIN" >/dev/null 2>&1 || TIMEOUT_BIN=timeout
if ! command -v "$TIMEOUT_BIN" >/dev/null 2>&1; then
    echo "neither gtimeout nor timeout found on PATH; install coreutils via brew" >&2
    exit 1
fi

{
    echo "===================================================================="
    echo "Overnight evolve start: $START_TIME"
    echo "  SHA:    $START_SHA"
    echo "  branch: $START_BRANCH"
    echo "  log:    $LOG"
    echo "  kill:   touch $KILL_SWITCH (from another terminal)"
    echo "  budget: 8h (timeout via $TIMEOUT_BIN)"
    echo "===================================================================="
} | tee "$LOG"

# The actual evolve invocation. Stdin closed (no interactive prompts).
# ao evolve v2 supervisor flags. --max-cycles caps; --ensure-cleanup runs
# stale-run cleanup after each cycle. The skill-doc --quality/--test-first
# flags belong to the Claude-side /evolve skill invocation, not the Go CLI.
"$TIMEOUT_BIN" 8h ao evolve --max-cycles 10 --ensure-cleanup </dev/null 2>&1 | tee -a "$LOG"
EXIT_CODE="${PIPESTATUS[0]}"

END_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
END_SHA="$(git rev-parse HEAD)"
COMMITS_MADE="$(git rev-list "$START_SHA..$END_SHA" --count 2>/dev/null || echo "?")"
FILES_CHANGED="$(git diff --name-only "$START_SHA" "$END_SHA" 2>/dev/null | wc -l | tr -d ' ')"

KILL_FIRED="no"
[ -f "$KILL_SWITCH" ] && KILL_FIRED="YES"

EXIT_NOTE="clean"
case "$EXIT_CODE" in
    0)   EXIT_NOTE="clean" ;;
    124) EXIT_NOTE="hit 8h timeout (gtimeout SIGTERM)" ;;
    *)   EXIT_NOTE="non-zero ($EXIT_CODE) — see log" ;;
esac

{
    echo "# Overnight Evolve Summary — $DATE"
    echo
    echo "- **Start**: $START_TIME (SHA=$START_SHA, branch=$START_BRANCH)"
    echo "- **End**:   $END_TIME (SHA=$END_SHA)"
    echo "- **Exit code**: $EXIT_CODE — $EXIT_NOTE"
    echo "- **Commits made**: $COMMITS_MADE"
    echo "- **Files changed**: $FILES_CHANGED"
    echo "- **Kill switch fired**: $KILL_FIRED"
    echo "- **Log**: \`$LOG\`"
    echo
    echo "## Commits"
    echo
    echo '```'
    git log --oneline "$START_SHA..$END_SHA" 2>/dev/null || echo "no new commits"
    echo '```'
    echo
    echo "## Files changed"
    echo
    echo '```'
    git diff --stat "$START_SHA" "$END_SHA" 2>/dev/null | head -50
    echo '```'
    echo
    echo "## Fitness"
    echo
    echo '```json'
    ao goals measure --json 2>/dev/null | jq -c '.gates // .' 2>/dev/null || echo "ao goals measure unavailable"
    echo '```'
    echo
    echo "## Next look"
    echo
    echo "- \`cat $LOG\` for cycle-level detail"
    echo "- \`git diff $START_SHA..HEAD\` for full review"
    echo "- \`bd ready\` for new queue items spawned by post-mortem harvest"
} > "$SUMMARY"

echo
echo "==== summary written: $SUMMARY ===="
cat "$SUMMARY"
exit "$EXIT_CODE"
