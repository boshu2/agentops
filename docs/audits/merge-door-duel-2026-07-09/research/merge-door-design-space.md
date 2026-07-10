# Merge-Door Design Space: High-Throughput Trunk-Based Systems Mapped onto AgentOps

> Research survey, 2026-07-09. READ-ONLY analysis; no code changed. Grounding: GOALS.md
> Directive 17, epic `age-verification-economics-ebec` (esp. `.4` router spec, `.10` delta
> re-review spec, `.9` pre-flight gate landed), AGENTS-WORKFLOW land flow, the pawl/bind/close
> machinery, `docs/audits/verification-economics.md` (322 verdict edges, 2.8% refute, 0 escapes).

## 0. The as-is door and the constraint set

Current land flow (per bead): commit → **pawl review (cross-family, fresh-context, minutes)** →
emit-verdict (SEALED) → `#trivial` bind commit → push (pre-push Go gate = full deterministic
battery, per-commit tree-gated, CI-equivalent) → verify-landed → close bead.

The cross-family reviewer sits **on the critical path of integration**. Every land pays reviewer
latency serially, even though:

- The **deterministic floor is already strong**: the pre-push gate validates *each commit against
  its own tree* (build, vet, race suite, ~30 GOALS gates). Main is deterministically green by
  construction — stronger than Google's presubmit-light.
- The **refute rate is ~2.8%** (4–9 REFUTED per few hundred binds in the current regime), and
  **0 escapes ever** (rule-of-three CI: true escape rate ≤ ~2%). So >97% of lands pay full
  reviewer latency to confirm what was true.
- The pre-flight gate (`.9`, landed 36a891ac8) already collapsed the multi-round bottleneck:
  the reviewer never sees a deterministically-red tree.

Constraints any design must respect:

| Constraint | Source |
|---|---|
| **No verdict = not done** — the product invariant; can be re-scoped, never weakened | CLAUDE.md doctrine |
| Sealed verdicts + `#trivial` bind commits; binds reference `merge_sha` **from origin after push** | pawl machinery; memory `merge-sha-from-origin-after-push` |
| Verdicts are evidence- and commit-bound; patch-id REBOUND exists for clean rebases (age-rk3r.9) | membrane core |
| Hot repo, multi-session (Mac + bushido + swarms); worktree-per-bead; direct-to-main, no PRs | AGENTS-WORKFLOW |
| Directive 17 honesty: phase-2 actions (router, cheap tier) stay DEFER until meter data | GOALS.md §17 |
| Andon on CLASS-repeat, not round-count; ≥3 same-class refutes → STOP re-scope | membrane memories |
| REFUTED requires a runnable repro (pawl-refute discipline); warm pawl can degrade → cold fallback | memories |
| dcg blocks reset/restore/stash-clear/rm-rf; `git revert` (forward-only history) is permitted | dcg |

The design question: **where does the expensive probabilistic check sit relative to integration,
and what compensates when it fires late?**

---

## 1. Optimistic post-merge verification (Google TAP, Meta land-then-verify)

**Mechanism.** Presubmit is deliberately light (fast, affected-targets subset); the heavy suite
runs *postsubmit*, batched across many CLs at milestone cut points. On red: **culprit finding**
(binary search / per-target bisection over the batch), then **revert-first, debug-later** — the
rollback is a compensating transaction that restores known-good, and it gets *expedited* review
precisely because "returns to a previously verified state" is itself the evidence. Build cops /
sheriffs own the drain. Meta's equivalent: land-time checks + async signals + revert bots;
"fix-forward vs revert" is a policy decision per defect class, not per engineer mood.

**Optimizes.** Producer latency → ~0; integration throughput decoupled from verification
capacity entirely. Verification becomes a *consumer* of the commit stream, batchable and
schedulable.

**Failure modes and how industry handles them.**
- *Revert storms*: a bad commit with dependents stacked on top → reverts conflict. Handled by
  fast detection (small batches near the tip) and by making revert mechanical (`git revert` of a
  named sha, no hand-editing).
- *Culprit ambiguity in batches*: N commits, one red signal. Handled by bisection; cost is
  O(log N) re-runs, so batch size is tuned to failure rate.
- *Flaky signal → false reverts*: the worst social failure. Handled by quarantine lists and
  "must reproduce before revert" rules.
- *Broken-window contamination*: everyone who syncs pulls the defect. Google tolerates this
  because presubmit catches the compile/test class; only the subtle class leaks.

**Fit to AgentOps: HIGH — and strictly safer here than at Google.** The contamination risk that
makes TAP scary is mostly absent: AgentOps' pre-push battery is *heavier* than Google presubmit
(full race suite, per-commit tree gating), so the only class that can land unverified is the
class the pawl catches — semantic/contract/intent defects — which is exactly the ~2.8% refute
class, and which does not break a puller's build. The compensating-transaction discipline maps
cleanly:

- **Revert-needs-review, resolved**: a revert commit's justification IS the REFUTED verdict.
  Bind the revert to the *existing* refuting verdict id (compensating transaction referencing
  the original), and verify it deterministically: the revert must be a clean `git revert <sha>`
  whose tree-diff is the inverse patch — checkable by machine, no LLM in the loop. Reverts ride
  an L0 deterministic lane. This closes the "does a revert itself need cross-family review?"
  loop without a second reviewer round-trip.
- **Culprit finding**: mostly moot — the pawl reviews per-bead-arc, commit-bound, so a REFUTED
  names its commit and its defects. Batched ambiguity only arises if the membrane is allowed to
  review a *window* of commits at once (a cost optimization); then bisection over the window is
  cheap because the window is WIP-capped (§4).
- **Flaky-reviewer false reverts**: already doctrinally handled — REFUTED without a runnable
  repro is not actionable (pawl-refute memory); spurious-REFUTED from a degraded warm pawl has
  the cold fallback. Rule: **no repro → HOLD, never auto-revert.**

**Residual risk.** The window between land and verdict is real exposure for *release* consumers.
Compensator: releases/tags cut only from the **verified frontier** (last commit with all verdicts
bound at or before it) — the "last-known-good green snapshot" pointer, exactly Google's LKG cut.

---

## 2. Merge queues / trains (bors, homu, GitHub merge queue, Zuul)

**Mechanism.** Serialize the *merge decision*: candidate changes enter a queue; the queue tests
`main + change` (bors), or **speculatively** tests `main + c1`, `main + c1 + c2`, … in parallel
(GitHub MQ, Zuul), committing each result only if everything ahead of it passed. On failure:
evict the culprit, restart speculation behind it. Batching + bisection (bors rollups) amortize
CI cost.

**Optimizes.** Main is *always* green at the tested granularity — the strongest invariant in the
space. Speculation recovers parallelism that serialization destroys.

**Failure modes.** Queue latency explodes under load (Little's Law: W = L/λ; any failure
restarts everything behind it — quadratic waste at high arrival × nontrivial failure rate);
flaky tests poison whole trains; head-of-line blocking; heavy infrastructure (queue state,
speculative refs, eviction logic).

**Fit to AgentOps: LOW as an architecture — but its one good idea transfers.** Three reasons it
does not fit:

1. It keeps verification **on the critical path** — the exact property Directive 17 is trying to
   escape. The queue only helps when the check is fast relative to arrival rate; the pawl is
   minutes of LLM latency with a probabilistic output. A REFUTED restarts the train: with a
   2.8% refute rate and swarm-burst arrival, trains re-speculate constantly.
2. AgentOps is a single-remote direct-to-main repo with worktree-per-bead lanes; the "queue" is
   already implicit in `git pull --rebase` + agent-mail reservations. Standing up bors-class
   infra to serialize what one hot remote already serializes buys nothing.
3. Determinism is already guaranteed per-commit by the pre-push gate — the invariant merge
   queues exist to protect (deterministic green main) **already holds without a queue**.

What transfers: **speculation without serialization.** Zuul's insight — verify N *while* N+1..k
proceed on the optimistic assumption N passes — is exactly the async/parallel review of §6, with
the WIP cap of §4 playing the role of the queue-depth bound. Take the speculation, leave the
serialization.

---

## 3. Two-phase done (landed ≠ done) — the pivot pattern

**Mechanism.** Decouple **integration** (push, gated deterministically) from **acceptance**
(verdict, asynchronous). The bead stays open until the verdict lands; **no-verdict = not-done
binds CLOSURE (and release), not PUSH.** The flow becomes:

```
commit → pre-push deterministic gate → push → [async: pawl review of pushed sha →
sealed verdict → #trivial bind commit] → verify-landed → close bead
```

This is a *re-sequencing* of existing machinery, not new machinery: binds already trail the work
commit as provenance-only `#trivial` commits referencing `merge_sha` taken from origin *after*
push; verify-landed already checks merge-base ancestry before close; the close door is already
where the product invariant is enforced. The amend-into-bind trap (`.11`) was a hazard *created*
by coupling review to the pre-push window — decoupling deletes that hazard class rather than
guarding it.

**The invariant, stated precisely (what main guarantees at any instant):**

> Main = **verified prefix** ++ **pending window**. Every commit in both regions is
> deterministically green against its own tree (pre-push gate, unchanged). Every commit in the
> verified prefix has a sealed, commit-bound verdict in the ledger. The pending window is
> bounded: ≤ N commits (§4), each with a review in flight or queued. **"Done" claims — bead
> closures, release tags, GOALS progress — only ever reference the verified prefix.**

The product guarantee is *re-scoped, not weakened*: nothing is ever **called done** without a
verdict. What changes is that integration is no longer conflated with acceptance — which is also
the honest description of what the system already believes (the bead, not the push, is the unit
of done; the ledger, not the branch tip, is the proof surface).

**Compensating transactions (what happens on REFUTED-after-land):**
- **Auto-file** a P0 fix bead carrying the defect list, bound to the refuting verdict (this is
  the "auto-filed catch" — the experiment readout, §7). Default for contained defects (the tree
  is deterministically green by construction, so "contained" is the common case).
- **Auto-revert** for the escalation classes (security, contract/schema, public-surface, data
  handling): mechanical `git revert`, bound to the refuting verdict id, L0 lane (§1). Dependents
  in the window that conflict with the revert get evicted to fix beads.
- **Andon on revert-storm**: ≥2 reverts in a window, or any revert-of-revert → stop-the-line,
  human decision. Consistent with the existing CLASS-repeat andon rule.
- **Freeze valve**: while a REFUTED is unresolved in a bounded context, the WIP cap for that BC
  drops to 0 (no new lands stack on a known-bad region).

**Failure modes.**
- *Pending window grows unbounded when the reviewer stalls* → that is not a failure of this
  pattern, it is the reason §4 exists. Without a cap, two-phase done silently degrades into
  "verify never."
- *Dependents stacked on a refuted commit* → bounded by the cap; decided by defect class
  (fix-forward default, revert for escalation classes).
- *Verdict/commit ordering skew* (verdicts land out of push order) → already handled: binds
  reference merge_sha, the ledger is per-record hash-chained, closure checks its own bead's
  verdict, not global order.
- *Psychological/process drift*: "landed" starts *feeling* like done → counter with the frontier
  made visible (report `.2` prints pending-window depth; the close door still refuses).

**Fit: VERY HIGH.** Highest fit-to-machinery of any pattern in this survey — it is the existing
parts in a different order, and it is the precondition that makes §§4–6 meaningful.

---

## 4. Backpressure / WIP limits (Little's Law on the commit stream)

**Mechanism.** Cap the number of **unverified-but-landed** commits at N. Push proceeds freely
while pending < N; at the cap, push blocks (or queues) with an explicit message: *"pending
verdicts: N — drain the review queue or escalate."* Little's Law makes the sizing empirical:
steady-state pending L = λ (land rate) × W (verdict latency). The meter (`.1`, landed) measures
both, so **N is set from data, not invented** — precisely the Directive 17 honesty rule. Start
N ≈ 2–3× the measured λW headroom.

This is Kanban WIP discipline applied to a commit stream: the cap does not *create* the
constraint (reviewer throughput), it makes the constraint **visible and load-bearing** instead of
implicit. When the cap binds, the correct response is the kanban one — the blocked producer
becomes a drainer (run/attend the pending reviews), not a queue-jumper.

**Implementation surface (all existing):** one check in the Go pre-push gate — count commits on
`origin/main` after the ledger's verified frontier (highest merge_sha with a bound verdict,
ancestry-checked); compare to N. The frontier is derivable from `docs/provenance/ledger.jsonl`
already; the report script already reads it. **Must be global-per-repo** (ledger-derived), not
session-local — multi-session lanes (Mac/bushido/swarms) share the one window; cap contention
coordinates through agent-mail like any other shared-resource conflict.

**Failure modes.** Cap too low → verification is back on the critical path (the failure §3
escaped); cap too high → decorative; frontier ambiguity under rebase churn → anchor on
merge_sha ancestry, never on patch content; **cap-hit livelock** in an unattended swarm (every
lane blocked pushing, nobody draining) → the cap-hit message must *route* ("run pawl-drain"),
not merely refuse, and repeated cap-hits are an andon signal (reviewer capacity, not producer
discipline, is the constraint).

**Fit: VERY HIGH.** Cheapest pattern in the space (one deterministic check over an existing
artifact), and it is the safety valve that makes §3 honest.

---

## 5. Risk-tiered routing (ebec.4)

**Mechanism (as specified in the bead — deterministic + fail-closed):**

| Tier | Scope (deterministic rules) | Verification |
|---|---|---|
| L0 | docs/provenance-only diffs; mechanical reverts (§1) | deterministic gates only |
| L1 | small, single-BC code diffs | single cheap-family fresh-context review |
| L2 | multi-file / cross-BC, security, contracts, public surfaces | full cross-family duel |
| L3 | one-way doors | tri-family / council |

Uncertainty **fails closed** to the higher tier. Route decision + outcome logged per close;
misroutes are escapes and **compile into router rules via the EM spine** — the router is itself
inside the self-improvement loop.

**Industry analogs.** Google/Meta predictive test selection and diff-risk classifiers (route CI
depth by change risk); Chromium CQ trybot selection; docs-only ship-it fast paths; OWNERS-scoped
review depth. The consistent industry lesson: **path/scope-based deterministic rules first,
learned classifiers only after a labeled corpus exists** — which is exactly the bead's shape
(deterministic rules now; per-tier refute yield from the report decides recalibration).

**What it optimizes.** The **demand side**: §§1/3/4 move verification off the critical path;
this shrinks how much verification is needed at all. They compound: with tiering, the pending
window (§4) holds fewer L2 reviews, so W drops, so the same cap N supports higher λ. CPCD per
tier (Directive 17 ruler) is the feedback instrument: a cheap tier whose refute yield collapses
to zero is either safe or blind — the escape harvest (`.6`) disambiguates.

**Failure modes.**
- *Misclassification*: an innocuous-looking diff crossing a contract boundary. Counter: rules on
  file paths × the BC component map, fail-closed on any ambiguity; misroute-as-escape recompiles
  the rule.
- *Goodhart/slicing*: an agent (or a habit) slices work to duck under tier thresholds. Counter:
  **tier by the union write-scope of the bead arc**, never per-commit; the bead is the routing
  unit, matching "the bead is the unit of done."
- *Tier ossification*: rules never loosen. Counter: Directive 17's over-provisioning clause —
  budget untouched at current spend → move a lane one tier cheaper *and watch the meter*.

**Sequencing constraint (binding):** GOALS.md holds phase-2 in DEFER until the ruler reads two
weeks of meter data. Any composite must sequence the router *after* the meter matures — the
bead's own done-criteria (20+ routed closes with per-tier yield) already encodes this.

**Fit: HIGH** (specified, gated correctly, waiting on data — not on design).

---

## 6. Speculative / parallel review + delta re-review (ebec.10)

**Mechanism.** Two related moves:
- **Speculative parallel review** (Zuul's good idea, minus the train): review commit/arc N while
  N+1..N+k land. Natural and almost free once §3 lands — reviews of pending-window entries are
  independent jobs against pinned shas; nothing serializes them. The WIP cap is the speculation
  depth bound.
- **Delta re-review** (`.10`, deferred with a full safety design): on REFUTED→fix, the reviewer
  gets the prior defect list + `git diff <last-reviewed-sha>..HEAD` + a bounded "rest unchanged,
  reviewed at <sha>" attestation — not the full 147-file diff again. N rounds stop costing N
  full reviews. Lineage-aware, sibling to the patch-id REBOUND path (age-rk3r.9) which already
  spares clean rebases a re-review.

**What it optimizes.** Verdict latency W (parallelism) and re-review cost (the multiplier that
made the retire-wave night expensive). Feeds directly back into §4's Little's-Law sizing.

**Failure modes.**
- *Semantic interference between parallel reviews*: N REFUTED while N+2 (built atop it) already
  CONFIRMED — the confirmation implicitly assumed N. Counters: review at **bead-arc granularity**
  (arcs are the semantic unit; write-scope non-overlap between concurrent arcs is already the
  wave-planning rule); the cap bounds blast radius; a REFUTED invalidates only verdicts whose
  arcs *write-intersect* it (cheap to compute from the diffs) — those re-enter as delta reviews.
- *Delta blindness*: a fix that is individually fine but whose interaction with the unchanged
  remainder is the defect. Counter (from the bead's own design): full re-review when the delta
  is large or on operator `--full-review`; and the attestation names the sha the remainder was
  verified at, keeping the chain auditable.
- *Flaky reviewer across rounds* (contradictory refutes): existing scope-grind andon (≥3
  same-class refutes → STOP re-scope) and warm→cold fallback already govern this.

**Fit: MEDIUM.** REBOUND machinery proves the lineage-aware pattern works; but `.10` is a
membrane-core change the epic explicitly deferred to fresh-mind + council, and `.9` already
captured most of its value. In composites it is the *last* optimizer, never a prerequisite.

---

## 7. The observability framing: main as an experiment stream

The deepest consequence of §3 is a reframe, not a mechanism. Once acceptance is asynchronous:

- **Each commit is an experiment.** Its hypothesis is its bead's acceptance criteria (the
  Given/When/Then it claims to satisfy). Admission control (pre-push gate) is the guardrail
  metric; the pawl is **post-hoc measurement**, not a door.
- **A REFUTED-after-land is the experiment's readout, not a gate failure.** The auto-filed fix
  bead (with defect list, bound to the verdict) is the published result. The membrane stops
  being a bouncer and becomes an instrument — which is the only posture under which Directive
  17's "spend to the error budget, not to zero" is even coherent. A door must be paranoid; an
  instrument is sized to the error budget.
- **The ledger is the lab notebook**: hash-chained, sealed, commit-bound — already built for
  exactly this evidentiary role.

**Three Ways mapping (DevOps):**

| Way | Merge-door expression | Status |
|---|---|---|
| **Flow** (fast left-to-right) | Verification off the critical path (§3); WIP-limited (§4); demand-tiered (§5) | this design |
| **Feedback** (amplify loops) | Async verdicts feed **discovery `known_risks`** / pre-mortem compiled-prevention — a catch becomes a named risk on the *next* plan | surface already landed (`skills/pre-mortem/references/compiled-prevention.md`, `skills/discovery/references/dag.md`) |
| **Learning** (experiment culture) | The catch corpus / EM spine / external measurement register: every escape and misroute compiles into a deterministic check or router rule | EM spine proven e2e; corpus-compounding honestly held unproven (ADR-0011) |

This framing also resolves the apparent tension with ADR-0011 (self-improvement is
data-starved because a competent membrane catches everything at review): moving review post-land
and tiering it cheaper is **the only named path to real escape data** — the epic's own thesis
that "cost cutting and corpus compounding rescue each other."

---

## 8. Pattern-fit matrix

| Pattern | Throughput gain | Safety | Complexity | Fit-to-existing-machinery |
|---|---|---|---|---|
| 1. Optimistic post-merge verify | **High** (producer latency → ~0) | Medium-High (deterministic floor holds; semantic window bounded by §4; revert lane mechanical) | Medium (compensation discipline, frontier pointer) | **High** — trailing binds, merge_sha-from-origin, verify-landed, dcg-legal revert all exist |
| 2. Merge queue / train | Low-Medium (serializes; speculation claws some back) | **Highest** (main green at tested granularity — but AgentOps already has this deterministically) | **High** (queue infra, eviction, speculative refs) | **Low** — solo-remote, LLM-latency reviewer on path, invariant already held by pre-push gate |
| 3. Two-phase done | **High** (unblocks the stream) | **High** (invariant re-scoped to closure/release, never weakened) | Low-Medium (re-sequencing + auto-file/auto-revert) | **Very high** — existing parts, different order |
| 4. WIP cap / backpressure | n/a (protective; preserves 3's gain honestly) | **High** (bounds exposure; makes constraint visible) | **Low** (one gate check over the ledger frontier) | **Very high** — meter (.1) sizes N from data |
| 5. Risk-tiered routing | Medium-High (demand ↓ ⇒ W ↓ ⇒ same cap, more λ) | Medium-High (fail-closed; misroute = escape → EM spine) | Medium (deterministic rules × BC map; logging) | **High** — ebec.4 fully specified; D17-gated on meter data |
| 6. Speculative + delta review | Medium (W ↓; kills the N-rounds×full-review multiplier) | Medium (interference + delta-blindness need arc-granularity + invalidation rule) | Medium-High (membrane-core; epic wants council) | Medium — REBOUND proves lineage-aware pattern; `.10` deferred by design |

---

## 9. Composite designs

### Composite A — **The Async Membrane**: two-phase done + WIP cap + auto-compensation (§3+§4+§1's revert lane)

The minimal composite that changes the economics. Pure re-sequencing: push gated only by the
deterministic battery; pawl reviews the *pushed* sha; bind trails; **close** gated on verdict;
global WIP cap N (data-sized) in the pre-push gate; REFUTED-after-land → auto-filed P0 fix bead,
tier-independent; auto-revert reserved for escalation classes, riding the L0 mechanical-revert
lane bound to the refuting verdict id. Releases cut from the verified frontier only.

**Failure story.**
- *REFUTED-after-land, 3 commits stacked on top*: tree is still deterministically green (always
  is — the floor never moved). Default fix-forward: P0 fix bead + BC-scoped freeze (cap→0 for
  that BC until resolved). If the defect class is contract/security/public-surface: mechanical
  revert, deterministically verified as the inverse patch, bound to the existing REFUTED verdict
  — no second review round-trip; conflicting dependents evicted to fix beads.
- *Revert war* (session A reverts, session B — mid-flight, unaware — re-lands): revert-of-revert
  is an unconditional andon → stop-the-line, human call; agent-mail reservation on the contested
  path; the ledger arbitrates (which verdict is newest, what did it bind).
- *Flaky reviewer* (spurious REFUTED from a degraded warm pawl): repro-required rule — REFUTED
  without a runnable repro downgrades to HOLD and never triggers compensation; warm→cold
  fallback; repeated same-class spurious refutes hit the scope-grind andon.

**Migration path (incremental, each slice reversible by flipping a config/skill line back):**
1. *(observe)* Meter verdict latency + land rate for two weeks (`.1` landed — running now);
   derive N. No behavior change.
2. *(pilot lane)* Flip push-before-pawl for the L0-shaped lane only (docs/provenance diffs —
   already effectively async via the #trivial waiver). Confirms trailing-bind bookkeeping
   end-to-end with zero semantic risk.
3. *(flip the default)* Push-then-pawl for all lands, cap N=small (e.g. 3). The surfaces are
   `pawl-review` + the pre-push gate + AGENTS-WORKFLOW prose — order, not architecture.
   Rollback = flip the order back; nothing structural is destroyed.
4. *(compensation)* Auto-file-on-REFUTED, then the mechanical-revert L0 lane, then the BC freeze
   valve. Each is additive and independently removable.

### Composite B — A + risk-tiered routing (ebec.4) — the demand-side compounding

Everything in A, plus the L0–L3 router at the close door once Directive 17's ruler has two weeks
of meter data (the epic's own gate). The compounding is multiplicative: A removes verification
from the path; B shrinks how much of it exists; the cap admits more λ at the same N because W
fell.

**Failure story (adds to A's):** *misroute* — a small-looking diff that touches `schemas/`
routes L1 instead of L2. Deterministic path×BC rules fail closed on ambiguity, so the miss
requires a rule gap, not a judgment error; discovered post-hoc it is by definition an escape and
compiles into a router rule (EM spine — the bead spec says exactly this). *Goodhart slicing*
(work sliced to duck tiers) is preempted by routing on the bead-arc's union write-scope.

**Migration:** land A slices 1–3 first; router pilots in log-only mode (route computed +
recorded, everything still runs L2) for 20+ closes — this satisfies the bead's done-criteria
data requirement *before* any tier actually cheapens; then enforce tier-by-tier from L0 down.
Reversible: log-only mode is a no-op to remove; enforcement rolls back per-tier.

### Composite C — B + speculative/delta review (ebec.10) — the full pipeline

Everything in B, plus parallel review of pending-window entries (free once A lands: independent
jobs against pinned shas, cap-bounded) and delta re-review packets on REFUTED→fix. This is the
end-state that kills both remaining cost multipliers (serial W, and N-rounds × full-review).

**Failure story (adds to B's):** *interference* — N REFUTED while N+2 (write-intersecting,
already CONFIRMED) sits above it: the invalidation rule re-enters write-intersecting verdicts as
delta reviews; non-intersecting verdicts stand. *Delta blindness* — the fix interacts with the
unchanged remainder: bounded by the spec's own valve (large delta or `--full-review` forces full
packet) and by the attested last-reviewed sha keeping the audit chain honest. *Rebase storm* —
clean rebases ride the existing patch-id REBOUND; content-bearing rebases get delta packets.

**Migration:** only after A+B are steady-state, and `.10` goes through the fresh-mind + council
door the epic already prescribed. Parallel review (the cheap half) can land first — it is pure
scheduling; delta packets (the membrane-core half) land last.

### Recommendation ordering

**A now** (re-sequencing of existing parts; the meter is already running; the pending window is
where escape data — the ADR-0011 revival fuel — comes from). **B when the ruler earns it**
(D17's own gate, already encoded in the bead). **C last, through council** (the epic's own
deferral). Composite A alone converts the door into an instrument and repays its cost on the
first swarm-burst night; B and C are compounding optimizations on top of an invariant that never
moved: **no verdict = not done — at the close, where it always really lived.**
