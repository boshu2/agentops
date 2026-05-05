#!/usr/bin/env bash
set -euo pipefail

# Structural quality assessment of a SKILL.md file.
# Emits a single JSON line to stdout with quality dimensions.
#
# Usage: bash check-skill-comprehension.sh <path-to-SKILL.md>

file="${1:?Usage: check-skill-comprehension.sh <SKILL.md path>}"

if [[ ! -f "$file" ]]; then
  printf '{"error":"file not found: %s"}\n' "$file" >&2
  exit 1
fi

content=$(cat "$file")

# --- Checks ---

has_steps=false
if printf '%s\n' "$content" | grep -iqE '^##\s+(Execution Steps|Steps|Execution)'; then
  has_steps=true
fi

has_flags=false
if printf '%s\n' "$content" | grep -iqE '^##\s+(Flags|Arguments|Options)'; then
  has_flags=true
fi

has_examples=false
if printf '%s\n' "$content" | grep -iqE '^##\s+(Examples|Quick Start)'; then
  has_examples=true
fi

has_output_spec=false
if printf '%s\n' "$content" | grep -iqE '(completion marker|output|artifact|<promise>)'; then
  has_output_spec=true
fi

word_count=$(wc -w < "$file" | tr -d '[:space:]')
word_count=$((word_count + 0))  # ensure integer

sufficient_length=false
if (( word_count > 100 )); then
  sufficient_length=true
fi

# --- Score ---

checks_passed=0
for check in "$has_steps" "$has_flags" "$has_examples" "$has_output_spec" "$sufficient_length"; do
  if [[ "$check" == "true" ]]; then
    (( checks_passed++ )) || true
  fi
done

# quality_score = checks_passed / 5, two decimal places
quality_score=$(awk "BEGIN { printf \"%.2f\", $checks_passed / 5 }")

# --- Output ---

if command -v jq >/dev/null 2>&1; then
  jq -n \
    --argjson has_steps "$has_steps" \
    --argjson has_flags "$has_flags" \
    --argjson has_examples "$has_examples" \
    --argjson has_output_spec "$has_output_spec" \
    --argjson word_count "$word_count" \
    --argjson sufficient_length "$sufficient_length" \
    --argjson quality_score "$quality_score" \
    '{has_steps: $has_steps, has_flags: $has_flags, has_examples: $has_examples, has_output_spec: $has_output_spec, word_count: $word_count, sufficient_length: $sufficient_length, quality_score: $quality_score}'
else
  printf '{"has_steps":%s,"has_flags":%s,"has_examples":%s,"has_output_spec":%s,"word_count":%d,"sufficient_length":%s,"quality_score":%s}\n' \
    "$has_steps" "$has_flags" "$has_examples" "$has_output_spec" "$word_count" "$sufficient_length" "$quality_score"
fi

exit 0
