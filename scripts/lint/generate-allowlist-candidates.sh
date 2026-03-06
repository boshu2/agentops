#!/usr/bin/env bash
# Generate candidate allowlist entries from converted codex skills.
# Run AFTER conversion, BEFORE validation.
# Usage: generate-allowlist-candidates.sh <converted-skills-dir>
set -euo pipefail

CONVERTED_DIR="${1:?Usage: generate-allowlist-candidates.sh <converted-skills-dir>}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ALLOWLIST="${SCRIPT_DIR}/codex-residual-allowlist.txt"

# Find unallowlisted markers using the same matching rules as
# validate-codex-runtime-sections.sh so warnings align with the blocking gate.
mapfile -t candidates < <(
  find "$CONVERTED_DIR" -name "SKILL.md" -type f | sort | xargs awk -v allowlist_file="$ALLOWLIST" '
function normalize_word_boundaries(pattern,    n, i, out, parts) {
  n = split(pattern, parts, /\\b/)
  if (n == 1) {
    return pattern
  }

  out = ""
  for (i = 1; i <= n; i++) {
    out = out parts[i]
    if (i < n) {
      if (i % 2 == 1) {
        out = out "(^|[^[:alnum:]_])"
      } else {
        out = out "([^[:alnum:]_]|$)"
      }
    }
  }

  return out
}

function is_allowlisted(line,    i) {
  for (i = 1; i <= allowlist_count; i++) {
    if (line ~ allowlist_patterns[i]) {
      return 1
    }
  }
  return 0
}

BEGIN {
  while ((getline raw < allowlist_file) > 0) {
    if (raw ~ /^[[:space:]]*#/ || raw ~ /^[[:space:]]*$/) {
      continue
    }
    allowlist_count++
    allowlist_patterns[allowlist_count] = normalize_word_boundaries(raw)
  }
  close(allowlist_file)
}

{
  if ($0 ~ /(^|[^[:alnum:]_])([Cc]laude|[Aa]nthropic|team-create|send-message)([^[:alnum:]_]|$)/) {
    if (!is_allowlisted($0)) {
      split(FILENAME, path_parts, "/")
      skill_name = path_parts[length(path_parts) - 1]
      printf "# %s: %d:%s\n", skill_name, FNR, $0
    }
  }
}
'
)

if [[ ${#candidates[@]} -eq 0 ]]; then
  echo "No unallowlisted residual markers found."
  exit 0
fi

echo "Found ${#candidates[@]} unallowlisted residual markers:"
printf '%s\n' "${candidates[@]}"
echo ""
echo "Add patterns to $ALLOWLIST to allow these, or fix the converter rules."
exit 1
