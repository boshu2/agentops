#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GATE_SCRIPT="$REPO_ROOT/scripts/check-contract-compatibility.sh"

PASS_COUNT=0
FAIL_COUNT=0
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

pass() {
  echo "PASS: $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

create_fixture() {
  local fixture="$TMP_DIR/fixture-$1"
  mkdir -p "$fixture/docs/contracts" "$fixture/scripts"

  cat >"$fixture/docs/documentation-index.md" <<'EOF'
# Contract Index

- [Known Contract](contracts/known-contract.md)
EOF

  cat >"$fixture/docs/contracts/known-contract.md" <<'EOF'
# Known Contract
EOF

  cat >"$fixture/scripts/contract-orphans-allowlist.txt" <<'EOF'
# Format: path | reason | owner | expires
EOF

  echo "$fixture"
}

run_gate() {
  local target_root="$1"
  local output_file="$2"

  set +e
  "$GATE_SCRIPT" "$target_root" >"$output_file" 2>&1
  local status=$?
  set -e
  return "$status"
}

assert_gate_passes() {
  local description="$1"
  local target_root="$2"
  local output_file
  output_file="$(mktemp "$TMP_DIR/output-pass-XXXXXX")"

  if run_gate "$target_root" "$output_file"; then
    pass "$description"
  else
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description"
  fi
}

assert_gate_fails_with() {
  local description="$1"
  local target_root="$2"
  local expected="$3"
  local output_file
  output_file="$(mktemp "$TMP_DIR/output-fail-XXXXXX")"

  if run_gate "$target_root" "$output_file"; then
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description (expected failure, got success)"
    return
  fi

  if grep -Fq "$expected" "$output_file"; then
    pass "$description"
  else
    echo "--- output ($description) ---"
    cat "$output_file"
    fail "$description (missing expected text: $expected)"
  fi
}

test_script_executable() {
  if [[ -x "$GATE_SCRIPT" ]]; then
    pass "check-contract-compatibility.sh is executable"
  else
    fail "check-contract-compatibility.sh is not executable"
  fi
}

test_repo_baseline_passes() {
  assert_gate_passes "repository baseline passes gate" "$REPO_ROOT"
}

test_orphan_fails_without_allowlist() {
  local fixture
  fixture="$(create_fixture "orphan-fail")"
  cat >"$fixture/docs/contracts/orphan-contract.md" <<'EOF'
# Orphan Contract
EOF

  assert_gate_fails_with \
    "orphan fails when not catalogued and not allowlisted" \
    "$fixture" \
    "docs/contracts/orphan-contract.md exists on disk but not in documentation-index.md (not allowlisted)"
}

test_orphan_passes_with_allowlist_entry() {
  local fixture
  fixture="$(create_fixture "orphan-allow")"
  cat >"$fixture/docs/contracts/orphan-contract.md" <<'EOF'
# Orphan Contract
EOF
  cat >>"$fixture/scripts/contract-orphans-allowlist.txt" <<'EOF'
docs/contracts/orphan-contract.md | awaiting index update | @docs-team | 2099-12-31
EOF

  assert_gate_passes "allowlisted orphan passes" "$fixture"
}

test_malformed_allowlist_fails() {
  local fixture
  fixture="$(create_fixture "malformed")"
  cat >>"$fixture/scripts/contract-orphans-allowlist.txt" <<'EOF'
docs/contracts/orphan-contract.md | missing fields only
EOF

  assert_gate_fails_with \
    "malformed allowlist entry fails gate" \
    "$fixture" \
    "malformed (expected: path | reason | owner | expires)"
}

test_wildcard_allowlist_fails() {
  local fixture
  fixture="$(create_fixture "wildcard")"
  cat >>"$fixture/scripts/contract-orphans-allowlist.txt" <<'EOF'
docs/contracts/*.md | broad exception | @docs-team | 2099-12-31
EOF

  assert_gate_fails_with \
    "wildcard allowlist path is rejected" \
    "$fixture" \
    "path contains wildcard"
}

test_stale_allowlist_fails() {
  local fixture
  fixture="$(create_fixture "stale")"
  cat >>"$fixture/scripts/contract-orphans-allowlist.txt" <<'EOF'
docs/contracts/known-contract.md | stale allowlist entry | @docs-team | 2099-12-31
EOF

  assert_gate_fails_with \
    "allowlist entry for catalogued contract fails" \
    "$fixture" \
    "allowlist entry is stale (already catalogued in documentation-index.md)"
}

echo "================================"
echo "Testing contract compatibility gate"
echo "================================"
echo ""

test_script_executable
test_repo_baseline_passes
test_orphan_fails_without_allowlist
test_orphan_passes_with_allowlist_entry
test_malformed_allowlist_fails
test_wildcard_allowlist_fails
test_stale_allowlist_fails

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi
exit 0
