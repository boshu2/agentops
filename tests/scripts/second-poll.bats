#!/usr/bin/env bats
# ag-mg757: second-poll.sh runs the cross-family (gpt/codex) refuter and
# assembles the multi-model pawl verdict command as one operator move. These
# cases feed a fake CODEX_BIN so they assert the parse/assemble/surface behavior
# deterministically without a live codex.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/second-poll.sh"
  FIX="$(mktemp -d)"
  REPO="$(mktemp -d)"   # a throwaway repo dir for --repo / evidence writes

  cat > "$FIX/codex-confirmed" <<'EOF'
#!/usr/bin/env bash
echo "VERDICT: CONFIRMED"
echo "diff is clean, tests cover the acceptance"
EOF
  cat > "$FIX/codex-refuted" <<'EOF'
#!/usr/bin/env bash
echo "VERDICT: REFUTED"
echo "the edge case in foo() is unhandled"
EOF
  cat > "$FIX/codex-garbage" <<'EOF'
#!/usr/bin/env bash
echo "I am not sure how to respond"
EOF
  chmod +x "$FIX"/codex-*
}

teardown() { rm -rf "$FIX" "$REPO"; }

@test "CONFIRMED: emits a multi-model command with gpt CONFIRMED + claude placeholder" {
  CODEX_BIN="$FIX/codex-confirmed" run bash "$SCRIPT" ag-x 12 abc1234def --repo "$REPO"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q -- "--mode multi-model"
  echo "$output" | grep -q -- "--refuter gpt:CONFIRMED:"
  echo "$output" | grep -q -- "--refuter claude:<CONFIRMED|REFUTED>"
  echo "$output" | grep -qi "CONFIRMED"
}

@test "REFUTED: surfaced prominently, exit 1, no CONFIRMED-style verdict line" {
  CODEX_BIN="$FIX/codex-refuted" run bash "$SCRIPT" ag-x 12 abc1234def --repo "$REPO"
  [ "$status" -eq 1 ]
  echo "$output" | grep -q "REFUTED"
  # Must NOT emit a multi-model CONFIRMED command on a refute.
  ! echo "$output" | grep -q -- "--refuter gpt:CONFIRMED"
}

@test "UNKNOWN: no parseable verdict is treated as not-confirmed (exit 1)" {
  CODEX_BIN="$FIX/codex-garbage" run bash "$SCRIPT" ag-x 12 abc1234def --repo "$REPO"
  [ "$status" -eq 1 ]
  ! echo "$output" | grep -q -- "--refuter gpt:CONFIRMED"
}

@test "second family unavailable: fails loud (exit 2), does not degrade to single" {
  CODEX_BIN="/nonexistent/codex-xyz" run bash "$SCRIPT" ag-x 12 abc1234def --repo "$REPO"
  [ "$status" -eq 2 ]
  echo "$output" | grep -qi "unavailable"
}

@test "--write refuses to write a half-filled multi-model verdict" {
  CODEX_BIN="$FIX/codex-confirmed" run bash "$SCRIPT" ag-x 12 abc1234def --repo "$REPO" --write
  [ "$status" -eq 1 ]
  echo "$output" | grep -qi "placeholder"
}

@test "evidence file is written non-empty (pawl-verdict requires real evidence)" {
  CODEX_BIN="$FIX/codex-confirmed" run bash "$SCRIPT" ag-ev 12 deadbeef111 --repo "$REPO"
  [ "$status" -eq 0 ]
  [ -s "$REPO/.agents/pawl-verdicts/second-poll-ag-ev-gpt.md" ]
}
