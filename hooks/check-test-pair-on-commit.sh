#!/bin/bash
# check-test-pair-on-commit.sh — PreToolUse hook: warn when a git commit
# stages code changes (.go / .sh) without paired test changes
# (_test.go / .bats), as required by
# docs/contracts/update-principles.md (Principle 2: drift-blocking test
# included).
#
# Scope (v1): warn-only, non-blocking. Fires on Bash tool calls that
# match `git commit -m …` or `git commit -F …`. Inspects the staged
# diff (git diff --cached --name-only) and emits a warning when the
# code-without-test pattern is detected.
#
# This is the harness automation for cycle 45's hypothesis H45.2
# (source-surface detection) at the "test paired with code" level.
# Cycle 54 wired Principle 4 (fitness delta); this cycle wires
# Principle 2.
#
# Sibling pattern: hooks/update-principles-check.sh (cycle 54 commit
# ecb3b3ba) — same PreToolUse:Bash matcher, same kill-switch layering,
# same warn-only posture. Each principle gets its own hook so single-
# concern (Principle 1) is preserved at the enforcer layer too.
#
# Heuristics for code-without-test:
#  - any *.go staged AND no *_test.go staged → WARN
#  - any *.sh under hooks/, scripts/ staged AND no *.bats staged → WARN
#  - exemptions: docs/, .md only, generated dirs (cli/embedded/), and
#    files matching /testdata/ or /testfixtures/ (these are test
#    fixtures, not source)
#
# practices: [continuous-integration, design-by-contract, code-complete]
set -uo pipefail

# Kill switches — both global and hook-specific
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_TEST_PAIR_CHECK_DISABLED:-}" = "1" ] && exit 0

# Only fire on Bash tool calls
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [ -z "$TOOL_NAME" ]; then
    INPUT=$(cat)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[ "$TOOL_NAME" != "Bash" ] && exit 0

# Extract the command from the tool input
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$COMMAND" ]; then
    [ -z "${INPUT:-}" ] && INPUT=$(cat)
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || exit 0
fi

# Match only `git commit -m …` or `git commit -F …` patterns
case "$COMMAND" in
    *"git commit "*-m\ *|*"git commit "*--message\ *|*"git commit "*-F\ *|*"git commit "*--file\ *)
        ;;
    *)
        exit 0
        ;;
esac

# Inspect staged diff. If we cannot read it (no git, not a repo, etc.)
# fail open — better to under-warn than to block on tooling failures.
STAGED=$(git diff --cached --name-only 2>/dev/null) || exit 0
[ -z "$STAGED" ] && exit 0

# Classify staged files. Exclude generated, fixture, and docs-only paths.
HAS_GO_SRC=0
HAS_GO_TEST=0
HAS_SHELL_SRC=0
HAS_BATS=0
while IFS= read -r f; do
    [ -z "$f" ] && continue
    case "$f" in
        # Generated / fixture / testdata exclusions
        cli/embedded/*) continue ;;
        */testdata/*|*/testfixtures/*) continue ;;
        # Test files
        *_test.go) HAS_GO_TEST=1 ;;
        *.bats) HAS_BATS=1 ;;
        # Source files (after excluding tests above)
        *.go) HAS_GO_SRC=1 ;;
        hooks/*.sh|scripts/*.sh|lib/*.sh) HAS_SHELL_SRC=1 ;;
    esac
done <<< "$STAGED"

# Build warning if code-without-test is detected
WARNS=()
if [ "$HAS_GO_SRC" -eq 1 ] && [ "$HAS_GO_TEST" -eq 0 ]; then
    WARNS+=("staged *.go change without paired *_test.go")
fi
if [ "$HAS_SHELL_SRC" -eq 1 ] && [ "$HAS_BATS" -eq 0 ]; then
    WARNS+=("staged shell change under hooks/scripts/lib without paired *.bats test")
fi

# If no warnings, exit silently
[ "${#WARNS[@]}" -eq 0 ] && exit 0

# Emit warning via additionalContext, non-blocking
MSG="WARN: Commit stages code without paired tests (docs/contracts/update-principles.md Principle 2). Detected: $(IFS='; '; echo "${WARNS[*]}"). To make the change drift-blocking, add the test in the same commit OR explicitly note rationale (refactor with existing coverage, docs-only changes, etc.) in the commit body."

# JSON-encode the message safely via jq (avoids quoting traps)
JSON_OUT=$(jq -nc --arg msg "$MSG" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: $msg}}' 2>/dev/null) || exit 0
echo "$JSON_OUT"

# Always exit 0 — warn-only in v1
exit 0
