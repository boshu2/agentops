#!/usr/bin/env bash
set -euo pipefail
# shellcheck disable=SC2016

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-ci-policy-parity.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

if [[ ! -x "$SCRIPT" ]]; then
  echo "FAIL: missing executable script: $SCRIPT" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

write_agents() {
  local path="$1"
  local header_line="$2"
  local rows="$3"
  cat > "$path" <<EOF
# Agent Instructions

$header_line

### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
$rows
EOF
}

write_workflow() {
  local path="$1"
  local needs_line="$2"
  local fail_expr="$3"
  local extra_jobs="${4:-}"
  cat > "$path" <<EOF
name: Validate
on:
  push:
    branches: [main]

jobs:
  doc-release-gate:
    runs-on: ubuntu-latest
  hook-preflight:
    runs-on: ubuntu-latest
  security-toolchain-gate:
    runs-on: ubuntu-latest
$extra_jobs
  summary:
    needs: [$needs_line]
    runs-on: ubuntu-latest
    if: always()
    steps:
      - name: Check results
        run: |
          if $fail_expr; then
            exit 1
          fi
EOF
}

run_with_fixtures() {
  local agents_file="$1"
  local workflow_file="$2"
  local out_file="$3"
  CI_POLICY_PARITY_AGENTS_PATH="$agents_file" \
  CI_POLICY_PARITY_WORKFLOW_PATH="$workflow_file" \
  bash "$SCRIPT" > "$out_file" 2>&1
}

test_pass_aligned_policy() {
  local fixture="$TMP_DIR/pass"
  mkdir -p "$fixture"
  local agents="$fixture/AGENTS.md"
  local workflow="$fixture/validate.yml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks except security-toolchain-gate (non-blocking)." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **hook-preflight** | hook safety | missing guard |\n| **security-toolchain-gate** | scanners | tool not installed |'

  write_workflow "$workflow" \
    "doc-release-gate, hook-preflight, security-toolchain-gate" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]] || [[ \"\${{ needs.hook-preflight.result }}\" != \"success\" ]]"

  if run_with_fixtures "$agents" "$workflow" "$out"; then
    pass "passes when AGENTS CI policy matches workflow"
  else
    fail "should pass when policy is aligned"
    sed 's/^/  /' "$out"
  fi
}

test_fail_job_list_drift() {
  local fixture="$TMP_DIR/job-drift"
  mkdir -p "$fixture"
  local agents="$fixture/AGENTS.md"
  local workflow="$fixture/validate.yml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **hook-preflight** | hook safety | missing guard |\n| **extra-doc-job** | extra | drift |'

  write_workflow "$workflow" \
    "doc-release-gate, hook-preflight" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]] || [[ \"\${{ needs.hook-preflight.result }}\" != \"success\" ]]"

  if run_with_fixtures "$agents" "$workflow" "$out"; then
    fail "should fail when AGENTS job table drifts from workflow needs"
    return
  fi

  if grep -q "Job list drift detected" "$out"; then
    pass "reports job list drift"
  else
    fail "missing job list drift message"
  fi
}

test_fail_nonblocking_drift() {
  local fixture="$TMP_DIR/nonblocking-drift"
  mkdir -p "$fixture"
  local agents="$fixture/AGENTS.md"
  local workflow="$fixture/validate.yml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks except security-toolchain-gate (non-blocking)." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **security-toolchain-gate** | scanners | tool not installed |'

  write_workflow "$workflow" \
    "doc-release-gate, security-toolchain-gate" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]] || [[ \"\${{ needs.security-toolchain-gate.result }}\" != \"success\" ]]"

  if run_with_fixtures "$agents" "$workflow" "$out"; then
    fail "should fail when non-blocking policy drifts"
    return
  fi

  if grep -q "Non-blocking policy drift detected" "$out"; then
    pass "reports non-blocking drift"
  else
    fail "missing non-blocking drift message"
  fi
}

test_fail_workflow_orphan_job() {
  local fixture="$TMP_DIR/workflow-orphan-job"
  mkdir -p "$fixture"
  local agents="$fixture/AGENTS.md"
  local workflow="$fixture/validate.yml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks except security-toolchain-gate (non-blocking)." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **hook-preflight** | hook safety | missing guard |\n| **security-toolchain-gate** | scanners | tool not installed |'

  write_workflow "$workflow" \
    "doc-release-gate, hook-preflight, security-toolchain-gate" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]] || [[ \"\${{ needs.hook-preflight.result }}\" != \"success\" ]]" \
    $'  retrieval-quality:\n    runs-on: ubuntu-latest'

  if run_with_fixtures "$agents" "$workflow" "$out"; then
    fail "should fail when a top-level workflow job is missing from summary.needs"
    return
  fi

  if grep -q "Workflow job list drift detected" "$out"; then
    pass "reports top-level workflow jobs missing from summary.needs"
  else
    fail "missing workflow job list drift message"
  fi
}

echo "== test-ci-policy-parity =="
test_pass_aligned_policy
test_fail_job_list_drift
test_fail_nonblocking_drift
test_fail_workflow_orphan_job

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
