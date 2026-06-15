#!/usr/bin/env bash
set -euo pipefail
file="${1:?Usage: check-br-beads-dir.sh <shell-script>}"
bad=0
while IFS= read -r line; do
  line="${line%%#*}"
  [[ "$line" =~ (^|[[:space:]\;\&\|])br[[:space:]]+ ]] || continue
  if [[ ! "$line" =~ BEADS_DIR=[^[:space:]\;\&\|]*_beads[\"\']?[[:space:]]+br[[:space:]]+ ]]; then
    echo "unscoped br command: $line"
    bad=1
  fi
done < "$file"
exit "$bad"
