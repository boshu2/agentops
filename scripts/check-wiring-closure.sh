#!/usr/bin/env bash
# Validates artifact-to-reference wiring across the project.
# Exit 0 = all wired, exit 1 = orphans found.
# Used as a GOALS.yaml meta-goal.
set -euo pipefail

ERRORS=0

# 1. Every scripts/check-*.sh must appear in GOALS.yaml
if [ -f GOALS.yaml ]; then
  for script in scripts/check-*.sh; do
    [ -f "$script" ] || continue
    base=$(basename "$script")
    if ! grep -q "$base" GOALS.yaml 2>/dev/null; then
      echo "UNWIRED SCRIPT: $base not referenced in GOALS.yaml"
      ERRORS=$((ERRORS + 1))
    fi
  done
fi

# 2. Every hook script referenced in hooks.json must exist on disk
if [ -f hooks/hooks.json ]; then
  # Extract script paths from hooks.json command fields
  scripts_referenced=$(grep -oE 'hooks/[a-zA-Z0-9_-]+\.sh' hooks/hooks.json 2>/dev/null | sort -u)
  for script in $scripts_referenced; do
    if ! [ -f "$script" ]; then
      echo "MISSING HOOK SCRIPT: $script referenced in hooks.json but does not exist"
      ERRORS=$((ERRORS + 1))
    fi
  done
fi

# 3. Every skill directory should appear in SKILL-TIERS.md
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

# 4. Every lib/scripts/*.sh should be referenced by at least one SKILL.md or hook
for lib_script in lib/scripts/*.sh; do
  [ -f "$lib_script" ] || continue
  base=$(basename "$lib_script")
  if ! grep -rq "$base" skills/*/SKILL.md hooks/ 2>/dev/null; then
    echo "ORPHANED LIB SCRIPT: $base not referenced by any skill or hook"
    ERRORS=$((ERRORS + 1))
  fi
done

# 5. Every lib/*.sh helper should be referenced by at least one hook
for helper_script in lib/*.sh; do
  [ -f "$helper_script" ] || continue
  base=$(basename "$helper_script")
  if ! grep -rq "$base" hooks/ cli/embedded/hooks/ 2>/dev/null; then
    echo "ORPHANED LIB HELPER: $base not referenced by any hook"
    ERRORS=$((ERRORS + 1))
  fi
done

if [ "$ERRORS" -eq 0 ]; then
  echo "All wiring checks passed"
  exit 0
fi

echo "Found $ERRORS wiring issue(s)"
exit 1
