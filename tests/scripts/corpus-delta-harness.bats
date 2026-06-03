#!/usr/bin/env bats
# ag-u6jh (ag-8p8o W1a): corpus-delta-harness.sh is the side A/B lane that runs a
# task in two context arms (empty vs organic corpus) and emits a ContextDeltaScorecard.
# These cases exercise the harness PLUMBING deterministically via an injected STUB
# runner (no LLM). They prove the harness mechanics (isolation, K-seed aggregation,
# delta math, scorecard shape) — NOT the corpus-delta product claim, which needs a
# real agent + held tasks (ag-nfux/ag-epgk).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  HARNESS="$REPO_ROOT/scripts/corpus-delta-harness.sh"
  TMP="$BATS_TEST_TMPDIR"
  # Stub runner: passes iff AO_AGENTS_DIR contains a learnings/ marker (simulates
  # "the corpus has useful content"). Contract: <task> <agent> <seed>, prints JSON.
  STUB="$TMP/stub-runner.sh"
  cat > "$STUB" <<'STUB_EOF'
#!/usr/bin/env bash
if [[ -n "${AO_AGENTS_DIR:-}" && -f "${AO_AGENTS_DIR}/learnings/marker.md" ]]; then
  echo '{"pass": true, "score": 1, "total": 1}'
else
  echo '{"pass": false, "score": 0, "total": 1}'
fi
STUB_EOF
  chmod +x "$STUB"
  # Organic-corpus fixture (content present)
  CORPUS="$TMP/corpus"
  mkdir -p "$CORPUS/learnings"
  echo "# a prior decision" > "$CORPUS/learnings/marker.md"

  FAKEBIN="$TMP/fakebin"
  PROMPT_LOG="$TMP/prompts.txt"
  mkdir -p "$FAKEBIN"
  cat > "$FAKEBIN/claude" <<'FAKE_CLAUDE_EOF'
#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "-p" ]]; then
    shift
    printf '%s\n' "${1:-}" >> "${PROMPT_LOG:?}"
  fi
  shift || true
done
FAKE_CLAUDE_EOF
  chmod +x "$FAKEBIN/claude"
}

# Assert on the --out file (clean JSON; stdout/stderr carry a human progress line).
@test "context_on (corpus present) beats context_off (empty): delta = 1.0" {
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --task demo --seeds 3 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  jq -e '.aggregate_delta == 1' "$TMP/sc.json" >/dev/null
  jq -e '.context_off.aggregate_score == 0' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.aggregate_score == 1' "$TMP/sc.json" >/dev/null
  jq -e '.context_off.status == "fail"' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.status == "pass"' "$TMP/sc.json" >/dev/null
}

@test "scorecard has the ContextDeltaScorecard-compatible shape" {
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --task demo --seeds 2 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  jq -e 'has("schema_version") and has("suite_id") and has("context_off") and has("context_on") and has("aggregate_delta")' "$TMP/sc.json" >/dev/null
  jq -e '.seeds_per_arm == 2' "$TMP/sc.json" >/dev/null
  jq -e '.evidence_kind == "harness_plumbing"' "$TMP/sc.json" >/dev/null
}

@test "no delta when both arms see the same (empty) corpus" {
  EMPTY="$TMP/empty"; mkdir -p "$EMPTY"
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --task demo --seeds 3 --corpus "$EMPTY" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  jq -e '.aggregate_delta == 0' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.aggregate_score == 0' "$TMP/sc.json" >/dev/null
}

@test "--out writes the scorecard to a file" {
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  [ -f "$TMP/sc.json" ]
  jq -e '.aggregate_delta == 1' "$TMP/sc.json" >/dev/null
}

@test "equivalent default runner path uses default args and AO_AGENTS_DIR prompt" {
  run env PATH="$FAKEBIN:$PATH" PROMPT_LOG="$PROMPT_LOG" CORPUS_DELTA_RUNNER="$REPO_ROOT/scripts/../scripts/eval-agent-harness.sh" "$HARNESS" --task ops-01 --seeds 1 --agent claude --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  [ -f "$TMP/sc.json" ]
  prompts="$(< "$PROMPT_LOG")"
  [[ "$prompts" == *"$CORPUS"* ]]
  [[ "$prompts" == *"Use only that corpus path"* ]]
}

@test "requires --task" {
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --seeds 1
  [ "$status" -eq 2 ]
}
