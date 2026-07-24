#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE_ROOT/scripts"
  cp "$REPO_ROOT/scripts/regen-all.sh" "$FIXTURE_ROOT/scripts/regen-all.sh"

  cat >"$FIXTURE_ROOT/scripts/codex-sync.sh" <<'SH'
#!/usr/bin/env bash
echo "seeded codex failure" >&2
exit 23
SH
  chmod +x "$FIXTURE_ROOT/scripts/codex-sync.sh"
}

@test "regen-all stops before a later projection after the first failure" {
  run bash "$FIXTURE_ROOT/scripts/regen-all.sh" --check

  [ "$status" -ne 0 ]
  [[ "$output" == *"Codex twins"* ]]
  [[ "$output" == *"seeded codex failure"* ]]
  [[ "$output" != *"GC skill projection"* ]]
  [[ "$output" != *"All generated projections are current."* ]]
}
