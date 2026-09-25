#!/usr/bin/env bats
# Anti-spiral guard (2026-07-28 incident: three days of planning/validation
# artifacts, zero implementation commits). The deterministic surface is the
# evidence store: store-verdict mechanically refuses a verdict over an empty
# subject manifest, so no PASS can be persisted for a run that changed nothing.
# Scenario guards that are runtime-behavioral (an orchestrator dispatching an
# unsolicited planning lane) cannot be replayed in repo CI and are not faked here.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "store-verdict refuses an empty subject manifest" {
  local ws="$BATS_TEST_TMPDIR/ws"
  mkdir -p "$ws"
  printf 'intent bytes\n' > "$ws/intent.txt"
  printf '{}\n' > "$ws/draft.json"
  local candidate="$BATS_TEST_TMPDIR/ao"
  (cd "$REPO_ROOT/cli" && go build -o "$candidate" ./cmd/ao)
  local protected="$BATS_TEST_TMPDIR/protected"
  mkdir -p "$protected"
  "$candidate" provenance manifest --root "$ws" --include missing > "$ws/empty-manifest.json"
  run "$candidate" provenance store-verdict \
    --root "$ws" --evidence-root "$protected" \
    --draft "$ws/draft.json" \
    --intent-source "$ws/intent.txt" \
    --subject-manifest "$ws/empty-manifest.json" \
    --author-context-id author-1 \
    --validator-context-id validator-1 \
    --freshness-source runtime \
    --freshness-attester-id attester-1 \
    --scope-result PASS
  [ "$status" -ne 0 ]
  [[ "$output" == *"no entries"* ]]
}
