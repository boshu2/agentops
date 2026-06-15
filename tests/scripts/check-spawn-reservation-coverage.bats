#!/usr/bin/env bats
# ag-xe6u5: check-spawn-reservation-coverage.sh flags atm-spawned workers that
# are registered but hold NO reservation (not born into coordination). These
# cases feed fixtures via AGENTS_JSON / RESERVATIONS_JSON so they assert the
# warn/strict/skip + recency-filter behavior deterministically without a live AM.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-spawn-reservation-coverage.sh"
  FIX="$(mktemp -d)"
  # Two live workers (1m ago); one stale (2h ago).
  cat > "$FIX/agents.json" <<'EOF'
{"agents":[
  {"name":"GoldenCliff","last_active":"1m ago"},
  {"name":"TopazLeopard","last_active":"1m ago"},
  {"name":"OldGhost","last_active":"2h ago"}
]}
EOF
  # Only GoldenCliff holds a reservation.
  cat > "$FIX/res-partial.json" <<'EOF'
{"all_active":[{"agent":"GoldenCliff","path":"cli/"}]}
EOF
  # Both live workers reserved.
  cat > "$FIX/res-full.json" <<'EOF'
{"all_active":[{"agent":"GoldenCliff","path":"cli/"},{"agent":"TopazLeopard","path":"tests/"}]}
EOF
  cat > "$FIX/res-empty.json" <<'EOF'
{"all_active":[]}
EOF
}

teardown() { rm -rf "$FIX"; }

@test "warn-only by default: flags uncoordinated live worker, still exits 0" {
  AGENTS_JSON="$FIX/agents.json" RESERVATIONS_JSON="$FIX/res-partial.json" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "TopazLeopard"
  # The stale OldGhost must NOT be flagged (recency filter).
  ! echo "$output" | grep -q "OldGhost"
}

@test "strict mode: fails when a live worker holds no reservation" {
  AGENTS_JSON="$FIX/agents.json" RESERVATIONS_JSON="$FIX/res-partial.json" run bash "$SCRIPT" --strict
  [ "$status" -eq 1 ]
  echo "$output" | grep -q "TopazLeopard"
}

@test "passes (exit 0) when every live worker holds a reservation" {
  AGENTS_JSON="$FIX/agents.json" RESERVATIONS_JSON="$FIX/res-full.json" run bash "$SCRIPT" --strict
  [ "$status" -eq 0 ]
  echo "$output" | grep -qi "PASS"
}

@test "strict: all live workers uncoordinated (empty reservations) fails" {
  AGENTS_JSON="$FIX/agents.json" RESERVATIONS_JSON="$FIX/res-empty.json" run bash "$SCRIPT" --strict
  [ "$status" -eq 1 ]
  echo "$output" | grep -q "GoldenCliff"
  echo "$output" | grep -q "TopazLeopard"
}

@test "skips cleanly (exit 0) when Agent Mail data is unavailable" {
  # A non-existent fixture makes read_agents fail (the same path as `am` being
  # absent), so the script skips rather than blocking.
  AGENTS_JSON="$FIX/does-not-exist.json" RESERVATIONS_JSON="$FIX/res-empty.json" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qi "skip"
}

@test "stale-only swarm: nothing to check, exits 0" {
  cat > "$FIX/stale.json" <<'EOF'
{"agents":[{"name":"OldGhost","last_active":"2h ago"}]}
EOF
  AGENTS_JSON="$FIX/stale.json" RESERVATIONS_JSON="$FIX/res-empty.json" run bash "$SCRIPT" --strict
  [ "$status" -eq 0 ]
  echo "$output" | grep -qi "nothing to check"
}
