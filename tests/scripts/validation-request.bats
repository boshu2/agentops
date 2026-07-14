#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/skills/validate/scripts/validation-request.py"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  SPEC="$BATS_TEST_TMPDIR/spec.json"
  REQUEST="$BATS_TEST_TMPDIR/request.json"
  REQUEST_TWO="$BATS_TEST_TMPDIR/request-two.json"
  RECEIPT="$BATS_TEST_TMPDIR/receipt.json"
  RECEIPT_TWO="$BATS_TEST_TMPDIR/receipt-two.json"
  BAD_RECEIPT="$BATS_TEST_TMPDIR/bad-receipt.json"
  COUNT_FILE="$BATS_TEST_TMPDIR/gate-count.log"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

seed_repo() {
  local base_status="$1" candidate_status="$2" backing_mode="${3:-present}"
  mkdir -p "$FIXTURE_ROOT/scripts"
  git -C "$FIXTURE_ROOT" init -q
  git -C "$FIXTURE_ROOT" config user.name test
  git -C "$FIXTURE_ROOT" config user.email test@example.com

  cat >"$FIXTURE_ROOT/scripts/factual-gate.sh" <<'GATE'
#!/usr/bin/env bash
set -u
count_file="$1"
if [[ "${2:-}" == "--json" ]]; then
  status_file="gate-status.txt"
  json_flag="$2"
else
  status_file="${2:-gate-status.txt}"
  json_flag="${3:-}"
fi
[[ "$json_flag" == "--json" ]] || exit 2
status="$(tr -d '\n' < "$status_file")"
printf '%s %s\n' "$(git rev-parse HEAD)" "$status" >>"$count_file"
if [[ -n "${VALIDATION_GATE_DELAY:-}" ]]; then
  sleep "$VALIDATION_GATE_DELAY"
fi
case "$status" in
  PASS) printf '%s\n' '{"status":"PASS","facts":{"checks":1}}'; exit 0 ;;
  FAIL) printf '%s\n' '{"status":"FAIL","facts":{"checks":1}}'; exit 1 ;;
  UNKNOWN) printf '%s\n' '{"status":"UNKNOWN","facts":{"checks":1}}'; exit 0 ;;
  ERROR) printf '%s\n' 'not-json'; exit 0 ;;
esac
GATE
  chmod +x "$FIXTURE_ROOT/scripts/factual-gate.sh"
  printf '%s\n' "$base_status" >"$FIXTURE_ROOT/gate-status.txt"
  printf '%s\n' 'accept candidate behavior' >"$FIXTURE_ROOT/acceptance.txt"
  printf '%s\n' 'author evidence' >"$FIXTURE_ROOT/evidence.txt"
  printf '%s\n' 'scoped S1 claim' >"$FIXTURE_ROOT/claim.txt"
  printf '%s\n' 'PASS' >"$FIXTURE_ROOT/diagnostic-status.txt"
  printf '%s\n' 'PASS' >"$FIXTURE_ROOT/release-status.txt"
  printf '%s\n' 'delete me' >"$FIXTURE_ROOT/obsolete.txt"
  printf '%s\n' 'base' >"$FIXTURE_ROOT/subject.txt"

  local backing_path backing_sha
  if [[ "$backing_mode" == "missing" ]]; then
    backing_path="scripts/missing-gate.sh"
    backing_sha="0000000000000000000000000000000000000000000000000000000000000000"
  else
    backing_path="scripts/factual-gate.sh"
    backing_sha="$(sha256_file "$FIXTURE_ROOT/scripts/factual-gate.sh")"
  fi
  jq -n \
    --arg count "$COUNT_FILE" \
    --arg backing_path "$backing_path" \
    --arg backing_sha "$backing_sha" \
    '{
      schema_version: 1,
      gates: [{
        id: "fact",
        lane: "mandatory",
        proof_kind: "executable_assertion",
        argv: ["bash", "scripts/factual-gate.sh", $count, "--json"],
        backing: [{path: $backing_path, sha256: $backing_sha}]
      }]
    }' >"$FIXTURE_ROOT/gate-registry.json"

  git -C "$FIXTURE_ROOT" add .
  git -C "$FIXTURE_ROOT" commit -q -m base
  BASE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"

  printf '%s\n' 'candidate' >"$FIXTURE_ROOT/subject.txt"
  printf '%s\n' "$candidate_status" >"$FIXTURE_ROOT/gate-status.txt"
  git -C "$FIXTURE_ROOT" add .
  git -C "$FIXTURE_ROOT" commit -q -m candidate
  CANDIDATE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"
  write_spec
}

write_spec() {
  local owned_paths
  owned_paths="$(git -C "$FIXTURE_ROOT" diff --name-only "$BASE_SHA..$CANDIDATE_SHA" |
    jq -Rsc 'split("\n") | map(select(length > 0))')"
  jq -n \
    --arg request_id "request-${BATS_TEST_NUMBER:-0}" \
    --arg base "$BASE_SHA" \
    --arg candidate "$CANDIDATE_SHA" \
    --argjson owned_paths "$owned_paths" \
    '{
      schema_version: 1,
      request_id: $request_id,
      base_sha: $base,
      candidate_sha: $candidate,
      subtree_paths: ["scripts"],
      owned_paths: $owned_paths,
      acceptance_path: "acceptance.txt",
      evidence_paths: ["evidence.txt"],
      claim_dependency_paths: ["claim.txt"],
      gate_registry_path: "gate-registry.json",
      toolchain_paths: ["scripts/factual-gate.sh"],
      selected_gate_ids: ["fact"],
      author_id: "author-session",
      validator: {
        validator_id: "fresh-validator",
        fresh: true,
        route: "single_fresh"
      }
    }' >"$SPEC"
}

freeze_request() {
  run python3 "$CHECKER" freeze \
    --repo "$FIXTURE_ROOT" --spec "$SPEC" --output "$REQUEST"
}

run_request() {
  run python3 "$CHECKER" run \
    --repo "$FIXTURE_ROOT" --request "$REQUEST" --output "$RECEIPT"
}

use_auditable_claim_dependencies() {
  jq 'del(.claim_dependency_digests) | .claim_dependency_paths = ["claim.txt"]' \
    "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"
}

add_nonbinding_gate() {
  local lane="$1" gate_id="$2" status_file="$3" gate_status="${4:-FAIL}"
  local backing_sha
  backing_sha="$(sha256_file "$FIXTURE_ROOT/scripts/factual-gate.sh")"
  printf '%s\n' "$gate_status" >"$FIXTURE_ROOT/$status_file"
  jq --arg count "$COUNT_FILE" --arg lane "$lane" --arg gate_id "$gate_id" \
    --arg status_file "$status_file" --arg backing_sha "$backing_sha" '
    .gates += [{
      id: $gate_id,
      lane: $lane,
      proof_kind: "executable_assertion",
      argv: ["bash", "scripts/factual-gate.sh", $count, $status_file, "--json"],
      backing: [{path: "scripts/factual-gate.sh", sha256: $backing_sha}]
    }]
  ' "$FIXTURE_ROOT/gate-registry.json" >"$FIXTURE_ROOT/gate-registry.json.tmp"
  mv "$FIXTURE_ROOT/gate-registry.json.tmp" "$FIXTURE_ROOT/gate-registry.json"
  git -C "$FIXTURE_ROOT" add gate-registry.json "$status_file"
  git -C "$FIXTURE_ROOT" commit -q -m "$lane-gate"
  CANDIDATE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"
  write_spec
  jq --arg gate_id "$gate_id" '.selected_gate_ids += [$gate_id]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"
}

check_bad_receipt() {
  run python3 "$CHECKER" check-receipt --request "$REQUEST" --receipt "$BAD_RECEIPT"
}

@test "freezes exact identities and routes mandatory green to one fresh validator" {
  seed_repo PASS PASS

  freeze_request
  [ "$status" -eq 0 ]
  jq -e --arg base "$BASE_SHA" --arg candidate "$CANDIDATE_SHA" '
    .candidate.base_sha == $base and
    .candidate.delivery_sha == $candidate and
    (.candidate.tree_sha | length) == 40 and
    (.candidate.semantic_id | length) == 64 and
    (.candidate.changed_surfaces | length) >= 1 and
    .validator.route == "single_fresh" and
    .selected_gates[0].proof_kind == "executable_assertion" and
    (.validator | has("inventory_count") | not)
  ' "$REQUEST"

  run_request
  [ "$status" -eq 0 ]
  jq -e '
    .model_spend_allowed == true and
    .disposition == "READY" and
    .next_action == "VALIDATE_SINGLE_FRESH" and
    (.gate_executions | length) == 1 and
    .gate_executions[0].proof_kind == "executable_assertion" and
    .gate_executions[0].candidate.status == "PASS"
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 1 ]
  run python3 "$CHECKER" check-receipt --request "$REQUEST" --receipt "$RECEIPT"
  [ "$status" -eq 0 ]
  common_dir="$(git -C "$FIXTURE_ROOT" rev-parse --path-format=absolute --git-common-dir)"
  claim_file="$(printf '%s\n' "$common_dir"/agentops-validation-runs/*.json)"
  receipt_digest="$(sha256_file "$RECEIPT")"
  jq -e --arg receipt_digest "$receipt_digest" '
    .state == "COMPLETED" and
    .disposition == "READY" and
    .receipt_sha256 == $receipt_digest
  ' "$claim_file"
}

@test "executes every selected factual gate exactly once" {
  seed_repo PASS PASS
  backing_sha="$(sha256_file "$FIXTURE_ROOT/scripts/factual-gate.sh")"
  jq --arg count "$COUNT_FILE" --arg backing_sha "$backing_sha" '
    .gates += [{
      id: "fact-two",
      lane: "mandatory",
      proof_kind: "schema",
      argv: ["bash", "scripts/factual-gate.sh", $count, "--json"],
      backing: [{path: "scripts/factual-gate.sh", sha256: $backing_sha}]
    }]
  ' "$FIXTURE_ROOT/gate-registry.json" >"$FIXTURE_ROOT/gate-registry.json.tmp"
  mv "$FIXTURE_ROOT/gate-registry.json.tmp" "$FIXTURE_ROOT/gate-registry.json"
  git -C "$FIXTURE_ROOT" add gate-registry.json
  git -C "$FIXTURE_ROOT" commit -q -m second-gate
  CANDIDATE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"
  write_spec
  jq '.selected_gate_ids = ["fact", "fact-two"]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request
  [ "$status" -eq 0 ]
  run_request

  [ "$status" -eq 0 ]
  jq -e '
    (.gate_executions | map(.id)) == ["fact", "fact-two"] and
    (.gate_executions | map(.proof_kind)) == ["executable_assertion", "schema"]
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 2 ]
}

@test "rejects inventory-count validator routing" {
  seed_repo PASS PASS
  jq '.validator.inventory_count = 2' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"invalid_spec"* ]]
  [ ! -e "$REQUEST" ]
  [ ! -s "$COUNT_FILE" ]
}

@test "rejects a wrong explicit base before freezing" {
  seed_repo PASS PASS
  empty_tree="$(git -C "$FIXTURE_ROOT" mktree </dev/null)"
  wrong_base="$(printf '%s\n' wrong | git -C "$FIXTURE_ROOT" commit-tree "$empty_tree")"
  jq --arg base "$wrong_base" '.base_sha = $base' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"base_not_ancestor"* ]]
  [ ! -e "$REQUEST" ]
}

@test "rejects post-freeze mutation before factual execution" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  printf '%s\n' mutation >>"$FIXTURE_ROOT/evidence.txt"

  run_request

  [ "$status" -eq 1 ]
  jq -e '.model_spend_allowed == false and .preflight_errors[0].code == "candidate_mutated"' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "stops on missing registry backing before factual execution" {
  seed_repo PASS PASS missing
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 1 ]
  jq -e '
    .model_spend_allowed == false and
    .preflight_errors[0].code == "missing_registry_backing" and
    .preflight_errors[0].defect_class == "registry_integrity"
  ' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "stops on missing evidence before factual execution" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  jq '.evidence += [{"path":"missing-evidence.txt","sha256":"0000000000000000000000000000000000000000000000000000000000000000"}]' \
    "$REQUEST" >"$REQUEST.tmp"
  mv "$REQUEST.tmp" "$REQUEST"

  run_request

  [ "$status" -eq 1 ]
  jq -e '.model_spend_allowed == false and .preflight_errors[0].code == "missing_evidence"' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "refuses a duplicate run without executing the gate twice" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 2 ]
  [[ "$output" == *"duplicate_run"* ]]
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 1 ]
}

@test "candidate-introduced mandatory red is attributed before REPAIR" {
  seed_repo PASS FAIL
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 1 ]
  jq -e '
    .model_spend_allowed == false and
    .disposition == "REPAIR" and
    .gate_executions[0].candidate.status == "FAIL" and
    .gate_executions[0].baseline.status == "PASS" and
    .gate_executions[0].attribution == "candidate_introduced"
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 2 ]
}

@test "pre-existing mandatory red stays blocked but is not REPAIR" {
  seed_repo FAIL FAIL
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 1 ]
  jq -e '
    .model_spend_allowed == false and
    .disposition == "NOTE" and
    .gate_executions[0].candidate.status == "FAIL" and
    .gate_executions[0].baseline.status == "FAIL" and
    .gate_executions[0].attribution == "pre_existing"
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 2 ]
}

@test "UNKNOWN and malformed factual output stop before model spend" {
  seed_repo PASS UNKNOWN
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 1 ]
  jq -e '.model_spend_allowed == false and .disposition == "BLOCK" and .gate_executions[0].candidate.status == "UNKNOWN"' "$RECEIPT"

  rm -f "$RECEIPT" "$COUNT_FILE"
  printf '%s\n' ERROR >"$FIXTURE_ROOT/gate-status.txt"
  git -C "$FIXTURE_ROOT" add gate-status.txt
  git -C "$FIXTURE_ROOT" commit -q -m error-candidate
  CANDIDATE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"
  write_spec
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 1 ]
  jq -e '.model_spend_allowed == false and .disposition == "BLOCK" and .gate_executions[0].candidate.status == "ERROR"' "$RECEIPT"
}

@test "freezes auditable claim dependency references and stops on a missing claim" {
  seed_repo PASS PASS
  use_auditable_claim_dependencies
  freeze_request
  [ "$status" -eq 0 ]
  jq -e '
    .claim_dependencies[0].path == "claim.txt" and
    .claim_dependencies[0].sha256 == .claim_dependency_digests[0] and
    (.claim_dependency_digests | length) == 1
  ' "$REQUEST"
  jq '
    .claim_dependencies += [{
      path: "missing-claim.txt",
      sha256: "0000000000000000000000000000000000000000000000000000000000000000"
    }] |
    .claim_dependency_digests += ["0000000000000000000000000000000000000000000000000000000000000000"]
  ' "$REQUEST" >"$REQUEST.tmp"
  mv "$REQUEST.tmp" "$REQUEST"

  run_request

  [ "$status" -eq 1 ]
  jq -e '.preflight_errors[0].code == "missing_claim_dependency" and .model_spend_allowed == false' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "stale claim dependency identity stops before factual execution" {
  seed_repo PASS PASS
  use_auditable_claim_dependencies
  freeze_request
  [ "$status" -eq 0 ]
  jq '.claim_dependencies[0].sha256 = "0000000000000000000000000000000000000000000000000000000000000000"' \
    "$REQUEST" >"$REQUEST.tmp"
  mv "$REQUEST.tmp" "$REQUEST"

  run_request

  [ "$status" -eq 1 ]
  jq -e '.preflight_errors[0].code == "stale_claim_dependency" and .model_spend_allowed == false' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "freeze rejects legacy opaque claim digests without references" {
  seed_repo PASS PASS
  jq '
    del(.claim_dependency_paths) |
    .claim_dependency_digests = ["1111111111111111111111111111111111111111111111111111111111111111"]
  ' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"invalid_spec"* ]]
  [ ! -e "$REQUEST" ]
}

@test "concurrent runs atomically admit one candidate execution" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]

  run bash -c '
    set +e
    VALIDATION_GATE_DELAY=0.3 python3 "$1" run --repo "$2" --request "$3" --output "$4" >"$5/one.log" 2>&1 &
    first=$!
    VALIDATION_GATE_DELAY=0.3 python3 "$1" run --repo "$2" --request "$3" --output "$4" >"$5/two.log" 2>&1 &
    second=$!
    wait "$first"; first_status=$?
    wait "$second"; second_status=$?
    [[ "$first_status:$second_status" == "0:2" || "$first_status:$second_status" == "2:0" ]]
  ' _ "$CHECKER" "$FIXTURE_ROOT" "$REQUEST" "$RECEIPT" "$BATS_TEST_TMPDIR"

  [ "$status" -eq 0 ]
  jq -e '.disposition == "READY"' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 1 ]
}

@test "same frozen request cannot execute again through a different output" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]

  run python3 "$CHECKER" run \
    --repo "$FIXTURE_ROOT" --request "$REQUEST" --output "$RECEIPT_TWO"

  [ "$status" -eq 2 ]
  [[ "$output" == *"duplicate_run"* ]]
  [ ! -e "$RECEIPT_TWO" ]
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 1 ]
}

@test "request reserialization cannot bypass canonical admission" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq -cS . "$REQUEST" >"$REQUEST_TWO"

  run python3 "$CHECKER" run \
    --repo "$FIXTURE_ROOT" --request "$REQUEST_TWO" --output "$RECEIPT_TWO"

  [ "$status" -eq 2 ]
  [[ "$output" == *"duplicate_run"* ]]
  [ ! -e "$RECEIPT_TWO" ]
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 1 ]
}

@test "restart after simulated reservation crash refuses without a gate" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]

  run env AGENTOPS_VALIDATION_TEST_CRASH_AFTER_RESERVATION=1 \
    python3 "$CHECKER" run --repo "$FIXTURE_ROOT" --request "$REQUEST" --output "$RECEIPT"
  [ "$status" -eq 75 ]
  jq -e '.state == "RESERVED" and (.request_sha256 | length) == 64' "$RECEIPT"

  run python3 "$CHECKER" run \
    --repo "$FIXTURE_ROOT" --request "$REQUEST" --output "$RECEIPT_TWO"

  [ "$status" -eq 2 ]
  [[ "$output" == *"duplicate_run"* ]]
  [ ! -e "$RECEIPT_TWO" ]
  [ ! -s "$COUNT_FILE" ]
}

@test "diagnostic FAIL is attributed but remains nonbinding" {
  seed_repo PASS PASS
  add_nonbinding_gate diagnostic diagnostic-fact diagnostic-status.txt
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 0 ]
  jq -e '
    .disposition == "READY" and
    .model_spend_allowed == true and
    .next_action == "VALIDATE_SINGLE_FRESH" and
    .gate_executions[1].lane == "diagnostic" and
    .gate_executions[1].candidate.status == "FAIL" and
    .gate_executions[1].baseline.status == "PASS" and
    .gate_executions[1].attribution == "candidate_introduced"
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 3 ]
}

@test "release FAIL is attributed but remains nonbinding to validation" {
  seed_repo PASS PASS
  add_nonbinding_gate release release-fact release-status.txt
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 0 ]
  jq -e '
    .disposition == "READY" and
    .model_spend_allowed == true and
    .gate_executions[1].lane == "release" and
    .gate_executions[1].candidate.status == "FAIL" and
    .gate_executions[1].baseline.status == "PASS" and
    .gate_executions[1].attribution == "candidate_introduced"
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 3 ]
}

@test "diagnostic UNKNOWN remains a global integrity blocker" {
  seed_repo PASS PASS
  add_nonbinding_gate diagnostic diagnostic-fact diagnostic-status.txt UNKNOWN
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 1 ]
  jq -e '
    .disposition == "BLOCK" and
    .model_spend_allowed == false and
    .gate_executions[1].lane == "diagnostic" and
    .gate_executions[1].candidate.status == "UNKNOWN" and
    (.gate_executions[1] | has("baseline") | not)
  ' "$RECEIPT"
  [ "$(wc -l <"$COUNT_FILE" | tr -d ' ')" -eq 2 ]
}

@test "a request without a mandatory lane blocks before every gate" {
  seed_repo PASS PASS
  add_nonbinding_gate diagnostic diagnostic-fact diagnostic-status.txt
  jq '.selected_gate_ids = ["diagnostic-fact"]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"
  freeze_request
  [ "$status" -eq 0 ]

  run_request

  [ "$status" -eq 1 ]
  jq -e '
    .preflight_errors[0].code == "missing_mandatory_gate" and
    .gate_executions == [] and
    .model_spend_allowed == false and
    .disposition == "BLOCK"
  ' "$RECEIPT"
  [ ! -s "$COUNT_FILE" ]
}

@test "freeze rejects an omitted changed deletion" {
  seed_repo PASS PASS
  git -C "$FIXTURE_ROOT" rm -q obsolete.txt
  git -C "$FIXTURE_ROOT" commit -q -m delete-obsolete
  CANDIDATE_SHA="$(git -C "$FIXTURE_ROOT" rev-parse HEAD)"
  write_spec
  jq '.owned_paths -= ["obsolete.txt"]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"owned_paths_mismatch"* ]]
  [ ! -e "$REQUEST" ]
}

@test "freeze rejects an extra unchanged owned path" {
  seed_repo PASS PASS
  jq '.owned_paths += ["acceptance.txt"]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"owned_paths_mismatch"* ]]
  [ ! -e "$REQUEST" ]
}

@test "freeze rejects an empty subtree identity set" {
  seed_repo PASS PASS
  jq '.subtree_paths = []' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"subtree_paths"* ]]
  [ ! -e "$REQUEST" ]
}

@test "freeze rejects duplicate subtree identities" {
  seed_repo PASS PASS
  jq '.subtree_paths = ["scripts", "scripts"]' "$SPEC" >"$SPEC.tmp"
  mv "$SPEC.tmp" "$SPEC"

  freeze_request

  [ "$status" -eq 1 ]
  [[ "$output" == *"subtree_paths"* ]]
  [ ! -e "$REQUEST" ]
}

@test "receipt rejects READY authority with mandatory FAIL" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq '
    .gate_executions[0].candidate.status = "FAIL" |
    .gate_executions[0].baseline = .gate_executions[0].candidate |
    .gate_executions[0].baseline.status = "PASS" |
    .gate_executions[0].attribution = "candidate_introduced"
  ' "$RECEIPT" >"$BAD_RECEIPT"

  check_bad_receipt

  [ "$status" -eq 1 ]
}

@test "receipt rejects READY authority with no executions" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq '.gate_executions = []' "$RECEIPT" >"$BAD_RECEIPT"

  check_bad_receipt

  [ "$status" -eq 1 ]
}

@test "receipt rejects attribution without or against its baseline" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq '.gate_executions[0].attribution = "candidate_introduced"' "$RECEIPT" >"$BAD_RECEIPT"
  check_bad_receipt
  [ "$status" -eq 1 ]

  jq '
    .gate_executions[0].candidate.status = "FAIL" |
    .gate_executions[0].baseline = .gate_executions[0].candidate |
    .gate_executions[0].baseline.status = "FAIL" |
    .gate_executions[0].attribution = "candidate_introduced"
  ' "$RECEIPT" >"$BAD_RECEIPT"
  check_bad_receipt
  [ "$status" -eq 1 ]
}

@test "receipt rejects a proof kind that differs from the frozen registry" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq '.gate_executions[0].proof_kind = "schema"' "$RECEIPT" >"$BAD_RECEIPT"

  check_bad_receipt

  [ "$status" -eq 1 ]
  [[ "$output" == *"gate executions do not exactly match selected gates"* ]]
}

@test "receipt rejects contradictory READY preflight evidence" {
  seed_repo PASS PASS
  freeze_request
  [ "$status" -eq 0 ]
  run_request
  [ "$status" -eq 0 ]
  jq '.preflight_errors = [{
    code: "stale_claim_dependency",
    defect_class: "evidence_integrity",
    detail: "claim.txt"
  }]' \
    "$RECEIPT" >"$BAD_RECEIPT"

  check_bad_receipt

  [ "$status" -eq 1 ]
}
