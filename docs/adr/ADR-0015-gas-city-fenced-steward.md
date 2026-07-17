# ADR-0015: Gas City Fenced Steward Factory

- **Status:** Accepted; v1 implemented and single-bead qualified (2026-07-17),
  multi-wave fault matrix pending
- **Author:** AgentOps maintainers
- **Scope:** Optional Gas City product-factory adapter; no expansion of the
  AgentOps core loop
- **Builds on:** [Operating loop](../architecture/operating-loop.md),
  [Gas City execution adapter](../contracts/gas-city-execution-adapter.md), and
  [optional orchestration ports](../contracts/orchestration-ports.md)
- **Evidence:**
  [Gas City role-topology audit](../audits/gas-city-role-topology-2026-07-17/README.md)

## Context

The current `agentops-executor` pack deliberately implements only a replaceable
execution boundary: one caller-supplied packet goes to one fresh Codex or Claude
Implementer or Validator, returns runtime evidence, and stops. It has no Mayor,
program graph, retry authority, Git lifecycle, integration queue, PR ownership,
or release semantics.

That boundary is correct for one AgentOps experiment but insufficient for the
operator's requested product workflow: converse with one Mayor about a large
product, decompose it into safe parallel experiments, run Codex and Claude lanes,
reject weak work without recycling its author, integrate accepted candidates in
bounded waves, and drive protected PRs to `main`.

Gas City already supplies a role-agnostic orchestrator, durable bead graph,
agents, formulas, rigs, packs, orders, waits, events, health reconciliation, and
session scaling. It hardcodes no Mayor, Witness, Refinery, or other Gastown role.
The design question is therefore which semantic authorities earn configured
roles and which responsibilities belong in deterministic mechanisms.

A dated archaeology and blind Fable-versus-GPT-5.6-sol duel compared the current
AgentOps contract, Gas City SDK, Gastown pack, archived Mt Olympus roles, and a
product-scale branch/PR model. Both independent proposals converged on a
persistent semantic Mayor, fresh Judges, a deterministic state spine, and a
bounded delivery authority. Both rejected a resident Witness/Governor and a
per-quest conductor tier as default topology.

## Decision

Build a separate optional pack, provisionally `agentops-factory`, around the
existing thin executor. The factory uses the **Fenced Steward** topology:

1. One city-scoped **Mayor** is the operator-facing semantic planner. It proposes
   product graphs, scopes, dependencies, provider routing, and newly identified
   successor experiments.
2. Fresh **Validator/Judge** pools judge Mayor-authored plans, frozen candidate
   subjects, integration cuts, and current merge-eligible PR heads.
3. One deterministic **graph reducer/admission gate** is the sole writer of
   graph transitions and admission certificates.
4. One logical **Refinery** per rig owns fenced delivery state, an integration
   worktree and branch, PR mechanics, and delivery receipts. Its optional LLM
   triage remains fresh, zero-minimum, and max-one.
5. Gas City controller, formulas, waits, events, leases, orders, and repository
   policy own mechanics. Protected repository policy is the only writer to
   `main`.
6. Keep `agentops-executor` unchanged in responsibility. The factory imports it;
   it does not turn AgentOps' one-experiment protocol into a queue, retry loop,
   or delivery system.

The outer factory bead graph and the inner AgentOps experiment have distinct
completion semantics:

```text
outer factory bead graph
  operator -> Mayor proposal -> plan review -> graph admission
  -> many independent AgentOps experiments -> delivery

inner AgentOps experiment
  exact intent -> Implement once -> fresh Validate once
  -> PASS | FAIL | NOT_PROVEN -> report and stop
```

For the thin executor, Gas City transport-bead closure is transport state. For
the factory, experiment and Refinery beads are the actual work units and close
only through verdict- and delivery-gated reducer transitions. Neither kind of
closure implies an AgentOps release verdict.

## Authority boundaries

| Responsibility | May do | Must never do |
|---|---|---|
| Mayor | Converse with operator; propose DAG, acceptance-preserving scopes, priorities, dependencies, provider routing, and successor experiments | Implement; judge; write terminal graph state; mutate integration Git or PRs; merge; silently change acceptance |
| Plan-review Judge | Freshly judge a plan digest for missing acceptance, semantic coupling, unsafe decomposition, and unowned scope | Repair the plan; dispatch work; become persistent |
| Worker | Execute one bounded experiment in one leased branch/worktree/index and declared write scope | Write a verdict; resume after rejection; touch peer/integration/`main` branches |
| Validator/Judge | Emit one binding `verdict.v2` over an exact immutable intent and subject | Implement, repair, merge, mutate graph state, or reinterpret a verdict after subject mutation |
| Reducer/admission gate | Verify identities, digests, scope, leases, verdict policy; write one graph transition and admission certificate | Make semantic judgments, synthesize PASS, or retry an experiment |
| Refinery | Assemble admitted SHAs; regenerate once; publish fenced integration cut; manage PR mechanics; record landed receipt | Author semantic repair; judge; replan; start experiments; bypass repository policy; raw-push `main` |
| Protected repository gate | Enforce required CI, validation, review, merge queue, and final branch write | Accept stale validation or treat Refinery identity as approval |

Persistence describes different things:

- Mayor has a persistent semantic identity while a product program is active;
  its context may recycle from the durable bead graph.
- Refinery has persistent authority recorded on its bead, not a persistent
  LLM transcript.
- Validator has persistent role semantics, but every judgment uses a fresh
  context.

## Rejection ratchet

`FAIL` and `NOT_PROVEN` terminally end the exact experiment they judge.

The immutable verdict creates a blocking rescope bead. A separate HQ transport
bead routes that work through a fresh Mayor context; only the Mayor or operator
may propose a newly scoped successor, and only the reducer may admit it. The
successor preserves acceptance and non-goals while changing its execution
approach, and receives a new experiment identity, branch, worktree/index,
lease, and fresh worker. The rejected worker is never resumed; the rejected
subject remains evidence; Refinery cannot repair or admit it conditionally.
Automatic rescope is bounded (three attempts by default); the ceiling leaves
the rescope bead in HOLD for an explicit operator resume rather than looping.

If the successor changes product acceptance, the operator must first approve
the canonical intent change. A GC close, CI result, retry counter, Mayor opinion,
or Refinery classification cannot upgrade a binding result.

## Validation and integration policy

- Routine candidate: one fresh Validator, normally from a provider family other
  than the worker.
- High-risk or disputed candidate: fresh Codex and Claude judgments.
- First merge-eligible mixed-author PR head during the proof period: Codex and
  Claude PASS over the same exact subject.
- Any merge, rebase, regeneration, or other content mutation invalidates the
  prior subject certificate and requires fresh validation.
- One deterministic admission certificate references the immutable component
  verdict digests. It cannot erase dissent or invent a verdict.
- A dispute may receive one fresh re-panel, then operator judgment. The Mayor
  never breaks a Validator tie.

## Git and delivery policy

```text
main                                           protected
gc/candidate/<program>/<node>/<attempt>        one worker, frozen exact SHA
gc/integration/<program>/<wave>/<epoch>        one fenced Refinery record
```

Every concurrent writer receives a distinct worktree and Git index. Worker
credentials and hooks restrict pushes to that worker's candidate namespace.
Refinery holds a monotonic fencing epoch per repository and target branch;
every integration push and PR mutation presents the current token and uses
compare-and-swap or force-with-lease against the recorded head.

Integration proceeds in bounded trains with a configured candidate-count limit
(five by default in v1). Refinery applies exact admitted SHAs in stable DAG
order in a scratch tree, runs deterministic checks after each application,
regenerates shared outputs once, publishes one integration cut, obtains fresh
semantic validation, and opens or updates one PR. At most one wave per target is
merge-eligible in v1; dependent later waves may be draft PRs with explicit
dependency metadata.

Use separate forge identities for PR authorship and semantic validation.

## Provider policy

Codex and Claude are physical routing variants of the same semantic roles.
Each `agent.toml` selects one provider, each packet names `codex` or `claude`,
and runtime evidence must match.

Do not run two simultaneous Mayors merely to use both providers. On bo-mac,
start with a Codex Mayor and bounded fresh Claude plan-review, worker, and
Validator lanes because the host contract records a Claude long-running
process-spawn failure mode. Provider unavailability must be visible policy data;
it cannot silently lower validation requirements.

## Role disposition

| Historical name | Disposition |
|---|---|
| Mayor | Keep as the only persistent semantic identity; make it config-deletable |
| Judge-Witness | Fresh plan, candidate, integration, and PR-head Judge modes |
| Gastown recovery-Witness | Controller health, events, waits, lease/reap, branch freeze, and alerts |
| Persistent assurance Governor | Reject; use scheduled fresh sampling and durable projections |
| Refinery | Keep durable delivery authority and fresh max-one triage; no resident LLM |
| Deacon / Boot / Dog | Controller health, thresholds, exec orders, and escalation |
| Zeus | Reject as Mayor duplicate |
| Apollo | Dormant Mayor mode; trial a quest-scoped pool only after measured overload |
| Athena | Deterministic retrieval/context compilation; on-demand synthesis |
| Themis | Fresh Validator plus deterministic admission certificate |
| Argus | Deterministic audit order; findings only |
| Hades | Receipt- and fence-gated cleanup order |
| Hermes | Reject; beads, immutable evidence refs, events, waits, PRs, and receipts are the bus |
| Chiron | Fresh feedback-shaping mode inside a Mayor-proposed successor intent |
| Named heroes | Provider/skill variants on fungible fresh workers |

## Rejected alternatives

### Expand `agentops-executor` into the factory

Rejected because it would collapse the replaceable one-packet transport boundary
into work ownership, retry, Git, and delivery policy.

### Persistent Witness/Governor

Rejected because durable history and deterministic observation already provide
its longitudinal inputs. A resident context adds context gravity, availability
dependency, duplicate state, and veto creep without a unique trust seam.

### Mayor plus per-quest Apollo conductors

Deferred. It adds a second planning authority before measured evidence that the
Mayor is overloaded. Preserve the prompt as a dormant mode and activate only on
explicit scale triggers.

### Universal dual-provider validation

Rejected as the routine default because it can make the Validator queue the
system bottleneck. Retain it for high-risk/disputed candidates and the proof
period's merge-eligible mixed-author PR head.

### One PR per worker or one giant PR

Rejected. Per-worker PRs externalize shared-generation and integration coherence
to the forge; giant PRs erase reviewability. Use bounded integration trains.

## Consequences

- The operator gets one Mayor interface without granting it implementation,
  judgment, graph-write, or landing authority.
- Product-scale parallelism becomes safe only after scope compilation,
  worktree/index isolation, leases, fencing, exact-SHA admission, and fresh
  validation exist.
- Gas City's retry-capable formula machinery must not retry a semantically
  rejected AgentOps experiment. A redo is a new Mayor-proposed identity.
- Rebase and deterministic regeneration are not semantically invisible: if
  bytes change, the old certificate is stale.
- The factory remains optional and removable. The headless graph/reducer plane
  must operate without a Mayor so deleting the Mayor is a configuration change,
  not a rewrite.

## Rollout and falsification

Implementation completed the first five collapse-order stages:

1. Promote and pin the thin executor after its stability gate.
2. Build schemas, graph compiler, scope conflict detection, worktree/index
   allocation, fenced leases, reducer, and admission certificates.
3. Scale provider-specific Worker and Validator pools above one.
4. Add Mayor and fresh plan review.
5. Add fenced Refinery delivery and protected PR integration.
6. **Pending:** run a proof week with deliberate stale-token, moved-SHA, dead-worker,
   generated-file, semantic-CI, main-moved, provider-outage, and stale-validation
   faults.

The first live canary used both provider families and delivered the exact
Refinery integration head through protected PR
[#916](https://github.com/boshu2/agentops/pull/916). That is evidence for the
single-bead happy path, not a substitute for stage 6.

Kill or absorb the Mayor if at least 80 percent of its actions are deterministic
state restatements or it provides no unique semantic correction or reduction in
operator reload time. Absorb Refinery triage into deterministic orders if at
least 80 percent of its wakes make no unique judgment. Do not add a resident
Witness or Apollo tier without a controlled trial demonstrating unique value.
