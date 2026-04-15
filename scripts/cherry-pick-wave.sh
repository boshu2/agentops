#!/usr/bin/env bash
set -euo pipefail
# cherry-pick-wave.sh — Cherry-pick commits from swarm worktrees to current branch.
# Usage:
#   cherry-pick-wave.sh                      # cherry-pick from all agent worktrees
#   cherry-pick-wave.sh --dry-run            # preview only
#   cherry-pick-wave.sh --pattern "swarm-*"  # custom worktree name pattern
#   cherry-pick-wave.sh --cleanup-only       # remove worktrees without cherry-picking
#   cherry-pick-wave.sh --yes                # skip confirmation prompt
#   cherry-pick-wave.sh --force-delete       # explicit opt-in to destructive removal
#
# Safety: destructive worktree removal is gated. Without --yes, --force-delete,
# or an interactive tty confirming the prompt, removal is reported as a dry-run
# and no `git worktree remove --force` is invoked.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DRY_RUN=false; CLEANUP_ONLY=false; YES=false; FORCE_DELETE=false; PATTERN="agent-*"
WT_BASE="$REPO_ROOT/.claude/worktrees"

usage() { cat <<'EOF'
Usage: cherry-pick-wave.sh [OPTIONS]
Options:
  --dry-run        Show what would be cherry-picked without making changes
  --pattern PAT    Anchored glob for worktree dirs under .claude/worktrees
                   (default: "agent-*"). Must begin with [A-Za-z0-9_-];
                   leading wildcards (*, ?, [) and path traversal are rejected.
  --cleanup-only   Remove worktrees without cherry-picking
  --yes, -y        Skip confirmation prompt (also authorizes destructive removal)
  --force-delete   Explicit opt-in to destructive worktree removal even when
                   stdin is not a tty. Without --yes / --force-delete / a tty,
                   removal defaults to dry-run.
  --help, -h       Show this help message
EOF
}

die() { echo "ERROR: $*" >&2; exit 1; }

changed_files_summary() {
  git -C "$REPO_ROOT" diff-tree --no-commit-id --name-only -r "$1" 2>/dev/null \
    | sed 's|.*/||' | head -5 | paste -sd', ' -
}

confirm() {
  [[ "$YES" == "true" ]] && return 0
  printf "%s [y/N] " "$1"; read -r ans; [[ "$ans" =~ ^[Yy] ]]
}

# destructive_consent: returns 0 if the caller has authorized destructive
# worktree removal (--yes, --force-delete, or an interactive tty). Otherwise
# returns 1 and the caller must dry-run.
destructive_consent() {
  [[ "$YES" == "true" ]] && return 0
  [[ "$FORCE_DELETE" == "true" ]] && return 0
  [[ -t 0 ]] && return 0
  return 1
}

remove_worktrees() {
  local consent="false"
  if destructive_consent; then
    consent="true"
  fi
  if [[ "$consent" != "true" ]]; then
    echo "[dry-run] worktree removal skipped (no --yes / --force-delete / tty)"
    for i in "${!WT_NAMES[@]}"; do
      local wt="${WORKTREES[$i]}" name="${WT_NAMES[$i]}"
      echo "[dry-run] would remove worktree: $wt ($name)"
    done
    echo "[dry-run] re-run with --force-delete or --yes to actually remove"
    return 0
  fi
  echo "Cleaning up worktrees..."
  for i in "${!WT_NAMES[@]}"; do
    local wt="${WORKTREES[$i]}" name="${WT_NAMES[$i]}" c="${WT_COMMITS[$i]}"
    if ! git -C "$REPO_ROOT" worktree remove --force "$wt" 2>/dev/null; then
        echo "WARN: git worktree remove failed for $wt; leaving path in place" >&2
    fi
    [[ "$c" -gt 0 ]] && echo "  Removed $name" || echo "  Removed $name (no changes)"
  done
  git -C "$REPO_ROOT" worktree prune 2>/dev/null || true
}

# ── Argument Parsing ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)      DRY_RUN=true; shift ;;
    --cleanup-only) CLEANUP_ONLY=true; shift ;;
    --yes|-y)       YES=true; shift ;;
    --force-delete) FORCE_DELETE=true; shift ;;
    --pattern)      PATTERN="${2:?--pattern requires a value}"; shift 2 ;;
    --help|-h)      usage; exit 0 ;;
    *)              die "Unknown option: $1" ;;
  esac
done

# Validate --pattern: reject empty, path traversal, leading wildcards.
# Anchors the worktree match to a known prefix character class so a stray
# "*" or "?/[" pattern cannot expand to match unexpected directories under
# .claude/worktrees.
if [[ -z "$PATTERN" ]]; then
    die "Invalid --pattern: must not be empty"
fi
if [[ "$PATTERN" == *..* ]] || [[ "$PATTERN" == */* ]]; then
    die "Invalid --pattern: must not contain '..' or '/'. Got: $PATTERN"
fi
if [[ ! "$PATTERN" =~ ^[A-Za-z0-9_-] ]]; then
    die "Invalid --pattern: must start with [A-Za-z0-9_-] (anchored prefix). Got: $PATTERN"
fi

echo "Resolved worktree pattern: $WT_BASE/$PATTERN"

# ── Pre-flight ───────────────────────────────────────────────────────────────
git -C "$REPO_ROOT" rev-parse --is-inside-work-tree &>/dev/null \
  || die "Not inside a git repository: $REPO_ROOT"
if [[ "$DRY_RUN" != "true" ]] && [[ "$CLEANUP_ONLY" != "true" ]]; then
  if ! git -C "$REPO_ROOT" diff --quiet 2>/dev/null \
     || ! git -C "$REPO_ROOT" diff --cached --quiet 2>/dev/null; then
    die "Current branch has uncommitted changes. Commit or stash first."
  fi
fi
CURRENT_BRANCH="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)"

# ── Discover worktrees ───────────────────────────────────────────────────────
WORKTREES=()
if [[ -d "$WT_BASE" ]]; then
  for d in "$WT_BASE"/$PATTERN; do [[ -d "$d" ]] && WORKTREES+=("$d"); done
fi
if [[ ${#WORKTREES[@]} -eq 0 ]]; then
  echo "No worktree directories found matching '$PATTERN' in $WT_BASE"; exit 0
fi
echo "Found ${#WORKTREES[@]} worktree directory(ies) matching pattern"

# ── Analyze each worktree ────────────────────────────────────────────────────
declare -a WT_NAMES=() WT_BRANCHES=() WT_COMMITS=() WT_SUMMARIES=()
TOTAL_COMMITS=0; HAS_CHANGES=0

for wt in "${WORKTREES[@]}"; do
  name="$(basename "$wt")"; WT_NAMES+=("$name")
  branch=""
  if git -C "$wt" rev-parse --is-inside-work-tree &>/dev/null 2>&1; then
    branch="$(git -C "$wt" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  fi
  WT_BRANCHES+=("$branch")
  if [[ -z "$branch" ]]; then
    WT_COMMITS+=("0"); WT_SUMMARIES+=("branch unknown"); continue
  fi
  merge_base="$(git -C "$REPO_ROOT" merge-base "$CURRENT_BRANCH" "$branch" 2>/dev/null || true)"
  if [[ -z "$merge_base" ]]; then
    WT_COMMITS+=("0"); WT_SUMMARIES+=("no merge-base"); continue
  fi
  shas="$(git -C "$REPO_ROOT" rev-list --reverse "${merge_base}..${branch}" 2>/dev/null || true)"
  count=0; files=""
  if [[ -n "$shas" ]]; then
    count="$(echo "$shas" | wc -l | tr -d ' ')"
    files="$(for sha in $shas; do changed_files_summary "$sha"; done \
      | tr ',' '\n' | sort -u | head -5 | paste -sd', ' -)"
  fi
  WT_COMMITS+=("$count")
  if [[ "$count" -gt 0 ]]; then
    WT_SUMMARIES+=("$files"); TOTAL_COMMITS=$((TOTAL_COMMITS + count)); HAS_CHANGES=$((HAS_CHANGES + 1))
  else
    WT_SUMMARIES+=("no changes")
  fi
done

# ── Display summary ──────────────────────────────────────────────────────────
echo "Found ${#WORKTREES[@]} worktrees with commits:"
for i in "${!WT_NAMES[@]}"; do
  c="${WT_COMMITS[$i]}"; s="${WT_SUMMARIES[$i]}"
  [[ "$c" -gt 0 ]] \
    && echo "  ${WT_NAMES[$i]}: $c commit(s) ($s)" \
    || echo "  ${WT_NAMES[$i]}: 0 commits ($s)"
done
echo ""

# ── Cleanup-only mode ────────────────────────────────────────────────────────
if [[ "$CLEANUP_ONLY" == "true" ]]; then
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "Dry run: would remove ${#WORKTREES[@]} worktree(s)."; exit 0
  fi
  confirm "Remove ${#WORKTREES[@]} worktree(s)?" || { echo "Aborted."; exit 0; }
  remove_worktrees; echo "Done."; exit 0
fi

# ── Dry-run / no-work exit ───────────────────────────────────────────────────
if [[ "$DRY_RUN" == "true" ]]; then
  echo "Dry run: would cherry-pick $TOTAL_COMMITS commit(s) from $HAS_CHANGES worktree(s)."
  exit 0
fi
if [[ "$TOTAL_COMMITS" -eq 0 ]]; then echo "No commits to cherry-pick."; exit 0; fi

# ── Confirm & cherry-pick ────────────────────────────────────────────────────
confirm "Cherry-pick $TOTAL_COMMITS commit(s) from $HAS_CHANGES worktree(s)?" \
  || { echo "Aborted."; exit 0; }

echo "Cherry-picking..."
picked=0; step=0
for i in "${!WT_NAMES[@]}"; do
  [[ "${WT_COMMITS[$i]}" -gt 0 ]] || continue
  step=$((step + 1))
  name="${WT_NAMES[$i]}"; branch="${WT_BRANCHES[$i]}"
  merge_base="$(git -C "$REPO_ROOT" merge-base "$CURRENT_BRANCH" "$branch")"
  shas="$(git -C "$REPO_ROOT" rev-list --reverse "${merge_base}..${branch}")"
  for sha in $shas; do
    if ! git -C "$REPO_ROOT" cherry-pick "$sha" 2>&1; then
      git -C "$REPO_ROOT" cherry-pick --abort 2>/dev/null || true
      echo ""; echo "CONFLICT in $name at commit $sha"
      echo "To resolve manually:"
      echo "  cd $REPO_ROOT"
      echo "  git cherry-pick $sha"
      echo "  # fix conflicts, then: git cherry-pick --continue"
      exit 1
    fi
    picked=$((picked + 1))
  done
  echo "  [$step/$HAS_CHANGES] $name: OK"
done

# ── Post cherry-pick cleanup ─────────────────────────────────────────────────
echo ""; remove_worktrees
echo ""; echo "Done: $picked commit(s) cherry-picked from $HAS_CHANGES worktree(s)."
