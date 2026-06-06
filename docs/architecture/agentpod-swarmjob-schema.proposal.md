# AgentPod-spec + SwarmJob-spec + watch surface — schema proposal

> **STATUS: PROPOSAL — one-way door #2 (the workload-object API), pending council ratification.**
> This is the *schema-level* design for `ag-egpu.1`. It operationalizes the two net-new deltas the
> architecture spine — [`the-agent-factory.md`](the-agent-factory.md) (`ag-yv25`, cross-model
> reviewed 2026-06-05) — names for door #2: **(a) an AgentPod-spec + SwarmJob-spec on the bead**
> (model / sidecars / volume-tier / agent-count / anti-affinity, scaled to stakes per the cost law)
> and **(b) a watch/event surface** so controllers react to bead-state changes instead of polling.
> It does **not** restate the architecture — read the spine first; this is the field-level contract
> beneath it. **No schema in `schemas/` changes until the council ratifies.** Drafted 2026-06-06 by
> an autonomous tick; the ratification decision and any edits are the operator's.

## Why this is the one-way door

The spine establishes *that* door #2 is "the bead schema, plus a Pod/Job spec and an event surface."
This doc fixes the *shape of those fields* — and that shape is the part every downstream consumer
(`ag-v1xk` reconcile controller, `ag-xanm` scheduler, `ag-7dnb` operator surface) codes against.
k8s never removed a `PodSpec` field. We design the fields once, here, before anything depends on them.

## What already exists (the reconciliation, not a reinvention)

The spine's mapping says the object model is "mostly already built — it is the bead schema." At the
*schema file* level that is concretely true, and the design must reuse these rather than fork them:

| Spine concept | Existing schema | What it already gives us |
|---|---|---|
| AgentPod-spec (one agent's run shape) | `schemas/worker-spec.v1.schema.json` | model · tools · effort · timeout · prompt — **already a Pod template** |
| The bead (workload object) | `schemas/bead.v1.schema.json` | intent · status · deps · verdict_id · output_path |
| Deployment/quest | `schemas/quest.v1.schema.json` | goal · beads · status |
| Node (compute host) | `schemas/remote-compute-target.schema.json` | provider · capabilities · auth_ref |
| Watch/event surface | `schemas/watch-event.v1.schema.json` | append-only `.agents/watch/events.jsonl`, `additionalProperties:true` |

**The delta is therefore small and additive.** `worker-spec.v1` is an AgentPod-spec minus the
swarm-shape fields; `watch-event.v1` is the event surface minus the `bead.*` lifecycle events. The
three sections below specify exactly that gap and nothing more.

---

## Delta (a.1) — AgentPod-spec: extend `worker-spec.v1`, don't fork it

`worker-spec.v1` describes one agent's run. The spine's AgentPod adds three swarm-topology fields it
lacks. Proposed as a **superset**, so existing `worker-spec.v1` files validate unchanged:

```jsonc
// agentpod-spec.v1 (PROPOSED) = worker-spec.v1 + topology fields
{
  "schema_version": 1,
  "worker": { /* worker-spec.v1 inline or $ref — model/tools/effort/timeout/prompt */ },
  "function": "generator",      // planner | generator | oracle | scribe | integrator   (spine Roles×Primitives)
  "anti_affinity": {            // the membrane, as schedulable constraint
    "model_family": true,       // oracle must NOT share a model family with the generator it judges
    "node": false               // node anti-affinity lands with the scheduler (ag-xanm)
  },
  "volume_tier": "branch"       // branch (Tier 1, shared clone) | worktree (Tier 2, on conflict)
}
```

- `function` binds to the spine's **Planner / Generator / Oracle / Scribe / Integrator** roles — *not* a
  new RBAC enum (RBAC stays `roles.go`); it is the legible product layer the spine already defines.
- `integrator` is **merge-execution**: it takes Oracle-passed, green PRs and integrates them to main
  (rebase/update-branch, CI-shepherd, serialize merges, keep trunk green, revert on red). It is distinct
  from the Oracle (which *decides* whether to merge — the verdict/authority) and the Orchestrator (which
  *schedules* pods). Maps to `RoleWorker` (it holds write/merge perms); it canNOT be folded into the
  Oracle (`RoleVerifier` "never edits") or the Orchestrator (control-plane injection boundary).
  **Added by 2-of-3 cross-model quorum 2026-06-06 (ag-egpu; Codex + agy APPROVE, Claude author-recused)
  — derivation: msg #160.** Two quorum-mandated guardrails:
  1. **Mechanics-only authority (Codex):** the Integrator may run integration mechanics, but any
     *semantic* conflict-resolution that changes code re-enters the Generator/Oracle gate — an
     Integrator is never a privileged post-verdict author path that mutates approved code without fresh
     anti-affine review.
  2. **Circuit-breaker (agy):** automated merge/revert and rebase loops must be bounded (max retries,
     halt-on-thrash) so a misbehaving Integrator cannot thrash trunk or burn CI without limit.
- `anti_affinity.model_family` is the schema-level expression of the no-self-grade membrane
  (`liveness` kernel: author≠judge, ≥2 families). An Oracle AgentPod with `model_family:true` may not
  be scheduled onto the Generator's family.
- `volume_tier` maps to the spine's **Volume** axis (worktree ≠ Pod boundary): default `branch`,
  escalate to `worktree` on a conflict signal.

## Delta (a.2) — SwarmJob-spec: the typed composition on the bead

The spine's SwarmJob = "a Generator AgentPod (+ Planner upstream) + ≥2 anti-affine Oracle AgentPods +
a Scribe sidecar, driven by an Orchestrator, scaled to stakes." That composition is currently implicit
in `crank`/`swarm` prose. Typed:

```jsonc
// swarm-job.v1 (PROPOSED) — attached to a bead (epic or leaf) as the desired-state Pod set
{
  "schema_version": 1,
  "bead_id": "<bead-id>",
  "strategy": "wave",            // wave | parallel | pipeline
  "max_concurrency": 12,         // the <=12 agent cap (CLAUDE.md) as a schema-enforced field
  "pods": [
    { "function": "generator", "spec": { /* agentpod-spec.v1 */ }, "count": 1,
      "for_each": "child_bead" },
    { "function": "oracle",    "spec": { /* agentpod-spec.v1, anti_affinity.model_family:true */ },
      "count": 2 }              // >=2 anti-affine Oracles = the quorum floor for a one-way door
  ],
  "stakes": "reversible"        // reversible | one-way-door  -> drives the cost-law derivation below
}
```

### The cost law, as a derivation rule (the spine's law made mechanical)

The spine states the law; this is the rule a scheduler runs. The default SwarmJob is **derived** from
the bead's stakes so the common case needs no hand-authoring:

| Bead stakes signal | Derived SwarmJob |
|---|---|
| reversible / two-way door (`type=chore/docs/task`, no `one-way-door` label) | 1 Generator + 1 Oracle (cheap pre-check), single model family — *generation is cheap* |
| one-way door / admission gate (`label:one-way-door`, schema change, deletion, merge-to-main) | 1 Generator + **≥2 anti-affine Oracles**, best models — *real quorum at the gate* |
| epic with N independent children | one Generator pod `for_each: child_bead`, shared Oracle pool |

The floor the spine names is enforced here: **a single Oracle is a pre-check, never a quorum** — it
cannot admit one-way-door work. `derivation: manual` lets an operator override. This honors the
fungibility-charter (single-model default; diversity is reached for *only* at a gate, by stakes).

## Delta (b) — `bead.*` events: extend `watch-event.v1`, additively

`watch-event.v1` already ships the append-only event surface — but its enum is swarm-scoped
(`wave.*`, `worker.*`). A reconcile controller (`ag-v1xk`) that wants to react to *bead* state has
nothing to subscribe to and must poll `bd ready` — the exact anti-pattern k8s's watch API kills.
Because the schema is `additionalProperties:true` and consumers already tolerate unknown fields, this
is a pure enum extension (no breaking change):

```text
bead.created   bead.ready        bead.claimed      bead.claim_stale
bead.blocked   bead.unblocked    bead.status_changed
bead.verdict_landed              bead.closed
```

Each reuses the existing `ts` / `issue_id` / `data` fields. Emitter = the `bd` write path
(claim/status/verdict transitions). Consumer = the reconcile controller. This is the minimum that
turns the inner loop from poll to watch.

---

## Open questions the council must rule on (the ratifiable decisions)

1. **AgentPod-spec / SwarmJob-spec binding:** carry as bead **metadata projection**
   (`bd update --metadata`, reversible, no `bead.v1` migration) — *recommended* — vs. a first-class
   `bead.v1` field the scheduler can filter on at queue time. Recommendation: metadata first; promote
   to a field only when `bd ready` must filter on it.
2. **SwarmJob vs. existing output contracts:** `swarm-job.v1` (desired-state *input*) sits **beside**
   `swarm-evidence.schema.json` / `orchestration-result.v1` (the *output* contracts) — *recommended* —
   rather than merging; input and output specs compose.
3. **`bead.*` emitter placement:** emit from inside **`bd`'s write path** (durable, crash-safe,
   runtime-agnostic) — *recommended* — vs. an `ao` projection that tails Dolt (can drop events on
   crash). Same argument as fungibility-charter commitment 3 (durable surface is source of truth).
4. **`function` field vs. `roles.go`:** confirm `function` is the legible product layer only, with
   `roles.go` (RBAC authority) unchanged — Planner+Generator share `Worker` authority (spine's
   explicit clarification). No RBAC enum churn.

## Door register entry

| Field | Value |
|---|---|
| Door | #2 — factory control-plane API (workload-object model) |
| Reversibility | **one-way** (every controller/scheduler/agent codes against it) |
| Spine | `docs/architecture/the-agent-factory.md` (`ag-yv25`) — this doc is its field-level contract |
| Deltas | (a.1) AgentPod-spec = `worker-spec.v1` + topology · (a.2) SwarmJob-spec + cost-law derivation · (b) `bead.*` events on `watch-event.v1` |
| Reuses (no fork) | `worker-spec.v1`, `watch-event.v1`, `bead.v1`, `quest.v1`, `remote-compute-target` |
| Net-new schemas | `agentpod-spec.v1`, `swarm-job.v1` (proposed); additive enum on `watch-event.v1` |
| Migration risk | **low** — supersets + additive enum + metadata projection; no field removals |
| Blocks until ratified | `ag-v1xk` (reconcile controller) · `ag-xanm` (scheduler) · `ag-7dnb` (operator surface) |

## Related

- [`the-agent-factory.md`](the-agent-factory.md) — the architecture spine (`ag-yv25`); read first.
- [`fungibility-charter.md`](fungibility-charter.md) — single-model default; cost-law diversity is opt-in.
- Beads: `ag-egpu` (epic) · `ag-egpu.1` (this design) · `ag-v1xk` · `ag-xanm` · `ag-7dnb` · `ag-o2tc` (HA store).
- Schemas reconciled: `schemas/{worker-spec.v1,watch-event.v1,bead.v1,quest.v1,remote-compute-target}.schema.json`.
