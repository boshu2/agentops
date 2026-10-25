#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: rpi$' "$skill_dir/SKILL.md"
grep -Fq 'dependencies: [plan, implement, validate]' "$skill_dir/SKILL.md"
grep -Fq 'Own the authorized outcome through finish.' "$skill_dir/SKILL.md"
grep -Fq 'ordinary known' "$skill_dir/SKILL.md"
grep -Fq 'Acceptance changes need caller authority.' "$skill_dir/SKILL.md"
grep -Fq 'at most one bounded' "$skill_dir/SKILL.md"
grep -Fq 'reviewers remain required.' "$skill_dir/SKILL.md"
grep -Fq 'author-distinct' "$skill_dir/SKILL.md"
grep -Fq 'A repair does not start another' "$skill_dir/SKILL.md"
grep -Fq 'Authors cannot issue binding PASS.' "$skill_dir/SKILL.md"
grep -Fq 'When no machine' "$skill_dir/SKILL.md"
# The three rules the plugin eval measured: review only on named triggers, what
# the validator is given, and no `done` while a caller decision is open.
grep -Fq 'checks and CI are the gate: finish.' "$skill_dir/SKILL.md"
grep -Fq 'desired verdict; it does not re-run the checks.' "$skill_dir/SKILL.md"
grep -Fq 'done` while a choice is open.' "$skill_dir/SKILL.md"
if grep -Eq 'Plan is closed for that intent|dependencies:.*anti-ceremony|plan_packet_digest' "$skill_dir/SKILL.md"; then
  echo 'rpi retains a retired phase lock, mandatory specialist or planning packet' >&2
  exit 1
fi
for ref in boundaries outer-goal; do
  test -s "$skill_dir/references/$ref.md"
done
echo 'rpi lean skill contract: PASS'
