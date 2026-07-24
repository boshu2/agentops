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
