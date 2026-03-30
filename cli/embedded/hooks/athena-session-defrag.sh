#!/usr/bin/env bash
set -euo pipefail

# Kill switch
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0

# Only run defrag if ao CLI available
if ! command -v ao &>/dev/null; then
  exit 0
fi

repo_root() {
  git rev-parse --show-toplevel 2>/dev/null || pwd
}

file_mtime() {
  stat -f %m "$1" 2>/dev/null || stat -c %Y "$1" 2>/dev/null || printf '0\n'
}

is_placeholder_pattern() {
  awk '
    BEGIN { delims = 0; body = 0 }
    /^---[[:space:]]*$/ { delims++; next }
    delims >= 2 && $0 !~ /^[[:space:]]*$/ { body = 1 }
    END { exit (delims >= 2 && body == 0) ? 0 : 1 }
  ' "$1"
}

has_duplicate_heading() {
  awk '
    /^##[[:space:]]+/ {
      count[$0]++
      if (count[$0] > 1) {
        dup = 1
      }
    }
    END { exit dup ? 0 : 1 }
  ' "$1"
}

scan_normalization_defects() {
  local root="$1"
  local patterns_dir="$root/.agents/patterns"
  local learnings_dir="$root/.agents/learnings"
  local athena_dir="$root/.agents/athena"
  local scan_dirs=()
  local placeholders=0
  local stacked=0
  local bundled=0
  local duplicate_headings=0
  local stale_contradictions=0
  local latest_extraction_mtime=0
  local file=""

  if [ -d "$learnings_dir" ]; then
    scan_dirs+=("$learnings_dir")
  fi
  if [ -d "$patterns_dir" ]; then
    scan_dirs+=("$patterns_dir")
  fi

  if [ "${#scan_dirs[@]}" -gt 0 ]; then
    while IFS= read -r -d '' file; do
      local frontmatter_count
      local learning_headings

      frontmatter_count=$(awk '/^---[[:space:]]*$/{n++} END{print n+0}' "$file")
      if [ "$frontmatter_count" -gt 2 ]; then
        stacked=$((stacked + 1))
      fi

      if [[ "$file" == "$patterns_dir/"* ]] && is_placeholder_pattern "$file"; then
        placeholders=$((placeholders + 1))
      fi

      learning_headings=$(awk '/^#{1,3}[[:space:]]+Learning([[:space:]:-]|$)/{n++} END{print n+0}' "$file")
      if [ "$learning_headings" -gt 1 ]; then
        bundled=$((bundled + 1))
      fi

      if has_duplicate_heading "$file"; then
        duplicate_headings=$((duplicate_headings + 1))
      fi
    done < <(find "${scan_dirs[@]}" -type f -name '*.md' -print0)
  fi

  if [ -d "$athena_dir" ]; then
    while IFS= read -r -d '' file; do
      local mtime
      mtime=$(file_mtime "$file")
      if [ "$mtime" -gt "$latest_extraction_mtime" ]; then
        latest_extraction_mtime="$mtime"
      fi
    done < <(find "$athena_dir" -type f \( -iname '*extraction*' -o -iname '*knowledge-analysis*' -o -iname '*extraction-complete*' \) -print0)

    if [ "$latest_extraction_mtime" -gt 0 ]; then
      while IFS= read -r -d '' file; do
        local contradiction_mtime
        contradiction_mtime=$(file_mtime "$file")
        if [ "$contradiction_mtime" -lt "$latest_extraction_mtime" ]; then
          stale_contradictions=$((stale_contradictions + 1))
        fi
      done < <(find "$athena_dir" -type f \( -iname '*contradict*' -o -iname '*contradiction*' \) -print0)
    fi
  fi

  printf 'placeholders=%d, stacked_frontmatter=%d, bundled_multi_learning=%d, duplicate_headings=%d, stale_contradictions=%d' \
    "$placeholders" \
    "$stacked" \
    "$bundled" \
    "$duplicate_headings" \
    "$stale_contradictions"
}

# Lightweight defrag only — prune and dedup, no mining
if ao defrag --prune --dedup 2>/dev/null; then
  STATUS="completed"
else
  STATUS="failed (non-fatal)"
fi

ROOT="$(repo_root)"
DEFECT_SUMMARY="$(scan_normalization_defects "$ROOT")"

echo "{\"hookSpecificOutput\":{\"hookEventName\":\"SessionEnd\",\"additionalContext\":\"Athena defrag ${STATUS}; normalization defects: ${DEFECT_SUMMARY}\"}}"
