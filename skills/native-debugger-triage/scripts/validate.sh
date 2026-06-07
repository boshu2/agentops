#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_dir="$(cd "${script_dir}/.." && pwd)"
skill_md="${skill_dir}/SKILL.md"
spec_json="${skill_dir}/skill.spec.json"
skill_name="native-debugger-triage"

fail() {
  printf '%s validation failed: %s\n' "${skill_name}" "$1" >&2
  exit 1
}

[[ -f "${skill_md}" ]] || fail "missing SKILL.md"
[[ -f "${spec_json}" ]] || fail "missing skill.spec.json"
python3 -m json.tool "${spec_json}" >/dev/null || fail "skill.spec.json is not valid JSON"

python3 - "${skill_md}" "${spec_json}" "${skill_name}" <<'PY'
import json
import re
import sys
from pathlib import Path

skill_md = Path(sys.argv[1])
spec_json = Path(sys.argv[2])
skill_name = sys.argv[3]

def fail(msg):
    raise SystemExit(f"{skill_name} validation failed: {msg}")

text = skill_md.read_text(encoding="utf-8")
match = re.match(r"\A---\n(.*?)\n---\n", text, re.S)
if not match:
    fail("missing closed YAML frontmatter")
fm = match.group(1)

checks = [
    (rf"(?m)^name:\s*{re.escape(skill_name)}\s*$", "name"),
    (r"(?m)^description:\s*", "description"),
    (r"Triggers:", "description Triggers"),
    (r"(?m)^skill_api_version:\s*1\s*$", "skill_api_version"),
    (r"(?m)^user-invocable:\s*false\s*$", "user-invocable"),
    (r"(?m)^context:\s*$", "context"),
    (r"(?m)^\s+window:\s*\S+", "context.window"),
    (r"(?m)^metadata:\s*$", "metadata"),
    (r"(?m)^\s+tier:\s*\S+", "metadata.tier"),
    (r"(?m)^\s+stability:\s*\S+", "metadata.stability"),
    (r"(?m)^\s+dependencies:\s*(\[.*\]|$)", "metadata.dependencies"),
    (r"(?m)^output_contract:\s*", "output_contract"),
]
for pattern, label in checks:
    if not re.search(pattern, fm):
        fail(f"missing {label}")

spec = json.loads(spec_json.read_text(encoding="utf-8"))
if spec.get("name") != skill_name:
    fail("skill.spec.json name mismatch")
if spec.get("skill_api_version") != 1:
    fail("skill.spec.json skill_api_version must be 1")
if spec.get("entrypoint") != "SKILL.md":
    fail("skill.spec.json entrypoint must be SKILL.md")
if spec.get("user_invocable") is not False:
    fail("skill.spec.json user_invocable must be false")
if spec.get("validation", {}).get("command") != f"bash skills/{skill_name}/scripts/validate.sh":
    fail("skill.spec.json validation command mismatch")
print(f"{skill_name} validation passed")
PY
