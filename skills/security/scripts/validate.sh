#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"

[[ -s "$SKILL" ]]
grep -q '^name: security$' "$SKILL"
grep -q '^  effects: \[\]$' "$SKILL"
[[ "$(awk '/^---$/{n++;next} n==2 && /^## /{print;exit}' "$SKILL")" == "## Critical Constraints" ]]
grep -Fq '**Artifact directory:**' "$SKILL"
grep -Fq '**Validator command:**' "$SKILL"
grep -Fq 'report stops after evidence' "$SKILL"
! grep -Eiq 'AUTO-REDO|ONE-HELPER|HELPER-ESCALATE|ao (pawl|land)|next_action' "$SKILL"

[[ -s "$SKILL_DIR/references/policy-example.json" ]]
[[ -s "$SKILL_DIR/references/agentops-redteam-pack.json" ]]
[[ -s "$SKILL_DIR/references/security-suite-runbook.md" ]]
python3 -m py_compile "$SKILL_DIR/scripts/security_suite.py" "$SKILL_DIR/scripts/prompt_redteam.py"
python3 -c 'import json, pathlib, sys; root=pathlib.Path(sys.argv[1]); [json.loads((root/name).read_text()) for name in ("policy-example.json", "agentops-redteam-pack.json")]' "$SKILL_DIR/references"

echo "security contract: PASS"
