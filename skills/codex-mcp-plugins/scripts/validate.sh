#!/usr/bin/env bash
# validate.sh — self-check for the codex-mcp-plugins skill.
# Checks: frontmatter completeness, the line-start Triggers: marker, the required
# section spine, the Form-A line budget (<=250), and that referenced sibling/global
# skills actually resolve. Exit 0 on pass, 1 on any failure.
#
# Execute: bash scripts/validate.sh   (run from the skill dir or anywhere)
set -euo pipefail

# Resolve the skill root from this script's location (portable; no hardcoded home).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"
SPEC_JSON="$SKILL_DIR/skill.spec.json"

fail=0
err() { printf 'FAIL: %s\n' "$1" >&2; fail=1; }
ok()  { printf 'ok:   %s\n' "$1"; }

[ -f "$SKILL_MD" ] || { err "SKILL.md missing at $SKILL_MD"; exit 1; }

# --- 1. Frontmatter present and complete ----------------------------------------
head -n1 "$SKILL_MD" | grep -qx -- '---' || err "frontmatter must open with '---' on line 1"
for key in 'name:' 'description:' 'skill_api_version:' 'metadata:' 'tier:'; do
  grep -q "^[[:space:]]*$key" "$SKILL_MD" || err "frontmatter missing key: $key"
done
grep -q '^name: codex-mcp-plugins$' "$SKILL_MD" || err "name must equal dir (codex-mcp-plugins)"

# --- 2. Triggers: line-start marker (FAIL-severity per the standard) -------------
grep -qE '^[[:space:]]*Triggers:' "$SKILL_MD" || err "description needs a line starting with 'Triggers:'"

# --- 3. Required section spine, in order ----------------------------------------
spine=(
  '^# codex-mcp-plugins'
  '^## ⚠️ Critical Constraints'
  '^## Workflow / Methodology'
  '^## Output Specification'
  '^## Quality Rubric'
  '^## Examples'
  '^## Troubleshooting'
  '^## See Also'
)
last=0
for pat in "${spine[@]}"; do
  ln="$(grep -nE "$pat" "$SKILL_MD" | head -n1 | cut -d: -f1 || true)"
  if [ -z "${ln:-}" ]; then
    err "missing section: ${pat#^}"
  elif [ "$ln" -le "$last" ]; then
    err "section out of order: ${pat#^} (line $ln <= prev $last)"
  else
    last="$ln"
  fi
done

# --- 4. Form-A line budget (<=250) ----------------------------------------------
lines="$(wc -l < "$SKILL_MD" | tr -d ' ')"
if [ "$lines" -le 250 ]; then ok "line budget $lines/250"; else err "line budget exceeded: $lines > 250 (Form A)"; fi

# --- 5. Referenced skills resolve (See Also currency) ---------------------------
for s in agent-mail beads-br caam; do
  [ -d "$SKILL_DIR/../$s" ] || err "referenced sibling skill not found: $s"
done
[ -d "$SKILL_DIR/../codex-exec" ] || err "referenced sibling skill not found: codex-exec"

# --- 6. Sidecar spec present and parseable --------------------------------------
if [ -f "$SPEC_JSON" ]; then
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import json,sys; json.load(open('$SPEC_JSON'))" 2>/dev/null \
      && ok "skill.spec.json valid JSON" || err "skill.spec.json is not valid JSON"
  else
    ok "skill.spec.json present (no python3 to parse)"
  fi
else
  err "skill.spec.json sidecar missing"
fi

if [ "$fail" -eq 0 ]; then
  printf '\nPASS: codex-mcp-plugins skill meets the authoring standard.\n'
  exit 0
else
  printf '\nFAILED: fix the above before staging.\n' >&2
  exit 1
fi
