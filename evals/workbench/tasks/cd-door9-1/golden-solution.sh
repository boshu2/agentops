#!/usr/bin/env bash
set -euo pipefail
root="${1:?Usage: check-no-claude-print.sh <search-root>}"
matches="$(grep -RIlE 'claude[[:space:]]+(-p|--print)([[:space:]]|$)' "$root" 2>/dev/null || true)"
if [[ -n "$matches" ]]; then
  printf '%s\n' "$matches"
  exit 1
fi
exit 0
