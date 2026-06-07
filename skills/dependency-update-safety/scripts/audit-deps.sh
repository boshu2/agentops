#!/usr/bin/env bash
# audit-deps.sh — list outdated dependencies for the detected ecosystem.
# Usage: audit-deps.sh [project-dir]   (default: cwd)
# Output: one line per behind package, plus the detected ecosystem.
# Exit 0 = audit ran (with or without findings); exit 2 = no manifest found.
set -uo pipefail

DIR="${1:-.}"
cd "$DIR" 2>/dev/null || { echo "ERROR: cannot cd to $DIR" >&2; exit 2; }

echo "# library-updater audit — $(pwd)"

found=0

if [ -f package.json ]; then
  found=1
  echo "## ecosystem: npm"
  if command -v npm >/dev/null 2>&1; then
    # `npm outdated` exits 1 when packages are behind; that is expected, not an error.
    npm outdated || true
  else
    echo "npm not installed; read package.json manually" >&2
  fi
fi

if [ -f go.mod ]; then
  found=1
  echo "## ecosystem: go"
  if command -v go >/dev/null 2>&1; then
    go list -u -m all 2>/dev/null | grep '\[' || echo "(no updates reported)"
  else
    echo "go not installed; read go.mod manually" >&2
  fi
fi

if [ -f pyproject.toml ] || [ -f requirements.txt ]; then
  found=1
  echo "## ecosystem: python"
  if command -v pip >/dev/null 2>&1; then
    pip list --outdated 2>/dev/null || echo "(pip list --outdated produced nothing)"
  else
    echo "pip not installed; read the manifest manually" >&2
  fi
fi

if [ -f Cargo.toml ]; then
  found=1
  echo "## ecosystem: cargo"
  if command -v cargo >/dev/null 2>&1; then
    cargo update --dry-run 2>&1 | grep -i 'updating\|latest' || echo "(no updates reported)"
  else
    echo "cargo not installed; read Cargo.toml manually" >&2
  fi
fi

if [ "$found" -eq 0 ]; then
  echo "ERROR: no supported manifest found in $(pwd)" >&2
  echo "supported: package.json, go.mod, pyproject.toml/requirements.txt, Cargo.toml" >&2
  exit 2
fi

echo "# done — group findings by jump type (patch/minor/major) before applying"
exit 0
