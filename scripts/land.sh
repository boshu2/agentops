#!/usr/bin/env bash
# land.sh - deterministic land path for one pawl-gated bead (age-genn).
#
# Usage:
#   scripts/land.sh <bead-id> [--author-family <family>] [--background] [--no-close]
#
# The wrapper fails fast before expensive gate work when HEAD does not cite the
# bead, then runs the current proof path:
#   ship gate -> pawl review -> pawl-land push -> post-land provenance -> br close.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

SHIP_SCRIPT="${LAND_SHIP_SCRIPT:-$SCRIPT_DIR/ship.sh}"
PAWL_REVIEW_SCRIPT="${LAND_PAWL_REVIEW_SCRIPT:-$SCRIPT_DIR/pawl-review.sh}"
PAWL_LAND_SCRIPT="${LAND_PAWL_LAND_SCRIPT:-$SCRIPT_DIR/pawl-land.sh}"
POST_LAND_SCRIPT="${LAND_POST_LAND_SCRIPT:-$SCRIPT_DIR/post-land-provenance-emit.sh}"
AO_BIN="${AO_BIN:-$(command -v ao 2>/dev/null || true)}"
BR_BIN="${BR_BIN:-$(command -v br 2>/dev/null || true)}"
LAND_LOG_DIR="${LAND_LOG_DIR:-${TMPDIR:-/tmp}/agentops-land}"

BEAD=""
# Producer metadata passed to pawl-review; this must not imply a Claude runtime
# default on bo-mac. Override with LAND_AUTHOR_FAMILY or --author-family.
AUTHOR_FAMILY="${LAND_AUTHOR_FAMILY:-operator}"
BACKGROUND=0
NO_CLOSE=0
DRY_RUN=0

usage() {
  sed -n '2,8p' "$0"
}

die() {
  echo "land: ERROR: $*" >&2
  exit 2
}

commit_cites_bead() {
  local msg="$1" wanted="$2" match token
  while IFS= read -r match; do
    [[ -n "$match" ]] || continue
    token="$(sed -E 's/^.*((age|ag)-[a-z0-9][a-z0-9.-]*)$/\1/' <<<"$match")"
    token="${token%"${token##*[a-z0-9]}"}"
    [[ "$token" == "$wanted" ]] && return 0
  done < <(grep -Eo '(^|[^[:alnum:].-])((age|ag)-[a-z0-9][a-z0-9.-]*)' <<<"$msg" || true)
  return 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --author-family)
      [[ -n "${2:-}" ]] || die "--author-family requires a value"
      AUTHOR_FAMILY="$2"
      shift 2
      ;;
    --background)
      BACKGROUND=1
      shift
      ;;
    --foreground)
      BACKGROUND=0
      shift
      ;;
    --no-close)
      NO_CLOSE=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      die "unknown flag: $1"
      ;;
    *)
      [[ -z "$BEAD" ]] || die "only one bead id is allowed"
      BEAD="$1"
      shift
      ;;
  esac
done

[[ -n "$BEAD" ]] || die "usage: scripts/land.sh <bead-id>"

if [[ "$BACKGROUND" -eq 1 ]]; then
  mkdir -p "$LAND_LOG_DIR"
  log="$LAND_LOG_DIR/${BEAD}-$(date -u +%Y%m%dT%H%M%SZ).log"
  child_args=("$BEAD" --foreground --author-family "$AUTHOR_FAMILY")
  [[ "$NO_CLOSE" -eq 1 ]] && child_args+=(--no-close)
  [[ "$DRY_RUN" -eq 1 ]] && child_args+=(--dry-run)
  nohup "$0" "${child_args[@]}" >"$log" 2>&1 &
  echo "land: started background land for $BEAD (pid=$!, log=$log)"
  exit 0
fi

cd "$REPO_ROOT"

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [[ -z "$branch" || "$branch" == "HEAD" ]]; then
  die "detached HEAD; run from a named bead worktree"
fi
if [[ "$branch" == "main" || "$branch" == "master" ]]; then
  die "refusing to land from $branch; create a bead worktree branch first"
fi

head_sha="$(git rev-parse HEAD)"
head_short="$(git rev-parse --short HEAD)"
head_msg="$(git log -1 --format=%B "$head_sha")"
if ! commit_cites_bead "$head_msg" "$BEAD"; then
  cat >&2 <<EOF
land: ERROR: HEAD ($head_short) does not cite $BEAD.
Amend the commit before running expensive gate work:
  git commit --amend --no-edit --trailer "Refs: $BEAD"
EOF
  exit 2
fi

if ! git diff --quiet -- || ! git diff --cached --quiet -- \
  || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  die "working tree has uncommitted changes; commit the bead's code before landing"
fi

[[ -x "$SHIP_SCRIPT" ]] || die "ship script missing or not executable: $SHIP_SCRIPT"
[[ -x "$PAWL_REVIEW_SCRIPT" ]] || die "pawl review script missing or not executable: $PAWL_REVIEW_SCRIPT"
[[ -x "$PAWL_LAND_SCRIPT" ]] || die "pawl land script missing or not executable: $PAWL_LAND_SCRIPT"
[[ -x "$POST_LAND_SCRIPT" ]] || die "post-land provenance script missing or not executable: $POST_LAND_SCRIPT"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "land: dry-run OK for $BEAD at $head_short on $branch"
  echo "land: would run ship -> pawl-review -> pawl-land -> post-land provenance -> br close"
  exit 0
fi

echo "land: $BEAD head=$head_short branch=$branch"

# ship.sh still owns the inventory-aware gate-mode selection. Use the explicit
# current wrapper after this point so the operator cannot drift into hand-rolled
# push/close steps.
"$SHIP_SCRIPT"

if ! git diff --quiet -- || ! git diff --cached --quiet -- \
  || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  cat >&2 <<EOF
land: ERROR: ship produced uncommitted changes, likely from inventory regeneration.
Review and amend them into the bead commit, then rerun land:
  git status --short
  git add -A && git commit --amend --no-edit
EOF
  exit 2
fi

"$PAWL_REVIEW_SCRIPT" "$BEAD" --scope head --author-family "$AUTHOR_FAMILY"
"$PAWL_LAND_SCRIPT" "$BEAD"

if [[ -n "$AO_BIN" ]]; then
  AGENTOPS_PROVENANCE_EMIT_STRICT=1 AGENTOPS_PROVENANCE_REQUIRED_VERDICT_BEAD="$BEAD" AGENTOPS_PROVENANCE_REQUIRED_VERDICT_HEAD="$head_sha" AO_BIN="$AO_BIN" "$POST_LAND_SCRIPT"
else
  AGENTOPS_PROVENANCE_EMIT_STRICT=1 AGENTOPS_PROVENANCE_REQUIRED_VERDICT_BEAD="$BEAD" AGENTOPS_PROVENANCE_REQUIRED_VERDICT_HEAD="$head_sha" "$POST_LAND_SCRIPT"
fi

if [[ "$NO_CLOSE" -eq 0 ]]; then
  [[ -n "$BR_BIN" ]] || die "br not on PATH; close $BEAD manually with br close"
  if [[ -z "${BEADS_DIR:-}" ]]; then
    [[ -n "$AO_BIN" ]] || die "ao not on PATH; set BEADS_DIR or AO_BIN before closing $BEAD"
    BEADS_DIR="$("$AO_BIN" beads dir)"
    export BEADS_DIR
  fi
  "$BR_BIN" close "$BEAD" --reason "Landed $head_short via scripts/land.sh; ship gate, pawl review, pawl-land, and strict post-land provenance completed." --json
fi

echo "land: DONE $BEAD at $head_short"
