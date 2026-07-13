#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-local.sh"
TMP_DIR=""

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT

assert_contains() {
  local pattern="$1"
  local label="$2"

  if rg -q "$pattern" "$SCRIPT"; then
    pass "$label"
  else
    fail "$label"
  fi
}

echo "== test-validate-local =="
assert_contains 'install-dev-hooks\.sh' "validate-local references repo hook bootstrap"
assert_contains 'core\.hooksPath' "validate-local warns when core.hooksPath is not .githooks"
assert_contains 'Manual Local Validation' "validate-local describes itself as manual validation"
assert_contains 'gate check' "validate-local wraps the Go push gate"
assert_contains 'agentops-validate-local\.lock' "validate-local serializes concurrent runs"

TMP_DIR="$(mktemp -d)"
FAKE_REPO="$TMP_DIR/repo"
mkdir -p "$FAKE_REPO/scripts"
cp "$SCRIPT" "$FAKE_REPO/scripts/validate-local.sh"
chmod +x "$FAKE_REPO/scripts/validate-local.sh"
cat > "$FAKE_REPO/scripts/ao" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
# Fake ao for lock serialization test — sleep so the lock is held.
if [[ "${1:-}" == "gate" ]]; then
  sleep 5
  exit 0
fi
exit 0
EOF
chmod +x "$FAKE_REPO/scripts/ao"
mkdir -p "$FAKE_REPO/cli/bin"
ln -sf ../../scripts/ao "$FAKE_REPO/cli/bin/ao"

git -C "$TMP_DIR" init repo >/dev/null 2>&1
git -C "$FAKE_REPO" config core.hooksPath .githooks

(
  cd "$FAKE_REPO"
  bash scripts/validate-local.sh --skip-claude >/dev/null 2>&1
) &
FIRST_PID=$!

LOCK_DIR="$FAKE_REPO/.git/agentops-validate-local.lock"
for _ in $(seq 1 50); do
  [[ -d "$LOCK_DIR" ]] && break
  sleep 0.1
done

if [[ -d "$LOCK_DIR" ]]; then
  set +e
  SECOND_OUTPUT="$(
    cd "$FAKE_REPO" &&
      bash scripts/validate-local.sh --skip-claude 2>&1
  )"
  SECOND_STATUS=$?
  set -e

  if [[ "$SECOND_STATUS" -eq 1 && "$SECOND_OUTPUT" == *"already running"* ]]; then
    pass "validate-local rejects concurrent runs with a clear lock error"
  else
    fail "validate-local rejects concurrent runs with a clear lock error"
  fi
else
  fail "validate-local creates its repo-scoped lock before running the shared gate"
fi

wait "$FIRST_PID"

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
