#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-skill-runtime-parity.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

if [[ ! -f "$SCRIPT" ]]; then
  echo "FAIL: missing script: $SCRIPT" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"
  mkdir -p "$fixture/scripts" "$fixture/cli/internal/quality" "$fixture/skills/fixture" "$fixture/skills-codex/fixture"
  cp "$SCRIPT" "$fixture/scripts/validate-skill-runtime-parity.sh"

  cat > "$fixture/cli/internal/quality/stale_refs.go" <<'EOF'
package quality

var DeprecatedCommands = map[string]string{
	"ao work goals": "ao goals",
	"ao know lookup": "ao lookup",
	"ao quality metrics": "ao metrics",
	"ao start init": "ao init",
}
EOF
}

write_skill() {
  local path="$1"
  local body="$2"
  cat > "$path" <<EOF
---
name: fixture
description: fixture
---

$body
EOF
}

run_fixture() {
  local fixture="$1"
  local out_file="$2"
  (
    cd "$fixture"
    bash scripts/validate-skill-runtime-parity.sh
  ) > "$out_file" 2>&1
}

test_pass_with_current_commands() {
  local fixture="$TMP_DIR/pass"
  local out="$fixture/out.txt"

  setup_fixture "$fixture"
  write_skill "$fixture/skills/fixture/SKILL.md" "Use \`ao goals measure --json\` and \`ao lookup --query \\\"topic\\\"\`."
  write_skill "$fixture/skills-codex/fixture/SKILL.md" "Use \`ao metrics flywheel status\` after \`ao init --hooks --minimal-hooks\`."

  if run_fixture "$fixture" "$out"; then
    pass "passes when skill docs use current ao commands and hook claims"
  else
    fail "should pass with current commands"
    sed 's/^/  /' "$out"
  fi
}

test_fail_on_deprecated_ao_command() {
  local fixture="$TMP_DIR/fail-deprecated"
  local out="$fixture/out.txt"

  setup_fixture "$fixture"
  write_skill "$fixture/skills/fixture/SKILL.md" "Run \`ao work goals measure\` before continuing."
  write_skill "$fixture/skills-codex/fixture/SKILL.md" "Current command is \`ao goals measure\`."

  if run_fixture "$fixture" "$out"; then
    fail "should fail on deprecated ao command reference"
    return
  fi

  if grep -q "deprecated command reference found: ao work goals" "$out"; then
    pass "fails on deprecated ao command reference"
  else
    fail "missing deprecated command finding"
  fi
}

test_fail_on_stale_hook_claim() {
  local fixture="$TMP_DIR/fail-hook-claim"
  local out="$fixture/out.txt"

  setup_fixture "$fixture"
  write_skill "$fixture/skills/fixture/SKILL.md" "Use \`ao init --hooks --full\` for all 8 events."
  write_skill "$fixture/skills-codex/fixture/SKILL.md" "Minimal mode is SessionStart + Stop."

  if run_fixture "$fixture" "$out"; then
    fail "should fail on stale hook-install claims"
    return
  fi

  if grep -q "hook coverage count is stale" "$out" && grep -q "SessionStart + SessionEnd + Stop" "$out"; then
    pass "fails on stale hook-install claims"
  else
    fail "missing stale hook-install finding"
  fi
}

echo "== test-skill-runtime-parity =="
test_pass_with_current_commands
test_fail_on_deprecated_ao_command
test_fail_on_stale_hook_claim

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
