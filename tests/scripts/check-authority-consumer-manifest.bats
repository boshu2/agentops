#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/skills/plan/scripts/check-authority-consumer-manifest.py"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  MANIFEST="$BATS_TEST_TMPDIR/manifest.json"
  INVENTORY_OUTPUT="$BATS_TEST_TMPDIR/inventory-paths.txt"
  mkdir -p "$FIXTURE_ROOT"
  printf 'contract authority\n' >"$FIXTURE_ROOT/authority.txt"
  printf 'contract consumer a\n' >"$FIXTURE_ROOT/consumer-a.txt"
  printf 'contract consumer b\n' >"$FIXTURE_ROOT/consumer-b.txt"
  (
    cd "$FIXTURE_ROOT"
    rg -l --fixed-strings contract . | sort >"$INVENTORY_OUTPUT"
  )
}

@test "an independently captured live consumer omitted from observed_paths fails closed" {
  write_disjoint_manifest
  (
    cd "$FIXTURE_ROOT"
    rg -l --fixed-strings contract . | sort >"$INVENTORY_OUTPUT"
  )
  jq '
    .inventory.observed_paths -= ["consumer-b.txt"]
    | del(.consumers[] | select(.path == "consumer-b.txt"))
    | .slices = [.slices[0]]
  ' "$MANIFEST" >"$MANIFEST.tmp"
  mv "$MANIFEST.tmp" "$MANIFEST"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 1 ]
  [[ "$output" == *"inventory output path omitted from observed_paths: consumer-b.txt"* ]]
}

@test "inventory.command remains provenance and is never executed" {
  write_disjoint_manifest
  sentinel="$BATS_TEST_TMPDIR/manifest-command-executed"
  jq --arg command "touch $sentinel" '.inventory.command = $command' \
    "$MANIFEST" >"$MANIFEST.tmp"
  mv "$MANIFEST.tmp" "$MANIFEST"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 0 ]
  [ ! -e "$sentinel" ]
}

write_disjoint_manifest() {
  jq -n '{
    schema_version: 1,
    migration_id: "fixture-migration",
    authorities: [{path: "authority.txt", symbols: ["contract"]}],
    inventory: {
      command: "rg -l --fixed-strings contract .",
      observed_paths: ["authority.txt", "consumer-a.txt", "consumer-b.txt"],
      complete: true
    },
    consumers: [
      {path: "consumer-a.txt", authority_path: "authority.txt", kind: "runtime"},
      {path: "consumer-b.txt", authority_path: "authority.txt", kind: "generated"}
    ],
    slices: [
      {id: "S1", write_scope: ["authority.txt", "consumer-a.txt"]},
      {id: "S2", write_scope: ["consumer-b.txt"]}
    ]
  }' >"$MANIFEST"
}

@test "complete checked consumer inventory classifies disjoint scopes as parallel-safe" {
  write_disjoint_manifest

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 0 ]
  [ "$(jq -r '.manifest_status' <<<"$output")" = "complete" ]
  [ "$(jq -r '.scope_classification' <<<"$output")" = "disjoint" ]
  [ "$(jq -r '.parallel_safe' <<<"$output")" = "true" ]
}

@test "complete checked consumer inventory classifies a shared write as serialized" {
  write_disjoint_manifest
  jq '(.slices[] | select(.id == "S2") | .write_scope) += ["consumer-a.txt"]' \
    "$MANIFEST" >"$MANIFEST.tmp"
  mv "$MANIFEST.tmp" "$MANIFEST"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 0 ]
  [ "$(jq -r '.manifest_status' <<<"$output")" = "complete" ]
  [ "$(jq -r '.scope_classification' <<<"$output")" = "shared" ]
  [ "$(jq -r '.parallel_safe' <<<"$output")" = "false" ]
  [ "$(jq -r '.shared_paths[0]' <<<"$output")" = "consumer-a.txt" ]
}

@test "unchecked or explicitly incomplete inventory fails closed" {
  write_disjoint_manifest
  jq '.inventory.complete = false' "$MANIFEST" >"$MANIFEST.tmp"
  mv "$MANIFEST.tmp" "$MANIFEST"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 1 ]
  [ "$(jq -r '.manifest_status' <<<"$output")" = "incomplete" ]
  [ "$(jq -r '.scope_classification' <<<"$output")" = "incomplete" ]
  [ "$(jq -r '.parallel_safe' <<<"$output")" = "false" ]
}

@test "an observed generated consumer omitted from the manifest fails closed" {
  write_disjoint_manifest
  jq 'del(.consumers[] | select(.path == "consumer-b.txt"))' \
    "$MANIFEST" >"$MANIFEST.tmp"
  mv "$MANIFEST.tmp" "$MANIFEST"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 1 ]
  [[ "$output" == *"observed path is not classified: consumer-b.txt"* ]]
}

@test "an inventory path missing at the checked repository root fails closed" {
  write_disjoint_manifest
  rm "$FIXTURE_ROOT/consumer-b.txt"

  run python3 "$CHECKER" --repo "$FIXTURE_ROOT" \
    --inventory-output "$INVENTORY_OUTPUT" "$MANIFEST"

  [ "$status" -eq 1 ]
  [[ "$output" == *"path does not exist: consumer-b.txt"* ]]
}
