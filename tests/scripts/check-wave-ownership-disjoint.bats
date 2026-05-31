#!/usr/bin/env bats

# Tests for scripts/check-wave-ownership-disjoint.sh — the ag-lmdx.6 wave pre-gate
# that asserts each slice's owned node-state set is pairwise-disjoint BEFORE a
# parallel wave of agents spawns. Hermetic: builds manifests in a tmpdir, no
# network, no git, no live ao/bd.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-wave-ownership-disjoint.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Scenario: Overlapping node ownership blocks the wave.
@test "overlapping node-state ownership FAILS naming the contended artifact" {
    cat > "$TMP_DIR/wave.json" <<'JSON'
[
  {"id": "slice-1", "subject": "advance A", "owns": ["artifact-A", "artifact-B"]},
  {"id": "slice-2", "subject": "advance A too", "owns": ["artifact-A", "artifact-C"]}
]
JSON
    run bash "$SCRIPT" "$TMP_DIR/wave.json"

    [ "$status" -eq 1 ]
    [[ "$output" == *"CONFLICT: node-state 'artifact-A'"* ]]
    [[ "$output" == *"slice-1"* ]]
    [[ "$output" == *"slice-2"* ]]
    [[ "$output" == *"wave NOT spawned"* ]]
}

# Scenario: Disjoint ownership proceeds.
@test "disjoint node-state ownership PASSES and wave may spawn" {
    cat > "$TMP_DIR/wave.json" <<'JSON'
[
  {"id": "slice-1", "subject": "advance A", "owns": ["artifact-A", "artifact-B"]},
  {"id": "slice-2", "subject": "advance C", "owns": ["artifact-C", "artifact-D"]}
]
JSON
    run bash "$SCRIPT" "$TMP_DIR/wave.json"

    [ "$status" -eq 0 ]
    [[ "$output" == *"Wave pre-gate PASSED"* ]]
    [[ "$output" == *"2 slice(s)"* ]]
}

@test "reads the manifest from stdin" {
    run bash -c "printf '%s' '[{\"id\":\"s1\",\"owns\":[\"x\"]},{\"id\":\"s2\",\"owns\":[\"x\"]}]' | bash '$SCRIPT' -"
    [ "$status" -eq 1 ]
    [[ "$output" == *"CONFLICT: node-state 'x'"* ]]
}

@test "stdin disjoint passes" {
    run bash -c "printf '%s' '[{\"id\":\"s1\",\"owns\":[\"x\"]},{\"id\":\"s2\",\"owns\":[\"y\"]}]' | bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Wave pre-gate PASSED"* ]]
}

@test "empty manifest array is a no-op skip (exit 0)" {
    echo '[]' > "$TMP_DIR/wave.json"
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP: empty wave manifest"* ]]
}

@test "slice with no declared ownership is BLOCKED (cannot prove disjointness)" {
    cat > "$TMP_DIR/wave.json" <<'JSON'
[
  {"id": "slice-1", "subject": "advance A", "owns": ["artifact-A"]},
  {"id": "slice-2", "subject": "undeclared"}
]
JSON
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 1 ]
    [[ "$output" == *"BLOCKED: slice slice-2 declares no node-state ownership"* ]]
}

@test "non-array JSON is an input error (exit 2)" {
    echo '{"id":"s1","owns":["x"]}' > "$TMP_DIR/wave.json"
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 2 ]
    [[ "$output" == *"must be a JSON array"* ]]
}

@test "unparseable input is an input error (exit 2)" {
    echo 'not json' > "$TMP_DIR/wave.json"
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 2 ]
    [[ "$output" == *"not valid JSON"* ]]
}

@test "missing manifest file is an input error (exit 2)" {
    run bash "$SCRIPT" "$TMP_DIR/does-not-exist.json"
    [ "$status" -eq 2 ]
    [[ "$output" == *"manifest not found"* ]]
}

@test "duplicate node within the same slice WARNs but does not block" {
    cat > "$TMP_DIR/wave.json" <<'JSON'
[
  {"id": "slice-1", "subject": "dup", "owns": ["artifact-A", "artifact-A"]},
  {"id": "slice-2", "subject": "other", "owns": ["artifact-B"]}
]
JSON
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN: slice slice-1 lists node-state 'artifact-A' more than once"* ]]
    [[ "$output" == *"Wave pre-gate PASSED"* ]]
}

@test "three-slice wave with a single overlapping pair FAILS" {
    cat > "$TMP_DIR/wave.json" <<'JSON'
[
  {"id": "a", "owns": ["n1", "n2"]},
  {"id": "b", "owns": ["n3"]},
  {"id": "c", "owns": ["n2", "n4"]}
]
JSON
    run bash "$SCRIPT" "$TMP_DIR/wave.json"
    [ "$status" -eq 1 ]
    [[ "$output" == *"CONFLICT: node-state 'n2'"* ]]
    [[ "$output" == *"slice a"* ]]
    [[ "$output" == *"slice c"* ]]
}
