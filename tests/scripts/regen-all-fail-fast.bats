#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE_ROOT/scripts"
  cp "$REPO_ROOT/scripts/regen-all.sh" "$FIXTURE_ROOT/scripts/regen-all.sh"

  cat >"$FIXTURE_ROOT/scripts/publish-generated-projections.py" <<'PY'
#!/usr/bin/env python3
import sys
print("seeded transactional publisher failure", file=sys.stderr)
raise SystemExit(23)
PY
  chmod +x "$FIXTURE_ROOT/scripts/publish-generated-projections.py"
  mkdir -p "$FIXTURE_ROOT/docs/contracts"
  cat >"$FIXTURE_ROOT/docs/contracts/generated-projection-owners.v1.json" <<'JSON'
{}
JSON
  cat >"$FIXTURE_ROOT/scripts/audit-codex-parity.sh" <<'SH'
#!/usr/bin/env bash
echo "late parity gate ran" >&2
exit 99
SH
  chmod +x "$FIXTURE_ROOT/scripts/audit-codex-parity.sh"
}

@test "regen-all stops before a later projection after the first failure" {
  run bash "$FIXTURE_ROOT/scripts/regen-all.sh" --check

  [ "$status" -ne 0 ]
  [[ "$output" == *"transactional generated projections"* ]]
  [[ "$output" == *"seeded transactional publisher failure"* ]]
  [[ "$output" != *"late parity gate ran"* ]]
  [[ "$output" != *"All generated projections are current."* ]]
}

# --- direct-execution mode witnesses -----------------------------------------
#
# The test above invokes the script as `bash <path>`, which supplies the
# interpreter explicitly and is therefore blind to the file's mode by
# construction. That is exactly why a script tracked 100644 while carrying a
# `#!/usr/bin/env bash` shebang stayed green through the regression: every
# shipped caller (the Makefile, the gate runner) also passes an interpreter.
# These three witnesses exercise DIRECT execution, where the mode is load-bearing.

@test "regen-all.sh is tracked executable" {
  run git -C "$REPO_ROOT" ls-files -s scripts/regen-all.sh
  [ "$status" -eq 0 ]
  [[ "$output" == 100755* ]]
  [ -x "$REPO_ROOT/scripts/regen-all.sh" ]
}

@test "direct execution reaches the seeded failure and is not a 126 refusal" {
  # setup's `cp` preserves the source mode, so the fixture is executable exactly
  # when the tracked script is.
  run "$FIXTURE_ROOT/scripts/regen-all.sh" --check

  [ "$status" -ne 0 ]
  # 126 is "found but not executable". Asserting merely non-zero here would let a
  # mode regression masquerade as the seeded failure being proven.
  [ "$status" -ne 126 ]
  [[ "$output" == *"seeded transactional publisher failure"* ]]
  [[ "$output" != *"late parity gate ran"* ]]
  [[ "$output" != *"All generated projections are current."* ]]
}

@test "a non-executable copy is refused with exactly 126 before any work runs" {
  # The discriminating control for the witness above: it pins what a mode
  # regression actually looks like, so the two cases cannot be confused.
  cp "$FIXTURE_ROOT/scripts/regen-all.sh" "$FIXTURE_ROOT/scripts/regen-all-noexec.sh"
  chmod 644 "$FIXTURE_ROOT/scripts/regen-all-noexec.sh"

  run "$FIXTURE_ROOT/scripts/regen-all-noexec.sh" --check

  [ "$status" -eq 126 ]
  [[ "$output" != *"seeded transactional publisher failure"* ]]
  [[ "$output" != *"All generated projections are current."* ]]
}
