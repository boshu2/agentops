---
id: pattern-2026-05-01-hook-fires-test
type: pattern
date: 2026-05-01
maturity: provisional
utility: 0.7
source: epic soc-irg1 (I3 edit-scope guard port)
---

# Pattern: L4 hook-fires test (simulate harness stdin contract)

## When to Apply

Any time you ship a new shell hook (PreToolUse / PostToolUse / SessionStart / etc.) that reads tool input from stdin. `bash -n` only validates parse, not behavior. A unit-level test of the hook script in isolation tells you nothing about whether the hook FIRES correctly when registered with the harness.

The pattern applies when ANY of:
- The hook is new (port from another tool, or first-of-its-kind)
- The hook reads structured input (JSON tool params)
- The hook has multiple decision branches (allow / block / fail-open)
- The hook is BLOCKING (matters that it returns non-zero on the right path)

## Pattern

```bash
#!/usr/bin/env bash
# tests/hooks/test-<hook-name>-fires.sh
set -e
PASS=0; FAIL=0
HOOK="hooks/<hook-name>.sh"

run_case() {
  local name="$1"; local expected_exit="$2"; local input="$3"
  local actual_exit=0
  echo "$input" | bash "$HOOK" >/dev/null 2>&1 || actual_exit=$?
  if [ "$actual_exit" -eq "$expected_exit" ]; then
    echo "PASS: $name (exit $actual_exit)"
    PASS=$((PASS+1))
  else
    echo "FAIL: $name (expected $expected_exit, got $actual_exit)"
    FAIL=$((FAIL+1))
  fi
}

# CRITICAL CASES — must cover all of:
# 1. Allow path (hook returns 0 when nothing to block)
# 2. Block path (hook returns non-zero when blocking)
# 3. Fail-open on malformed JSON (defensive parse)
# 4. Fail-open on missing required fields
# 5. Edge cases specific to the hook's logic

run_case "edit inside frozen scope allowed"   0  '{"tool":{"name":"Edit","params":{"file_path":"frozen-dir/foo"}}}'
run_case "edit outside frozen scope blocked"  2  '{"tool":{"name":"Edit","params":{"file_path":"other/foo"}}}'
run_case "malformed JSON fails open"          0  '{"malformed'
run_case "missing lock file allows edit"      0  '{"tool":{"name":"Edit","params":{"file_path":"any/path"}}}'
run_case "empty frozen_dirs allows edit"      0  '{"tool":{"name":"Edit","params":{"file_path":"any/path"}}}'
run_case "missing target path allows edit"    0  '{"tool":{"name":"Edit","params":{}}}'
run_case "edit in nested frozen subdir allowed" 0 '{"tool":{"name":"Edit","params":{"file_path":"frozen-dir/sub/x"}}}'

echo "===== summary: $PASS passed, $FAIL failed ====="
[ "$FAIL" -eq 0 ]
```

## Why It Matters

Three failure modes this pattern catches:
1. **The hook script crashes on malformed input** → blocks all edits silently. (`bash -n` doesn't detect this.)
2. **The hook fails to extract the target path correctly** → false-allows or false-blocks. Real Claude Code stdin is `{"tool":{"name":"Edit","params":{"file_path":"..."}}}` for Edit/Write but `{"tool":{"name":"Bash","params":{"command":"..."}}}` for Bash — different shapes. Hook must handle both.
3. **The hook is registered in `hooks/hooks.json` but the matcher doesn't trigger it** → orthogonal to the script's correctness. (Catch via integration test in `tests/install/`.)

Cost of writing this test: ~30 minutes for a new hook. Cost of a false-blocking hook in production: hours of confused debugging while every edit fails for unknown reasons.

## Counter-Applies

- The hook is purely advisory (always exits 0 regardless of input) → unit test is fine
- The hook is documented as fail-closed AND has audit logging → blocking on malformed input may be intentional
- The hook is auto-generated from a higher-level spec → test the generator, not the output

## Reference Implementation

`tests/hooks/test-edit-scope-guard-fires.sh` (118 lines, 7 cases) — shipped in epic soc-irg1, commit `890bdf0f`.
