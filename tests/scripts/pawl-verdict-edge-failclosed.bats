#!/usr/bin/env bats
# pawl-verdict.sh — FAIL-CLOSED verdict->ledger edge emit (F2, age-pawl-intent-zhndq.2).
#
# "No verdict = not done" is enforced on the LEDGER EDGE (ao done / verify_prepush read the
# edge, not the verdict file). The old emit was fail-OPEN: a failed `ao provenance emit-verdict`
# only WARNed, so a CONFIRMED verdict FILE could exist with no edge — a silent desync where the
# operator sees "ready to push" but `ao done` refuses the close. F2 makes `write` propagate a
# distinct EDGE-UNBOUND code (7) on a genuine emit failure so the caller can surface it, while:
#   - PAWL_EDGE_FAIL_OPEN=1 restores the warn-and-continue behavior (status 0);
#   - a SUCCESSFUL emit is byte-identical (status 0);
#   - PREPUSH parking (emit succeeded, autobind deferred) is a SUCCESS, never a failure.
# The verdict FILE always survives an EDGE-UNBOUND (it is the recovery input).
#
# Harness mirrors pawl-verdict-autobind.bats: `ao` is a PATH stub (AO_STUB_EXIT / AO_STUB_APPEND).

setup() {
  AGENTOPS_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$AGENTOPS_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d "${BATS_TMPDIR:-/tmp}/pawl-edge-fc.XXXXXX")"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; VDIR="$TMP/verdicts"; mkdir -p "$BIN" "$VDIR"
  AO_LOG="$TMP/ao.log"; : > "$AO_LOG"
  REPO="$TMP/repo"; mkdir -p "$REPO/docs/provenance" "$REPO/schemas"
  git -C "$REPO" init -q
  git -C "$REPO" config user.email t@e.com; git -C "$REPO" config user.name t
  printf '%s\n' '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"genesis","from_type":"bead","to_id":"genesis-commit","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"genesis","trust_tier":"authored","ts":"2026-06-13T23:55:06Z","prev_hash":"","payload_hash":"52b578a2","hash":"ae78526f"}' \
    > "$REPO/docs/provenance/ledger.jsonl"
  git -C "$REPO" add -A; git -C "$REPO" commit -qm "init ledger"
  SHA="1111111222222233333334444444555555566666"
  cat > "$BIN/ao" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$AO_LOG"
if [[ "\${AO_STUB_APPEND:-0}" == "1" && "\${1:-}" == "provenance" && "\${2:-}" == "emit-verdict" ]]; then
  printf '%s\n' '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fc-test@1111111","from_type":"verdict","to_id":"1111111222222233333334444444555555566666","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-fc-test disposition=CONFIRMED","trust_tier":"inferred","ts":"2026-07-01T00:00:00Z","prev_hash":"ae78526f","payload_hash":"deadbeef","hash":"beefdead"}' >> "$REPO/docs/provenance/ledger.jsonl"
fi
exit "\${AO_STUB_EXIT:-0}"
EOF
  chmod +x "$BIN/ao"; export PATH="$BIN:$PATH"
  cd "$REPO"
}
teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

run_write() {
  env -u GIT_PREFIX -u GIT_DIR -u PAWL_PREPUSH -u PAWL_AUTOBIND -u AGENTOPS_REPO_ROOT -u AO_BIN "$@" \
    bash "$SCRIPT" write age-fc-test 0 \
    --disposition CONFIRMED --head "$SHA" --author-context author-ctx \
    --refuter claude:CONFIRMED:refuter-ctx --dir "$VDIR"
}

# 1. Genuine emit failure (ao exits non-zero, no edge appended) -> EDGE-UNBOUND (7), verdict KEPT.
@test "emit failure -> write returns EDGE-UNBOUND (7); verdict file survives" {
  run run_write AO_STUB_EXIT=1
  [ "$status" -eq 7 ]
  [ -f "$VDIR/age-fc-test.json" ]           # the verdict file is the recovery input — must survive
  grep -q CONFIRMED "$VDIR/age-fc-test.json"
}

# 2. PAWL_EDGE_FAIL_OPEN=1 restores warn-and-continue (status 0) on the same failure.
@test "emit failure + PAWL_EDGE_FAIL_OPEN=1 -> warn-and-continue (status 0)" {
  run run_write AO_STUB_EXIT=1 PAWL_EDGE_FAIL_OPEN=1
  [ "$status" -eq 0 ]
  [ -f "$VDIR/age-fc-test.json" ]
}

# 3. Successful emit -> byte-identical happy path (status 0, edge bound).
@test "successful emit -> status 0 (happy path unchanged)" {
  run run_write AO_STUB_APPEND=1
  [ "$status" -eq 0 ]
  [ -f "$VDIR/age-fc-test.json" ]
  grep -q "provenance emit-verdict --file $VDIR/age-fc-test.json" "$AO_LOG"
}

# 4. PREPUSH parking is a SUCCESS (emit succeeded, autobind deferred) — NEVER an EDGE-UNBOUND.
@test "prepush parking (successful emit, deferred bind) -> status 0, not a failure" {
  run run_write AO_STUB_APPEND=1 PAWL_PREPUSH=1
  [ "$status" -eq 0 ]
  [ -f "$VDIR/age-fc-test.json" ]
  # Parked: no new commit created in prepush context (the row is deferred, not committed).
  [ "$(git -C "$REPO" rev-list --count HEAD)" -eq 1 ]
}
