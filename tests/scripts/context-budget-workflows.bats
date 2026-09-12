#!/usr/bin/env bats

setup() {
  command -v node >/dev/null || skip "node is required"
  HARNESS="$BATS_TEST_DIRNAME/../fixtures/context-budget-workflows.mjs"
}

@test "bulk-read: one bounded cheap reader per file and bounded result caps" {
  run node "$HARNESS" reader-normal
  [ "$status" -eq 0 ]
}

@test "bulk-read: malformed or content-bearing results become explicit unknown coverage" {
  run node "$HARNESS" reader-output
  [ "$status" -eq 0 ]
}

@test "context workflows: null args and non-integer limits fail before dispatch" {
  run node "$HARNESS" invalid-args
  [ "$status" -eq 0 ]
}

@test "code-write: distinct batches use real metadata then sequential cheap writers" {
  run node "$HARNESS" writer-normal
  [ "$status" -eq 0 ]
}

@test "code-write: missing references never start a worker" {
  run node "$HARNESS" required-reference
  [ "$status" -eq 0 ]
}

@test "code-write: path, symlink, hardlink, and missing-parent aliases start no writers" {
  run node "$HARNESS" target-aliases
  [ "$status" -eq 0 ]
}

@test "code-write: unavailable or malformed target metadata starts no writers" {
  run node "$HARNESS" target-failures
  [ "$status" -eq 0 ]
}

@test "code-write: absent case-variant targets are rejected before any writer" {
  run node "$HARNESS" target-case-aliases
  [ "$status" -eq 0 ]
}

@test "code-write: metadata probe shell-quotes paths and rejects dangling links" {
  run node "$HARNESS" target-probe
  [ "$status" -eq 0 ]
}

@test "code-write: raw check output and oversized receipts never reach the caller" {
  run node "$HARNESS" writer-output
  [ "$status" -eq 0 ]
}

@test "code-write: writing then crashing reports unknown side effects" {
  run node "$HARNESS" writer-crash
  [ "$status" -eq 0 ]
}
