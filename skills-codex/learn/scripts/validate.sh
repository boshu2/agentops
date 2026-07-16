#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: learn$' "$skill_dir/SKILL.md"
grep -Fq 'optional, off-path consumer' "$skill_dir/SKILL.md"
! grep -Eiq 'receipt|plan_impact|next_action|retry|delivery|closure' "$skill_dir/SKILL.md"
echo 'learn skill contract: PASS'
