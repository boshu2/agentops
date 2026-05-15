# Parity-surface inventory grew from 4 to 7+ across cycles 64-70

Date: 2026-05-12
Author: evolve session cycles 44-71

## TL;DR

When a cycle adds a new gate, hook, CI lane, or contract, multiple
cataloging surfaces must be updated in lockstep. Cycle 64 (commit
`673d90f9`) was framed as the canonical example of "wire the 4 parity
surfaces in one commit." Subsequent cycles 66, 68, and 70 each
revealed that the actual inventory is **at least 7 surfaces**, not 4.
This document is the durable export of the full inventory so future
cycles do not relearn it by CI failure.

## The original "4-surface" framing (cycle 64)

Adding `scripts/check-docs-learning-references.sh` required:

1. `scripts/pre-push-gate.sh` — new lane invocation
2. `.github/workflows/validate.yml` — new `validate-...` CI job
3. The CI workflow's `summary.needs[]` list — include the new job
4. `AGENTS.md` "CI Jobs and What They Check" — new row

Cycle 64 wired all 4 in commit `673d90f9` and claimed `CI_POLICY_PARITY:
PASS (60 jobs; 7 non-blocking)` as proof. The commit was framed as
exemplar shape for "complete parity wiring in one cycle."

## The expansion (cycles 66, 68, 70)

Cycle 66 (commit `ee9e627b`) caught **two more surfaces** that cycle 64
missed:

5. **`tests/scripts/pre-push-gate.bats` `make_stub` entry** — the
   fake-repo setup stubs every pre-push helper. Adding a new helper
   without adding the stub fails bats tests 533, 534, 543, 548, 549,
   550, 551, 552 (the structural invariant in test 531 catches the
   gap). Cycle 64 missed this; cycle 66 added it.

6. **`registry.json` hook entry** — when a new PreToolUse hook is
   added to `hooks/hooks.json`, `scripts/generate-registry.sh` must be
   re-run and `registry.json` re-committed. The `registry-check` CI
   job hard-fails on staleness. Cycles 54, 55, 57 each added a
   PreToolUse hook but never regenerated registry.json (hook count
   drift: 43 → 46). Cycle 66 fixed it in one regen.

Cycle 68 (commit `3c8b33ae`) caught a **seventh surface**:

7. **CI workflow change-filter inclusion** — `.github/workflows/
   validate.yml`'s `dorny/paths-filter@v4` filters gate which jobs
   run. The `bats-tests` job was triggered on `hooks | shell | ci`
   filters; `.bats` files matched none, so bats-only commits silently
   skipped bats-tests. The skip looked like success. Cycle 68 added a
   `bats: ['**/*.bats']` filter and wired it into bats-tests.

Cycle 70 (commit `8073e12a`) caught an **eighth surface** of a
different shape — narrative drift:

8. **Doc claims about tracked vs local-only state** — `skills/evolve/
   SKILL.md` + `references/cycle-history.md` claimed
   `cycle-history.jsonl` is "the committed canonical cycle ledger"
   when in fact the nested `.agents/.gitignore` denies all paths.
   Misleading doc claims are themselves a parity surface — what the
   docs assert must match what the gitignore policy enforces.

## The compounding insight

After cycle 64 landed and was framed as "complete," three more cycles
each found one new surface. The expected total surface count is
**at least 7+ for code-shape additions, plus narrative-claim surfaces
on top**.

The cycle-45 anti-pattern ("ship surface without parity wiring") was
expected to be fixed by cycle 64's exemplar — but cycle 64 itself
exemplified the anti-pattern by missing 2 surfaces (bats stub,
registry). The recursion is the lesson: a "complete inventory" claim
should be treated with suspicion; the next CI run after it often
reveals one more surface.

## Detection mechanism that works

Drift-detection-by-CI-failure works as well as drift-detection-by-
grep. When cycle 66 fixed two more surfaces, all three (bats, registry,
canary) failed on the same CI run as different jobs — pointing at the
same root cause. The shape of the failure (`registry-check: FAIL`,
`bats-tests: not ok 531`) named exactly which surface was missing.

When cycle 68 found the path-filter gap, the signal was the
**absence** of a failure — `bats-tests: SKIPPED` was the tell. A green
CI run with key jobs skipped is the same as a missed assertion; the
gap is invisible unless drift-detection looks for it.

## Concrete recommendation for future cycles

When adding a new script/hook/CI/contract surface, run this checklist:

```
[ ] scripts/pre-push-gate.sh — new lane invocation (if a gate)
[ ] .github/workflows/validate.yml — new CI job (if a gate)
[ ] validate.yml summary.needs[] — include new job
[ ] validate.yml summary echo step
[ ] AGENTS.md "CI Jobs and What They Check" — new row (if a gate)
[ ] tests/scripts/pre-push-gate.bats — make_stub entry (if pre-push)
[ ] registry.json regenerated (if a new hook in hooks/hooks.json)
[ ] cli/embedded/hooks/ synced via `cd cli && make sync-hooks`
[ ] docs/documentation-index.md — new link (if a contract)
[ ] CI workflow path-filter — new filter or extend existing (if
    artifact under new path)
[ ] No narrative drift in skills/*/SKILL.md vs runtime truth
```

Cycle 71's drift sweep across cycles 52-70 found zero remaining gaps
after cycles 66/68/70 closed them. The checklist above is the
durable form of "what cycle 71's sweep verified."

## Tracked commits (chronological)

- `673d90f9` cycle 64 — original 4-surface framing
- `ee9e627b` cycle 66 — surfaces 5 + 6 (bats stub + registry regen)
- `3c8b33ae` cycle 68 — surface 7 (path-filter)
- `8073e12a` cycle 70 — surface 8 (narrative-vs-truth)

## 2026-05-12 update — surface 9 confirmed in the wild

After cycle 71 declared the drift sweep empty, an operator commit
chain manifested the **9th** surface live:

- `3fb9963f` added 68 lines to `hooks/session-start.sh` (codex hooks
  flag migration), but missed the corresponding update under
  `cli/embedded/hooks/session-start.sh`. CI failed on the
  `embedded-sync` required job.
- `e8e139d1` fixed it by syncing the embedded copy.

The mechanism: any change under `hooks/` or `lib/hook-helpers.sh` must
be propagated to `cli/embedded/` via `cd cli && make sync-hooks`. The
`embedded-sync` CI job catches it if missed. This was already in the
recommendation checklist as one of the 11 items, but its real-world
demonstration confirms the failure mode is recurrent — making it
worth promoting from "checklist item" to "named surface #9 with a
tracked commit-chain example."

Tracked commits for surface 9:

- `3fb9963f` — anti-pattern demonstration (hooks/*.sh added without
  cli/embedded/ sync; embedded-sync CI job blocked)
- `e8e139d1` — fix-up (synced cli/embedded/hooks/session-start.sh)
