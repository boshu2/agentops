#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skill="$skill_dir/SKILL.md"
grep -q '^name: operationalize$' "$skill"
grep -q '^## Constraints$' "$skill"
grep -Fq 'Redact restricted values' "$skill"
grep -Fq 'obtain separate' "$skill"
grep -Fq 'errors name only the' "$skill"
test -x "$skill_dir/scripts/publish-output.sh"

test_tmp="$(mktemp -d)"
trap 'rm -rf -- "$test_tmp"' EXIT
clean="$test_tmp/clean.md"
sensitive="$test_tmp/sensitive.md"
printf '%s\n' '# Proposal' '## Sensitive-output review' '- Classification: internal' '- Redaction review: passed' '- Model approval: none' '- Sensitive approval: none' '- Audience: caller' "- Output path: $clean" '- Retention until: none' > "$clean"
bash "$skill_dir/scripts/validate-output.sh" "$clean"
printf '%s\n' '# Proposal' 'Authorization: Bearer abcdefghijklmnopqrstuvwxyz' '## Sensitive-output review' '- Classification: restricted' '- Redaction review: passed' '- Model approval: caller:model-fixture' '- Sensitive approval: caller:write-fixture' '- Audience: local-reviewer' "- Output path: $sensitive" '- Retention until: 2026-09-01T00:00:00Z' > "$sensitive"
if bash "$skill_dir/scripts/validate-output.sh" "$sensitive"; then
  echo 'operationalize contract: sensitive fixture passed without approval' >&2
  exit 1
fi
bash "$skill_dir/scripts/validate-output.sh" "$sensitive" \
  --model-output-approval caller:model-fixture \
  --sensitive-output-approval caller:write-fixture \
  --authorized-path "$sensitive" \
  --authorized-audience local-reviewer
if bash "$skill_dir/scripts/validate-output.sh" "$sensitive" \
  --model-output-approval caller:model-fixture \
  --sensitive-output-approval caller:write-fixture \
  --authorized-path "$sensitive" \
  --authorized-audience wrong-audience; then
  echo 'operationalize contract accepted an approval for another audience' >&2
  exit 1
fi

publish_root="$test_tmp/project/.agents/scratch/operationalize"
mkdir -p "$publish_root"
destination="$publish_root/proposal.md"
candidate="$test_tmp/candidate.md"
printf '%s\n' '# Proposal' 'Authorization: Bearer abcdefghijklmnopqrstuvwxyz' '## Sensitive-output review' '- Classification: restricted' '- Redaction review: passed' '- Model approval: caller:model-fixture' '- Sensitive approval: caller:write-fixture' '- Audience: local-reviewer' "- Output path: $destination" '- Retention until: 2026-09-01T00:00:00Z' > "$candidate"
if bash "$skill_dir/scripts/publish-output.sh" \
  --source "$candidate" \
  --output-root "$publish_root" \
  --destination "$destination" \
  --authorization-id caller:publish-fixture; then
  echo 'operationalize contract published a sensitive candidate without approvals' >&2
  exit 1
fi
[[ ! -e "$destination" ]]
bash "$skill_dir/scripts/publish-output.sh" \
  --source "$candidate" \
  --output-root "$publish_root" \
  --destination "$destination" \
  --authorization-id caller:publish-fixture \
  --model-output-approval caller:model-fixture \
  --sensitive-output-approval caller:write-fixture \
  --authorized-audience local-reviewer
cmp "$candidate" "$destination"
destination_digest="$(shasum -a 256 "$destination" | awk '{print $1}')"
if bash "$skill_dir/scripts/publish-output.sh" \
  --source "$candidate" \
  --output-root "$publish_root" \
  --destination "$destination" \
  --authorization-id caller:publish-fixture \
  --model-output-approval caller:model-fixture \
  --sensitive-output-approval caller:write-fixture \
  --authorized-audience local-reviewer; then
  echo 'operationalize contract overwrote an existing destination' >&2
  exit 1
fi
[[ "$(shasum -a 256 "$destination" | awk '{print $1}')" == "$destination_digest" ]]
echo 'operationalize skill contract: PASS'
