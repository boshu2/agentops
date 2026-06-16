# Evidence — Directive 16 epic done-test: the unattended self-hosting loop closes

**Epic:** `age-d16-self-hosting-route-nkr` — "AgentOps drives its OWN next epic to verified-done, end-to-end, unattended."
**Arc:** the terminal integration that composes the five landed organs (M1–M5) into one closed loop, satisfying the mechanical done-test in `.agents/plans/2026-06-16-directive-16-self-hosting-route.md` §"Epic done-test" (criteria 1–6).

## What this proves

The self-hosting loop **closes end-to-end** — a completed run's failure recovers, reaches "accepted" only through a fresh-context pawl verdict (no self-approval), and its evidence is mined back into the next seeded bead — composing the **real** organs, not mocks:

| organ | role in the loop | landed |
|---|---|---|
| M1 verdict sensor | `ao provenance emit-verdict` lands a verdict row in the ledger | `8e2b403f3` |
| M2 recovery state-machine | `scripts/recovery-statemachine.sh` branches fix-forward \| re-scope \| andon | `8a35dbabd` |
| M3 binding verdict | `scripts/pawl-verdict.sh` authorizes only a fresh-context CONFIRMED verdict; refuses self-approval | `629f0069e` |
| M4 ASSAY tick | `scripts/assay/self-improvement-tick.sh` mines the ledger → files a follow-up bead | `3db013a44` |
| M5 front-door guard | `scripts/check-frontdoor-admission.sh` (admission membrane) | `d368f346c` |

## The runner

`scripts/epic-d16-donetest.sh` drives the mechanical sequence over the real organs in an **isolated, self-contained temp root** (its own `docs/` + `schemas/` + `_beads`), so it is **repeatable and never pollutes the real `_beads` / provenance ledger** (verified: the real ledger line-count is identical before and after — `tests/scripts/epic-d16-donetest.bats` asserts it). It is a deterministic composition of the organs (not a multi-hour live coding worker); a live `codex exec` worker actually implementing a slice is the production demonstration, but the loop's mechanical closure — recovery → binding verdict → self-improvement — is proven here.

## A proven run (the six criteria)

```
{"donetest":"age-d16-self-hosting-route-nkr","result":"PASS",
 "seed_bead":"ag-odq","rescope_bead":"ag-cyt","verdict_head":"d16d0e7e57ca",
 "mined_bead":"ag-k9p","self_approval":"refused"}
```

| # | criterion (plan §48–55) | evidence |
|---|---|---|
| 1 | **Seed** — a real follow-up bead with a runnable acceptance is minted | bead `ag-odq` (with a `## Scenarios` Given/When/Then) |
| 2 | **No-human boundary** — launched on the Codex/local runtime; operator does not touch after launch | `codex exec --skip-git-repo-check …` + start/stop stamps (no wall-clock) |
| 3 | **Failure injection** — a slice fails; M2's recovery branch fires | branch=**rescope** → follow-up `ag-cyt` filed; **the seed is NOT closed** (recovery never holds close authority) |
| 4 | **Acceptance** — reaches accepted ONLY via a fresh-context pawl verdict row; self-approval refused | a `verdict` row lands in the ledger (head_sha `d16d0e7e57ca`); a second verdict whose only refuter ran in the author's own context is **REFUSED** at `pawl-verdict.sh check` |
| 5 | **Self-improvement** — the run's evidence is mined into a follow-up suggestion bead | M4 ASSAY tick filed `ag-k9p` |
| 6 | **Evidence paths** — the closing artifact lists every path | this table; missing any one ⇒ the runner emits `result:FAIL` |

## Acceptance test

`tests/scripts/epic-d16-donetest.bats` — 4 cases (skips cleanly if `br`/`ao`/an organ is absent; runs for real where they exist): the loop closes (PASS), every mechanical evidence path is present, the real repo ledger is untouched (isolation), and a `--workdir` inside the repo is refused (the isolation guard).

## Build notes (re-anchored on real interfaces)

- `ao provenance emit-verdict` resolves the ledger by walking up from cwd for a `docs/`+`schemas/` root (no env/flag override) — and `pawl-verdict.sh write` emits internally — so EVERY write/emit runs with `cwd=$WORKDIR` (a self-contained root outside the repo) to keep the real ledger untouched. Verified empirically.
- The pawl-verdict schema requires `pr ≥ 1`; the isolated store uses prefix `ag` so the verdict's bead id is an `ag-…` the M4 miner recognizes (the real ledger is all `ag-`).

## Status

The epic's mechanical done-test (§48–55) **PASSES**: M1–M5 integrate into one unattended goal→verified-done loop. The remaining child `…nkr.2` (RULER corpus-Δ PILOT) is **independent** (plan DAG line 44 — "run in parallel anytime") and continues in its own (Mossy's) lane; it does not gate the loop's closure.
