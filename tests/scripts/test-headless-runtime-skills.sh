#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-headless-runtime-skills.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }
contains_text() { grep -F -q -- "$1" "$2"; }

if [[ ! -f "$SCRIPT" ]]; then
    echo "FAIL: missing script: $SCRIPT" >&2
    exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

make_fixture() {
    local root="$1"

    mkdir -p \
        "$root/scripts" \
        "$root/cli/bin" \
        "$root/skills/compile" \
        "$root/skills/research" \
        "$root/skills-codex/compile" \
        "$root/skills-codex/research"

    # Stub ao that implements skills link for the headless Codex setup path.
    cat > "$root/cli/bin/ao" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "skills" && "${2:-}" == "link" ]]; then
  dest=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dest) dest="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  if [[ -n "$dest" ]]; then
    mkdir -p "$dest"
  fi
  exit 0
fi
exit 0
EOF
    chmod +x "$root/cli/bin/ao"

    cat > "$root/skills/compile/SKILL.md" <<'EOF'
---
name: compile
description: >
  Active knowledge intelligence. Runs Mine → Grow → Defrag cycle.
skill_api_version: 1
---
EOF

    cat > "$root/skills/research/SKILL.md" <<'EOF'
---
name: research
description: 'Deep codebase exploration.'
skill_api_version: 1
---
EOF

    cat > "$root/skills-codex/compile/SKILL.md" <<'EOF'
---
name: compile
description: 'Active knowledge intelligence. Runs Mine → Grow → Defrag cycle.'
skill_api_version: 1
---
EOF

    cat > "$root/skills-codex/research/SKILL.md" <<'EOF'
---
name: research
description: 'Deep codebase exploration.'
skill_api_version: 1
---
EOF
}

make_mock_claude() {
    local bin_dir="$1"
    cat > "$bin_dir/claude" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

saw_help=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--print)
      echo "mock claude refuses print invocation" >&2
      exit 99
      ;;
    *)
      if [[ "$1" == "--help" ]]; then
        saw_help=1
      fi
      shift
      ;;
  esac
done

if [[ "$saw_help" == "1" ]]; then
  echo "Claude help"
  exit 0
fi

echo "unexpected Claude args: $*" >&2
exit 1
EOF
    chmod +x "$bin_dir/claude"
}

make_mock_codex() {
    local bin_dir="$1"
    local mode="${2:-pass}"
    cat > "$bin_dir/codex" <<EOF
#!/usr/bin/env bash
set -euo pipefail

if [[ "\$1" != "exec" ]]; then
  echo "unexpected codex command: \$*" >&2
  exit 1
fi

state_file="$(dirname "$0")/.codex-state"
python3 - <<'PY'
import json
from pathlib import Path

mode = ${mode@Q}
state_file = Path(${bin_dir@Q}) / ".codex-state"
if mode == "retry-missing":
    count = int(state_file.read_text().strip()) if state_file.exists() else 0
    count += 1
    state_file.write_text(str(count))
    if count == 1:
        text = '[{"name":"research","description":"Deep codebase exploration."}]'
    else:
        text = '[{"name":"compile","description":"Active knowledge intelligence. Runs Mine → Grow → Defrag cycle."},{"name":"research","description":"Deep codebase exploration."}]'
elif mode == "missing":
    text = '[{"name":"research","description":"Deep codebase exploration."}]'
else:
    text = '[{"name":"compile","description":"Knowledge compiler. Reads raw .agents/ artifacts and compiles them into an interlinked markdown wiki."},{"name":"research","description":"Deep codebase exploration."}]'

for payload in (
    {"type": "thread.started", "thread_id": "fixture"},
    {"type": "turn.started"},
    {"type": "item.completed", "item": {"id": "item_0", "type": "agent_message", "text": text}},
    {"type": "turn.completed"},
):
    print(json.dumps(payload))
PY
EOF
    chmod +x "$bin_dir/codex"
}

test_passes_with_mocked_runtimes() {
    local repo="$TMP_DIR/pass-repo"
    local bin_dir="$TMP_DIR/pass-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir"
    make_mock_codex "$bin_dir" pass

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --workdir "$TMP_DIR/workdir-pass" >"$TMP_DIR/pass.log" 2>&1; then
        pass "passes with mocked Claude load check and Codex inventory"
    else
        fail "passes with mocked Claude load check and Codex inventory"
        sed -n '1,80p' "$TMP_DIR/pass.log" >&2
    fi
}

test_fails_when_codex_inventory_is_missing_skill() {
    local repo="$TMP_DIR/fail-repo"
    local bin_dir="$TMP_DIR/fail-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_codex "$bin_dir" missing

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime codex --workdir "$TMP_DIR/workdir-fail" >"$TMP_DIR/fail.log" 2>&1; then
        fail "fails when Codex inventory is missing a skill"
    elif contains_text 'missing skills: compile' "$TMP_DIR/fail.log"; then
        pass "fails when Codex inventory is missing a skill"
    else
        fail "fails when Codex inventory is missing a skill"
        sed -n '1,80p' "$TMP_DIR/fail.log" >&2
    fi
}

test_retries_when_codex_inventory_omits_skill_once() {
    local repo="$TMP_DIR/codex-retry-repo"
    local bin_dir="$TMP_DIR/codex-retry-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_codex "$bin_dir" retry-missing

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime codex --workdir "$TMP_DIR/workdir-codex-retry" \
        >"$TMP_DIR/codex-retry.log" 2>&1; then
        if contains_text 'Codex inventory mismatch on attempt 1/2; retrying' "$TMP_DIR/codex-retry.log" && \
            contains_text 'codex: inventory verified' "$TMP_DIR/codex-retry.log"; then
            pass "retries when Codex inventory omits a skill once"
        else
            fail "retries when Codex inventory omits a skill once"
            sed -n '1,80p' "$TMP_DIR/codex-retry.log" >&2
        fi
    else
        fail "retries when Codex inventory omits a skill once"
        sed -n '1,80p' "$TMP_DIR/codex-retry.log" >&2
    fi
}

test_claude_runtime_uses_non_print_load_check_only() {
    local repo="$TMP_DIR/claude-load-repo"
    local bin_dir="$TMP_DIR/claude-load-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir"

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime claude --workdir "$TMP_DIR/workdir-claude-load" >"$TMP_DIR/claude-load.log" 2>&1; then
        if contains_text 'claude: non-print load check passed' "$TMP_DIR/claude-load.log"; then
            pass "Claude runtime uses non-print load check only"
        else
            fail "Claude runtime uses non-print load check only"
            sed -n '1,80p' "$TMP_DIR/claude-load.log" >&2
        fi
    else
        fail "Claude runtime uses non-print load check only"
        sed -n '1,80p' "$TMP_DIR/claude-load.log" >&2
    fi
}

echo "== test-headless-runtime-skills =="
test_passes_with_mocked_runtimes
test_fails_when_codex_inventory_is_missing_skill
test_retries_when_codex_inventory_omits_skill_once
test_claude_runtime_uses_non_print_load_check_only

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
exit 0
