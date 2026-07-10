# Session Audit — the control-plane program (2026-07-08 → 07-10)

> Consolidated audit of "all this work." Ground truth = bead states (`ao beads exec show`)
> + `origin/main` git log, not narration. Written as the session's closing artifact.

## The through-line

One thesis, driven end to end: **AgentOps is a control plane for stochastic agentic work —
enforce at the action, verify off the critical path, measure everything, feed it back.** The
session started as an audit + goal-completion pass and grew, through Bo's live steering, into
a shaped three-plane program (flow · measurement · enforcement), each grounded in a real
re-baseline/audit and gated by an adversarial pass.

## A. LANDED + CLOSED (verified on origin/main)

### Prior epics swept
- **`age-tc0l` Shift-left the membrane** — CLOSED. 8 slices: catch corpus → `ao membrane
  digest` → class-normalization (22→11) → placeholder filter → inject into pre-mortem +
  discovery → Go catch-extraction → routed-catch salvage → recurrence deltas. The loop
  **demonstrably closes**: a real `[gates]` catch flows corpus→digest→pre-mortem known_risk.
- **`age-36be` Flywheel-as-system** — CLOSED. Discipline made mechanical: goal-crafting
  skill (satisfied by the concurrent `goal-design` skill), batch-by-Gherkin gate, close-time
  learning checkpoint, on-the-loop `ao yield report`.
- **`age-pkrl` Membrane land-friction** — CLOSED (4 quick-wins + `ao land`), adversarially
  validated. **`age-zcvn` dual-support** — CLOSED (bd+br tracker-agnostic).

### Async Membrane wave 1 (`age-xnet`, epic OPEN — foundation landed)
- **`age-fdae` R1** — CLOSED. Verified-frontier + pending window in `ao yield report`. Live
  reading: frontier at 2026-07-04, 179 commits pending. Caught (pawl) a worktree-vs-origin
  ledger evidence fail-open + (race gate) a root-commit fixture infidelity.
- **`age-ekam` R3a (keystone)** — CLOSED. LKG frontier, compensated-ancestry `RESOLVED` (4
  arms, uniform REFUTED precedence, A6 evidence floor), 5 executed-red liveness tests. **2
  refute rounds** via direct codex-exec review: non-mainline merge-ancestor selection +
  two-resolver coherence split — both fixed.
- **`age-ivoq` R0** — CLOSED. METER un-broken (was 549/549 cost=0); duel proof packet
  mirrored to `docs/audits/merge-door-duel-2026-07-09/`. **3 refute rounds**: octal
  fail-open, attempt-distinctness under-count, malformed-token-as-measured, numeric-zero
  collision — every one a real correctness bug in gate-green code.

### Follow-ups & andon — closed
`age-dcek` (goal-design schema bug, un-redded main), `age-e65t`/`age-j8rn` (andon-router
graft + desc trim), `age-znst`/`age-74yi` (goal-crafting + batch gate), `age-i9ce`/`age-7hgb`
(fast-gate hardening), `age-7cws` (routed-catch reason), **`age-7awa` (andon: reviewer
degraded — RESOLVED, both parked slices drained via the direct codex-exec lane).**

## B. SHAPED, GATED, NOT YET BUILT (the forward program)

- **Async Membrane `age-xnet`** — 7 slices open: R2 (`age-vn4s`) L0 trailing-bind pilot on
  the docs/#trivial lane → R3b close gate (on R3a's `IsResolved` seam) → R4a/R4b
  compensators → R5 contract amendments + `--landed` binding + land-lane review phase → R6
  the flip (WIP cap, `AGENTOPS_PENDING_CAP` fail-closed) → R7 gate cost-down. **Design duel-PASSED** (3 rounds, claude+gpt, `ao plan-pawl decide` RC 0);
  forced 3 structural redesigns (frontier liveness → compensation grounding → evidence
  floor). Proof packet at `docs/audits/merge-door-duel-2026-07-09/`; model doc at
  `docs/architecture/the-flywheel.md`.
- **Telemetry plane `age-abgc`** — 13 slices open. **Gated on the zoning contract** (O0a-c):
  no stream lands without a declared retention policy — the structural fix for the
  junk-drawer audit's finding (193MB, 70% dead, prune-never-stuck because retention was
  prose not mechanism). O1 envelope → O2 real cost → O3 `ao telemetry export` (OpenMetrics,
  no daemon) → O4-O6 instrument the silent 78%-wrapper / 70%-implement / gate timings → O7-O8
  Loki+Grafana → O9 retention. **3 zoning calls settled by the wizards' duel** (consensus:
  compress-then-quarantine dedup + producer-fix; archive mine-events + adopt-or-kill v2;
  SINGULAR canonical + close the escape-hatch generator).
- **Admission control `age-4qw1`** — 4 slices open. Hooks as Kyverno-style validating
  admission (deny-by-default, silent-on-happy-path — the *enforcement* use the 3.0 teardown
  acquitted; only *injection* was convicted). H1 policy engine → H2 day-1 enforce cohort →
  H3 audit cohort → H4 default-wire + per-policy value-proof.

## C. OPEN FOLLOW-UPS (small, scoped)

`age-60qt` (REFUTED-review spend uncounted — a real meter gap found in R0's own review),
`age-cals` (24 pre-existing pawl-review bats host-flakes), `age-bau6` (mine-events v2
adopt-or-kill). None blocking.

## Meta-findings (what the session proved about the system)

1. **The membrane earns its cost at full strength.** The two parked slices came back through
   the direct `codex exec` reviewer and it REFUTED both across 5 rounds with **7 real
   correctness defects in gate-green code** the stalling pawl-wrapper never surfaced. Cross-
   family judgment catches what deterministic gates + author self-review miss.
2. **The wrapper is the flaky part, not the reviewer** (banked to `age-7awa`): when the pawl
   wrapper stalls but the reviewer is alive, review via direct `codex exec` + seal the
   verdict from the evidence (reap-to-verdict). Direct input for O5 (wrapper decomposition).
3. **The andon works.** Reviewer-infra degradation parked slices safely with a written resume
   path rather than grinding a dead reviewer; the duel breaker forced plan redesigns before
   beads; the zoning duel caught 4 defects either solo answer would have shipped.
4. **The constraint is policy, not resource** (measured): WIP=1 single-piece flow, codex 3%
   utilized, 78% of review latency is bash wrapper — which is *why* async verification (+44%
   throughput) is the redesign, and why the meter (now un-broken) had to come first.

## Honest scope

No compounding-moat claims (ADR-0004/0011 stand). The proven product is the verification
itself (no verdict = not done). Each feedback loop is falsifiable per-loop (did the class
recur less; did the frontier advance; did takt drop). The forward program is tracked and
adversarially vetted, not asserted.
