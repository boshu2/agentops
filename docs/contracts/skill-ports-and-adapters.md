# Skill system architecture

The skill system serves the operations layer: portable skills that implement
the semantic work-and-proof protocol, plus optional seams for strategies,
evidence, and runtime adapters. A Goal / Mayor is a caller-side execution
orchestrator that may drive several traversals; it calls AgentOps and is never
an AgentOps-owned control level.

```text
caller product boundary + fitness evidence
  -> caller-selected Goal / Mayor campaign (execution orchestrator)
       -> select one experiment intent
       -> RPI traversal: Plan -> Implement -> fresh Validate -> report and stop
       -> consume the immutable report and verdict
       -> ratchet the graph, select another experiment, or stop
  -> optional post-verdict learning
       -> evidence-backed capability proposal
       -> ordinary RPI traversal to change the skill system
```

The split is semantic, not runtime-specific. A Goal may run in one agent,
several explicit workers, or an external controller. RPI remains one
independently judged traversal in every execution shape.

## Authority by level

| Level | Owns | Does not own |
|---|---|---|
| Caller and product | Desired outcome, product boundary, authority, terminal acceptance | An AgentOps semantic verdict without fresh validation |
| Goal / Mayor (caller-side orchestrator) | Campaign graph, experiment selection, cumulative budgets, ratchet, breakers, terminal campaign report | Rewriting verdicts or issuing its own PASS for a candidate |
| RPI | One ordered experiment dispatch and one report | Campaign continuation, retries, queues, delivery, or work selection |
| Plan | One experiment's acceptance, non-goals, write scope, and first useful check | The campaign graph or a duplicate planning artifact |
| Implement | One exact candidate and factual check evidence | Semantic judgment, repair loops, or later work selection |
| Validate | Independent judgment over unchanged intent and exact subject identity | Candidate edits, continuation, closure, or delivery |
| Runtime adapter | Execution of an explicit packet and runtime facts | Experiment selection, phase meaning, or verdict authority |

The core hard-dependency graph is deliberately small:

```text
rpi -> plan
rpi -> implement
rpi -> validate
```

Hard dependencies mean the source skill cannot perform its declared behavior
without the target. Optional advice, evidence, strategies, and runtime
transport never become hard core dependencies.

## Campaign boundary

A terminal caller outcome that may require several evidence-driven experiments
belongs to a Goal / Mayor. The campaign freezes terminal acceptance and
authority, stores experiments as caller-owned graph nodes, and selects one
bounded experiment at a time. Each selected experiment ends in its own RPI
report and fresh validation result. A declared campaign consumer may additionally
require immutable `verdict.v2` evidence.

The Goal may continue after an informative red result when all of these remain
true:

- the next experiment addresses frozen acceptance or a named blocking
  uncertainty;
- the prior result added non-duplicative, decision-relevant evidence;
- cumulative campaign ceilings have not been renewed or exceeded;
- the next experiment remains inside caller authority.

Campaign attempts, waves, helpers, ratchets, and breakers are Goal state. RPI
may carry their identifiers as opaque correlation facts but never interprets,
resets, or renews them.

## Experiment boundary

One RPI invocation is an evidence transaction:

```text
single-mint resolved intent bytes
  -> exact intent digest reference
  -> one bounded subject change
  -> before/final subject manifests + complete changed paths
  -> factual receipts + effect-receipt.v1
  -> one author-distinct Validate
  -> PASS | FAIL | NOT_PROVEN
  -> optional verdict.v2 / rpi-report.v2 for declared consumers
  -> stop
```

Plan may be an identity/refinement step when the selected graph node is already
well shaped. Implement may use several focused edits and deterministic checks
inside one RED-to-GREEN experiment, but it cannot revise acceptance or start a
second candidate after the subject freezes. Validate judges once and cannot
repair the candidate.

`FAIL` and `NOT_PROVEN` are valid experiment results. They do not imply that a
campaign is over, and they do not authorize RPI to continue. The caller or Goal
decides whether another experiment is justified.

Intent bytes are minted once. Later phases and remote validators receive the
snapshot by digest reference; they do not re-fetch or reserialize the living
source. Acceptance criteria receive stable IDs at freeze. A required criterion
without evidence is `unchecked_required` and forces NOT_PROVEN; a
`declared_exclusion` is valid only when the caller excluded it before the
candidate froze.

Every verdict binds the validator implementation and the verdict, report, and
subject-manifest schema digests. Proof contracts advance through an explicit
transition judged by the previously active contract. A candidate proof
contract may emit shadow results while qualifying, but it cannot activate
itself or reinterpret earlier verdicts.

## Optional seams

Every non-core skill fits one or more explicit seams. Placement grants no new
authority.

| Seam | Purpose | Typical skills | Handoff |
|---|---|---|---|
| Product and fitness | Define or measure the desired state | `product`, `fitness` (`goals` compatibility alias) | Evidence to caller or Goal |
| Campaign design | Compile or lint the outer autonomy contract | `craft-goal` | Goal prompt or safety report |
| Goal observation | Report durable state without steering | `status`, `handoff` | Facts to caller or Goal |
| Intent evidence | Reduce uncertainty before one experiment is frozen | `research`, `codebase-recon`, `domain`, `standards`, `cass`, `reverse-engineer` | Cited facts to Goal or Plan |
| Option shaping | Generate, challenge, or route candidate hypotheses | `idea-genie`, `reality-check`, `automation-shape-routing` | Advisory options, gaps, or semantic route to caller, Goal, or Plan |
| Plan review | Challenge scope or a frozen experiment intent | `scope`, `premortem` | Findings to caller or Goal |
| Implement method | Produce the candidate or focused factual evidence | `test`, `refactor`, `doc`, `scaffold`, `converter`, `skill-builder`, `workflow-builder` | Subject changes and receipts to Implement |
| Validation evidence | Supply a bounded deterministic or specialist check | `security`, `test`, `standards`, `codebase-recon` | Evidence to one accountable Validate |
| Judgment strategy | Add independent perspectives without writing the verdict | `council` | Advisory report to Plan or Validate |
| Post-verdict analysis | Analyze recurrence or causality without changing outcomes | `learn`, `postmortem` | Observations or hypotheses to caller or Goal |
| Capability evolution | Find repeated toil/patterns and propose reusable behavior | `toil-mining`, `pattern-mining`, `operationalize` | Proposal to a later Plan |
| Runtime transport | Execute supplied packets or coordinate explicit actors | `agent-native`, `agy-native`, `codex-exec`, `ntm`, `swarm`, `using-gc`, `agent-mail` | Candidate, evidence, or runtime error |
| Cross-cutting support | Prepare or protect the environment without steering | `bootstrap`, `account-rotation`, `cc-hooks`, `dcg`, `rch`, `sbh`, `ms` | Factual result to the invoking owner |

An optional strategy that finds a material defect cannot silently edit its
input. For example, a Premortem finding after Plan causes the current RPI to
stop before Implement; the caller or Goal may revise the experiment source and
start a new RPI.

`shared` is not a durable miscellaneous seam owner. Runtime-neutral contracts
belong under declared contract owners, while adapter mechanics remain with
their adapters. Executed 2026-07-29: the runtime-neutrality contract moved to
`docs/contracts/runtime-neutrality.md`, its last consumer edge was migrated,
and the `shared` skill is a non-routable tombstone pending the observed-zero
deletion window.

## Execution shape is orthogonal

The semantic unit is chosen before the runtime topology:

1. Decide whether the request is one experiment, a multi-experiment terminal
   campaign, indefinite automation, or evidence-only analysis.
2. For a campaign, establish the Goal contract and select the current
   experiment.
3. Choose inline execution, bounded fanout, persistent workers, or an explicit
   factory.
4. Run each candidate through its own RPI boundary unless several edits are one
   coherent subject with one acceptance surface.

Disjoint file paths are necessary but not sufficient for concurrency. Packets
must also isolate generated surfaces, shared resources, external effects, and
failure cleanup. Runtime completion, delivery acknowledgements, pane state,
quest state, or retries never become RPI or verdict state.

## Skill source contract

`skills/<slug>/SKILL.md` remains the source of truth for one skill. Each
canonical skill must make these facts decidable:

- its unique trigger and false-positive boundary;
- its primary system layer and optional seams;
- its skill-grantable authority and forbidden authority;
- exact inputs, outputs, and side effects;
- the smallest ordered behavior and stop condition;
- unavailable-tool and partial-evidence behavior;
- whether it edits a subject, supplies evidence, judges, or only transports;
- its behavioral probe or deterministic validator.

Lifecycle metadata never grants experiment selection, campaign continuation,
or verdict authority. Experiment selection belongs to the caller or Goal.
Core membership comes from the hard dependency graph rather than a
`core_phase` label, and option generation uses the advisory
`option_shaping` seam rather than `experiment_select`.

The overhaul will encode system placement as generated metadata rather than a
second handwritten inventory. Until that encoding lands, the canonical
placement matrix lives in the
[active overhaul plan](../plans/2026-07-24-skill-system-overhaul.md), while existing
`dependencies`, `context_rel`, `consumes`, and `produces` retain their current
meanings:

- `metadata.dependencies`: required behavioral delegation only;
- `context_rel`: DDD relationship, not phase order;
- `consumes` and `produces`: artifact flow;
- `metadata.effects`: every material local, runtime, or external mutation;
- `output_contract`: the binding result shape or schema.

Generated catalogs, routers, runtime projections, maps, and graphs derive from
skill metadata. They are never edited as architecture sources.

## Invariants

- Only RPI depends on all three core phases.
- Only Validate may write `verdict.v2`, and only for a caller request or
  declared downstream consumer.
- Only the caller or Goal selects another experiment.
- A runtime adapter never changes a semantic outcome.
- A strategy report never masquerades as readiness or PASS.
- A specialist that edits the subject operates under Implement authority.
- A post-verdict consumer never mutates the verdict it reads.
- Intent bytes are minted once and transported by digest reference.
- A candidate proof contract cannot issue its own activation.
- Required evidence gaps cannot be reclassified as exclusions after freeze.
- Observed effects must fit declared effects and caller authority.
- Campaign totals are monotonic and cannot be reset by a new wave, helper,
  subject, process, or artifact.
- Generated surfaces have one source owner and publish transactionally from
  complete staging with recovery from exact pre-run bytes.
- Every live skill has one primary placement, explicit optional seams, accurate
  effects, and a checkable output.
