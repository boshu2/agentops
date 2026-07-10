# SynthesisPacket — The Async Membrane: two-phase done, trunk-based push-to-main

> Synthesized from R1 (constraint model), R2 (re-baseline), R3 (design space), P1 (product),
> P2 (architecture), P3 (operations) — `.agents/research/merge-door-*.md`, `.agents/plans/2026-07-09-merge-door-P*.md`.
> Duel input; beads are created only after `ao plan-pawl decide` PASS.

## Selected design (Composite A, hardened)

**Two-phase done.** `main = verified-prefix ++ pending-window(≤N)`, every commit
deterministically green against its own tree (race suite + per-commit builds never move
off-path). The cross-family judgment review moves **off the push path** into an async
review lane; **no-verdict = not-done re-binds to bead CLOSURE and the LKG frontier**
(releases/dones reference only the verified prefix). The membrane doesn't die — it moves:
from turnstile to telemetry on an experiment stream. Each landed commit is an experiment:
hypothesis = the bead's acceptance; guardrail = the deterministic gate; readout =
PENDING → CONFIRMED / REFUTED(+defects → auto-filed P0 fix bead + catch class → discovery
`known_risks`, already landed).

**Why (measured):** the constraint is a *policy*, not a resource — WIP=1 single-piece flow
with the orchestrator idle-waiting 14.1 min/land on membrane stages (30% of takt); codex is
3% utilized; stage-3 latency is 78% wrapper / 22% model; retry amplification 1.79
attempts/land. Async stages 3+4 ⇒ ~+44% throughput at current λ; the honest per-user win at
WIP=1 is **unblocked pushing + batched drains** (P1), not a 2× headline.

## The product contract (P1)

- Three states, one word reserved: **LANDED → VERIFIED → DONE**; only the third is "done".
  The close door (`br close`/acceptance) is UNCHANGED — it refuses without a verdict.
- `ao yield report` is the daily driver: pending window, verdicts-as-they-arrive, andon
  queue, verified-frontier sha. Ships **before** any flow change (visibility first).
- Default posture: instant push (deterministic gate only) + **review-at-close, foreground**
  for zero-infra/single-CLI users (the wait moves from push to close). On-the-loop async
  drains (land-lane watch / cron / NTM tick / gc) are **opt-in**.
- **Auto-revert is never default.** Escalation classes only, repro-required.
- Tracker-agnostic (bd + br). No corpus-compounding claims (ADR-0004/0011).

## The invariant + contract surface (P2)

- I1 pending-window bounded (`AGENTOPS_PENDING_CAP`, ledger-derived, one deterministic
  pre-push check; unset = kill switch; start N=3 from λ·W ≈ 0.3, latency data — not gated
  phase-2 cost data). I2 deterministic-green everywhere, unchanged. I3 LKG frontier =
  highest origin/main sha with all-ancestor verdicts (provenance-ancestry-derived). I4
  fail-closed relocates: post-land ambiguity holds closure + frontier + BC cap→0, never
  silent-proceed.
- Clause amendments (drafted verbatim in P2 §2): pawls.md:12 (confirm-before-CLAIM,
  two-phase for mutate-shared-trunk), :13 (**only after the compensator ships** — the true
  one-way door), :21, :197/:199 post-land breaker analogues, **:229 resolved toward the
  executable** (fresh-context default; tier escalation belongs to the ebec.4 router),
  GOALS.md:215 (binding verdict at the CLOSE pawl), verdict schema gains `--landed`
  binding mode (ancestor-of-origin/main) — **deletes the rebind/restamp churn class**.
- Compensator: REFUTED-after-land ⇒ always auto-file P0 fix bead bound to the refuting
  verdict id (escape.go shape, zero schema change). Contained → fix-forward + BC cap→0.
  Contract/security → **mechanical revert bound to the refuting verdict**, machine-verified
  inverse patch, L0 lane, no second review; **no-repro → HOLD** (honors the refinery's
  18-30% blind-revert flake prior). Revert-of-revert → unconditional andon.
- Runner: **land-lane review phase** (`land-lane-run.sh --watch`, the already-built
  single serialized writer of epic agentops-2pl) — makes ledger hash-chain contention
  structurally impossible; checkout via the age-8ais detached-worktree re-target;
  failover cron/NTM-tick `--once`; CI stays observe-only (grace window = cap).

## The migration ladder (P3 + P2's re-order: compensation BEFORE the flip)

| Rung | Slice (one behavior) | Rollback |
|---|---|---|
| R0 | Fix METER instrumentation (549/549 cost_usd=0 — the D17 ruler is broken; blocks all before/after claims) | revert |
| R1 | Land `ao yield report` (in flight, age-mv67) + frontier line | revert |
| R2 | L0 trailing-bind pilot on the docs/#trivial lane (already async via waiver — proves bookkeeping at zero risk) | revert |
| R3 | Close-door hardening + LKG frontier computation (while the door is still pre-land) | revert |
| R4 | Compensators, 3 sub-slices: auto-file P0 → mechanical-revert L0 lane → BC freeze valve | each revertible |
| R5 | Contract amendments + `--landed` binding + land-lane review phase (pilot: review runs async, door still blocks) | revert |
| R6 | **Flip the default**: push = deterministic-only; WIP cap on; review trails | `unset AGENTOPS_PENDING_CAP` + revert flip; drain oldest-first |
| R7 | Gate cost-down (next constraint at ~19% of new takt): single `ao` build, **adopt the landed age-wy2t `--scope range`** (shipped, unadopted — the per-commit ~40s loop), bind-commit dedup (2×), 21% replay-tax fixes | revert |

Each rung: measured against R1 baselines (1.30 lands/h active, 46-min takt, 14.1-min
membrane wait, 1.79 attempts/land, 21% stage-4 failures); holds ≥1wk/≥20 lands. The
decisive fresh measurement R6 unlocks: **REFUTED-after-land rate** — the pending window is
the only named source of real escape data (the ADR-0011 revival mechanism).

## Rejected alternatives

- **Merge queues/trains** (bors/GH-MQ): keep the slow probabilistic reviewer on the path;
  the invariant they protect (deterministic-green main) already holds without one. Take
  Zuul's speculation later, not the serialization.
- **Immediate ebec.4 router promotion**: GOALS.md:242-244 is a binding honesty rule; the
  ladder needs no bypass (nothing routes by tier or cheapens review). Early promotion is a
  **named andon item escalated to the operator**, never silent. ebec.10 stays
  council-gated.
- **Auto-revert by default**: trust-destroying; repro-required escalation only.
- **Big-bang flip**: every rung reversible; the flip is one env/config change.

## Round-2 amendments (duel round-1 findings, all adopted)

**A1 — Compensated-ancestry frontier (gpt FAIL #1, the liveness hole).** The naive rule
("every ancestor carries a verdict") deadlocks at the first REFUTED-after-land: forward-only
compensation never removes the refuted sha from ancestry, so the frontier could never
advance again. Amended I3: **LKG frontier = highest origin/main sha whose every ancestor is
RESOLVED**, where `RESOLVED(c)` := CONFIRMED verdict bound to c ∨ **verified-by-waiver**
(#trivial / provenance-only per the existing `trivial-waiver.sh` predicate, recorded as a
waiver edge) ∨ (REFUTED ∧ a **resolution edge** exists: a landed compensating commit —
mechanical revert or fix-forward — bound to the refuting verdict id in the provenance
ledger, itself RESOLVED). Resolution edges are ordinary hash-chained provenance edges
(relation `resolves`), appended by the compensator. **Frontier-liveness acceptance tests
are part of the R3 slice contract (executed-red before implementation):** (a)
REFUTED→fix-forward advances the frontier past both commits; (b) REFUTED→revert advances;
(c) unresolved REFUTED holds the frontier + blocks dependent closures + trips BC cap→0;
(d) a #trivial commit advances via waiver; (e) a resolution chain (revert-of-revert)
requires andon before any edge counts.

**A2 — Closure is NOT mechanically fail-closed today (gpt FAIL #2).** `ao done` is
warn-first and raw `br close` bypasses any check — the synthesis previously overstated
this. R3 is restated from "close-door hardening" to **"CREATE the mechanical close gate"**:
bead closure for landed work must refuse (fail-closed) when no CONFIRMED verdict is bound
to the bead's landed sha (or resolution edge per A1) — implemented at the `ao`
close path with the posture configurable for product users (strict default in this repo),
and honest docs that raw-tracker `br close` bypass is closed by policy + the close-integrity
audit, with the gate check `closure.verdict-bound` as the mechanical backstop.

**A3 — Cap kill-switch fails CLOSED (claude WARN).** `AGENTOPS_PENDING_CAP` env contract:
**unset ⇒ synchronous door (equivalent to cap=0)** — the pre-async behavior. The async flow
runs ONLY with an explicit positive cap. There is no state in which the door is async and
uncapped; R6 rollback = unset (one lever, fail-closed), plus the flip revert.

**A4 — Bind-batching honesty (claude WARN).** Out-of-band ledger binds still land as
provenance commits, batched by the land-lane single writer: the gain is ~1.1–1.2× commit
deflation (batching), not the earlier "deletes bind commits" 2× claim. Frontier treatment
of those provenance-only commits is the waiver edge of A1.

## Round-3 amendments (duel round-2 findings, all adopted)

**A5 — RESOLVED gains a fourth base case: verified-by-compensation (gpt FAIL, claude edit 1).**
A mechanical revert gets no second model review by design — so it is neither CONFIRMED nor
provenance-only, and round-2's definition left its recursion ungrounded (test (b)
unimplementable). Amended: `RESOLVED(c)` := CONFIRMED ∨ verified-by-waiver ∨
**verified-by-compensation** ∨ (REFUTED ∧ resolution-edge to a RESOLVED compensator).
`verified-by-compensation(c)` is DETERMINISTIC: c is a machine-verified inverse patch of a
REFUTED commit (git-diff equality check against the inverse), bound to the refuting verdict
id — the check itself emits an L0 verdict edge (the verification IS the review; no model).

**A6 — Resolution-edge evidence floor (claude edit 2, the deepest one).** "Itself RESOLVED"
only proves the fix commit passed its *own* acceptance — not that the original defect is
gone; a superficial fix could advance the frontier past still-broken code. Amended: a
`resolves` edge is valid ONLY with (i) the **refuting verdict's repro executed GREEN at the
compensating sha** recorded in `evidence_ref`, and (ii) binding to the auto-filed P0 fix
bead. No green repro at the fix sha → the edge does not count → frontier holds (fail-closed).

**A7 — Disjunction precedence + edge validation (both judges).** An existing REFUTED verdict
**dominates** the waiver arm (a REFUTED provenance-only commit can never silently
waiver-resolve). The `resolves` edge gets schema support + validation: strict-descendant
(compensator is a descendant of the refuted sha), acyclicity, uniqueness (one live
resolution edge per refuted sha), and **exact reuse of the fail-closed `trivial-waiver.sh`
predicate** for the waiver arm — no parallel waiver logic.

**A8 — A2 provisos (gpt).** Strict closure posture **disables `--force-no-verdict`**; the
`closure.verdict-bound` backstop validates the **ledger proof** (hash-chained edge for the
landed sha), never mere stamp/file presence.

## Open questions (for the judges)

1. Cap semantics cross-host: advisory ±1 until ag-arpk (GitHub MQ as cross-host
   serializer) — acceptable?
2. Review-at-close default for single-CLI users: is the moved wait honest enough, or does
   it need a batching affordance day-1?
3. The mechanical-revert "no second review" claim: inverse-patch verification is
   deterministic, but the *decision* to revert routes by severity — is the severity
   classifier's fail-closed default (HOLD) sufficient?
4. Does R6 need a per-BC pending cap in addition to the global one?

## Risks (top, cross-lens)

- Landed-feels-done drift → frontier-only releases + report habit + close-door unchanged.
- Broken METER blocks every claim → R0 first, non-negotiable.
- Compensator flake (blind-revert prior) → repro-required, HOLD default.
- Concurrent sessions during migration → land queue (submit-don't-push) day-1 for second
  writers; worktree-per-bead; never append the hash-chained ledger concurrently.
- Single-CLI users gain little at WIP=1 → sell honestly (P1): unblocked push, not 2×.
