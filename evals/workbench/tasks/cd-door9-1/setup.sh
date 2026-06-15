#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$WORKDIR/sample"
cp "$SCRIPT_DIR/prompt.md" "$WORKDIR/prompt.md"
echo 'codex exec "fix it"' > "$WORKDIR/sample/runner.sh"
