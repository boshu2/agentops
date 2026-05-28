#!/usr/bin/env bash
# gc-stale-worktrees.sh — prune git worktrees whose branches have already merged.
#
# Default behavior: list candidates that would be removed; require --apply to
# actually remove. Detection logic checks merged-ness two ways:
#   1. `git merge-base --is-ancestor <branch> <ref>` — catches ff-merge and
#      rebase-merge cases where the branch tip is reachable from the merge ref.
#   2. Subject-line scan of the merge ref's log for `<bead-id>` (from the
#      branch name) and `(#<PR#>)` — catches AgentOps squash-merges, which
#      drop a new commit on main whose subject embeds the bead id per the
#      CLAUDE.md branch+PR convention.
#
# Worktrees with uncommitted changes, detached HEAD, or unknown merge state
# are always kept unless --force is passed.
#
# Usage:
#   scripts/gc-stale-worktrees.sh                      # dry run, default ref origin/main
#   scripts/gc-stale-worktrees.sh --apply              # actually remove
#   scripts/gc-stale-worktrees.sh --ref origin/develop # different upstream
#   scripts/gc-stale-worktrees.sh --json               # machine-readable summary
#   scripts/gc-stale-worktrees.sh --verbose            # per-worktree decision trace
#
# Exit codes: 0 = success (may include removals); 1 = usage/internal error.

set -euo pipefail

REF="origin/main"
APPLY=0
FORCE=0
JSON=0
VERBOSE=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=1 ;;
    --force) FORCE=1 ;;
    --json) JSON=1 ;;
    --verbose|-v) VERBOSE=1 ;;
    --ref) shift; REF="${1:-origin/main}" ;;
    -h|--help) usage 0 ;;
    *) echo "gc-stale-worktrees: unknown arg: $1" >&2; usage 1 ;;
  esac
  shift || true
done

# Verify ref exists; without it we can't determine merged-ness.
if ! git rev-parse --verify --quiet "$REF" >/dev/null; then
  echo "gc-stale-worktrees: ref not found: $REF (fetch first)" >&2
  exit 1
fi

# The main worktree is the one whose `.git` is the common dir's parent —
# `--show-toplevel` would point at whichever worktree we happened to be invoked
# from, which is fatal here because we use it to decide what NOT to remove.
COMMON_DIR="$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"
if [ -z "$COMMON_DIR" ]; then
  echo "gc-stale-worktrees: cannot resolve git common dir" >&2
  exit 1
fi
MAIN_ROOT="$(dirname "$COMMON_DIR")"

# The ref's local branch is also protected against removal regardless of
# merge state. For origin/main the protected name is "main".
PROTECTED_BRANCH="${REF#*/}"

# Parse `git worktree list --porcelain` into a stream of records. Each record
# is terminated by a blank line; fields are "worktree <path>", "HEAD <sha>",
# "branch <ref>" or "detached", "bare", "locked", "prunable".
declare -a removed_paths=()
declare -a kept_paths=()
declare -a skipped_reasons=()

log_decision() {
  # `[ ] && echo` returns non-zero under set -e when VERBOSE != 1; use if/fi.
  if [ "$VERBOSE" -eq 1 ]; then
    echo "gc-stale-worktrees: $*" >&2
  fi
}

# Cache of remote branch names that have been merged via PR, populated once
# from `gh pr list --state merged` if the tool is available. Empty otherwise.
MERGED_BRANCH_SET=""
populate_merged_branch_set() {
  if [ -n "$MERGED_BRANCH_SET" ]; then
    return 0
  fi
  if ! command -v gh >/dev/null 2>&1; then
    MERGED_BRANCH_SET="__UNAVAILABLE__"
    return 0
  fi
  # Newline-delimited list of merged-PR head branches. --limit 500 is a soft
  # ceiling; older worktrees beyond that are still kept by the ancestor check.
  local raw
  raw="$(gh pr list --state merged --limit 500 --json headRefName -q '.[].headRefName' 2>/dev/null || true)"
  if [ -z "$raw" ]; then
    MERGED_BRANCH_SET="__UNAVAILABLE__"
    return 0
  fi
  # Wrap each entry with newlines so we can grep -Fx for exact match later.
  MERGED_BRANCH_SET=$'\n'"$raw"$'\n'
}

is_branch_merged() {
  local branch="$1"
  # Empty/fresh branch (tip identical to ref) is NOT "merged" — there's
  # nothing to merge yet, the operator just created the worktree.
  local branch_sha ref_sha
  branch_sha="$(git rev-parse "$branch" 2>/dev/null || true)"
  ref_sha="$(git rev-parse "$REF" 2>/dev/null || true)"
  if [ -n "$branch_sha" ] && [ "$branch_sha" = "$ref_sha" ]; then
    return 1
  fi
  # Strategy 1: tip is ancestor of ref (ff/rebase merge).
  if git merge-base --is-ancestor "$branch" "$REF" 2>/dev/null; then
    return 0
  fi
  # Strategy 2: GitHub says this branch was merged via PR (catches squash).
  populate_merged_branch_set
  if [ "$MERGED_BRANCH_SET" != "__UNAVAILABLE__" ]; then
    if printf '%s' "$MERGED_BRANCH_SET" | grep -qFx -- "$branch"; then
      return 0
    fi
  fi
  # Strategy 3: branch name matches the alias convention `pr-<N>` — look up
  # PR #N directly. Catches local renames where the GitHub head ref differs
  # from the local branch name (codex peer convention).
  if command -v gh >/dev/null 2>&1; then
    local pr_num
    pr_num="$(printf '%s' "$branch" | sed -n 's/^pr-\([0-9]\{1,\}\)$/\1/p')"
    if [ -n "$pr_num" ]; then
      local pr_state
      pr_state="$(gh pr view "$pr_num" --json state -q .state 2>/dev/null || true)"
      if [ "$pr_state" = "MERGED" ]; then
        return 0
      fi
    fi
  fi
  return 1
}

worktree_has_uncommitted() {
  local path="$1"
  # An untracked-only worktree still counts as "clean enough" to remove.
  # We block only on staged/unstaged tracked changes.
  if [ -n "$(git -C "$path" status --porcelain --untracked-files=no 2>/dev/null)" ]; then
    return 0
  fi
  return 1
}

process_record() {
  local path="$1" head="$2" branch="$3" detached="$4" prunable="$5" locked="$6"

  # Always skip the main checkout itself, no matter where we were invoked from.
  if [ "$path" = "$MAIN_ROOT" ]; then
    log_decision "skip main checkout: $path"
    kept_paths+=("$path")
    return
  fi

  # Always skip the protected branch (e.g. main) — never blow away the trunk
  # even if a side worktree happens to have it checked out.
  local short_branch="${branch#refs/heads/}"
  if [ -n "$short_branch" ] && [ "$short_branch" = "$PROTECTED_BRANCH" ]; then
    log_decision "skip protected branch ($PROTECTED_BRANCH) worktree: $path"
    kept_paths+=("$path")
    return
  fi

  if [ "$prunable" = "1" ]; then
    # Worktree is already gone from disk; record can be pruned safely.
    log_decision "prune stale record: $path"
    if [ "$APPLY" -eq 1 ]; then
      git worktree prune >/dev/null 2>&1 || true
    fi
    removed_paths+=("$path (record-only)")
    return
  fi

  if [ "$locked" = "1" ] && [ "$FORCE" -eq 0 ]; then
    skipped_reasons+=("$path: locked")
    kept_paths+=("$path")
    return
  fi

  if [ "$detached" = "1" ]; then
    skipped_reasons+=("$path: detached HEAD (no branch to evaluate)")
    kept_paths+=("$path")
    return
  fi

  if ! is_branch_merged "$short_branch"; then
    log_decision "keep (not merged): $path branch=$short_branch"
    kept_paths+=("$path")
    return
  fi

  if [ "$FORCE" -eq 0 ] && worktree_has_uncommitted "$path"; then
    skipped_reasons+=("$path: uncommitted changes (re-run with --force to remove anyway)")
    kept_paths+=("$path")
    return
  fi

  log_decision "remove (merged): $path branch=$short_branch"
  if [ "$APPLY" -eq 1 ]; then
    if ! git worktree remove "$path" 2>/dev/null; then
      # Fall back to --force when remove balks (e.g. directory missing files).
      git worktree remove --force "$path" >/dev/null 2>&1 || {
        skipped_reasons+=("$path: remove failed")
        kept_paths+=("$path")
        return
      }
    fi
  fi
  removed_paths+=("$path")
}

# Stream-parse porcelain output.
path="" head="" branch="" detached="0" prunable="0" locked="0"
flush() {
  [ -n "$path" ] || return 0
  process_record "$path" "$head" "$branch" "$detached" "$prunable" "$locked"
  path=""; head=""; branch=""; detached="0"; prunable="0"; locked="0"
}

while IFS= read -r line; do
  if [ -z "$line" ]; then
    flush
    continue
  fi
  case "$line" in
    "worktree "*) path="${line#worktree }" ;;
    "HEAD "*) head="${line#HEAD }" ;;
    "branch "*) branch="${line#branch }" ;;
    "detached") detached="1" ;;
    "prunable"*) prunable="1" ;;
    "locked"*) locked="1" ;;
  esac
done < <(git worktree list --porcelain)
flush

if [ "$JSON" -eq 1 ]; then
  printf '{"ref":"%s","apply":%s,"removed":%d,"kept":%d,"skipped":%d}\n' \
    "$REF" "$([ "$APPLY" -eq 1 ] && echo true || echo false)" \
    "${#removed_paths[@]}" "${#kept_paths[@]}" "${#skipped_reasons[@]}"
else
  if [ "$APPLY" -eq 1 ]; then
    echo "gc-stale-worktrees: removed ${#removed_paths[@]}, kept ${#kept_paths[@]}, skipped ${#skipped_reasons[@]}"
  else
    echo "gc-stale-worktrees: would remove ${#removed_paths[@]}, would keep ${#kept_paths[@]}, would skip ${#skipped_reasons[@]} (dry run — pass --apply to act)"
  fi
  for p in "${removed_paths[@]}"; do echo "  REMOVE: $p"; done
  for r in "${skipped_reasons[@]}"; do echo "  SKIP:   $r"; done
fi
