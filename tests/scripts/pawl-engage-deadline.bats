#!/usr/bin/env bats
# verification-surface-honesty S3: the engage-deadline must track measured route
# latency (a 240s static deadline under a 261s panel p50 degraded two live routes
# into give-ups on 2026-07-10), and a route that degrades on timeout or service
# loss must record an outcome DISTINGUISHABLE from a substantive refutation —
# HOLD (insufficient-reviewers / service-unavailable), never a defect-claiming
# REFUTED. What counts as CONFIRMED is untouched: pawl_decide (the quorum) is
# not under test here and not modified — route_outcome maps its decision plus
# session availability onto the recorded/bound outcome.
#
# All pure (no tmux/atm), same harness shape as pawl-adaptive.bats.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  log() { :; }
}
teardown() { rm -rf "$TMP"; }

# write_metrics <file> <latency...>: one real-shaped metrics.jsonl row per latency.
write_metrics() {
  local f="$1"; shift
  : > "$f"
  local l
  for l in "$@"; do
    printf '{"ts":"2026-07-10T18:00:00Z","bead":"fx","tier":"multi","families":"cc cod","latency_s":%d,"opus":"CONFIRMED","codex":"CONFIRMED","agy":"n/a","confirmed":2,"total":2,"disposition":"CONFIRMED","agreement":"agree"}\n' "$l" >> "$f"
  done
}

# --- resolve_engage_deadline: derived from measured p95, override wins, hard ceiling ---

@test "resolve_engage_deadline: no metrics file -> static default unchanged" {
  PAWL_ENGAGE_DEADLINE_EXPLICIT="" PAWL_ENGAGE_DEADLINE=240 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/absent.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "240" ]
}

@test "resolve_engage_deadline: measured p95 above the default -> deadline rises to p95" {
  # The live 2026-07-10 shape: p95 262 vs static 240 — the deadline must be >= p95.
  write_metrics "$TMP/m.jsonl" 210 216 220 261 262
  PAWL_ENGAGE_DEADLINE_EXPLICIT="" PAWL_ENGAGE_DEADLINE=240 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/m.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" -ge 262 ]
  [ "$output" -le 320 ]
}

@test "resolve_engage_deadline: p95 below the default -> default stands (never lowers)" {
  write_metrics "$TMP/m.jsonl" 90 95 100 110 120
  PAWL_ENGAGE_DEADLINE_EXPLICIT="" PAWL_ENGAGE_DEADLINE=240 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/m.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "240" ]
}

@test "resolve_engage_deadline: junk/truncated samples cannot ratchet past the ROUTE_TIMEOUT ceiling" {
  # Low-n, timeout-truncated garbage must not push the deadline unboundedly.
  write_metrics "$TMP/m.jsonl" 900 1000
  PAWL_ENGAGE_DEADLINE_EXPLICIT="" PAWL_ENGAGE_DEADLINE=240 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/m.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "320" ]
}

@test "resolve_engage_deadline: explicit operator override wins over measured p95" {
  write_metrics "$TMP/m.jsonl" 261 262 263 264 265
  PAWL_ENGAGE_DEADLINE_EXPLICIT=1 PAWL_ENGAGE_DEADLINE=200 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/m.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "200" ]
}

@test "resolve_engage_deadline: corrupt lines are skipped, remaining samples still derive" {
  write_metrics "$TMP/m.jsonl" 262
  printf 'not-json {{\n' >> "$TMP/m.jsonl"
  PAWL_ENGAGE_DEADLINE_EXPLICIT="" PAWL_ENGAGE_DEADLINE=240 ROUTE_TIMEOUT=320
  run resolve_engage_deadline "$TMP/m.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "262" ]
}

# --- route_outcome: honest labels; CONFIRMED semantics untouched ---

@test "route_outcome: CONFIRMED decision passes through untouched (all families)" {
  run route_outcome "CONFIRMED:full:2" 0
  [ "$status" -eq 0 ]
  [ "$output" = "CONFIRMED:full" ]
}

@test "route_outcome: CONFIRMED degraded-but-quorate stays CONFIRMED even if session later lost" {
  # Quorum met before the loss — what counts as CONFIRMED is unchanged.
  run route_outcome "CONFIRMED:degraded:2" 1
  [ "$status" -eq 0 ]
  [ "$output" = "CONFIRMED:degraded" ]
}

@test "route_outcome: a real refutation stays a defect-claiming REFUTED" {
  run route_outcome "REFUTED:refuted:1" 0
  [ "$status" -eq 0 ]
  [ "$output" = "REFUTED:refuted" ]
}

@test "route_outcome: a real refutation stays REFUTED even when the session was lost" {
  run route_outcome "REFUTED:refuted:0" 1
  [ "$status" -eq 0 ]
  [ "$output" = "REFUTED:refuted" ]
}

@test "route_outcome: timeout give-ups with no substantive refuter -> HOLD:insufficient-reviewers" {
  run route_outcome "REFUTED:insufficient:1" 0
  [ "$status" -eq 0 ]
  [ "$output" = "HOLD:insufficient-reviewers" ]
}

@test "route_outcome: standing session lost mid-wait with no refuter -> HOLD:service-unavailable" {
  run route_outcome "REFUTED:insufficient:0" 1
  [ "$status" -eq 0 ]
  [ "$output" = "HOLD:service-unavailable" ]
}
