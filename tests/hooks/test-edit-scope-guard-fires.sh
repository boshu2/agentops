#!/bin/bash
# tests/hooks/test-edit-scope-guard-fires.sh
#
# L2 hook activation test for hooks/edit-scope-guard.sh (pre-mortem Finding 2).
# Verifies the hook ACTUALLY fires post-install:
#   1. Path inside frozen scope → exit 0 (allowed)
#   2. Path outside frozen scope → exit non-zero (blocked)
#   3. Malformed JSON input → exit 0 (fail-open per Finding 3)
#   4. Missing lock file → exit 0 (no enforcement)
#   5. Empty frozen_dirs → exit 0 (no enforcement)
#   6. Missing target path → exit 0 (nothing to check)
#
# NOTE on pre-mortem Finding 2 pseudocode: the doc's second case asserted
# "unprotected/foo.go ⇒ exit 0", but the spec contract says edits OUTSIDE every
# frozen dir are blocked. The implementer reconciled to spec semantics
# (positive allow-list); see SKILL.md "Behavior Contract".
#
# Run from repo root: bash tests/hooks/test-edit-scope-guard-fires.sh

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOOK="$REPO_ROOT/hooks/edit-scope-guard.sh"

if [ ! -x "$HOOK" ]; then
  chmod +x "$HOOK" 2>/dev/null || true
fi

TEST_REPO=$(mktemp -d)
trap 'rm -rf "$TEST_REPO"' EXIT
mkdir -p "$TEST_REPO/.agents"

# Tell the hook where to find the lock without depending on `git rev-parse`.
export AO_SCOPE_LOCK_ROOT="$TEST_REPO"
export AO_SCOPE_LOCK="$TEST_REPO/.agents/scope.lock"

PASS=0
FAIL=0

assert_zero() {
  local label="$1"
  local got="$2"
  if [ "$got" -eq 0 ]; then
    echo "PASS: $label (exit 0)"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $label — want exit 0, got $got"
    FAIL=$((FAIL + 1))
  fi
}

assert_nonzero() {
  local label="$1"
  local got="$2"
  if [ "$got" -ne 0 ]; then
    echo "PASS: $label (exit $got)"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $label — want non-zero, got 0"
    FAIL=$((FAIL + 1))
  fi
}

# Seed lock with a single frozen dir.
seed_lock_protected() {
  cat > "$AO_SCOPE_LOCK" <<'JSON'
{
  "schema_version": 1,
  "frozen_dirs": ["protected/"],
  "acquired_at": "2026-05-01T19:30:00Z",
  "acquired_by": "test"
}
JSON
}

# --- Case 1: edit inside frozen scope → allowed ---
seed_lock_protected
echo '{"tool":{"name":"Edit","params":{"file_path":"protected/foo.go"}}}' | bash "$HOOK" 2>/dev/null
assert_zero "edit inside frozen scope allowed" "$?"

# --- Case 2: edit outside frozen scope → blocked ---
echo '{"tool":{"name":"Edit","params":{"file_path":"unprotected/foo.go"}}}' | bash "$HOOK" 2>/dev/null
assert_nonzero "edit outside frozen scope blocked" "$?"

# --- Case 3: malformed JSON input (Finding 3 fail-open) ---
echo '{"malformed' | bash "$HOOK" 2>/dev/null
assert_zero "malformed JSON fails open" "$?"

# --- Case 4: missing lock file → allow ---
rm -f "$AO_SCOPE_LOCK"
echo '{"tool":{"name":"Edit","params":{"file_path":"anywhere/foo.go"}}}' | bash "$HOOK" 2>/dev/null
assert_zero "missing lock file allows edit" "$?"

# --- Case 5: empty frozen_dirs → allow ---
cat > "$AO_SCOPE_LOCK" <<'JSON'
{
  "schema_version": 1,
  "frozen_dirs": [],
  "acquired_at": "2026-05-01T19:30:00Z",
  "acquired_by": "test"
}
JSON
echo '{"tool":{"name":"Edit","params":{"file_path":"anywhere/foo.go"}}}' | bash "$HOOK" 2>/dev/null
assert_zero "empty frozen_dirs allows edit" "$?"

# --- Case 6: missing target path → allow ---
echo '{"tool":{"name":"Edit","params":{}}}' | bash "$HOOK" 2>/dev/null
assert_zero "missing target path allows edit" "$?"

# --- Case 7: subdir of frozen dir → allowed ---
seed_lock_protected
echo '{"tool":{"name":"Edit","params":{"file_path":"protected/sub/bar.go"}}}' | bash "$HOOK" 2>/dev/null
assert_zero "edit in nested frozen subdir allowed" "$?"

echo
echo "===== summary: $PASS passed, $FAIL failed ====="
[ "$FAIL" -eq 0 ] || exit 1
exit 0
