#!/usr/bin/env bash
# validate.sh — self-check for the cc-subagents skill (Form A).
# Checks: frontmatter completeness, line-start Triggers: marker, section spine,
# Form-A line budget (<=250), and that every references/ link resolves.
# Exit 0 on PASS, 1 on FAIL. Execute: bash scripts/validate.sh
set -euo pipefail

# Resolve skill dir (parent of this scripts/ dir) regardless of cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"

fail=0
pass() { printf 'PASS  %s\n' "$1"; }
err()  { printf 'FAIL  %s\n' "$1"; fail=1; }

# --- 0. SKILL.md exists ---
[ -f "$SKILL" ] || { err "SKILL.md missing at $SKILL"; exit 1; }

# --- 1. Frontmatter: opens with --- and has required keys ---
head -1 "$SKILL" | grep -qx -- '---' && pass "frontmatter opens with ---" \
  || err "SKILL.md must start with '---'"

for key in 'name:' 'description:' 'skill_api_version:'; do
  grep -qE "^${key}" "$SKILL" && pass "frontmatter has $key" \
    || err "frontmatter missing required key: $key"
done

# name must equal the directory name
dir_name="$(basename "$SKILL_DIR")"
fm_name="$(grep -E '^name:' "$SKILL" | head -1 | sed 's/^name:[[:space:]]*//')"
[ "$fm_name" = "$dir_name" ] && pass "name matches dir ($dir_name)" \
  || err "name '$fm_name' != dir '$dir_name'"

# --- 2. Triggers: FAIL-severity, must be a real line-start marker ---
if grep -qE '^[[:space:]]*Triggers:' "$SKILL"; then
  pass "Triggers: marker present"
else
  err "Triggers: clause missing (FAIL-severity per authoring standard)"
fi

# --- 3. Section spine (required headings) ---
for sec in \
  '## ⚠️ Critical Constraints' \
  '## Output Specification' \
  '## Quality Rubric' \
  '## Examples' \
  '## Troubleshooting' \
  '## See Also'; do
  grep -qF "$sec" "$SKILL" && pass "spine: $sec" \
    || err "spine: missing '$sec'"
done

# --- 4. Form-A line budget (<=250) ---
lines="$(wc -l < "$SKILL" | tr -d ' ')"
if [ "$lines" -le 250 ]; then
  pass "line budget $lines/250 (Form A)"
else
  err "line budget exceeded: $lines > 250 (Form A hard cap)"
fi

# --- 5. references/ links resolve (one level deep) ---
if [ -d "$SKILL_DIR/references" ]; then
  while IFS= read -r ref; do
    [ -z "$ref" ] && continue
    if [ -f "$SKILL_DIR/references/$ref" ]; then
      pass "reference resolves: references/$ref"
    else
      err "dead reference: references/$ref"
    fi
  done < <(grep -oE 'references/[A-Za-z0-9._-]+\.md' "$SKILL" | sed 's#references/##' | sort -u)
fi

echo "----"
if [ "$fail" -eq 0 ]; then
  echo "RESULT: PASS"
  exit 0
else
  echo "RESULT: FAIL"
  exit 1
fi
