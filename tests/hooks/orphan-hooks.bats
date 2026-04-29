#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    HOOKS_DIR="$TMP_DIR/hooks"
    mkdir -p "$HOOKS_DIR"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_manifests() {
    local registered="${1:-registered-json.sh}"
    cat > "$HOOKS_DIR/hooks.json" <<EOF
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {"type": "command", "command": "\${CLAUDE_PLUGIN_ROOT}/hooks/$registered"}
        ]
      }
    ]
  }
}
EOF
    cat > "$HOOKS_DIR/codex-hooks.json" <<'EOF'
{
  "hooks": {}
}
EOF
}

write_json_hook() {
    local name="$1"
    cat > "$HOOKS_DIR/$name" <<'EOF'
#!/usr/bin/env bash
printf '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"fixture"}}\n'
EOF
    chmod +x "$HOOKS_DIR/$name"
}

@test "orphan hook audit fails for unregistered JSON-emitting hook" {
    write_manifests
    write_json_hook "registered-json.sh"
    write_json_hook "orphan-json.sh"

    run env \
        AGENTOPS_HOOKS_DIR="$HOOKS_DIR" \
        AGENTOPS_HOOKS_JSON="$HOOKS_DIR/hooks.json" \
        AGENTOPS_CODEX_HOOKS_JSON="$HOOKS_DIR/codex-hooks.json" \
        bash "$REPO_ROOT/tests/hooks/test-orphan-hooks.sh"

    [ "$status" -eq 1 ]
    [[ "$output" == *"Unregistered + JSON-emitting: orphan-json.sh"* ]]
}

@test "orphan hook audit passes registered JSON-emitting hook" {
    write_manifests
    write_json_hook "registered-json.sh"

    run env \
        AGENTOPS_HOOKS_DIR="$HOOKS_DIR" \
        AGENTOPS_HOOKS_JSON="$HOOKS_DIR/hooks.json" \
        AGENTOPS_CODEX_HOOKS_JSON="$HOOKS_DIR/codex-hooks.json" \
        bash "$REPO_ROOT/tests/hooks/test-orphan-hooks.sh"

    [ "$status" -eq 0 ]
    [[ "$output" == *"No unregistered hooks emitting JSON output"* ]]
}
