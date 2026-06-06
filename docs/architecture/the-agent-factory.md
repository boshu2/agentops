# The Agent Factory — Workload-Object Model + PodSpec + Watch Surface

> **STATUS: PROPOSAL — one-way door #2, pending council ratification.** This document
> is the design gate for `ag-egpu.1`, NOT a committed contract. No schema in
> `schemas/` changes until the council ratifies. It blocks `ag-v1xk` (reconcile
> controller), `ag-xanm` (fungible compute), and `ag-7dnb` (operator interface) —
> none of those may code against this surface until it is ratified, because the
> hardest-to-change thing in a control plane is its API (k8s's lesson). Drafted
> 2026-06-06 by an autonomous factory tick; the council decision and any edits are
> the operator's.

## Why this is a one-way door

A workload-object schema is the factory's API. Once a scheduler queues against it, a
controller reconciles against it, and N agents code against it, every field is load-
bearing and every rename is a migration across the whole control plane. k8s never got
to remove a field from `PodSpec`. We pay the design cost once, here, on purpose —
before anything depends on it.

## The k8s mapping (resolved)

The factory is k8s's *control-plane shape* pointed at stochastic agents instead of
containers. The mapping is the design's backbone — every proposed field earns its
place by analogy to a primitive that already works at cluster scale.

| k8s primitive | AgentOps analog | Status |
|---|---|---|
| etcd (state of record) | `bd`/Dolt (→ br/JSONL-in-git per the `ag-o2tc` backbone pivot) | **EXISTS** |
| Node (fungible compute) | `remote-compute-target.schema.json` (Mac/bushido/cloud) | **EXISTS** (schema); scheduling across them is `ag-xanm` |
| Deployment / Job (desired workload) | quest / epic-bead (intent + `.feature` acceptance + dep DAG) | **EXISTS** |
| Pod (one running unit) | one agent run against one leaf bead | **EXISTS** (implicit) |
| **PodSpec** (how to run the Pod) | `worker-spec.v1.schema.json` (model/tools/effort/timeout) | **EXISTS as a template; NOT bound to a bead** ← delta (a) |
| **Job with N pods / replica set** | SwarmJob-spec (role composition over worker-specs) | **MISSING** ← delta (b) |
| Scheduler (queue → node) | `bd ready` (unblocked work surface) | **EXISTS** |
| Controller / reconcile loop | `rpi` (inner) / `evolve` (outer) | **EXISTS**; formalization is `ag-v1xk` |
| watch/event API (controllers react, not poll) | `watch-event.v1` (`.agents/watch/events.jsonl`) | **EXISTS but swarm-scoped; no `bead.*` events** ← delta (c) |
| admission control | quorum-at-gates / verdict ledger (cost law) | **EXISTS** |

**The object model is ~85% built.** The bead's framing is correct: beads *are* the
workload objects. The net-new surface is three deltas, each small and each reconciling
with a schema that already ships. The design's job is to specify those three — not to
reinvent the object model.

---

## Delta (a) — Bind a PodSpec to a bead, derived from stakes

### Problem

`worker-spec.v1` already describes *how* to run one agent (model, tools, effort,
timeout). What is missing is the *binding*: which bead gets which spec, and the rule
that derives it. Today every agent is dispatched with the same implicit spec
(inherit-model, full tools). That violates the **cost law** ([memory:
cost-law-quorum-at-gates-cheap-at-generation]): tokens must follow reversibility —
multi-model quorum only at one-way doors and admission gates, cheap single-model for
reversible generation. Without a PodSpec on the bead, the scheduler cannot scale the
Pod to the stakes.

### Proposal

Add an **optional** `pod_spec` projection on the bead, plus a **stakes → PodSpec
derivation rule** so the common case needs no hand-authoring. The PodSpec reuses
`worker-spec.v1` fields verbatim and adds only swarm-shape fields.

```jsonc
// pod_spec.v1 (PROPOSED) — attached to a bead via metadata or a sidecar projection.
// Superset of worker-spec.v1; agent-count + diversity are the only net-new fields.
{
  "schema_version": 1,
  "agent_count": 1,                 // replicas; >1 = redundant/quorum pod
  "model_diversity": "single",      // single | mixed  (charter commitment 6: opt-in)
  "worker": { /* worker-spec.v1 inline or $ref */ },
  "anti_affinity": ["files:<glob>"],// don't co-schedule writers of the same file
  "volume_tier": "worktree",        // worktree | shared-ro | none
  "derivation": "auto"              // auto = derived from stakes; manual = author-set
}
```

**Derivation rule (the cost law as code):**

| Bead stakes signal | Derived PodSpec |
|---|---|
| reversible, single surface (`type=chore/docs/task`, no `one-way-door` label) | `agent_count: 1, model_diversity: single` (cheap generation) |
| one-way door / admission gate (`label: one-way-door`, schema change, deletion, merge-to-main) | `agent_count: 3, model_diversity: mixed` (quorum at the gate) |
| epic with N independent children | `SwarmJob` (delta b), not a single Pod |

The derivation is the default; an operator may override with an explicit `pod_spec`
(`derivation: manual`). This keeps charter commitment 1 (single-model default) and
commitment 6 (diversity is opt-in) intact: diversity is reached for *only* at a gate,
mechanically, by the stakes of the work — never as the assumed shape.

### Open question for council

- **Binding location:** `pod_spec` as bead **metadata** (projection, `bd
  update --metadata`) vs. a **first-class bead field** (schema migration to
  `bead.v1`). Metadata is reversible and avoids a `bead.v1` bump now; a first-class
  field is queryable by the scheduler without a metadata read. **Recommendation:
  metadata projection first** (reversible, no migration), promote to a field only when
  the scheduler needs to filter on it at queue time.

---

## Delta (b) — SwarmJob-spec (role composition)

### Problem

A `worker-spec` is one Pod. An epic that fans out to a wave of differentiated roles
(spec-author → test-writer → impl → reviewer) is a *Job with N pods of M shapes* — the
missing `team-spec`. `crank`/`swarm` already spawn waves, but the role composition is
implicit in skill prose, not a typed object a controller can reconcile.

### Proposal

```jsonc
// swarm-job.v1 (PROPOSED) — the typed wave/team composition for an epic-bead.
{
  "schema_version": 1,
  "epic_id": "<bead-id>",
  "strategy": "wave",               // wave | parallel | pipeline
  "max_concurrency": 12,            // honors the <=12 agent cap (hard)
  "roles": [
    { "name": "spec-author", "pod_spec": { /* pod_spec.v1 */ }, "count": 1 },
    { "name": "impl",        "pod_spec": { /* pod_spec.v1 */ }, "count": 3,
      "for_each": "child_bead" }    // one Pod per ready child bead
  ],
  "barrier": "per-wave"             // per-wave | none (pipeline = none)
}
```

`roles[].pod_spec` reuses delta (a). `max_concurrency` makes the **≤12 agent cap**
(`CLAUDE.md` Session Constraints) a schema-enforced field, not a prose rule. `for_each:
child_bead` is how a SwarmJob expands to the `bd ready` queue — the scheduler already
has the DAG.

### Open question for council

- Does `swarm-job.v1` subsume the existing `swarm-evidence.schema.json` /
  `orchestration-backend.v1` pair, or sit beside them? **Recommendation: beside** —
  evidence/result are *output* contracts; SwarmJob is the *input* (desired-state)
  contract. They compose; they don't merge.

---

## Delta (c) — `bead.*` events so controllers react instead of poll

### Problem

`watch-event.v1` already gives a real append-only event surface
(`.agents/watch/events.jsonl`) — but its event enum is **swarm-scoped**
(`wave.started`, `worker.spawned`, …). A reconcile controller (`ag-v1xk`) that wants to
react to *bead state changes* (a bead became ready, a claim went stale, a verdict
landed) has nothing to subscribe to and must poll `bd ready`. Polling is exactly the
anti-pattern k8s's watch API exists to kill.

### Proposal

Extend the `watch-event.v1` `event` enum with a `bead.*` family (additive — the schema
is `additionalProperties: true` and consumers already tolerate unknown fields):

```text
bead.created      bead.claimed       bead.claim_stale
bead.ready        bead.status_changed bead.verdict_landed
bead.blocked      bead.unblocked      bead.closed
```

Each carries the existing `issue_id`/`ts`/`data` fields. The emitter is the `bd` write
path (claim/status/verdict transitions); the consumer is the reconcile controller in
`ag-v1xk`. This is the minimum that turns the loop from poll to watch.

### Open question for council

- **Emitter placement:** emit `bead.*` from inside `bd` (every write, runtime-agnostic)
  vs. from an `ao` projection that tails Dolt. **Recommendation: from `bd`** — same
  argument as charter commitment 3 (durable surface is the source of truth); an `ao`
  tail can drop events on crash, a write-path emit cannot.

---

## What this proposal deliberately does NOT do

- **No `bead.v1` schema migration** in this pass — delta (a)/(b) ride on metadata
  projections; delta (c) is additive to an `additionalProperties: true` schema. The
  one-way door is the *design commitment to these shapes*, not a code change. Building
  the schemas is downstream of ratification.
- **No scheduler/controller code** — that is `ag-v1xk` / `ag-xanm`, explicitly blocked
  on this gate.
- **No new compute-target fields** — `remote-compute-target.schema.json` already models
  the Node; cross-host scheduling is `ag-xanm`'s problem, not this API's.

## Door register entry (for the constitution / council)

| Field | Value |
|---|---|
| Door | #2 — factory control-plane API (workload-object model) |
| Reversibility | **one-way** (every consumer codes against it) |
| Ratification | council quorum required before `ag-v1xk`/`ag-xanm`/`ag-7dnb` build |
| Deltas | (a) PodSpec-on-bead + stakes derivation · (b) SwarmJob-spec · (c) `bead.*` events |
| Reuses | `worker-spec.v1`, `watch-event.v1`, `quest.v1`, `bead.v1`, `remote-compute-target` |
| Net-new schemas | `pod_spec.v1`, `swarm-job.v1` (proposed); enum extension to `watch-event.v1` |
| Migration risk | **low** — metadata projections + additive enum; no field removals |

## Related

- `docs/architecture/fungibility-charter.md` — PodSpec derivation honors commitments 1 (single-model default) and 6 (diversity opt-in).
- `ag-egpu` (parent epic) · `ag-v1xk` (reconcile controller, blocked) · `ag-xanm` (fungible compute, blocked) · `ag-7dnb` (operator interface, blocked) · `ag-o2tc` (HA control-plane store).
- Schemas reconciled: `schemas/{worker-spec.v1,watch-event.v1,quest.v1,bead.v1,remote-compute-target}.schema.json`.
