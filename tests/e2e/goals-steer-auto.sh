#!/usr/bin/env bash
# tests/e2e/goals-steer-auto.sh — F5 e2e for epic soc-58nt.
#
# Exercises `ao goals steer recommend` and `ao goals steer apply` end-to-end
# in an isolated temp repo.  Never touches this repo's GOALS.md or .agents/ —
# all work happens under a mktemp directory.
#
# Steps:
#   1. Seed a temp GOALS.md + verdict-ledger with an eligible failure streak.
#   2. `ao goals steer recommend` → assert recommendation surfaced, GOALS.md unchanged.
#   3. `ao goals steer apply` WITHOUT consent (auto_apply:false, no --yes) → assert GOALS.md unchanged.
#   4. Seed auto_apply:true policy; `ao goals steer apply --yes` → assert priority bump applied,
#      non-target content byte-preserved, cooldown record written.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AO_BIN="/tmp/ao-e2e-f5"

log()  { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

# ── build ─────────────────────────────────────────────────────────────────────
if [[ ! -x "$AO_BIN" ]] || [[ "$REPO_ROOT/cli/cmd/ao" -nt "$AO_BIN" ]]; then
  log "ao binary absent or stale — building to $AO_BIN"
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao )
fi
log "ao binary: $AO_BIN"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
log "temp root: $WORK"

# ── fixture: GOALS.md with two directives ─────────────────────────────────────
# d-reduce-flaky is the chronic-failure target; d-ship-fast is healthy (no ledger).
mkdir -p "$WORK/docs"
cat > "$WORK/GOALS.md" <<'GOALSEOF'
# Goals

F5 e2e fixture: auto re-steer.

## Directives

### 1. Ship fast

Deploy continuously.

**Directive ID:** d-ship-fast
**Steer:** increase

### 2. Reduce flaky tests

Stabilise the suite.

**Directive ID:** d-reduce-flaky
**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | `exit 0` | 5 | Smoke |
GOALSEOF
GOALS_BEFORE="$(cat "$WORK/GOALS.md")"
log "fixture GOALS.md written ($(wc -l < "$WORK/GOALS.md") lines)"

# ── fixture: verdict-ledger with an eligible failure streak ───────────────────
# Policy defaults: minimum_evidence_count=5, failure_streak_length=3.
# Seed 8 iterations: 3 passes then 5 consecutive fails (streak=5 >= 3, evidence=8 >= 5).
LEDGER_DIR="$WORK/.agents/goals"
mkdir -p "$LEDGER_DIR"
LEDGER_PATH="$LEDGER_DIR/verdict-ledger.json"

cat > "$LEDGER_PATH" <<'LEDGEREOF'
{
  "schema_version": "verdict-ledger.v1",
  "generated_at": "2026-05-17T08:00:00Z",
  "records": [
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T01:00:00Z","scenario_verdict":"pass","scenario_satisfaction":0.90,"scenario_count":3,"evaluated_count":3,"run_id":"r1"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T02:00:00Z","scenario_verdict":"pass","scenario_satisfaction":0.88,"scenario_count":3,"evaluated_count":3,"run_id":"r2"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T03:00:00Z","scenario_verdict":"pass","scenario_satisfaction":0.85,"scenario_count":3,"evaluated_count":3,"run_id":"r3"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T04:00:00Z","scenario_verdict":"fail","scenario_satisfaction":0.40,"scenario_count":3,"evaluated_count":3,"run_id":"r4"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T05:00:00Z","scenario_verdict":"fail","scenario_satisfaction":0.38,"scenario_count":3,"evaluated_count":3,"run_id":"r5"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T06:00:00Z","scenario_verdict":"fail","scenario_satisfaction":0.36,"scenario_count":3,"evaluated_count":3,"run_id":"r6"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T07:00:00Z","scenario_verdict":"fail","scenario_satisfaction":0.35,"scenario_count":3,"evaluated_count":3,"run_id":"r7"},
    {"record_type":"iteration","directive_id":"d-reduce-flaky","run_timestamp":"2026-05-17T08:00:00Z","scenario_verdict":"fail","scenario_satisfaction":0.33,"scenario_count":3,"evaluated_count":3,"run_id":"r8"}
  ]
}
LEDGEREOF
log "fixture verdict-ledger written (8 iterations, streak=5)"

# ═══════════════════════════════════════════════════════════════════════════════
# step 1: recommend — surfaced, GOALS.md byte-unchanged
# ═══════════════════════════════════════════════════════════════════════════════
log "step 1: ao goals steer recommend"
log "  argv = ao goals steer recommend -o json"
STDOUT1="$WORK/stdout-step1.txt"
STDERR1="$WORK/stderr-step1.txt"
( cd "$WORK" && "$AO_BIN" goals steer recommend -o json \
    > "$STDOUT1" 2> "$STDERR1" )
EXIT1=$?
log "  exit code: $EXIT1"
log "  stderr: $(cat "$STDERR1" 2>/dev/null || true)"
log "  stdout:"
cat "$STDOUT1"

[[ "$EXIT1" -eq 0 ]] \
  || fail "step 1: exit code $EXIT1, want 0"

# Recommendation for d-reduce-flaky must be surfaced.
REC_DIRECTIVE="$(jq -r '.recommendations[0].directive_id // empty' "$STDOUT1")"
REC_TYPE="$(jq -r '.recommendations[0].mutation_type // empty' "$STDOUT1")"
log "  recommendation: directive=$REC_DIRECTIVE  type=$REC_TYPE"

[[ "$REC_DIRECTIVE" == "d-reduce-flaky" ]] \
  || fail "step 1: recommendation directive = '$REC_DIRECTIVE', want 'd-reduce-flaky'"
[[ "$REC_TYPE" == "priority_bump" ]] \
  || fail "step 1: recommendation type = '$REC_TYPE', want 'priority_bump'"

# auto_apply must be reported as false (default policy).
AUTO_APPLY="$(jq -r '.auto_apply' "$STDOUT1")"
log "  auto_apply: $AUTO_APPLY"
[[ "$AUTO_APPLY" == "false" ]] \
  || fail "step 1: auto_apply = '$AUTO_APPLY', want 'false' (default policy)"

# GOALS.md must be byte-identical after recommend.
GOALS_AFTER_STEP1="$(cat "$WORK/GOALS.md")"
[[ "$GOALS_AFTER_STEP1" == "$GOALS_BEFORE" ]] \
  || fail "step 1: GOALS.md was modified by recommend (must be recommendation-only)"

log "step 1 PASS: recommendation surfaced for d-reduce-flaky, GOALS.md unchanged"

# ═══════════════════════════════════════════════════════════════════════════════
# step 2: apply WITHOUT consent (no --yes, auto_apply:false) → GOALS.md unchanged
# ═══════════════════════════════════════════════════════════════════════════════
log "step 2: ao goals steer apply (no --yes, no auto_apply policy)"
STDOUT2="$WORK/stdout-step2.txt"
STDERR2="$WORK/stderr-step2.txt"
( cd "$WORK" && "$AO_BIN" goals steer apply \
    > "$STDOUT2" 2> "$STDERR2" ) && EXIT2=0 || EXIT2=$?
log "  exit code: $EXIT2"
log "  stderr: $(cat "$STDERR2" 2>/dev/null || true)"
log "  stdout: $(cat "$STDOUT2" 2>/dev/null || true)"

# Without consent (no policy file → auto_apply:false) the command must fail.
[[ "$EXIT2" -ne 0 ]] \
  || fail "step 2: exit code $EXIT2, want non-zero (auto_apply not enabled)"

# GOALS.md must still be byte-identical.
GOALS_AFTER_STEP2="$(cat "$WORK/GOALS.md")"
[[ "$GOALS_AFTER_STEP2" == "$GOALS_BEFORE" ]] \
  || fail "step 2: GOALS.md was mutated despite missing consent"

log "step 2 PASS: apply blocked without consent, GOALS.md unchanged"

# ═══════════════════════════════════════════════════════════════════════════════
# step 3: apply WITH consent (auto_apply:true + --yes) → bump applied
# ═══════════════════════════════════════════════════════════════════════════════
log "step 3: seed auto_apply:true policy, run ao goals steer apply --yes"

# Write a policy with auto_apply:true.
cat > "$WORK/docs/re-steer-policy.json" <<'POLICYEOF'
{
  "minimum_evidence_count": 5,
  "failure_streak_length": 3,
  "cooldown_iterations": 5,
  "allowed_mutation_types": ["priority_bump"],
  "max_priority_bump": 3,
  "auto_apply": true,
  "allow_steer_flip": false
}
POLICYEOF
log "  policy written: $WORK/docs/re-steer-policy.json"

STDOUT3="$WORK/stdout-step3.txt"
STDERR3="$WORK/stderr-step3.txt"
log "  argv = ao goals steer apply --yes"
( cd "$WORK" && "$AO_BIN" goals steer apply --yes \
    > "$STDOUT3" 2> "$STDERR3" )
EXIT3=$?
log "  exit code: $EXIT3"
log "  stderr: $(cat "$STDERR3" 2>/dev/null || true)"
log "  stdout: $(cat "$STDOUT3" 2>/dev/null || true)"

[[ "$EXIT3" -eq 0 ]] \
  || fail "step 3: exit code $EXIT3, want 0 (auto_apply:true + --yes)"

GOALS_AFTER_STEP3="$(cat "$WORK/GOALS.md")"
log "  GOALS.md diff (before → after):"
diff <(printf '%s\n' "$GOALS_BEFORE") <(printf '%s\n' "$GOALS_AFTER_STEP3") || true

# d-reduce-flaky was #2; a priority bump must move it to #1.
if ! printf '%s\n' "$GOALS_AFTER_STEP3" | grep -q "### 1. Reduce flaky tests"; then
  fail "step 3: 'Reduce flaky tests' not at position #1 after apply"
fi

# Non-target content must be preserved byte-for-byte.
for fragment in \
  "F5 e2e fixture: auto re-steer." \
  "| gate-one |" \
  "**Directive ID:** d-ship-fast" \
  "**Directive ID:** d-reduce-flaky" \
  "**Steer:** increase"
do
  if ! printf '%s\n' "$GOALS_AFTER_STEP3" | grep -qF "$fragment"; then
    fail "step 3: non-target content '$fragment' not preserved in GOALS.md"
  fi
done

# GOALS.md must be different from before (mutation happened).
[[ "$GOALS_AFTER_STEP3" != "$GOALS_BEFORE" ]] \
  || fail "step 3: GOALS.md is byte-identical; apply did not mutate"

log "step 3a PASS: priority bump applied, non-target content preserved"

# ── assert cooldown record written ────────────────────────────────────────────
COOLDOWN_KIND="$(jq -r '
  .records[]
  | select(.record_type == "cooldown" and .directive_id == "d-reduce-flaky")
  | .cooldown_kind' "$LEDGER_PATH" 2>/dev/null | head -1)"
COOLDOWN_MUTATION="$(jq -r '
  .records[]
  | select(.record_type == "cooldown" and .directive_id == "d-reduce-flaky")
  | .mutation_type' "$LEDGER_PATH" 2>/dev/null | head -1)"
log "  cooldown record: kind=$COOLDOWN_KIND  mutation_type=$COOLDOWN_MUTATION"

[[ "$COOLDOWN_KIND" == "applied" ]] \
  || fail "step 3: cooldown_kind = '$COOLDOWN_KIND', want 'applied'"
[[ "$COOLDOWN_MUTATION" == "priority_bump" ]] \
  || fail "step 3: cooldown mutation_type = '$COOLDOWN_MUTATION', want 'priority_bump'"

log "step 3b PASS: cooldown record written with kind=applied, mutation_type=priority_bump"

log "PASS: F5 e2e goals-steer-auto (recommend → blocked apply → consented apply + cooldown)"
