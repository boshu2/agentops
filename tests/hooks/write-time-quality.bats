#!/usr/bin/env bats
#
# Tests for hooks/write-time-quality.sh — pins the per-language quality
# heuristics (Go fmt.Println in library code, Python bare except / eval /
# missing return type hints, shell set -euo pipefail / unquoted vars), the
# IS_TEST exemption logic, the tool-name filter, and JSON output shape.
# The hook had zero coverage; a regression in any heuristic would silently
# drop signal on every Edit/Write.

setup() {
    load helpers/test_helper
    _helper_setup
    HOOK="$REPO_ROOT/hooks/write-time-quality.sh"
}

teardown() {
    _helper_teardown
}

# fire FILE_PATH TOOL_NAME — pipes a tool_use payload at the hook with the
# given file_path and tool name, captures combined stdout+stderr in $output
# and the exit status in $status. The hook prints JSON on stdout and a
# human-readable digest on stderr; merging both lets warning-substring
# assertions match either stream while JSON-shape tests use $stdout_only.
fire() {
    local file="$1"
    local tool="${2:-Write}"
    local payload
    payload=$(jq -n --arg t "$tool" --arg f "$file" \
        '{"tool_name":$t,"tool_input":{"file_path":$f}}')
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$payload" "$HOOK"
}

# fire_stdout FILE_PATH TOOL_NAME — same as fire but captures only stdout
# (the JSON envelope), used for JSON-shape assertions where the stderr
# digest would corrupt jq parsing.
fire_stdout() {
    local file="$1"
    local tool="${2:-Write}"
    local payload
    payload=$(jq -n --arg t "$tool" --arg f "$file" \
        '{"tool_name":$t,"tool_input":{"file_path":$f}}')
    run bash -c 'printf "%s" "$1" | bash "$2" 2>/dev/null' -- "$payload" "$HOOK"
}

@test "non-Edit/Write tool is silently skipped" {
    local f="$TMP_TEST_DIR/x.go"
    cat > "$f" <<'EOF'
package main
func main() { fmt.Println("hi") }
EOF
    fire "$f" Read
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "missing FILE_PATH is silently skipped" {
    local payload='{"tool_name":"Write","tool_input":{}}'
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$payload" "$HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "non-existent file is silently skipped" {
    fire "$TMP_TEST_DIR/does-not-exist.go" Write
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "unsupported extension is silently skipped" {
    local f="$TMP_TEST_DIR/notes.txt"
    echo "hello" > "$f"
    fire "$f"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "AGENTOPS_HOOKS_DISABLED kill switch short-circuits" {
    local f="$TMP_TEST_DIR/lib.py"
    cat > "$f" <<'EOF'
def f(x):
    try:
        return x
    except:
        pass
EOF
    local payload
    payload=$(jq -n --arg t "Write" --arg f "$f" \
        '{"tool_name":$t,"tool_input":{"file_path":$f}}')
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' -- "$payload" "$HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "Go: fmt.Println in non-main package emits warning" {
    local f="$TMP_TEST_DIR/util.go"
    cat > "$f" <<'EOF'
package util

import "fmt"

func Hello() {
    fmt.Println("noisy library code")
}
EOF
    fire "$f"
    [ "$status" -eq 0 ]
    [[ "$output" == *"fmt.Print call"* ]]
    [[ "$output" == *"library code"* ]]
    # JSON envelope must parse and report >0 warnings (stdout-only to skip stderr)
    fire_stdout "$f"
    echo "$output" | jq -e '.hookSpecificOutput.warning_count > 0' >/dev/null
    echo "$output" | jq -e '.hookSpecificOutput.language == "go"' >/dev/null
}

@test "Go: fmt.Println in package main produces no warning" {
    local f="$TMP_TEST_DIR/main.go"
    cat > "$f" <<'EOF'
package main

import "fmt"

func main() {
    fmt.Println("hi")
}
EOF
    fire "$f"
    [[ "$output" != *"fmt.Print call"* ]]
}

@test "Go: fmt.Println in *_test.go produces no fmt warning" {
    local f="$TMP_TEST_DIR/util_test.go"
    cat > "$f" <<'EOF'
package util_test

import (
    "fmt"
    "testing"
)

func TestX(t *testing.T) {
    fmt.Println("test diag")
}
EOF
    fire "$f"
    [[ "$output" != *"fmt.Print call"* ]]
}

@test "Python: bare except: emits warning" {
    local f="$TMP_TEST_DIR/lib.py"
    cat > "$f" <<'EOF'
def f(x):
    try:
        return x / 0
    except:
        return 0
EOF
    fire "$f"
    [[ "$output" == *"bare except"* ]]
}

@test "Python: eval() outside test file emits warning" {
    local f="$TMP_TEST_DIR/runner.py"
    cat > "$f" <<'EOF'
def run(code):
    return eval(code)
EOF
    fire "$f"
    [[ "$output" == *"eval/exec"* ]]
}

@test "Python: eval() in test_*.py is exempted" {
    local f="$TMP_TEST_DIR/test_runner.py"
    cat > "$f" <<'EOF'
def test_eval():
    assert eval("1+1") == 2
EOF
    fire "$f"
    [[ "$output" != *"eval/exec"* ]]
}

@test "Python: public function without return type hint warns" {
    local f="$TMP_TEST_DIR/api.py"
    cat > "$f" <<'EOF'
def public_func(x):
    return x

def _private(x):
    return x
EOF
    fire "$f"
    [[ "$output" == *"return type hint"* ]]
}

@test "Shell: missing 'set -euo pipefail' warns" {
    local f="$TMP_TEST_DIR/script.sh"
    cat > "$f" <<'EOF'
#!/usr/bin/env bash
echo hi
EOF
    fire "$f"
    [[ "$output" == *"set -euo pipefail"* ]]
}

@test "Shell: header with 'set -euo pipefail' suppresses warning" {
    local f="$TMP_TEST_DIR/script.sh"
    cat > "$f" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "hi"
EOF
    fire "$f"
    [[ "$output" != *"missing 'set -euo pipefail'"* ]]
}

@test "Output JSON envelope includes file, language, warning_count, warnings array" {
    local f="$TMP_TEST_DIR/lib.py"
    cat > "$f" <<'EOF'
def f(x):
    try:
        return eval(x)
    except:
        return None
EOF
    fire_stdout "$f"
    echo "$output" | jq -e '.hookSpecificOutput.hookEventName == "write_time_quality"' >/dev/null
    echo "$output" | jq -e '.hookSpecificOutput.file' >/dev/null
    echo "$output" | jq -e '.hookSpecificOutput.language == "python"' >/dev/null
    echo "$output" | jq -e '.hookSpecificOutput.warnings | type == "array"' >/dev/null
    count=$(echo "$output" | jq '.hookSpecificOutput.warning_count')
    [ "$count" -ge 2 ]
}
