#!/usr/bin/env bash
# run-tests.sh — run the project's test gate for the detected ecosystem.
# Usage: run-tests.sh [project-dir]   (default: cwd)
# Exit code IS the verdict: 0 = green (keep the batch), non-zero = red (roll back).
set -uo pipefail

DIR="${1:-.}"
cd "$DIR" 2>/dev/null || { echo "ERROR: cannot cd to $DIR" >&2; exit 2; }

# Prefer an explicit Make target if the repo declares one.
if [ -f Makefile ] && grep -qE '^test:' Makefile; then
  echo "== make test =="
  exec make test
fi

if [ -f package.json ]; then
  echo "== npm test =="
  exec npm test
fi

if [ -f go.mod ]; then
  echo "== go test ./... =="
  exec go test ./...
fi

if [ -f pyproject.toml ] || [ -f requirements.txt ]; then
  echo "== pytest =="
  if command -v pytest >/dev/null 2>&1; then exec pytest; fi
  exec python -m pytest
fi

if [ -f Cargo.toml ]; then
  echo "== cargo test =="
  exec cargo test
fi

echo "ERROR: no recognized test command for $(pwd)" >&2
echo "set up a Makefile 'test' target or run the suite manually" >&2
exit 2
