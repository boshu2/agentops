#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skill="$skill_dir/SKILL.md"
grep -q '^name: idea-genie$' "$skill"
grep -q '^## Constraints$' "$skill"
grep -Fq 'at most 20 sources' "$skill"
grep -Fq 'Every adapter and model identity is named before' "$skill"
grep -Fq 'terminates/reaps the group' "$skill"
grep -Fq 'explicit `insufficient` result' "$skill"
grep -Fq '`run-check` bounded runner' "$skill"
test -x "$skill_dir/scripts/validate-output.sh"
test -x "$skill_dir/scripts/validate-challenge.sh"

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
python3 - "$fixture_dir" <<'PY'
import copy
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
digest = "b" * 64
input_packet = {"source_refs": ["question.md"], "byte_length": 256, "sha256": digest}
portfolio = {
    "schema_version": "idea-portfolio.v1",
    "authorization_id": "caller:idea-fixture",
    "input_packet": input_packet,
    "limits": {"max_passes": 3, "max_candidates": 10, "max_output_bytes": 65536},
    "effects": {"network_requests": 0, "bytes_fetched": 0, "credentials_used": [], "writes": ["idea-portfolio.json"]},
    "status": "candidates",
    "observations": [{"claim": "The bounded problem recurs.", "evidence": "question.md#evidence"}],
    "assumptions": ["The cited observation remains current."],
    "candidates": [{
        "id": "candidate-a",
        "evidence": ["question.md#evidence"],
        "overlaps": [],
        "scenario": {"given": "a bounded request", "when": "the candidate is applied", "then": "the cited need is addressed"},
    }],
    "termination": {"reason": "novelty-saturated", "passes_run": 2, "novel_candidates_last_pass": 0},
}

def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")

emit("portfolio.json", portfolio)
missing_auth = copy.deepcopy(portfolio)
missing_auth["authorization_id"] = ""
emit("portfolio-missing-auth.json", missing_auth)
oversized = copy.deepcopy(portfolio)
oversized["input_packet"]["byte_length"] = 262145
emit("portfolio-oversized.json", oversized)
not_saturated = copy.deepcopy(portfolio)
not_saturated["termination"]["novel_candidates_last_pass"] = 1
emit("portfolio-not-saturated.json", not_saturated)

attempts = [
    {"context_id": "judge-a", "adapter": "fixture", "model_identity": "model-a", "status": "completed", "output_bytes": 128, "cleanup_confirmed": True},
    {"context_id": "judge-b", "adapter": "fixture", "model_identity": "model-b", "status": "completed", "output_bytes": 128, "cleanup_confirmed": True},
]
challenge = {
    "schema_version": "idea-challenge.v1",
    "authorization_id": "caller:challenge-fixture",
    "input_packet": input_packet,
    "limits": {"judge_count": 2, "per_judge_timeout_seconds": 30, "round_timeout_seconds": 90, "max_judge_output_bytes": 4096},
    "door_class": "one-way",
    "sealed_generation": True,
    "round_status": "sufficient",
    "judge_attempts": attempts,
    "perspectives": [
        {"id": "option-a", "context_id": "judge-a", "model_identity": "model-a", "proposal": "Use the bounded path.", "evidence": ["question.md#bounds"]},
        {"id": "option-b", "context_id": "judge-b", "model_identity": "model-b", "proposal": "Keep the status quo.", "evidence": ["question.md#baseline"]},
    ],
    "cross_reviews": [{"reviewer": "option-a", "subject": "option-b", "dimensions": {"risk": "The baseline leaves the cited failure."}}],
    "disagreements": ["Whether change risk exceeds status-quo risk."],
    "refutations": [{"claim": "The change is unnecessary.", "attempt": "Replay the cited failure.", "result": "The failure remains."}],
    "handoff": {"owner": "plan", "artifact_dir": ".agents/scratch/ideas/fixture", "route": "sealed-challenge"},
    "cleanup_verified": True,
}
emit("challenge.json", challenge)

timed_out = copy.deepcopy(challenge)
timed_out.update({
    "round_status": "insufficient", "perspectives": [], "cross_reviews": [],
    "disagreements": [], "refutations": [],
    "handoff": {"owner": "plan", "artifact_dir": ".agents/scratch/ideas/fixture", "route": "stop-insufficient"},
})
for attempt in timed_out["judge_attempts"]:
    attempt.update({"status": "timed_out", "output_bytes": 0, "error": "deadline exceeded"})
emit("challenge-timed-out.json", timed_out)

two_way = copy.deepcopy(challenge)
two_way.update({
    "door_class": "two-way", "sealed_generation": False,
    "limits": {"judge_count": 1, "per_judge_timeout_seconds": 30, "round_timeout_seconds": 90, "max_judge_output_bytes": 4096},
    "judge_attempts": [attempts[0]], "perspectives": [], "cross_reviews": [],
    "disagreements": [], "refutations": [], "requires_ntm": False,
    "lightweight_challenge": "One fresh context found no bounded blocker.",
    "handoff": {"owner": "plan", "artifact_dir": ".agents/scratch/ideas/fixture", "route": "single-fresh-context"},
})
emit("challenge-two-way.json", two_way)

challenge_missing_auth = copy.deepcopy(challenge)
challenge_missing_auth["authorization_id"] = ""
emit("challenge-missing-auth.json", challenge_missing_auth)
challenge_oversized = copy.deepcopy(challenge)
challenge_oversized["input_packet"]["byte_length"] = 262145
emit("challenge-oversized.json", challenge_oversized)
unclean = copy.deepcopy(timed_out)
unclean["judge_attempts"][0]["cleanup_confirmed"] = False
emit("challenge-unclean.json", unclean)
false_synthesis = copy.deepcopy(timed_out)
false_synthesis["disagreements"] = ["Invented after timeout."]
emit("challenge-false-synthesis.json", false_synthesis)
PY

bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/portfolio.json"
for rejected in portfolio-missing-auth portfolio-oversized portfolio-not-saturated; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "idea-genie accepted negative portfolio fixture: $rejected" >&2
    exit 1
  fi
done

for accepted in challenge challenge-timed-out challenge-two-way; do
  bash "$skill_dir/scripts/validate-challenge.sh" "$fixture_dir/$accepted.json"
done
for rejected in challenge-missing-auth challenge-oversized challenge-unclean challenge-false-synthesis; do
  if bash "$skill_dir/scripts/validate-challenge.sh" "$fixture_dir/$rejected.json"; then
    echo "idea-genie accepted negative challenge fixture: $rejected" >&2
    exit 1
  fi
done

echo 'idea-genie skill contract: PASS'
