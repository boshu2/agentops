#!/usr/bin/env bash
set -euo pipefail

# regression-bisect.sh
# Deterministic helper for bisecting regressions between a known-good and bad commit.
#
# Usage:
#   scripts/regression-bisect.sh --good <sha> --bad <sha> --check "<command>"
# Example:
#   scripts/regression-bisect.sh --good abc123 --bad def456 --check "go test ./internal/gates/..."

GOOD=""
BAD=""
CHECK=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --good)
      GOOD="${2:-}"
      shift 2
      ;;
    --bad)
      BAD="${2:-}"
      shift 2
      ;;
    --check)
      CHECK="${2:-}"
      shift 2
      ;;
    -h|--help)
      sed -n '1,20p' "$0"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "$GOOD" || -z "$BAD" || -z "$CHECK" ]]; then
  echo "Missing required arguments. Use --good, --bad, and --check." >&2
  exit 2
fi

echo "Starting regression bisect"
echo "  good:  $GOOD"
echo "  bad:   $BAD"
echo "  check: $CHECK"

git bisect reset >/dev/null 2>&1 || true
git bisect start "$BAD" "$GOOD"

set +e
git bisect run bash -lc "$CHECK"
RC=$?
set -e

echo
echo "Bisect complete. Current commit is the first bad commit candidate:"
git rev-parse --short HEAD

git bisect reset >/dev/null 2>&1 || true
exit "$RC"
