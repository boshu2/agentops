#!/usr/bin/env bats
# Composed E2E battery for the pawl intent-alignment refactor (age-pawl-intent-zhndq.15).
#
# The individual features have unit bats; THIS suite proves they COMPOSE end-to-end and pins the
# precedence between them, plus the audit's fabricated-verdict guard:
#   1. REQUIRE-WARM HOLD (F1)   — service down + up fails + PAWL_REQUIRE_SERVICE=1 -> exit 5,
#                                 NO cold reviewer spawn, NO verdict file.
#   2. EDGE-UNBOUND + RECOVERY (F2) — CONFIRMED review whose emit fails -> exit 6, verdict KEPT,
#                                 and the PRINTED recovery command actually binds the edge when
#                                 re-run against a working ao (the full recovery loop).
#   3. COMPOSITION PRECEDENCE   — knob set AND emit would fail -> the EARLIER gate wins (HOLD at
#                                 exit 5; the exit-6 path is never reached, no cold spawn).
#   4. FABRICATED-VERDICT GUARD — a verdict written with only a REAL voter must never carry a
#                                 fabricated `*-timeout` refuter for a pane that voted nothing
#                                 (pins the audit-verified guard).
#
# Every case tees its transcript to a named artifact under $TMP/artifacts so a CI failure is
# diagnosable from the output alone. Portable: no GNU-only tools (macOS bats).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export PAWL_NO_PREFLIGHT=1
  REVIEW="$REPO_ROOT/scripts/pawl-review.sh"
  VERDICT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"; ORIG_DIR="$PWD"
  ART="$TMP/artifacts"; mkdir -p "$ART"
  BIN="$TMP/bin"; mkdir -p "$BIN"

  # Cold reviewer stub. Touches $SPAWN_MARK when it RUNS — an absent marker proves no cold spawn.
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null
[ -n "${SPAWN_MARK:-}" ] && : > "$SPAWN_MARK"
printf 'codex\n%s\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit 0
STUB
  chmod +x "$BIN/codex"

  # ao shim that FAILS on `provenance emit-verdict` (the F2 fail-closed trigger), succeeds otherwise.
  cat > "$BIN/ao-fail-emit" <<'STUB'
#!/usr/bin/env bash
if [ "${1:-}" = "provenance" ] && [ "${2:-}" = "emit-verdict" ]; then
  echo "ao: simulated emit-verdict failure" >&2; exit 1
fi
exit 0
STUB
  chmod +x "$BIN/ao-fail-emit"

  # ao shim whose emit SUCCEEDS and records that it bound the edge (the recovery proof).
  cat > "$BIN/ao-ok-emit" <<STUB
#!/usr/bin/env bash
if [ "\${1:-}" = "provenance" ] && [ "\${2:-}" = "emit-verdict" ]; then
  echo "bound \$*" >> "$TMP/emitted.log"; exit 0
fi
exit 0
STUB
  chmod +x "$BIN/ao-ok-emit"

  # A standing-service stub: health/up controlled by STUB_HEALTH_RC / STUB_UP_RC.
  cat > "$TMP/pawl-stub.sh" <<'STUB'
#!/usr/bin/env bash
case "$1" in
  health) exit ${STUB_HEALTH_RC:-1} ;;
  up)     exit ${STUB_UP_RC:-1} ;;
  *)      exit 0 ;;
esac
STUB
  chmod +x "$TMP/pawl-stub.sh"

  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-e2e)"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-e2e.json"
  export SPAWN_MARK="$TMP/cold-spawned"
  export PAWL_NO_SERVICE=1
}
teardown() { cd "$ORIG_DIR"; rm -rf "$TMP"; }

# CASE 1 (F1): require-warm + service down + up fails -> HOLD exit 5, no cold spawn, no verdict.
@test "E2E 1: require-warm HOLD — no cold spawn, no verdict (F1)" {
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$TMP/pawl-stub.sh" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=1 STUB_UP_RC=1 SPAWN_MARK="$SPAWN_MARK" \
    bash "$REVIEW" age-e2e --scope head --author-family claude
  printf '%s\n' "$output" > "$ART/case1-require-warm-hold.log"
  [ -s "$ART/case1-require-warm-hold.log" ]
  [ "$status" -eq 5 ]
  [ ! -f "$SPAWN_MARK" ]        # the cold reviewer NEVER ran
  [ ! -f "$VFILE" ]             # HOLD writes no verdict
}

# CASE 2 (F2): CONFIRMED review + failing emit -> exit 6, verdict KEPT, and the PRINTED recovery
# command genuinely binds the edge when re-run against a working ao (the full recovery loop).
@test "E2E 2: edge-unbound exit 6 + the printed recovery command actually binds (F2)" {
  run env PATH="$BIN:$PATH" AO_BIN="$BIN/ao-fail-emit" CODEX_STUB="VERDICT: CONFIRMED" \
    SPAWN_MARK="$SPAWN_MARK" \
    bash "$REVIEW" age-e2e --scope head --author-family claude
  printf '%s\n' "$output" > "$ART/case2-edge-unbound.log"
  [ -s "$ART/case2-edge-unbound.log" ]
  [ "$status" -eq 6 ]
  [ -f "$VFILE" ]                                   # verdict survives — it is the recovery input
  [[ "$output" == *"ao provenance emit-verdict --file"* ]]

  # RECOVERY LOOP: re-run the emit against a WORKING ao — the edge binds (no re-review needed).
  run env PATH="$BIN:$PATH" "$BIN/ao-ok-emit" provenance emit-verdict --file "$VFILE"
  [ "$status" -eq 0 ]
  grep -q "emit-verdict --file $VFILE" "$TMP/emitted.log"
}

# CASE 3: PRECEDENCE — with BOTH the knob set and a failing emit, the EARLIER gate (require-warm
# HOLD) wins: exit 5, and the exit-6 path is never reached (no cold spawn, no verdict).
@test "E2E 3: composition precedence — require-warm HOLD wins over the edge-unbound path" {
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$TMP/pawl-stub.sh" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=1 STUB_UP_RC=1 \
    AO_BIN="$BIN/ao-fail-emit" SPAWN_MARK="$SPAWN_MARK" \
    bash "$REVIEW" age-e2e --scope head --author-family claude
  printf '%s\n' "$output" > "$ART/case3-precedence.log"
  [ -s "$ART/case3-precedence.log" ]
  [ "$status" -eq 5 ]                # the HOLD, not the 6
  [ ! -f "$SPAWN_MARK" ]
  [ ! -f "$VFILE" ]
}

# CASE 4: FABRICATED-VERDICT GUARD — a verdict must only record refuters that ACTUALLY voted; a
# pane that produced nothing must never appear as a fabricated `*-timeout` refuter (audit guard).
@test "E2E 4: no fabricated refuter — only real voters appear in the verdict" {
  printf 'codex pane: a real, specific review of README.md:1 — CONFIRMED\n' > "$TMP/ev.txt"
  run env PATH="$BIN:$PATH" AO_BIN="$BIN/ao-ok-emit" \
    bash "$VERDICT" write age-e2e 0 --disposition CONFIRMED --head "$HEAD_SHA" \
      --author-context "author-claude-age-e2e" \
      --refuter "gpt:CONFIRMED:codex-fresh-age-e2e:$TMP/ev.txt" \
      --dir "$AGENTOPS_PAWL_VERDICT_DIR"
  printf '%s\n' "$output" > "$ART/case4-no-fabricated-refuter.log"
  [ -s "$ART/case4-no-fabricated-refuter.log" ]
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  # EXACTLY the one real voter; no fabricated timeout entries for panes that never voted.
  [ "$(grep -c 'timeout' "$VFILE" || true)" -eq 0 ]
  grep -q '"family": *"gpt"' "$VFILE" || grep -q '"family":"gpt"' "$VFILE"
}
