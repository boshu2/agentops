#!/usr/bin/env bash
# Headless RPI phase runner with timeouts.
# Chains Research → Implement phases non-interactively.
# Usage: ./scripts/run-rpi-phases.sh "description of work"
set -euo pipefail

# Shared fail-closed codex runner (STALL/ECHO/MISSING defenses + distinct exit
# codes). age-gate-the-ungated-egwt.8. `CDPATH=` is an intentional env-prefix
# (clears CDPATH for that one cd), not a botched assignment.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/codex-exec.sh"

TASK="${1:?Usage: run-rpi-phases.sh '<task description>'}"
SCRATCH_DIR="./scratch"
mkdir -p "$SCRATCH_DIR"

echo "=== RPI Phase 1: Research (10 min timeout) ==="
CODEX_EXEC_TIMEOUT=600 CODEX_EXEC_DIR="$(pwd)" CODEX_EXEC_SANDBOX=workspace-write CODEX_EXEC_EXPECT_OUTPUT=0 \
  CODEX_EXEC_PROMPT_ARG="Execute RPI Phase 1: research and write findings to disk for: ${TASK}. Do NOT plan or ask questions — just execute. Write output to ${SCRATCH_DIR}/research.md" \
  codex_exec_guarded \
  || echo "Phase 1 timed out or failed"

echo ""
echo "=== RPI Phase 2: Implement (15 min timeout) ==="
CODEX_EXEC_TIMEOUT=900 CODEX_EXEC_DIR="$(pwd)" CODEX_EXEC_SANDBOX=workspace-write CODEX_EXEC_EXPECT_OUTPUT=0 \
  CODEX_EXEC_PROMPT_ARG="Execute RPI Phase 2: implement changes for: ${TASK}. Use research from ${SCRATCH_DIR}/research.md if available. Run tests after each change. Do NOT stop to plan — implement and verify." \
  codex_exec_guarded \
  || echo "Phase 2 timed out or failed"

echo ""
echo "=== RPI Complete ==="
echo "Research: ${SCRATCH_DIR}/research.md"
echo "Verify: git diff --stat"
