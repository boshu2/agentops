#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CASE=0
}

json_file() {
  CASE=$((CASE + 1))
  local path="$BATS_TEST_TMPDIR/case-$CASE.json"
  printf '%s\n' "$1" > "$path"
  printf '%s\n' "$path"
}

assert_accepts() {
  local validator="$1" payload="$2" path
  [ -x "$validator" ]
  path="$(json_file "$payload")"
  run "$validator" "$path"
  [ "$status" -eq 0 ]
}

assert_rejects() {
  local validator="$1" payload="$2" path
  [ -x "$validator" ]
  path="$(json_file "$payload")"
  run "$validator" "$path"
  [ "$status" -ne 0 ]
}

# B1.1
@test "idea-genie produces an evidence-grounded idea-portfolio artifact" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-portfolio.v1","status":"candidates","observations":[{"claim":"users cannot discover the route","evidence":"docs/SKILLS.md:1"}],"assumptions":["generated maps are current"],"candidates":[{"id":"I1","evidence":["docs/SKILLS.md:1"],"overlaps":[],"scenario":{"given":"a user has a goal","when":"routing runs","then":"the owner is named"}}],"termination":{"reason":"novelty-saturated","novel_candidates_last_pass":0}}'
  assert_rejects "$v" '{"schema_version":"idea-portfolio.v1","status":"candidates","observations":[],"assumptions":[],"candidates":[{"id":"I1","evidence":[],"overlaps":[],"scenario":{"given":"x","when":"y","then":"z"}}],"termination":{"reason":"novelty-saturated","novel_candidates_last_pass":0}}'
}

# B1.2
@test "idea-genie can return no-new-work without manufacturing candidates" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-portfolio.v1","status":"no-new-work","observations":[{"claim":"all candidates overlap","evidence":"skills/catalog.json"}],"assumptions":[],"candidates":[],"termination":{"reason":"all-overlap-or-unsupported","novel_candidates_last_pass":0}}'
  assert_rejects "$v" '{"schema_version":"idea-portfolio.v1","status":"no-new-work","observations":[],"assumptions":[],"candidates":[{"id":"padding"}],"termination":{"reason":"fixed-count-reached","novel_candidates_last_pass":1}}'
}

# B2.1
@test "dueling-idea-genies emits a sealed challenge packet for plan-pawl" {
  v="$REPO_ROOT/skills/dueling-idea-genies/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["port ownership"],"refutations":[{"claim":"P1","attempt":"existing seam","result":"survived"}],"handoff":{"owner":"ao plan-pawl decide","artifact_dir":".agents/duel/run-1"}}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":false,"perspectives":[{"id":"P1","context_id":"same"},{"id":"P2","context_id":"same"}],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"self-score"}}'
  run go -C "$REPO_ROOT/cli" test ./internal/planpawl
  [ "$status" -eq 0 ]
}

# B2.2
@test "dueling-idea-genies routes reversible choices without NTM ceremony" {
  v="$REPO_ROOT/skills/dueling-idea-genies/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"two-way","sealed_generation":false,"perspectives":[],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"idea-genie","route":"single-fresh-context"},"requires_ntm":false}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"two-way","sealed_generation":true,"perspectives":[],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"ntm"},"requires_ntm":true}'
}

# B3.1
@test "codebase-recon validates evidence-bounded fact inference unknown claims" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  evidence="$BATS_TEST_TMPDIR/evidence.txt"; printf 'entry -> domain -> test\n' > "$evidence"
  assert_accepts "$v" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"deadbeef\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"a flow exists\",\"confidence\":\"high\",\"evidence\":[\"$evidence\"]},{\"kind\":\"unknown\",\"text\":\"remote behavior\",\"confidence\":\"low\",\"evidence\":[]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"
  assert_rejects "$v" '{"schema_version":"codebase-recon.v1","mode":"baseline","commit":"deadbeef","flows":[],"claims":[{"kind":"fact","text":"unsupported","confidence":"high","evidence":[]}],"coverage":{"inspected":[],"uninspected":[]}}'
}

# B3.2
@test "codebase-recon requires a verified delta when a prior pack exists" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  prior="$BATS_TEST_TMPDIR/prior.md"; printf 'prior baseline\n' > "$prior"
  assert_accepts "$v" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"feedface\",\"prior_recon\":\"$prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"
  assert_rejects "$v" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"feedface\",\"prior_recon\":\"$prior\",\"baseline_verified\":false,\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[],\"uninspected\":[]}}"
}

# B4.1
@test "pattern-mining promotes only a three-exemplar holdout-proven pattern" {
  v="$REPO_ROOT/skills/pattern-mining/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"pattern-mining.v1","outcome":"promote","exemplars":["a.go:1","b.go:2","c.go:3"],"invariants":["fail closed"],"variations":["transport"],"incidental":["identifier"],"holdout":{"source":"d.go:4","result":"pass"},"back_application":"pass","route":"operationalize"}'
  assert_rejects "$v" '{"schema_version":"pattern-mining.v1","outcome":"promote","exemplars":["a.go:1","b.go:2"],"invariants":["x"],"variations":[],"incidental":[],"holdout":{"source":"c.go:3","result":"pass"},"back_application":"pass","route":"skill"}'
}

# B4.2
@test "pattern-mining keeps weak evidence as a hypothesis" {
  v="$REPO_ROOT/skills/pattern-mining/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"pattern-mining.v1","outcome":"hypothesis","exemplars":["a.go:1","b.go:2"],"invariants":[],"variations":[],"incidental":[],"holdout":{"source":"","result":"not-run"},"back_application":"not-run","route":"no-action"}'
  assert_rejects "$v" '{"schema_version":"pattern-mining.v1","outcome":"hypothesis","exemplars":["a.go:1"],"invariants":[],"variations":[],"incidental":[],"holdout":{"source":"","result":"not-run"},"back_application":"not-run","route":"skill"}'
}

# B5.1
@test "NTM AgentWorker executes the real robot spawn send observe lifecycle" {
  run go -C "$REPO_ROOT/cli" test ./internal/adapters/agentworker_ntm ./internal/adapters/agentmail_cli -run '^(TestNTMWorkerLifecycle|TestAgentMailCLIIdentityReservationAckLifecycle)$'
  [ "$status" -eq 0 ]
}

# B5.2
@test "agent lifecycle uses suspect then bounded nudge then replacement" {
  run go -C "$REPO_ROOT/cli" test ./internal/agentworker -run '^TestSupervisorTwoTickPolicy$'
  [ "$status" -eq 0 ]
}

# B6.1
@test "pawl-review returns a fresh read-only nonce-bound NTM lane result" {
  run go -C "$REPO_ROOT/cli" test ./internal/ports ./internal/adapters/reviewlane_worker -run '^(TestReviewRequestV1RejectsMutableOrSelfReview|TestNTMReviewLaneFreshReadOnlyNonce)$'
  [ "$status" -eq 0 ]
}

# B6.2
@test "review transport loss cannot become semantic REFUTED or CONFIRMED" {
  run go -C "$REPO_ROOT/cli" test ./internal/ports ./internal/adapters/reviewlane_worker -run '^(TestReviewLaneResultV1SeparatesTransportFromSemanticFailure|TestReviewLaneTransportFailureIsNotRefutation)$'
  [ "$status" -eq 0 ]
}

# B7.1
@test "using-gc exposes optional worker and review-lane composition" {
  checker="$REPO_ROOT/scripts/check-skill-mesh.py"; [ -x "$checker" ]
  run "$checker" --optional-edge using-gc:agent-native --optional-edge using-gc:pawl-review
  [ "$status" -eq 0 ]
}

# B7.2
@test "GC real finalizer emits canonical verdicts with contained nonempty evidence" {
  run bats --filter 'CANONICAL' packs/agentops-membrane/tests/finalize.bats
  [ "$status" -eq 0 ]
  run bats packs/agentops-membrane/tests/close-gate.bats
  [ "$status" -eq 0 ]
  run bats packs/agentops-membrane/tests/breaker-escalation.bats
  [ "$status" -eq 0 ]
}

# B7.3
@test "GC and NTM remain independently selectable adapters" {
  checker="$REPO_ROOT/scripts/check-skill-mesh.py"; [ -x "$checker" ]
  run "$checker" --independent using-gc:ntm --independent using-gc:agent-native
  [ "$status" -eq 0 ]
}

# B8.1
@test "ATM-era callers migrate to agent-native and pawl-review" {
  checker="$REPO_ROOT/scripts/check-skill-mesh.py"; [ -x "$checker" ]
  run "$checker" --retired using-atm:agent-native --retired pre-land-refuters:pawl-review
  [ "$status" -eq 0 ]
}

# B8.2
@test "canonical skills keep NTM and Agent Mail as external adapters" {
  checker="$REPO_ROOT/scripts/check-orchestration-skill-boundaries.sh"; [ -x "$checker" ]
  run "$checker"
  [ "$status" -eq 0 ]
}

# B9.1
@test "every admitted new capability is reachable from an existing entry point" {
  checker="$REPO_ROOT/scripts/check-skill-mesh.py"; [ -x "$checker" ]
  run "$checker" --require-reachable idea-genie,dueling-idea-genies,codebase-recon,pattern-mining,pawl-review
  [ "$status" -eq 0 ]
}

# B9.2
@test "entry points delegate without copying leaf workflows" {
  checker="$REPO_ROOT/scripts/check-clean-room-similarity.py"; [ -x "$checker" ]
  run "$checker" --mesh-only
  [ "$status" -eq 0 ]
}

# B10.1
@test "existing catalog context-map and ao graph regenerate every live skill" {
  run bash scripts/generate-skill-catalog.sh --check
  [ "$status" -eq 0 ]
  run bash scripts/validate-context-map-drift.sh
  [ "$status" -eq 0 ]
  run go -C "$REPO_ROOT/cli" test ./internal/skills -run '^(TestGraphJSONCarriesTypedEdgesAndDiagnostics|TestMermaidUsesDependenciesNotArtifactConsumes)$'
  [ "$status" -eq 0 ]
}

# B10.2
@test "graph topology rejects duplicates dangling cycles and unreachable non-roots" {
  run go -C "$REPO_ROOT/cli" test ./internal/skills -run '^(TestBuildGraphRejects|TestBuildGraphPreservesExplicitRoot)'
  [ "$status" -eq 0 ]
  stale="$BATS_TEST_TMPDIR/catalog.json"
  run bash scripts/generate-skill-catalog.sh --out "$stale"
  [ "$status" -eq 0 ]
  jq '.skills |= .[1:] | .skill_count -= 1' "$stale" > "$stale.tmp" && mv "$stale.tmp" "$stale"
  run bash scripts/generate-skill-catalog.sh --check --out "$stale"
  [ "$status" -ne 0 ]
  [[ "$output" == *"bash scripts/generate-skill-catalog.sh"* ]]
}

@test "clean-room gate rejects planted copied text and validates captured manifests" {
  checker="$REPO_ROOT/scripts/check-clean-room-similarity.py"; [ -x "$checker" ]
  planted="$BATS_TEST_TMPDIR/planted"; mkdir -p "$planted/external" "$planted/agentops"
  printf 'alpha beta gamma delta epsilon zeta eta theta iota kappa\n' > "$planted/external/SKILL.md"
  printf 'alpha beta gamma delta epsilon zeta eta theta iota kappa\n' > "$planted/agentops/SKILL.md"
  run "$checker" --external-root "$planted/external" --agentops-root "$planted/agentops" --min-words 8
  [ "$status" -ne 0 ]
  run "$checker" --check-manifests --check-receipt "$REPO_ROOT/docs/audits/clean-room-adoption-receipt-2026-07-09.json" --ci-denylist
  [ "$status" -eq 0 ]
  cp "$REPO_ROOT/docs/audits/manifests/external-skill-official-2026-07-09.txt" "$BATS_TEST_TMPDIR/tampered.txt"
  printf 'tampered-entry\n' >> "$BATS_TEST_TMPDIR/tampered.txt"
  run "$checker" --official-manifest "$BATS_TEST_TMPDIR/tampered.txt" --check-manifests
  [ "$status" -ne 0 ]
}
