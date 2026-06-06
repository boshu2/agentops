#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/classify-bd-remember.py"
  FIXTURE="$REPO_ROOT/tests/fixtures/bd-remember-memories.json"
}

@test "classifies every bd memory and preserves lineage" {
  run python3 "$SCRIPT" \
    --input "$FIXTURE" \
    --expected-count 4 \
    --generated-at "2026-06-05T00:00:00+00:00"

  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.schema_version == "bd-remember-migration-manifest.v1"'
  echo "$output" | jq -e '.total == 4 and .classified == 4 and .unclassified == 0'
  echo "$output" | jq -e '.counts["pull-learning"] == 1'
  echo "$output" | jq -e '.counts.bead == 2'
  echo "$output" | jq -e '.counts.discard == 1'
  echo "$output" | jq -e '.items[] | select(.key == "landed-means-origin-main") | .proposed_learning.reach == "pull"'
  echo "$output" | jq -e '.items[] | select(.key == "landed-means-origin-main") | .proposed_learning.maturity == "provisional"'
  echo "$output" | jq -e '.items[] | select(.key == "landed-means-origin-main") | .proposed_learning.source == "migrated-bd-remember"'
  echo "$output" | jq -e '.items[] | select(.key == "landed-means-origin-main") | .lineage.body == "Verify landed work against origin/main or GitHub, not local fast-forward state."'
  echo "$output" | jq -e '.items[] | select(.key == "ag-rbx8-pr-body-evidence") | .disposition == "bead"'
  echo "$output" | jq -e '.items[] | select(.key == "old-evolve-cli") | .disposition == "discard"'
}

@test "markdown manifest exposes zero-unclassified invariant" {
  run python3 "$SCRIPT" \
    --input "$FIXTURE" \
    --format markdown \
    --generated-at "2026-06-05T00:00:00+00:00"

  [ "$status" -eq 0 ]
  [[ "$output" == *"# bd remember migration manifest"* ]]
  [[ "$output" == *"Unclassified: **0**"* ]]
  [[ "$output" == *"\`pull-learning\`: **1**"* ]]
  [[ "$output" == *"\`bead\`: **2**"* ]]
  [[ "$output" == *"\`discard\`: **1**"* ]]
}

@test "expected-count mismatch fails before emitting manifest" {
  run python3 "$SCRIPT" --input "$FIXTURE" --expected-count 196

  [ "$status" -eq 1 ]
  [[ "$output" == *"expected 196 memories, got 4"* ]]
  [[ "$output" != *"bd-remember-migration-manifest.v1"* ]]
}
