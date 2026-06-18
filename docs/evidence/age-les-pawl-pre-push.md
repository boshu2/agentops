# age-les pre-push pawl evidence — multi-model enforces the fresh-context floor

**Bead:** age-les — Decide + close the multi-model self-approval bypass in the pawl merge gate.
**Mode:** fresh-context (default). Author context: `age-les-claude-main`. Refuter context: a fresh-context subagent (distinct invocation, no shared accumulated context).

## Decision

**multi-model is STRICTLY STRONGER than fresh-context, not a swap.** The fresh-context floor (≥1 refuter whose `context_id` != `author_context_id`) is the foundational independence property — it catches the dominant failure (a worker rubber-stamping its own work because it shares the author's context). multi-model ADDS ≥2-family diversity ON TOP of that floor; it must never waive it.

Grounding: this aligns the code with the contract's OWN intent. `pawls.md` already said multi-model catches "the above **plus** a single model's blind spots" — "the above" being the fresh-context catch. The "only the diversity requirement changes with mode" line was the mismatch (and the source of the bypass), now corrected.

## The bypass (before)

`scripts/pawl-verdict.sh check` in `mode=multi-model` enforced ONLY ≥2 distinct families. A verdict with two distinct families whose refuters BOTH ran in the author's context (`context_id == author_context_id`) authorized the merge — family-diverse but zero context-independence (the author reviewing its own work with two models loaded). Within the gate's own threat model (a sloppy agent opting up to multi-model and self-approving).

## The fix

- `pawl-verdict.sh`: the fresh-context floor block is lifted **verbatim** out of the `fresh-context` case arm and enforced **unconditionally** after the `esac` — so it runs in every mode. The multi-model arm now only adds the ≥2-family check; the unknown-mode arm still `return 1`s before the floor.
- `reconcile-pr.bats`: new case — multi-model, 2 families, BOTH in author's context → HOLD exit 5. The existing multi-model-with-distinct-contexts case still merges (exit 0).
- `pawls.md`: corrected the swap→strictly-stronger framing in all three places.

## Adversarial review (CONFIRMED)

A fresh-context reviewer red-teamed the change and **CONFIRMED**:

- **Bypass closed, proven** — probe (multi-model + 2 families + both refuters sharing author_context_id) returns exit 1 with the floor message; bats test 26 codifies it. The floor block was relocated verbatim, so the same independence check now runs in every mode.
- **No false-refusal regression** — multi-model with distinct contexts still authorizes (test 25); one in-context + one fresh refuter passes (≥1 fresh is correctly sufficient). 40 tests across the three suites pass; shellcheck clean.
- **Control flow sound** — `*)` unknown-mode `return 1`s before the floor; unknown/null/empty `author_context_id` are schema-rejected one layer earlier (both jsonschema and jq-fallback), so they never reach the floor.
- **Bare-string `!=` residual** is pre-existing (relocated, not introduced) and the exact trivial/out-of-threat-model item the bead flagged (it grants no privilege — the author would forge a near-twin of its own context against itself). Doc now matches code.

## Deterministic evidence

```
shellcheck scripts/pawl-verdict.sh                          # clean
bats tests/scripts/reconcile-pr.bats                        # 30/30
bats tests/scripts/{check-pawl-pre-push,pawl-verdict-binding-verdict,pawl-verdict-provenance-roundtrip,pawl-verdict-jq-schema-fallback}.bats  # 13/13
ao gate check --fast --scope head                           # 29/29 pass, 0 fail
```

Live probe: a multi-model verdict with both refuters in the author's context is now REFUSED (`PAWL-GATE: mode=multi-model needs >=1 refuter whose context_id != author_context_id …`).
