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

@test "runner is invoked from the sandbox workspace" {
  WSTUB="$TMP/workspace-stub.sh"
  printf '#!/usr/bin/env bash\n[ "$PWD" = "${CORPUS_DELTA_WORKSPACE}" ] && echo '"'"'{"pass":true}'"'"' || echo '"'"'{"pass":false}'"'"'\n' > "$WSTUB"
  chmod +x "$WSTUB"
  run env CORPUS_DELTA_RUNNER="$WSTUB" "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  jq -e '.context_off.aggregate_score == 1' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.aggregate_score == 1' "$TMP/sc.json" >/dev/null
}

@test "requires --task" {
  run env CORPUS_DELTA_RUNNER="$STUB" "$HARNESS" --seeds 1
  [ "$status" -eq 2 ]
}

# --- ag-5apc: always-loaded-root contamination self-test ---------------------------
# Proves the off arm cannot reach knowledge from the always-loaded roots (repo CLAUDE.md,
# .claude/rules/*, user-global ~/.claude/CLAUDE.md, auto-memory). A CONTAM stub passes iff
# it finds a MARKER anywhere in its sandbox (HOME + workspace); the harness seeds the
# marker into FIXTURE sources for the always-loaded roots. on -> sees marker; off -> must not.
setup_contam() {
  MK="CORPUS_DELTA_MARKER_a5pc"
  SRCREPO="$TMP/srcrepo"; mkdir -p "$SRCREPO/.claude/rules"
  echo "$MK project-claude" > "$SRCREPO/CLAUDE.md"
  echo "$MK rules" > "$SRCREPO/.claude/rules/go.md"
  USERCLAUDE="$TMP/user-CLAUDE.md"; echo "$MK user-global" > "$USERCLAUDE"
  MEMDIR="$TMP/mem"; mkdir -p "$MEMDIR"; echo "$MK memory" > "$MEMDIR/MEMORY.md"
  # CONTAM stub: greps its whole sandbox (HOME + workspace) for the marker.
  CSTUB="$TMP/contam-stub.sh"
  cat > "$CSTUB" <<CSTUB_EOF
#!/usr/bin/env bash
if grep -rqsl "$MK" "\${HOME}" "\${CORPUS_DELTA_WORKSPACE:-/nonexistent}" 2>/dev/null; then
  echo '{"pass": true, "score": 1, "total": 1}'
else
  echo '{"pass": false, "score": 0, "total": 1}'
fi
CSTUB_EOF
  chmod +x "$CSTUB"
}

@test "always-loaded marker reaches context_on but NOT context_off (delta=1)" {
  setup_contam
  run env CORPUS_DELTA_RUNNER="$CSTUB" \
    CORPUS_DELTA_REPO_ROOT="$SRCREPO" \
    CORPUS_DELTA_USER_CLAUDE="$USERCLAUDE" \
    CORPUS_DELTA_MEM_DIR="$MEMDIR" \
    "$HARNESS" --task demo --seeds 2 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  # off arm provably blind to all always-loaded roots; on arm sees them.
  jq -e '.context_off.aggregate_score == 0' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.aggregate_score == 1' "$TMP/sc.json" >/dev/null
  jq -e '.aggregate_delta == 1' "$TMP/sc.json" >/dev/null
}

@test "HOME_BASE auth survives both arms while context is stripped from off" {
  setup_contam
  BASE="$TMP/homebase"; mkdir -p "$BASE"
  mkdir -p "$BASE/rules"
  echo '{"token":"x"}' > "$BASE/.credentials.json"   # auth-like file, no marker
  echo "$MK leaked-via-base" > "$BASE/CLAUDE.md"      # context the base must NOT carry into off
  echo "$MK leaked-via-base-rules" > "$BASE/rules/base.md"
  # Stub: pass iff it sees the AUTH file (proves base copied into both arms).
  ASTUB="$TMP/auth-stub.sh"
  printf '#!/usr/bin/env bash\n[ -f "${HOME}/.claude/.credentials.json" ] && echo '"'"'{"pass":true}'"'"' || echo '"'"'{"pass":false}'"'"'\n' > "$ASTUB"
  chmod +x "$ASTUB"
  run env CORPUS_DELTA_RUNNER="$ASTUB" CORPUS_DELTA_HOME_BASE="$BASE" \
    "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/sc.json"
  [ "$status" -eq 0 ]
  # auth present in BOTH arms -> both pass -> delta 0 (auth is runtime, not context)
  jq -e '.context_off.aggregate_score == 1' "$TMP/sc.json" >/dev/null
  jq -e '.context_on.aggregate_score == 1' "$TMP/sc.json" >/dev/null
  # and base-carried context markers must NOT survive into the off arm
  run env CORPUS_DELTA_RUNNER="$CSTUB" CORPUS_DELTA_HOME_BASE="$BASE" \
    CORPUS_DELTA_REPO_ROOT="$TMP/none" CORPUS_DELTA_USER_CLAUDE="$TMP/none" CORPUS_DELTA_MEM_DIR="$TMP/none" \
    "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/sc2.json"
  jq -e '.context_off.aggregate_score == 0' "$TMP/sc2.json" >/dev/null
}

@test "a degraded seed flags the run degraded + delta_valid=false (not a false null) (ag-t8n)" {
  # Stub mimics a broken agent: emits degraded:true + pass:false (launch/timeout/rate-limit).
  DSTUB="$TMP/degr-stub.sh"
  printf '#!/usr/bin/env bash\necho '"'"'{"pass":false,"score":0,"total":1,"degraded":true,"agent_exit":1}'"'"'\n' > "$DSTUB"
  chmod +x "$DSTUB"
  run env CORPUS_DELTA_RUNNER="$DSTUB" "$HARNESS" --task demo --seeds 2 --corpus "$CORPUS" --out "$TMP/d.json"
  [ "$status" -eq 0 ]
  jq -e '.degraded == true' "$TMP/d.json" >/dev/null
  jq -e '.delta_valid == false' "$TMP/d.json" >/dev/null
  jq -e '.context_off.degraded_seeds == 2' "$TMP/d.json" >/dev/null
  jq -e '.context_on.degraded_seeds == 2' "$TMP/d.json" >/dev/null
  jq -e '.context_off.status == "degraded"' "$TMP/d.json" >/dev/null
  jq -e '.context_on.status == "degraded"' "$TMP/d.json" >/dev/null
  jq -e 'has("elapsed_seconds") and (.context_on | has("elapsed_seconds"))' "$TMP/d.json" >/dev/null
}

@test "a clean (non-degraded) run reports delta_valid=true, zero degraded seeds (ag-t8n)" {
  PSTUB="$TMP/pass-stub.sh"
  printf '#!/usr/bin/env bash\necho '"'"'{"pass":true,"score":1,"total":1}'"'"'\n' > "$PSTUB"
  chmod +x "$PSTUB"
  run env CORPUS_DELTA_RUNNER="$PSTUB" "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/c.json"
  [ "$status" -eq 0 ]
  jq -e '.degraded == false' "$TMP/c.json" >/dev/null
  jq -e '.delta_valid == true' "$TMP/c.json" >/dev/null
  jq -e '.context_off.degraded_seeds == 0' "$TMP/c.json" >/dev/null
}

@test "codex auth (~/.codex) is carried into BOTH sandbox arms (ag-94f)" {
  # codex auth lives in ~/.codex, not ~/.claude — without this, codex 401s in the
  # isolated sandbox HOME and every seed degrades. Auth must reach BOTH arms.
  CODEXSRC="$TMP/codexhome"; mkdir -p "$CODEXSRC"
  echo '{"OPENAI_API_KEY":"x"}' > "$CODEXSRC/auth.json"
  echo 'model = "gpt-5.5"' > "$CODEXSRC/config.toml"
  # Stub: pass iff it sees the codex auth file inside the sandbox HOME.
  CXSTUB="$TMP/codex-auth-stub.sh"
  printf '#!/usr/bin/env bash\n[ -f "${HOME}/.codex/auth.json" ] && echo '"'"'{"pass":true}'"'"' || echo '"'"'{"pass":false}'"'"'\n' > "$CXSTUB"
  chmod +x "$CXSTUB"
  run env CORPUS_DELTA_RUNNER="$CXSTUB" CORPUS_DELTA_CODEX_HOME="$CODEXSRC" \
    "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/cx.json"
  [ "$status" -eq 0 ]
  # auth present in BOTH arms -> both pass -> delta 0 (auth is runtime, not context)
  jq -e '.context_off.aggregate_score == 1' "$TMP/cx.json" >/dev/null
  jq -e '.context_on.aggregate_score == 1' "$TMP/cx.json" >/dev/null
}

@test "missing CORPUS_DELTA_RUNNER fails fast before any seed (no built-in runner)" {
  # the former default runner (eval-agent-harness.sh over evals/workbench) was
  # removed; a run with no runner must refuse before any arm is built
  run env -u CORPUS_DELTA_RUNNER "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS"
  [ "$status" -eq 2 ]
  [[ "$output" == *"CORPUS_DELTA_RUNNER is required"* ]]
  [[ "$output" != *"[corpus-delta] arm="* ]]
}

@test "non-executable CORPUS_DELTA_RUNNER fails fast before any seed" {
  run env CORPUS_DELTA_RUNNER="$TMP/does-not-exist.sh" "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is not an executable file"* ]]
}

@test "custom CORPUS_DELTA_RUNNER owns its agent contract: any --agent is accepted" {
  run env CORPUS_DELTA_RUNNER=/bin/true "$HARNESS" --task demo --agent claude --seeds 1 --corpus "$CORPUS"
  [[ "$output" != *"CORPUS_DELTA_RUNNER is required"* ]]
}

@test "CORPUS_DELTA_EVIDENCE_KIND=live_agent refuses a stub-named runner and rejects unknown kinds (ag-t8n)" {
  # live_agent is a claim about the runner: the setup stub is named stub-runner.sh,
  # so labeling it live_agent is refused before any arm is built.
  run env CORPUS_DELTA_RUNNER="$STUB" CORPUS_DELTA_EVIDENCE_KIND=live_agent "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/ek.json"
  [ "$status" -eq 2 ]
  [[ "$output" == *"requires a real runner outside the test tree"* ]]
  [[ "$output" != *"[corpus-delta] arm="* ]]
  [ ! -f "$TMP/ek.json" ]
  run env CORPUS_DELTA_RUNNER="$STUB" CORPUS_DELTA_EVIDENCE_KIND=bogus "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS"
  [ "$status" -eq 2 ]
  [[ "$output" == *"CORPUS_DELTA_EVIDENCE_KIND must be"* ]]
}

@test "CORPUS_DELTA_EVIDENCE_KIND=live_agent labels the scorecard for a runner outside the test tree not named stub or fake" {
  mkdir -p "$TMP/runner"
  cp "$STUB" "$TMP/runner/real-runner"
  chmod +x "$TMP/runner/real-runner"
  run env CORPUS_DELTA_RUNNER="$TMP/runner/real-runner" CORPUS_DELTA_EVIDENCE_KIND=live_agent "$HARNESS" --task demo --seeds 1 --corpus "$CORPUS" --out "$TMP/ek.json"
  [ "$status" -eq 0 ]
  jq -e '.evidence_kind == "live_agent"' "$TMP/ek.json" >/dev/null
  jq -e '.aggregate_delta == 1' "$TMP/ek.json" >/dev/null
  # The label is a caller claim; the receipt binds the runner it was made about.
  local want_sha
  want_sha="$( (sha256sum "$TMP/runner/real-runner" 2>/dev/null || shasum -a 256 "$TMP/runner/real-runner") | awk '{print $1}')"
  [ "$(jq -r '.runner.path' "$TMP/ek.json")" = "$TMP/runner/real-runner" ]
  [ "$(jq -r '.runner.sha256' "$TMP/ek.json")" = "$want_sha" ]
}

@test "CORPUS_DELTA_RUNNER as a bare PATH command resolves through command -v before any cd" {
  mkdir -p "$TMP/bin"
  cp "$STUB" "$TMP/bin/corpus-delta-demo-runner"
  chmod +x "$TMP/bin/corpus-delta-demo-runner"
  run env PATH="$TMP/bin:$PATH" CORPUS_DELTA_RUNNER=corpus-delta-demo-runner "$HARNESS" --task demo --seeds 2 --corpus "$CORPUS" --out "$TMP/path.json"
  [ "$status" -eq 0 ]
  jq -e '.aggregate_delta == 1 and .context_on.passes == 2' "$TMP/path.json" >/dev/null
}

@test "CORPUS_DELTA_RUNNER as a relative path resolves against the caller's cwd, not the sandbox" {
  run bash -c 'cd "$1" && CORPUS_DELTA_RUNNER=./stub-runner.sh "$2" --task demo --seeds 2 --corpus "$3" --out "$1/rel.json"' _ "$TMP" "$HARNESS" "$CORPUS"
  [ "$status" -eq 0 ]
  jq -e '.aggregate_delta == 1 and .context_on.passes == 2' "$TMP/rel.json" >/dev/null
}

@test "a bogus CORPUS_DELTA_EVIDENCE_KIND fails before the runner is ever invoked" {
  TSTUB="$TMP/touch-runner.sh"
  cat > "$TSTUB" <<'T_EOF'
#!/usr/bin/env bash
touch "$CORPUS_DELTA_INVOKED_MARKER"
echo '{"pass": true, "score": 1, "total": 1}'
T_EOF
  chmod +x "$TSTUB"
  run env CORPUS_DELTA_RUNNER="$TSTUB" CORPUS_DELTA_INVOKED_MARKER="$TMP/invoked" CORPUS_DELTA_EVIDENCE_KIND=bogus "$HARNESS" --task demo --seeds 2 --corpus "$CORPUS"
  [ "$status" -eq 2 ]
  [[ "$output" == *"CORPUS_DELTA_EVIDENCE_KIND must be"* ]]
  [[ "$output" != *"[corpus-delta] arm="* ]]
  [ ! -e "$TMP/invoked" ]
}
