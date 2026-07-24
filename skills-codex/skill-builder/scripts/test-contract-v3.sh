#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
export PYTHONDONTWRITEBYTECODE=1

cd "$REPO_ROOT"
if ! python3 -m unittest discover \
  -s skills/skill-builder/tests \
  -p 'test_contract_v3.py' >/dev/null 2>&1; then
  python3 -m unittest discover \
    -s skills/skill-builder/tests \
    -p 'test_contract_v3.py'
  exit 1
fi
echo "skill-contract.v3 probe: PASS"
