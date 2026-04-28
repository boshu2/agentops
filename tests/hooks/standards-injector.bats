#!/usr/bin/env bats
#
# Tests for hooks/standards-injector.sh — pins all six extension-to-language
# mappings, the symlink/path-traversal guards, and the kill-switch behavior.
# test-hooks.bats already covers python and unknown-extension; this fixture
# extends to the remaining five languages and exercises the security path.
# A regression in the language map (silent skip of go/ts/sh/js/yaml) would
# remove the standards-context injection without any user-visible signal.

setup() {
    load helpers/test_helper
    _helper_setup
    HOOK="$REPO_ROOT/hooks/standards-injector.sh"
}

teardown() {
    _helper_teardown
}

# fire FILE_PATH — pipe a tool_input JSON with the given file_path at the hook.
# Captures combined stdout+stderr into $output, exit status into $status.
fire() {
    local file="$1"
    local payload
    payload=$(jq -n --arg f "$file" '{"tool_input":{"file_path":$f}}')
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$payload" "$HOOK"
}

# Sanity check: the standards files referenced by the hook must exist in
# this repo, otherwise the per-language tests would silently exit 0 and
# pass for the wrong reason. Probe each before running the matrix.
@test "all referenced standards reference files exist" {
    for lang in python go typescript shell javascript yaml; do
        [ -f "$REPO_ROOT/skills/standards/references/${lang}.md" ]
    done
}

@test "go extension injects go standards" {
    fire "/some/repo/foo.go"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext | length > 0' >/dev/null
    echo "$output" | jq -e '.hookSpecificOutput.hookEventName == "PreToolUse"' >/dev/null
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$body" == *"# Go Standards"* ]]
    [[ "$body" == *"gofmt"* ]]
    [[ "$body" == *"skills/standards/references/go.md"* ]]
    [ "${#body}" -lt 1200 ]
}

@test "ts extension injects typescript standards" {
    fire "/some/repo/foo.ts"
    [ "$status" -eq 0 ]
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$body" == *"# TypeScript Standards"* ]]
    [[ "$body" == *"skills/standards/references/typescript.md"* ]]
    [ "${#body}" -lt 1200 ]
}

@test "tsx extension also injects typescript standards" {
    fire "/some/repo/foo.tsx"
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$body" == *"# TypeScript Standards"* ]]
    [[ "$body" == *"skills/standards/references/typescript.md"* ]]
    [ "${#body}" -lt 1200 ]
}

@test "sh extension injects shell standards" {
    fire "/some/repo/script.sh"
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$body" == *"# Shell Standards"* ]]
    [[ "$body" == *"set -euo pipefail"* ]]
    [[ "$body" == *"skills/standards/references/shell.md"* ]]
    [ "${#body}" -lt 1200 ]
}

@test "js extension injects javascript standards" {
    fire "/some/repo/app.js"
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$body" == *"# JavaScript Standards"* ]]
    [[ "$body" == *"skills/standards/references/javascript.md"* ]]
    [ "${#body}" -lt 1200 ]
}

@test "yaml and yml both inject yaml standards" {
    fire "/some/repo/conf.yaml"
    body1=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    fire "/some/repo/conf.yml"
    body2=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [ "$body1" = "$body2" ]
    [[ "$body1" == *"# YAML Standards"* ]]
    [[ "$body1" == *"skills/standards/references/yaml.md"* ]]
    [ "${#body1}" -lt 1200 ]
}

@test "full standards injection is explicit legacy opt-in" {
    local payload
    payload=$(jq -n '{"tool_input":{"file_path":"/some/repo/foo.go"}}')
    run bash -c 'printf "%s" "$1" | AGENTOPS_STANDARDS_FULL_INJECT=1 bash "$2" 2>&1' -- "$payload" "$HOOK"
    [ "$status" -eq 0 ]
    body=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    expected=$(cat "$REPO_ROOT/skills/standards/references/go.md")
    [ "$body" = "$expected" ]
}

@test "extensionless file path is silently skipped" {
    fire "/some/repo/Makefile"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "missing file_path is silently skipped" {
    local payload='{"tool_input":{}}'
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$payload" "$HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "AGENTOPS_HOOKS_DISABLED short-circuits before reading stdin" {
    local payload='{"tool_input":{"file_path":"/x.go"}}'
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' -- "$payload" "$HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "unsupported extension is silently skipped" {
    fire "/some/repo/notes.txt"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}
