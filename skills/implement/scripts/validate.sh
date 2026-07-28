#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
grep -q '^name: implement$' "$skill_dir/SKILL.md"
# test_runtime_derives_subject
grep -Fq 'exactly one bounded experiment' "$skill_dir/SKILL.md"
grep -Fq 'runtime derive actual changed paths' "$skill_dir/SKILL.md"
if grep -Fq 'candidate-packet.v1' "$skill_dir/SKILL.md"; then
  echo 'implement skill contract: candidate-packet.v1 is retired' >&2
  exit 1
fi
bats "$repo_root/tests/scripts/tranche-candidate-boundary.bats" >/dev/null
echo 'implement skill contract: PASS'
