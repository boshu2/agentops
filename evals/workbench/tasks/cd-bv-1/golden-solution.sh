#!/usr/bin/env bash
set -euo pipefail
file="${1:?Usage: check-bv-robot-mode.sh <shell-script>}"
bad=0
while IFS= read -r line; do
  line="${line%%#*}"
  [[ "$line" =~ (^|[[:space:]\;\&\|])bv([[:space:]]|$) ]] || continue
  if [[ ! "$line" =~ (^|[[:space:]\;\&\|])bv[[:space:]]+--robot-[A-Za-z0-9_-]+ ]]; then
    echo "bare bv command: $line"
    bad=1
  fi
done < "$file"
exit "$bad"
