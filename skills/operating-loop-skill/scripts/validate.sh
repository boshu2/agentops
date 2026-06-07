#!/usr/bin/env bash
# validate.sh — self-check for the operating-loop-skill against the Skill Authoring Standard.
# Checks: frontmatter completeness, the required section spine (in order), Form-A line budget,
# a real Triggers: clause, and that referenced sidecar artifacts exist.
# Exit 0 = PASS; non-zero = FAIL (count of failures). EXECUTE this; do not read it as prose.
set -u

here="$(cd "$(dirname "$0")/.." && pwd)"
skill="$here/SKILL.md"
spec="$here/skill.spec.json"
fails=0
pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1" >&2; fails=$((fails+1)); }

# 0. file present
[ -f "$skill" ] || { fail "SKILL.md missing at $skill"; exit 1; }

# 1. frontmatter: opens with --- and has required keys
head -1 "$skill" | grep -qx -- "---" && pass "frontmatter opens with ---" \
  || fail "first line is not ---"
for key in '^name:' '^description:' '^skill_api_version:'; do
  grep -qE "$key" "$skill" && pass "frontmatter has ${key#^}" \
    || fail "frontmatter missing ${key#^}"
done
# name must equal the directory name (lowercase-hyphen contract)
dirname_actual="$(basename "$here")"
name_val="$(grep -E '^name:' "$skill" | head -1 | sed 's/^name:[[:space:]]*//')"
[ "$name_val" = "$dirname_actual" ] && pass "name matches dir ($name_val)" \
  || fail "name '$name_val' != dir '$dirname_actual'"

# 2. Triggers: clause present as a real line-start marker (FAIL severity per §0/§8.2)
grep -qE '^[[:space:]]*Triggers:[[:space:]]*$' "$skill" && pass "Triggers: marker present" \
  || fail "no line-start 'Triggers:' marker in description"

# 3. required section spine, IN ORDER
spine=(
  "## Why This Exists"
  "## Overview / When to Use"
  "## ⚠️ Critical Constraints"
  "## Workflow / Methodology"
  "## Output Specification"
  "## Quality Rubric"
  "## Examples"
  "## Troubleshooting"
  "## See Also / References"
)
last_line=0
for sec in "${spine[@]}"; do
  ln="$(grep -nF -- "$sec" "$skill" | head -1 | cut -d: -f1)"
  if [ -z "$ln" ]; then
    fail "missing section: $sec"
  elif [ "$ln" -lt "$last_line" ]; then
    fail "section out of order: $sec (line $ln after $last_line)"
  else
    pass "section present + ordered: $sec"
    last_line="$ln"
  fi
done

# 4. Form-A line budget (<=250 hard)
lines="$(wc -l < "$skill" | tr -d ' ')"
[ "$lines" -le 250 ] && pass "line budget ok ($lines <= 250)" \
  || fail "line budget exceeded ($lines > 250) — Form A is capped at 250"

# 5. spec sidecar present + parseable (recommended by §7; required here)
if [ -f "$spec" ]; then
  if command -v jq >/dev/null 2>&1; then
    jq -e '.metadata.triggers and .sections and .references' "$spec" >/dev/null 2>&1 \
      && pass "skill.spec.json parses + has sections/references/metadata.triggers" \
      || fail "skill.spec.json missing sections/references/metadata.triggers"
  else
    pass "skill.spec.json present (jq absent; skipped deep check)"
  fi
else
  fail "skill.spec.json sidecar missing"
fi

# 6. no >one-level reference chains advertised (references/ should be flat if present)
if [ -d "$here/references" ]; then
  if grep -rlE '\]\(\.\./|\]\([^)]*references/[^)]*/' "$here/references" >/dev/null 2>&1; then
    fail "a references/ file links more than one level deep"
  else
    pass "references one level deep"
  fi
fi

echo "----"
if [ "$fails" -eq 0 ]; then
  echo "RESULT: PASS"
  exit 0
fi
echo "RESULT: FAIL ($fails)"
exit "$fails"
