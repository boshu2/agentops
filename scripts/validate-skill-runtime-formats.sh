#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Skill runtime format validation ==="

# Every runtime loads the one skills/ tree, so one format lint covers them all.
echo "--- Skill format ---"
bash ./tests/skills/lint-skills.sh

echo "Skill runtime format validation passed."
