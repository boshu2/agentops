#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PASS=0
FAIL=0

pass() {
  echo "PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL=$((FAIL + 1))
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"

  mkdir -p \
    "$fixture/.claude-plugin" \
    "$fixture/.codex-plugin" \
    "$fixture/plugins" \
    "$fixture/schemas"

  cp "$ROOT/schemas/plugin-manifest.v1.schema.json" "$fixture/schemas/plugin-manifest.v1.schema.json"
  cp "$ROOT/schemas/codex-plugin-manifest.v1.schema.json" "$fixture/schemas/codex-plugin-manifest.v1.schema.json"
  cp "$ROOT/schemas/codex-marketplace.v1.schema.json" "$fixture/schemas/codex-marketplace.v1.schema.json"

  cat > "$fixture/.claude-plugin/plugin.json" <<'EOF'
{
  "name": "agentops",
  "version": "0.0.0"
}
EOF
}

write_legacy_codex_metadata() {
  local fixture="$1"

  cat > "$fixture/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "agentops",
  "description": "Legacy thin Codex plugin manifest.",
  "skills": "./skills-codex"
}
EOF

  cat > "$fixture/plugins/marketplace.json" <<'EOF'
{
  "name": "agentops-marketplace",
  "plugins": [
    {
      "name": "agentops",
      "source": {
        "source": "local",
        "path": "./"
      }
    }
  ]
}
EOF
}

write_plugin_creator_metadata() {
  local fixture="$1"

  cat > "$fixture/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "agentops",
  "version": "0.0.0",
  "description": "Modern Codex plugin manifest.",
  "skills": "./skills-codex",
  "mcpServers": "./mcp",
  "interface": {
    "displayName": "AgentOps",
    "shortDescription": "Repo-native memory and validation.",
    "longDescription": "AgentOps packages repo-native memory, validation gates, and repeatable agent workflows.",
    "developerName": "AgentOps",
    "category": "Productivity",
    "capabilities": [
      "Skills",
      "Hooks"
    ],
    "defaultPrompt": [
      "Use AgentOps to plan this change.",
      "Use AgentOps to validate this change."
    ],
    "brandColor": "#111827"
  }
}
EOF

  cat > "$fixture/plugins/marketplace.json" <<'EOF'
{
  "name": "agentops-marketplace",
  "interface": {
    "displayName": "AgentOps"
  },
  "plugins": [
    {
      "name": "agentops",
      "source": {
        "source": "local",
        "path": "./"
      },
      "policy": {
        "installation": "AVAILABLE",
        "authentication": "ON_INSTALL"
      },
      "category": "Productivity"
    }
  ]
}
EOF
}

run_manifest_validation() {
  local fixture="$1"
  local out="$2"

  bash "$ROOT/scripts/validate-manifests.sh" --repo-root "$fixture" > "$out" 2>&1
}

test_legacy_codex_metadata_shape_still_validates() {
  local fixture="$TMP_DIR/legacy"
  local out="$fixture/out.txt"

  setup_fixture "$fixture"
  write_legacy_codex_metadata "$fixture"

  if run_manifest_validation "$fixture" "$out"; then
    pass "legacy Codex plugin metadata shape validates"
  else
    fail "legacy Codex plugin metadata shape should validate"
    sed 's/^/  /' "$out"
  fi
}

test_plugin_creator_metadata_shape_validates() {
  local fixture="$TMP_DIR/plugin-creator"
  local out="$fixture/out.txt"

  setup_fixture "$fixture"
  write_plugin_creator_metadata "$fixture"

  if run_manifest_validation "$fixture" "$out"; then
    pass "plugin-creator Codex plugin metadata shape validates"
  else
    fail "plugin-creator Codex plugin metadata shape should validate"
    sed 's/^/  /' "$out"
  fi
}

echo "== test-codex-plugin-metadata-schema =="
test_legacy_codex_metadata_shape_still_validates
test_plugin_creator_metadata_shape_validates

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
