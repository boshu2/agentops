#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-skill-cli-snippets.sh"

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
  local repo="$1"
  mkdir -p "$repo/scripts" "$repo/skills/example" "$repo/skills-codex/example" "$repo/cli"
  cp "$SCRIPT" "$repo/scripts/validate-skill-cli-snippets.sh"
  chmod +x "$repo/scripts/validate-skill-cli-snippets.sh"

  cat > "$repo/fake-ao" <<'EOF'
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
  *)
    exit 1
    ;;
esac
EOF
  chmod +x "$repo/fake-ao"
}

test_passes_for_current_commands() {
  local repo="$TMP_DIR/pass"
  setup_fixture "$repo"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
Use `ao lookup --query "topic" --json`.
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
Use `ao goals measure --json`.
EOF

  if (cd "$repo" && AGENTOPS_AO_BIN="$repo/fake-ao" bash scripts/validate-skill-cli-snippets.sh >/dev/null); then
    pass "passes for valid ao command snippets"
  else
    fail "should pass for valid ao command snippets"
  fi
}

test_fails_for_unknown_command() {
  local repo="$TMP_DIR/fail-command"
  setup_fixture "$repo"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
Use `ao work goals`.
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
Use `ao lookup --query "topic"`.
EOF

  if (cd "$repo" && AGENTOPS_AO_BIN="$repo/fake-ao" bash scripts/validate-skill-cli-snippets.sh >/dev/null 2>&1); then
    fail "should fail for unknown ao command snippets"
  else
    pass "fails for unknown ao command snippets"
  fi
}

test_fails_for_unknown_flag() {
  local repo="$TMP_DIR/fail-flag"
  setup_fixture "$repo"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
Use `ao lookup --badflag`.
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
Use `ao goals measure --json`.
EOF

  if (cd "$repo" && AGENTOPS_AO_BIN="$repo/fake-ao" bash scripts/validate-skill-cli-snippets.sh >/dev/null 2>&1); then
    fail "should fail for unknown flags"
  else
    pass "fails for unknown flags"
  fi
}

test_passes_for_pipeline_and_placeholder_flags() {
  local repo="$TMP_DIR/pipeline-placeholder"
  setup_fixture "$repo"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
Use `ao lookup --query="topic" --json | head -20`.
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
Use `ao --help` and `ao goals measure --json`.
EOF

  if (cd "$repo" && AGENTOPS_AO_BIN="$repo/fake-ao" bash scripts/validate-skill-cli-snippets.sh >/dev/null); then
    pass "passes for shell pipelines and normalized flag values"
  else
    fail "should pass for shell pipelines and normalized flag values"
  fi
}

test_fails_for_stale_beads_resolver() {
  local repo="$TMP_DIR/fail-beads-resolver"
  setup_fixture "$repo"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
Read the bead with `BEADS_DIR=$PWD/_beads br show ag-123`.
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
Use `ao lookup --query "topic"`.
EOF

  if (cd "$repo" && AGENTOPS_AO_BIN="$repo/fake-ao" bash scripts/validate-skill-cli-snippets.sh >/dev/null 2>&1); then
    fail "should fail for stale BEADS_DIR=$PWD/_beads skill examples"
  else
    pass "fails for stale BEADS_DIR=$PWD/_beads skill examples"
  fi
}

echo "== test-skill-cli-snippets =="
test_passes_for_current_commands
test_fails_for_unknown_command
test_fails_for_unknown_flag
test_passes_for_pipeline_and_placeholder_flags
test_fails_for_stale_beads_resolver

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
