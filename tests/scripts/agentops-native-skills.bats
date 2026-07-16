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
@test "idea-genie duel mode emits sealed advisory evidence for Plan" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-challenge.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["port ownership"],"refutations":[{"claim":"P1","attempt":"existing seam","result":"survived"}],"handoff":{"owner":"plan","artifact_dir":".agents/ideas/run-1"}}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":false,"perspectives":[{"id":"P1","context_id":"same"},{"id":"P2","context_id":"same"}],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"self-score"}}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["x"],"refutations":[{"claim":"P1","attempt":"x","result":"survived"}],"handoff":{"owner":"plan","artifact_dir":".agents/ideas/run-1"},"readiness":"PASS"}'
}

# B2.2
@test "idea-genie duel mode routes reversible choices without NTM ceremony" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-challenge.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"two-way","sealed_generation":false,"perspectives":[],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"plan","artifact_dir":".agents/ideas/run-2","route":"single-fresh-context"},"requires_ntm":false}'
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
