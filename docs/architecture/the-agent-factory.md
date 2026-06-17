# The Agent Factory — a Kubernetes-style control plane for stochastic workloads

> **Store binding corrected (2026-06-17).** This spine originally named the state store / etcd-analog as
> **bd/Dolt** — that is **retired**. The etcd-analog is now **two ledgers** that together play etcd: the
> **bead ledger** (`br`, git-JSONL — work / desired state; `BEADS_DIR=$PWD/_beads br …`, synced via
> `git -C _beads push`) and the **proof / verdict ledger** (`docs/provenance/ledger.jsonl` + yield
> `gate-verdict` events — admission state). The control-plane *shape* (acceptance store → scheduler →
> reconcile → membrane), the cost law, and the role/primitive model still stand — only the concrete store
> binding changed. See the **2026-06-17 delta** at the foot of this doc and AGENTS.md / CLAUDE.md for
> current tracker truth.

> **Status: cross-model reviewed (final whole-spine pass, 2026-06-05), pending operator ratification.**
> The architecture spine (`ag-yv25`, subsumes "canonize the membrane"). Reviewed *through the membrane*
> by a mixed council — Claude Opus 4.8 + Codex gpt-5.5 — across three passes: the architecture, the
> unit-of-compute debate (→ AgentPod), and this final whole-spine pass over the integrated Pod model +
> Roles × Primitives (Opus PASS + gpt-5.5 WARN → all agreed edits applied → both endorse canonization).
> Records in `.agents/council/2026-06-05-*`. Per `liveness.SignificantActionDocCanonization`, canonizing
> this very doc is itself a membrane-gated action. The single place that says how every AgentOps piece
> composes into one factory.

> **The 2026-06-17 delta is a dated addition, NOT part of the 2026-06-05 ratification lineage.** The
> adapter taxonomy, the agent two-altitude reconciliation, and the store correction were added under a
> separate `/discovery --mixed` cross-family pass (WARN → PASS-with-hardening; Claude + Codex converged).
> Plan + verdict: [`.agents/plans/2026-06-17-control-plane-primitives-unification.md`]. Editing this doc
> did **not** re-open the 2026-06-05 whole-spine ratification — that verdict stands on the spine as
> reviewed then; the delta carries its own gate.

## Thesis

Kubernetes solved how to run **declared workloads** reliably at scale: declare desired state,
let a control plane reconcile reality to it, reschedule failures, and never trust the current
state — diff against desired, converge or reject. But k8s never **judges the application
semantics** of what a workload produces — a container's output is the workload's own problem.
AI agents are the first workload whose **output itself is stochastic and cannot be trusted on
sight**: the same prompt does not yield the same result, and a confident answer can be wrong.

The agent factory borrows Kubernetes' entire control-plane **shape** and adds the one thing
k8s never needed — a **verification membrane** that admits nothing until an independent,
cross-model quorum **verifies it to an acceptance threshold** against an explicit contract.
Not *proof* — stochastic output cannot be proven — but bounded, evidence-backed assurance.
**Where k8s *runs* the workloads you hand it, the factory *generates* the work and *gates* it.**

## Control plane / data plane

```
                    ┌────────────────────── CONTROL PLANE (durable, HA) ──────────────────────┐
   goal +           │  acceptance store  │  scheduler  │  reconcile controllers │  MEMBRANE    │
   acceptance ─────▶│  (br + proof       │  (place by  │  (rpi inner / evolve   │  (admission: │
   contract         │   ledger = etcd)   │  capability,│  outer; declarative    │  cross-model │
                    │  quorum, not one   │  cost, model│  desired-state loop)   │  quorum gate)│
                    │  host              │             │                        │              │
                    └─────────┬──────────────────┬───────────────┬──────────────────┬──────────┘
                              │ schedule          │ probe          │ reschedule       │ admit / reject
                    ┌─────────▼───────────────────▼────────────────▼──────────────────▼──────────┐
                    │  DATA PLANE (fungible, disposable): agents on interchangeable compute        │
                    │  Claude · Codex · Gemini  ×  Mac · bushido · cloud   (heterogeneous by design)│
                    └────────────────────────────────────────────────────────────────────────────┘
                                            │ exhaust (every turn)
                                            ▼
                       COMPOUNDING CORPUS (.agents/) — state that gets smarter (no k8s analog)
```

- **Control plane** — durable, HA, must survive any single node: the acceptance/state store, the
  scheduler, the reconcile controllers, and the membrane. **Quorum, not one host; cross-model,
  not one model.** A quorum of one is not a quorum, at either altitude.
- **Data plane** — fungible, disposable: the stochastic agent-workers, scheduled across
  interchangeable compute, probed, and rescheduled on failure.

## The unit of work and the unit of compute

Two different units — and we already have the first one.

**The unit of WORK = a bead.** A bead is a declarative workload object: intent (title/description) +
acceptance contract (`.feature`/scenarios) + dependencies (`br dep`, the DAG) + status + priority +
provenance — stored in the bead ledger (`br`, git-JSONL — see the store correction in the banner). *We already have the object model.* So:

| k8s | factory | |
|---|---|---|
| Manifest / object | **a bead** | the declarative unit of work + its acceptance contract |
| Deployment | **an epic** (a bead with children) | desired state for a body of work; decomposes into child beads |
| the leaf workload | **a quest-sized bead** | the unit one Pod drives to completion |
| scheduler queue | **`br ready`** | beads whose dependencies are satisfied = schedulable now |
| Pending→Running→Succeeded | **`open → in_progress → closed`** | the workload lifecycle; `blocked` = unschedulable |
| reconciliation DAG | **the bead dependency graph** | what unblocks what |

**The unit of COMPUTE = an AgentPod (= one agent).** A cross-model debate (Opus 4.8 + gpt-5.5,
2026-06-05) converged here, correcting an earlier "Pod = swarm" draft. The schedulable unit is the
*smallest thing with independent fate, fresh context, lifecycle, and schedulability* — that is **one
agent**, not the swarm. The swarm is a **Job**. The worktree is a **Volume** (a separate axis), not
the Pod boundary.

| k8s | factory | what it is |
|---|---|---|
| Node | a **compute host** (Mac, bushido, cloud) | a capacity pool: which model CLIs / provider lanes are authed, token/rate budget, tmux capacity |
| **Pod (AgentPod)** | **one agent run** | fresh context, own model process + optional sidecars, own lifecycle/fate, schedulable onto any Node |
| Container | the **model process** (+ sidecars) | the claude/codex session; tool-server / MCP-helper sidecars |
| **Volume** | the **worktree / branch** | *separate axis, not the Pod boundary*: default = shared clone + own branch (Tier 1); escalate to an ephemeral worktree (Tier 2) on a conflict signal |
| Deployment / Job | a **swarm (SwarmJob)** | a controller-managed set of AgentPods for one bead/epic/wave, coordinating via Agent Mail (network) + the bead ledger (ownership), with placement policy |
| kubelet / runtime | **NTM** | spawns + manages AgentPods (panes) on a host |
| Pod network / IPC | **Agent Mail** | how AgentPods coordinate, lock, message |
| controller | **rpi / evolve** | drives the SwarmJob to completion; restart/reschedule failed AgentPods |

**The membrane falls out structurally:** an output is admitted only when ≥2 judge AgentPods of
**different model families** agree (`liveness.CheckSignificantAction`, enforced). This separates
author/judge identity and model-family by topology — judges are never co-resident with the author —
but it does **not** make failure domains fully independent: correlated semantic error remains
(boundary 6). *Model-family* anti-affinity is enforced today; *node* anti-affinity lands with the
scheduler (step 5, `ag-xanm`).

**The one exception** (both debaters flagged): genuine gang-scheduled, co-located collaboration on one
mutable worktree is a multi-container Pod — a **CollaborationSession** — explicitly marked *not*
membrane-independent; external judge AgentPods are required before admission.

**They meet at the scheduler:** it pulls a ready bead off `br ready`; the SwarmJob schedules AgentPods
(one per agent) onto Nodes; each drives its share; the membrane admits via independent judge AgentPods;
the bead closes. So one-way door #2 (the workload-object API) is **mostly already built — it is the bead
schema.** Net-new on that door: (a) an **AgentPod-spec + SwarmJob-spec on the bead** (model, sidecars,
volume tier, agent count, anti-affinity — scaled to stakes per the cost law), and (b) a **watch/event
surface** so controllers react to bead-state changes instead of polling.

> **The cost law (core — the same axis as the doors).** Pod composition scales to the **stakes**,
> because tokens follow **reversibility**: spend the quorum and strong models on **one-way doors and
> admission gates** (where agreement is load-bearing); run **cheap single-model with good context** on
> **two-way doors and generation** (where work is cheap and reversible). *Multi-model is for agreement,
> not for doing.* A high-stakes / one-way-door bead gets a rich SwarmJob (≥2 independent AgentPods,
> best models, real quorum); a routine bead gets a minimal one (one cheap Generator + one cheap
> Oracle). **The floor:** a single Oracle is a cheap pre-check, *not a quorum* — it cannot admit
> one-way-door / admission work, which always requires ≥2 anti-affine Oracles. The same reversibility
> axis governs both *which door* (decide slow vs. fast) and *how much compute* (rich vs. cheap SwarmJob).

## Roles × primitives — who does what, bound to how it runs

The combination of three orthogonal axes — the way k8s binds a ServiceAccount's Role to a Pod, never
merging the two vocabularies:
- **RBAC role** (`roles.go`, enforced) — *authority*: which verbs an actor may perform. Out-of-capability
  → admission; `author == judge` → hard deny. The mechanical, precise layer.
- **Function** — *what stage of work*, in canon-legible terms (the dark-factory **planner-generator-
  evaluator** pattern) — with our own name where it carries meaning.
- **Primitive** — *how it is scheduled* (the compute model above).

| Function | RBAC role (`roles.go`) | Verbs | Primitive | What it is |
|---|---|---|---|---|
| **Planner** | Worker | edit | AgentPod | shapes intent → beads + acceptance contracts |
| **Generator** | Worker | edit | AgentPod (Volume = branch/worktree) | writes the work — the author |
| **Oracle** *(canon: Validator)* | Verifier | judge | AgentPod, **anti-affine** to the Generator | the membrane's judge — rules on **semantic** correctness against the contract; **never edits what it judges** (no-self-grade, RBAC-enforced) |
| **Orchestrator** | Orchestrator | route, vote, shepherd, synthesize | **controller** (control-plane) | routes beads to SwarmJobs, synthesizes at gates; **cannot edit** (the prompt-injection defense) |
| **Scribe** | Scribe | record | sidecar Container | writes provenance to the proof/verdict ledger; never decides |
| **Heartbeat** | Heartbeat | nudge | liveness probe / CronJob | only nudges; the health checker |

**The Oracle is ours.** The canon calls this role "Validator" (a test-runner / internal reviewer); we
call it the **Oracle** — interpretation, pattern, the seeing of truth — because its job is not just to
run tests but to **judge whether stochastic output is actually right** (the semantic oracle the whole
factory is built around). "Validator" is the legible synonym; "Oracle" is the name. It is deliberately
*our* role concept, not Oracle-the-vendor's product.

**The composition:** a SwarmJob for a bead = a Generator AgentPod (+ a Planner upstream) + **≥2 anti-affine
Oracle AgentPods** + a Scribe sidecar, driven by an Orchestrator controller, nudged by a Heartbeat.
*Which* functions and *how many* = the SwarmJob template, scaled to the bead's stakes (the cost law).

**Why a dual vocabulary, not a rename:** `roles.go` stays the precise, enforced **authority** layer
(no enum churn); the function names are the legible **product** layer — the same operator-side /
product-side split as jargon-translator. We speak the canon (Planner / Generator / Validator /
Orchestrator) so we are legible to the category; we keep our enforced RBAC kernel; and we plant our own
flag on the one role that *is* the moat — the **Oracle**.

*Two clarifications the council required:* (a) **Planner and Generator are distinct *functions* but
share the *Worker* authority** — RBAC is deliberately coarser than the function vocabulary; the
function layer carries the distinction the kernel does not need to. (b) The Orchestrator's `vote` verb
is consensus *synthesis / tally*, **never a membrane judge-verdict** — only Oracles judge. Enforcement
is real and dual: the `liveness` kernel (`guards.Disjoint` — `author == judge` → Denied;
`quorum.CheckSignificantAction` — ≥2 agents across ≥2 model families) plus the `ao turn verify` path; a
`--allow-self` request is a recorded **waiver**, not admission.

**The differentiator, in the field's own words:** the dark-factory canon is planner-generator-evaluator
(GAN-shaped — one builds, one critiques), but its evaluator is hand-waved: same system, often the same
model. Stanford asks the open question — *"Built by Agents, Tested by Agents, Trusted by Whom?"* The
**Oracle is the answer**: cross-model, author≠judge, structurally independent (anti-affine AgentPods).
We run the canon pattern; the Oracle is the part the canon cannot yet do.

## The mapping — what we copy from k8s

| Kubernetes | Agent factory | AgentOps component | Status |
|---|---|---|---|
| Declarative manifest (desired state) | goal + **acceptance contract** | `.feature`/Gherkin + bead acceptance | **built** |
| Reconcile loop (controller) | reconcile-by-rejection loop | `rpi` (inner) / `evolve` (outer) | **built** |
| kubelet / pod spawn | dispatch agents in fresh-context worktrees | `crank` / `swarm` (in-session); **NTM** swarm panes (out-of-session) | **built** |
| etcd (quorum state) | acceptance + provenance store | **two ledgers**: bead ledger (`br`, git-JSONL — work state) + proof/verdict ledger (`docs/provenance/ledger.jsonl` + yield `gate-verdict` — admission state) | **git-JSONL + ledger** (single-host Dolt HA superseded; `ag-o2tc` context) |
| API server + object model + watch/events | declared workload objects + the change stream controllers act on | — (the beads ledger is the store; the object/event surface is implicit) | **to build** |
| Scheduler (resource fit) | place agent-tasks by capability/cost/model-fit | — (cron heartbeat only) | **to build** (`ag-xanm`) |
| Node pool / fungible nodes | fungible compute, heterogeneous by design | the fleet (Mac, bushido, cloud) | **partial** (`ag-xanm`) |
| Liveness / readiness probe (runtime health) | is the agent alive / not stuck-looping? | `rpi` stall-detect (partial); no reschedule | **partial** |
| Admission controller / validating webhook | gate every output before it lands — *is it correct?* | the membrane: author≠judge `liveness` kernel (`#737`) + CI convergence gate (`#746`) | **built at gate sites; universal path to build** |
| Self-healing reschedule | node dies → work moves to live compute | — (the crash proved we lack it) | **to build** |
| RBAC | author ≠ judge; role-capability matrix | `cli/internal/liveness/roles.go` (#733) | **built** |
| Namespaces | isolated context per worker | worktrees + per-worker context | **built** |
| Immutable/versioned config | version-locked binaries across nodes | `fleet-versions.env` + version-lockstep tooling | **built** (today) |
| Out-of-session swarm runtime (controller-manager + node agent) | persistent agent panes, robot API, pipelines, serve | **NTM** (native tmux) | **built** |
| Coordination (leases / events) | file locks, messaging, inboxes, conflict-prevention | **MCP Agent Mail** | **built** |
| kubectl (operator surface) | where-is-everything / what's-stuck | NTM robot API (partial) | **to build** (`ag-7dnb`) |

> **Substrate decision (load-bearing):** the orchestration substrate is **NTM** (native tmux
> swarms) **+ MCP Agent Mail** (file locks, messaging, inboxes) — **not Gas City.** The gastown
> SDK was deliberately **not adopted**: NTM's swarm runtime + Agent Mail's coordination cover
> dispatch and conflict-prevention without the SDK dependency. Do not re-introduce `gc` as the
> substrate in this architecture.

## The novel primitive — the verification membrane

This is the part you invent, not copy, because k8s never faced it. k8s **trusts** its workloads —
a deterministic container does not lie. An agent can produce plausible, confident, wrong output.
So the factory needs a **semantic oracle**: a way to judge stochastic output. The membrane is it.

- **Reconcile-by-rejection, not convergence.** k8s converges a Deployment to N replicas by
  construction. A stochastic generator cannot be driven to spec — the loop can only **reject or
  retry**. The control loop is a rejection loop.
- **The readiness probe is the membrane.** "Is it running" is not enough; "is the output correct,
  proven against the contract, by a verifier that is **never the author and never the same model**."
- **The quorum is raft-*shaped*, not raft.** etcd uses raft quorum to keep control-plane state
  consistent (a majority of independent replicas agree before a write commits). The no-self-grade
  membrane borrows the *shape*: a majority of independent **models** agree before a verdict commits
  (`ag-xdrw`'s orchestration tier — ≥2-of-3 cross-model ACK before a significant decision lands).
  But the voters **evaluate** rather than replicate, and model failures can be **correlated** — so
  it gives probabilistic semantic admission, not deterministic consensus safety. Closer to
  N-version programming than to raft.
- **The moat is structural.** Credible verification *requires* cross-vendor neutrality — a model
  vendor cannot grade its own model's output without committing the self-grade the membrane
  forbids. Neutrality is a precondition, not a feature.

## Where the analogy breaks (the honest boundaries)

1. **Stochastic + untrusted, not deterministic + trusted.** The root difference; everything above
   follows from it. Don't say "guarantee" — the factory *admits proven, rejects unproven*.
2. **Model-diversity is required, not incidental.** The quorum *needs* ≠ models. The fleet is
   heterogeneous node-pools by design (Claude/Codex/Gemini ≈ GPU/CPU/TPU pools), not uniform nodes.
3. **State compounds.** k8s has no "the longer it runs, the smarter it gets" property. The corpus
   (`.agents/`) is the factory's exhaust becoming next session's intake — measured, not asserted.
4. **Resource contracts are fuzzy.** A pod declares CPU/mem and bin-packs deterministically. An
   agent-task's cost (tokens, retries, wall-clock) is unpredictable — scheduling is probabilistic.
5. **"Done" is not binary.** A pod is running or not; an agent's output can be subtly wrong. The
   membrane replaces the binary liveness check with graduated, evidence-backed assurance.
6. **The judges are not independent failure domains.** Cross-model quorum *reduces* but does not
   eliminate correlated error — models trained on overlapping data share blind spots, so a
   confidently-wrong output can be confidently *ratified*. The **contract** (`.feature`/acceptance),
   not the judges' agreement, is ground truth; the quorum gates *against the contract*, and
   unanimous-but-wrong is a known residual risk, not a solved one. *(Demonstrated live: the
   cross-model council that reviewed this very doc had a judge confidently flag a "phantom" file
   that in fact exists — only checking the code refuted it.)*

## Build sequence (epic-of-epics, foundation-up)

Don't build the kubelet before etcd is HA. Order:

State and admission come before the scheduler — you can't schedule declared workload objects
that don't exist yet, and you must not let unproven work land before the gate is universal.

0. **IaC control-plane provisioning + version lockstep** — `fleet-versions.env`, `dolt-control-plane`, `ensure-dolt-version`. **done 2026-06-05.**
1. **HA quorum state store** (`ag-o2tc`) — Dolt replication + failover; the control plane survives any single host. *in progress.*
2. **Control-plane API + object model + watch/events** — declared workload objects (goal + acceptance contract) and the change stream controllers act on. The contract layer everything else schedules and reconciles against. **(One-way door #2 — must be council-designed before any controller, scheduler, or agent codes against it.)**
3. **Declarative workload + reconcile controller** — formalize the *already-built* `rpi`/`evolve` loop as a controller over those objects: a goal + acceptance contract → dispatch until admitted-or-rejected.
4. **Membrane as the universal admission path** (`ag-xdrw`) — the author≠judge gate is built at specific sites (`#737`/`#746`); make it the *standing* gate every workload output passes before it lands.
5. **Fungible compute + scheduler** (`ag-xanm`) — schedule declared workload objects onto live compute by capability/cost; the node-pool abstraction.
6. **Self-healing** — liveness probes + restart policy: a dead/stalled agent reschedules onto live compute (the crash gap).
7. **Operator surface** (`ag-7dnb`) — the factory's `kubectl`: where-am-I-needed, what's-stuck, what's-pending.

Each step is a closable epic. The membrane (step 4) is the most built; the **API/object model
(step 2), scheduler (step 5), and self-healing (step 6)** are the largest net-new — and the
scheduler + self-healing are exactly what the 2026-06-05 crash proved we lack.

## One-way doors vs two-way doors

The build is **BDD + agile** — ship slices, pivot freely. But not every decision is reversible.
Bezos's frame: **one-way doors** (irreversible, or expensive to reverse) get decided slowly and
deliberately, up front; **two-way doors** (reversible) get decided fast and pivoted. The
architecture's job is to **put the one-way doors in the *ports* and the two-way doors in the
*adapters*** — get the contracts right; stay agile on the mechanisms. (This is just hexagonal
architecture read through reversibility: a port is a contract others depend on; an adapter is a
swappable mechanism behind it.)

**One-way doors — decide up front, hard to reverse (the ports / contracts):**

| # | Decision | Why irreversible | Status |
|---|---|---|---|
| 1 | The membrane is the central primitive; no-self-grade is structural law | the whole product identity + every gate couples to it | **decided** (duel + council + proven on `main`) |
| 2 | The **workload-object schema** — declared goal + acceptance contract, the factory's API | k8s's hardest-to-change thing is its API; every controller/scheduler/agent codes against it | **OPEN — design before build step 2** |
| 3 | Acceptance-contract language = Gherkin/`.feature` + bead acceptance | every spec is written in it; changing it rewrites all specs | **decided** (in use) |
| 4 | State store = **two ledgers** as the etcd-analog: the bead ledger (`br`, git-JSONL — work state) + the proof/verdict ledger (`docs/provenance/ledger.jsonl` + yield `gate-verdict` — admission state) | data gravity; migrating the state store is a full migration | **decided** (store binding corrected 2026-06-17; bd/Dolt retired — see banner + 2026-06-17 delta) |
| 5 | Cross-vendor neutrality (operator-owned, portable, not vendor-locked) | the moat + a values commitment; coupling to one vendor is a rewrite to undo | **decided** (load-bearing) |

**Two-way doors — decide fast, pivot freely (the adapters / mechanisms):**

| Decision | Reverse by |
|---|---|
| Substrate = NTM + Agent Mail | swap the swarm runtime behind the dispatch port (already pivoted once: *not* gascity) |
| Scheduler algorithm (capability/cost/model-fit) | tune/replace behind the scheduling port |
| Quorum roster (which model families) | add/remove models — the quorum *mechanism* is stable, the roster isn't |
| HA topology (primary/standby vs N-way replication) | the *requirement* "survive single-host" is one-way; the *mechanism* is two-way |
| Self-healing policy (retries, backoff, reschedule triggers) | tune freely once probes exist |
| Compute hosts (Mac / bushido / cloud) | fungible by design — add/remove nodes |
| Operator surface / observability | additive, iterate freely |

**The rule this gives the build:** before cutting a step, ask *"am I deciding a port or an adapter?"*
A port → one-way door → design it **through the council** before committing. An adapter → two-way
door → build the simplest thing and pivot when it hurts. **The one OPEN one-way door is #2 — the
workload-object schema** — it gates build step 2 and must be council-designed before any controller,
scheduler, or agent codes against it. Everything downstream is an adapter we can pivot.

---

## 2026-06-17 delta — the adapter taxonomy, the agent two-altitude reconciliation, the store correction

> **Dated addition, separately gated.** Everything above is the 2026-06-05 whole-spine pass. This section
> is the 2026-06-17 delta from the `/discovery --mixed` cross-family pass (WARN → PASS-with-hardening;
> plan: [`.agents/plans/2026-06-17-control-plane-primitives-unification.md`]). It does not re-open the
> 2026-06-05 ratification. **This doc is the unifying *entry* for the control-plane primitives — it points
> at the sibling owners ([primitive-chains.md](primitive-chains.md), [canonical-loop-model.md](canonical-loop-model.md),
> [loop-map.md](loop-map.md), [control-loop-model.md](control-loop-model.md)); it does not compete with them.**

### Behavior sibling: this doc is the *citizens*, control-loop-model.md is the *behavior*

The factory above names **who the citizens are** (roles × primitives × adapters — the control-plane
structure). [control-loop-model.md](control-loop-model.md) names **how they behave over two timescales**
(fast = convergence within a run; slow = governed improvement across runs, the SPC governor). Structure +
behavior are the two halves of one control system; read them together. The membrane gate the citizens pass
through is specified in [pawls.md](../contracts/pawls.md); the seven moves they execute are
[operating-loop.md](operating-loop.md); the loop's other altitudes are indexed in [loop-map.md](loop-map.md).

### The adapter taxonomy — which named thing is which citizen

The factory is a composable set of control-plane primitives expressed **within** the existing DDD/hexagonal
architecture ([ADR-0001](../adr/ADR-0001-ddd-hexagonal-adoption.md), [ports-and-adapters.md](ports-and-adapters.md)) —
not a new architecture. Every named AgentOps surface is one of four citizen kinds:

| Surface | Citizen kind | What it is |
|---|---|---|
| **Skills** (`/plan`, `/discovery`, `/rpi`, …) | **use-case logic** (behind a driving port) | the application-layer behavior the loop runs; they *call* the instruments, they are not themselves the instrument |
| **`ao` CLI** | **driven adapter — the deterministic instrument** | the windshield: ground-truth checks, gates, ledger writes. Repeatable + codeable ⇒ an `ao` subcommand; skills just call it |
| **MCP** | **driven adapter — alternate transport** | the same instruments over a different transport; guards apply per-port. *Leaving the box changes the threat model* — an out-of-process/remote transport widens the trust boundary the in-process CLI did not |
| **Hooks** | **guard adapter — the tool-use seam** | mechanical, un-reasoned-past enforcement at the tool-call boundary (silent on the happy path, fires only on a real violation) |
| **Gates / pawls** | **guard adapter — admission at a port boundary** | the membrane's deterministic admission checks; a passed gate ratchets and cannot reopen |
| **The agent** | **actuator — the AgentPod / data-plane workload** | the stochastic worker that *does* the work. **Governed, never the controller.** (Already consistent with this doc's data-plane framing.) |
| **The trigger layer** (`/goal`, cron, Workflow, Ralph, ATM, the in-session driver) | **invoker = the Orchestrator-controller** | the thin reconcile loop that reads the verdict, applies the bounding primitives, and routes to the next altitude — it never reasons about the work (control-loop-model §6) |

**The invoker / actuator split (the load-bearing rule):** trust the **controller + the gates**, never the
**actuator**. The invoker (Orchestrator-controller) gates and routes; the actuator (agent) generates and is
gated. This is just *orchestration determinism runs inverse to worker determinism* ([loop-map.md](loop-map.md)
"The system, not the DAG") stated as a citizen split: the map (controller + gates) is deterministic and
trusted; the route (what the agent does) is stochastic and re-routed on a failed verdict.

**"invoker" is a synonym, not a new primitive.** It names the Orchestrator/driving-adapter role already in
the Roles × Primitives table above (the Orchestrator controller) — added only because "trigger layer" and
"the thing that kicks off a run" needed one word. No new primitive is introduced; the cathedral guard holds.

**No-orchestrator-object.** There is no monolithic "Orchestrator" component to build. Orchestration is a
**swappable invoker role** — any trigger that runs the reconcile loop (the in-session driver, an NTM swarm,
cron, a Workflow) plays it. AgentOps ships no daemon for it ([ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md));
out-of-session, a substrate (NTM + MCP Agent Mail + managed-agents) plays the invoker. The role is fixed;
the thing playing it is two-way-door.

### The agent at two altitudes — reconciling ports-and-adapters.md

[ports-and-adapters.md](ports-and-adapters.md) classifies slash commands, MCP, and the autonomous loops
the **agent drives through** as **primary (driving) adapters** — the agent calls *into* the Go-runtime
domain hexagon. This doc classifies the agent as the **data-plane workload (AgentPod, actuator)**. **Both
are true at different altitudes — this is not a contradiction:**

- **Runtime altitude** (the Go-runtime hexagon, ports-and-adapters.md): the agent **invokes adapters** — it
  is on the *driving* side of the inner domain hexagon, calling `ao`/skills/MCP into the domain.
- **Factory altitude** (this doc, the process control plane): the agent **is the governed workload** — the
  AgentPod the controller schedules, probes, and gates; data-plane, not control-plane.

Same agent, two hexagons nested at different scales: it *drives* the code-level domain and *is driven by*
the process-level controller. When you read "the agent is a driving adapter" (runtime) and "the agent is the
data-plane actuator" (factory), apply this note — they describe the same agent from inside vs. outside the
process control plane. (Forward-linked both ways: ports-and-adapters.md → here for the factory altitude;
here → ports-and-adapters.md for the runtime altitude.)

### The store correction (recap) — two ledgers play etcd

The etcd-analog is **two ledgers**, deliberately not flattened into one:

- **Bead ledger** — `br` (git-JSONL, `BEADS_DIR=$PWD/_beads br …`): **work / desired state** (the
  declarative workload objects + the dependency DAG). Synced via `git -C _beads push`; never `git add _beads`.
- **Proof / verdict ledger** — `docs/provenance/ledger.jsonl` + the yield `gate-verdict` events:
  **admission state** (what has been verified and admitted; the in-situ catch-rate reads from here).

bd/Dolt is retired (single-host SPOF, no offline lane). The two ledgers together provide the durable,
HA-able state the control plane reconciles and admits against.
