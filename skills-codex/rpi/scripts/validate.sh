#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: rpi$' "$skill_dir/SKILL.md"
grep -Fq 'Plan -> Implement -> fresh Validate -> report' "$skill_dir/SKILL.md"
grep -Fq 'dispatches each core phase at most once' "$skill_dir/SKILL.md"
grep -Fq 'rpi-report.v2' "$skill_dir/SKILL.md"
grep -Fq 'append a next action' "$skill_dir/SKILL.md"
! grep -Fq 'Continuation envelope' "$skill_dir/SKILL.md"
! grep -Fq 'repair revision per wave' "$skill_dir/SKILL.md"
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$skill_dir/tests" -p 'test_*.py'
echo 'rpi skill contract: PASS'
