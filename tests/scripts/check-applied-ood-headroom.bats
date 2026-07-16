#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-applied-ood-headroom.sh"
  TMP="$(mktemp -d)"
}

teardown() {
  rm -rf "$TMP"
}

@test "applied-OOD headroom hook is an explicit retired tombstone" {
  run "$SCRIPT"

  [ "$status" -eq 2 ]
  [[ "$output" == *"RETIRED: applied-OOD headroom depended on the removed ao eval surface"* ]]
}

@test "applied-OOD tombstone never invokes a supplied evaluator" {
  sentinel="$TMP/evaluator-ran"
  mkdir -p "$TMP/bin"
  cat > "$TMP/bin/ao" <<SH
#!/usr/bin/env bash
touch "$sentinel"
SH
  chmod +x "$TMP/bin/ao"

  run env AO_BIN="$TMP/bin/ao" "$SCRIPT"

  [ "$status" -eq 2 ]
  [ ! -e "$sentinel" ]
}

@test "validate workflow still mentions applied-OOD headroom (path filter / coverage)" {
  run grep -F "scripts/check-applied-ood-headroom.sh" "$REPO_ROOT/.github/workflows/validate.yml"
  [ "$status" -eq 0 ]

  [ ! -e "$REPO_ROOT/scripts/hooks/pre-push.local" ]
}
