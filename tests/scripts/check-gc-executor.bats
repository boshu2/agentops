#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "Gas City executor static contract is green" {
  run "$REPO_ROOT/scripts/check-gc-executor.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Gas City executor and bead-native factory static contract: PASS"* ]]
}

@test "GC command result schema cannot carry a semantic verdict field" {
  run python3 - "$REPO_ROOT/packs/agentops-executor/commands/run-packet/schemas/result.schema.json" <<'PY'
import json
import sys

schema = json.load(open(sys.argv[1], encoding="utf-8"))
serialized = json.dumps(schema, sort_keys=True)
assert '"verdict"' not in serialized
PY
  [ "$status" -eq 0 ]
}
