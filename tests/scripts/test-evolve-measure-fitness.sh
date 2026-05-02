#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/evolve-measure-fitness.sh"
REAL_JQ="$(command -v jq)"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

make_timeout_stub() {
  local bin_dir="$1"
  cat > "$bin_dir/timeout" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
seconds="$1"
shift
if [[ "${MOCK_TIMEOUT_MODE:-pass}" == "timeout" ]]; then
  exit 124
fi
exec "$@"
EOF
  chmod +x "$bin_dir/timeout"
}

make_jq_stub() {
  local bin_dir="$1"
  cat > "$bin_dir/jq" <<'EOF'
#!/usr/bin/env bash
exec "__REAL_JQ__" "$@"
EOF
  perl -0pi -e 's|__REAL_JQ__|'"$REAL_JQ"'|g' "$bin_dir/jq"
  chmod +x "$bin_dir/jq"
}

make_ao_stub() {
  local bin_dir="$1"
  cat > "$bin_dir/ao" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" > "${MOCK_AO_ARGS_FILE:?}"

mode="${MOCK_AO_MODE:-pass}"
case "$mode" in
  pass)
    cat <<'JSON'
{"goals":[{"goal_id":"g1","result":"pass","duration_s":0.1,"weight":1}],"summary":{"total":1,"passing":1,"failing":0,"skipped":0,"score":100}}
JSON
    ;;
  invalid)
    printf '{"summary":{"total":1}}\n'
    ;;
  fail)
    exit 1
    ;;
  *)
    echo "unknown MOCK_AO_MODE: $mode" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$bin_dir/ao"
}

assert_file_contains() {
  local path="$1"
  local expected="$2"
  if grep -Fq -- "$expected" "$path"; then
    return 0
  fi
  echo "--- file: $path ---" >&2
  cat "$path" >&2
  return 1
}

test_script_exists() {
  if [[ -x "$SCRIPT" ]]; then
    pass "evolve fitness wrapper exists and is executable"
  else
    fail "evolve fitness wrapper exists and is executable"
  fi
}

test_success_writes_atomically() {
  local repo="$TMP_DIR/success"
  local bin_dir="$repo/bin"
  mkdir -p "$bin_dir" "$repo/.agents/evolve"
  make_timeout_stub "$bin_dir"
  make_jq_stub "$bin_dir"
  make_ao_stub "$bin_dir"

  local args_file="$repo/ao-args.txt"
  local output="$repo/.agents/evolve/fitness-latest.json"
  printf 'old\n' > "$output"

  if PATH="$bin_dir:$PATH" MOCK_AO_ARGS_FILE="$args_file" bash "$SCRIPT" \
    --repo-root "$repo" \
    --output .agents/evolve/fitness-latest.json >"$repo/run.log" 2>&1; then
    if assert_file_contains "$output" '"goals"' &&
      assert_file_contains "$args_file" 'goals measure --json --timeout 60 --total-timeout 75'; then
      pass "successful run writes validated JSON to output"
    else
      fail "successful run writes validated JSON to output"
    fi
  else
    cat "$repo/run.log" >&2
    fail "successful run writes validated JSON to output"
  fi
}

test_invalid_json_preserves_previous_snapshot() {
  local repo="$TMP_DIR/invalid"
  local bin_dir="$repo/bin"
  mkdir -p "$bin_dir" "$repo/.agents/evolve"
  make_timeout_stub "$bin_dir"
  make_jq_stub "$bin_dir"
  make_ao_stub "$bin_dir"

  local args_file="$repo/ao-args.txt"
  local output="$repo/.agents/evolve/fitness-latest.json"
  printf '{"goals":[{"goal_id":"old"}]}\n' > "$output"

  if PATH="$bin_dir:$PATH" MOCK_AO_MODE=invalid MOCK_AO_ARGS_FILE="$args_file" bash "$SCRIPT" \
    --repo-root "$repo" \
    --output .agents/evolve/fitness-latest.json >"$repo/run.log" 2>&1; then
    fail "invalid JSON fails without clobbering previous snapshot"
  elif assert_file_contains "$output" '"goal_id":"old"' && ! [[ -s "$output" && "$(cat "$output")" == '{"summary":{"total":1}}' ]]; then
    pass "invalid JSON fails without clobbering previous snapshot"
  else
    cat "$repo/run.log" >&2
    fail "invalid JSON fails without clobbering previous snapshot"
  fi
}

test_command_failure_preserves_previous_snapshot() {
  local repo="$TMP_DIR/fail"
  local bin_dir="$repo/bin"
  mkdir -p "$bin_dir" "$repo/.agents/evolve"
  make_timeout_stub "$bin_dir"
  make_jq_stub "$bin_dir"
  make_ao_stub "$bin_dir"

  local args_file="$repo/ao-args.txt"
  local output="$repo/.agents/evolve/fitness-latest.json"
  printf '{"goals":[{"goal_id":"old"}]}\n' > "$output"

  if PATH="$bin_dir:$PATH" MOCK_AO_MODE=fail MOCK_AO_ARGS_FILE="$args_file" bash "$SCRIPT" \
    --repo-root "$repo" \
    --output .agents/evolve/fitness-latest.json >"$repo/run.log" 2>&1; then
    fail "command failure preserves previous snapshot"
  elif assert_file_contains "$output" '"goal_id":"old"' && assert_file_contains "$repo/run.log" 'goals measurement failed'; then
    pass "command failure preserves previous snapshot"
  else
    cat "$repo/run.log" >&2
    fail "command failure preserves previous snapshot"
  fi
}

test_goal_passthrough() {
  local repo="$TMP_DIR/goal"
  local bin_dir="$repo/bin"
  mkdir -p "$bin_dir" "$repo/.agents/evolve"
  make_timeout_stub "$bin_dir"
  make_jq_stub "$bin_dir"
  make_ao_stub "$bin_dir"

  local args_file="$repo/ao-args.txt"

  if PATH="$bin_dir:$PATH" MOCK_AO_ARGS_FILE="$args_file" bash "$SCRIPT" \
    --repo-root "$repo" \
    --output .agents/evolve/fitness-latest-post.json \
    --goal target-goal \
    --timeout 12 \
    --total-timeout 20 >"$repo/run.log" 2>&1; then
    if assert_file_contains "$args_file" '--goal target-goal' && assert_file_contains "$args_file" '--timeout 12'; then
      pass "goal flag is passed through to ao goals measure"
    else
      fail "goal flag is passed through to ao goals measure"
    fi
  else
    cat "$repo/run.log" >&2
    fail "goal flag is passed through to ao goals measure"
  fi
}

echo "== test-evolve-measure-fitness =="
test_script_exists
test_success_writes_atomically
test_invalid_json_preserves_previous_snapshot
test_command_failure_preserves_previous_snapshot
test_goal_passthrough

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
