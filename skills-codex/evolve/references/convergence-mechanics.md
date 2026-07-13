# Convergence Mechanics — How the Loop Compounds Instead of Drifts

The /evolve loop only compounds when each cycle reads prior cycles' outcomes and lets them change behavior. Append-only ledgers that no step reads are write-only artifacts — they accumulate without compounding.

This reference documents the four feedback mechanisms that turn raw cycle output into next-cycle behavior change.

## Mechanism 1: Step 0 reads prior-failure surface

In Step 0 (Setup), after `mkdir -p .agents/evolve`, the loop reads the last 3 entries of `cycle-history.jsonl`. For any entry where `gate` field contains a FAIL marker, it extracts the failure surface (e.g. "registry-check stale", "bats-tests goals-validate") and injects the matching learning before work selection.

```bash
last3=$(scripts/evolve-read-cycle-history.sh recent 3)  # routes through BC3 LoopReaderPort (soc-y5vh.4)
fail_surfaces=$(echo "$last3" | jq -r 'select(.gate | test("FAIL|FAILED|BLOCKED")) | .gate' 2>/dev/null)
if [ -n "$fail_surfaces" ]; then
    # Search learnings for surface keywords; print whichever match
    keywords=$(echo "$fail_surfaces" | grep -oE 'registry|bats|markdown|supergate|canary|coverage|toolchain' | sort -u)
    for kw in $keywords; do
        ao lookup --query "$kw failure" --limit 2 2>/dev/null || \
            find .agents/learnings -name "*$kw*.md" -mtime -30 | head -2
    done
fi
```

Without this, the 2026-05-07 CI-toil learning sat for 7 days while I burned 5 cycles re-hitting the same `registry.json` non-determinism. Reading the learning at Step 0 would have surfaced the `git ls-files` fix on cycle 45.

## Mechanism 2: Step 1.5 healing-first classifier

Before Step 2 (measure fitness) and Step 3 (select work), the loop classifies the cycle:

```bash
# Step 1.5: healing-first classifier — routes through BC2 CIStatusPort
# (cli/cmd/ao/ci_status_adapter.go, cycle 117 productionCIStatus) per
# soc-y5vh.2. No inline gh shell-outs.
last_ci=$(ao ci recent --limit 1 2>/dev/null | jq -r '.Conclusion // empty')
if [ "$last_ci" = "failure" ]; then
    CYCLE_MODE="restorative"
    # Read failure surface, search for matching learning
    # (see Mechanism 1)
    # Selection ladder downgrade: skip Step 3.1 (harvested) for feature
    # work; only allow harvested items typed bug/fix/ci-failure.
else
    CYCLE_MODE="feature"
fi
```

Restorative cycles ONLY take work that reduces CI red. New PG4 promotions, feature additions, doc growth — all blocked until `last_ci=success`.

This eliminates the cycle-46-47 pattern where I added new evidence files onto a CI-red base.

## Mechanism 3: Hypothesis tracking for skill changes

When a cycle edits `skills/evolve/SKILL.md`, it MUST append to the hypothesis
ledger through the typed BC3 `HypothesisLedgerPort` (soc-y5vh.8):

```bash
ao loop hypothesis append --id "H<cycle>.<patch>" --cycle-landed N --check-at-cycle $((N+15)) \
  --patch "<one-line>" --hypothesis "<expected effect>" --measure "<how to verify>"
```

This routes through `productionHypothesisLedger` instead of a raw append to
`.agents/evolve/hypotheses.jsonl`; the port rejects empty and duplicate IDs.
At `check_at_cycle`, the loop reads the ledger with `ao loop hypothesis list`
(one JSON record per line), evaluates each PENDING row's `measure`, and
writes the verdict (VERIFIED / FALSIFIED). Falsified hypotheses are
revisited: either the patch is wrong, or the measurement was wrong.

Without this, cycle 45's 6 patches landed unmeasured and 2 of them (H45.2 source-surface auto-rebuild, H45.3 grep-based gate parsing) were silently inert for the next 5 cycles — text in SKILL.md but no harness automation behind them.

## Mechanism 4: Convergence criteria with a STOP

`.agents/evolve/session-convergence.json` records the terminal state; the STOP
decision is evaluated through the typed BC3 `ConvergenceCheckPort` (soc-y5vh.8):

```bash
ao loop converged --green-streak "$STREAK" --unconsumed-high-medium "$HM" --fitness-baseline
# emits {converged, ci_green_streak, unconsumed_high_medium, fitness_baseline_captured, reasons}
```

The predicate is pure — the loop supplies the evidence it already has
(`ao ci recent` for the streak, the next-work findings count, the
fitness-baseline flag). The criteria are met when all hold:

- CI Validate green for the last 3 pushes (green streak ≥ 3)
- HIGH+MEDIUM unconsumed next-work entries ≤ 1
- a fitness baseline has been captured

When `ao loop converged` reports `converged: true`, the loop emits a teardown
report and does NOT call `ScheduleWakeup`. The wakeup chain terminates. The
autonomous loop is bounded by criteria, not by wakeup count. `reasons` names
every unmet criterion when `converged` is false.

Without an explicit STOP, the loop drifts indefinitely. With STOP, it converges.

## Anti-drift rules

1. **Restorative-only after red.** Any cycle whose `gate` field has FAIL → cycle N+1 is restorative.
2. **3 consecutive restorative without restoration → escalate.** Don't silently grind.
3. **Scope shift resets the streak.** If the operator broadens the convergence target mid-session, reset `ci-green-streak` counter to 0.

## Why this is the load-bearing change

Cycles 44-50 wrote ~30 KB of bookkeeping (cycle-history, learning, retro, evidence, hypotheses) but produced ~0 compounded behavior — every cycle had to re-derive the lesson cycle 44 should have surfaced. The compounding lives in the read path, not the write path. These four mechanisms make the read path real.
