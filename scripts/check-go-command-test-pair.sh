#!/usr/bin/env bash
set -euo pipefail

# check-go-command-test-pair.sh
# Enforce command/test co-change for cli/cmd/ao command surface changes.
#
# Rule:
# - If one or more non-test Go files under cli/cmd/ao/ change in the push scope,
#   at least one *_test.go file under cli/cmd/ao/ must also change.
#
# Exit codes:
#   0 - pass / not applicable
#   1 - validation failed

if [[ "${AGENTOPS_SKIP_COMMAND_TEST_PAIR:-}" == "1" ]]; then
    echo "SKIP: command/test pairing check disabled via AGENTOPS_SKIP_COMMAND_TEST_PAIR=1"
    exit 0
fi

collect_changed_files() {
    if git rev-parse --git-dir >/dev/null 2>&1; then
        # Prefer an explicit push range when the gate exports one
        # (ao gate check --scope range:<base>..<head>). A detached landing
        # worktree has no @{upstream}, so the fallbacks below see only the tip
        # commit and a c1+c2 train whose tests land in c2 falsely fails at c1.
        # The explicit range spans the whole train. It is authoritative: an
        # empty range diff means "nothing in range" (SKIP), not "fall through".
        if [[ -n "${AGENTOPS_GATE_RANGE:-}" ]]; then
            git diff --name-only "$AGENTOPS_GATE_RANGE" 2>/dev/null || true
            return 0
        fi
        if git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' >/dev/null 2>&1; then
            ahead_files="$(git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true)"
            if [[ -n "$ahead_files" ]]; then
                printf '%s\n' "$ahead_files"
                return 0
            fi
        fi
        git diff --name-only --cached 2>/dev/null || true
        git diff --name-only 2>/dev/null || true
        git show --name-only --pretty=format: HEAD 2>/dev/null || true
    fi
}

tmp_files="$(mktemp)"
trap 'rm -f "$tmp_files"' EXIT

collect_changed_files | sed '/^[[:space:]]*$/d' | sort -u > "$tmp_files"

if [[ ! -s "$tmp_files" ]]; then
    echo "SKIP: no changed files detected"
    exit 0
fi

mapfile -t changed_cmd_files < <(grep -E '^cli/cmd/ao/.*\.go$' "$tmp_files" | grep -Ev '_test\.go$' || true)
mapfile -t changed_test_files < <(grep -E '^cli/cmd/ao/.*_test\.go$' "$tmp_files" || true)

if [[ "${#changed_cmd_files[@]}" -eq 0 ]]; then
    echo "SKIP: no cli/cmd/ao command-surface Go changes"
    exit 0
fi

if [[ "${#changed_test_files[@]}" -eq 0 ]]; then
    echo "FAIL: command files changed in cli/cmd/ao without any *_test.go changes." >&2
    echo "Changed command files:" >&2
    printf '  - %s\n' "${changed_cmd_files[@]}" >&2
    echo "Add at least one cli/cmd/ao/*_test.go change in the same commit/push." >&2
    exit 1
fi

echo "PASS: command/test pairing check succeeded."
echo "  command files changed: ${#changed_cmd_files[@]}"
echo "  test files changed:    ${#changed_test_files[@]}"
exit 0
