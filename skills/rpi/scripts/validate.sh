#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: rpi$' "$skill_dir/SKILL.md"
grep -Fq 'Plan -> Implement -> fresh Validate -> report' "$skill_dir/SKILL.md"
grep -Fq 'dispatches each core phase at most once' "$skill_dir/SKILL.md"
! grep -Eiq 'retry|queue|lease|claim|git (commit|push)|delivery transition|ao land|pawl' "$skill_dir/SKILL.md"
echo 'rpi skill contract: PASS'
