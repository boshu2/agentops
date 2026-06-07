#!/usr/bin/env bash
# validate.sh — self-check for the acfs skill (AUTHORING-STANDARD §6/§8).
#   Checks: frontmatter completeness · section spine + order · Triggers marker ·
#           Form-A line budget (≤250) · sidecar present. Exit 0 on PASS, 1 on FAIL.
#   Structure-only (does not exercise the acfs binary); run from anywhere.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$DIR/SKILL.md"
SPEC="$DIR/skill.spec.json"
fail=0
ok(){ printf '  \033[0;32mPASS\033[0m %s\n' "$1"; }
no(){ printf '  \033[0;31mFAIL\033[0m %s\n' "$1"; fail=$((fail+1)); }

[ -f "$SKILL" ] || { echo "FAIL: SKILL.md not found at $SKILL"; exit 1; }

echo "── frontmatter ──"
# block-scalar frontmatter delimited by --- / ---
head -1 "$SKILL" | grep -qx -- '---' && ok "opens with frontmatter" || no "missing leading ---"
for key in 'name:' 'description:' 'skill_api_version:'; do
  grep -qE "^${key}" "$SKILL" && ok "has $key" || no "missing required key: $key"
done
# name must equal the dir name
nm="$(grep -E '^name:' "$SKILL" | head -1 | awk '{print $2}')"
[ "$nm" = "acfs" ] && ok "name matches dir (acfs)" || no "name '$nm' != dir 'acfs'"

echo "── triggers (FAIL-severity) ──"
grep -qE '^[[:space:]]*Triggers:' "$SKILL" && ok "Triggers: marker at line-start" \
  || no "no line-start Triggers: clause in description"

echo "── section spine (required order) ──"
spine=(
 "## ⚠️ Critical Constraints"
 "## Workflow"
 "## Output Specification"
 "## Quality Rubric"
 "## Examples"
 "## Troubleshooting"
 "## See Also"
)
last=0
for s in "${spine[@]}"; do
  ln="$(grep -nF "$s" "$SKILL" | head -1 | cut -d: -f1)"
  if [ -z "$ln" ]; then no "missing section: $s"; continue; fi
  if [ "$ln" -gt "$last" ]; then ok "section ordered: $s (L$ln)"; last="$ln";
  else no "section OUT OF ORDER: $s (L$ln after L$last)"; fi
done

echo "── line budget (Form A ≤250) ──"
lc="$(wc -l < "$SKILL" | tr -d ' ')"
[ "$lc" -le 250 ] && ok "SKILL.md $lc lines (≤250)" || no "SKILL.md $lc lines (>250 — Form A breach)"

echo "── references resolve (one level deep) ──"
miss=0
while IFS= read -r ref; do
  [ -f "$DIR/$ref" ] || { no "dead reference link: $ref"; miss=$((miss+1)); }
done < <(grep -oE '\(references/[a-z0-9-]+\.md' "$SKILL" | sed 's/^(//')
[ "$miss" -eq 0 ] && ok "no dead references/ links"

echo "── sidecar ──"
if [ -f "$SPEC" ]; then
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import json,sys; json.load(open('$SPEC'))" 2>/dev/null \
      && ok "skill.spec.json present + valid JSON" || no "skill.spec.json invalid JSON"
  else ok "skill.spec.json present (no python3 to lint JSON)"; fi
else no "skill.spec.json missing"; fi

echo
if [ "$fail" -eq 0 ]; then echo "VALIDATE: PASS (0 failures)"; exit 0
else echo "VALIDATE: FAIL ($fail failure(s))"; exit 1; fi
