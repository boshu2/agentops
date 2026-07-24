#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: validate$' "$skill_dir/SKILL.md"
grep -Fq 'PASS`, `FAIL`, or `NOT_PROVEN`' "$skill_dir/SKILL.md"
grep -Fq 'sole verdict writer' "$skill_dir/SKILL.md"
grep -Fq 'verdict.v3' "$skill_dir/SKILL.md"
grep -Fq 'Never re-snapshot intent during storage' "$skill_dir/SKILL.md"

PYTHONDONTWRITEBYTECODE=1 python3 "$skill_dir/scripts/validate_v3.py" --help >/dev/null
PYTHONDONTWRITEBYTECODE=1 python3 "$skill_dir/scripts/record_proof_transition.py" --help >/dev/null
python3 - "$skill_dir" <<'PY'
import json
import sys
from pathlib import Path
from jsonschema import Draft202012Validator

skill = Path(sys.argv[1]).resolve()
root = next(
    (
        candidate
        for candidate in skill.parents
        if (candidate / "schemas" / "verdict.v3.schema.json").is_file()
    ),
    None,
)
if root is None:
    raise SystemExit("cannot locate AgentOps schemas")
for name in (
    "subject-manifest.v2.schema.json",
    "scope-index.v1.schema.json",
    "check-receipt.v1.schema.json",
    "effect-receipt.v1.schema.json",
    "verdict.v3.schema.json",
    "rpi-report.v2.schema.json",
):
    Draft202012Validator.check_schema(
        json.loads((root / "schemas" / name).read_text(encoding="utf-8"))
    )
PY

PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$skill_dir/scripts" -p 'test_kernel_v3.py'
PYTHONDONTWRITEBYTECODE=1 python3 "$skill_dir/scripts/check_kernel_v3_corpus.py"
echo 'validate skill contract: PASS'
