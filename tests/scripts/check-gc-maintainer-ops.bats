#!/usr/bin/env bats
# Acceptance surface for scripts/check-gc-maintainer-ops.sh (adapter.gc-maintainer):
# syntax-check scripts/gc-maintainer-ops.sh and run its behavioral suite. The red
# case is the gate's negative witness for the check-liveness ratchet.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-gc-maintainer-ops.sh"
}

@test "checker exists and is executable" {
  [ -f "$SCRIPT" ]
  [ -x "$SCRIPT" ]
}

@test "red: syntax-broken maintainer script -> check-gc-maintainer-ops.sh exits non-zero" {
  broken="$BATS_TEST_TMPDIR/gc-maintainer-ops.sh"
  printf '#!/usr/bin/env bash\nif then fi (\n' > "$broken"
  run env GC_MAINTAINER_SCRIPT="$broken" "$SCRIPT"
  [ "$status" -ne 0 ]
}

@test "green: live maintainer surface passes" {
  run "$SCRIPT"
  [ "$status" -eq 0 ] || { printf '%s\n' "$output"; false; }
}
