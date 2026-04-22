#!/usr/bin/env bash
# run-tests.sh — run Python unit tests without requiring pytest.
#
# Uses stdlib unittest so it works anywhere python3 is installed. CI jobs that
# already have pytest can also run `pytest tests/python/` — the tests inherit
# from unittest.TestCase so both runners work.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"
exec python3 -m unittest discover -s tests/python -p 'test_*.py' "$@"
