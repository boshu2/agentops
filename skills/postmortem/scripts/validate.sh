#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

grep -q '^name: postmortem$' "$skill_dir/SKILL.md"
grep -Fq 'retrospective causal analysis' "$skill_dir/SKILL.md"
grep -Fq 'does not re-run acceptance validation' "$skill_dir/SKILL.md"
grep -Fq 'counterfactual' "$skill_dir/SKILL.md"
grep -Fq 'Empty or inconclusive analysis is valid' "$skill_dir/SKILL.md"
grep -Fq 'Freeze at most 20 allowlisted evidence sources' "$skill_dir/SKILL.md"
grep -Fq 'terminates and reaps the whole group' "$skill_dir/SKILL.md"
grep -Fq 'postmortem-run.v1' "$skill_dir/SKILL.md"
grep -Fq '`run-check` bounded runner' "$skill_dir/SKILL.md"
grep -q '^Feature: Postmortem tests retrospective causal claims$' "$skill_dir/references/postmortem.feature"
test -x "$skill_dir/scripts/validate-output.sh"

if grep -Eiq 'ao (pawl|land)|git (commit|push)|br (close|update)' "$skill_dir/SKILL.md"; then
  echo 'postmortem contract contains forbidden delivery or tracker execution' >&2
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
headings = [
    "Causal Question", "Pinned Inputs", "Timeline", "Supported Claims",
    "Rejected Claims", "Counterfactuals", "Unknowns", "Experiments",
]
report = "# Postmortem fixture\n\n" + "\n\n".join(f"## {heading}\n\nFixture evidence." for heading in headings) + "\n"
(root / "report.md").write_text(report, encoding="utf-8")
(root / "incomplete-report.md").write_text(report + "\n## Incomplete\n\nThe declared judge timed out.\n", encoding="utf-8")
digest = "d" * 64
base = {
    "schema_version": "postmortem-run.v1",
    "authorization_id": "caller:postmortem-fixture",
    "input_packet": {"source_refs": ["verdict.json"], "byte_length": 1024, "sha256": digest},
    "limits": {"max_judges": 1, "per_judge_timeout_seconds": 30, "overall_timeout_seconds": 90, "max_judge_output_bytes": 4096},
    "judge_attempts": [{"context_id": "judge-a", "adapter": "fixture", "model_identity": "model-a", "status": "completed", "output_bytes": 256, "cleanup_confirmed": True}],
    "cleanup_verified": True,
    "subject_unchanged": True,
    "status": "complete",
}

def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")

emit("complete.json", base)
incomplete = copy.deepcopy(base)
incomplete.update({"status": "incomplete"})
incomplete["judge_attempts"][0].update({"status": "timed_out", "output_bytes": 0, "error": "deadline exceeded"})
emit("incomplete.json", incomplete)
missing_auth = copy.deepcopy(base)
missing_auth["authorization_id"] = ""
emit("missing-auth.json", missing_auth)
oversized = copy.deepcopy(base)
oversized["input_packet"]["byte_length"] = 262145
emit("oversized.json", oversized)
unclean = copy.deepcopy(incomplete)
unclean["judge_attempts"][0]["cleanup_confirmed"] = False
emit("unclean.json", unclean)
false_complete = copy.deepcopy(incomplete)
false_complete["status"] = "complete"
emit("false-complete.json", false_complete)
PY

bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/report.md" "$fixture_dir/complete.json"
bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/incomplete-report.md" "$fixture_dir/incomplete.json"
for rejected in missing-auth oversized; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/report.md" "$fixture_dir/$rejected.json"; then
    echo "postmortem contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done
for rejected in unclean false-complete; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/incomplete-report.md" "$fixture_dir/$rejected.json"; then
    echo "postmortem contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

echo 'postmortem skill contract: PASS'
