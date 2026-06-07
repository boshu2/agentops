#!/usr/bin/env bash
# validate.sh — self-check for the agy-rules-workflows skill (AUTHORING-STANDARD §6/§8).
# Checks: frontmatter completeness, the required section spine + order, the Form-A
# line budget (<=250), the mandatory line-start Triggers: marker, companion-artifact
# presence (skill.spec.json + this scripts/ dir), and that every references/ link in
# SKILL.md resolves on disk. Exit 0 on pass, 1 on any failure.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL="$ROOT/SKILL.md"
SPEC="$ROOT/skill.spec.json"

fail=0
err() { printf 'FAIL: %s\n' "$1" >&2; fail=1; }
ok()  { printf 'ok: %s\n' "$1"; }

# --- 0. SKILL.md exists -------------------------------------------------------
[ -f "$SKILL" ] || { err "SKILL.md missing at $SKILL"; exit 1; }

# --- 1. Frontmatter present + required keys -----------------------------------
fm="$(awk 'NR==1 && $0!="---"{exit 1} NR>1 && $0=="---"{exit 0} {print}' "$SKILL" 2>/dev/null || true)"
[ -n "$fm" ] || err "frontmatter block (--- ... ---) not found at top of SKILL.md"
for key in "name:" "description:" "skill_api_version:" "metadata:" "output_contract:"; do
  printf '%s\n' "$fm" | grep -q "^$key" || err "frontmatter missing required key: $key"
done
# name must equal the directory name (lowercase-hyphen rule).
dir_name="$(basename "$ROOT")"
fm_name="$(printf '%s\n' "$fm" | awk -F': *' '/^name:/{print $2; exit}')"
[ "$fm_name" = "$dir_name" ] || err "frontmatter name ($fm_name) != directory ($dir_name)"

# --- 2. Triggers: marker (FAIL-severity) -------------------------------------
grep -qE '^[[:space:]]*Triggers:' "$SKILL" || err "no line-start 'Triggers:' marker in description"

# --- 3. Required section spine, in order --------------------------------------
spine=(
  "# agy-rules-workflows"
  "## Overview / When to Use"
  "## ⚠️ Critical Constraints"
  "## Workflow / Methodology"
  "## Output Specification"
  "## Quality Rubric"
  "## Examples"
  "## Troubleshooting"
  "## See Also / References"
)
last=0
for sec in "${spine[@]}"; do
  ln="$(grep -nF -m1 "$sec" "$SKILL" | cut -d: -f1 || true)"
  if [ -z "$ln" ]; then
    err "required section missing: $sec"
  elif [ "$ln" -lt "$last" ]; then
    err "section out of order: $sec (line $ln before previous $last)"
  else
    last="$ln"
  fi
done

# --- 4. Form-A line budget (<= 250) ------------------------------------------
lines="$(wc -l < "$SKILL" | tr -d ' ')"
if [ "$lines" -le 250 ]; then ok "line budget $lines/250 (Form A)"; else err "Form-A budget exceeded: $lines > 250"; fi

# --- 5. References resolve (one level deep) ----------------------------------
# Every SKILL-LOCAL references/<file>.md must exist on disk. Match only when
# 'references/' is NOT preceded by another path segment (skip absolute paths
# like ~/.agents/research/... that live outside this skill).
while IFS= read -r relpath; do
  [ -z "$relpath" ] && continue
  if [ -f "$ROOT/$relpath" ]; then ok "ref resolves: $relpath"; else err "dead reference: $relpath"; fi
done < <(grep -oE '(^|[^/A-Za-z0-9_-])references/[A-Za-z0-9_-]+\.md' "$SKILL" \
           | grep -oE 'references/[A-Za-z0-9_-]+\.md' | sort -u)

# --- 6. Companion artifacts ---------------------------------------------------
[ -f "$SPEC" ] || err "skill.spec.json missing"
if [ -f "$SPEC" ] && command -v python3 >/dev/null 2>&1; then
  python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$SPEC" \
    && ok "skill.spec.json is valid JSON" || err "skill.spec.json is not valid JSON"
fi
# Hook stub scripts referenced by the reference file must exist + be executable.
for s in scope-guard.sh close-guard.sh; do
  if [ -x "$SCRIPT_DIR/$s" ]; then ok "hook script present + executable: $s"; else err "missing/non-exec hook script: $s"; fi
done

# --- 7. No banned worker-dispatch directive ----------------------------------
# This skill is AGY-native; it must never instruct dispatching workers via
# 'claude -p' (billing/membrane rule). The skill NAMES the sibling Claude skins
# in prose, so only flag an imperative "use ... claude -p".
if grep -niE '\b(use|run|via|with|dispatch[a-z ]*) [^.]*claude (-p|--print)' "$SKILL" >/dev/null; then
  err "a directive to USE 'claude -p' appears in SKILL.md"
else
  ok "no 'claude -p' worker-dispatch directive"
fi

if [ "$fail" -eq 0 ]; then
  printf '\nPASS: agy-rules-workflows SKILL.md meets the authoring standard.\n'
  exit 0
else
  printf '\nFAILED: fix the issues above.\n' >&2
  exit 1
fi
