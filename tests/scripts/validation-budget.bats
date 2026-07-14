#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  ADAPTER="$REPO_ROOT/skills/validate/scripts/validation-budget.py"
  GOVERNOR="$REPO_ROOT/skills/rpi/scripts/run-governor.py"
  STATE_DIR="$BATS_TEST_TMPDIR/state"
  REQUEST="$BATS_TEST_TMPDIR/request.json"
  FACTUAL_RECEIPT="$BATS_TEST_TMPDIR/factual-receipt.json"
  BUDGET_RECEIPT="$BATS_TEST_TMPDIR/budget-receipt.json"
  mkdir -p "$STATE_DIR"
  write_request
  write_factual_receipt READY
}

canonical_sha256() {
  python3 - "$1" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
canonical = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
print(hashlib.sha256(canonical).hexdigest())
PY
}

init_run() {
  local run_id="$1"
  python3 "$GOVERNOR" init \
    --state-dir "$STATE_DIR" \
    --run-id "$run_id" \
    --max-reviewer-tokens 5 \
    --max-elapsed-seconds 5 \
    --max-review-contexts 5 \
    --max-deterministic-executions 5 >/dev/null
}

write_request() {
  jq -n '{
    schema_version: 1,
    request_id: "validation-budget-fixture",
    candidate: {
      semantic_id: ("a" * 64),
      delivery_sha: ("1" * 40),
      base_sha: ("2" * 40),
      tree_sha: ("3" * 40),
      subtrees: [{path: "skills/validate", tree_sha: ("4" * 40)}],
      changed_surfaces: [{path: "subject.txt", status: "M"}],
      owned_paths: [{path: "subject.txt", blob_oid: ("5" * 40)}]
    },
    acceptance: {path: "acceptance.txt", sha256: ("b" * 64)},
    evidence: [{path: "evidence.txt", sha256: ("c" * 64)}],
    claim_dependencies: [{path: "claim.txt", sha256: ("d" * 64)}],
    claim_dependency_digests: [("d" * 64)],
    gate_registry: {path: "gate-registry.json", sha256: ("e" * 64)},
    toolchain: [{path: "tool.sh", sha256: ("f" * 64)}],
    selected_gates: [{
      id: "mandatory-fact",
      lane: "mandatory",
      proof_kind: "executable_assertion",
      entry_sha256: ("0" * 64)
    }],
    author_id: "author-session",
    validator: {
      validator_id: "fresh-validator",
      fresh: true,
      route: "single_fresh"
    }
  }' >"$REQUEST"
}

write_factual_receipt() {
  local shape="$1"
  local request_sha
  request_sha="$(canonical_sha256 "$REQUEST")"

  case "$shape" in
    READY)
      jq -n --slurpfile request "$REQUEST" --arg request_sha "$request_sha" '{
        schema_version: 1,
        request_id: $request[0].request_id,
        request_sha256: $request_sha,
        candidate: $request[0].candidate,
        validator_route: "single_fresh",
        preflight_errors: [],
        gate_executions: [{
          id: "mandatory-fact",
          lane: "mandatory",
          proof_kind: "executable_assertion",
          candidate: {
            status: "PASS",
            exit_code: 0,
            output_sha256: ("6" * 64),
            facts: {checks: 1}
          },
          attribution: "not_applicable"
        }],
        model_spend_allowed: true,
        disposition: "READY",
        next_action: "VALIDATE_SINGLE_FRESH"
      }' >"$FACTUAL_RECEIPT"
      ;;
    UNKNOWN|ERROR)
      jq -n --slurpfile request "$REQUEST" --arg request_sha "$request_sha" \
        --arg factual_status "$shape" '{
        schema_version: 1,
        request_id: $request[0].request_id,
        request_sha256: $request_sha,
        candidate: $request[0].candidate,
        validator_route: "single_fresh",
        preflight_errors: [],
        gate_executions: [{
          id: "mandatory-fact",
          lane: "mandatory",
          proof_kind: "executable_assertion",
          candidate: {
            status: $factual_status,
            exit_code: 0,
            output_sha256: ("7" * 64),
            facts: {}
          },
          attribution: "not_applicable"
        }],
        model_spend_allowed: false,
        disposition: "BLOCK",
        next_action: null
      }' >"$FACTUAL_RECEIPT"
      ;;
    FAIL)
      jq -n --slurpfile request "$REQUEST" --arg request_sha "$request_sha" '{
        schema_version: 1,
        request_id: $request[0].request_id,
        request_sha256: $request_sha,
        candidate: $request[0].candidate,
        validator_route: "single_fresh",
        preflight_errors: [],
        gate_executions: [{
          id: "mandatory-fact",
          lane: "mandatory",
          proof_kind: "executable_assertion",
          candidate: {
            status: "FAIL",
            exit_code: 1,
            output_sha256: ("8" * 64),
            facts: {checks: 1}
          },
          baseline: {
            status: "PASS",
            exit_code: 0,
            output_sha256: ("9" * 64),
            facts: {checks: 1}
          },
          attribution: "candidate_introduced"
        }],
        model_spend_allowed: false,
        disposition: "REPAIR",
        next_action: "REPAIR_CANDIDATE"
      }' >"$FACTUAL_RECEIPT"
      ;;
    READY_NONBINDING_FAIL)
      jq -n --slurpfile request "$REQUEST" --arg request_sha "$request_sha" '{
        schema_version: 1,
        request_id: $request[0].request_id,
        request_sha256: $request_sha,
        candidate: $request[0].candidate,
        validator_route: "single_fresh",
        preflight_errors: [],
        gate_executions: [{
          id: "mandatory-fact",
          lane: "mandatory",
          proof_kind: "executable_assertion",
          candidate: {
            status: "PASS",
            exit_code: 0,
            output_sha256: ("6" * 64),
            facts: {checks: 1}
          },
          attribution: "not_applicable"
        }, {
          id: "diagnostic-fact",
          lane: "diagnostic",
          proof_kind: "schema",
          candidate: {
            status: "FAIL",
            exit_code: 1,
            output_sha256: ("a" * 64),
            facts: {checks: 1}
          },
          baseline: {
            status: "PASS",
            exit_code: 0,
            output_sha256: ("b" * 64),
            facts: {checks: 1}
          },
          attribution: "candidate_introduced"
        }, {
          id: "release-fact",
          lane: "release",
          proof_kind: "identity",
          candidate: {
            status: "FAIL",
            exit_code: 1,
            output_sha256: ("c" * 64),
            facts: {checks: 1}
          },
          baseline: {
            status: "PASS",
            exit_code: 0,
            output_sha256: ("d" * 64),
            facts: {checks: 1}
          },
          attribution: "candidate_introduced"
        }],
        model_spend_allowed: true,
        disposition: "READY",
        next_action: "VALIDATE_SINGLE_FRESH"
      }' >"$FACTUAL_RECEIPT"
      ;;
  esac
}

add_nonbinding_selections() {
  jq '.selected_gates += [{
    id: "diagnostic-fact",
    lane: "diagnostic",
    proof_kind: "schema",
    entry_sha256: ("1" * 64)
  }, {
    id: "release-fact",
    lane: "release",
    proof_kind: "identity",
    entry_sha256: ("2" * 64)
  }]' "$REQUEST" >"$REQUEST.tmp"
  mv "$REQUEST.tmp" "$REQUEST"
}

admit_review() {
  local run_id="$1"
  shift
  python3 "$ADAPTER" admit \
    --state-dir "$STATE_DIR" \
    --run-id "$run_id" \
    --request "$REQUEST" \
    --factual-receipt "$FACTUAL_RECEIPT" \
    --output "$BUDGET_RECEIPT" \
    --reviewer-tokens 2 \
    --elapsed-seconds 3 \
    --review-contexts 1 \
    --deterministic-executions 1 \
    "$@"
}

admit_review_missing_meter() {
  local run_id="$1" missing="$2"
  local args=(
    admit
    --state-dir "$STATE_DIR"
    --run-id "$run_id"
    --request "$REQUEST"
    --factual-receipt "$FACTUAL_RECEIPT"
    --output "$BUDGET_RECEIPT"
  )
  [ "$missing" = reviewer-tokens ] || args+=(--reviewer-tokens 2)
  [ "$missing" = elapsed-seconds ] || args+=(--elapsed-seconds 3)
  [ "$missing" = review-contexts ] || args+=(--review-contexts 1)
  [ "$missing" = deterministic-executions ] || args+=(--deterministic-executions 1)
  python3 "$ADAPTER" "${args[@]}"
}

@test "Validate records one semantic-review admission against the same persistent run before dispatch" {
  init_run same-run

  run admit_review same-run

  [ "$status" -eq 0 ]
  jq -e '
    .status == "AUTHORIZED" and
    .reason == "admitted-before-dispatch" and
    .run_id == "same-run" and
    .action == "semantic-review" and
    .validator_dispatch_allowed == true and
    .next_action == "VALIDATE_SINGLE_FRESH" and
    .governor.admission_id == "same-run:1" and
    .governor.disposition == "NOTE" and
    .governor.helper_allowed == false and
    .charge == {
      reviewer_tokens: 2,
      elapsed_seconds: 3,
      review_contexts: 1,
      deterministic_executions: 1
    }
  ' "$BUDGET_RECEIPT"
  jq -e '
    .run_id == "same-run" and
    .usage == {
      waves: 0,
      reviewer_tokens: 2,
      elapsed_seconds: 3,
      review_contexts: 1,
      deterministic_executions: 1
    } and
    (.admissions | length) == 1 and
    .admissions[0].action == "semantic-review" and
    .admissions[0].status == "recorded"
  ' "$STATE_DIR/same-run.json"
  run python3 "$ADAPTER" check-receipt --receipt "$BUDGET_RECEIPT"
  [ "$status" -eq 0 ]
}

@test "diagnostic and release FAIL stay nonbinding when the S1 S8 aggregate receipt is READY" {
  init_run nonbinding-lanes
  add_nonbinding_selections
  write_factual_receipt READY_NONBINDING_FAIL

  run admit_review nonbinding-lanes

  [ "$status" -eq 0 ]
  jq -e '
    .status == "AUTHORIZED" and
    .reason == "admitted-before-dispatch" and
    .validator_dispatch_allowed == true and
    .governor.admission_id == "nonbinding-lanes:1"
  ' "$BUDGET_RECEIPT"
  jq -e '
    (.admissions | length) == 1 and
    .admissions[0].action == "semantic-review"
  ' "$STATE_DIR/nonbinding-lanes.json"
}

@test "every genuinely spent hard ceiling yields typed nonauthorizing evidence without a helper" {
  local meter option reason
  for meter in reviewer-tokens elapsed-seconds review-contexts deterministic-executions; do
    run_id="ceiling-${meter}"
    init_run "$run_id"
    option="--${meter}"
    reason="hard-ceiling:${meter//-/_}"

    run admit_review "$run_id" "$option" 6

    [ "$status" -ne 0 ]
    jq -e --arg run_id "$run_id" --arg reason "$reason" '
      .status == "NONAUTHORIZING" and
      .run_id == $run_id and
      .reason == $reason and
      .validator_dispatch_allowed == false and
      .next_action == null and
      .governor.admission_id == null and
      .governor.disposition == "ANDON" and
      .governor.helper_allowed == false
    ' "$BUDGET_RECEIPT"
    jq -e '(.admissions | length) == 0 and (.usage | [.[]] | add) == 0' \
      "$STATE_DIR/$run_id.json"
    rm "$BUDGET_RECEIPT"
  done
}

@test "an unavailable required meter is typed nonauthorizing evidence and does not call the governor" {
  local meter
  for meter in reviewer-tokens elapsed-seconds review-contexts deterministic-executions; do
    run_id="missing-${meter}"
    init_run "$run_id"

    run admit_review_missing_meter "$run_id" "$meter"

    [ "$status" -ne 0 ]
    jq -e --arg reason "missing-meter:${meter//-/_}" '
      .status == "NONAUTHORIZING" and
      .reason == $reason and
      .validator_dispatch_allowed == false and
      .next_action == null and
      .governor == null
    ' "$BUDGET_RECEIPT"
    jq -e '(.admissions | length) == 0 and (.usage | [.[]] | add) == 0' \
      "$STATE_DIR/$run_id.json"
    rm "$BUDGET_RECEIPT"
  done
}

@test "mandatory FAIL ERROR UNKNOWN and missing proof cannot reach budget admission or become WARN or PASS" {
  local shape
  for shape in FAIL UNKNOWN ERROR; do
    run_id="proof-$(printf '%s' "$shape" | tr '[:upper:]' '[:lower:]')"
    init_run "$run_id"
    write_factual_receipt "$shape"

    run admit_review "$run_id"

    [ "$status" -ne 0 ]
    jq -e --arg reason "factual-proof-not-ready:${shape}" '
      .status == "NONAUTHORIZING" and
      .reason == $reason and
      .validator_dispatch_allowed == false and
      .next_action == null and
      .governor == null and
      (.status != "WARN" and .status != "PASS")
    ' "$BUDGET_RECEIPT"
    jq -e '(.admissions | length) == 0' "$STATE_DIR/$run_id.json"
    rm "$BUDGET_RECEIPT"
  done

  init_run proof-missing
  write_factual_receipt READY
  jq 'del(.gate_executions)' "$FACTUAL_RECEIPT" >"$FACTUAL_RECEIPT.tmp"
  mv "$FACTUAL_RECEIPT.tmp" "$FACTUAL_RECEIPT"

  run admit_review proof-missing

  [ "$status" -ne 0 ]
  jq -e '
    .status == "NONAUTHORIZING" and
    .reason == "invalid-factual-proof" and
    .validator_dispatch_allowed == false and
    .next_action == null and
    .governor == null
  ' "$BUDGET_RECEIPT"
  jq -e '(.admissions | length) == 0' "$STATE_DIR/proof-missing.json"
}

@test "absent and invalid-JSON factual inputs emit schema-valid typed nonauthorizing evidence" {
  init_run absent-proof
  rm "$FACTUAL_RECEIPT"

  run admit_review absent-proof

  [ "$status" -ne 0 ]
  [ -s "$BUDGET_RECEIPT" ]
  jq -e '
    .status == "NONAUTHORIZING" and
    .reason == "missing-factual-proof" and
    .factual_receipt_availability == "absent" and
    .factual_receipt_sha256 == null and
    .validator_dispatch_allowed == false and
    .governor == null
  ' "$BUDGET_RECEIPT"
  run python3 "$ADAPTER" check-receipt --receipt "$BUDGET_RECEIPT"
  [ "$status" -eq 0 ]
  jq -e '(.admissions | length) == 0' "$STATE_DIR/absent-proof.json"

  rm "$BUDGET_RECEIPT"
  init_run invalid-json-proof
  printf '{broken\n' >"$FACTUAL_RECEIPT"

  run admit_review invalid-json-proof

  [ "$status" -ne 0 ]
  [ -s "$BUDGET_RECEIPT" ]
  jq -e '
    .status == "NONAUTHORIZING" and
    .reason == "invalid-factual-proof-json" and
    .factual_receipt_availability == "invalid_json" and
    (.factual_receipt_sha256 | length) == 64 and
    .validator_dispatch_allowed == false and
    .governor == null
  ' "$BUDGET_RECEIPT"
  run python3 "$ADAPTER" check-receipt --receipt "$BUDGET_RECEIPT"
  [ "$status" -eq 0 ]
  jq -e '(.admissions | length) == 0' "$STATE_DIR/invalid-json-proof.json"
}

@test "a mismatched or missing run identity cannot consume another run" {
  init_run intended-run

  run admit_review other-run

  [ "$status" -ne 0 ]
  jq -e '
    .status == "NONAUTHORIZING" and
    .run_id == "other-run" and
    .reason == "missing-state" and
    .validator_dispatch_allowed == false and
    .governor.disposition == "NOTE"
  ' "$BUDGET_RECEIPT"
  jq -e '(.admissions | length) == 0' "$STATE_DIR/intended-run.json"
}

@test "the Validate adapter owns no private retry budget helper or escalation machine" {
  run rg -n -i \
    'attempt[_ -]?count|retry[_ -]?count|phase[_ -]?budget|helper_history|command_helper|command_break|human-authority' \
    "$REPO_ROOT/skills/validate/scripts/validation-budget.py" \
    "$REPO_ROOT/schemas/validation-budget-receipt.v1.schema.json"

  [ "$status" -eq 1 ]
}
