#!/usr/bin/env bats
# Unit tests for the membrane's deterministic close-door verdict (finalize.sh +
# finalize.jq). This is where the membrane's correctness is proven CHEAPLY — no
# live drill, no agents, no gc — by feeding fixture review-quorum.lane.v1 JSONs
# and asserting the disposition + exit code + pawl-verdict.v1 shape.
#
# The four contract paths (bead .1 asks): CONFIRMED, REFUTED (hard finding),
# DEGRADED (transient lane loss — no attempt consumed), and nonce-missing
# rejection (stale verdict → fail-closed).

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIN="$PACK/membrane/finalize.sh"
  TMP="$BATS_TEST_TMPDIR"
  OUT="$TMP/pawl.json"
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
    --out "$OUT" -- "$@"
}

@test "PATH 1 — CONFIRMED: two cross-family lanes pass, nonce matches (exit 0)" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 0 ]
  [[ "$output" == CONFIRMED* ]]
  [ "$(jq -r '.disposition' "$OUT")" = "CONFIRMED" ]
  [ "$(jq -r '.schema_version' "$OUT")" = "pawl-verdict.v1" ]
  [ "$(jq -r '.head_sha' "$OUT")" = "deadbeef" ]
  [ "$(jq -r '.nonce' "$OUT")" = "n1" ]
  [ "$(jq '.refuters | length' "$OUT")" -eq 2 ]
}

@test "PATH 2 — REFUTED: a hard fail lane with a finding (exit 2)" {
  lane "$TMP/a.json" codex-lane gpt fail none "" n1 '[{"title":"missing max","body":"x","file":"calc.sh","severity":"high"}]'
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  [[ "$output" == REFUTED* ]]
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
  [ "$(jq -r '.failure_class' "$OUT")" = "hard" ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *lane_failed* ]]
}

@test "PATH 3 — DEGRADED: transient lane loss does not consume an attempt (exit 3)" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  lane "$TMP/b.json" agy-lane gemini blocked transient provider_rate_limited n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 3 ]
  [[ "$output" == DEGRADED* ]]
  [ "$(jq -r '.disposition' "$OUT")" = "DEGRADED" ]
  [ "$(jq -r '.failure_class' "$OUT")" = "transient" ]
}

@test "PATH 3b — DEGRADED: an expected family simply did not report (exit 3)" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  run_finalize "$TMP/a.json"    # gemini lane missing entirely
  [ "$status" -eq 3 ]
  [ "$(jq -r '.disposition' "$OUT")" = "DEGRADED" ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *provider_unavailable* ]]
}

@test "PATH 4 — REFUTED: nonce mismatch is a stale verdict, fail-closed (exit 2)" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n0   # wrong nonce
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *nonce_mismatch* ]]
}

@test "PATH 5 — DEGRADED: zero lanes (no reviewer output) = awaiting, not refute (exit 3)" {
  run_finalize
  [ "$status" -eq 3 ]
  [ "$(jq -r '.disposition' "$OUT")" = "DEGRADED" ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *awaiting_reviewers* ]]
}

@test "PATH 6 — REFUTED: same-family quorum is not cross-family (fail-closed)" {
  # both lanes gpt: cross-family precondition unmet -> hard, not a silent pass
  run "$FIN" --nonce n1 --round 1 --subject age-x --base-ref main \
    --expected-families "gpt,gpt" --head-sha deadbeef --author builder-1 \
    --out "$OUT" -- "$TMP/a.json"
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  run "$FIN" --nonce n1 --round 1 --subject age-x --base-ref main \
    --expected-families "gpt" --head-sha deadbeef --author builder-1 \
    --out "$OUT" -- "$TMP/a.json"
  [ "$status" -eq 2 ]
  [ "$(jq -r '.disposition' "$OUT")" = "REFUTED" ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *cross_family_precondition_unmet* ]]
}

@test "read-only violation (verifier mutated files) is a hard REFUTED" {
  lane "$TMP/a.json" codex-lane gpt pass none "" n1
  # tamper: passed=false
  jq '.read_only_enforcement.passed=false' "$TMP/a.json" > "$TMP/a2.json"
  lane "$TMP/b.json" agy-lane gemini pass none "" n1
  run_finalize "$TMP/a2.json" "$TMP/b.json"
  [ "$status" -eq 2 ]
  [[ "$(jq -r '.failure_reason' "$OUT")" == *read_only_mutation_detected* ]]
}
