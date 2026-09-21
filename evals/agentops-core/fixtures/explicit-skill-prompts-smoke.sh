#!/usr/bin/env bash
# Offline structural canary. Live selection and tool ordering are outside scope.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
bash "$ROOT/tests/explicit-skill-requests/run-all.sh"
bats "$ROOT/tests/scripts/explicit-skill-requests.bats"
echo "explicit skill prompt catalog passed (Tier S structural proof only)"
