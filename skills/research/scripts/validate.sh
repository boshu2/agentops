#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: research$' "$skill_dir/SKILL.md"
grep -Fq 'Answer one bounded question with current evidence' "$skill_dir/SKILL.md"
grep -Fq 'Report unchecked scope and stop' "$skill_dir/SKILL.md"
grep -Fq 'Do not emit approval' "$skill_dir/SKILL.md"
grep -q '^Feature: Research answers one bounded question$' \
  "$skill_dir/references/research.feature"
python3 -m json.tool "$skill_dir/schemas/findings.json" >/dev/null

if rg -n 'ao lookup|ao land|auto-redo|Gate 1|\.agents/rpi/next-work|finding-compiler' \
  "$skill_dir/SKILL.md" "$skill_dir/references" "$skill_dir/schemas"; then
  echo 'research contract contains retired lifecycle behavior' >&2
  exit 1
fi

echo 'research skill contract: PASS'
