#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/check-codex-parity-drift.sh"

PASS=0
FAIL=0
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

create_fixture() {
  local repo="$1"
  mkdir -p "$repo/scripts"
  git -C "$repo" init -q >/dev/null 2>&1
  /bin/cp "$SCRIPT" "$repo/scripts/check-codex-parity-drift.sh"
  chmod +x "$repo/scripts/check-codex-parity-drift.sh"
}

test_shell_wrapper_path() {
  local repo="$TMP_DIR/shell-wrapper"
  create_fixture "$repo"

  cat > "$repo/scripts/audit-codex-parity.sh" <<'EOF'
#!/usr/bin/env bash
echo "PASS: clean"
EOF
  chmod +x "$repo/scripts/audit-codex-parity.sh"

  if (cd "$repo" && bash scripts/check-codex-parity-drift.sh >/dev/null); then
    pass "uses shell wrapper when present"
  else
    fail "shell wrapper path should pass"
  fi
}

test_python_fallback_executes_python() {
  local repo="$TMP_DIR/python-fallback"
  local output=""
  create_fixture "$repo"

  cat > "$repo/scripts/audit-codex-parity.py" <<'EOF'
#!/usr/bin/env python3
import sys
print("DRIFT: python fallback executed")
sys.exit(1)
EOF

  if output="$(cd "$repo" && bash scripts/check-codex-parity-drift.sh 2>&1)"; then
    echo "$output"
    fail "python fallback should fail when the Python audit reports drift"
    return
  fi

  if [[ "$output" == *"DRIFT: python fallback executed"* ]]; then
    pass "python fallback executes with python3 instead of bash"
  else
    echo "$output"
    fail "python fallback output missing drift sentinel"
  fi
}

echo "== test-codex-parity-drift =="
test_shell_wrapper_path
test_python_fallback_executes_python

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
