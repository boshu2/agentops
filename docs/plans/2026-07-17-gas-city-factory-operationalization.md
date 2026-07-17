# Gas City Factory Operationalization Proposal

```yaml
schema_version: operationalization-proposal.v1
proposal_id: gas-city-fenced-steward-v1
status: implemented-single-bead-qualified
owner: AgentOps maintainers
trigger: >-
  An operator has a product-sized intent that benefits from parallel Codex and
  Claude experiments, requires collision-free writers, and must reach main
  through fresh validation and protected pull-request delivery.
artifact_shape: >-
  An optional Gas City pack plus deterministic schemas, reducer, isolation and
  delivery commands; it imports the existing one-packet executor instead of
  changing the AgentOps core loop.
activation_example: >-
  Give the Mayor a digest-bound AgentOps product intent, admit two disjoint
  experiment nodes, run one with Codex and one with Claude in separate
  worktrees, validate exact candidates in fresh opposite-family contexts, and
  deliver only PASSed SHAs through a fenced integration branch and protected PR.
negative_example: >-
  A single bounded experiment, a shared mutable writer, a request to push main
  directly, or an intent without acceptance criteria does not activate the
  factory; use the ordinary AgentOps loop or stop for operator clarification.
rollback: >-
  Suspend/remove the agentops-factory import and its isolated city. Candidate
  branches, verdicts, and delivery receipts remain evidence; agentops-executor
  and the AgentOps core loop continue unchanged.
```

## Desired behavior

One operator-facing Mayor turns canonical product intent into a reviewed,
content-addressed DAG proposal. A deterministic reducer atomically materializes
that proposal as Gas City beads and updates only bead status, dependencies, and
metadata. Fresh Codex and Claude Workers operate only in
leased, disjoint worktrees. Fresh Validators bind `PASS | FAIL | NOT_PROVEN` to
the exact intent and subject. Non-PASS returns to the Mayor as immutable
evidence and can produce only a newly identified successor with a fresh Worker.
A fenced Refinery admits exact PASSed SHAs, publishes a bounded integration
cut, obtains fresh validation after every byte-changing operation, and uses the
repository's protected PR policy as the only route to `main`.

The bead is the unit of work and the bead graph is the sole lifecycle ledger.
The pack defines reusable roles, prompts, commands, and policy; JSON graph,
review, verdict, certificate, and delivery files are immutable evidence named
by bead metadata, never a parallel state machine. Gas City supplies routing,
session, health, event, wait, and
formula mechanisms. AgentOps still owns one experiment and its evidence-backed
verdict. Gas City closure, CI green, or a model's opinion cannot substitute for
that verdict.

## Inputs and outputs

| Surface | Required input | Produced output |
|---|---|---|
| Mayor proposal | Canonical intent bytes/digest, acceptance, non-goals, repository and base SHA | `program-graph.v1` proposal with nodes, dependencies, scopes, risk, provider preference, and checks |
| Plan review | Exact graph digest and canonical intent digest | Immutable `plan-review.v1`; no graph mutation |
| Reducer | Reviewed graph plus policy and repository facts | One atomic bead graph: program, experiment, dependency, and Refinery beads with metadata-bound leases/fences |
| Worker experiment | One admitted experiment bead and one executor packet | Frozen candidate SHA, manifests, scope/check receipts, runtime identity attached to that bead |
| Validator | Exact intent, candidate subject, author/runtime facts, evidence | One durable `verdict.v2` |
| Admission | Exact candidate and verdict digests | `admission-certificate.v1` or deterministic rejection |
| Refinery | PASSed admitted SHAs and current fenced delivery record | Integration cut, PR state, current-head validation, CI/review receipts |
| Landing | Protected merge result | `delivery-receipt.v1` binding program intent through landed SHA |

## Evidence basis

This proposal primarily transcribes the accepted
[Fenced Steward ADR](../adr/ADR-0015-gas-city-fenced-steward.md), so the
three-instance floor is not used to invent its authority model. Empirical
corroboration comes from three independent AgentOps/GC qualification runs and
three earlier membrane quests:

- qualification 6: distinct `ga-wisp-wrm2` author and `ga-wisp-4vbk`
  Validator, exact manifest re-derived at start/end, PASS;
- qualification 7b: distinct `ga-wisp-qrud` author and `ga-wisp-deq6`
  Validator, exact diff and all criteria checked, PASS;
- qualification 8: distinct `ga-wisp-l5xi` author and `ga-wisp-uh1e`
  Validator, runtime-fact binding and unchanged subject proved, PASS;
- earlier `hello`, `csv-stats`, and `install-gc-city` quests exercised
  confirmed, transient/degraded, and hard-refutation paths, as recorded in the
  prior [GC MVP adoption evidence](../audits/gc-mvp-2026-07-05/adoption-layer-slate.md).

The [Bun Rust-port study](../audits/gas-city-role-topology-2026-07-17/bun-rust-port-research.md)
independently supports contract-before-fanout, phase-specific queues,
controller-owned paths, worktree/resource isolation, and fresh adversarial
contexts. It is corroboration, not authority for AgentOps completion semantics.

## Rules, anchors, and reapply proof

| ID | Operational rule | Quote-bank anchors | Reapply proof |
|---|---|---|---|
| R1 | Keep the factory outside the AgentOps core and import the thin executor unchanged in responsibility. | ADR: “Build a separate optional pack”; architecture: “outer factory program” versus “inner AgentOps experiment”; prior adoption slate: “coexisting substrate.” | Remove/suspend the factory import in a test city and prove `agentops-executor` packet tests and direct run command still work unchanged. |
| R2 | Mayor proposes semantics but never implements, judges, writes graph truth, or delivers. | ADR authority table: “Mayor … Must never … Implement; judge; write terminal graph state”; role audit: “only persistent semantic identity.” | Feed a Mayor result containing a verdict, Git action, or terminal transition to the reducer; schema/policy admission must reject it without state mutation. |
| R3 | The reducer is the singular deterministic writer of bead transitions; no JSON lifecycle ledger exists. | ADR: “sole writer of graph transitions”; role audit: “singular graph-state writer”; Bun study: “workflow owned identity and routing facts.” | Race two events with the same expected epoch; exactly one fenced bead transition succeeds, replay is idempotent, and no sidecar state file is created. |
| R4 | Every concurrent writer gets a distinct worktree, index, candidate branch, lease, and disjoint declared scope; shared paths serialize. | ADR Git policy; architecture Worker pools; Bun study: “prohibited global Git operations … four worktrees.” | Admit disjoint nodes concurrently and prove distinct paths/branches; submit overlapping scopes or a shared generated companion and prove one node remains blocked. |
| R5 | A binding Validator is fresh, author-distinct, read-only, and bound to the exact intent and subject. | Core operating contract: “fresh Validate over the exact content”; three qualification verdicts above; ADR validation policy. | Attempt author-collapsed identity, moved subject, writable validation scope, or missing runtime freshness; each must yield deterministic rejection/`NOT_PROVEN`, never PASS admission. |
| R6 | `FAIL` or `NOT_PROVEN` freezes the experiment and creates a blocking rescope bead routed through a fresh Mayor context; any retry is a newly identified successor with a fresh Worker. | ADR “Rejection ratchet”; operator requirement in role audit; Bun transfer: “Mayor rescope -> new identity and fresh Worker.” | Submit a successor reusing experiment ID, unchanged execution fields, changed acceptance/non-goals, branch, lease, or worker context; reducer rejects it. A new identity/worktree can be admitted only after a Mayor successor proposal. Automatic routing stops at the configured attempt ceiling and leaves the rescope bead in HOLD for explicit operator resume. |
| R7 | Refinery accepts only exact admitted SHAs and uses monotonic fencing; every mutation invalidates prior semantic validation. | ADR Refinery and Git policy; architecture delivery flow; Bun study: “Any Refinery mutation … requires fresh integrated-subject validation.” | Present a stale epoch, moved candidate SHA, rebased integration head, or stale validation digest; publish/PR transition fails until current bytes receive fresh validation. |
| R8 | Protected repository policy is the only writer to `main`; factory landing ends in a durable receipt and does not imply release. | ADR protected repository gate; architecture “Delivery does not imply release”; prior adoption slate “Don't auto-merge / auto-push.” | Deny direct `main` push from Worker/Mayor/Refinery credentials; merge only through the test PR policy, then verify the receipt binds intent, candidates, integration head, PR, and landed SHA. |

## Holdout and negative cases

The implementation must not be promoted by happy-path tests alone. The holdout
set is the fault matrix in ADR-0015: stale token, moved candidate after PASS,
dead Worker, shared generated artifact, semantic coupling despite disjoint paths,
`FAIL`, `NOT_PROVEN`, semantic CI defect, moved `main`, provider outage,
author-collapsed validation, and stale current-head validation.

Additional negative examples:

- two Mayors concurrently claiming terminal authority;
- a Validator that repairs before issuing its verdict;
- a Refinery that edits a semantic defect rather than returning evidence;
- Formula retry resuming a semantically rejected AgentOps experiment;
- one Worker using stash/reset or touching a peer/integration branch;
- a GC work item marked closed being treated as PASS or delivery;
- a docs site or MkDocs deployment being treated as a factory requirement.

## Smallest fitting artifact and implementation state

A skill or one-shot workflow is too small because the behavior requires durable
beads, isolated concurrent writers, fencing, and protected delivery. The pack
is configuration and mechanism distribution, not a unit of work. The smallest
fitting executable form is a pack whose commands create and transition beads:

```text
packs/agentops-factory/
  agent definitions for Mayor and fresh Judge/Refinery triage variants
  commands backed by deterministic schema/reducer/isolation/delivery code
  schemas and doctor checks
deploy/gc/ factory capacity/import configuration
tests/ fault, graph, provider, worktree, and delivery scenarios
```

The pack, reducer, schemas, dual-provider roles, worktree isolation, admission
certificates, and fenced Refinery delivery path are implemented. A disposable
city exercised Mayor -> Claude Worker -> fresh Codex Validator -> Codex Refiner
-> fresh Claude integration Validator and landed protected PR
[#916](https://github.com/boshu2/agentops/pull/916) at
`b80a752aad3843af66160b08a823aaed57e07169`.

That qualification covers one experiment bead and one integration train. The
multi-wave concurrency and holdout fault matrix above remain required before
claiming broad factory promotion.

Deletion condition: remove the Mayor role if at least 80 percent of its actions
are mechanical restatements or it adds no unique semantic correction/operator
reload reduction. Remove LLM Refinery triage under the same threshold. Preserve
the headless reducer, evidence, and delivery records so either role deletion is
a configuration change rather than a rewrite.
