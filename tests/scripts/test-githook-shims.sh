#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

assert_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"

  if rg -q "$pattern" "$file"; then
    pass "$label"
  else
    fail "$label"
  fi
}

echo "== test-githook-shims =="
assert_contains "$ROOT/.githooks/pre-commit" 'bd hooks run pre-commit' "pre-commit delegates via bd hooks run"
assert_contains "$ROOT/.githooks/pre-push" 'bd hooks run pre-push' "pre-push delegates via bd hooks run"
# Shared-gate assertions retired by soc-g2r9 (PR #357): CI is the sole
# authoritative push gate; the local pre-push hook delegates only to bd.
# See docs/contracts/local-pre-push-gate-retirement.md.
assert_contains "$ROOT/.githooks/pre-push" 'HOOK_STDIN_FILE="\$\(mktemp\)"' "pre-push captures hook stdin before running bd hooks"
assert_contains "$ROOT/.githooks/pre-push" 'run_without_git_env bd hooks run pre-push "\$@" <"\$HOOK_STDIN_FILE"' "pre-push replays saved stdin to bd hooks under clean git env"
assert_contains "$ROOT/.githooks/post-merge" 'bd hooks run post-merge' "post-merge delegates via bd hooks run"
assert_contains "$ROOT/.githooks/post-checkout" 'bd hooks run post-checkout' "post-checkout delegates via bd hooks run"
assert_contains "$ROOT/.githooks/prepare-commit-msg" 'bd hooks run prepare-commit-msg' "prepare-commit-msg delegates via bd hooks run"

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
