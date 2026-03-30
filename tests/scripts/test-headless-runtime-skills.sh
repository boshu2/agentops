#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-headless-runtime-skills.sh"

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

make_fixture() {
    local root="$1"

    mkdir -p \
        "$root/scripts" \
        "$root/skills/athena" \
        "$root/skills/research" \
        "$root/skills-codex/athena" \
        "$root/skills-codex/research"

    cat > "$root/scripts/install-codex-plugin.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

codex_home=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --codex-home)
      codex_home="${2:-}"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -z "$codex_home" ]]; then
  echo "missing --codex-home" >&2
  exit 2
fi

mkdir -p "$codex_home"
cat > "$codex_home/.agentops-codex-install.json" <<'JSON'
{"installed":true}
JSON
EOF
    chmod +x "$root/scripts/install-codex-plugin.sh"

    cat > "$root/skills/athena/SKILL.md" <<'EOF'
---
name: athena
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

    cat > "$root/skills-codex/athena/SKILL.md" <<'EOF'
---
name: athena
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
    local mode="${2:-pass}"
    cat > "$bin_dir/claude" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

prompt=""
saw_help=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p)
      prompt="${2:-}"
      shift 2
      ;;
    *)
      if [[ "$1" == "--help" ]]; then
        saw_help=1
      fi
      shift
      ;;
  esac
done

mode="__MODE__"
state_file="__STATE_FILE__"
if [[ "$saw_help" == "1" ]]; then
  echo "Claude help"
  exit 0
fi
if [[ "$mode" == "fallback" ]]; then
  exit 124
fi
if [[ "$mode" == "malformed" ]]; then
  python3 - <<'PY'
import json

for payload in (
    {"type": "assistant", "message": {"content": [{"type": "text", "text": 'not-json'}]}},
    {"type": "result"},
):
    print(json.dumps(payload))
PY
  exit 0
fi

if [[ "$prompt" == *"compact JSON array of skill names"* ]]; then
  text='["athena","research"]'
  if [[ "$mode" == "retry-missing" ]]; then
    count=0
    if [[ -f "$state_file" ]]; then
      count="$(cat "$state_file")"
    fi
    count=$((count + 1))
    printf '%s' "$count" > "$state_file"
    if [[ "$count" -eq 1 ]]; then
      text='["athena"]'
    fi
  fi
  if [[ "$mode" == "malformed" ]]; then
    python3 - <<'PY'
import json

for payload in (
    {"type": "assistant", "message": {"content": [{"type": "text", "text": 'not-json'}]}},
    {"type": "result"},
):
    print(json.dumps(payload))
PY
    exit 0
  fi
  MOCK_CLAUDE_TEXT="$text" python3 - <<'PY'
import json
import os

text = os.environ["MOCK_CLAUDE_TEXT"]
for payload in (
    {"type": "assistant", "message": {"content": [{"type": "text", "text": text}]}},
    {"type": "result"},
):
    print(json.dumps(payload))
PY
  exit 0
fi

echo "unexpected Claude prompt: $prompt" >&2
exit 1
EOF
    python3 - <<'PY' "$bin_dir/claude" "$mode" "$bin_dir/.claude-state"
from pathlib import Path
import sys

path = Path(sys.argv[1])
mode = sys.argv[2]
state_file = sys.argv[3]
path.write_text(
    path.read_text()
    .replace("__MODE__", mode)
    .replace("__STATE_FILE__", state_file)
)
PY
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
        text = '[{"name":"athena","description":"Active knowledge intelligence. Runs Mine → Grow → Defrag cycle."},{"name":"research","description":"Deep codebase exploration."}]'
elif mode == "missing":
    text = '[{"name":"research","description":"Deep codebase exploration."}]'
else:
    text = '[{"name":"athena","description":"Active knowledge intelligence. Runs Mine → Grow → Defrag cycle."},{"name":"research","description":"Deep codebase exploration."}]'

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
        pass "passes with mocked Claude and Codex inventories"
    else
        fail "passes with mocked Claude and Codex inventories"
        sed -n '1,80p' "$TMP_DIR/pass.log" >&2
    fi
}

test_fails_when_codex_inventory_is_missing_skill() {
    local repo="$TMP_DIR/fail-repo"
    local bin_dir="$TMP_DIR/fail-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir"
    make_mock_codex "$bin_dir" missing

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime codex --workdir "$TMP_DIR/workdir-fail" >"$TMP_DIR/fail.log" 2>&1; then
        fail "fails when Codex inventory is missing a skill"
    elif rg -q 'missing skills: athena' "$TMP_DIR/fail.log"; then
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
    make_mock_claude "$bin_dir"
    make_mock_codex "$bin_dir" retry-missing

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime codex --workdir "$TMP_DIR/workdir-codex-retry" \
        >"$TMP_DIR/codex-retry.log" 2>&1; then
        if rg -q 'Codex inventory mismatch on attempt 1/2; retrying' "$TMP_DIR/codex-retry.log" && \
            rg -q 'codex: inventory verified' "$TMP_DIR/codex-retry.log"; then
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

test_warns_and_passes_when_claude_inventory_falls_back_to_help() {
    local repo="$TMP_DIR/fallback-repo"
    local bin_dir="$TMP_DIR/fallback-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir" fallback

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime claude --workdir "$TMP_DIR/workdir-fallback" >"$TMP_DIR/fallback.log" 2>&1; then
        if rg -q 'load-check fallback passed' "$TMP_DIR/fallback.log"; then
            pass "warns and passes when Claude inventory falls back to explicit load-check fallback"
        else
            fail "warns and passes when Claude inventory falls back to explicit load-check fallback"
            sed -n '1,80p' "$TMP_DIR/fallback.log" >&2
        fi
    else
        fail "warns and passes when Claude inventory falls back to explicit load-check fallback"
        sed -n '1,80p' "$TMP_DIR/fallback.log" >&2
    fi
}

test_warns_and_passes_when_claude_output_is_malformed() {
    local repo="$TMP_DIR/malformed-repo"
    local bin_dir="$TMP_DIR/malformed-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir" malformed

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime claude --workdir "$TMP_DIR/workdir-malformed" >"$TMP_DIR/malformed.log" 2>&1; then
        if rg -q 'assistant output was not a JSON array' "$TMP_DIR/malformed.log" && \
            rg -q 'load-check fallback passed' "$TMP_DIR/malformed.log"; then
            pass "warns and passes when Claude output is malformed but load check succeeds"
        else
            fail "warns and passes when Claude output is malformed but load check succeeds"
            sed -n '1,80p' "$TMP_DIR/malformed.log" >&2
        fi
    else
        fail "warns and passes when Claude output is malformed but load check succeeds"
        sed -n '1,80p' "$TMP_DIR/malformed.log" >&2
    fi
}

test_retries_when_claude_inventory_omits_skill_once() {
    local repo="$TMP_DIR/retry-repo"
    local bin_dir="$TMP_DIR/retry-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir" retry-missing

    if PATH="$bin_dir:$PATH" bash "$SCRIPT" --repo-root "$repo" --runtime claude --workdir "$TMP_DIR/workdir-retry" \
        >"$TMP_DIR/retry.log" 2>&1; then
        if rg -q 'inventory mismatch on attempt 1/2; retrying' "$TMP_DIR/retry.log" && \
            rg -q 'claude: inventory verified' "$TMP_DIR/retry.log"; then
            pass "retries when Claude inventory omits a skill once"
        else
            fail "retries when Claude inventory omits a skill once"
            sed -n '1,80p' "$TMP_DIR/retry.log" >&2
        fi
    else
        fail "retries when Claude inventory omits a skill once"
        sed -n '1,80p' "$TMP_DIR/retry.log" >&2
    fi
}

test_fails_in_strict_mode_when_claude_uses_load_check_fallback() {
    local repo="$TMP_DIR/strict-repo"
    local bin_dir="$TMP_DIR/strict-bin"
    mkdir -p "$bin_dir"
    make_fixture "$repo"
    make_mock_claude "$bin_dir" fallback

    if HEADLESS_RUNTIME_SKILL_CLAUDE_STRICT=1 PATH="$bin_dir:$PATH" \
        bash "$SCRIPT" --repo-root "$repo" --runtime claude --workdir "$TMP_DIR/workdir-strict" \
        >"$TMP_DIR/strict.log" 2>&1; then
        fail "fails in strict mode when Claude falls back to load-check"
    elif rg -q 'requires verified Claude inventory' "$TMP_DIR/strict.log"; then
        pass "fails in strict mode when Claude falls back to load-check"
    else
        fail "fails in strict mode when Claude falls back to load-check"
        sed -n '1,80p' "$TMP_DIR/strict.log" >&2
    fi
}

echo "== test-headless-runtime-skills =="
test_passes_with_mocked_runtimes
test_fails_when_codex_inventory_is_missing_skill
test_retries_when_codex_inventory_omits_skill_once
test_warns_and_passes_when_claude_inventory_falls_back_to_help
test_warns_and_passes_when_claude_output_is_malformed
test_retries_when_claude_inventory_omits_skill_once
test_fails_in_strict_mode_when_claude_uses_load_check_fallback

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
exit 0
