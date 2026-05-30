#!/usr/bin/env bats
# ag-mptr: agent-output-validate.sh is the gate logic behind the reusable
# .github/workflows/agent-output-validate.yml — it runs `ao validate --gate`
# against an agent-produced PR/artifact and propagates the verdict to a process
# exit code (the SAME authoritative gate as interactive work; CI is the gate,
# not a hook). These cases are the passing+failing fixture evidence required by
# the bead's acceptance, exercised deterministically via an injectable fake ao
# (AO_BIN) so they don't depend on constructing real ratchet-failing artifacts.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/agent-output-validate.sh"
  WORKFLOW="$BATS_TEST_DIRNAME/../../.github/workflows/agent-output-validate.yml"
  FIX="$(mktemp -d)"
  # Fake ao binaries: each echoes its invocation and exits with a fixed code,
  # matching `ao validate --gate` semantics (0 PASS/WARN, 1 FAIL, 2 error).
  for name in pass fail err; do
    case "$name" in
      pass) code=0 ;;
      fail) code=1 ;;
      err)  code=2 ;;
    esac
    cat > "$FIX/ao-$name" <<EOF
#!/usr/bin/env bash
echo "FAKE ao \$*"
exit $code
EOF
    chmod +x "$FIX/ao-$name"
  done
}

teardown() { rm -rf "$FIX"; }

@test "PASS fixture: a clean gate (ao exits 0) makes the script exit 0" {
  run env AO_BIN="$FIX/ao-pass" bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"validate --gate"* ]]
}

@test "FAIL fixture: a violating gate (ao exits 1) makes the script exit 1" {
  run env AO_BIN="$FIX/ao-fail" bash "$SCRIPT"
  [ "$status" -eq 1 ]
}

@test "internal error (ao exits 2) is propagated, not swallowed as a pass" {
  run env AO_BIN="$FIX/ao-err" bash "$SCRIPT"
  [ "$status" -eq 2 ]
}

@test "missing ao binary fails loudly (does not silently pass)" {
  run env AO_BIN="$FIX/does-not-exist" bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"ao"* ]]
}

@test "forwards scope flags (--changes) through to ao validate --gate" {
  run env AO_BIN="$FIX/ao-pass" bash "$SCRIPT" --changes plan.md
  [ "$status" -eq 0 ]
  [[ "$output" == *"--changes plan.md"* ]]
}

@test "script is executable and uses strict bash mode" {
  [ -x "$SCRIPT" ]
  grep -qE "set -euo pipefail" "$SCRIPT"
}

@test "reusable workflow exists, is workflow_call, and runs the script" {
  [ -f "$WORKFLOW" ]
  grep -qE "^\s*workflow_call:" "$WORKFLOW"
  grep -q "agent-output-validate.sh" "$WORKFLOW"
}
