#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

PASS_COUNT=0
FAIL_COUNT=0
MOCK_TOOLCHAIN="$(mktemp)"

cleanup() {
  rm -f "$MOCK_TOOLCHAIN"
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

create_mock_toolchain() {
  cat >"$MOCK_TOOLCHAIN" <<'MOCK'
#!/bin/bash
cat <<'JSON'
{
  "timestamp": "2026-02-19T00:00:00Z",
  "target": "/tmp/repo",
  "tools_run": 2,
  "tools_skipped": 9,
  "tools": {
    "ruff": "pass",
    "golangci-lint": "pass",
    "gitleaks": "not_installed",
    "shellcheck": "pass",
    "radon": "skipped",
    "semgrep": "not_installed",
    "trivy": "not_installed",
    "gosec": "pass",
    "hadolint": "skipped",
    "pytest": "skipped",
    "go-test": "skipped"
  },
  "findings": {
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0
  },
  "gate_status": "PASS",
  "output_dir": "/tmp/agentops-tooling"
}
JSON
exit 0
MOCK
  chmod +x "$MOCK_TOOLCHAIN"
}

test_executable() {
  if [[ -x "scripts/security-gate.sh" ]]; then
    pass "security-gate.sh is executable"
  else
    fail "security-gate.sh is not executable"
  fi
}

test_help() {
  if scripts/security-gate.sh --help >/dev/null 2>&1; then
    pass "--help works"
  else
    fail "--help failed"
  fi
}

test_invalid_mode() {
  if scripts/security-gate.sh --mode nope >/dev/null 2>&1; then
    fail "invalid mode should fail"
  else
    pass "invalid mode fails"
  fi
}

test_json_output() {
  local output
  create_mock_toolchain
  output=$(SECURITY_GATE_TOOLCHAIN_SCRIPT="$MOCK_TOOLCHAIN" scripts/security-gate.sh --mode quick --json 2>/dev/null || true)

  if echo "$output" | jq empty >/dev/null 2>&1; then
    pass "JSON output is valid"
  else
    fail "JSON output is invalid"
    return
  fi

  local has_mode has_gate has_toolchain
  has_mode=$(echo "$output" | jq -e '.mode' >/dev/null 2>&1 && echo yes || echo no)
  has_gate=$(echo "$output" | jq -e '.gate_status' >/dev/null 2>&1 && echo yes || echo no)
  has_toolchain=$(echo "$output" | jq -e '.toolchain.findings' >/dev/null 2>&1 && echo yes || echo no)

  if [[ "$has_mode" == "yes" && "$has_gate" == "yes" && "$has_toolchain" == "yes" ]]; then
    pass "JSON output has required fields"
  else
    fail "JSON output missing required fields (mode=$has_mode gate=$has_gate toolchain=$has_toolchain)"
  fi
}

test_artifacts() {
  create_mock_toolchain
  local test_output_dir
  test_output_dir="$(mktemp -d)"
  SECURITY_GATE_TOOLCHAIN_SCRIPT="$MOCK_TOOLCHAIN" \
  SECURITY_GATE_OUTPUT_DIR="$test_output_dir/security" \
  TOOLCHAIN_OUTPUT_DIR="$test_output_dir/tooling" \
  scripts/security-gate.sh --mode quick >/dev/null 2>&1 || true

  local latest
  latest=$(ls -td "$test_output_dir/security"/* 2>/dev/null | head -1 || true)

  if [[ -z "$latest" ]]; then
    fail "no security artifacts created"
    rm -rf "$test_output_dir"
    return
  fi

  if [[ -f "$latest/security-gate-summary.json" ]]; then
    pass "security-gate summary artifact created"
  else
    fail "missing security-gate-summary.json"
  fi
  rm -rf "$test_output_dir"
}

echo "================================"
echo "Testing security-gate.sh"
echo "================================"
echo ""

test_executable
test_help
test_invalid_mode
test_json_output
test_artifacts

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi
exit 0
