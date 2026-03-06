#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Verify SKILL.md exists and has frontmatter
[[ -f "$SKILL_DIR/SKILL.md" ]] || { echo "FAIL: missing SKILL.md"; exit 1; }
head -1 "$SKILL_DIR/SKILL.md" | grep -q "^---$" || { echo "FAIL: missing frontmatter"; exit 1; }
echo "OK: $(basename "$SKILL_DIR")"
