#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/check-skill-flag-refs.sh"
TARGET_SCRIPT="$ROOT/scripts/validate-skill-cli-snippets.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

[[ -f "$SCRIPT" ]] || {
  echo "FAIL: missing script: $SCRIPT" >&2
  exit 1
}

[[ -f "$TARGET_SCRIPT" ]] || {
  echo "FAIL: missing script: $TARGET_SCRIPT" >&2
  exit 1
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"
  mkdir -p "$fixture/scripts" "$fixture/skills/fixture" "$fixture/skills/other" "$fixture/cli/cmd/ao" "$fixture/cli/docs"
  cp "$SCRIPT" "$fixture/scripts/check-skill-flag-refs.sh"
  cp "$TARGET_SCRIPT" "$fixture/scripts/validate-skill-cli-snippets.sh"
  chmod +x "$fixture/scripts/check-skill-flag-refs.sh" "$fixture/scripts/validate-skill-cli-snippets.sh"

  cat > "$fixture/cli/docs/COMMANDS.md" <<'EOF'
### `ao lookup`
### `ao goals`
#### `ao goals measure`
### `ao hooks`
#### `ao hooks install`
EOF

  cat > "$fixture/cli/cmd/ao/root.go" <<'EOF'
package main
func f() {
  cmd.Flags().Bool("json", false, "")
  cmd.Flags().Bool("full", false, "")
  cmd.Flags().String("query", "", "")
}
EOF

  cat > "$fixture/fake-ao" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" != "help" ]]; then
  exit 1
fi
shift
case "$*" in
  "")
    cat <<'INNER'
Usage:
  ao [command]

Flags:
  -h, --help
INNER
    exit 0
    ;;
  "lookup")
    cat <<'INNER'
Usage:
  ao lookup [flags]

Flags:
      --query string
      --json
INNER
    exit 0
    ;;
  "goals measure")
    cat <<'INNER'
Usage:
  ao goals measure [flags]

Flags:
      --json
INNER
    exit 0
    ;;
  "hooks install")
    cat <<'INNER'
Usage:
  ao hooks install [flags]

Flags:
      --full
INNER
    exit 0
    ;;
  *)
    exit 1
    ;;
esac
EOF
  chmod +x "$fixture/fake-ao"
}

write_doc() {
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

test_passes_with_valid_examples() {
  local fixture="$TMP_DIR/pass"
  setup_fixture "$fixture"
  write_doc "$fixture/skills/fixture/SKILL.md" "Use \`ao goals measure --json\` and \`ao lookup --query \"x\"\`."
  write_doc "$fixture/skills/other/SKILL.md" "Install hooks with \`ao hooks install --full\`."

  if (cd "$fixture" && AGENTOPS_AO_BIN="$fixture/fake-ao" bash ./scripts/check-skill-flag-refs.sh >/dev/null); then
    pass "passes with valid command and flag examples"
  else
    fail "should pass with valid command and flag examples"
  fi
}

test_fails_on_unknown_command() {
  local fixture="$TMP_DIR/fail-command"
  setup_fixture "$fixture"
  write_doc "$fixture/skills/fixture/SKILL.md" "Run \`ao madeup command --json\`."
  write_doc "$fixture/skills/other/SKILL.md" "Use \`ao lookup --query \"x\"\`."

  if (cd "$fixture" && AGENTOPS_AO_BIN="$fixture/fake-ao" bash ./scripts/check-skill-flag-refs.sh >/dev/null 2>&1); then
    fail "should fail on unknown command"
  else
    pass "fails on unknown command"
  fi
}

test_fails_on_unknown_flag() {
  local fixture="$TMP_DIR/fail-flag"
  setup_fixture "$fixture"
  write_doc "$fixture/skills/fixture/SKILL.md" "Use \`ao goals measure --bogus\`."
  write_doc "$fixture/skills/other/SKILL.md" "Use \`ao lookup --query \"x\"\`."

  if (cd "$fixture" && AGENTOPS_AO_BIN="$fixture/fake-ao" bash ./scripts/check-skill-flag-refs.sh >/dev/null 2>&1); then
    fail "should fail on unknown flag"
  else
    pass "fails on unknown flag"
  fi
}

echo "== test-skill-cli-examples =="
test_passes_with_valid_examples
test_fails_on_unknown_command
test_fails_on_unknown_flag

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
