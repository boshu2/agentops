#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
export PYTHONDONTWRITEBYTECODE=1

cd "$REPO_ROOT"
test_patterns=(test_contract_v3.py)
if [[ "${AGENTOPS_WRITE_CONFINED:-}" != "1" ]]; then
  test_patterns+=(test_probe_runner.py)
fi
for test_pattern in "${test_patterns[@]}"; do
  if ! python3 -m unittest discover \
    -s skills/skill-builder/tests \
    -p "$test_pattern" >/dev/null 2>&1; then
    python3 -m unittest discover \
      -s skills/skill-builder/tests \
      -p "$test_pattern"
    exit 1
  fi
done
echo "skill-contract.v3 probe: PASS"
