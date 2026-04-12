#!/usr/bin/env bash
# Test JSON schemas are valid and have required properties
# Test harness: intentionally -uo (not -euo) to accumulate PASS/FAIL
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [[ "$expected" == "$actual" ]]; then
        echo "  PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc (expected=$expected, actual=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test: JSON Schemas ==="

# team-spec.json
echo "team-spec.json:"
assert_eq "file exists" "true" "$(test -f "$REPO_ROOT/lib/schemas/team-spec.json" && echo true || echo false)"
assert_eq "valid JSON" "true" "$(jq empty "$REPO_ROOT/lib/schemas/team-spec.json" 2>/dev/null && echo true || echo false)"
assert_eq "additionalProperties false" "false" "$(jq -r '.additionalProperties' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "has team_id property" "string" "$(jq -r '.properties.team_id.type' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "has runtime property" "string" "$(jq -r '.properties.runtime.type' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "runtime enum" "codex,claude" "$(jq -r '.properties.runtime.enum | join(",")' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "has repo_path property" "string" "$(jq -r '.properties.repo_path.type' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "has agents property" "array" "$(jq -r '.properties.agents.type' "$REPO_ROOT/lib/schemas/team-spec.json")"
assert_eq "agents additionalProperties false" "false" "$(jq -r '.properties.agents.items.additionalProperties' "$REPO_ROOT/lib/schemas/team-spec.json")"

# worker-output.json
echo "worker-output.json:"
assert_eq "file exists" "true" "$(test -f "$REPO_ROOT/lib/schemas/worker-output.json" && echo true || echo false)"
assert_eq "valid JSON" "true" "$(jq empty "$REPO_ROOT/lib/schemas/worker-output.json" 2>/dev/null && echo true || echo false)"
assert_eq "additionalProperties false" "false" "$(jq -r '.additionalProperties' "$REPO_ROOT/lib/schemas/worker-output.json")"
assert_eq "has status property" "string" "$(jq -r '.properties.status.type' "$REPO_ROOT/lib/schemas/worker-output.json")"
assert_eq "has artifacts property" "array" "$(jq -r '.properties.artifacts.type' "$REPO_ROOT/lib/schemas/worker-output.json")"
assert_eq "has token_usage property" "object" "$(jq -r '.properties.token_usage.type' "$REPO_ROOT/lib/schemas/worker-output.json")"
assert_eq "token_usage additionalProperties false" "false" "$(jq -r '.properties.token_usage.additionalProperties' "$REPO_ROOT/lib/schemas/worker-output.json")"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
