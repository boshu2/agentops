#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: premortem$' "$skill_dir/SKILL.md"
grep -Fq 'optional plan-challenge strategy' "$skill_dir/SKILL.md"
grep -Fq 'It is not part of the required RPI sequence' "$skill_dir/SKILL.md"
grep -Fq 'advisory findings' "$skill_dir/SKILL.md"
grep -Fq 'Freeze a bounded input packet before dispatch' "$skill_dir/SKILL.md"
grep -Fq 'sends TERM then KILL to the whole' "$skill_dir/SKILL.md"
grep -Fq 'group and waits for cleanup' "$skill_dir/SKILL.md"
grep -Fq 'restoration/digest verification failure stops' "$skill_dir/SKILL.md"
grep -q '^Feature: Premortem optionally challenges one frozen plan$' \
  "$skill_dir/references/premortem.feature"
test -f "$skill_dir/schemas/premortem-plan-review.v1.schema.json"
test -x "$skill_dir/scripts/validate-output.sh"
python3 -m json.tool "$skill_dir/schemas/premortem-plan-review.v1.schema.json" >/dev/null

if grep -Eiq 'ao (pawl|land)|git (commit|push)|br (close|update)|auto-redo|next[_ -]action' \
  "$skill_dir/SKILL.md"; then
  echo 'premortem contract contains forbidden lifecycle authority' >&2
  exit 1
fi

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
python3 - "$fixture_dir" <<'PY'
import copy
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
digest = "c" * 64
base = {
    "schema_version": "premortem-plan-review.v1",
    "intent_digest": digest,
    "author_context_id": "author-fixture",
    "judge_context_id": "judge-fixture",
    "authorization_id": "caller:premortem-fixture",
    "judge_adapter": "fixture",
    "judge_model_identity": "model-a",
    "judge_status": "completed",
    "judge_output_bytes": 512,
    "judge_process_group_reaped": True,
    "disposable_state_clean": True,
    "input_packet": {"source_refs": ["intent.md"], "byte_length": 512, "sha256": digest},
    "limits": {"judge_timeout_seconds": 30, "overall_timeout_seconds": 60, "max_judge_output_bytes": 4096},
    "findings": [{"id": "risk-1", "statement": "The rollback assertion lacks a fixture.", "evidence": ["intent.md#rollback"]}],
    "checked": ["rollback criterion"],
    "not_checked": [],
}

def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")

emit("completed.json", base)
timed_out = copy.deepcopy(base)
timed_out.update({"judge_status": "timed_out", "judge_output_bytes": 0, "findings": [], "checked": [], "not_checked": ["judge did not return"]})
emit("timed-out.json", timed_out)
missing_auth = copy.deepcopy(base)
missing_auth["authorization_id"] = ""
emit("missing-auth.json", missing_auth)
oversized = copy.deepcopy(base)
oversized["input_packet"]["byte_length"] = 262145
emit("oversized.json", oversized)
unclean = copy.deepcopy(timed_out)
unclean["judge_process_group_reaped"] = False
emit("unclean.json", unclean)
false_finding = copy.deepcopy(timed_out)
false_finding["findings"] = [{"id": "invented", "statement": "A timed-out judge said this.", "evidence": ["none"]}]
emit("false-finding.json", false_finding)
PY

bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/completed.json"
bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/timed-out.json"
for rejected in missing-auth oversized unclean false-finding; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "premortem contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

echo 'premortem skill contract: PASS'
