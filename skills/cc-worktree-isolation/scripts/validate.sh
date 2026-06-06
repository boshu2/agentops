#!/usr/bin/env bash
# validate.sh — self-check for the cc-worktree-isolation skill.
# EXECUTE intent (not read-as-reference): `bash scripts/validate.sh`.
# Checks frontmatter completeness, the required section spine, and the Form-A line budget.
# Exit 0 = PASS; exit 1 = FAIL (prints every failing check).
set -euo pipefail

# Resolve skill dir from this script's location (portable; no hardcoded home).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SKILL_MD="${SKILL_DIR}/SKILL.md"
SPEC_JSON="${SKILL_DIR}/skill.spec.json"

fail=0
err() { echo "FAIL: $*" >&2; fail=1; }

# --- 0. File exists ---
[ -f "${SKILL_MD}" ] || { echo "FAIL: SKILL.md not found at ${SKILL_MD}" >&2; exit 1; }

# --- 1. Form-A line budget (hard ≤250) ---
lines=$(wc -l < "${SKILL_MD}" | tr -d ' ')
if [ "${lines}" -gt 250 ]; then
  err "Form-A line budget exceeded: ${lines} > 250 (this is a procedure skill, not a Form-B reference contract)"
fi

# --- 2. Frontmatter: required keys present in the leading --- block ---
fm="$(awk 'NR==1 && $0=="---"{f=1;next} f&&$0=="---"{exit} f{print}' "${SKILL_MD}")"
[ -n "${fm}" ] || err "no YAML frontmatter block found"
for key in "name:" "description:" "skill_api_version:"; do
  echo "${fm}" | grep -q "^${key}" || err "frontmatter missing required key: ${key}"
done
# name must equal the directory name (lowercase-hyphen contract)
dirname_actual="$(basename "${SKILL_DIR}")"
name_val="$(echo "${fm}" | awk -F': *' '/^name:/{print $2; exit}' | tr -d '"'"'"' ')"
[ "${name_val}" = "${dirname_actual}" ] || err "frontmatter name '${name_val}' != directory '${dirname_actual}'"

# --- 3. Triggers clause is FAIL-severity (must be a real line-start marker) ---
echo "${fm}" | grep -qE '^[[:space:]]*Triggers:' || err "description missing a 'Triggers:' clause (FAIL-severity per §0/§8.2)"

# --- 4. Required section spine present + ordered ---
# Ordered list of headings that must appear in this order.
spine=(
  "## ⚠️ Critical Constraints"
  "## Workflow / Methodology"
  "## Output Specification"
  "## Quality Rubric"
  "## Examples"
  "## Troubleshooting"
  "## See Also / References"
)
last_line=0
for heading in "${spine[@]}"; do
  ln=$(grep -nF "${heading}" "${SKILL_MD}" | head -1 | cut -d: -f1 || true)
  if [ -z "${ln}" ]; then
    err "missing required section: '${heading}'"
  elif [ "${ln}" -le "${last_line}" ]; then
    err "section out of order: '${heading}' appears before a prior spine section"
  else
    last_line="${ln}"
  fi
done

# --- 5. Critical Constraints within the first 80 lines (§5) ---
cc_line=$(grep -nF "## ⚠️ Critical Constraints" "${SKILL_MD}" | head -1 | cut -d: -f1 || true)
if [ -n "${cc_line}" ] && [ "${cc_line}" -gt 80 ]; then
  err "'## ⚠️ Critical Constraints' at line ${cc_line} — must be within the first 80 lines"
fi

# --- 6. spec.json sidecar present + valid JSON (§7) ---
if [ -f "${SPEC_JSON}" ]; then
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import json,sys; json.load(open('${SPEC_JSON}'))" 2>/dev/null || err "skill.spec.json is not valid JSON"
  fi
else
  err "skill.spec.json sidecar missing (§7)"
fi

if [ "${fail}" -ne 0 ]; then
  echo "validate.sh: FAIL" >&2
  exit 1
fi
echo "validate.sh: PASS (${lines} lines, spine ordered, frontmatter + triggers + spec.json OK)"
exit 0
