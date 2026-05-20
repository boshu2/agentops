#!/usr/bin/env bats
# Regression tests for scripts/auto-resume-stale-claims.sh (soc-vuu6.27 slice 4a).
#
# The script shells out to `ao` and `jq`. We stub `ao` via PATH so the test
# is deterministic and doesn't depend on a real ao binary. `jq` we use real
# (it's a hard dep on every dev box; we even bats-test other scripts that
# use it).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/auto-resume-stale-claims.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin"
  # Resume-call log lives at $TMP/resume-calls.log so tests can assert on it.
  RESUME_LOG="$TMP/resume-calls.log"
  : > "$RESUME_LOG"
  export RESUME_LOG
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# stub_ao writes a fake `ao` binary that:
#   - on `ao beads stale-claims ...` outputs $1 (the canned JSON)
#   - on `ao beads resume <id> ...` appends the id to $RESUME_LOG
#   - on `ao beads resume <id-that-fails>` (any id starting with "fail-")
#     exits 1 to simulate a transfer failure
stub_ao() {
  local canned_json="$1"
  cat >"$TMP/bin/ao" <<EOF
#!/usr/bin/env bash
if [ "\$1" = "beads" ] && [ "\$2" = "stale-claims" ]; then
  printf '%s' '$canned_json'
  exit 0
fi
if [ "\$1" = "beads" ] && [ "\$2" = "resume" ]; then
  echo "\$3" >> "$RESUME_LOG"
  case "\$3" in
    fail-*) exit 1 ;;
    *)      exit 0 ;;
  esac
fi
exit 0
EOF
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$ORIG_PATH"
}

run_driver() {
  run "$SCRIPT" "$@"
}

@test "zero stale claims = exit 0, friendly message, no resume calls" {
  stub_ao '[]'
  run_driver
  [ "$status" -eq 0 ]
  [[ "$output" == *"no stale claims"* ]]
  [ "$(wc -l < "$RESUME_LOG" | tr -d ' ')" -eq 0 ]
}

@test "two stale claims = two resume calls, exit 0" {
  stub_ao '[{"bead_id":"soc-a.1"},{"bead_id":"soc-b.2"}]'
  run_driver
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 stale claim"* ]]
  [[ "$output" == *"2 succeeded, 0 failed"* ]]
  grep -qx 'soc-a.1' "$RESUME_LOG"
  grep -qx 'soc-b.2' "$RESUME_LOG"
}

@test "--dry-run does not invoke resume" {
  stub_ao '[{"bead_id":"soc-x.1"},{"bead_id":"soc-y.2"}]'
  run_driver --dry-run
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY-RUN"* ]]
  [ "$(wc -l < "$RESUME_LOG" | tr -d ' ')" -eq 0 ]
}

@test "one resume fails = exit 1, OK ids still transferred" {
  stub_ao '[{"bead_id":"soc-a.1"},{"bead_id":"fail-bead"},{"bead_id":"soc-b.2"}]'
  run_driver
  [ "$status" -eq 1 ]
  [[ "$output" == *"1 failed"* ]] || [[ "${stderr:-$output}" == *"FAIL"* ]]
  # The two OK beads should still have been attempted.
  grep -qx 'soc-a.1' "$RESUME_LOG"
  grep -qx 'soc-b.2' "$RESUME_LOG"
}

@test "--max caps the number of transfers" {
  # 4 candidates, --max 2 → only 2 transfers, 2 over-cap.
  stub_ao '[{"bead_id":"soc-a"},{"bead_id":"soc-b"},{"bead_id":"soc-c"},{"bead_id":"soc-d"}]'
  run_driver --max 2
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 succeeded"* ]]
  [[ "$output" == *"2 over-cap"* ]]
  [ "$(wc -l < "$RESUME_LOG" | tr -d ' ')" -eq 2 ]
}

@test "--threshold gets passed through (and ao stub receives it)" {
  # We can't easily check what args ao saw without a more complex stub;
  # instead, assert that running with a non-default threshold doesn't break.
  stub_ao '[{"bead_id":"soc-x"}]'
  run_driver --threshold 12
  [ "$status" -eq 0 ]
  [[ "$output" == *"threshold 12h"* ]]
}

@test "--quiet suppresses OK lines, keeps summary" {
  stub_ao '[{"bead_id":"soc-a"},{"bead_id":"soc-b"}]'
  run_driver --quiet
  [ "$status" -eq 0 ]
  [[ "$output" != *"OK   soc-a"* ]]
  [[ "$output" == *"succeeded"* ]]
}

@test "unknown flag exits 2" {
  stub_ao '[]'
  run_driver --weasel
  [ "$status" -eq 2 ]
}

@test "missing ao binary exits 2 with clear error" {
  # Make a coreutils-only PATH that excludes ao.
  mkdir -p "$TMP/coreutils-only"
  for cmd in bash sh sed awk grep tr sort cat mkdir mv rm cp ls wc dirname basename head tail printf jq mktemp; do
    full="$(command -v "$cmd" 2>/dev/null || true)"
    [ -n "$full" ] && ln -sf "$full" "$TMP/coreutils-only/$cmd"
  done
  export PATH="$TMP/coreutils-only"
  run_driver
  [ "$status" -eq 2 ]
  [[ "$output" == *"ao CLI not on PATH"* ]]
}
