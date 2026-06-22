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

@test "pawl-verdict rebind re-stamps head, preserves refuters, re-fires the sensor (age-standing-pawl-service-ml8.4)" {
  local sha1="611615d9b78717eca0fa1b2d1eb75a54c9dc6970"
  local sha2="aaaaaaaabbbbbbbbccccccccdddddddd00001111"
  run bash "$SCRIPT" write age-rebind-test 0 \
    --disposition CONFIRMED --head "$sha1" \
    --author-context author-ctx --mode multi-model \
    --refuter claude:CONFIRMED:opus-ctx --refuter gpt:CONFIRMED:codex-ctx \
    --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  local out="$TMP/verdicts/age-rebind-test.json"
  [ "$(jq -r .head_sha "$out")" = "$sha1" ]
  : > "$AO_LOG"   # clear so we can prove rebind ALSO fires the sensor

  run bash "$SCRIPT" rebind age-rebind-test 0 --head "$sha2" --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  # head moved to the new commit...
  [ "$(jq -r .head_sha "$out")" = "$sha2" ]
  # ...refuters + mode + disposition preserved (the review did not re-run)...
  [ "$(jq -r '.refuters | length' "$out")" = "2" ]
  [ "$(jq -r .mode "$out")" = "multi-model" ]
  [ "$(jq -r .disposition "$out")" = "CONFIRMED" ]
  # ...and the sensor fired again for the new head.
  grep -q "provenance emit-verdict --file $out" "$AO_LOG"
}

@test "pawl-verdict rebind refuses a non-CONFIRMED or missing verdict (fail-closed)" {
  # missing verdict
  run bash "$SCRIPT" rebind age-absent 0 --head "611615d9b78717eca0fa1b2d1eb75a54c9dc6970" --dir "$TMP/verdicts"
  [ "$status" -ne 0 ]
  # REFUTED verdict is not landable -> rebind refuses. --reason is REQUIRED on a
  # REFUTED write (EM.2.1, a candidate escape); supply it so the verdict file IS
  # written and rebind refuses it for being non-CONFIRMED (the intent), not merely
  # for being absent.
  bash "$SCRIPT" write age-refuted 0 --disposition REFUTED --head "611615d9b78717eca0fa1b2d1eb75a54c9dc6970" \
    --author-context a --refuter claude:REFUTED:ctx --reason "no agreement" --dir "$TMP/verdicts" >/dev/null 2>&1 || true
  [ -f "$TMP/verdicts/age-refuted.json" ]   # the REFUTED verdict was actually written
  run bash "$SCRIPT" rebind age-refuted 0 --head "aaaaaaaabbbbbbbbccccccccdddddddd00001111" --dir "$TMP/verdicts"
  [ "$status" -ne 0 ]
}
