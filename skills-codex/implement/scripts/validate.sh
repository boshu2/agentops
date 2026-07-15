#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
grep -q '^name: implement$' "$skill_dir/SKILL.md"
grep -Fq 'exactly one bounded experiment' "$skill_dir/SKILL.md"
python3 -m json.tool "$repo_root/schemas/candidate-packet.v1.schema.json" >/dev/null
! grep -Eiq 'pawl|ao |\bbr\b|beads|claim|queue|lease|git (commit|push)|close|release|land' "$skill_dir/SKILL.md"
echo 'implement skill contract: PASS'
