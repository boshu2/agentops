#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"

for path in \
  SKILL.md \
  scripts/heal.sh \
  scripts/audit.sh \
  scripts/score_agentops_skill.py \
  schemas/audit-report.json \
  references/audit-checks.md \
  references/codex-parity.md; do
  [[ -f "$SKILL_DIR/$path" ]] || {
    echo "heal-skill validate: missing $path" >&2
    exit 1
  }
done

bash -n "$SKILL_DIR/scripts/heal.sh" "$SKILL_DIR/scripts/audit.sh"
bash "$SKILL_DIR/scripts/heal.sh" --check --strict "$SKILL_DIR"

before="$(find "$SKILL_DIR" -type f -exec shasum -a 256 {} + | sort | shasum -a 256 | awk '{print $1}')"
bash "$SKILL_DIR/scripts/heal.sh" --check "$SKILL_DIR" >/dev/null
after="$(find "$SKILL_DIR" -type f -exec shasum -a 256 {} + | sort | shasum -a 256 | awk '{print $1}')"
[[ "$before" == "$after" ]] || {
  echo "heal-skill validate: check mode mutated its target" >&2
  exit 1
}

if rg -n 'ao land|git (commit|push)|append-skill-disposition|flywheel close-loop' \
  "$SKILL_DIR/scripts/heal.sh" "$SKILL_DIR/scripts/audit.sh" \
  "$SKILL_DIR/scripts/score_agentops_skill.py"; then
  echo "heal-skill validate: lifecycle authority remains" >&2
  exit 1
fi

echo "heal-skill validate: PASS"
