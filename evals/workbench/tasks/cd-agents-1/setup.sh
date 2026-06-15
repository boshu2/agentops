#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$WORKDIR"
cp "$SCRIPT_DIR/prompt.md" "$WORKDIR/prompt.md"
cat > "$WORKDIR/changed-files.txt" <<'TXT'
.agents/specs/2026-06-14-yield-vector.md
docs/3.0.md
TXT
