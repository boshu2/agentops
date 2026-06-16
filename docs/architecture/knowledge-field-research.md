# The Tiered Stigmergic Knowledge Field — research synthesis

> Built 2026-06-16 from a 5-scout literature sweep (agentic memory architectures;
> RL/credit-assignment for memory; self-improving/experiential agents; stigmergy +
> decay math; async producer-consumer + control theory). Companion to
> `LOOP-INVENTORY.md`. Purpose: ground the knowledge loop's reinforcement layer —
> the moat — in evidence, not vibes.

## The question (the moat)
Validation proves a trail is *walkable* (the work is correct). It does **not** tell
you which retrieved context *led to food* (caused the good outcome). That second
signal — **credit assignment for knowledge** — is the RL reward problem applied to
memory, and it's the unbuilt core. Capture it and compounding actually compounds.

## The convergence (5 literatures, one equation)
Independently, swarm intelligence, memory science, memory-RL, experiential agents,
and control theory describe the **same machine**: a field of trails, each with a
*strength* that is **reinforced by verified use and evaporates over time**, read by
ephemeral agents, gated so only quality deposits.

**Trail strength — the same formula three times:**
- ACO pheromone: `τ ← (1−ρ)·τ + Δτ`, deposit `Δτ ∝ 1/L_k` (solution quality), winner-only.
- MemoryOS heat (SOTA tiered memory): `Heat = α·N_visit + β·L_interaction + γ·exp(−Δt/μ)`.
- Ebbinghaus/MemoryBank: `R = e^(−t/S)`, and `S += 1` on each recall (use → slower decay).
- Generative Agents retrieval: `recency(exp-decay) · importance · relevance`.

These are one model: **strength = f(verified-use frequency, magnitude, recency-decay).**

## The credit-assignment answer (the central finding)
The memory-RL cluster (**MemRL** 2601.03192; **Memory-R1** 2508.19828; **Memory-R2 /
LoGo-GRPO** 2605.21768; **CriticSearch** 2511.12159; **ProRAG**, **TreeMem**, **PiCA**)
converges hard:

> **Presence ≠ contribution.** Never reinforce a trail because it was retrieved
> *alongside* a win. Reinforce on **marginal/counterfactual contribution** — did the
> outcome differ *with vs. without* this memory, from the *same starting state*.

- **Strongest (rigorous):** Memory-R2's counterfactual re-rollout from an identical
  anchor state isolates each memory's *marginal* effect; explicitly names and solves
  the "useful vs. merely-redundant/popular" trap.
- **Cheapest (runnable now):** CriticSearch's **retrospective hindsight critic** — a
  frozen cross-family judge sees the trajectory **+ the known-good outcome** and scores
  which cited trail was *load-bearing* vs. merely present (~80% human agreement, beats
  outcome-only). This is implementable with our existing cross-family gate.
- **MemRL:** two-phase retrieval (semantic recall → value re-rank); failures decrement
  value, so retrieved-but-unhelpful actively decays.

## Our actual bug: cold-start, not the reward rule
We already have the right *shape* (citations: retrieved/applied/helpful/harmful → EMA
utility → maturity ladder). The audit's 0% cross-session citation is **not an
attribution failure — it's an exploration failure**:
- EMA-on-citations is a **bandit that only learns about arms it pulls.** With 0%
  retrieval, every trail keeps its prior forever. The update rule is fine; **it's
  never invoked.**
- Fixes (RL literature, in order): **(1) break cold-start** — optimistic
  initialization (seed new trails high so the retriever must try them) + an
  **ε-exploration floor** (always sample some weak/untried trails); **(2) wire the
  pull** into decision-point skills so retrieval is non-zero at all; **(3) upgrade
  reward** from co-presence to hindsight/marginal contribution; **(4) add a
  decay/compression penalty** so unhelpful-but-retrieved trails lose strength.

**A silent signal is an exploration failure. Fix the policy (make consume happen)
before tuning the reward.**

## The decay model (settled)
Per-item **exponential** decay (`2^(−Δt/h)`, memoryless, O(1) lazily on read), with a
**half-life `h` that *multiplies* on each verified reinforcement** (SM-2 expanding
intervals / Ebbinghaus S-on-recall). The aggregate field then *automatically* shows
the desirable **power-law** durability (Wixted-Ebbesen; power-law = averaged
exponentials) — proven-durable knowledge becomes near-permanent, transient evaporates
fast — **without hard-coding a tail.** Tier = a half-life band (hot/fast-ρ →
canon/slow-ρ).

## Anti-death-spiral (the pawl is load-bearing)
Stigmergy amplifies whatever is reinforced — including a wrong trail (the **ant mill**:
positive feedback with no correction). LLM agents make it worse: they **hallucinate
trails that don't exist** and would deposit on phantom edges. Four guards (adopt all):
1. **Evaporation** — a wrong trail that stops paying off decays (primary self-correction).
2. **MAX-MIN bounds** `[τ_min, τ_max]` — ceiling caps any trail's dominance (retrieval
   never collapses onto one source); floor keeps every once-verified trail discoverable
   (can always escape a local optimum); reset-to-`τ_max` on detected monoculture.
3. **Quality-gated, winner-only deposit** — **only a gate-passed outcome reinforces a
   trail.** Raw access/citation count is at most a weak prior (`η`), never the deposit
   (`Δτ`). This severs popularity→strength→popularity.
4. **Exploration noise** — occasionally surface a non-top trail; transiently down-weight
   a just-retrieved item so the swarm doesn't pile on.

**This is the Brownian Ratchet, formalized:** the pawl (verification) is the deposit
function; it makes the stock monotone — gold ratchets in, slop can't. The gate is
load-bearing *exactly where the agent is least trustworthy.*

## The substrate (async, append-only, replayable)
- **Decouple produce (harvest) from consume (retrieve)** via a **bounded queue** —
  never couple them at the session boundary (that was the bookend bug). Backpressure
  for free: if validation can't keep up, harvest slows; queue depth is a golden signal.
- **`citations.jsonl` is already CQRS/event-sourcing.** Append-only log = source of
  truth; tiers/weights/decayed-scores are **projections**. Go **Kappa** (one replayable
  stream), so **tuning = drop projection + replay**, not migration. Delivery =
  at-least-once + content-hash idempotency (re-feeding exhaust is a no-op).
  (2026: ESAA puts event-sourcing under agents explicitly — 2602.23193.)
- **Control it like a controller, not a threshold.** PID with **integral** action
  (hold set-points without offset), conservative gain + short delays (projection lag is
  loop delay → oscillation if ignored). **Ashby's requisite variety:** need ≥ as many
  dials as failure modes (stale/poisoned/drifted/mis-tiered) → one global decay can't
  stabilize it; per-tier controls. **Meadows:** the numeric dials (decay, thresholds)
  are *lowest* leverage; invest in pruning-loop strength and the set-point definition.
  **Allostasis** (2508.12791): let set-points *drift with workload*, treat noise as
  signal — heavy-harvest week → tighten proactively.

## Measurement (holdout as the promotion gate, not downstream of it)
Every experiential result (ExpeL's monotonic curve, Voyager's held-out transfer) proves
reuse **only** with-vs-without on tasks the artifact's author never saw — **never
self-report** (matches our own `re-measure-done` / cross-family-gate memories). Our
holdout battery (`s-2026-06-12-*`) is the right instrument but currently runs
*downstream* of promotion; it should **be** the promotion gate: a learning earns its
next tier only when a holdout run *with it beats the run without it.*

## The moat: nobody has built the SHARED, VERIFIED field
Every published memory system (MemGPT, A-MEM, Mem0, MemoryOS, HippoRAG, MemoryBank,
Generative Agents) is **single-agent** — one user, one history. Stigmergy is inherently
**multi-agent** (many depositors, many readers). The open territory, stated by the 2026
surveys as unsolved:
- **a shared multi-agent graded field** (A-MEM's deposit-reshapes-neighbors is the only
  published analog, still single-agent),
- **real eviction/selective-forgetting** (MemoryAgentBench: *no system masters it* — the
  evaporation half is least solved),
- **governance against reinforcement amplifying errors/poisoning** (SSGM, 2603.11768).

Fuse them — **CoALA typed nodes + MemoryOS heat/decay + HippoRAG associative read +
multi-agent verified deposit (the pawl) + real eviction** — and that is *exactly* the
tiered stigmergic knowledge field. Bo's instinct is at the research frontier; the
**verified-deposit + multi-agent + real-evaporation** combination is unbuilt, and the
verification gate (which we already have and others lack) is the differentiator.

## Proven vs. open
- **Proven:** hot/warm/cold paging; recency·importance·relevance ranking; Ebbinghaus
  decay + recall-reinforcement; MemoryOS heat-tiering at SOTA; PageRank associative
  retrieval; LLM reflection raw→structured; counterfactual/hindsight credit assignment.
- **Open (our claim space):** shared multi-agent field; real eviction; verified-deposit
  gating at scale; holdout-as-promotion-gate; the controller/allostatic tuning layer.

## The decision (build order)
1. **Break cold-start + wire the pull** — optimistic init + ε-floor + mandatory
   decision-point retrieval (`ao lookup --gold` in discovery/plan/pre-mortem). *Without
   this nothing else matters; it's the silent-signal fix.*
2. **Emit tiers, don't gate to binary** — gold carries a strength scalar (MemoryOS heat
   form) + tier band; retrieval ranks by strength×relevance; lazy exponential decay with
   multiply-on-verified-use.
3. **Upgrade the reward to hindsight contribution** — CriticSearch-style cross-family
   retrospective judge converts a citation into a marginal-contribution score; only
   gate-passed outcomes deposit.
4. **Holdout becomes the promotion gate** — with-vs-without on unseen scenarios.
5. **Make the substrate a controller** — Kappa replay over the append-only ledger;
   per-tier dials; measure golden signals; tune (later, allostatic).
The reward/maturity machinery already exists — step 1 lights it up; steps 2–4 make the
light mean *useful*; step 5 makes it self-tuning.

## Citations
Memory-RL: MemRL [2601.03192], Memory-R1 [2508.19828], Memory-R2 [2605.21768],
CriticSearch [2511.12159], ProRAG [2601.21912], TreeMem [2605.04811], PiCA [2605.09287].
Architectures: MemGPT [2310.08560], Generative Agents [2304.03442], CoALA [2309.02427],
MemoryBank [2305.10250], A-MEM [2502.12110], Mem0 [2504.19413], HippoRAG [2405.14831],
MemoryOS [2506.06326], MemOS [2507.03724]. Experiential: Reflexion [2303.11366],
Voyager [2305.16291], ExpeL [2308.10144], CBR-for-agents [2504.06943], Compound
Engineering (Every). Stigmergy/decay: ACO + MAX-MIN (Stützle), Ant mill [1703.06859],
Duolingo HLR (Settles ACL'16), Wixted-Ebbesen power law (1997). Substrate/control:
ESAA [2602.23193], allostasis [2508.12791], Ashby requisite variety, Meadows leverage
points, PID (Åström). Governance: SSGM [2603.11768].
