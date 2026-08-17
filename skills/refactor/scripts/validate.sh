#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
runner="$repo_root/skills/validate/scripts/validate.py"
grep -q '^name: refactor$' "$skill_dir/SKILL.md"
# test_behavior_preservation
grep -Fq 'preserving observable behavior' "$skill_dir/SKILL.md"
grep -Fq 'Record an honest baseline' "$skill_dir/SKILL.md"
grep -Fq 'Apply one bounded transformation' "$skill_dir/SKILL.md"
grep -Fq 'same set of pre-existing failures' "$skill_dir/SKILL.md"
grep -Fq 'post-hoc neutrality' "$skill_dir/SKILL.md"
# test_bounded_commands
grep -q '^## Execution constraints$' "$skill_dir/SKILL.md"
grep -Fq 'scripts/validate.py run-check' "$skill_dir/SKILL.md"
grep -Fq 'terminate and reap the complete process group' "$skill_dir/SKILL.md"
grep -Fq 'primary tree retains its pre-command digest' "$skill_dir/SKILL.md"
grep -Fq 'an arbitrary check cannot inherit authority' "$skill_dir/references/refactor.feature"
grep -Fq 'a failed disposable check cannot leak mutation' "$skill_dir/references/refactor.feature"

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
disposable="$fixture_dir/disposable"
mkdir -p "$disposable/work"
printf '%s\n' 'unchanged primary fixture' > "$fixture_dir/primary.txt"
primary_before="$(shasum -a 256 "$fixture_dir/primary.txt" | awk '{print $1}')"

containment_available=false
if [[ "$(uname -s)" == "Darwin" && -x /usr/bin/sandbox-exec ]] || command -v bwrap >/dev/null; then
  containment_available=true
fi

if [[ "$containment_available" == true ]]; then
python3 "$runner" run-check \
  --cwd "$disposable/work" \
  --disposable-root "$disposable" \
  --authorization-id 'intent:refactor-fixture' \
  --timeout-seconds 2 \
  --max-output-bytes 4096 \
  --effect read-only \
  -- /bin/sh -c 'test -f ../missing || true' > "$fixture_dir/read-only.json"
python3 - "$fixture_dir/read-only.json" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["result"] == "PASS"
assert value["process_group_reaped"] is True
assert value["mutation_detected"] is False
PY

if python3 "$runner" run-check \
  --cwd "$disposable/work" \
  --disposable-root "$disposable" \
  --authorization-id '' \
  --timeout-seconds 2 \
  --max-output-bytes 4096 \
  --effect disposable-mutation \
  -- /bin/sh -c 'printf leaked > unauthorized.txt'; then
  echo 'refactor contract allowed a command without authorization' >&2
  exit 1
fi
[[ ! -e "$disposable/work/unauthorized.txt" ]]

python3 "$runner" run-check \
  --cwd "$disposable/work" \
  --disposable-root "$disposable" \
  --authorization-id 'caller:refactor-mutation-fixture' \
  --timeout-seconds 2 \
  --max-output-bytes 4096 \
  --effect disposable-mutation \
  -- /bin/sh -c 'printf disposable > candidate.txt' > "$fixture_dir/mutation.json"
[[ -f "$disposable/work/candidate.txt" ]]
[[ "$(shasum -a 256 "$fixture_dir/primary.txt" | awk '{print $1}')" == "$primary_before" ]]
python3 - "$fixture_dir/mutation.json" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["result"] == "PASS"
assert value["mutation_detected"] is True
PY
else
  if python3 "$runner" run-check \
    --cwd "$disposable/work" \
    --disposable-root "$disposable" \
    --authorization-id 'caller:no-backend-fixture' \
    --timeout-seconds 2 \
    --max-output-bytes 4096 \
    --effect disposable-mutation \
    -- /bin/sh -c 'printf leaked > unavailable.txt'; then
    echo 'refactor contract ran without a containment backend' >&2
    exit 1
  fi
  [[ ! -e "$disposable/work/unavailable.txt" ]]
fi

echo 'refactor skill contract: PASS'
