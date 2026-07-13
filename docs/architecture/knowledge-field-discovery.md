# Discovery — the tiered stigmergic knowledge field (the reinforcement layer)

> Discovery packet. Research basis: `KNOWLEDGE-FIELD-RESEARCH.md` (5-scout sweep) +
> `LOOP-INVENTORY.md`. This shapes intent as BDD and slices it into a crank-ready
> DAG with acceptance examples. Guarded by a fresh-context premortem (below).
> Complexity: full (foundational). Lane: knowledge/corpus (carved to this lane by
> the active single-agent doctrine lane — no collision).

## intent
Make the agent system an **organism whose memory lives in the environment**: capture
the exhaust of agent work, **reinforce the trails that demonstrably caused verified
good outcomes** (strength↑), **evaporate** unused trails, and **pull the strongest
trails into the next agent's context at decision points** — so context *compounds*
across sessions instead of evaporating at session end. Built on what we already have
(`citations.jsonl` ledger, `ao wiki gold`, `ao lookup --gold`, the cross-family gate,
the holdout battery), not a greenfield store.

**BDD — the load-bearing acceptance examples (the contract):**
- **Closing the loop:** *Given* a learning that was retrieved and cited in a
  gate-passed session, *When* `ao wiki gold` next runs, *Then* that learning's
  strength/tier increases and it ranks higher in the next retrieval.
- **Cold-start broken:** *Given* a corpus where retrieval rate is 0%, *When* a
  session runs discovery/plan/premortem, *Then* relevant gold is retrieved
  (retrieval rate > 0) and a citation is recorded.
- **Credit, not co-presence:** *Given* two trails both present in a winning session —
  one load-bearing, one merely co-retrieved — *When* credit is assigned, *Then* only
  the load-bearing trail gains strength.
- **Compounding, measured:** *Given* a held-out scenario, *When* it is run with vs.
  without the promoted knowledge, *Then* with-knowledge outperforms — and *only then*
  does the knowledge earn its next tier.
- **No death-spiral:** *Given* a popular-but-unverified (or hallucinated) trail, *When*
  it is retrieved repeatedly without a gate-passed outcome, *Then* its strength does
  NOT increase (deposit requires the pawl).

## boundary
- **In scope (the build):** the produce→consume→reward cycle made real on the existing
  ledger; tiered strength (not binary); cold-start fix; hindsight credit assignment;
  holdout-as-promotion-gate; Kappa projection substrate.
- **Non-goals (explicit):** (1) NOT the full *shared multi-agent* field + real eviction
  in this epic — that's the moat endgame (S6), a separate epic once the single-agent
  loop is proven. (2) NOT replacing the reward/maturity machinery — it exists; we feed
  it. (3) NOT a new datastore — `citations.jsonl` + projections (Kappa). (4) NOT
  bookend hooks — produce is async/decoupled, consume is decision-point pull.
- **Write scope:** agentops `cli/internal/{wiki,search,lifecycle}`, the decision-point
  skills (discovery/plan/premortem), `.ao/wiki` projection. **Lane:** knowledge/corpus
  (mine). **Door 9:** no `claude -p`; cross-family critic uses `codex exec`/AGY.
- **Coordination:** the field lives in agentops; the active lane owns orchestration/
  single-agent + the bead DB. Operationalize into beads *in agentops*, coordinated, not
  in the contended mt-olympus tracker.

## the slice DAG (vertical slices, each with acceptance)
Ordered by the research build-order. **S1 is the MVP that proves the loop closes;**
the rest deepen it. Each slice ships behind a flag, additive, gates-green.

**S0 — Author knowledge-field holdout scenarios (NEW — premortem caught this).**
- The promotion gate S4 names **does not exist**: the current holdout battery is 4
  scenarios about BDD-foundry/planning quality, in mt-olympus, not agentops. Author
  *knowledge-reuse* holdout scenarios in agentops: with-vs-without-knowledge tasks the
  author never sees. Without this, S3 has nothing to validate "load-bearing vs present"
  against and S4 is unbuildable.
- *Acceptance:* ≥N agentops holdout scenarios that each run with-knowledge and
  without-knowledge and score the outcome delta; isolated so implementers never see them.

**S1 — Break cold-start + wire the pull (the unlock) — BOUNDED against ADR-0002.**
- Make decision-point retrieval real: discovery + plan + premortem call `ao lookup
  --gold`. (Correction: today's retrieve is **fail-open best-effort** (`2>/dev/null ||
  true`) **and hits the raw `.agents/` corpus, not `--gold`** — so this is net-new, not
  "extend the pattern.") Retriever gets **optimistic init** + **ε-floor**, written as
  **projection parameters (S5 Kappa), not hard-coded** (so S6 retunes by replay).
- **The bound is the point** (S1 risks re-triggering the bookend spray — delta=0 @
  10.35M tokens): hard cap **top-K ≤ 3, byte-bounded**; inject **pointers/citations, not
  full bodies** (make dag.md's "pointers not full context" binding); **rebuild `ao`
  preflight** (the resident `cli/ao` predates `--gold`).
- **Lock the deposit chokepoint now:** a single `deposit(trail, gate_verdict)` function
  that **refuses without a gate-verdict** (no-op body in S1) — so the pawl is structural,
  not a later retrofit around an LLM judge.
- *Acceptance (the discriminating A/B):* retrieval rate 0→>0 **AND** a retrieval-on
  vs retrieval-off A/B shows **non-zero outcome delta at bounded token cost** — the same
  A/B that caught delta=0 in ADR-0002. **If S1 can't beat delta=0 under the token cap, it
  IS the spray returning — fail loudly here, not at S5.**

**S2 — Emit tiers, not binary (graded strength + decay).**
- `ao wiki gold` emits a **heat scalar** (`α·N_visit + β·magnitude + γ·exp(−Δt/μ)`,
  MemoryOS form) + a tier band; retrieval ranks by strength×relevance; decay computed
  **lazily on read** (`2^(−Δt/h)`); **verified reuse multiplies `h`** (→ power-law durability).
- *Reality (premortem):* gold today is **thin + flat** — ~20 learnings/3 patterns/59
  findings, **every doc Utility 0.35 (the default prior)**; reward has never propagated
  into gold. So S2's "verified citation raises half-life" needs **S1 to have driven ≥1
  non-default utility into gold first** (explicit DAG edge S1→S2).
- *Acceptance:* a gold doc carries heat+tier+half-life; after a verified citation its
  half-life rises above the flat prior; a stale uncited doc's effective score decays on read.

**S3 — Hindsight credit assignment (the moat mechanism) — DE-RISKED, gated behind S2.**
- On a gate-passed outcome, a **CriticSearch-style cross-family retrospective critic**
  (`codex exec`/AGY) scores which cited trail was *load-bearing* vs. *merely present*;
  the marginal score feeds the **S1 deposit chokepoint** (already gate-locked).
- *Cost de-risk (premortem):* the critic per gate-pass is a real quota/latency sink and
  its ~80% agreement is benchmark-not-our-trajectories. So: **run it on a sampled
  subset**, and **prove on the S0 holdout that hindsight-credit beats the cheap
  co-presence prior (equal credit to all cited trails) BEFORE paying for it fleet-wide.**
  If it doesn't beat the prior, S3 isn't worth its token bill.
- *Deliverable:* the "credit, not co-presence" BDD example needs a **fixture** — a
  constructed trajectory with one load-bearing + one co-present trail and a known answer
  (name it in S3).
- *Acceptance:* the "credit, not co-presence" + "no death-spiral" BDD examples pass on
  the fixture; hindsight beats the co-presence prior on S0 holdout.

**S4 — Holdout becomes the promotion gate. (Depends on S0 — the gate doesn't exist yet.)**
- A learning earns its next tier **only** when an **S0 knowledge-reuse holdout** run
  *with* it beats the run *without* it. Holdout moves from downstream-of-promotion to
  *being* the gate. Guard: don't let gate-use contaminate the holdout (rotate/burn-track
  scenarios, as the existing holdout battery already does).
- *Acceptance:* the "compounding, measured" BDD example passes; a learning that doesn't
  improve the S0 holdout does not promote.

**S5 — Substrate as controller (Kappa + dials).**
- Treat `citations.jsonl` as the append-only event log; tiers/weights/scores are
  **projections** (drop + replay to retune); decouple harvest (producer) from retrieve
  (consumer) via a bounded queue; per-tier dials; golden signals observable.
- *Acceptance:* change a decay rate → drop projection → replay → new field, no
  migration; queue depth/retrieval-hit-rate/staleness are reported as signals.

**S6 — (separate epic, the endgame/moat):** shared multi-agent field + real eviction +
governance against poisoning. Unbuilt territory per the 2026 surveys; gated on S1–S5.

## evidence
The 5 BDD acceptance examples above are the executable contract. Research + proven-vs-
open + formulas + citations: `KNOWLEDGE-FIELD-RESEARCH.md`. Existing instruments to
reuse: `ao lookup --gold` (shipped), `citations.jsonl`/`feedback.jsonl`,
`ratchet/maturity.go`, the cross-family gate, the holdout battery (`s-2026-06-12-*`).

## decision
Slice S1-first because the failure is **cold-start, not the reward rule** — and the
premortem sharpened *what* the cold-start is: retrieval IS happening (3,739 citations /
209 sessions, ongoing) but **against the raw `.agents/` corpus, not gold, and reward
never flows back into gold** (every gold doc sits at the 0.35 default prior). So S1's
real job is narrower and clearer: route the pull at *gold* and let reward reach gold.
It's the cheapest unlock and makes every later slice measurable (no credit to trails
never pulled). Single-agent loop first, multi-agent moat (S6) after — de-risks the
unbuilt frontier behind a working baseline; S1's exploration params are written as
projection params so S6 retunes by replay, not rewrite.

## constraint
Door 9 (no `claude -p`; critic via `codex exec`/AGY). Additive + flag-gated; full
package suite + pre-push gate green before each merge (per the push-skill guard). The
**pawl is load-bearing**: nothing deposits except a gate-passed outcome — without it
this becomes a confident-slop amplifier (the ant-mill / LLM-phantom-trail risk). No
bookend hooks (the delta=0 / 10.35M-token lesson).

## next_action
On PASS-WITH-CHANGES (applied) → operationalize **S0 + S1** into agentops `br` beads
(coordinated with the active lane; NOT the contended mt-olympus tracker) and
`/rpi --from=implementation`. S2–S5 follow; S6 is a separate epic.

## Premortem verdict (fresh-context, 2026-06-16): PASS-WITH-CHANGES — all applied
Re-baseline (verified against agentops, not assumed):
- `ao lookup --gold`: **CONFIRMED** (commit a04b6d51f) — but the resident `cli/ao`
  binary predates the flag → **rebuild preflight required** (folded into S1).
- reward→maturity machinery (`feedback.go`, `ratchet/maturity.go`): **CONFIRMED, complete.**
- gold corpus: **CONFIRMED but thin + flat** (~20/3/59 docs, all Utility 0.35 default —
  reward never propagated to gold) → S1→S2 DAG edge added.
- "premortem already does mandatory retrieval": **OVERSTATED** — it's fail-open
  best-effort and hits raw `.agents/`, not `--gold` → S1 corrected to net-new.
- holdout battery as the promotion gate: **WRONG for this purpose** (4 scenarios, about
  planning not knowledge-reuse, in mt-olympus not agentops) → **S0 added** to author
  knowledge-reuse holdout scenarios in agentops; S4 now depends on it.
- ADR-0002 (delta=0 @ 10.35M) + flywheel-audit (0% cross-session): **CONFIRMED.**

Changes applied: (1) S0 holdout scenarios added (the missing gate); (2) S1 bounded
against ADR-0002 (top-K≤3, pointers-not-bodies, token-budget A/B as acceptance, rebuild
preflight); (3) deposit chokepoint locked in S1, S3 de-risked (sampled critic, prove-vs-
co-presence-prior on S0 holdout first, fixture named); (4) S1 exploration params written
as projection params so S6 retunes by replay. Open residual: S3 critic cost is the one
unproven risk — its acceptance is explicitly "beat the cheap prior or don't ship it."
