#!/usr/bin/env bash
# Structural resolution of one explicit qualified request. No runtime is launched.
# Usage: ./run-test.sh <skill-name> [repo-root]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${2:-$(cd "$SCRIPT_DIR/../.." && pwd)}"
SKILL_NAME="${1:-}"
if [[ ! "$SKILL_NAME" =~ ^[a-z][a-z0-9-]*$ ]]; then
    echo "FAIL: expected a canonical skill slug, got '$SKILL_NAME'" >&2
    exit 1
fi
PROMPT_FILE="$REPO_ROOT/tests/explicit-skill-requests/prompts/$SKILL_NAME.txt"
if [[ ! -s "$PROMPT_FILE" ]]; then
    echo "FAIL: prompt fixture missing or empty: $PROMPT_FILE" >&2
    exit 1
fi
# The qualified token is an explicit address, not natural-language trigger proof.
if ! grep -oE "/agentops:[a-z][a-z0-9-]*" "$PROMPT_FILE" | grep -Fx "/agentops:$SKILL_NAME" >/dev/null; then
    echo "FAIL: prompt does not explicitly address /agentops:$SKILL_NAME" >&2
    exit 1
fi
for surface in skills skills-codex; do
    target="$REPO_ROOT/$surface/$SKILL_NAME/SKILL.md"
    if [[ ! -s "$target" ]]; then
        echo "FAIL: canonical target missing or empty: $target" >&2
        exit 1
    fi
    name=$(awk '/^---/{if(++c==1) next; exit} /^name:/{sub(/^name:[[:space:]]*/, ""); gsub(/^["\047]|["\047]$/, ""); print}' "$target")
    if [[ "$name" != "$SKILL_NAME" ]]; then
        echo "FAIL: $surface/$SKILL_NAME name '$name' differs from requested slug" >&2
        exit 1
    fi
done
echo "PASS: /agentops:$SKILL_NAME resolves structurally to canonical and Codex artifacts"
