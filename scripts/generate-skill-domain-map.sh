#!/usr/bin/env bash
# scripts/generate-skill-domain-map.sh
#
# Generate the data sections of docs/reference/agentops-skill-domain-map.md
# from canonical sources:
#   - docs/contracts/bounded-contexts.yaml   (BC1-BC5 definitions)
#   - docs/contracts/skill-dispositions.yaml (per-skill judgment data)
#   - skills/<name>/SKILL.md                 (hexagonal_role cross-check)
#
# The script replaces content between these markers in the .md file:
#   <!-- BEGIN:audit-summary -->     ... <!-- END:audit-summary -->
#   <!-- BEGIN:domain-taxonomy -->   ... <!-- END:domain-taxonomy -->
#   <!-- BEGIN:full-skill-map -->    ... <!-- END:full-skill-map -->
#
# Hand-written prose outside these markers is preserved.
#
# Modes:
#   (default)    write the .md file in place
#   --check      generate to a temp file; diff against committed; exit 1 if different
#                (this is the CI golden-file gate)
#   --stdout     write to stdout instead of the file
#
# Determinism: dispositions sorted alphabetically by skill slug. No timestamps.
#
# Phase 3 of soc-zxia (registries-drift remediation).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BC_YAML="${REPO_ROOT}/docs/contracts/bounded-contexts.yaml"
DISP_YAML="${REPO_ROOT}/docs/contracts/skill-dispositions.yaml"
SKILLS_DIR="${REPO_ROOT}/skills"
MAP_DOC="${REPO_ROOT}/docs/reference/agentops-skill-domain-map.md"

MODE="write"
for arg in "$@"; do
  case "$arg" in
    --check)  MODE="check" ;;
    --stdout) MODE="stdout" ;;
    -h|--help)
      sed -n '2,28p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "ERROR: unknown arg: $arg (try --help)" >&2
      exit 2
      ;;
  esac
done

for f in "${BC_YAML}" "${DISP_YAML}" "${MAP_DOC}"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: required file missing: $f" >&2
    exit 2
  fi
done

export BC_YAML DISP_YAML SKILLS_DIR MAP_DOC MODE

python3 - <<'PY'
import os
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed; install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

BC_YAML    = Path(os.environ["BC_YAML"])
DISP_YAML  = Path(os.environ["DISP_YAML"])
SKILLS_DIR = Path(os.environ["SKILLS_DIR"])
MAP_DOC    = Path(os.environ["MAP_DOC"])
MODE       = os.environ.get("MODE", "write")

bcs   = yaml.safe_load(BC_YAML.read_text()).get("bounded_contexts", [])
disps = yaml.safe_load(DISP_YAML.read_text()).get("dispositions", [])
disps = sorted(disps, key=lambda d: d["skill"])

# Cross-check: every disposition row's hexagonal_role must match the
# skill's frontmatter, every skill must have a disposition row.
fm_role = {}
for d in sorted(SKILLS_DIR.iterdir()):
    if not d.is_dir():
        continue
    skill_md = d / "SKILL.md"
    if not skill_md.is_file():
        continue
    text = skill_md.read_text()
    m = re.match(r'---\n(.*?)\n---', text, re.DOTALL)
    if not m:
        continue
    try:
        fm = yaml.safe_load(m.group(1)) or {}
    except yaml.YAMLError:
        continue
    fm_role[d.name] = fm.get("hexagonal_role")

actual_skills = set(fm_role.keys())
disp_skills   = {d["skill"] for d in disps}

missing_disps = sorted(actual_skills - disp_skills)
extra_disps   = sorted(disp_skills - actual_skills)
role_drifts   = []
for d in disps:
    sk = d["skill"]
    expected = fm_role.get(sk)
    if expected is not None and expected != d["hexagonal_role"]:
        role_drifts.append((sk, d["hexagonal_role"], expected))

errs = []
if missing_disps:
    errs.append(f"skill(s) exist with no disposition row: {missing_disps}")
if extra_disps:
    errs.append(f"disposition row(s) reference nonexistent skill: {extra_disps}")
if role_drifts:
    errs.append(f"hexagonal_role drift (yaml vs SKILL.md): {role_drifts}")
if errs:
    for e in errs:
        print("ERROR:", e, file=sys.stderr)
    print("Fix docs/contracts/skill-dispositions.yaml before regenerating.", file=sys.stderr)
    sys.exit(2)


# --- Render the three replaceable sections ---

audit_summary = f"""| Signal | Result |
|---|---:|
| Skills audited | {len(actual_skills)} |
| Domains classified | {len(set(d['domain'] for d in disps))} of 5 (BC1-BC5) |
| Dispositions assigned | {len(disp_skills)} / {len(actual_skills)} |"""


taxonomy_lines = ["| Domain | Product layer | Responsibility |", "|---|---|---|"]
for bc in bcs:
    taxonomy_lines.append(
        f"| {bc['id']} {bc['name']} | {bc['product_layer']} | {bc['responsibility']} |"
    )
domain_taxonomy = "\n".join(taxonomy_lines)


full_map_lines = [
    "| Skill | Domain | Hex role | First disposition | Rationale |",
    "|---|---|---|---|---|",
]
for d in disps:
    full_map_lines.append(
        f"| `{d['skill']}` | {d['domain']} | {d['hexagonal_role']} | {d['disposition']} | {d['rationale']}. |"
    )
full_skill_map = "\n".join(full_map_lines)


# --- Replace marker blocks in the .md ---

text = MAP_DOC.read_text()

def replace_block(name, content):
    global text
    pattern = re.compile(
        rf'(<!-- BEGIN:{name} -->\n).*?(\n<!-- END:{name} -->)',
        re.DOTALL,
    )
    if not pattern.search(text):
        # Block markers do not exist yet — insert near a known section.
        # For first-run bootstrap, we expect the doc to already contain
        # markers (added in this PR). Hard-fail otherwise.
        print(f"ERROR: marker block <!-- BEGIN:{name} --> not found in {MAP_DOC.name};"
              " the doc must contain marker pairs for every generated section.",
              file=sys.stderr)
        sys.exit(2)
    text = pattern.sub(rf'\1{content}\2', text)

replace_block("audit-summary",    audit_summary)
replace_block("domain-taxonomy",  domain_taxonomy)
replace_block("full-skill-map",   full_skill_map)


# --- Output ---

if MODE == "stdout":
    sys.stdout.write(text)
elif MODE == "check":
    current = MAP_DOC.read_text()
    if text == current:
        print("PASS — skill-domain-map.md matches generator output.")
        sys.exit(0)
    print("FAIL — skill-domain-map.md DIVERGES from generator output.", file=sys.stderr)
    print("Run: bash scripts/generate-skill-domain-map.sh", file=sys.stderr)
    print(file=sys.stderr)
    import difflib
    diff = difflib.unified_diff(
        current.splitlines(keepends=True),
        text.splitlines(keepends=True),
        fromfile="committed",
        tofile="generated",
        n=2,
    )
    sys.stderr.writelines(diff)
    sys.exit(1)
else:  # write
    MAP_DOC.write_text(text)
    print(f"Generated {MAP_DOC.relative_to(SCRIPT_DIR.parent.parent.parent.parent) if False else MAP_DOC.name}")
    print(f"  {len(actual_skills)} skills, {len(bcs)} BCs, {len(disps)} dispositions")
PY
