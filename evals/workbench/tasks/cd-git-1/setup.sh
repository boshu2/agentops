#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$WORKDIR"
cp "$SCRIPT_DIR/prompt.md" "$WORKDIR/prompt.md"
cat > "$WORKDIR/sample.sh" <<'SH'
git push -u origin task/ag-demo
SH
