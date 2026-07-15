#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"

bash "$REPO_ROOT/skills/heal-skill/scripts/heal.sh" --check --strict "$SKILL_DIR"

for path in SKILL.md scripts/build.sh scripts/init.sh schemas/build-report.json; do
  [[ -f "$SKILL_DIR/$path" ]] || {
    echo "skill-builder validate: missing $path" >&2
    exit 1
  }
done

for script in scripts/build.sh scripts/init.sh; do
  [[ -x "$SKILL_DIR/$script" ]] || {
    echo "skill-builder validate: not executable: $script" >&2
    exit 1
  }
done

if rg -n 'from-pattern|flywheel close-loop|append-skill-disposition' "$SKILL_DIR/SKILL.md" \
  || rg -n 'git (status|commit|push)|ao land|retry|queue|lease' \
    "$SKILL_DIR/scripts/build.sh" "$SKILL_DIR/scripts/init.sh"; then
  echo "skill-builder validate: obsolete lifecycle behavior remains" >&2
  exit 1
fi

echo "skill-builder validate: PASS"
