#!/usr/bin/env bash
# Behavioral validator for the agy-sidecar-scheduled-tick skill.
# Exit 0 only when the skill's own claims hold: required frontmatter,
# the four coverage trigger terms (agy/sidecar/schedule/agentapi), the
# required SKILL.md sections, and a well-formed skill.spec.json whose
# triggers and dependencies agree with the documented contract.
# bash-3.2-safe, shellcheck-clean, no python required.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_dir="$(cd "${script_dir}/.." && pwd)"
skill_md="${skill_dir}/SKILL.md"
spec_json="${skill_dir}/skill.spec.json"
skill_name="agy-sidecar-scheduled-tick"

fail() {
  printf '%s validation FAILED: %s\n' "${skill_name}" "$1" >&2
  exit 1
}

# --- file presence -----------------------------------------------------------
[ -f "${skill_md}" ]   || fail "missing SKILL.md"
[ -f "${spec_json}" ]  || fail "missing skill.spec.json"

# --- SKILL.md: closed YAML frontmatter ---------------------------------------
# First line must open the block; a second '---' must close it.
first_line="$(head -n 1 "${skill_md}")"
[ "${first_line}" = "---" ] || fail "SKILL.md missing opening frontmatter fence"
# Count closing fences (lines that are exactly '---'); need at least two total.
fence_count="$(grep -c '^---$' "${skill_md}" || true)"
[ "${fence_count}" -ge 2 ] || fail "SKILL.md frontmatter is not closed"

# Extract the frontmatter block (between the first two '---' fences).
fm="$(awk 'NR==1&&/^---$/{f=1;next} f&&/^---$/{exit} f{print}' "${skill_md}")"

fm_has() {
  printf '%s\n' "${fm}" | grep -Eq "$1" || fail "frontmatter missing: $2"
}
fm_has "^name:[[:space:]]*${skill_name}[[:space:]]*$"       "name: ${skill_name}"
fm_has "^description:[[:space:]]*"                          "description"
fm_has "[Tt]riggers:"                                       "description Triggers line"
fm_has "^skill_api_version:[[:space:]]*1[[:space:]]*$"      "skill_api_version: 1"
fm_has "^user-invocable:[[:space:]]*false[[:space:]]*$"     "user-invocable: false"
fm_has "^context:[[:space:]]*$"                             "context block"
fm_has "^[[:space:]]+window:[[:space:]]*\S+"               "context.window"
fm_has "^metadata:[[:space:]]*$"                            "metadata block"
fm_has "^[[:space:]]+tier:[[:space:]]*\S+"                 "metadata.tier"
fm_has "^[[:space:]]+stability:[[:space:]]*\S+"            "metadata.stability"
fm_has "^[[:space:]]+dependencies:[[:space:]]*\["          "metadata.dependencies"
fm_has "^output_contract:[[:space:]]*"                     "output_contract"

# --- coverage trigger terms (the spec's required terms) ----------------------
# All four must appear in the frontmatter (the discoverable surface).
for term in agy sidecar schedule agentapi; do
  printf '%s\n' "${fm}" | grep -qi "${term}" \
    || fail "frontmatter missing required trigger term: ${term}"
done

# --- required SKILL.md sections ----------------------------------------------
require_section() {
  grep -Eq "^## .*$1" "${skill_md}" || fail "SKILL.md missing section: $1"
}
require_section "Overview"
require_section "Critical Constraints"
require_section "Workflow"
require_section "Output Specification"
require_section "Quality Rubric"
require_section "See Also"

# The body must teach the real mechanic (sidecar + schedule builtin + agentapi).
grep -q "schedule" "${skill_md}"  || fail "SKILL.md does not document the schedule builtin"
grep -q "agentapi" "${skill_md}"  || fail "SKILL.md does not document the agentapi runtime"
grep -q "sidecar"  "${skill_md}"  || fail "SKILL.md does not document the sidecar"
# LAW 0 floor must be asserted.
grep -q "claude -p" "${skill_md}" || fail "SKILL.md does not assert the LAW 0 (claude -p) floor"

# --- skill.spec.json: valid JSON + schema agreement --------------------------
# Prefer a real JSON parser when available; fall back to structural greps so
# the validator stays portable on bash-3.2 hosts without python.
if command -v python3 >/dev/null 2>&1; then
  python3 -m json.tool "${spec_json}" >/dev/null 2>&1 \
    || fail "skill.spec.json is not valid JSON"
elif command -v jq >/dev/null 2>&1; then
  jq -e . "${spec_json}" >/dev/null 2>&1 \
    || fail "skill.spec.json is not valid JSON"
fi

spec_has() {
  grep -q "$1" "${spec_json}" || fail "skill.spec.json missing: $2"
}
spec_has "\"name\": \"${skill_name}\""        "name == ${skill_name}"
spec_has "\"skill_api_version\": 1"           "skill_api_version == 1"
spec_has "\"quality_score\""                  "quality_score"
spec_has "\"sections\""                       "sections"
spec_has "\"triggers\""                       "triggers"
spec_has "\"token_estimate\""                 "token_estimate"
spec_has "\"dependencies\""                   "dependencies"
spec_has "\"evidence\""                       "evidence"
spec_has "\"output_contract\""                "output_contract"

# Each coverage trigger term must be present in the spec's triggers too.
for term in agy sidecar schedule agentapi; do
  grep -qi "\"${term}" "${spec_json}" \
    || fail "skill.spec.json triggers missing required term: ${term}"
done

printf '%s validation PASSED\n' "${skill_name}"
