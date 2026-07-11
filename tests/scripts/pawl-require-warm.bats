#!/usr/bin/env bats
# pawl-review.sh — PAWL_REQUIRE_SERVICE / --require-warm (F1, age-pawl-intent-zhndq.1).
#
# The warm NTM panel is the intended review substrate but was never GUARANTEED: any warm
# failure (auto-up failure, or a routed verdict that fails the real gate) silently falls
# through to a cold codex-exec. `--require-warm` (env PAWL_REQUIRE_SERVICE=1) converts that
# silent cold fallback into an honest HOLD (exit 5) for WARM-ELIGIBLE modes only — strict /
# converge / staged / non-codex reviewer stay documented COLD exceptions (the knob prints a
# notice, never silently ignored). An UNTRUSTED repo requesting the knob HOLDs naming why
# (untrusted repos are always cold by design — pre-mortem 2026-07-11 security note).
#
# Same harness as pawl-review.bats: codex is a PATH stub; the standing service is a stub
# script pointed at via PAWL_SERVICE_SCRIPT; everything runs in a temp repo.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export PAWL_NO_PREFLIGHT=1
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  # The cold codex refuter. It writes the prompt to PKT_CAPTURE ONLY when it actually runs —
  # so an EMPTY/absent PKT_CAPTURE proves the cold path was never entered (the HOLD assertion).
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
prompt="$(cat)"
[[ -n "${PKT_CAPTURE:-}" ]] && printf '%s\n' "$prompt" > "$PKT_CAPTURE"
printf 'codex\n%s\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit "${CODEX_EXIT:-0}"
STUB
  chmod +x "$BIN/codex"
  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  export PKT_CAPTURE="$TMP/pkt.txt"
  export PAWL_NO_SERVICE=1
}

teardown() { cd "$ORIG_DIR"; rm -rf "$TMP"; }

# A stub standing-pawl service honoring STUB_HEALTH_RC / STUB_UP_RC / route write mode.
_service_stub() {
  cat > "$TMP/pawl-stub.sh" <<STUB
#!/usr/bin/env bash
case "\$1" in
  health) exit \${STUB_HEALTH_RC:-0} ;;
  up)     exit \${STUB_UP_RC:-0} ;;
  route)
    bead="\$2"; pr="\${4:-0}"; disp="\${STUB_ROUTE_DISP:-CONFIRMED}"
    if [ "\${STUB_VALID:-0}" = "1" ]; then
      printf 'opus pane: real review, CONFIRMED\n'  > "$TMP/ev-o.txt"
      printf 'codex pane: real review, CONFIRMED\n' > "$TMP/ev-c.txt"
      bash "$REPO_ROOT/scripts/pawl-verdict.sh" write "\$bead" "\$pr" \
        --disposition "\$disp" --head "$HEAD_SHA" \
        --author-context "pawl-route-author-\$bead" --mode multi-model \
        --refuter "claude:\$disp:opus-pane:$TMP/ev-o.txt" \
        --refuter "gpt:\$disp:codex-pane:$TMP/ev-c.txt" \
        --dir "$AGENTOPS_PAWL_VERDICT_DIR" >/dev/null 2>&1 || true
    else
      printf '{"bead_id":"%s","disposition":"%s","head_sha":"%s","mode":"multi-model"}\n' \
        "\$bead" "\$disp" "$HEAD_SHA" > "$AGENTOPS_PAWL_VERDICT_DIR/\$bead.json"
    fi
    exit \${STUB_ROUTE_RC:-0} ;;
  *) exit 0 ;;
esac
STUB
  echo "$TMP/pawl-stub.sh"
}

# 1. HOLD on auto-up failure: knob set, service down, `up` fails -> exit 5, NO cold spawn.
@test "require-warm: service down + up fails -> HOLD exit 5, no cold codex-exec" {
  stub="$(_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=1 STUB_UP_RC=1 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL_REQUIRE_SERVICE"* ]] || [[ "$output" == *"require-warm"* ]]
  # The cold refuter must never have run: its prompt-capture stays absent/empty.
  [ ! -s "$PKT_CAPTURE" ]
  # No verdict written on a HOLD.
  [ ! -f "$VFILE" ]
}

# 2. HOLD on a routed verdict that fails the real gate: knob set, health up, route writes an
#    invalid verdict -> HOLD exit 5 (NOT a cold fallback).
@test "require-warm: healthy route writes an invalid verdict -> HOLD exit 5, no cold fallback" {
  stub="$(_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=0 STUB_ROUTE_RC=0 STUB_ROUTE_DISP=CONFIRMED STUB_VALID=0 \
    CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL_REQUIRE_SERVICE"* ]] || [[ "$output" == *"require-warm"* ]]
  [ ! -s "$PKT_CAPTURE" ]
  # The HOLD must honor "NO verdict": the invalid verdict the route wrote must be removed, never
  # left at the canonical path (cross-family refute, 2026-07-11).
  [ ! -f "$VFILE" ]
}

# 2b. The knob DOES pass when a warm route lands a gate-valid verdict (the happy warm path).
@test "require-warm: healthy route with a gate-valid verdict -> exit 0 (warm satisfied)" {
  stub="$(_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=0 STUB_ROUTE_RC=0 STUB_ROUTE_DISP=CONFIRMED STUB_VALID=1 \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"VERIFIED by pawl-verdict.sh check"* ]]
  [ -f "$VFILE" ]
}

# 3. Unset-knob regression: with the knob OFF, service down -> today's behavior (cold review,
#    exit 0). Byte-identical to pre-F1.
@test "require-warm: knob unset -> cold fallthrough unchanged (exit 0, cold ran)" {
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=1 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  # The cold refuter DID run (prompt captured) — the fallback path is intact when unforced.
  [ -s "$PKT_CAPTURE" ]
  [ -f "$VFILE" ]
}

# 3b. Data safety: require-warm HOLD (service down, up fails, NO route ran) must NEVER delete a
#     PRE-EXISTING gate-valid verdict for this same bead+head — reaching the cold path proves only
#     that THIS invocation got no routed verdict, not that an existing file is invalid.
@test "require-warm: HOLD preserves a pre-existing gate-valid verdict (no data loss)" {
  # Write a genuine gate-valid verdict for HEAD_SHA via the real writer.
  printf 'opus pane: real review, CONFIRMED\n'  > "$TMP/pre-o.txt"
  printf 'codex pane: real review, CONFIRMED\n' > "$TMP/pre-c.txt"
  bash "$REPO_ROOT/scripts/pawl-verdict.sh" write age-rev-test 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context "pre-existing-author" \
    --mode multi-model \
    --refuter "claude:CONFIRMED:opus-pre:$TMP/pre-o.txt" \
    --refuter "gpt:CONFIRMED:codex-pre:$TMP/pre-c.txt" \
    --dir "$AGENTOPS_PAWL_VERDICT_DIR" >/dev/null 2>&1
  [ -f "$VFILE" ]   # precondition: the valid verdict exists
  stub="$(_service_stub)"
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=0 PAWL_SERVICE_SCRIPT="$stub" \
    PAWL_REQUIRE_SERVICE=1 STUB_HEALTH_RC=1 STUB_UP_RC=1 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 5 ]
  # The pre-existing gate-valid verdict must SURVIVE the HOLD — never deleted.
  [ -f "$VFILE" ]
  grep -q CONFIRMED "$VFILE"
}

# 4. Strict is a documented COLD exception: the knob must NOT silently HOLD it — it prints an
#    exception notice and lets strict run its own path (which today is its own UNAVAILABLE/HOLD).
@test "require-warm + --strict -> exception notice printed (warm knob does not govern strict)" {
  run env PATH="$BIN:$PATH" PAWL_NO_SERVICE=1 PAWL_REQUIRE_SERVICE=1 CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head --strict
  # The exception notice names strict as a cold exception to the warm requirement.
  [[ "$output" == *"require-warm"* || "$output" == *"PAWL_REQUIRE_SERVICE"* ]]
  [[ "$output" == *"strict"* ]]
  # It must NOT emit the warm-eligible HOLD reason (strict has its own HOLD semantics).
  [[ "$output" != *"warm-eligible review could not route"* ]]
}

# 5. Untrusted-repo precedence: the knob on an untrusted/stranger repo HOLDs naming why —
#    it must NOT be a silent no-op (an injected PAWL_NO_SERVICE could otherwise neutralize it).
@test "require-warm on an untrusted repo -> HOLD naming untrusted (never a silent cold no-op)" {
  run env PATH="$BIN:$PATH" PAWL_UNTRUSTED_REPO=1 PAWL_REQUIRE_SERVICE=1 PAWL_NO_SERVICE=1 \
    CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 5 ]
  [[ "$output" == *"untrusted"* ]]
  [ ! -s "$PKT_CAPTURE" ]
  [ ! -f "$VFILE" ]
}
