#!/usr/bin/env bash
# cd-ci-1 setup: prepare an isolated workspace with the task prompt + a sample
# (clean) validate.yml so the agent sees the structure it must reason about.
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$WORKDIR"
cp "$SCRIPT_DIR/prompt.md" "$WORKDIR/prompt.md"
cat > "$WORKDIR/validate.yml" <<'YML'
jobs:
  correctness:
    continue-on-error: false
  lint:
    continue-on-error: true
  summary:
    needs: [correctness]
YML
