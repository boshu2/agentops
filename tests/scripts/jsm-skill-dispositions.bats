#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-jsm-skill-dispositions.py"
  TMP_DIR="$(mktemp -d)"
  SNAPSHOT="$TMP_DIR/frozen.json"
  AUDIT="$TMP_DIR/audit.md"
  MANIFEST="$TMP_DIR/manifest.txt"
  MOCK_JSM="$TMP_DIR/jsm"

  printf '%s\n' alpha beta >"$MANIFEST"
  cat >"$SNAPSHOT" <<'EOF'
{"schema_version":1,"source_command":"jsm list --remote --jeffreys --json","names":["alpha","beta"]}
EOF
  write_audit alpha beta
}

teardown() {
  rm -r "$TMP_DIR"
}

write_audit() {
  {
    printf '%s\n' '| # | External package | AgentOps disposition |' '|---:|---|---|'
    local i=1 name
    for name in "$@"; do
      printf "| %s | \`%s\` | keep external |\n" "$i" "$name"
      i=$((i + 1))
    done
  } >"$AUDIT"
}

run_check() {
  run env \
    JSM_DISPOSITIONS_SNAPSHOT="$SNAPSHOT" \
    JSM_DISPOSITIONS_AUDIT="$AUDIT" \
    JSM_DISPOSITIONS_MANIFEST="$MANIFEST" \
    JSM_BIN="$MOCK_JSM" \
    python3 "$CHECKER" "$@"
}

@test "checked-in frozen names each have exactly one disposition" {
  run_check --check
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 frozen JSM packages; 2 one-to-one dispositions"* ]]
}

@test "missing and duplicate disposition decisions fail closed" {
  write_audit alpha alpha
  run_check --check
  [ "$status" -ne 0 ]
  [[ "$output" == *"duplicate"* ]]
  [[ "$output" == *"missing"* ]]
}

@test "refresh atomically accepts only a complete remote name set" {
  cat >"$MOCK_JSM" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '{"skills":[{"name":"beta"},{"name":"alpha"}]}'
EOF
  chmod +x "$MOCK_JSM"

  run_check --refresh
  [ "$status" -eq 0 ]
  run jq -e '.names == ["alpha","beta"] and (.refreshed_at | type == "string")' "$SNAPSHOT"
  [ "$status" -eq 0 ]
  [ ! -e "$SNAPSHOT.candidate" ]
}

@test "remote refresh failure preserves the prior checked-in snapshot" {
  cat >"$MOCK_JSM" <<'EOF'
#!/usr/bin/env bash
exit 23
EOF
  chmod +x "$MOCK_JSM"
  before="$(shasum -a 256 "$SNAPSHOT" | awk '{print $1}')"

  run_check --refresh
  [ "$status" -ne 0 ]
  [[ "$output" == *"refresh failed"* ]]
  [[ "$output" == *"--refresh"* ]]
  after="$(shasum -a 256 "$SNAPSHOT" | awk '{print $1}')"
  [ "$after" = "$before" ]
}
