#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grep -q '^name: learn$' "$skill_dir/SKILL.md"
grep -Fq 'optional, off-path consumer' "$skill_dir/SKILL.md"
grep -Fq 'default TTL is 7 days' "$skill_dir/SKILL.md"
grep -Fq 'scripts/prune-expired.sh --apply' "$skill_dir/SKILL.md"
if grep -Eiq 'receipt|plan_impact|next_action|retry|delivery|closure' "$skill_dir/SKILL.md"; then
  echo 'learn contract contains forbidden lifecycle vocabulary' >&2
  exit 1
fi

scratch_tmp="$(mktemp -d)"
trap 'rm -rf -- "$scratch_tmp"' EXIT
learn_root="$scratch_tmp/.agents/scratch/learn"
outside="$scratch_tmp/outside.json"
mkdir -p "$learn_root"
printf '%s\n' '{"schema_version":"learning-observations.v1","created_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-08T00:00:00Z","source_digests":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"observations":["expired"]}' > "$learn_root/expired.json"
printf '%s\n' '{"schema_version":"learning-observations.v1","created_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-20T00:00:00Z","source_digests":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"observations":["live"]}' > "$learn_root/live.json"
printf '%s\n' '{"not":"owned"}' > "$outside"
ln -s "$outside" "$learn_root/external.json"
if bash "$skill_dir/scripts/prune-expired.sh" --root "$learn_root" --authorization-id '' --now 2026-01-10T00:00:00Z --apply; then
  echo 'learn contract accepted cleanup without authorization' >&2
  exit 1
fi
[[ -f "$learn_root/expired.json" ]]
bash "$skill_dir/scripts/prune-expired.sh" --root "$learn_root" --authorization-id test:ttl --now 2026-01-10T00:00:00Z --apply
[[ ! -e "$learn_root/expired.json" ]]
[[ -f "$learn_root/live.json" ]]
[[ -L "$learn_root/external.json" ]]
[[ -f "$outside" ]]
bash "$skill_dir/scripts/validate-output.sh" "$learn_root/live.json"

crowded_root="$scratch_tmp/crowded/.agents/scratch/learn"
mkdir -p "$crowded_root"
python3 - "$crowded_root" <<'PY'
import sys
from pathlib import Path
root = Path(sys.argv[1])
for index in range(1001):
    (root / f"entry-{index}").touch()
PY
if bash "$skill_dir/scripts/prune-expired.sh" --root "$crowded_root" --authorization-id test:ceiling --now 2026-01-10T00:00:00Z; then
  echo 'learn contract accepted a directory above its entry ceiling' >&2
  exit 1
fi
echo 'learn skill contract: PASS'
