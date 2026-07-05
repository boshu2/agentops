# The Factory Chain — 10 epics, agile-pivoted

> A chain to *really use* the gas-city factory: to keep building **the factory itself** and to build
> **AgentOps** through it. Filed as `age-gc-factory-chain` (E1–E10). **This is a hypothesis, not a
> waterfall.** Each epic ends at an **evidence gate**; the orchestrator re-plans the *remaining*
> epics from what that gate proves (the [`/rpi` agile re-plan loop](../../architecture/operating-loop.md)
> applied at epic granularity). Only E1–E2 are decomposed now; E3–E10 are markers whose children
> the prior epic's evidence writes.

## The spine

Three movements, and a loop back:

**I · Trust the tool (E1–E3)** — make the factory reliable, installable, and its gate hardened, so
we can lean on it without babysitting.
**II · Use the tool (E4–E7)** — drive real AgentOps work through it, scale it, patch only what
evidence proves load-bearing, and wire it into the learning loop.
**III · Give it away & self-run (E8–E10)** — distribution for the rest of us, cost discipline, and
the factory improving itself — which restarts the chain.

Dual by design: E1·E2·E3·E5·E6·E9·E10 build **the factory**; E4·E7·E8 use it to build **AgentOps /
user value**. The current `age-gc-adoption-u0he` epic is the raw material for E1+E2.

## The chain

### E1 — Graduate the factory to zero-touch
**Goal:** a real quest converges through the membrane with **0 operator nudges** (from the dogfood's
2 and the fitness run's 5). Kill the reconciler/spawn wedge (`age-gc-adoption-u0he.7`) and the
delivery gap; land `install-gc-city.sh` (`.1`) *through the factory* to CONFIRMED.
**Why now:** everything downstream leans on the factory being trustworthy; this is the floor.
**Evidence gate:** the AL.1 quest re-runs and converges, 0 nudges, human merges.
**PIVOT:** spawn fixed by *config* (warm min≥1 pool) → cheap, proceed. Fixed only by a *fork patch* →
**E6 pulls forward** and the fork-maintenance cost becomes a first-class question now, not later.

### E2 — A second operator can run it
**Goal:** someone who isn't Bo stands up a membrane city and drives a quest. Bootstrap + `using-gc`
skill + quickstart (`age-gc-adoption-u0he.1/.2/.3/.4`).
**Why now:** "usable" is false until a second person can do it from cold.
**Evidence gate:** clean machine → one command → working city; an operator following only the skill
drives a quest to CONFIRMED, resolving the known stalls by the documented moves.
**PIVOT:** what breaks for operator #2 re-scopes the docs/skill — and may reveal the factory is
still Bo-shaped, forcing more of E1 before E2 can close.

### E3 — Harden the gate by dogfooding it
**Goal:** run a **batch** of real quests, measure the membrane's escape rate and false-positive rate,
and tighten the close-gate (diff-frame disambiguation, read-only proof, nonce, family-diversity).
**Why now:** the dogfood already surfaced a false-positive (gemini diff-frame) and a real catch — a
batch turns anecdotes into a rate we can drive down.
**Evidence gate:** ≥10 quests run; escape + false-positive rates recorded; every *recurring*
false-positive killed; no CONFIRMED-then-wrong escape survives.
**PIVOT:** the escape corpus says which gate weaknesses are real vs theoretical — and whether the
membrane's quality is high enough that E7 (the corpus flywheel) is even worth betting on
(ADR-0011's data-starvation risk).

### E4 — The factory ships real AgentOps work
**Goal:** drive a slice of the actual AgentOps backlog (real `br ready` beads) through the factory
**instead of** the orchestrator's own subagents. Prove it does product work, not just self-work.
**Why now:** self-improvement that never ships user value is Orchestrator Gravity; this is the
anti-gravity forcing function.
**Evidence gate:** N AgentOps beads shipped via the factory; quality ≥ the subagent baseline (spot
a difference or don't).
**PIVOT:** where the factory *underperforms* my subagents defines its real fit — the routing rule
("which work-types go to the city, which stay in-session"). That rule re-plans E5+.

### E5 — Concurrency: waves through the city
**Goal:** run N disjoint quests in parallel via gc's native drain/fanout; throughput scales past one
quest at a time.
**Why now:** a single-quest factory is a demo; a wave-capable one is a lane.
**Evidence gate:** a wave of ≥3 disjoint quests completes; throughput + collision behavior measured.
**PIVOT:** contention/collision patterns re-plan the wave discipline — and may promote a scheduling
or lease patch into E6.

### E6 — Selective fork patches
**Goal:** patch gc **only** where E1–E5 proved a patch load-bearing (spawn if config failed; a
headless verifier lane if delivery stays a nudge; `source_bead_id`). Stand up the fork capability
(`age-gc-adoption-u0he.8`, LICENSE-gated), patch on cold files (`reviewquorum` = 0 commits/30d).
**Why now:** by here we have evidence for exactly which patches earn their rebase cost — not before.
**Evidence gate:** target nudge count hit; rebase burden measured over one upstream cycle.
**PIVOT:** if the fork treadmill is too costly, retreat to pack/config and accept the residual nudge
tax. **Keep `finalize.jq`** — do not wire the internal Go (weaker + fork-locked; see fork-patch doc).

### E7 — The learning loop
**Goal:** the factory's verdicts, escapes, and provenance feed the AgentOps knowledge corpus; a
caught escape **compiles into a gate check** (the escape-corpus flywheel, e2e).
**Why now:** E3 produced the escape data; this is what makes the membrane *learn from its own
catches*.
**Evidence gate:** ≥1 real escape from E3/E4 becomes a derived, committed gate check that would
catch it next time.
**PIVOT:** this is the real-data test of "does the corpus compound?" (ADR-0011, currently unproven).
The evidence here **keeps or kills** the flywheel bet — a genuine one-way-door for the product thesis.

### E8 — Distribution: fire for the rest of us
**Goal:** package the factory + membrane so an external user runs a city — registry publish,
onboarding, a clean-room "your first city" path (`age-gc-adoption-u0he.5`).
**Why now:** the elixir is the boon brought back; a tool only Bo runs isn't that.
**Evidence gate:** an external or clean-room user stands up and runs a city end-to-end from docs
alone. LICENSE-gated (gastownhall, not ours to republish naively).
**PIVOT:** external adoption friction re-plans packaging — and may reveal the membrane pack needs to
be more self-contained than a fork-dependent build allows.

### E9 — Cost-law + reviewer diversity
**Goal:** meter cost per quest (fix the empty `usage.jsonl`); tune which model families gate which
doors — quorum at one-way doors, cheap single-family at generation.
**Why now:** at scale (E5) and external use (E8), an unmetered factory is a quota risk.
**Evidence gate:** cost-per-quest visible; quorum policy tuned to stakes with the cost data behind it.
**PIVOT:** the cost curve re-plans model routing — and may pull a headless/cheaper verifier lane
back into scope.

### E10 — The self-improving factory
**Goal:** the factory runs idea-wizard **on itself** — proposes and builds its own next improvement
*through its own membrane*, human-merged.
**Why now:** the loop closes — the factory that builds itself, gated by itself.
**Evidence gate:** the factory ships one self-proposed improvement that passes its own membrane.
**PIVOT:** this becomes the steady state. The chain restarts here — E10's output re-seeds E3/E4.

## The pivot contract (how the orchestrator re-plans between epics)

After each epic's gate, before starting the next: read the evidence, then **re-scope the remaining
epics** — reorder, merge, drop, or re-decompose. Specific standing pivots:
- **E1 spawn = fork-patch, not config** → E6 pulls forward to slot 2.
- **E3 escape rate ≈ 0** → E7's flywheel bet weakens (data starvation); consider dropping or
  demoting it.
- **E4 factory underperforms subagents on a work-type** → that type never goes to the city; the
  routing rule narrows the factory's remit (and may shrink E5/E8).
- **E7 corpus doesn't compound** → the flywheel thesis is falsified; E10 becomes "human-curated
  improvement," not auto.
- Any epic that needs a hot-file gc patch (`internal/config`) → stop, cost the rebase treadmill
  explicitly, prefer pack/config.

## Non-goals (the gravity guard, standing)
No rebuilding what gc ships (store/waves/orders/dashboard). No hot-file fork patches without an
evidence gate demanding them. No auto-merge — a human always merges. No epic starts before the prior
epic's gate is green (or deliberately waived, logged). The membrane's *safety* is never traded for
throughput.
