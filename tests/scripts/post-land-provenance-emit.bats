#!/usr/bin/env bats
# age-genn: post-land provenance stays non-blocking for hook callers, but strict
# closeout callers get a non-zero exit when required proof cannot run.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/post-land-provenance-emit.sh"
  TMP="$(mktemp -d)"
  REPO="$TMP/repo"
  mkdir -p "$REPO"
  cd "$REPO"
  git init -q
  git config user.email test@example.com
  git config user.name Test
  echo base > README.md
  git add README.md
  git commit -q -m "init"
  mkdir -p "$REPO/.agents/pawl-verdicts"
  cat > "$REPO/ao-ok" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$REPO/ao-ok"
}

teardown() {
  rm -rf "$TMP"
}

@test "post-land provenance skip remains non-blocking by default" {
  run env AGENTOPS_PROVENANCE_EMIT_SKIP=1 bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"skip requested"* ]]
}

@test "post-land provenance skip fails in strict mode" {
  run env AGENTOPS_PROVENANCE_EMIT_SKIP=1 AGENTOPS_PROVENANCE_EMIT_STRICT=1 bash "$SCRIPT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"skip requested"* ]]
}

@test "missing ao remains non-blocking by default" {
  run env AO_BIN="$TMP/missing-ao" bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"ao binary not found"* ]]
}

@test "missing ao fails in strict mode" {
  run env AO_BIN="$TMP/missing-ao" AGENTOPS_PROVENANCE_EMIT_STRICT=1 bash "$SCRIPT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"ao binary not found"* ]]
}

@test "strict required verdict fails when bound to a different head" {
  cat > "$REPO/.agents/pawl-verdicts/age-genn.json" <<'EOF'
{"bead_id":"age-genn","disposition":"CONFIRMED","head_sha":"1111111111111111111111111111111111111111"}
EOF

  run env \
    AO_BIN="$REPO/ao-ok" \
    AGENTOPS_PROVENANCE_EMIT_STRICT=1 \
    AGENTOPS_PROVENANCE_REQUIRED_VERDICT_BEAD=age-genn \
    AGENTOPS_PROVENANCE_REQUIRED_VERDICT_HEAD=2222222222222222222222222222222222222222 \
    bash "$SCRIPT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"required pawl verdict is stale"* ]]
}
