#!/usr/bin/env bash
# Transaction helper for the bounded skill-builder projection write set.
# Source this file; it installs an EXIT trap that restores every snapshotted
# path unless sb_txn_commit is called.

SB_TXN_ACTIVE=0
SB_TXN_COMMITTED=0
SB_TXN_DIR=""
SB_TXN_REPO=""
SB_TXN_PATHS=()
SB_TXN_EXISTED=()
SB_TXN_ABSENT_PARENTS=()
SB_TXN_MAX_FILES=20000
SB_TXN_MAX_BYTES=$((512 * 1024 * 1024))
SB_TXN_TOTAL_FILES=0
SB_TXN_TOTAL_BYTES=0

sb_txn_fail() { echo "skill-builder transaction: $*" >&2; return 1; }

sb_txn_snapshot() {
  local rel="$1" target index count=0 size file bytes=0 existed=0 cursor
  [[ "$rel" != /* && "$rel" != *".."* && -n "$rel" ]] \
    || sb_txn_fail "invalid managed path: $rel" || return 1
  target="$SB_TXN_REPO/$rel"
  index="${#SB_TXN_PATHS[@]}"
  cursor="$(dirname "$target")"
  while [[ "$cursor" != "$SB_TXN_REPO" ]]; do
    if [[ -e "$cursor" || -L "$cursor" ]]; then
      [[ -d "$cursor" && ! -L "$cursor" ]] \
        || sb_txn_fail "managed path has an unsafe parent: $rel" || return 1
    else
      SB_TXN_ABSENT_PARENTS+=("$cursor")
    fi
    cursor="$(dirname "$cursor")"
  done
  if [[ -e "$target" || -L "$target" ]]; then
    [[ ! -L "$target" ]] || sb_txn_fail "managed path is a symlink: $rel" || return 1
    if [[ -d "$target" ]]; then
      [[ -z "$(find "$target" ! -type f ! -type d -print -quit)" ]] \
        || sb_txn_fail "managed tree contains a symlink or special file: $rel" || return 1
      count="$(find "$target" -type f | wc -l | tr -d ' ')"
      while IFS= read -r -d '' file; do
        if stat -f '%z' "$file" >/dev/null 2>&1; then size="$(stat -f '%z' "$file")"; else size="$(stat -c '%s' "$file")"; fi
        bytes=$((bytes + size))
      done < <(find "$target" -type f -print0)
    elif [[ -f "$target" ]]; then
      if stat -f '%z' "$target" >/dev/null 2>&1; then size="$(stat -f '%z' "$target")"; else size="$(stat -c '%s' "$target")"; fi
      count=1
      bytes="$size"
    else
      sb_txn_fail "managed path is not a regular file or directory: $rel" || return 1
    fi

    (( SB_TXN_TOTAL_FILES + count <= SB_TXN_MAX_FILES )) \
      || sb_txn_fail "managed write set exceeds $SB_TXN_MAX_FILES files" || return 1
    (( SB_TXN_TOTAL_BYTES + bytes <= SB_TXN_MAX_BYTES )) \
      || sb_txn_fail "managed write set exceeds $SB_TXN_MAX_BYTES bytes" || return 1
    mkdir -p "$SB_TXN_DIR/backup/$index"
    cp -a "$target" "$SB_TXN_DIR/backup/$index/value"
    existed=1
    SB_TXN_TOTAL_FILES=$((SB_TXN_TOTAL_FILES + count))
    SB_TXN_TOTAL_BYTES=$((SB_TXN_TOTAL_BYTES + bytes))
  fi
  SB_TXN_PATHS+=("$rel")
  SB_TXN_EXISTED+=("$existed")
}

sb_txn_start() {
  local repo="$1"
  [[ "$repo" == /* && -d "$repo" && ! -L "$repo" ]] \
    || sb_txn_fail "repo root must be an absolute non-symlink directory" || return 1
  SB_TXN_REPO="$(cd "$repo" && pwd -P)"
  [[ "$repo" == "$SB_TXN_REPO" ]] \
    || sb_txn_fail "repo root must use its canonical spelling" || return 1
  SB_TXN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/skill-builder-txn.XXXXXX")"
  SB_TXN_TOTAL_FILES=0
  SB_TXN_TOTAL_BYTES=0
  SB_TXN_ABSENT_PARENTS=()
  SB_TXN_ACTIVE=1
  trap sb_txn_on_exit EXIT INT TERM
}

sb_txn_snapshot_shared() {
  sb_txn_snapshot "skills/catalog.json"
  sb_txn_snapshot "registry.json"
  sb_txn_snapshot "docs/SKILL-ROUTER.md"
  sb_txn_snapshot "docs/SKILLS.md"
  sb_txn_snapshot "skills/SKILL-TIERS.md"
  sb_txn_snapshot "docs/reference/agentops-skill-domain-map.md"
  sb_txn_snapshot "docs/reference/agentops-skill-graph.md"
  sb_txn_snapshot "docs/contracts/context-map.md"
  sb_txn_snapshot "images/claude/manifest.json"
  sb_txn_snapshot "images/codex/manifest.json"
  sb_txn_snapshot "images/gemini/plugin.json"
  sb_txn_snapshot "images/gemini/skills"
  sb_txn_snapshot "skills-codex/.agentops-manifest.json"
  sb_txn_snapshot "skills-codex-overrides/catalog.json"
}

sb_txn_begin() {
  local repo="$1" slug="$2" include_source="${3:-0}"
  [[ "$slug" =~ ^[a-z][a-z0-9-]{0,63}$ ]] \
    || sb_txn_fail "invalid bounded slug" || return 1
  sb_txn_start "$repo"

  [[ "$include_source" -eq 0 ]] || sb_txn_snapshot "skills/$slug"
  sb_txn_snapshot ".agents/scratch/skill-builder/${slug}-build.json"
  sb_txn_snapshot "skills-codex/$slug"
  sb_txn_snapshot_shared
}

sb_txn_begin_projections() {
  local repo="$1" slug
  shift
  (( $# >= 1 && $# <= 64 )) \
    || sb_txn_fail "projection transaction requires 1 to 64 skills" || return 1
  for slug in "$@"; do
    [[ "$slug" =~ ^[a-z][a-z0-9-]{0,63}$ ]] \
      || sb_txn_fail "invalid bounded slug" || return 1
  done
  sb_txn_start "$repo"
  for slug in "$@"; do
    sb_txn_snapshot "skills-codex/$slug"
  done
  sb_txn_snapshot_shared
}

sb_txn_rollback() {
  local index rel target parent
  [[ "$SB_TXN_ACTIVE" -eq 1 ]] || return 0
  for ((index=${#SB_TXN_PATHS[@]}-1; index>=0; index--)); do
    rel="${SB_TXN_PATHS[$index]}"
    target="$SB_TXN_REPO/$rel"
    rm -rf -- "$target"
    if [[ "${SB_TXN_EXISTED[$index]}" -eq 1 ]]; then
      mkdir -p "$(dirname "$target")"
      cp -a "$SB_TXN_DIR/backup/$index/value" "$target"
    fi
  done
  for parent in "${SB_TXN_ABSENT_PARENTS[@]}"; do
    rmdir -- "$parent" 2>/dev/null || true
  done
}

sb_txn_commit() {
  SB_TXN_COMMITTED=1
}

sb_txn_on_exit() {
  local rc=$?
  trap - EXIT INT TERM
  if [[ "$SB_TXN_ACTIVE" -eq 1 && "$SB_TXN_COMMITTED" -eq 0 ]]; then
    sb_txn_rollback || rc=1
  fi
  [[ -z "$SB_TXN_DIR" ]] || rm -rf -- "$SB_TXN_DIR"
  exit "$rc"
}
