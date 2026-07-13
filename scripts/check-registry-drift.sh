#!/usr/bin/env bash
# scripts/check-registry-drift.sh
#
# Detect drift between skills/ (source of truth) and the generated domain map.
#
# Checks:
#   1. Skill count in the domain-map audit table matches skills/.
#   2. Every skill listed in the Full Skill Map table exists in skills/.
#   3. Every skill in skills/ is listed in the Full Skill Map table.
#   4. hexagonal_role column in the doc matches each SKILL.md frontmatter
#      `hexagonal_role:` field.
#
# Exit codes:
#   0 = no drift
#   1 = drift detected
#   2 = usage error / missing inputs
#
# Modes:
#   --check       (default) report drift to stdout, non-zero on any
#   --json        emit machine-readable JSON report instead of human prose
#
# Schema reference: schemas/skill-frontmatter.v2.schema.json
# Lesson:           .agents/learnings/2026-05-17-registries-drift.md
# Contract:         docs/reference/agentops-skill-domain-map.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SKILLS_DIR="${REPO_ROOT}/skills"
MAP_DOC="${REPO_ROOT}/docs/reference/agentops-skill-domain-map.md"

MODE="check"
JSON_OUT=0
for arg in "$@"; do
  case "$arg" in
    --check)      MODE="check" ;;
    --json)       JSON_OUT=1 ;;
    -h|--help)
      sed -n '2,32p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "ERROR: unknown arg: $arg (try --help)" >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "${SKILLS_DIR}" ]]; then
  echo "ERROR: skills/ not found at ${SKILLS_DIR}" >&2
  exit 2
fi
if [[ ! -f "${MAP_DOC}" ]]; then
  echo "ERROR: registry doc missing: ${MAP_DOC}" >&2
  exit 2
fi

export SKILLS_DIR MAP_DOC MODE JSON_OUT

exec python3 - <<'PY'
import json
import os
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed; install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

SKILLS_DIR = Path(os.environ["SKILLS_DIR"])
MAP_DOC    = Path(os.environ["MAP_DOC"])
MODE       = os.environ.get("MODE", "check")
JSON_OUT   = os.environ.get("JSON_OUT") == "1"

findings = []  # list of dicts: {severity, code, msg, suggest?}


def add(severity, code, msg, suggest=None):
    f = {"severity": severity, "code": code, "msg": msg}
    if suggest:
        f["suggest"] = suggest
    findings.append(f)


# ---- Source of truth: actual skills/ contents ----
actual_skills = sorted(
    p.name for p in SKILLS_DIR.iterdir()
    if p.is_dir()
    and p.name not in {"pre-mortem", "post-mortem", "pre_mortem", "post_mortem"}
    and (p / "SKILL.md").is_file()
)
actual_count = len(actual_skills)

fm_role = {}
for name in actual_skills:
    skill_md = SKILLS_DIR / name / "SKILL.md"
    text = skill_md.read_text()
    m = re.match(r'---\n(.*?)\n---', text, re.DOTALL)
    if not m:
        add("warn", "SKILL_NO_FRONTMATTER",
            f"skills/{name}/SKILL.md has no YAML frontmatter; skipping hex-role check")
        continue
    try:
        fm = yaml.safe_load(m.group(1)) or {}
    except yaml.YAMLError as e:
        add("warn", "SKILL_FRONTMATTER_BAD",
            f"skills/{name}/SKILL.md frontmatter parse failed: {e}")
        continue
    fm_role[name] = fm.get("hexagonal_role")


# ---- Parse the generated audit count ----
map_text = MAP_DOC.read_text()

# Map audit table: "| Skills audited | 78 |"
map_audit_match = re.search(r'\|\s*Skills audited\s*\|\s*(\d+)\s*\|', map_text)
map_audit_count = int(map_audit_match.group(1)) if map_audit_match else None

def check_count(label, declared, doc_path):
    """Compare the generated declared count to actual."""
    if declared is None:
        add("warn", "COUNT_NOT_FOUND",
            f"could not find declared count in {doc_path.name} ({label}); pattern may have drifted")
        return False
    if declared == actual_count:
        return True
    add("fail", "COUNT_DRIFT",
        f"{doc_path.name} ({label}): declared {declared} skills, actual {actual_count}",
        f"sed -i 's/{declared}\\([^0-9]\\)/{actual_count}\\1/g' {doc_path}  # manual review required")
    return False


check_count("audit table", map_audit_count, MAP_DOC)


# ---- Parse Full Skill Map rows ----
# Match: | `skill-name` | BC?  ... | hex-role | disposition | rationale |
row_re = re.compile(r'^\|\s*`([a-z][a-z0-9-]*)`\s*\|[^|]+\|\s*([a-z-]+)\s*\|', re.MULTILINE)
map_rows = {m.group(1): m.group(2) for m in row_re.finditer(map_text)}
map_skills = set(map_rows.keys())

missing_from_doc = sorted(set(actual_skills) - map_skills)
extra_in_doc     = sorted(map_skills - set(actual_skills))

for sk in missing_from_doc:
    add("fail", "MAP_MISSING_SKILL",
        f"skills/{sk}/ exists but is not in {MAP_DOC.name} Full Skill Map",
        f"Add a row to {MAP_DOC.name} classifying `{sk}`")
for sk in extra_in_doc:
    add("fail", "MAP_EXTRA_SKILL",
        f"{MAP_DOC.name} lists `{sk}` but skills/{sk}/ does not exist",
        f"Remove `{sk}` row from {MAP_DOC.name} or restore skills/{sk}/")


# ---- hexagonal_role per-skill consistency ----
for sk in sorted(set(actual_skills) & map_skills):
    fr = fm_role.get(sk)
    dr = map_rows.get(sk)
    if fr is None:
        add("warn", "FRONTMATTER_NO_HEX_ROLE",
            f"skills/{sk}/SKILL.md has no `hexagonal_role` field; map declares `{dr}`",
            f"Either add `hexagonal_role: {dr}` to frontmatter or remove from map if the skill is being deprecated")
    elif fr != dr:
        add("fail", "HEX_ROLE_DRIFT",
            f"`{sk}`: frontmatter hexagonal_role={fr}, map column={dr}",
            f"Pick one as truth; update the other")


# ---- Emit report ----
fails  = [f for f in findings if f["severity"] == "fail"]
warns  = [f for f in findings if f["severity"] == "warn"]
infos  = [f for f in findings if f["severity"] == "info"]

if JSON_OUT:
    print(json.dumps({
        "actual_count": actual_count,
        "declared_map_audit":     map_audit_count,
        "missing_from_doc":       missing_from_doc,
        "extra_in_doc":           extra_in_doc,
        "findings":               findings,
        "verdict":                "FAIL" if fails else ("WARN" if warns else "PASS"),
    }, indent=2))
else:
    print(f"Registry drift check: {actual_count} skills in skills/")
    print(f"  declared in map audit:     {map_audit_count}")
    print()
    for f in findings:
        tag = {"fail": "FAIL", "warn": "WARN", "info": "INFO"}[f["severity"]]
        print(f"[{tag}] {f['code']}: {f['msg']}")
        if "suggest" in f:
            print(f"       suggest: {f['suggest']}")
    print()
    if not findings:
        print("PASS — no drift.")
    elif fails:
        print(f"FAIL — {len(fails)} drift finding(s) (warns: {len(warns)}, infos: {len(infos)})")
    else:
        print(f"WARN — {len(warns)} warning(s) (infos: {len(infos)})")

sys.exit(1 if fails else 0)
PY
