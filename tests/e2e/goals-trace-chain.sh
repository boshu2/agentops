#!/usr/bin/env bash
# tests/e2e/goals-trace-chain.sh — F4 e2e for epic soc-58nt.
#
# Exercises `ao goals trace` and `ao goals render` end-to-end in an isolated
# temp repo: seeds a GOALS.md with a directive + linked scenario + a
# scenario-results artifact + a learning, then asserts:
#
#   step 1: --from <directive> renders the full chain (tree + JSON)
#   step 2: --orphans classifies a deliberately broken link as error
#   step 3: `ao goals render` emits Gherkin from given/when/then arrays
#   step 4: --orphans --strict exits non-zero on a warning-only trace
#
# Never touches this repo's GOALS.md or .agents/ — all work is under a mktemp
# directory that is cleaned on EXIT.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AO_BIN="/tmp/ao-e2e-f4"

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

# ── fixture: GOALS.md with one fully-wired directive ─────────────────────────
cat > "$WORK/GOALS.md" <<'GOALSEOF'
# Goals

F4 e2e fixture: trace chain.

## Directives

### 1. Fitness gate blocks on scenario satisfaction

**Directive ID:** d-fitness-gate-bdd
**Steer:** Make `ao goals measure` fail when linked scenarios are unsatisfied.
**Scenarios:** s-2026-05-17-e01

### 2. Lonely directive with no scenarios

**Directive ID:** d-lonely-directive
**Steer:** nothing linked here
GOALSEOF
log "fixture GOALS.md written"

# ── fixture: promoted scenario with given/when/then ──────────────────────────
SPEC_DIR="$WORK/spec/scenarios"
mkdir -p "$SPEC_DIR"
cat > "$SPEC_DIR/s-2026-05-17-e01.json" <<'SCENEOF'
{
  "id": "s-2026-05-17-e01",
  "directive_id": "d-fitness-gate-bdd",
  "version": 1,
  "date": "2026-05-17",
  "goal": "Fitness gate refuses to pass with an unjudged active scenario",
  "narrative": "An operator runs ao goals measure with a directive whose scenario has no verdict.",
  "expected_outcome": "The gate exits non-zero and names the unjudged scenario.",
  "satisfaction_threshold": 0.8,
  "source": "human",
  "status": "active",
  "given": ["a GOALS.md directive links to an active scenario", "no scenario-results artifact exists"],
  "when": ["the operator runs ao goals measure"],
  "then": ["the fitness gate reports unsatisfied scenario", "the exit code is non-zero"]
}
SCENEOF
log "fixture scenario s-2026-05-17-e01.json written"

# ── fixture: scenario-results artifact ───────────────────────────────────────
ARTIFACT_DIR="$WORK/.agents/rpi"
mkdir -p "$ARTIFACT_DIR"
cat > "$ARTIFACT_DIR/scenario-results.json" <<'ARTIFEOF'
{
  "schema_version": "scenario-results.v1",
  "run_id": "run-e2e-f4",
  "iteration": 1,
  "generated_at": "2026-05-17T12:00:00Z",
  "results": [
    {
      "scenario_id": "s-2026-05-17-e01",
      "directive_id": "d-fitness-gate-bdd",
      "score": 0.95,
      "threshold": 0.8,
      "verdict": "pass",
      "judged_at": "2026-05-17T11:55:00Z",
      "evidence": []
    }
  ]
}
ARTIFEOF
log "fixture scenario-results artifact written"

# ── fixture: RPI run verdict artifact (bead link) ────────────────────────────
RUN_DIR="$WORK/.agents/rpi/runs/2026-05-17-soc-58nt.2.6"
mkdir -p "$RUN_DIR"
cat > "$RUN_DIR/verdict.md" <<'VERDICTEOF'
---
bead_id: soc-58nt.2.6
scenario_id: s-2026-05-17-e01
run_id: run-e2e-f4
---

# Verdict: scenario-results producer

This RPI run artifact records that bead soc-58nt.2.6 produced the result.
VERDICTEOF
log "fixture RPI verdict artifact written"

# ── fixture: learning linked to directive + artifact ─────────────────────────
LEARNING_DIR="$WORK/docs/learnings"
mkdir -p "$LEARNING_DIR"
cat > "$LEARNING_DIR/2026-05-17-trace-chain-e2e.md" <<'LEARNEOF'
---
title: E2E trace chain learning
directive_id: d-fitness-gate-bdd
scenario_id: s-2026-05-17-e01
source: .agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md
date: 2026-05-17
---

The fitness gate consumes scenario-results artifacts. This learning is linked
to directive d-fitness-gate-bdd via explicit frontmatter.
LEARNEOF
log "fixture learning written"

# ── fixture: a second directive with a broken scenario reference ──────────────
# Append to GOALS.md so --orphans finds a broken_scenario_ref error (a missing
# scenario file), which does NOT require bd availability.
cat >> "$WORK/GOALS.md" <<'BROKENEOF'

### 3. Directive with a broken scenario link

**Directive ID:** d-broken-link
**Steer:** exercise the broken-scenario-ref error path
**Scenarios:** s-9999-99-99-does-not-exist
BROKENEOF
log "fixture broken-scenario-ref directive appended to GOALS.md"

# ─────────────────────────────────────────────────────────────────────────────
# step 1: --from renders the chain (tree output)
# ─────────────────────────────────────────────────────────────────────────────
log "step 1: ao goals trace --from d-fitness-gate-bdd (tree)"
TREE_OUT="$(cd "$WORK" && "$AO_BIN" goals trace --from d-fitness-gate-bdd 2>&1)" || true
log "  tree output:"
printf '%s\n' "$TREE_OUT"

[[ "$TREE_OUT" == *"directive d-fitness-gate-bdd"* ]] \
  || fail "step 1: tree missing 'directive d-fitness-gate-bdd'"
[[ "$TREE_OUT" == *"directive_has_scenario"* ]] \
  || fail "step 1: tree missing directive_has_scenario edge"
[[ "$TREE_OUT" == *"s-2026-05-17-e01"* ]] \
  || fail "step 1: tree missing scenario s-2026-05-17-e01"
log "step 1 PASS: tree renders directive->scenario chain"

# ── step 1b: --from with JSON output ─────────────────────────────────────────
log "step 1b: ao goals trace --from d-fitness-gate-bdd -o json"
JSON_OUT="$(cd "$WORK" && "$AO_BIN" goals trace --from d-fitness-gate-bdd -o json 2>&1)"
log "  JSON output (first 3 lines):"
printf '%s\n' "$JSON_OUT" | head -3

# Every line must parse as a standalone JSON object.
LINE_COUNT=0
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  LINE_COUNT=$((LINE_COUNT + 1))
  printf '%s' "$line" | jq empty \
    || fail "step 1b: line $LINE_COUNT is not valid JSON: $line"
done <<< "$JSON_OUT"
[[ "$LINE_COUNT" -ge 2 ]] \
  || fail "step 1b: expected at least 2 JSON lines (edges + summary), got $LINE_COUNT"

# Last line must carry the summary flag.
LAST_LINE="$(printf '%s\n' "$JSON_OUT" | tail -1)"
SUMMARY_FLAG="$(printf '%s' "$LAST_LINE" | jq -r '.summary // false')"
[[ "$SUMMARY_FLAG" == "true" ]] \
  || fail "step 1b: last JSON line is not the summary record: $LAST_LINE"

# Run twice and assert identical output (deterministic ordering).
JSON_OUT2="$(cd "$WORK" && "$AO_BIN" goals trace --from d-fitness-gate-bdd -o json 2>&1)"
[[ "$JSON_OUT" == "$JSON_OUT2" ]] \
  || fail "step 1b: JSON output is not deterministic across two runs"
log "step 1b PASS: JSON is line-delimited, deterministic, and ends with summary"

# ─────────────────────────────────────────────────────────────────────────────
# step 2: --orphans classifies broken link as error
# ─────────────────────────────────────────────────────────────────────────────
log "step 2: ao goals trace --orphans (expect non-zero exit: broken artifact ref)"
ORPHAN_EXIT=0
ORPHAN_OUT="$(cd "$WORK" && "$AO_BIN" goals trace --orphans 2>&1)" || ORPHAN_EXIT=$?
log "  orphans exit code: $ORPHAN_EXIT"
log "  orphans output:"
printf '%s\n' "$ORPHAN_OUT"

[[ "$ORPHAN_EXIT" -ne 0 ]] \
  || fail "step 2: expected non-zero exit from --orphans (broken_scenario_ref error present)"
[[ "$ORPHAN_OUT" == *"broken_scenario_ref"* ]] \
  || fail "step 2: orphans output missing broken_scenario_ref finding"
[[ "$ORPHAN_OUT" == *"ERROR"* ]] \
  || fail "step 2: orphans human output missing ERROR marker"
log "step 2 PASS: --orphans correctly classifies broken scenario ref as error"

# ── step 2b: --orphans --strict escalates warnings to non-zero ───────────────
# We need a clean fixture (no errors, only warnings) for the strict test.
# Use a fresh temp dir with only the lonely directive (no scenarios → warning).
STRICT_WORK="$(mktemp -d)"
trap 'rm -rf "$STRICT_WORK"' EXIT
cat > "$STRICT_WORK/GOALS.md" <<'STRICTEOF'
# Goals

## Directives

### 1. Lonely directive

**Directive ID:** d-only-warnings
**Steer:** nothing linked
STRICTEOF
log "step 2b: ao goals trace --orphans --strict (warning-only fixture, expect non-zero)"
STRICT_EXIT=0
STRICT_OUT="$(cd "$STRICT_WORK" && "$AO_BIN" goals trace --orphans --strict 2>&1)" || STRICT_EXIT=$?
log "  strict exit code: $STRICT_EXIT"
log "  strict output:"
printf '%s\n' "$STRICT_OUT"

[[ "$STRICT_EXIT" -ne 0 ]] \
  || fail "step 2b: --orphans --strict must exit non-zero when warnings exist"
[[ "$STRICT_OUT" == *"directive_no_scenarios"* ]] || [[ "$STRICT_OUT" == *"warning"* ]] \
  || fail "step 2b: strict output missing directive_no_scenarios or warning marker"
log "step 2b PASS: --orphans --strict exits non-zero on warning-only trace"

# ─────────────────────────────────────────────────────────────────────────────
# step 3: `ao goals render` emits Gherkin from given/when/then
# ─────────────────────────────────────────────────────────────────────────────
log "step 3: ao goals render (Gherkin output)"
RENDER_OUT="$(cd "$WORK" && "$AO_BIN" goals render 2>&1)"
log "  render output:"
printf '%s\n' "$RENDER_OUT"

[[ "$RENDER_OUT" == *"@d-fitness-gate-bdd"* ]] \
  || fail "step 3: render missing @d-fitness-gate-bdd feature tag"
[[ "$RENDER_OUT" == *"Feature: Fitness gate blocks on scenario satisfaction"* ]] \
  || fail "step 3: render missing Feature line"
[[ "$RENDER_OUT" == *"@s-2026-05-17-e01"* ]] \
  || fail "step 3: render missing @s-2026-05-17-e01 scenario tag"
[[ "$RENDER_OUT" == *"Scenario: Fitness gate refuses to pass"* ]] \
  || fail "step 3: render missing Scenario goal line"
[[ "$RENDER_OUT" == *"Given a GOALS.md directive links to an active scenario"* ]] \
  || fail "step 3: render missing first Given step"
[[ "$RENDER_OUT" == *"And no scenario-results artifact exists"* ]] \
  || fail "step 3: render missing second Given (And) step"
[[ "$RENDER_OUT" == *"When the operator runs ao goals measure"* ]] \
  || fail "step 3: render missing When step"
[[ "$RENDER_OUT" == *"Then the fitness gate reports unsatisfied scenario"* ]] \
  || fail "step 3: render missing first Then step"
[[ "$RENDER_OUT" == *"And the exit code is non-zero"* ]] \
  || fail "step 3: render missing second Then (And) step"

# Lonely directive with no scenarios must emit the no-scenarios comment.
[[ "$RENDER_OUT" == *"# No scenarios linked to this directive."* ]] \
  || fail "step 3: render missing no-scenarios comment for d-lonely-directive"

log "step 3 PASS: ao goals render emits correct Gherkin from given/when/then"

# ─────────────────────────────────────────────────────────────────────────────
log "PASS: F4 e2e goals-trace-chain (trace-from tree + JSON deterministic, orphans error/warning/strict, render Gherkin)"
