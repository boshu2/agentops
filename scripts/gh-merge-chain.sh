#!/usr/bin/env bash
# gh-merge-chain.sh — sequence N PRs through auto-merge, calling
# `update-branch` whenever a predecessor merges and successors fall BEHIND.
#
# Closes F3 from .agents/learnings/2026-05-18-auto-merge-needs-update-branch-when-main-moves.md
#
# `gh pr merge --squash --auto` does not auto-rebase when main moves
# underneath the PR's head. The branch goes BEHIND and auto-merge stalls
# until `gh api repos/<owner>/<repo>/pulls/<n>/update-branch -X PUT` is
# called. This helper enables auto-merge on each PR, polls for merges,
# and calls update-branch on all unmerged successors after each merge —
# converting "manual nudge after every merge" into a single command.
#
# Usage:
#   scripts/gh-merge-chain.sh 321 322 323 324 325 326
#   scripts/gh-merge-chain.sh --dry-run 321 322 323
#   scripts/gh-merge-chain.sh --poll-interval 30 321 322 323
#   scripts/gh-merge-chain.sh --max-wait 1800 321 322 323
#
# Exit codes:
#   0  all PRs merged
#   1  one or more PRs failed (FAILURE check, CONFLICTING merge state, etc.)
#   2  --max-wait reached with unmerged PRs remaining

set -euo pipefail

POLL_INTERVAL="${POLL_INTERVAL:-20}"
MAX_WAIT="${MAX_WAIT:-3600}"
DRY_RUN=false
MERGE_METHOD="${MERGE_METHOD:-squash}"

print_help() {
    sed -nE 's/^# ?(.*)/\1/p' "$0" | head -25
}

PRS=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help) print_help; exit 0 ;;
        --dry-run) DRY_RUN=true; shift ;;
        --poll-interval) POLL_INTERVAL="$2"; shift 2 ;;
        --max-wait) MAX_WAIT="$2"; shift 2 ;;
        --merge-method) MERGE_METHOD="$2"; shift 2 ;;
        --) shift; while [[ $# -gt 0 ]]; do PRS+=("$1"); shift; done ;;
        -*) echo "unknown flag: $1" >&2; exit 2 ;;
        *) PRS+=("$1"); shift ;;
    esac
done

if [[ ${#PRS[@]} -eq 0 ]]; then
    echo "usage: $0 <pr-number> [<pr-number> ...]" >&2
    exit 2
fi

# Resolve owner/repo from gh once. In --dry-run we tolerate failure: the dry
# path doesn't call `gh api` for update-branch, so it can run in sandboxed
# CI environments where `gh repo view` can't authenticate. Real (non-dry)
# runs still require a resolvable repo.
REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || true)"
if [[ -z "$REPO" ]]; then
    if [[ "$DRY_RUN" == "true" ]]; then
        REPO="(dry-run: repo unresolved)"
    else
        echo "ERROR: cannot resolve current repo (gh repo view failed). Run inside a git checkout." >&2
        exit 2
    fi
fi

echo "merge-chain: ${#PRS[@]} PR(s) in repo $REPO with method=$MERGE_METHOD"
for n in "${PRS[@]}"; do echo "  - #$n"; done

# Phase 1: enable auto-merge on every PR (idempotent — silently no-ops if
# already set).
for pr in "${PRS[@]}"; do
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[dry-run] gh pr merge $pr --$MERGE_METHOD --auto"
        continue
    fi
    if ! gh pr merge "$pr" "--$MERGE_METHOD" --auto >/dev/null 2>&1; then
        echo "WARN: failed to set auto-merge on #$pr (already merged? closed?). Continuing."
    fi
done

[[ "$DRY_RUN" == "true" ]] && { echo "[dry-run] would now poll. exiting."; exit 0; }

# Phase 1.5: kick update-branch on every PR that starts BEHIND. Without this,
# an all-BEHIND chain deadlocks: no PR can merge until its branch is updated,
# and the Phase-2 trigger only fires AFTER a merge transition. This level-
# triggered initial nudge breaks the deadlock. (Fix per deadlock-finder-and-fixer
# Class 9 catalog: convert edge-triggered notifications to level-triggered.)
echo "merge-chain: kicking initial update-branch on any BEHIND PRs"
for pr in "${PRS[@]}"; do
    ms="$(gh pr view "$pr" --json mergeStateStatus -q .mergeStateStatus 2>/dev/null)"
    if [[ "$ms" == "BEHIND" ]]; then
        echo "  -> #$pr BEHIND; update-branch"
        gh api "repos/$REPO/pulls/$pr/update-branch" -X PUT >/dev/null 2>&1 || true
    fi
done

# Phase 2: poll. When a PR transitions to MERGED, call update-branch on the
# rest (best-effort — GitHub may transition them to BEHIND on its own
# schedule, but proactively poking avoids long stalls).
remaining=("${PRS[@]}")
start_ts=$(date +%s)

bump_remaining_branches() {
    local pr
    for pr in "${remaining[@]}"; do
        # Best-effort; ignore failures (PR may not be BEHIND yet).
        gh api "repos/$REPO/pulls/$pr/update-branch" -X PUT >/dev/null 2>&1 || true
    done
}

while [[ ${#remaining[@]} -gt 0 ]]; do
    now=$(date +%s)
    elapsed=$(( now - start_ts ))
    if [[ "$elapsed" -gt "$MAX_WAIT" ]]; then
        echo "ERROR: --max-wait $MAX_WAIT exceeded. Unmerged PRs:" >&2
        for pr in "${remaining[@]}"; do echo "  - #$pr" >&2; done
        exit 2
    fi

    new_remaining=()
    progress_made=false
    for pr in "${remaining[@]}"; do
        state="$(gh pr view "$pr" --json state -q .state 2>/dev/null)"
        case "$state" in
            MERGED)
                echo "merged: #$pr ($(date -u +%H:%M:%S))"
                progress_made=true
                ;;
            OPEN)
                new_remaining+=("$pr")
                ;;
            CLOSED)
                echo "WARN: #$pr is CLOSED without merge. Treating as failure." >&2
                exit 1
                ;;
            *)
                new_remaining+=("$pr")
                ;;
        esac
    done
    remaining=("${new_remaining[@]+"${new_remaining[@]}"}")

    # On any merge, prod the rest.
    if [[ "$progress_made" == "true" && ${#remaining[@]} -gt 0 ]]; then
        echo "  -> calling update-branch on ${#remaining[@]} remaining PR(s)"
        bump_remaining_branches
    fi

    if [[ ${#remaining[@]} -eq 0 ]]; then break; fi

    sleep "$POLL_INTERVAL"
done

echo "merge-chain: all ${#PRS[@]} PR(s) merged in ${elapsed}s"
exit 0
