#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-agents-operating-contract-verdict.py"
  TEST_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$TEST_REPO/docs/contracts" "$TEST_REPO/schemas"
  cp "$REPO_ROOT/schemas/agents-operating-contract-verdict.v1.schema.json" "$TEST_REPO/schemas/"
  printf 'contract\n' >"$TEST_REPO/docs/contracts/agents-operating-contract.md"
  printf 'operating contract passage\n' >"$TEST_REPO/AGENTS.md"
  git -C "$TEST_REPO" init -q
  git -C "$TEST_REPO" config user.email test@example.com
  git -C "$TEST_REPO" config user.name test
  git -C "$TEST_REPO" add .
  git -C "$TEST_REPO" commit -qm fixture
  HEAD_SHA="$(git -C "$TEST_REPO" rev-parse HEAD)"
  CANDIDATE_SHA="$(shasum -a 256 "$TEST_REPO/AGENTS.md" | awk '{print $1}')"
  CONTRACT_SHA="$(shasum -a 256 "$TEST_REPO/docs/contracts/agents-operating-contract.md" | awk '{print $1}')"
  write_verdict
}

write_verdict() {
  TEST_REPO="$TEST_REPO" python3 - <<'PY'
import hashlib, json, os, subprocess
from pathlib import Path
repo = Path(os.environ["TEST_REPO"])
head = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
candidate = repo / "AGENTS.md"
ids = ["authority", "trust-boundary", "law0-runtime", "precedence", "ordered-loop-repair", "exact-done", "concurrency", "triggered-routes", "closeout"]
result = {
  "schema_version": "agents-operating-contract-verdict.v1",
  "contract_path": "docs/contracts/agents-operating-contract.md",
  "contract_sha256": hashlib.sha256((repo / "docs/contracts/agents-operating-contract.md").read_bytes()).hexdigest(),
  "candidate_path": "AGENTS.md",
  "pinned_head": head,
  "candidate_sha256": hashlib.sha256(candidate.read_bytes()).hexdigest(),
  "author_session": "author-1",
  "judge_session": "judge-1",
  "judge_family": "codex",
  "verdict": "PASS",
  "scenario_results": [
    {"scenario_id": item, "verdict": "PASS", "citations": [{"path": "AGENTS.md", "line_start": 1, "line_end": 1, "quote": "operating contract passage"}], "material_decision": "bounded fixture decision", "rationale": "fixture judgment"}
    for item in ids
  ],
  "findings": [],
  "not_checked": [],
  "commands": [{"command": "factual fixture check", "exit_code": 0}],
  "validated_at": "2026-07-12T13:00:00Z"
}
(repo / "verdict.json").write_text(json.dumps(result, indent=2) + "\n")
PY
}

run_checker() {
  run python3 "$CHECKER" \
    --root="$TEST_REPO" \
    --verdict=verdict.json \
    --candidate=AGENTS.md \
    --contract=docs/contracts/agents-operating-contract.md \
    --author-session=author-1 \
    --expected-judge-session=judge-1 \
    --expected-head="$HEAD_SHA" \
    --expected-candidate-sha256="$CANDIDATE_SHA" \
    --expected-contract-sha256="$CONTRACT_SHA"
}

@test "accepts a schema-valid independent verdict pinned to exact candidate bytes" {
  run_checker
  [ "$status" -eq 0 ]
  [[ "$output" == *"FACTS_VALID judge=judge-1 candidate=AGENTS.md"* ]]
}

@test "rejects author self-judgment" {
  jq '.judge_session = .author_session' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"judge_session must differ from author_session"* ]]
}

@test "rejects a self-asserted judge identity not supplied by dispatch" {
  jq '.judge_session = "invented-judge"' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"judge_session differs from trusted dispatch identity"* ]]
}

@test "rejects a verdict after candidate bytes change" {
  printf 'changed\n' >>"$TEST_REPO/AGENTS.md"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"candidate_sha256 differs from candidate bytes"* ]]
}

@test "rejects a verdict after behavior-contract bytes change" {
  printf 'changed\n' >>"$TEST_REPO/docs/contracts/agents-operating-contract.md"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"contract_sha256 differs from contract bytes"* ]]
}

@test "rejects a citation quote not present at the pinned lines" {
  jq '.scenario_results[0].citations[0].quote = "invented passage"' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"authority: citation quote differs from candidate lines"* ]]
}

@test "rejects duplicate scenario IDs even when the array has nine items" {
  jq '.scenario_results[8].scenario_id = "authority"' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"scenario_results contains duplicate scenario IDs"* ]]
  [[ "$output" == *"scenario_results missing: closeout"* ]]
}

@test "rejects PASS when structured facts require WARN" {
  jq '.not_checked = ["link rendering"]' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"aggregate verdict PASS must be WARN"* ]]
}


prepare_pair() {
  cp "$TEST_REPO/verdict.json" "$TEST_REPO/verdict-a.json"
  jq '.judge_session = "judge-2"' "$TEST_REPO/verdict.json" >"$TEST_REPO/verdict-b.json"
}

run_reconciler() {
  run python3 "$REPO_ROOT/scripts/reconcile-agents-operating-contract-verdicts.py" \
    --root="$TEST_REPO" \
    --verdict-a=verdict-a.json \
    --verdict-b=verdict-b.json \
    --candidate=AGENTS.md \
    --contract=docs/contracts/agents-operating-contract.md \
    --expected-head="$HEAD_SHA" \
    --expected-candidate-sha256="$CANDIDATE_SHA" \
    --expected-contract-sha256="$CONTRACT_SHA" \
    --author-session=author-1 \
    --judge-a-session=judge-1 \
    --judge-b-session=judge-2 \
    "$@"
}

@test "reconciliation accepts two distinct dispatch-bound PASS verdicts" {
  prepare_pair
  run_reconciler
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS judges=judge-1,judge-2 candidate=AGENTS.md"* ]]
}

@test "reconciliation rejects one duplicated verdict artifact" {
  prepare_pair
  run python3 "$REPO_ROOT/scripts/reconcile-agents-operating-contract-verdicts.py" \
    --root="$TEST_REPO" --verdict-a=verdict-a.json --verdict-b=verdict-a.json \
    --candidate=AGENTS.md --contract=docs/contracts/agents-operating-contract.md \
    --expected-head="$HEAD_SHA" --expected-candidate-sha256="$CANDIDATE_SHA" \
    --expected-contract-sha256="$CONTRACT_SHA" --author-session=author-1 \
    --judge-a-session=judge-1 --judge-b-session=judge-2
  [ "$status" -eq 1 ]
  [[ "$output" == *"verdict artifacts must be distinct paths"* ]]
}

@test "reconciliation rejects duplicate trusted or claimed judge identity" {
  prepare_pair
  jq '.judge_session = "judge-1"' "$TEST_REPO/verdict-b.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict-b.json"
  run python3 "$REPO_ROOT/scripts/reconcile-agents-operating-contract-verdicts.py" \
    --root="$TEST_REPO" --verdict-a=verdict-a.json --verdict-b=verdict-b.json \
    --candidate=AGENTS.md --contract=docs/contracts/agents-operating-contract.md \
    --expected-head="$HEAD_SHA" --expected-candidate-sha256="$CANDIDATE_SHA" \
    --expected-contract-sha256="$CONTRACT_SHA" --author-session=author-1 \
    --judge-a-session=judge-1 --judge-b-session=judge-1
  [ "$status" -eq 1 ]
  [[ "$output" == *"trusted judge session identities must be distinct"* ]]
  [[ "$output" == *"verdicts claim the same judge_session"* ]]
}

@test "reconciliation rejects cross-verdict artifact identity mismatch" {
  prepare_pair
  jq '.candidate_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' "$TEST_REPO/verdict-b.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict-b.json"
  run_reconciler
  [ "$status" -eq 1 ]
  [[ "$output" == *"verdict artifact identity mismatch: candidate_sha256"* ]]
}

@test "reconciliation rejects one PASS and one factual FAIL" {
  prepare_pair
  jq '.scenario_results[0].verdict = "FAIL" | .verdict = "FAIL"' "$TEST_REPO/verdict-b.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict-b.json"
  run_reconciler
  [ "$status" -eq 1 ]
  [[ "$output" == *"two independent PASS verdicts required: A=PASS B=FAIL"* ]]
}

@test "reconciliation rejects one PASS and one WARN" {
  prepare_pair
  jq '.scenario_results[0].verdict = "WARN" | .verdict = "WARN"' "$TEST_REPO/verdict-b.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict-b.json"
  run_reconciler
  [ "$status" -eq 1 ]
  [[ "$output" == *"two independent PASS verdicts required: A=PASS B=WARN"* ]]
}

@test "reconciliation rejects a missing second verdict" {
  prepare_pair
  rm "$TEST_REPO/verdict-b.json"
  run_reconciler
  [ "$status" -eq 1 ]
  [[ "$output" == *"verdict B failed factual validation"* ]]
}

@test "schema rejects findings that do not join to a contract scenario" {
  jq '.findings = [{"severity":"blocking","scenario_id":"invented","description":"bad join"}] | .verdict = "FAIL"' "$TEST_REPO/verdict.json" >"$TEST_REPO/changed.json"
  mv "$TEST_REPO/changed.json" "$TEST_REPO/verdict.json"
  run_checker
  [ "$status" -eq 1 ]
  [[ "$output" == *"schema findings.0.scenario_id"* ]]
}
