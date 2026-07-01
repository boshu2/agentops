#!/usr/bin/env bats
# gen-membrane-receipts.bats — the membrane-receipts generator derives honest,
# deterministic numbers from a real-shape ledger fixture and REFUSES to render
# when the hash chain is tampered.
#
# Fixture: tests/fixtures/provenance/membrane-receipts-ledger-slice.jsonl is a
# verbatim 150-line PREFIX of docs/provenance/ledger.jsonl (real persisted
# shape; a prefix preserves the hash chain from genesis, so `ao provenance
# verify` passes on it unmodified). Frozen expected values for that slice:
#   150 records = 127 verdict edges + 23 bead->commit edges
#   dispositions: 123 CONFIRMED, 3 REFUTED, 1 ESCALATE
#   REFUTED-then-fixed arcs: 2 (age-standing-pawl-service-ml8.7 refuted on
#     7c3ff9a and 114b35c, both later CONFIRMED on 0bb3dc3)
#   REFUTED without recorded fix: 1 (ag-721 on cafef00)

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  # Resolve an ao binary once for the whole file: PATH first, else build.
  if [ -z "${AO_BIN:-}" ]; then
    if command -v ao >/dev/null 2>&1 && ao provenance verify --help >/dev/null 2>&1; then
      AO_BIN="$(command -v ao)"
      echo "# using ao from PATH: $AO_BIN" >&3
    else
      AO_BIN="$BATS_FILE_TMPDIR/ao"
      echo "# ao not usable from PATH; building $AO_BIN" >&3
      (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao)
    fi
  fi
  export AO_BIN
}

setup() {
  SCRIPT="$REPO_ROOT/scripts/gen-membrane-receipts.sh"
  FRESHNESS="$REPO_ROOT/scripts/check-membrane-receipts-freshness.sh"
  FIXTURE="$REPO_ROOT/tests/fixtures/provenance/membrane-receipts-ledger-slice.jsonl"
  # Build a fixture "repo root": ao provenance verify resolves the ledger by
  # walking up for a dir holding docs/ + schemas/.
  ROOT="$BATS_TEST_TMPDIR/fixroot"
  mkdir -p "$ROOT/docs/provenance" "$ROOT/schemas"
  cp "$FIXTURE" "$ROOT/docs/provenance/ledger.jsonl"
  OUT_MD="$BATS_TEST_TMPDIR/out/membrane-receipts.md"
  OUT_JSON="$BATS_TEST_TMPDIR/out/membrane-receipts.json"
}

run_generator() {
  run env PROVENANCE_LEDGER="$ROOT/docs/provenance/ledger.jsonl" \
    RECEIPTS_MD="$OUT_MD" RECEIPTS_JSON="$OUT_JSON" AO_BIN="$AO_BIN" \
    "$SCRIPT"
  echo "# generator exit=$status" >&3
  printf '%s\n' "$output" | sed 's/^/# /' >&3
}

jqv() { jq -r "$1" "$OUT_JSON"; }

@test "generator derives correct counts from the real-shape fixture ledger" {
  run_generator
  [ "$status" -eq 0 ]
  [ -f "$OUT_MD" ]
  [ -f "$OUT_JSON" ]

  echo "# asserting JSON twin counts against frozen fixture expectations" >&3
  [ "$(jqv '.totals.ledger_records')" = "150" ]
  [ "$(jqv '.totals.verdict_events')" = "127" ]
  [ "$(jqv '.totals.bead_commit_edges')" = "23" ]
  [ "$(jqv '.dispositions.CONFIRMED')" = "123" ]
  [ "$(jqv '.dispositions.REFUTED')" = "3" ]
  [ "$(jqv '.dispositions.other.ESCALATE')" = "1" ]
  [ "$(jqv '.escapes.ledger_escape_records')" = "0" ]
  [ "$(jqv '.source.chain_verified')" = "true" ]
  [ "$(jqv '.time_range.first')" = "2026-06-13T23:55:06Z" ]
  [ "$(jqv '.time_range.last')" = "2026-06-23T22:21:51Z" ]

  echo "# asserting the human page carries the same derived numbers" >&3
  grep -Fq '| Ledger records | 150 |' "$OUT_MD"
  grep -Fq '| Verdict events (verdict → commit) | 127 |' "$OUT_MD"
  grep -Fq '| CONFIRMED verdicts | 123 |' "$OUT_MD"
  grep -Fq '| REFUTED verdicts | 3 |' "$OUT_MD"
  grep -Fq '| ESCALATE verdicts | 1 |' "$OUT_MD"
  grep -Fq '| Escape-tagged ledger records | 0 |' "$OUT_MD"
  grep -Eq '^Generated: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$' "$OUT_MD"
}

@test "REFUTED-arc detection: fixed arcs counted, unfixed refute surfaced honestly" {
  run_generator
  [ "$status" -eq 0 ]

  echo "# arc counts" >&3
  [ "$(jqv '.caught_defects.refuted_then_fixed')" = "2" ]
  [ "$(jqv '.caught_defects.refuted_without_recorded_fix')" = "1" ]
  [ "$(jqv '.exemplar_catches | length')" = "3" ]

  echo "# ml8.7 refutes must both link the later CONFIRMED commit 0bb3dc3" >&3
  [ "$(jqv '[.caught_defects.arcs[] | select(.bead=="age-standing-pawl-service-ml8.7") | .fixed_by.commit_short] | join(",")')" = "0bb3dc3,0bb3dc3" ]

  echo "# ag-721 has no CONFIRMED follow-up in the slice -> fixed_by null" >&3
  [ "$(jqv '.caught_defects.arcs[] | select(.bead=="ag-721") | .fixed_by')" = "null" ]

  echo "# exemplar lines link the real short SHAs from the ledger" >&3
  grep -Fq '`cafef00`' "$OUT_MD"
  grep -Fq '`7c3ff9a`' "$OUT_MD"
  grep -Fq 'no CONFIRMED follow-up recorded in this ledger' "$OUT_MD"
  grep -Fq 'fixed: CONFIRMED on `0bb3dc3`' "$OUT_MD"
}

@test "generator REFUSES to render when the ledger chain is tampered" {
  echo "# tampering: flipping a REFUTED disposition to CONFIRMED without re-hashing" >&3
  sed 's/disposition=REFUTED/disposition=CONFIRMED/' \
    "$ROOT/docs/provenance/ledger.jsonl" > "$BATS_TEST_TMPDIR/tampered.jsonl"
  mv "$BATS_TEST_TMPDIR/tampered.jsonl" "$ROOT/docs/provenance/ledger.jsonl"

  run_generator
  [ "$status" -eq 1 ]
  [[ "$output" == *"REFUSING to render"* ]]
  [[ "$output" == *"verification FAILED"* ]]

  echo "# refusal must leave NO rendered artifacts behind" >&3
  [ ! -f "$OUT_MD" ]
  [ ! -f "$OUT_JSON" ]
}

@test "deterministic output: two runs are byte-identical modulo the generated-at line" {
  run_generator
  [ "$status" -eq 0 ]
  cp "$OUT_MD" "$BATS_TEST_TMPDIR/run1.md"
  cp "$OUT_JSON" "$BATS_TEST_TMPDIR/run1.json"

  sleep 1  # force a different generated-at timestamp
  run_generator
  [ "$status" -eq 0 ]

  echo "# timestamps must differ (proves the filter below is load-bearing)" >&3
  run diff -q "$BATS_TEST_TMPDIR/run1.md" "$OUT_MD"
[ "$status" -ne 0 ]  # timestamps MUST differ or the modulo-filter below proves nothing

  echo "# md diff ignoring the single Generated line" >&3
  diff <(grep -v '^Generated: ' "$BATS_TEST_TMPDIR/run1.md") \
       <(grep -v '^Generated: ' "$OUT_MD")

  echo "# json diff ignoring the generated_at field" >&3
  diff <(jq 'del(.generated_at)' "$BATS_TEST_TMPDIR/run1.json") \
       <(jq 'del(.generated_at)' "$OUT_JSON")
}

@test "freshness check: fresh receipts PASS, stale receipts WARN (exit 0), strict fails" {
  run_generator
  [ "$status" -eq 0 ]

  run env PROVENANCE_LEDGER="$ROOT/docs/provenance/ledger.jsonl" RECEIPTS_MD="$OUT_MD" "$FRESHNESS"
  echo "# fresh: exit=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]

  echo "# aging the page: Generated 40 days before the ledger tip (tip 2026-06-23)" >&3
  sed 's/^Generated: .*/Generated: 2026-05-14T00:00:00Z/' "$OUT_MD" > "$BATS_TEST_TMPDIR/stale.md"

  run env PROVENANCE_LEDGER="$ROOT/docs/provenance/ledger.jsonl" RECEIPTS_MD="$BATS_TEST_TMPDIR/stale.md" "$FRESHNESS"
  echo "# stale: exit=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]

  run env PROVENANCE_LEDGER="$ROOT/docs/provenance/ledger.jsonl" RECEIPTS_MD="$BATS_TEST_TMPDIR/stale.md" "$FRESHNESS" --strict
  echo "# stale --strict: exit=$status output=$output" >&3
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]

  echo "# missing page warns (warn-only posture)" >&3
  run env PROVENANCE_LEDGER="$ROOT/docs/provenance/ledger.jsonl" RECEIPTS_MD="$BATS_TEST_TMPDIR/nope.md" "$FRESHNESS"
  echo "# missing: exit=$status output=$output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
}
