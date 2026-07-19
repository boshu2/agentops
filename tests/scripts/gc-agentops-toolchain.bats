#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  MATERIALIZE="$REPO_ROOT/deploy/gc/materialize-toolchain.sh"
  LOCK="$REPO_ROOT/deploy/gc/toolchain.lock.json"
}

@test "materializer describes the qualified pair by default" {
  run "$MATERIALIZE" --describe

  [ "$status" -eq 0 ]
  python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["id"] == "agentops-gc-v16-20260719"; assert value["status"] == "qualified"' <<<"$output"
}

@test "materializer can select the compatible official release" {
  run "$MATERIALIZE" --pair gascity-v1.3.5-sdk-release --describe

  [ "$status" -eq 0 ]
  python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["gc"]["ref"] == "v1.3.5"; assert value["bd"]["ref"] == "v1.1.0"' <<<"$output"
}

@test "materializer fails closed for an unknown pair" {
  run "$MATERIALIZE" --pair does-not-exist --describe

  [ "$status" -ne 0 ]
  [[ "$output" == *"unknown pair id"* ]]
}

@test "materializer refuses a nonempty destination before cloning" {
  destination="$BATS_TEST_TMPDIR/existing"
  mkdir -p "$destination"
  printf '%s\n' preserve >"$destination/user-file"

  run "$MATERIALIZE" --output "$destination"

  [ "$status" -ne 0 ]
  [[ "$output" == *"output directory is not empty"* ]]
  [ "$(cat "$destination/user-file")" = preserve ]
}
