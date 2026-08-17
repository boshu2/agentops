#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: research$' "$skill_dir/SKILL.md"
grep -Fq 'Answer one bounded question with current evidence' "$skill_dir/SKILL.md"
grep -Fq 'Report unchecked scope and stop' "$skill_dir/SKILL.md"
grep -Fq 'Do not emit approval' "$skill_dir/SKILL.md"
grep -Fq 'Build a source ledger before comparing claims' "$skill_dir/SKILL.md"
grep -Fq 'Produce one cited synthesis' "$skill_dir/SKILL.md"
grep -Fq 'Do not recursively launch another Research pass' "$skill_dir/SKILL.md"
grep -Fq 'Freeze local paths, external domains, data classes' "$skill_dir/SKILL.md"
grep -Fq 'Credentials default to none' "$skill_dir/SKILL.md"
grep -Fq 'Report observed effects even for a quick answer' "$skill_dir/SKILL.md"
grep -Fq '"source_ledger"' "$skill_dir/schemas/findings.json"
grep -Fq '"comparison"' "$skill_dir/schemas/findings.json"
grep -Fq '"effects"' "$skill_dir/schemas/findings.json"
test -x "$skill_dir/scripts/validate-output.sh"
grep -q '^Feature: Research answers one bounded question$' \
  "$skill_dir/references/research.feature"
grep -Fq 'Scenario: Multiple caller-supplied reports are synthesized once' \
  "$skill_dir/references/research.feature"
grep -Fq 'agreement, contradiction, and unknown are reported separately' \
  "$skill_dir/references/research.feature"
python3 -m json.tool "$skill_dir/schemas/findings.json" >/dev/null
python3 - "$skill_dir/schemas/findings.json" <<'PY'
import copy
import json
import sys
from jsonschema import Draft7Validator

schema = json.load(open(sys.argv[1], encoding="utf-8"))
validator = Draft7Validator(schema)
base = {
    "topic": "bounded question",
    "summary": "bounded answer",
    "findings": [],
    "checked": ["local source"],
    "not_checked": [],
    "effects": {
        "local_paths_read": ["README.md"],
        "network": {"authorization_id": None, "domains": [], "methods": [], "request_count": 0, "bytes_fetched": 0, "credentials_used": []},
        "writes": [],
        "sensitive_data": "none",
        "sensitive_output_approval_id": None,
    },
    "schema_version": 1,
}
validator.validate(base)
missing_approval = copy.deepcopy(base)
missing_approval["effects"]["network"].update({"domains": ["example.com"], "methods": ["GET"], "request_count": 1, "bytes_fetched": 10})
if not list(validator.iter_errors(missing_approval)):
    raise SystemExit("research schema accepted network access without authorization")
oversized = copy.deepcopy(base)
oversized["effects"]["network"].update({"authorization_id": "caller:1", "domains": ["example.com"], "methods": ["GET"], "request_count": 1, "bytes_fetched": 2097153})
if not list(validator.iter_errors(oversized)):
    raise SystemExit("research schema accepted oversized network retrieval")
sensitive = copy.deepcopy(base)
sensitive["effects"]["sensitive_data"] = "approved"
if not list(validator.iter_errors(sensitive)):
    raise SystemExit("research schema accepted sensitive output without approval")
print("research effects schema cases: PASS")
PY

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
python3 - "$fixture_dir" <<'PY'
import copy
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
base = {
    "topic": "bounded question",
    "summary": "bounded answer",
    "findings": [],
    "checked": ["local source"],
    "not_checked": [],
    "effects": {
        "local_paths_read": ["README.md"],
        "network": {"authorization_id": None, "domains": [], "methods": [], "request_count": 0, "bytes_fetched": 0, "credentials_used": []},
        "writes": ["research-findings.json"],
        "sensitive_data": "none",
        "sensitive_output_approval_id": None,
    },
    "schema_version": 1,
}
def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")
emit("local.json", base)
missing_auth = copy.deepcopy(base)
missing_auth["effects"]["network"].update({"domains": ["example.com"], "methods": ["GET"], "request_count": 1, "bytes_fetched": 10})
emit("missing-auth.json", missing_auth)
oversized = copy.deepcopy(base)
oversized["effects"]["network"].update({"authorization_id": "caller:research", "domains": ["example.com"], "methods": ["GET"], "request_count": 1, "bytes_fetched": 2097153})
emit("oversized.json", oversized)
sensitive = copy.deepcopy(base)
sensitive["effects"]["sensitive_data"] = "approved"
emit("sensitive.json", sensitive)
unbounded_text = copy.deepcopy(base)
unbounded_text["summary"] = "x" * 16385
emit("unbounded-text.json", unbounded_text)
secret_excerpt = copy.deepcopy(base)
secret_excerpt["summary"] = "Authorization: Bearer abcdefghijklmnopqrstuvwxyz"
emit("secret-excerpt.json", secret_excerpt)
PY
bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/local.json"
for rejected in missing-auth oversized sensitive unbounded-text secret-excerpt; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "research contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

if rg -n 'ao lookup|ao land|auto-redo|Gate 1|\.agents/rpi/next-work|finding-compiler' \
  "$skill_dir/SKILL.md" "$skill_dir/references" "$skill_dir/schemas"; then
  echo 'research contract contains retired lifecycle behavior' >&2
  exit 1
fi

echo 'research skill contract: PASS'
