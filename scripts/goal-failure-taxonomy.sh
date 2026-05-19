#!/usr/bin/env bash
set -euo pipefail

# goal-failure-taxonomy.sh
# Classify failing goals from ao fitness JSON into actionable categories.
#
# Usage:
#   scripts/goal-failure-taxonomy.sh [fitness-json]
# If omitted, uses latest .agents/evolve/fitness-*-pre.json

INPUT="${1:-}"

if [[ -z "$INPUT" ]]; then
  INPUT="$(ls -t .agents/evolve/fitness-*-pre.json 2>/dev/null | head -1 || true)"
fi

if [[ -z "$INPUT" || ! -f "$INPUT" ]]; then
  echo "ERROR: fitness JSON not found. Provide a file path or generate .agents/evolve/fitness-*-pre.json." >&2
  exit 1
fi

if ! jq -e . "$INPUT" >/dev/null 2>&1; then
  echo "ERROR: invalid JSON: $INPUT" >&2
  exit 1
fi

classify() {
  local id="${1:-}"
  case "$id" in
    *security*|*secret*|*hook-preflight*)
      echo "security"
      ;;
    *build*|*test*|*vet*|*wiring*|*evolve*|*rpi*|*session-start*|*kill-switch*)
      echo "reliability"
      ;;
    *coverage*|*complexity*|*smoke*|*opencode*|*skill-validation*|*semantic-stability*|*frontmatter*)
      echo "quality"
      ;;
    *goal-*|*pillar-*|*product-freshness*|*manifest-versions-match*|*ao-goals*)
      echo "governance"
      ;;
    *)
      echo "other"
      ;;
  esac
}

TMP="$(mktemp)"
jq -c '.goals[] | select(.result=="fail") | {goal_id, description}' "$INPUT" > "$TMP"

TOTAL="$(wc -l < "$TMP" | tr -d ' ')"

if [[ "$TOTAL" == "0" ]]; then
  jq -n '{summary:{total_failing:0,by_category:{}},failing_goals:[]}'
  rm -f "$TMP"
  exit 0
fi

OUT_TMP="$(mktemp)"
while IFS= read -r row; do
  gid="$(jq -r '.goal_id // ""' <<<"$row")"
  desc="$(jq -r '.description // ""' <<<"$row")"
  cat="$(classify "$gid")"
  jq -n --arg goal_id "$gid" --arg description "$desc" --arg category "$cat" \
    '{goal_id:$goal_id,category:$category,description:$description}' >> "$OUT_TMP"
done < "$TMP"

jq -s '
  {
    summary: {
      total_failing: length,
      by_category: (group_by(.category) | map({key: .[0].category, value: length}) | from_entries)
    },
    failing_goals: .
  }
' "$OUT_TMP"

rm -f "$TMP" "$OUT_TMP"
