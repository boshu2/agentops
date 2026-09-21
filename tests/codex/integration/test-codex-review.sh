#!/usr/bin/env bash
# Live CLI primitive: review an uncommitted change in a controlled repository.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tests/codex/integration/test-helpers.sh
source "$SCRIPT_DIR/test-helpers.sh"
codex_test_setup review
printf 'def greet(name):\n    return "Hello, " + name\n' > "$CODEX_TEST_PROJECT/hello.py"
git -C "$CODEX_TEST_PROJECT" add hello.py
git -C "$CODEX_TEST_PROJECT" commit -q -m "Fixture baseline"
printf 'def greet(name):\n    return "Hello, " + name.strip()\n' > "$CODEX_TEST_PROJECT/hello.py"
cd "$CODEX_TEST_PROJECT"
codex_test_run review codex review "${CODEX_TEST_MODEL_ARGS[@]}" --uncommitted
codex_test_require_output "$CODEX_TEST_ARTIFACTS/command.log"
echo "PASS: codex review --uncommitted completed with captured output"
