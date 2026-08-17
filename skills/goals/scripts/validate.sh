#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_skills="$(cd "$skill_dir/.." && pwd)"
skill="$skill_dir/SKILL.md"
fitness="$repo_skills/fitness/SKILL.md"

grep -q '^name: goals$' "$skill"
grep -Fq 'fitness output unchanged' "$skill"
for effect in read_goals_source read_goal_history_and_evidence optionally_write_goal_snapshot optionally_write_rendered_spec; do
  grep -Fq "$effect" "$skill"
  grep -Fq "$effect" "$fitness"
done
grep -Fq 'routers and operators do not mistake an alias invocation for read-only work' "$skill"
test -x "$repo_skills/fitness/scripts/validate-output.sh"
bash "$repo_skills/fitness/scripts/validate.sh"

echo 'goals alias contract: PASS'
