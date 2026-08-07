#!/usr/bin/env bash
# Validate that every skill with "ao lookup" also has "ao metrics cite".
# Skills that retrieve knowledge must also record citations so retrieved
# evidence stays attributable.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
errors=0

check_dir() {
  local dir_label="$1"
  local search_dir="$2"

  while IFS= read -r -d '' skill_file; do
    # Skip reference files, examples, and non-SKILL.md
    [[ "$skill_file" == */references/* ]] && continue
    [[ "$skill_file" == */scripts/* ]] && continue
    [[ "$(basename "$skill_file")" != "SKILL.md" ]] && continue

    # Check if this skill has ao lookup
    if grep -q 'ao lookup' "$skill_file" 2>/dev/null; then
      # Skip skills that only mention ao lookup in docs/examples (not executable steps)
      # Require ao metrics cite in the same file
      if ! grep -q 'ao metrics cite' "$skill_file" 2>/dev/null; then
        # Allow skills that only reference ao lookup in documentation context
        # (e.g., using-agentops which documents the CLI, inject which is deprecated)
        local skill_name
        skill_name="$(basename "$(dirname "$skill_file")")"
        case "$skill_name" in
          using-agentops|inject|SKILL-TIERS|swarm)
            # These reference ao lookup in documentation/worker prompts, not as executable steps
            continue
            ;;
        esac
        echo "FAIL: $dir_label/$skill_name/SKILL.md has 'ao lookup' but no 'ao metrics cite'"
        errors=$((errors + 1))
      fi
    fi
  done < <(find "$search_dir" -name "SKILL.md" -print0 2>/dev/null)
}

echo "=== Skill Citation Parity Check ==="

check_dir "skills" "$REPO_ROOT/skills"
check_dir "skills-codex" "$REPO_ROOT/skills-codex"

if [ "$errors" -gt 0 ]; then
  echo ""
  echo "FAIL: $errors skill(s) retrieve knowledge without recording citations."
  echo "Fix: Add 'ao metrics cite \"<path>\" --type applied 2>/dev/null || true' after knowledge application."
  exit 1
else
  echo "PASS: All skills with ao lookup also have ao metrics cite ($errors errors)"
  exit 0
fi
