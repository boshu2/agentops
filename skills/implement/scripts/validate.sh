#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: implement$' "$skill_dir/SKILL.md"
grep -Fq 'exactly one bounded experiment' "$skill_dir/SKILL.md"
grep -Fq 'repository root' "$skill_dir/SKILL.md"
grep -Fq 'Any later subject mutation is terminal' "$skill_dir/SKILL.md"
! grep -Fq 'repair revision of the intent' "$skill_dir/SKILL.md"
! grep -Fq 'candidate-packet.v1' "$skill_dir/SKILL.md"
PYTHONDONTWRITEBYTECODE=1 python3 "$skill_dir/scripts/freeze_candidate.py" --help >/dev/null
echo 'implement skill contract: PASS'
