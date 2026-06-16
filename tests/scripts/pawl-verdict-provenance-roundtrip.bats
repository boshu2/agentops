#!/usr/bin/env bats
# Producer→sensor wiring guard for the verdict provenance feed
# (age-d16-self-hosting-route-nkr.1 / M1).
#
# Proves the ONE untested seam: `scripts/pawl-verdict.sh write` (the producer)
# emits a schema-shaped verdict artifact AND fires the verdict sensor
# (`ao provenance emit-verdict --file <artifact>`). The sensor's own
# emit→ledger round-trip is covered by Go tests
# (provenance_emit_verdict_test.go); this locks that the producer actually
# invokes it with the artifact it just wrote — the wiring that, when absent,
# leaves the ledger at 0 verdict rows.
#
# `ao` is STUBBED via PATH (logs its args) so the test is hermetic and needs no
# built binary. `jq` is a real hard dep (as in the script).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  AO_LOG="$TMP/ao.log"
  : > "$AO_LOG"
  # Stub `ao`: log every invocation so we can assert the sensor was fired.
  cat >"$TMP/bin/ao" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$AO_LOG"
exit 0
EOF
  chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$PATH"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "pawl-verdict write emits a schema-shaped artifact and fires the verdict sensor" {
  local sha="611615d9b78717eca0fa1b2d1eb75a54c9dc6970"
  run bash "$SCRIPT" write age-d16-roundtrip 0 \
    --disposition CONFIRMED --head "$sha" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:refuter-ctx \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]

  # 1. Producer wrote the artifact.
  local out="$TMP/verdicts/age-d16-roundtrip.json"
  [ -f "$out" ]

  # 2. Artifact carries the three fields the sensor extracts, in the real shape.
  [ "$(jq -r .schema_version "$out")" = "pawl-verdict.v1" ]
  [ "$(jq -r .bead_id "$out")" = "age-d16-roundtrip" ]
  [ "$(jq -r .disposition "$out")" = "CONFIRMED" ]
  [ "$(jq -r .head_sha "$out")" = "$sha" ]

  # 3. Producer fired the sensor against the artifact it just wrote (the wiring).
  grep -q "provenance emit-verdict --file $out" "$AO_LOG"
}
