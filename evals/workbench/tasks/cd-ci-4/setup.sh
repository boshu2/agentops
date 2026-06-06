#!/usr/bin/env bash
# cd-ci-4 setup: workspace with the task prompt + a small sample tree that
# references a CI job name (so the agent sees the shape of the problem).
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$WORKDIR/sample/evals" "$WORKDIR/sample/docs"
cp "$SCRIPT_DIR/prompt.md" "$WORKDIR/prompt.md"
echo '{"artifact_contains": ["sample-gate"]}' > "$WORKDIR/sample/evals/gates.json"
echo "The sample-gate job runs on every PR." > "$WORKDIR/sample/docs/ci.md"
