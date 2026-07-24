#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: plan$' "$skill_dir/SKILL.md"
grep -Fq 'mint exactly one snapshot' "$skill_dir/SKILL.md"
grep -Fq 'never re-read the living source' "$skill_dir/SKILL.md"
grep -Fq 'Planning produces no AgentOps plan packet' "$skill_dir/SKILL.md"
! grep -Fq 'plan-packet.v1' "$skill_dir/SKILL.md"
python3 "$skill_dir/scripts/mint_intent.py" --help >/dev/null
echo 'plan skill contract: PASS'
