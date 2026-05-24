#!/usr/bin/env bash
# Validates artifact-to-reference wiring across the project.
# Exit 0 = all wired, exit 1 = orphans found.
# Used as a GOALS.yaml meta-goal.
set -euo pipefail

ERRORS=0

# 1. Every scripts/check-*.sh must be referenced somewhere (goals, CI, tests, or other scripts)
for script in scripts/check-*.sh; do
  [ -f "$script" ] || continue
  base=$(basename "$script")
  if ! grep -rq "$base" GOALS.md GOALS.yaml .github/workflows/ tests/ scripts/ 2>/dev/null; then
    echo "UNWIRED SCRIPT: $base not referenced in goals, CI, tests, or scripts"
    ERRORS=$((ERRORS + 1))
  fi
done

# 2. Every skill directory should appear in SKILL-TIERS.md
if [ -f skills/SKILL-TIERS.md ]; then
  for skill_dir in skills/*/; do
    [ -d "$skill_dir" ] || continue
    skill_name=$(basename "$skill_dir")
    if ! grep -q "$skill_name" skills/SKILL-TIERS.md 2>/dev/null; then
      echo "UNWIRED SKILL: $skill_name not in SKILL-TIERS.md"
      ERRORS=$((ERRORS + 1))
    fi
  done
fi

# 3. Every lib/scripts/*.sh should be referenced by at least one SKILL.md, script, or test
for lib_script in lib/scripts/*.sh; do
  [ -f "$lib_script" ] || continue
  base=$(basename "$lib_script")
  if ! grep -rq "$base" skills/*/SKILL.md scripts/ tests/ 2>/dev/null; then
    echo "ORPHANED LIB SCRIPT: $base not referenced by any skill, script, or test"
    ERRORS=$((ERRORS + 1))
  fi
done

# 4. Every lib/*.sh helper should be referenced by the CLI, a script, a skill, or a test
for helper_script in lib/*.sh; do
  [ -f "$helper_script" ] || continue
  base=$(basename "$helper_script")
  if ! grep -rq "$base" cli/ scripts/ skills/ tests/ 2>/dev/null; then
    echo "ORPHANED LIB HELPER: $base not referenced by the CLI, scripts, skills, or tests"
    ERRORS=$((ERRORS + 1))
  fi
done

if [ "$ERRORS" -eq 0 ]; then
  echo "All wiring checks passed"
  exit 0
fi

echo "Found $ERRORS wiring issue(s)"
exit 1
