#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_dir="$(cd "${script_dir}/.." && pwd)"
skill_md="${skill_dir}/SKILL.md"
spec_json="${skill_dir}/skill.spec.json"
skill_name="storage-watchdog-ops"

fail() {
  printf '%s validation failed: %s\n' "${skill_name}" "$1" >&2
  exit 1
}

[ -f "${skill_md}" ] || fail "missing SKILL.md"
[ -f "${spec_json}" ] || fail "missing skill.spec.json"

python3 -m json.tool "${spec_json}" >/dev/null 2>&1 || fail "skill.spec.json is not valid JSON"

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
body = text[match.end():]

frontmatter_checks = [
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
    (r"(?m)^\s+(?:dependencies|external_dependencies):", "metadata.dependencies"),
    (r"(?m)^output_contract:\s*", "output_contract"),
]
for pattern, label in frontmatter_checks:
    if not re.search(pattern, fm):
        fail(f"missing frontmatter {label}")

# Behavioral: the required coverage terms must appear in the description so the
# corpus indexer/router can actually route on them.
required_triggers = ["storage", "watchdog", "disk", "target"]
desc_match = re.search(r"(?ms)^description:\s*(.*?)\n[a-z_]+:", fm)
desc = desc_match.group(1).lower() if desc_match else ""
for term in required_triggers:
    if term not in desc:
        fail(f"required trigger term '{term}' missing from description")

# Behavioral: this is an operator RUNBOOK, so the four runbook phases must exist.
for needle in ["Check status", "Interpret", "Remediate", "Escalate"]:
    if needle.lower() not in body.lower():
        fail(f"runbook section '{needle}' missing from body")

# Behavioral: the runbook must reference the real daemon controls, not generic
# disk advice — these substantiate it is grounded in the actual service.
grounding = [
    "acfs-storage-watchdog.service",
    "--once",
    "--dry-run",
    "Cargo.toml",
    "systemctl --user",
]
for token in grounding:
    if token.lower() not in body.lower():
        fail(f"runbook not grounded in daemon: missing '{token}'")

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

spec_triggers = [t.lower() for t in spec.get("triggers", [])]
for term in required_triggers:
    if term not in spec_triggers:
        fail(f"required trigger term '{term}' missing from spec triggers")

print(f"{skill_name} validation passed")
PY
