#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/check-casefold-path-collisions.sh"

PASS=0
FAIL=0
TMP_DIR="$(mktemp -d)"
PASS_OUT="$TMP_DIR/casefold-pass.out"
FAIL_OUT="$TMP_DIR/casefold-fail.out"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

pass() {
  echo "PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL=$((FAIL + 1))
}

assert_passes() {
  local description="$1"
  local list_file="$2"
  if "$SCRIPT" --path-list "$list_file" >"$PASS_OUT" 2>&1; then
    pass "$description"
  else
    cat "$PASS_OUT"
    fail "$description"
  fi
}

assert_fails_with() {
  local description="$1"
  local list_file="$2"
  local expected="$3"
  set +e
  "$SCRIPT" --path-list "$list_file" >"$FAIL_OUT" 2>&1
  local status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    cat "$FAIL_OUT"
    fail "$description (expected failure)"
    return
  fi
  if grep -Fq "$expected" "$FAIL_OUT"; then
    pass "$description"
  else
    cat "$FAIL_OUT"
    fail "$description (missing expected output: $expected)"
  fi
}

if [[ -x "$SCRIPT" ]]; then
  pass "check-casefold-path-collisions.sh is executable"
else
  fail "check-casefold-path-collisions.sh is executable"
fi

clean_list="$TMP_DIR/clean.txt"
cat >"$clean_list" <<'EOF'
docs/index.md
docs/documentation-index.md
scripts/check-casefold-path-collisions.sh
EOF
assert_passes "distinct lowercase paths pass" "$clean_list"

collision_list="$TMP_DIR/collision.txt"
cat >"$collision_list" <<'EOF'
docs/INDEX.md
docs/index.md
README.md
EOF
assert_fails_with "case-folded docs index collision fails" "$collision_list" "docs/INDEX.md"
assert_fails_with "case-folded docs index collision reports lowercase peer" "$collision_list" "docs/index.md"

echo
echo "Results: $PASS PASS, $FAIL FAIL"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
