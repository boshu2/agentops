# PerspectivePlan: ARCHITECTURE / GATE-INTEGRITY lens — merge-door redesign (P2)

Lens: **the invariants must survive the re-sequencing.** Inputs: `.agents/research/merge-door-{constraint-model,rebaseline,design-space}.md` (2026-07-09). Target: design-space Composite A (two-phase done + WIP cap + compensation), amended where this lens demands stricter ordering.

## 1. The NEW invariant set (what main guarantees at any instant)

> **I1 — Structure.** `main = verified-prefix ++ pending-window`. Every commit in BOTH regions is deterministically green against its own tree (pre-push battery unchanged: build, per-commit tree builds age-yy24, full race suite, `ao gate check`). The race suite and per-commit builds NEVER move off-path — they ARE the deterministic floor.
> **I2 — No-verdict = not-done binds CLOSURE + the frontier, not push.** A bead closes only with a sealed verdict bound to its landed sha (`ao done --sha`, `pawl-verdict.sh check --landed`). Releases, tags, GOALS progress, and "done" claims reference ONLY the verified prefix (the LKG frontier = highest origin/main sha whose ancestors all carry bound verdicts, derived from `docs/provenance/ledger.jsonl` by merge_sha ancestry, never patch content).
> **I3 — The pending window is bounded and compensable.** ≤ N commits (N from meter data per D17, start ≈3; global-per-repo, ledger-derived, enforced as one deterministic check in the pre-push gate). Every pending commit has a review in flight or queued, and a defined compensator (§3) — "recoverable state" is a shipped mechanism, not a claim.
> **I4 — Fail-closed everywhere the timing moved.** Ambiguity pre-land holds the push (unchanged); ambiguity post-land holds the closure, the frontier, and the BC's cap — never silent-proceed into "done."

## 2. Contract clause amendments (exact replacement language)

| Clause | Replacement draft |
|---|---|
| `pawls.md:12` ("must confirm before the action proceeds") | "…must confirm **before the claim is accepted**. For mutate-shared-trunk, verification is two-phase: integration proceeds on the deterministic tier with a bounded pending window (≤N); the model verdict binds the **landed sha** and must confirm before the bead closes, before the commit enters the verified frontier, and before any release/done references it. Fail-closed: ambiguity → hold (pre-land: the push; post-land: closure + frontier), never silent-proceed." |
| `pawls.md:13` ("between them you only ever touch state you can recover") | "…a landed-but-pending trunk commit is recoverable state: deterministically green by construction, bounded to ≤N, and fully compensable (verdict-bound mechanical revert or P0 fix-forward, §compensation). The one-way doors are what LEAVES the verified frontier — closures, releases, dones." **Sequencing rule: this clause changes only after §3 ships** (re-baseline §one-way-door #2 — the real one-way door of the redesign). |
| `pawls.md:21` (guarded-by column) | "pre-push deterministic gate incl. WIP-cap check + async pawl reviewer (land-lane review phase, binds landed sha) + close door (`pawl-verdict.sh check --landed` at `br close`/`ao done`) + CI verdict-backstop (observe-only, pending-window grace)." |
| `pawls.md:197` (breaker → merge held) | "Pre-land, a tripped breaker holds the merge/push (unchanged). In the deferred lane, a tripped breaker holds the **frontier and the lane**: the commit is quarantined out of the verified prefix, its bead cannot close, the affected BC's WIP cap drops to 0, and the held object becomes the compensation decision (revert vs fix-forward) — itself routed through a door. Non-convergence never auto-closes and never enters the frontier." |
| `pawls.md:199` ("never auto-lands") | "Non-convergence never auto-lands (pre-land lane), never auto-closes, and never auto-enters the verified frontier (deferred lane)." |
| `pawls.md:229` vs `:90-91` contradiction | Resolve **in favor of `:90-91`** — the executable already sides there (`check-pawl-pre-push.sh:283-285`, NOTE not refusal), and precedence rule 1 says executable wins. Replace `:229`: "High-irreversibility doors (shared-ref rewrite, release cut, router-designated one-way doors) demand multi-model. Routine mutate-shared-trunk rides the fresh-context default (:90). Under deferred verification, push-time stakes are deterministic-tier; tier escalation belongs to the risk router (ebec.4) at the close/review, not the push." A contract clause the executable ignores is a standing integrity hole; retire it. |
| `GOALS.md:215` (D16 m3) | "…the in-repo ratchet writes the binding verdict at the **close pawl**: integration is admitted by the deterministic pre-push gate (pending window ≤N); the pawl binds its verdict to the landed sha before the bead may close or the commit may enter the verified/LKG frontier." |
| `head_sha` semantics (`schemas/pawl-verdict.v1.schema.json`, `pawls.md:76,:201`) | Add a **post-land binding mode**: `head_sha` = the landed sha observed on origin/main (merge_sha-from-origin rule). `pawl-verdict.sh check --landed` verifies head_sha is an ancestor of origin/main and inside the pending window. Architectural simplification: post-land shas are FINAL — the rebase-rebind/REBOUND churn and the amend-into-bind trap class (ebec.11) are deleted for this lane, not guarded. Pre-land verdicts keep existing semantics (the lane coexists). |

## 3. Compensator spec (REFUTED-after-land)

Precondition for ANY compensation: the REFUTED carries a **runnable repro** (pawl-refute discipline). No repro → **HOLD**, never revert — this is what keeps the refinery's blind-revert refusal (`refinery.go:4-5`, 18-30% flake) intact: we never revert on signal alone, only on a reproduced, verdict-bound defect. Flake-vs-defect triage reuses `land-lane-flaky-retry.sh`.

1. **Always:** auto-file a P0 fix bead carrying the defect list, bound to the refuting verdict id; emit `gate-verdict` — the ledger's escape shape (`escape.go:5-19`) already records overturns with **zero schema change**.
2. **Contained (default):** fix-forward; the BC's WIP cap → 0 until the fix bead closes (freeze valve). The tree is deterministically green by construction, so this is the common case.
3. **Contract / schema / security / public-surface / data classes:** **mechanical revert** — `git revert <sha>`, machine-verified as the exact inverse patch (tree-diff equality check, no LLM), bound to the **existing** refuting verdict id, riding an **L0 deterministic lane through the same pre-push gate** — no second model review (the revert's justification IS the verdict; "returns to verified state" is the evidence). Forward-history only (dcg-legal); never force-push. Write-intersecting dependents in the window evict to fix beads.
4. **Revert-of-revert:** **unconditional andon** — stop the line, human call, ledger arbitrates (newest verdict, what it binds), AM reservation on the contested path.

## 4. Async reviewer runner (ADR-0009)

**Primary: extend `land-lane-run.sh --watch` with a review phase** (the agentops-2pl single-writer lane; reuse `land-submit.sh` / `land-queue-next.sh` as the review queue). Why, from this lens: it is the only sanctioned runner that is *already the single serialized writer owning main* — putting verdict production, bind bookkeeping, and provenance appends **inside** that lane makes hash-chain contention (constraint-model §5.5) *structurally impossible* rather than lock-mitigated; it self-describes as ADR-0009-compliant ("thin foreground loop, not a service"); it is host-pinned to the always-on Mac and deliberately AM-independent; and its `LAND_LANE_GATE_ONLY_CMD`/`LAND_LANE_LAND_CMD` injection seams already separate gate from land — a third phase is a seam-native extension. **Checkout mechanism: the age-8ais detached-worktree re-target** (`check-pawl-pre-push.sh:183-220`) — review the pinned landed sha as its own tree, immune to subsequent lands. **Failover:** launchd/cron tick draining the queue via a `--once` review drain (the `pawls.md:224-227` reap precedent) when no lane is up; NTM tick during tended swarms. **CI (`verdict-backstop.yml`) stays observe-only** and gains a grace-window semantic (pending ≤ N annotated, never blocked; verdict production stays local — doctrinal bar holds).

## 5. Stage-4 fixed-cost fixes (R1 ranking → routing)

| Fix | Routing |
|---|---|
| Per-commit cockpit ~40s × N (age-8ais) | **Collapse on-path** into ONE `ao gate check --scope range <base>..<head>` — age-wy2t is LANDED (092ae748f); the loop's own comment pre-authorizes the follow-up. Deep per-commit assurance rides the async reviewer's detached-worktree pass. Biggest on-path deletion. |
| Bind-commit inflation 2× | **Deleted by the redesign**: verdicts bind post-land out-of-band (ledger append inside the review lane) — no `#trivial` bind commit per land, train length halves, range-gate N halves with it (constraint-model §5.4). |
| Double fresh-`ao` build | Stays on-path; dedupe to one build per push, cache keyed on `cli/` tree hash. Cheap. |
| In-lock mutable work | Provenance/ledger writes move to the review lane (single writer); the push lock shrinks to rebase+push only. |
| Full race suite (53-75s) | **Never moves.** It is invariant I1. |

## 6. Failure-mode table

| Failure | Detection | Response | Invariant held |
|---|---|---|---|
| Flaky reviewer REFUTE (no repro) | repro-required check on verdict | **HOLD, never revert**; warm→cold fallback; ≥3 same-class → scope-grind andon | I4 (no false compensation) |
| Reviewer stall (codex SPOF, 46-min tail) | pending window grows | cap N binds; push blocks with a ROUTING message ("drain review queue"), not a bare refusal; repeated cap-hits = andon (capacity signal). Frontier stops advancing — closes/releases fail-closed | I2, I3 |
| Ledger append contention at WIP>1 | n/a — prevented | all chain appends serialized inside the single-writer review lane; producers never append | I2 (chain integrity) |
| Concurrent-session pushes (Mac/bushido) | git non-FF + `push-serial.sh` rebase-retry | cap check is global (ledger-derived), not session-local; two same-instant pushes can overshoot to N+1 — acceptable (compensator bounds exposure); exact cross-host enforcement deferred to ag-arpk | I1, I3 (±1) |
| Deterministic-green under races | pre-push per-commit tree builds before lock; non-FF rebase re-runs the gate on the new tree | gate validates each commit against ITS OWN tree, not the worktree | I1 |
| REFUTED with dependents stacked | write-intersection over window diffs | contained → fix-forward + BC cap 0; escalation class → mechanical revert, evict intersecting dependents to fix beads | I3 |
| Revert war (A reverts, B re-lands) | revert-of-revert detector | unconditional andon; ledger arbitrates; AM reserve contested path | I4 |

## 7. Slice ordering under this lens

The design-space migration (observe→pilot→flip→compensation) is **re-ordered: compensation before the flip** — pawls.md:13 may not honestly change until recoverability is a mechanism (re-baseline §one-way-door #2). Every slice reversible by flipping order/config back.

1. **WIP-cap check + frontier derivation** in the pre-push gate, log-only → enforce. Deterministic, zero risk.
2. **Close-door hardening**: `--landed` binding mode + verdict-stamped close required. The invariant MOVES to closure while the door is still pre-land — no unenforced moment.
3. **Compensator arm** (auto-file, mechanical-revert L0 lane, freeze valve, andon rules) — built and tested while the door is still synchronous.
4. **Contract amendments** (§2) land together with:
5. **Runner**: land-lane review phase + detached-worktree checkout; pilot on the L0/docs lane (already async-shaped via the #trivial waiver).
6. **Flip the default** (push-then-pawl, N≈3) + age-wy2t range collapse + bind-commit deletion.

Router (ebec.4) stays D17 data-gated behind all of this; delta re-review (ebec.10) stays council-gated last. **The invariant never weakens — it relocates to where it always really lived: no verdict = not done, at the close.**
