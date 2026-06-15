#!/usr/bin/env bash
set -euo pipefail
file="${1:?Usage: check-no-main-push.sh <shell-script>}"
bad=0
while IFS= read -r line; do
  line="${line%%#*}"
  if [[ "$line" =~ git[[:space:]]+push([^#]*[[:space:]])?origin[[:space:]]+main([[:space:]]|$) ]]; then
    echo "direct main push: $line"
    bad=1
  fi
done < "$file"
exit "$bad"
