#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/audit-codex-hooks.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_hooks_fixture() {
  local codex_home="$1"
  mkdir -p "$codex_home"
  cat > "$codex_home/hooks.json" <<'EOF'
{
  "$schema": "../schemas/hooks-manifest.v1.schema.json",
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash /plugin/hooks/session-start.sh",
            "timeout": 10
          },
          {
            "type": "command",
            "command": "bash /plugin/hooks/factory-router.sh",
            "timeout": 8
          },
          {
            "type": "command",
            "command": "python3 /custom/noisy-context.py --event SessionStart",
            "timeout": 5
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash /plugin/hooks/quality-signals.sh",
            "timeout": 3
          },
          {
            "type": "command",
            "command": "python3 /custom/noisy-context.py --event UserPromptSubmit",
            "timeout": 5
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "bash /plugin/hooks/commit-review-gate.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash /custom/session-end-freshness.sh",
            "timeout": 35
          }
        ]
      }
    ]
  }
}
EOF
}

test_reports_foreign_handlers() {
  local codex_home="$TMP_DIR/report/.codex"
  local output
  setup_hooks_fixture "$codex_home"

  output="$(bash "$SCRIPT" --codex-home "$codex_home" --json)"
  if jq -e \
    '.total_handlers == 7
     and .agentops_handlers == 4
     and .foreign_handlers == 3
     and .foreign_model_visible_handlers == 2' <<< "$output" >/dev/null; then
    pass "reports AgentOps and foreign Codex hook counts"
  else
    fail "unexpected audit JSON: $output"
  fi

  if bash "$SCRIPT" --codex-home "$codex_home" --strict >/dev/null 2>&1; then
    fail "strict audit should fail when foreign handlers exist"
  else
    pass "strict audit fails on foreign Codex hooks"
  fi
}

test_prunes_foreign_handlers() {
  local codex_home="$TMP_DIR/prune/.codex"
  local output
  setup_hooks_fixture "$codex_home"

  output="$(bash "$SCRIPT" --codex-home "$codex_home" --prune-foreign --json)"

  if ! jq -e '.foreign_handlers == 0 and .agentops_handlers == 4' <<< "$output" >/dev/null; then
    fail "prune output did not report only AgentOps handlers: $output"
    return
  fi
  if ! jq -e '[.hooks | to_entries[] | .value[] | .hooks[] | select(.command | contains("/custom/"))] | length == 0' \
    "$codex_home/hooks.json" >/dev/null; then
    fail "foreign hooks still present after prune"
    return
  fi
  if ! jq -e '[.hooks | to_entries[] | .value[] | .hooks[] | select(.command | contains("/hooks/"))] | length == 4' \
    "$codex_home/hooks.json" >/dev/null; then
    fail "AgentOps hooks were not preserved after prune"
    return
  fi
  if ! find "$codex_home" -maxdepth 1 -name 'hooks.json.bak.*' | grep -q .; then
    fail "prune did not create a backup"
    return
  fi

  pass "prunes foreign hooks and preserves AgentOps handlers"
}

test_repairs_legacy_flat_shape() {
  local codex_home="$TMP_DIR/repair/.codex"
  local output
  mkdir -p "$codex_home"
  cat > "$codex_home/.agentops-codex-install.json" <<'EOF'
{"plugin_root":"/tmp/agentops-plugin"}
EOF
  cat > "$codex_home/hooks.json" <<'EOF'
{
  "$schema": "../schemas/hooks-manifest.v1.schema.json",
  "hooks": [
    {
      "name": "agentops-session-start",
      "event": "SessionStart",
      "command": "bash /old/hooks/session-start.sh",
      "timeout": 10000
    }
  ]
}
EOF

  output="$(bash "$SCRIPT" --codex-home "$codex_home" --repair-shape --json)"

  if ! jq -e '.hooks | type == "object" and length == 5' "$codex_home/hooks.json" >/dev/null; then
    fail "repair did not write native event-map hooks.json"
    return
  fi
  if ! jq -e '[.hooks | to_entries[] | .value[] | .hooks[]] | length == 20' "$codex_home/hooks.json" >/dev/null; then
    fail "repair did not restore the canonical handler set"
    return
  fi
  if ! jq -e '[.hooks | to_entries[] | .value[] | .hooks[] | select(.command | contains("/tmp/agentops-plugin/hooks/"))] | length == 20' \
    "$codex_home/hooks.json" >/dev/null; then
    fail "repair did not render commands with the active plugin root"
    return
  fi
  if ! jq -e '.total_handlers == 20 and .agentops_handlers == 20 and .foreign_handlers == 0' <<< "$output" >/dev/null; then
    fail "repair audit output had unexpected counts: $output"
    return
  fi
  if ! find "$codex_home" -maxdepth 1 -name 'hooks.json.bak.*' | grep -q .; then
    fail "repair did not create a backup"
    return
  fi

  pass "repairs legacy flat Codex hook manifests"
}

echo "== test-audit-codex-hooks =="
test_reports_foreign_handlers
test_prunes_foreign_handlers
test_repairs_legacy_flat_shape

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
