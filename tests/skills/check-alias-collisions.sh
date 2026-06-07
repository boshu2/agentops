#!/usr/bin/env bash
# check-alias-collisions.sh — Detect duplicate trigger-to-skill mappings.
# Extracts "Triggers:" from each SKILL.md description, finds duplicates,
# and exits 1 if any NEW (non-allowlisted) collisions exist.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# Known intentional collisions (trigger|skill1|skill2 — sorted alpha)
# These are semantic overlaps where multiple skills legitimately handle the same phrase.
ALLOWLIST=(
  "brainstorm|brainstorm|council"
  "is this ready|pre-mortem|vibe"
  "research|council|research"
  "where did this come from|provenance|trace"
  "where was i|recover|status"
)

# Extract triggers from all SKILL.md files into a temp file: trigger\tskill
TRIGGER_MAP=$(mktemp)
trap 'rm -f "$TRIGGER_MAP"' EXIT

for skill_file in "$REPO_ROOT"/skills/*/SKILL.md; do
  skill_name=$(basename "$(dirname "$skill_file")")
  # Skip leading-underscore scaffolding (e.g. skills/_fixtures/) — planted
  # test fixtures, not real skills.
  case "$skill_name" in _*) continue ;; esac

  # Extract description from YAML frontmatter
  desc=$(sed -n '/^---$/,/^---$/p' "$skill_file" \
    | grep -i '^description:' \
    | head -1 \
    | sed "s/^description:[[:space:]]*['\"]\\{0,1\\}//; s/['\"]\\{0,1\\}$//" )

  if [[ -z "$desc" ]]; then
    continue
  fi

  # Extract triggers after "Triggers:" or "Trigger:"
  trigger_section=$(echo "$desc" | sed -n 's/.*[Tt]riggers\{0,1\}:[[:space:]]*//p')
  if [[ -z "$trigger_section" ]]; then
    continue
  fi

  # Remove trailing period
  trigger_section="${trigger_section%.}"

  # Parse quoted triggers first, fall back to comma-separated
  if echo "$trigger_section" | grep -q '"'; then
    # Extract quoted strings
    echo "$trigger_section" | grep -oE '"[^"]+"' | sed 's/"//g' | while read -r trigger; do
      echo "$(echo "$trigger" | tr '[:upper:]' '[:lower:]')	$skill_name" >> "$TRIGGER_MAP"
    done
  else
    # Comma-separated
    echo "$trigger_section" | tr ',' '\n' | while read -r trigger; do
      trigger=$(echo "$trigger" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
      [[ -n "$trigger" ]] && echo "$trigger	$skill_name" >> "$TRIGGER_MAP"
    done
  fi
done

skill_count=0
for counted_skill_file in "$REPO_ROOT"/skills/*/SKILL.md; do
  counted_skill_name=$(basename "$(dirname "$counted_skill_file")")
  case "$counted_skill_name" in _*) continue ;; esac
  skill_count=$((skill_count + 1))
done

# Find duplicates: triggers that map to multiple skills
COLLISIONS=$(sort "$TRIGGER_MAP" | awk -F'\t' '
{
  if ($1 in skills) {
    skills[$1] = skills[$1] "|" $2
    count[$1]++
  } else {
    skills[$1] = $2
    count[$1] = 1
  }
}
END {
  for (t in count) {
    if (count[t] > 1) {
      print t "\t" skills[t]
    }
  }
}' | sort)

if [[ -z "$COLLISIONS" ]]; then
  echo "PASS: No trigger collisions found ($(wc -l < "$TRIGGER_MAP" | tr -d ' ') triggers across $skill_count skills)"
  exit 0
fi

# Check each collision against the allowlist
new_collisions=0
while IFS=$'\t' read -r trigger skill_list; do
  # Sort skills alphabetically for consistent comparison
  sorted_skills=$(echo "$skill_list" | tr '|' '\n' | sort | tr '\n' '|' | sed 's/|$//')

  allowed=false
  for entry in "${ALLOWLIST[@]}"; do
    allowed_trigger="${entry%%|*}"
    allowed_skills="${entry#*|}"
    if [[ "$trigger" == "$allowed_trigger" && "$sorted_skills" == "$allowed_skills" ]]; then
      allowed=true
      break
    fi
  done

  if $allowed; then
    echo "OK:   '$trigger' -> [$sorted_skills] (allowlisted)"
  else
    echo "NEW:  '$trigger' -> [$sorted_skills] ← NOT in allowlist"
    new_collisions=$((new_collisions + 1))
  fi
done <<< "$COLLISIONS"

echo ""
if [[ "$new_collisions" -gt 0 ]]; then
  echo "FAIL: $new_collisions new trigger collision(s) found"
  echo "Either:"
  echo "  1. Remove the duplicate trigger from one skill's description"
  echo "  2. Add to ALLOWLIST in this script if the overlap is intentional"
  exit 1
else
  echo "PASS: All collisions are allowlisted ($(wc -l < "$TRIGGER_MAP" | tr -d ' ') triggers checked)"
  exit 0
fi
