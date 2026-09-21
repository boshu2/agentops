#!/usr/bin/env bash
# Compatibility entrypoint for maintained Claude Code structural/install proof.
# Archived print helpers cannot establish live skill invocation or tool ordering.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Skill registration: Tier S structural proof only; live invocation is not checked."
exec bash "$SCRIPT_DIR/../skills/test-runtime-claude-code-smoke.sh"
