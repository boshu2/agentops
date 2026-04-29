#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
script_path="$repo_root/scripts/validate-daemon-product-e2e.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  if ! rg -q "$pattern" "$file"; then
    echo "---- $file ----" >&2
    sed -n '1,220p' "$file" >&2
    fail "$label"
  fi
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  if rg -q "$pattern" "$file"; then
    echo "---- $file ----" >&2
    sed -n '1,220p' "$file" >&2
    fail "$label"
  fi
}

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/daemon-product-e2e-test.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

runner="$tmpdir/fake-go-test.sh"
log="$tmpdir/runner.log"
cat > "$runner" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

echo "${AGENTOPS_DAEMON_E2E_SECTION:-missing}|$*" >> "${AGENTOPS_DAEMON_E2E_RUNNER_LOG:?}"
echo "fake go test for ${AGENTOPS_DAEMON_E2E_SECTION:-missing}: $*"

if [[ "${AGENTOPS_FAKE_FAIL_SECTION:-}" == "${AGENTOPS_DAEMON_E2E_SECTION:-}" ]]; then
  echo "forced section failure: ${AGENTOPS_DAEMON_E2E_SECTION}" >&2
  exit 42
fi
EOF
chmod +x "$runner"

run_with_fake_runner() {
  AGENTOPS_DAEMON_E2E_RUNNER="$runner" \
    AGENTOPS_DAEMON_E2E_RUNNER_LOG="$log" \
    "$script_path" "$@"
}

list_out="$tmpdir/list.out"
run_with_fake_runner --list > "$list_out"
assert_contains "$list_out" '^fake-gascity[[:space:]]' "section list includes fake-gascity"
assert_contains "$list_out" '^state-machine-invariants[[:space:]]' "section list includes state-machine-invariants"
assert_contains "$list_out" '^boundary-failpoints[[:space:]]' "section list includes boundary-failpoints"

all_out="$tmpdir/all.out"
run_with_fake_runner --fixture > "$all_out"
assert_contains "$all_out" '^== state-machine-invariants ==$' "text report includes state-machine-invariants section"
assert_contains "$all_out" '^== boundary-failpoints ==$' "text report includes boundary-failpoints section"
assert_contains "$all_out" 'daemon product e2e fixture gate: PASS' "all-section fixture run passes with fake runner"
assert_contains "$log" '^fake-gascity|./internal/gascity ./internal/agentworker ./internal/wikiworker -run Test' "fake runner receives fake-gascity go test args"
assert_contains "$log" '^boundary-failpoints|./internal/daemon -run Test' "fake runner receives boundary-failpoints go test args"

section_out="$tmpdir/section.out"
: > "$log"
run_with_fake_runner --section boundary-failpoints > "$section_out"
assert_contains "$section_out" '^== boundary-failpoints ==$' "single-section run includes requested section"
assert_not_contains "$section_out" '^== fake-gascity ==$' "single-section run excludes other sections"
if [[ "$(wc -l < "$log" | tr -d ' ')" != "1" ]]; then
  fail "single-section run should call the runner exactly once"
fi

fail_out="$tmpdir/fail.out"
: > "$log"
set +e
AGENTOPS_DAEMON_E2E_RUNNER="$runner" \
  AGENTOPS_DAEMON_E2E_RUNNER_LOG="$log" \
  AGENTOPS_FAKE_FAIL_SECTION="boundary-failpoints" \
  "$script_path" --section boundary-failpoints > "$fail_out" 2>&1
status=$?
set -e
if [[ "$status" -eq 0 ]]; then
  fail "forced boundary-failpoints failure should propagate"
fi
assert_contains "$fail_out" '^result: FAIL' "failing section reports FAIL"
assert_contains "$fail_out" 'forced section failure: boundary-failpoints' "failing section prints runner output"

json_out="$tmpdir/json.out"
run_with_fake_runner --json --section state-machine-invariants > "$json_out"
python3 - "$json_out" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
assert data["result"] == "PASS"
assert [section["name"] for section in data["sections"]] == ["state-machine-invariants"]
assert data["sections"][0]["status"] == "PASS"
PY

echo "PASS: validate-daemon-product-e2e.sh"
