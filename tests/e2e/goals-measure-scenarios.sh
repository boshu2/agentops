#!/usr/bin/env bash
# tests/e2e/goals-measure-scenarios.sh — F2 e2e for epic soc-58nt.
#
# Exercises `ao goals measure --scenarios-only` end-to-end in an isolated temp
# repo: seed a GOALS.md with two directives linked to scenarios, stage a
# scenario-results artifact with one PASSING and one FAILING result, assert the
# JSON output, then delete the artifact and assert the unknown/skip result.
#
# Never touches this repo's GOALS.md or .agents/ — all work happens under a
# mktemp directory.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AO_BIN="/tmp/ao-e2e-f2"

log()  { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

# Build the binary if absent or stale relative to the source.
if [[ ! -x "$AO_BIN" ]] || [[ "$REPO_ROOT/cli/cmd/ao" -nt "$AO_BIN" ]]; then
  log "ao binary absent or stale — building to $AO_BIN"
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao )
fi
log "ao binary: $AO_BIN"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
log "temp root: $WORK"

# ── fixture: GOALS.md with two directives linked to distinct scenarios ──────
cat > "$WORK/GOALS.md" <<'GOALSEOF'
# Goals

F2 e2e fixture: scenario-satisfaction gate.

## Directives

### 1. Ship the gate

**Directive ID:** d-ship-gate
**Steer:** maintain
**Scenarios:** s-2026-05-17-101, s-2026-05-17-102

### 2. Harden the loader

**Directive ID:** d-harden-loader
**Steer:** maintain
**Scenarios:** s-2026-05-17-103
**Scenario threshold:** 0.75
GOALSEOF
log "fixture GOALS.md written ($(wc -l < "$WORK/GOALS.md") lines)"

# ── fixture: scenario-results artifact with one PASSING and one FAILING ──────
# d-ship-gate:    s-101 pass (0.95 ≥ 0.8), s-102 pass (0.88 ≥ 0.8) → satisfaction 1.0 → PASS
# d-harden-loader: s-103 fail (0.50 < 0.8, own threshold) → satisfaction 0.0 → FAIL
ARTIFACT_DIR="$WORK/.agents/rpi"
mkdir -p "$ARTIFACT_DIR"
ARTIFACT_PATH="$ARTIFACT_DIR/scenario-results.json"
cat > "$ARTIFACT_PATH" <<'ARTIFACTEOF'
{
  "schema_version": "scenario-results.v1",
  "run_id": "run-e2e-f2",
  "iteration": 1,
  "generated_at": "2026-05-17T12:00:00Z",
  "results": [
    {
      "scenario_id": "s-2026-05-17-101",
      "directive_id": "d-ship-gate",
      "score": 0.95,
      "threshold": 0.8,
      "verdict": "pass",
      "judged_at": "2026-05-17T11:55:00Z",
      "evidence": [".agents/rpi/phase-3-result.json"]
    },
    {
      "scenario_id": "s-2026-05-17-102",
      "directive_id": "d-ship-gate",
      "score": 0.88,
      "threshold": 0.8,
      "verdict": "pass",
      "judged_at": "2026-05-17T11:56:00Z",
      "evidence": [".agents/rpi/phase-3-result.json"]
    },
    {
      "scenario_id": "s-2026-05-17-103",
      "directive_id": "d-harden-loader",
      "score": 0.50,
      "threshold": 0.8,
      "verdict": "fail",
      "judged_at": "2026-05-17T11:57:00Z",
      "evidence": [".agents/rpi/phase-4-result.json"]
    }
  ]
}
ARTIFACTEOF
log "seeded artifact: $ARTIFACT_PATH"
log "  s-2026-05-17-101 → pass (0.95 ≥ 0.8)"
log "  s-2026-05-17-102 → pass (0.88 ≥ 0.8)"
log "  s-2026-05-17-103 → fail (0.50 < 0.8)"

# ── step 1: run --scenarios-only with artifact present ────────────────────────
log "step 1: argv = ao goals measure --scenarios-only -o json"
log "  temp root: $WORK"
RAW_JSON="$(cd "$WORK" && "$AO_BIN" goals measure --scenarios-only -o json \
  2>>"$WORK/stderr-step1.log")"
STEP1_EXIT=$?
STDERR1="$(cat "$WORK/stderr-step1.log" 2>/dev/null || true)"
log "  exit code: $STEP1_EXIT"
log "  stderr: ${STDERR1:-(empty)}"
log "  stdout (raw):"
printf '%s\n' "$RAW_JSON"

# stdout must be clean JSON.
MODE="$(printf '%s' "$RAW_JSON" | jq -r '.mode')"
[[ "$MODE" == "scenarios-only" ]] \
  || fail "step 1: mode = '$MODE', want 'scenarios-only'"

# Snapshot must be absent in --scenarios-only mode.
SNAPSHOT="$(printf '%s' "$RAW_JSON" | jq -r '.snapshot')"
[[ "$SNAPSHOT" == "null" ]] \
  || fail "step 1: snapshot should be null under --scenarios-only, got: $SNAPSHOT"

# Parse per-directive results into a lookup by directive_id.
GATE_VERDICT="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-ship-gate") | .scenario_verdict')"
GATE_SAT="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-ship-gate") | .scenario_satisfaction')"
GATE_SATISFIED="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-ship-gate") | .evaluated_count')"
GATE_THRESHOLD="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-ship-gate") | .scenario_threshold')"

LOADER_VERDICT="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-harden-loader") | .scenario_verdict')"
LOADER_SAT="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-harden-loader") | .scenario_satisfaction')"
LOADER_THRESHOLD="$(printf '%s' "$RAW_JSON" \
  | jq -r '.directives[] | select(.directive_id=="d-harden-loader") | .scenario_threshold')"

log "  parsed d-ship-gate:     verdict=$GATE_VERDICT  satisfaction=$GATE_SAT  evaluated=$GATE_SATISFIED  threshold=$GATE_THRESHOLD"
log "  parsed d-harden-loader: verdict=$LOADER_VERDICT  satisfaction=$LOADER_SAT  threshold=$LOADER_THRESHOLD"

# d-ship-gate: both scenarios pass → satisfaction 1.0 → verdict pass.
[[ "$GATE_VERDICT" == "pass" ]] \
  || fail "step 1: d-ship-gate verdict = '$GATE_VERDICT', want 'pass'"
[[ "$GATE_SAT" == "1" ]] \
  || fail "step 1: d-ship-gate scenario_satisfaction = '$GATE_SAT', want '1'"
[[ "$GATE_THRESHOLD" == "0.8" ]] \
  || fail "step 1: d-ship-gate scenario_threshold = '$GATE_THRESHOLD', want '0.8' (default)"

# d-harden-loader: s-103 fails own threshold → satisfaction 0.0 → verdict fail.
[[ "$LOADER_VERDICT" == "fail" ]] \
  || fail "step 1: d-harden-loader verdict = '$LOADER_VERDICT', want 'fail' (named failing directive)"
[[ "$LOADER_SAT" == "0" ]] \
  || fail "step 1: d-harden-loader scenario_satisfaction = '$LOADER_SAT', want '0'"
[[ "$LOADER_THRESHOLD" == "0.75" ]] \
  || fail "step 1: d-harden-loader scenario_threshold = '$LOADER_THRESHOLD', want '0.75' (declared)"

# JSON stdout must start with '{' — no human-warning preamble.
TRIMMED_JSON="$(printf '%s' "$RAW_JSON" | sed 's/^[[:space:]]*//')"
[[ "${TRIMMED_JSON:0:1}" == "{" ]] \
  || fail "step 1: stdout does not start with '{'; possible human-warning preamble leaked into JSON output"

log "step 1 PASS: d-ship-gate=pass, d-harden-loader=fail, JSON stdout is clean"

# ── step 2: delete the artifact → unknown/skip result ────────────────────────
rm -f "$ARTIFACT_PATH"
log "step 2: deleted $ARTIFACT_PATH"
log "  argv = ao goals measure --scenarios-only -o json (no artifact)"

RAW_JSON2="$(cd "$WORK" && "$AO_BIN" goals measure --scenarios-only -o json \
  2>>"$WORK/stderr-step2.log")"
STEP2_EXIT=$?
STDERR2="$(cat "$WORK/stderr-step2.log" 2>/dev/null || true)"
log "  exit code: $STEP2_EXIT"
log "  stderr: ${STDERR2:-(empty)}"
log "  stdout (raw):"
printf '%s\n' "$RAW_JSON2"

# A missing artifact must NOT cause a non-zero exit (it is a clean skip, not
# a structural error).
[[ "$STEP2_EXIT" -eq 0 ]] \
  || fail "step 2: exit code $STEP2_EXIT, want 0 (missing artifact is skip, not an error)"

# Every directive must yield verdict "unknown".
GATE_V2="$(printf '%s' "$RAW_JSON2" \
  | jq -r '.directives[] | select(.directive_id=="d-ship-gate") | .scenario_verdict')"
LOADER_V2="$(printf '%s' "$RAW_JSON2" \
  | jq -r '.directives[] | select(.directive_id=="d-harden-loader") | .scenario_verdict')"
log "  parsed d-ship-gate verdict: $GATE_V2"
log "  parsed d-harden-loader verdict: $LOADER_V2"

[[ "$GATE_V2" == "unknown" ]] \
  || fail "step 2: d-ship-gate verdict = '$GATE_V2', want 'unknown' (no artifact)"
[[ "$LOADER_V2" == "unknown" ]] \
  || fail "step 2: d-harden-loader verdict = '$LOADER_V2', want 'unknown' (no artifact)"

log "step 2 PASS: all directives yield unknown with no artifact"

log "PASS: F2 e2e goals-measure-scenarios (artifact present → pass/fail → delete → unknown)"
