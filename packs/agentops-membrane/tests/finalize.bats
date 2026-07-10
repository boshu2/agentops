#!/usr/bin/env bats
# Unit tests for the membrane's deterministic close-door verdict (finalize.sh +
# finalize.jq). This is where the membrane's correctness is proven CHEAPLY — no
# live drill, no agents, no gc — by feeding fixture review-quorum.lane.v1 JSONs
# and asserting the disposition + exit code + pawl-verdict.v1 shape.
#
# The four contract paths (bead .1 asks): CONFIRMED, REFUTED (hard finding),
# DEGRADED (transient lane loss — nonsemantic, but native GC still consumes a
# failed check attempt), and nonce-missing
# rejection (stale verdict → fail-closed).

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  REPO="$(cd "$PACK/../.." && pwd)"
  FIN="$PACK/membrane/finalize.sh"
  TMP="$BATS_TEST_TMPDIR"
  OUT="$TMP/pawl.json"
  ATTEMPT="$TMP/attempt.json"
  EVDIR="$TMP/evidence"
  SCHEMA="$REPO/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$FIN" 2>/dev/null || true
}

# lane <file> <lane_id> <provider> <verdict> <fclass> <freason> <nonce> [findings_json]
lane() {
  local f="$1" id="$2" prov="$3" v="$4" fc="$5" fr="$6" nonce="$7" findings="${8:-[]}"
  local n; n="$(printf '%s' "$findings" | jq 'length')"
  jq -n --arg id "$id" --arg prov "$prov" --arg v "$v" --arg fc "$fc" \
        --arg fr "$fr" --arg nonce "$nonce" --argjson findings "$findings" --argjson n "$n" '{
    lane_id:$id, provider:$prov, model:($prov+"-model"), verdict:$v, summary:"t",
    findings_count:$n, findings:$findings, evidence:[], usage:null,
    read_only_enforcement:{observed:true,enabled:true,passed:true,baseline_command:"git status",after_command:"git status"},
    mutations_delta:{changed:[]}, failure_class:$fc, failure_reason:$fr, agentops_nonce:$nonce
  }' > "$f"
}

run_finalize() {
  run "$FIN" --nonce n1 --round 1 --subject age-x --base-ref main \
    --expected-families "gpt,gemini" --head-sha deadbeef --author builder-1 \
    --out "$OUT" --attempt-out "$ATTEMPT" --evidence-dir "$EVDIR" -- "$@"
}

assert_canonical_verdict() {
  python3 - "$SCHEMA" "$OUT" <<'PY'
import json, pathlib, sys
import jsonschema

schema = json.load(open(sys.argv[1]))
verdict = json.load(open(sys.argv[2]))
jsonschema.validate(verdict, schema)
root = pathlib.Path(verdict["refuters"][0]["evidence"]).parent.resolve()
for refuter in verdict["refuters"]:
    evidence = pathlib.Path(refuter["evidence"])
    assert evidence.is_file() and evidence.stat().st_size > 0
    assert evidence.resolve().is_relative_to(root)
PY
}

@test "CANONICAL PATH 1 — CONFIRMED is schema-valid and evidence-bound" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 0 ]
  [[ "$output" == CONFIRMED* ]]
  assert_canonical_verdict
  [ "$(jq -r '.disposition' "$OUT")" = "CONFIRMED" ]
  [ "$(jq -r '.schema_version' "$OUT")" = "pawl-verdict.v1" ]
  [ "$(jq -r '.head_sha' "$OUT")" = "deadbeef" ]
  [ "$(jq '.refuters | length' "$OUT")" -eq 2 ]
  [ "$(jq '[.refuters[].verdict] | all(. == "CONFIRMED")' "$OUT")" = true ]
  [ ! -e "$ATTEMPT" ]
}

@test "CANONICAL PATH 2 — REFUTED is schema-valid and preserves lane semantics" {
  lane "$TMP/a.json" codex-lane gpt fail none "" n1 '[{"title":"missing max","body":"x","file":"calc.sh","severity":"high"}]'
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  [[ "$output" == REFUTED* ]]
  assert_canonical_verdict
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
  [ "$(jq -r '.refuters[] | select(.context_id == "codex-lane") | .verdict' "$OUT")" = "REFUTED" ]
  [ "$(jq -r '.refuters[] | select(.context_id == "agy-lane") | .verdict' "$OUT")" = "CONFIRMED" ]
}

@test "CANONICAL PATH 3 — DEGRADED writes a nonsemantic attempt artifact, never a fake pawl verdict" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  lane "$TMP/b.json" agy-lane gemini blocked transient provider_rate_limited n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 3 ]
  [[ "$output" == DEGRADED* ]]
  [ ! -e "$OUT" ]
  [ "$(jq -r '.schema_version' "$ATTEMPT")" = "gc-review-attempt.v1" ]
  [ "$(jq -r '.outcome' "$ATTEMPT")" = "DEGRADED" ]
  [ "$(jq -r '.failure_class' "$ATTEMPT")" = "transient" ]
}

@test "CANONICAL PATH 3b — missing family cannot overwrite the latest terminal verdict" {
  printf '%s\n' '{"schema_version":"pawl-verdict.v1","sentinel":"keep"}' > "$OUT"
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  run_finalize "$TMP/a.json"    # gemini lane missing entirely
  [ "$status" -eq 3 ]
  [ ! -e "$OUT" ]
  [[ "$(jq -r '.failure_reason' "$ATTEMPT")" == *provider_unavailable* ]]
}

@test "CANONICAL PATH 4 — nonce mismatch is a schema-valid fail-closed REFUTED" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n0   # wrong nonce
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  assert_canonical_verdict
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
  [ "$(jq -r '.refuters[] | select(.context_id == "codex-lane") | .verdict' "$OUT")" = "REFUTED" ]
}

@test "CANONICAL PATH 5 — zero lanes produce only an awaiting-reviewers attempt" {
  run_finalize
  [ "$status" -eq 3 ]
  [ ! -e "$OUT" ]
  [[ "$(jq -r '.failure_reason' "$ATTEMPT")" == *awaiting_reviewers* ]]
}

@test "CANONICAL PATH 6 — same-family quorum is schema-valid fail-closed REFUTED" {
  # both lanes gpt: cross-family precondition unmet -> hard, not a silent pass
  run "$FIN" --nonce n1 --round 1 --subject age-x --base-ref main \
    --expected-families "gpt,gpt" --head-sha deadbeef --author builder-1 \
    --out "$OUT" -- "$TMP/a.json"
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  run "$FIN" --nonce n1 --round 1 --subject age-x --base-ref main \
    --expected-families "gpt" --head-sha deadbeef --author builder-1 \
    --out "$OUT" --attempt-out "$ATTEMPT" --evidence-dir "$EVDIR" -- "$TMP/a.json"
  [ "$status" -eq 2 ]
  assert_canonical_verdict
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
}

@test "read-only violation (verifier mutated files) is a hard REFUTED" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  # tamper: passed=false
  jq '.read_only_enforcement.passed=false' "$TMP/a.json" > "$TMP/a2.json"
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a2.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  assert_canonical_verdict
  [ "$(jq -r '.refuters[] | select(.context_id == "codex-lane") | .verdict' "$OUT")" = "REFUTED" ]
}

@test "CANONICAL evidence paths stay contained when a lane id is hostile" {
  lane "$TMP/a.json" "../../escape" gpt pass none "" n1
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 0 ]
  assert_canonical_verdict
  [ ! -e "$TMP/escape.json" ]
}
