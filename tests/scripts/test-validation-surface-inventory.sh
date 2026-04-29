#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-surface-inventory.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

run_validator() {
  local inventory="$1"
  local workflow="$2"
  local out="$3"
  VALIDATION_SURFACE_INVENTORY_PATH="$inventory" \
  VALIDATION_SURFACE_WORKFLOW_PATH="$workflow" \
  bash "$SCRIPT" > "$out" 2>&1
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
  retrieval-quality:
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

write_inventory() {
  local path="$1"
  local extra_surface="${2:-}"
  cat > "$path" <<EOF
{
  "schema_version": 1,
  "surfaces": [
    {
      "id": "ci.doc-release-gate",
      "command": "github-actions:doc-release-gate",
      "surface": "ci",
      "category": "docs",
      "purpose": "Docs release validation",
      "blocking_policy": "blocking",
      "fast_behavior": "Runs in CI",
      "full_behavior": "Runs in CI",
      "ci_job": "doc-release-gate",
      "docs_owner": "docs/CI-CD.md"
    },
    {
      "id": "ci.retrieval-quality",
      "command": "github-actions:retrieval-quality",
      "surface": "ci",
      "category": "retrieval",
      "purpose": "Advisory retrieval precision",
      "blocking_policy": "advisory",
      "fast_behavior": "Warns only",
      "full_behavior": "Warns only",
      "ci_job": "retrieval-quality",
      "docs_owner": "docs/CI-CD.md"
    }$extra_surface
  ]
}
EOF
}

test_pass_real_inventory() {
  local out="$TMP_DIR/real.out"
  if bash "$SCRIPT" > "$out" 2>&1; then
    pass "real inventory matches validate.yml"
  else
    fail "real inventory should pass"
    sed 's/^/  /' "$out"
  fi
}

test_fails_when_workflow_job_missing_from_inventory() {
  local fixture="$TMP_DIR/missing-inventory"
  mkdir -p "$fixture"
  local workflow="$fixture/validate.yml"
  local inventory="$fixture/inventory.json"
  local out="$fixture/out.txt"

  write_workflow "$workflow" \
    "doc-release-gate, retrieval-quality, hook-preflight" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]]" \
    $'  hook-preflight:\n    runs-on: ubuntu-latest'
  write_inventory "$inventory"

  if run_validator "$inventory" "$workflow" "$out"; then
    fail "should fail when workflow job is absent from inventory"
    return
  fi

  if grep -q "inventory CI jobs drift" "$out"; then
    pass "reports workflow job missing from inventory"
  else
    fail "missing inventory drift message"
  fi
}

test_fails_when_blocking_policy_drifts() {
  local fixture="$TMP_DIR/policy-drift"
  mkdir -p "$fixture"
  local workflow="$fixture/validate.yml"
  local inventory="$fixture/inventory.json"
  local out="$fixture/out.txt"

  write_workflow "$workflow" \
    "doc-release-gate, retrieval-quality" \
    "[[ \"\${{ needs.doc-release-gate.result }}\" != \"success\" ]] || [[ \"\${{ needs.retrieval-quality.result }}\" != \"success\" ]]"
  write_inventory "$inventory"

  if run_validator "$inventory" "$workflow" "$out"; then
    fail "should fail when inventory advisory policy drifts from workflow failset"
    return
  fi

  if grep -q "blocking policy drift" "$out"; then
    pass "reports blocking/advisory policy drift"
  else
    fail "missing blocking policy drift message"
  fi
}

echo "== test-validation-surface-inventory =="
test_pass_real_inventory
test_fails_when_workflow_job_missing_from_inventory
test_fails_when_blocking_policy_drifts

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
