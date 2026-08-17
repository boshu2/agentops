#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: plan$' "$skill_dir/SKILL.md"
# test_no_model_authored_packet
grep -Fq "Prefer the caller's tracker, if any" "$skill_dir/SKILL.md"
grep -Fq 'Planning produces no AgentOps packet' "$skill_dir/SKILL.md"
# test_control_bounds
grep -Fq 'Commands require declared authority' "$skill_dir/SKILL.md"
grep -Fq 'Control experiments are disposable and finite' "$skill_dir/SKILL.md"
grep -Fq 'terminates and reaps the whole process group' "$skill_dir/SKILL.md"
grep -Fq 'No failed restoration is hidden' "$skill_dir/SKILL.md"
grep -Fq 'digest check failure is reported as a blocked control experiment' "$skill_dir/SKILL.md"
if grep -Fq 'plan-packet.v1' "$skill_dir/SKILL.md"; then
  echo 'plan contract references a model-authored plan packet' >&2
  exit 1
fi
echo 'plan skill contract: PASS'
