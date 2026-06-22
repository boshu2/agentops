#!/usr/bin/env bash
# verify-pushed-commit-builds.sh — close the partial-commit-lands-broken escape (age-yy24).
#
# The pre-push gate (`ao gate check`, the worktree `go build ./...`) validates the
# WORKING TREE, not the COMMIT being pushed. A commit that silently omitted files
# (a multi-path `git add` that aborted on a deleted-file pathspec, dropping the rest)
# builds in the worktree but is build-broken as committed — it passed the gate + a
# cross-family pawl and landed origin/main red once (2026-06-22). This judges the
# COMMIT, not the tree: it builds each pushed commit in an ISOLATED temp worktree.
#
# It only does work when the worktree DIFFERS from HEAD for tracked-or-untracked
# source — a clean tree means worktree==HEAD, so the gate's worktree build already
# validated the commit (zero cost on the common clean-tree push, zero false alarms:
# it never fails merely because the user kept editing after committing).
#
# Usage:
#   verify-pushed-commit-builds.sh < <git-pre-push-stdin>   # lines: <lref> <lsha> <rref> <rsha>
#   verify-pushed-commit-builds.sh <sha> [<sha> ...]        # explicit shas (tests / manual)
#
# Exit: 0 = all pushed commits build (or skipped clean / infra-skip); 1 = a commit does
# NOT build (fail-closed — refuse the push). Fail-OPEN on infrastructure errors
# (mktemp / `git worktree add` failure) so a broken check never wedges a push.
#
# Env:
#   AGENTOPS_PREPUSH_SKIP_COMMIT_BUILD=1   skip entirely (emergency bypass)
#   AGENTOPS_COMMIT_BUILD_CMD              build command run inside the temp worktree
#                                          (default: "cd cli && go build ./...")
#   AGENTOPS_COMMIT_BUILD_PATHS           pathspecs for the dirty check (default: "cli scripts")
set -u

if [ "${AGENTOPS_PREPUSH_SKIP_COMMIT_BUILD:-0}" = "1" ]; then
    exit 0
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
BUILD_CMD="${AGENTOPS_COMMIT_BUILD_CMD:-cd cli && go build ./...}"
# Intentional word-split of the space-separated pathspec list into array elements.
# shellcheck disable=SC2206
DIRTY_PATHS=(${AGENTOPS_COMMIT_BUILD_PATHS:-cli scripts})
ZERO="0000000000000000000000000000000000000000"

# worktree_matches_head: true when the worktree equals HEAD for the watched source
# paths — both tracked changes AND untracked source files. In that case the gate's
# worktree build already validated the committed state, so there is nothing to add.
worktree_matches_head() {
    git -C "$REPO_ROOT" diff --quiet HEAD -- "${DIRTY_PATHS[@]}" 2>/dev/null || return 1
    local others
    others="$(git -C "$REPO_ROOT" ls-files --others --exclude-standard -- "${DIRTY_PATHS[@]}" 2>/dev/null | grep -cE '\.(go|sh)$')"
    [ "${others:-0}" -eq 0 ]
}

# build_commit_isolated <sha>: 0 = builds (or infra-skip), 1 = definitively does NOT build.
build_commit_isolated() {
    local sha="$1" tmpwt rc=0
    tmpwt="$(mktemp -d "${TMPDIR:-/tmp}/agentops-commitbuild.XXXXXX" 2>/dev/null)" || return 0
    if ! git -C "$REPO_ROOT" worktree add --detach --quiet "$tmpwt" "$sha" 2>/dev/null; then
        echo >&2 "⚠ verify-pushed-commit-builds: could not create a temp worktree for ${sha} — skipping (fail-open)."
        rm -rf "$tmpwt" 2>/dev/null
        return 0
    fi
    if ! ( cd "$tmpwt" && eval "$BUILD_CMD" ) >/dev/null 2>&1; then
        rc=1
    fi
    git -C "$REPO_ROOT" worktree remove --force "$tmpwt" >/dev/null 2>&1 || true
    rm -rf "$tmpwt" 2>/dev/null
    git -C "$REPO_ROOT" worktree prune >/dev/null 2>&1 || true
    return "$rc"
}

# MAX_COMMITS caps the isolated builds so a huge range can't wedge a push for minutes;
# beyond it we validate the tip + the most-recent MAX_COMMITS and warn (the tip is what
# main becomes; deep history is rarely first-introduced in one push).
MAX_COMMITS="${AGENTOPS_COMMIT_BUILD_MAX:-12}"

# Collect the commits to validate. Two modes:
#   explicit args  -> exactly those shas (tests / manual), no range expansion or skip.
#   git stdin      -> for each `<lref> <lsha> <rref> <rsha>` push line, the pushed RANGE
#                     `rsha..lsha` (every NEW commit), not just the tip — a tip that
#                     builds can still push a build-broken intermediate (refuted 2026-06-22).
#                     New branches / unknown rsha fall back to the tip only.
shas=()
explicit=0
clean=0
worktree_matches_head && clean=1

if [ "$#" -gt 0 ]; then
    explicit=1
    shas=("$@")
elif [ ! -t 0 ]; then
    while read -r _lref lsha _rref rsha; do
        [ -z "${lsha:-}" ] || [ "$lsha" = "$ZERO" ] && continue
        # Clean tree: the gate's worktree build already validated the TIP (lsha == HEAD),
        # but NEVER the intermediates — so only the tip is droppable when clean.
        range_commits=""
        if [ -n "${rsha:-}" ] && [ "$rsha" != "$ZERO" ] && git -C "$REPO_ROOT" cat-file -e "${rsha}^{commit}" 2>/dev/null; then
            range_commits="$(git -C "$REPO_ROOT" rev-list --reverse "${rsha}..${lsha}" 2>/dev/null)"
        else
            range_commits="$lsha"
        fi
        for c in $range_commits; do
            # On a clean tree, the tip is covered by the worktree build — skip just it.
            { [ "$clean" -eq 1 ] && [ "$c" = "$lsha" ]; } && continue
            shas+=("$c")
        done
    done
fi
[ "${#shas[@]}" -eq 0 ] && exit 0

# Explicit-arg mode honors the clean-tree skip too (so the hook's clean fast-path holds),
# unless a sha was given that is not HEAD.
if [ "$explicit" -eq 1 ] && [ "$clean" -eq 1 ]; then
    head_sha="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null)"
    filtered=()
    for c in "${shas[@]}"; do [ "$c" = "$head_sha" ] || filtered+=("$c"); done
    shas=("${filtered[@]}")
    [ "${#shas[@]}" -eq 0 ] && exit 0
fi

if [ "${#shas[@]}" -gt "$MAX_COMMITS" ]; then
    echo >&2 "⚠ verify-pushed-commit-builds: ${#shas[@]} pushed commits — validating only the most recent ${MAX_COMMITS}."
    shas=("${shas[@]: -$MAX_COMMITS}")
fi

for sha in "${shas[@]}"; do
    if ! build_commit_isolated "$sha"; then
        echo >&2 "✗ pre-push (age-yy24): a COMMIT being pushed (${sha}) does NOT build, though the working tree does."
        echo >&2 "  Partial-commit class — 'git show ${sha} --stat' likely omits files the code needs. Push refused."
        echo >&2 "  (override only with cause: AGENTOPS_PREPUSH_SKIP_COMMIT_BUILD=1)"
        exit 1
    fi
done
exit 0
