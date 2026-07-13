#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
validate_skill="$repo_root/skills/validate/SKILL.md"
learn_skill="$repo_root/skills/learn/SKILL.md"
postmortem_skill="$repo_root/skills/postmortem/SKILL.md"
verdict_schema="$repo_root/schemas/verdict.v1.schema.json"
learn_schema="$repo_root/skills/learn/schemas/learn-receipt.schema.json"
fixtures="$repo_root/tests/fixtures/validation-learning-boundary"

failures=0
fail() { printf 'FAIL: %s\n' "$1" >&2; failures=$((failures + 1)); }
require_text() { grep -Fq -- "$2" "$1" || fail "$3"; }
reject_pattern() { grep -Eiq -- "$2" "$1" && fail "$3" || true; }

scan_validate_execution() {
  grep -Eiq 'Skill\(skill="(learn|postmortem|premortem|push)"|ao (pawl|land|ratchet record|flywheel|forge)|git (commit|push)|br (close|update)|\.agents/findings/|AUTO-REDO' "$1"
}

scan_learn_execution() {
  grep -Eiq 'Skill\(skill="(premortem|push)"|ao (pawl|land)|git (commit|push)|br (close|update)|jq .*\.verdict|sed .*verdict' "$1"
}

for path in "$validate_skill" "$learn_skill" "$postmortem_skill" "$verdict_schema" "$learn_schema"; do
  [[ -s "$path" ]] || fail "missing boundary surface: ${path#"$repo_root"/}"
done

require_text "$validate_skill" 'Validate ends at proof.' 'Validate must declare its proof-only terminal boundary'
require_text "$validate_skill" 'Structured observations are part of the immutable verdict' 'Validate must emit structured observations in the verdict'
if scan_validate_execution "$validate_skill"; then
  fail 'Validate must not execute Learn/Postmortem/Premortem, delivery, tracker, finding promotion, or retry control'
fi
reject_pattern "$validate_skill" 'dependencies:[[:space:]]*\[[^]]*(pawl-review|learn|postmortem)' 'Validate frontmatter must not depend on Pawl, Learn, or Postmortem'

require_text "$learn_skill" 'The input verdict is immutable.' 'Learn must declare immutable verdict input'
require_text "$learn_skill" 'input_verdict_digest' 'Learn must bind observations to the input verdict digest'
require_text "$learn_skill" 'Postmortem is optional and runs only for retrospective causal analysis.' 'Learn must keep Postmortem optional and causal-only'
if scan_learn_execution "$learn_skill"; then
  fail 'Learn must not mutate verdicts or operate Premortem, delivery, Git, or tracker state'
fi

require_text "$postmortem_skill" 'retrospective causal analysis' 'Postmortem must be a causal-analysis specialization'
require_text "$postmortem_skill" 'does not re-run acceptance validation by default' 'Postmortem must not re-run the full acceptance proof by default'

SCHEMA="$verdict_schema" LEARN_SCHEMA="$learn_schema" python3 - <<'PY' || failures=$((failures + 1))
import json
import os
from pathlib import Path

verdict = json.loads(Path(os.environ["SCHEMA"]).read_text(encoding="utf-8"))
learn = json.loads(Path(os.environ["LEARN_SCHEMA"]).read_text(encoding="utf-8"))
if "observations" not in verdict.get("required", []):
    raise SystemExit("FAIL: verdict schema must require structured observations")
item = verdict.get("properties", {}).get("observations", {}).get("items", {})
required = set(item.get("required", []))
if not {"summary", "evidence_ref", "kind"}.issubset(required):
    raise SystemExit("FAIL: verdict observations must require summary, evidence_ref, and kind")
if "input_verdict_digest" not in learn.get("required", []):
    raise SystemExit("FAIL: Learn receipt must require input_verdict_digest")
if "verdict" in learn.get("properties", {}):
    raise SystemExit("FAIL: Learn receipt must not carry a mutable verdict field")
PY

[[ -s "$fixtures/valid-boundary.md" ]] || fail 'missing valid boundary fixture'
[[ -s "$fixtures/invalid-validate-delivery.md" ]] || fail 'missing invalid Validate fixture'
[[ -s "$fixtures/invalid-learn-mutation.md" ]] || fail 'missing invalid Learn fixture'
scan_validate_execution "$fixtures/valid-boundary.md" && fail 'valid fixture tripped Validate scanner' || true
scan_learn_execution "$fixtures/valid-boundary.md" && fail 'valid fixture tripped Learn scanner' || true
scan_validate_execution "$fixtures/invalid-validate-delivery.md" || fail 'Validate negative fixture did not exercise forbidden delivery'
scan_learn_execution "$fixtures/invalid-learn-mutation.md" || fail 'Learn negative fixture did not exercise verdict mutation'

if [[ $failures -ne 0 ]]; then
  printf 'validation-learning boundary: FAIL (%d)\n' "$failures" >&2
  exit 1
fi
echo 'validation-learning boundary: PASS'
