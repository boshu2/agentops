#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PROMPT_DIR="$ROOT/tests/explicit-skill-requests/prompts"

mapfile -t prompts < <(find "$PROMPT_DIR" -name "*.txt" -type f | sort)

if [[ "${#prompts[@]}" -lt 20 ]]; then
    echo "expected at least 20 explicit skill prompts, found ${#prompts[@]}" >&2
    exit 1
fi

for stale in extract knowledge; do
    if [[ -e "$PROMPT_DIR/$stale.txt" ]]; then
        echo "stale prompt still present: $stale.txt" >&2
        exit 1
    fi
done

for prompt_file in "${prompts[@]}"; do
    skill="$(basename "$prompt_file" .txt)"
    prompt="$(cat "$prompt_file")"

    if [[ ! -f "$ROOT/skills/$skill/SKILL.md" ]]; then
        echo "missing shared skill for prompt: $skill" >&2
        exit 1
    fi
    if [[ ! -f "$ROOT/skills-codex/$skill/SKILL.md" ]]; then
        echo "missing Codex skill for prompt: $skill" >&2
        exit 1
    fi
    if [[ "$prompt" != *"/$skill"* ]]; then
        echo "prompt $skill.txt does not mention /$skill" >&2
        exit 1
    fi
done

grep -q 'assert_skill_triggered' "$ROOT/tests/explicit-skill-requests/run-all.sh"
grep -q 'assert_no_premature_tools' "$ROOT/tests/explicit-skill-requests/run-all.sh"
grep -q 'tests/explicit-skill-requests/' "$ROOT/docs/TESTING.md"

echo "explicit skill prompt catalog passed (${#prompts[@]} prompts)"
