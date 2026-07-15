#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
grep -q '^name: plan$' "$skill_dir/SKILL.md"
grep -Fq 'PlanPacket' "$skill_dir/SKILL.md"
python3 -m json.tool "$repo_root/schemas/plan-packet.v1.schema.json" >/dev/null
! grep -Eiq 'ao |\bbr\b|beads|claim|queue|lease|delivery|release' "$skill_dir/SKILL.md"
echo 'plan skill contract: PASS'
