#!/usr/bin/env bash
# validate.sh — self-check for the codex-goals skill.
# Verifies: required frontmatter keys, name==dir, the Triggers: marker, the section
# spine, the Form-A line budget (<=250), and the currency of the codex binary +
# the `goals` feature this skill drives.
# Exit 0 = sound. Exit 1 = structural defect. Exit 2 reserved (we WARN, not fail,
# on missing tooling so the structural check stays portable).

set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
FAIL=0

echo "=== codex-goals skill validation ==="
echo "skill: $SKILL"
echo

if [ ! -f "$SKILL" ]; then
  echo "ERROR: SKILL.md not found at $SKILL"
  exit 1
fi

# --- 1. Frontmatter present and well-formed ----------------------------------
if [ "$(head -1 "$SKILL")" != "---" ]; then
  echo "FAIL: file does not start with a '---' frontmatter fence"
  FAIL=1
fi

# Extract the frontmatter block (between the first two '---' fences).
FM="$(awk 'NR>1 && /^---[[:space:]]*$/{exit} NR>1{print}' "$SKILL")"

for key in "name:" "description:" "skill_api_version:"; do
  if ! grep -q "^${key}" <<<"$FM"; then
    echo "FAIL: missing required frontmatter key '${key}'"
    FAIL=1
  fi
done

# name must equal the directory name.
DIRNAME="$(basename "$SKILL_DIR")"
NAME="$(grep '^name:' <<<"$FM" | head -1 | sed 's/^name:[[:space:]]*//')"
if [ "$NAME" != "$DIRNAME" ]; then
  echo "FAIL: frontmatter name '$NAME' != directory '$DIRNAME'"
  FAIL=1
fi

# --- 2. Triggers: marker (FAIL-severity per the authoring standard) ----------
# Must be a line-start (whitespace-only indent allowed) 'Triggers:' clause, NOT a
# bold '**Triggers:**' form — the selection linter keys on '^[[:space:]]*Triggers:'.
if ! grep -qE '^[[:space:]]*Triggers:' <<<"$FM"; then
  echo "FAIL: description lacks a line-start 'Triggers:' clause"
  FAIL=1
fi
if grep -qE '\*\*Triggers:\*\*' <<<"$FM"; then
  echo "FAIL: Triggers marker is bolded ('**Triggers:**'); use a plain 'Triggers:' line"
  FAIL=1
fi

# --- 3. Section spine --------------------------------------------------------
declare -a SPINE=(
  "# Codex Goals"
  "## ⚠️ Critical Constraints"
  "## Workflow / Methodology"
  "## Output Specification"
  "## Quality Rubric"
  "## Examples"
  "## Troubleshooting"
  "## See Also / References"
)
for sec in "${SPINE[@]}"; do
  if ! grep -qF "$sec" "$SKILL"; then
    echo "FAIL: missing required section '$sec'"
    FAIL=1
  fi
done

# --- 4. Form-A line budget (<= 250) -----------------------------------------
LINES="$(wc -l < "$SKILL" | tr -d ' ')"
if [ "$LINES" -gt 250 ]; then
  echo "FAIL: Form-A line budget exceeded ($LINES > 250)"
  FAIL=1
else
  echo "ok: line count $LINES (<= 250)"
fi

# --- 5. Bug guard: '-a' must never be paired with 'codex exec' ---------------
# codex exec has no -a/--ask-for-approval flag; it errors 'unexpected argument'.
if grep -nE 'codex exec[^\n]*[[:space:]]-a[[:space:]]' "$SKILL" >/dev/null 2>&1; then
  echo "FAIL: 'codex exec ... -a' found — exec rejects -a; use -c approval_policy=never"
  FAIL=1
else
  echo "ok: no 'codex exec -a' anti-pattern present"
fi

# --- 6. Tooling currency: the binary + feature this skill drives -------------
if command -v codex >/dev/null 2>&1; then
  if codex features list 2>/dev/null | grep -E '^goals[[:space:]]' | grep -q 'true'; then
    echo "ok: 'goals' feature is stable+true on this host"
  else
    echo "WARN: 'goals' not reported stable+true (codex features list) — verify before relying"
  fi
else
  echo "WARN: 'codex' not on PATH — skill is structurally valid but cannot run here"
fi

echo
if [ "$FAIL" -eq 0 ]; then
  echo "PASS: codex-goals is structurally sound"
  exit 0
fi
echo "FAIL: structural defects above"
exit 1
