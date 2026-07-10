#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

bash scripts/check-docs-cli-snippets.sh
bats tests/cli_contract_gate.bats
bash scripts/generate-cli-reference.sh --check
bash scripts/check-cmdao-surface-parity.sh

printf 'check-cli-contract: PASS\n'
