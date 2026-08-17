#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: implement$' "$skill_dir/SKILL.md"
# test_runtime_derives_subject
grep -Fq 'exactly one bounded experiment' "$skill_dir/SKILL.md"
grep -Fq 'runtime derive actual changed paths' "$skill_dir/SKILL.md"
# test_bounded_commands
grep -Fq 'exact argv already named as an acceptance check' "$skill_dir/SKILL.md"
grep -Fq 'whole process group and records explicit failure' "$skill_dir/SKILL.md"
grep -Fq 'caller-authorized disposable root' "$skill_dir/SKILL.md"
grep -Fq 'command-produced mutations are never copied back' "$skill_dir/SKILL.md"
if grep -Fq 'candidate-packet.v1' "$skill_dir/SKILL.md"; then
  echo 'implement contract references a model-authored candidate packet' >&2
  exit 1
fi
echo 'implement skill contract: PASS'
