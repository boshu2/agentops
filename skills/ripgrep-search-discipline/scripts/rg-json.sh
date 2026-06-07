#!/usr/bin/env bash
# rg-json.sh — run ripgrep in JSON mode and emit path:line:text tuples.
# Usage: rg-json.sh PATTERN [PATH ...]
# Exit 0 = matches, 1 = no matches, 2 = error (mirrors rg's own contract).
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: rg-json.sh PATTERN [PATH ...]" >&2
  exit 2
fi

if ! command -v rg >/dev/null 2>&1; then
  echo "rg-json.sh: ripgrep (rg) not found on PATH" >&2
  exit 2
fi

pattern="$1"; shift

# rg exits 1 on no-match; preserve that through the pipeline.
set +e
rg --json "$pattern" "$@"
rc=$?
set -e

if [[ $rc -eq 2 ]]; then
  exit 2
fi
exit "$rc"
