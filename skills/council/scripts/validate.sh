#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: council$' "$skill_dir/SKILL.md"
grep -Fq 'optional judgment strategy' "$skill_dir/SKILL.md"
grep -Fq 'does not mint a verdict of any version' "$skill_dir/SKILL.md"
grep -Fq 'Freeze one allowlisted packet' "$skill_dir/SKILL.md"
grep -Fq 'sends TERM then KILL to the whole' "$skill_dir/SKILL.md"
grep -Fq 'group and waits for cleanup' "$skill_dir/SKILL.md"
grep -Fq 'round_status` to `insufficient' "$skill_dir/SKILL.md"
grep -Fq '`run-check` bounded runner' "$skill_dir/SKILL.md"
test -f "$skill_dir/schemas/council-report.v1.schema.json"
test -x "$skill_dir/scripts/validate-output.sh"
python3 -m json.tool "$skill_dir/schemas/council-report.v1.schema.json" >/dev/null

if grep -Eiq 'ao (pawl|land)|git (commit|push)|br (close|update)|auto-redo' \
  "$skill_dir/SKILL.md"; then
  echo 'council contract contains forbidden lifecycle authority' >&2
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
digest = "a" * 64
base = {
    "schema_version": "council-report.v1",
    "question": "Which bounded option is supported?",
    "subject_digest": digest,
    "authorization_id": "caller:council-fixture",
    "input_packet": {"source_refs": ["intent.md"], "byte_length": 128, "sha256": digest},
    "limits": {
        "judge_count": 2,
        "per_judge_timeout_seconds": 30,
        "round_timeout_seconds": 90,
        "max_judge_output_bytes": 4096,
    },
    "round_status": "sufficient",
    "judge_attempts": [
        {"context_id": "judge-a", "adapter": "fixture", "model_identity": "model-a", "status": "completed", "output_bytes": 64, "cleanup_confirmed": True},
        {"context_id": "judge-b", "adapter": "fixture", "model_identity": "model-b", "status": "completed", "output_bytes": 64, "cleanup_confirmed": True},
    ],
    "judges": [
        {"context_id": "judge-a", "methodology": "static reading", "model_identity": "model-a", "judgment": "Option A is supported.", "evidence": ["intent.md#acceptance"]},
        {"context_id": "judge-b", "methodology": "counterexample search", "model_identity": "model-b", "judgment": "Option A survives the fixture.", "evidence": ["intent.md#bounds"]},
    ],
    "synthesis": {
        "consensus": [{"claim": "Option A is bounded.", "methodologies": ["static reading", "counterexample search"]}],
        "divergence": [], "minority": [], "unresolved": [],
    },
    "cleanup_verified": True,
}

def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")

emit("sufficient.json", base)
incomplete = copy.deepcopy(base)
incomplete.update({"round_status": "insufficient", "judges": [], "synthesis": {"consensus": [], "divergence": [], "minority": [], "unresolved": ["Both judges timed out."]}})
for attempt in incomplete["judge_attempts"]:
    attempt.update({"status": "timed_out", "output_bytes": 0, "error": "deadline exceeded"})
emit("timed-out.json", incomplete)
missing_auth = copy.deepcopy(base)
missing_auth["authorization_id"] = ""
emit("missing-auth.json", missing_auth)
oversized = copy.deepcopy(base)
oversized["input_packet"]["byte_length"] = 262145
emit("oversized.json", oversized)
unclean = copy.deepcopy(incomplete)
unclean["judge_attempts"][0]["cleanup_confirmed"] = False
emit("unclean.json", unclean)
thin_consensus = copy.deepcopy(incomplete)
thin_consensus["synthesis"]["consensus"] = [{"claim": "unsupported", "methodologies": ["none"]}]
emit("thin-consensus.json", thin_consensus)
echo_consensus = copy.deepcopy(base)
echo_consensus["synthesis"]["consensus"][0]["methodologies"] = ["static reading"]
emit("echo-consensus.json", echo_consensus)
PY

bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/sufficient.json"
bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/timed-out.json"
for rejected in missing-auth oversized unclean thin-consensus echo-consensus; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "council contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

echo 'council skill contract: PASS'
