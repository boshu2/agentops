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
  local nonblocking_jobs="${3:-}"
  cat > "$path" <<EOF
name: Validate
on:
  push:
    branches: [main]

jobs:
EOF
  local jobs_csv="${needs_line//,/ }"
  for job in $jobs_csv; do
    job="$(echo "$job" | xargs)"
    [[ -z "$job" ]] && continue
    {
      echo "  ${job}:"
      echo "    runs-on: ubuntu-latest"
    } >> "$path"
    if grep -qw "$job" <<<"$nonblocking_jobs"; then
      echo "    continue-on-error: true" >> "$path"
    fi
  done
  cat >> "$path" <<EOF
  summary:
    needs: [$needs_line]
    runs-on: ubuntu-latest
    if: always()
    steps:
      - name: Check results
        run: |
          echo summary
EOF
}

write_manifest() {
  local path="$1"
  shift
  {
    echo "jobs:"
    for entry in "$@"; do
      local name="${entry%%|*}"
      local rest="${entry#*|}"
      local reason="${rest%%|*}"
      local failure="${rest#*|}"
      printf '  - name: %s\n    reason: %s\n    failure: %s\n' \
        "$name" "$reason" "$failure"
    done
  } > "$path"
}

run_with_fixtures() {
  local agents_file="$1"
  local workflow_file="$2"
  local manifest_file="$3"
  local out_file="$4"
  AGENTS_PATH="$agents_file" \
  WORKFLOW_PATH="$workflow_file" \
  MANIFEST_PATH="$manifest_file" \
  bash "$SCRIPT" > "$out_file" 2>&1
}

test_pass_aligned_policy() {
  local fixture="$TMP_DIR/pass"
  mkdir -p "$fixture"
  local agents="$fixture/AGENTS.md"
  local workflow="$fixture/validate.yml"
  local manifest="$fixture/ci-jobs.yaml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks except security-toolchain-gate (non-blocking)." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **hook-preflight** | hook safety | missing guard |\n| **security-toolchain-gate** (non-blocking) | scanners | tool not installed |'

  write_workflow "$workflow" \
    "doc-release-gate, hook-preflight, security-toolchain-gate" \
    "security-toolchain-gate"

  write_manifest "$manifest" \
    "doc-release-gate|docs parity|stale docs" \
    "hook-preflight|hook safety|missing guard" \
    "security-toolchain-gate|scanners|tool not installed"

  if run_with_fixtures "$agents" "$workflow" "$manifest" "$out"; then
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
  local manifest="$fixture/ci-jobs.yaml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **hook-preflight** | hook safety | missing guard |\n| **extra-doc-job** | extra | drift |'

  write_workflow "$workflow" \
    "doc-release-gate, hook-preflight" \
    ""

  write_manifest "$manifest" \
    "doc-release-gate|docs parity|stale docs" \
    "hook-preflight|hook safety|missing guard"

  if run_with_fixtures "$agents" "$workflow" "$manifest" "$out"; then
    fail "should fail when AGENTS job table drifts from workflow needs"
    return
  fi

  if grep -q "table drifts from generator output" "$out"; then
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
  local manifest="$fixture/ci-jobs.yaml"
  local out="$fixture/out.txt"

  write_agents "$agents" \
    "The summary job gates on all checks except security-toolchain-gate (non-blocking)." \
    $'| **doc-release-gate** | docs parity | stale docs |\n| **security-toolchain-gate** | scanners | tool not installed |'

  write_workflow "$workflow" \
    "doc-release-gate, security-toolchain-gate" \
    "security-toolchain-gate"

  write_manifest "$manifest" \
    "doc-release-gate|docs parity|stale docs" \
    "security-toolchain-gate|scanners|tool not installed"

  if run_with_fixtures "$agents" "$workflow" "$manifest" "$out"; then
    fail "should fail when non-blocking policy drifts"
    return
  fi

  if grep -q "non-blocking" "$out"; then
    pass "reports non-blocking drift"
  else
    fail "missing non-blocking drift message"
  fi
}

echo "== test-ci-policy-parity =="
test_pass_aligned_policy
test_fail_job_list_drift
test_fail_nonblocking_drift

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
