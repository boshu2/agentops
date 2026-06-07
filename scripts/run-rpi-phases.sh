#!/usr/bin/env bash
# Headless RPI phase runner with timeouts.
# Chains Research → Implement phases non-interactively.
# Usage: ./scripts/run-rpi-phases.sh "description of work"
set -euo pipefail

TASK="${1:?Usage: run-rpi-phases.sh '<task description>'}"
SCRATCH_DIR="./scratch"
mkdir -p "$SCRATCH_DIR"

echo "=== RPI Phase 1: Research (10 min timeout) ==="
timeout 600 codex exec -C "$(pwd)" -s workspace-write \
  "Execute RPI Phase 1: research and write findings to disk for: ${TASK}. Do NOT plan or ask questions — just execute. Write output to ${SCRATCH_DIR}/research.md" \
  || echo "Phase 1 timed out or failed"

echo ""
echo "=== RPI Phase 2: Implement (15 min timeout) ==="
timeout 900 codex exec -C "$(pwd)" -s workspace-write \
  "Execute RPI Phase 2: implement changes for: ${TASK}. Use research from ${SCRATCH_DIR}/research.md if available. Run tests after each change. Do NOT stop to plan — implement and verify." \
  || echo "Phase 2 timed out or failed"

echo ""
echo "=== RPI Complete ==="
echo "Research: ${SCRATCH_DIR}/research.md"
echo "Verify: git diff --stat"
