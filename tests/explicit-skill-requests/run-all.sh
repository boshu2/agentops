#!/usr/bin/env bash
# Validate manifest and explicit-request artifact resolution, without live calls.
# Usage: ./run-all.sh [repo-root]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${1:-$(cd "$SCRIPT_DIR/../.." && pwd)}"
echo "Explicit request artifact resolution: Tier S structural proof"
echo "Live skill selection and first-tool ordering are not checked."
bash "$SCRIPT_DIR/../../scripts/validate-manifests.sh" --repo-root "$REPO_ROOT"
passed=0
failed=0
for prompt_file in "$REPO_ROOT/tests/explicit-skill-requests/prompts"/*.txt; do
    [[ -f "$prompt_file" ]] || continue
    if bash "$SCRIPT_DIR/run-test.sh" "$(basename "$prompt_file" .txt)" "$REPO_ROOT"; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi
done
# Every current public canonical skill needs an explicit request fixture.
for skill in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill" ]] || continue
    name="$(basename "$(dirname "$skill")")"
    [[ "$name" == _* ]] && continue
    if [[ ! -s "$REPO_ROOT/tests/explicit-skill-requests/prompts/$name.txt" ]]; then
        echo "FAIL: current skill $name lacks an explicit request fixture" >&2
        failed=$((failed + 1))
    fi
done
echo "Explicit request structure: $passed passed, $failed failed"
[[ "$passed" -gt 0 && "$failed" -eq 0 ]]
