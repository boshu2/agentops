# Operating Loop

> Canonical transition model for AgentOps work. The root
> [`AGENTS.md`](../../AGENTS.md) is the compact always-loaded router; this file
> owns the detailed proof loop. Repository-specific bead, worktree, candidate,
> landing, and report mechanics live in the
> [Agent Workflow Reference](../agent-workflow-reference.md).

AgentOps is not a CI server or a Git workflow. It is an operating contract for
turning intent into independently judged evidence. A consumer repository keeps
its own test, delivery, and CI policy. AgentOps supplies the role separation and
artifacts that make an agent's “done” claim inspectable.

## Roles and trust boundary

| Role | Owns | Cannot authorize |
|---|---|---|
| Orchestrator | acceptance, leaf selection, scope, evidence classification, next transition | a worker's unreviewed completion claim |
| Implementer | one bounded behavior, its RED/GREEN evidence, and the candidate it authors | the binding semantic verdict on that candidate |
| Validator | one frozen candidate, its declared claims, deterministic receipts, and one complete blocker set | repair, delivery, or a verdict on mutable work |

One model may fill all three roles across separate contexts. The invariant is
fresh, independent context at the verdict boundary: `author_id != validator_id`.
A council or mixed-model panel is an optional higher-rigor validator strategy,
not the default and not a second lifecycle.

## State flow

```text
READY DEMAND
  → ACTIVE TRANCHE (one active leaf at a time)
  → 1-3 WAVE CHECKPOINTS
  → FROZEN TRANCHE CANDIDATE
  → VALIDATED CANDIDATE
  → LEARNED CANDIDATE
  → DELIVERED
  → REMOTE VERIFIED
  → REPORTED / CLOSED LEAF
```

A goal or epic is aggregate demand. It never occupies WIP. One behavioral leaf
is the bounded tranche and remains the unit of implementation, proof, learning,
and delivery. It may take one to three sequential low-risk implementation waves,
with targeted acceptance after each. One writer holds one active leaf, and no
second leaf starts until the current leaf is remotely verified and reported.

## 1. Orient and shape acceptance

Read the request, repository contract, status, and only the canonical sources
triggered by the work. Resolve these questions before mutation:

- What authority permits the change?
- What exact behavior is requested, and what is explicitly outside scope?
- Which bounded context and source owner control it?
- What state already exists locally, in the tracker, and on the target remote?
- Which action is the next reversible move, and which later action is a pawl?

Acceptance is executable shared language, not a prose aspiration. Record:

- one capability name;
- Given/When/Then for the normal path and at least one edge;
- the first runnable acceptance check;
- exact writable paths and read-only consumers;
- non-goals and rollback;
- deterministic evidence and independent judgment required for done.

Use `/discovery` when acceptance or authority is missing, `/plan` to form
reviewable vertical slices, and `/premortem` to judge an accepted plan before
expensive or irreversible work. Premortem judges the plan, never the finished
implementation.

## 2. Pull one leaf and isolate it

Before tracked work intended to land, claim one ready BDD-shaped leaf and bind it
to one writer and one worktree. The leaf packet records:

- admitted remote base;
- one behavior and its acceptance examples;
- exact write scope and read-only consumers;
- first RED or honest pre-change baseline;
- prerequisites and concurrency conflicts;
- rollback and proof boundary.

A missing path discovered during implementation is not implicit permission to
expand scope. If a read-only consumer must change, or the behavior cannot be
completed inside the accepted write set, return `REPLAN` before editing it.

## 3. Build one vertical behavior

For behavior-changing work:

1. Author the named acceptance check.
2. Run it against the admitted base.
3. Confirm RED is caused by the missing behavior, not a missing harness, syntax,
   setup, unrelated baseline failure, or arbitrary threshold.
4. Make the smallest coherent change that turns it green.
5. Refactor under green without changing the acceptance check.

Docs-only, pure relocation, and accepted `--no-test-first` work use an honest
pre-change baseline plus a negative fixture when needed to prove the detector.
Every failure considered for `REPAIR` is first attributed to the candidate; a
base-reproduced failure is NOTE unless the leaf explicitly owns it.

A Crank tick is bounded implementation, not an unlimited retry controller. It
runs only checks selected from the changed behavior and authority surface. At a
wave boundary, the orchestrator reuses the accepted Premortem while plan inputs
and risk remain unchanged; material change receives one fresh Premortem before
another wave. Validate and Learn never sit between unchanged low-risk waves.

## 4. Freeze the tranche candidate

After at most three waves or 90 minutes, the complete intended tranche is
committed before binding review. Its candidate
receipt pins at least:

- admitted base SHA;
- candidate SHA and tree;
- owned paths with blob identities or explicit deletions;
- acceptance and claim identities;
- selected deterministic commands and receipts;
- relevant registry/toolchain identities;
- author identity and clean-worktree state.

The receipt is immutable evidence, not a mutable status document. Any candidate
edit invalidates the frozen identity and every verdict that consumed it. Base-only
movement may reuse semantic judgment only when owned blobs, deletions, acceptance,
claim dependencies, and evidence dependencies remain identical and explicit
overlap/mapping proof is green.

## 5. Prove facts, then judge meaning once

Deterministic checks prove facts such as syntax, schema, identity, paths, drift,
tests, and evidence integrity. Select the cheapest checks that cover the changed
surface, run each selected fact once for an exact input, and retain its receipt.
Scripts do not score prose usefulness or substitute for engineering judgment.

One validator in fresh context receives the frozen tranche identity, acceptance claims,
changed surface, and factual receipts. It returns:

- exact candidate and base identities;
- claim-by-claim citations;
- one complete blocker set, not serial discoveries;
- PASS or FAIL;
- NOTE, REPAIR, or REPLAN disposition for each finding.

Routine work uses one independent validator. Add a council or multiple model
families only for an explicitly named high-blast-radius or irreversible decision.
Green deterministic checks without the independent verdict are not done.

After one consolidated repair batch, refreeze and re-review only original
findings, changed claims, named interaction risks, and invalidated evidence. A
second distinct repair need is evidence that acceptance, slicing, or approach is
wrong and returns to `REPLAN`; it does not buy another whole-diff review loop.

After semantic repair and affected-claim closure, run the repository's full
deterministic terminal gate once on the final exact candidate. Persist its
exact-input receipt so delivery or pre-push may reuse it instead of rerunning the
same suite. Only then seal the final Validate result and hand it to Learn.

## 6. Govern evidence without turning every failure into an andon

| Disposition | Meaning | Next legal move |
|---|---|---|
| `NOTE` | cosmetic, pre-existing, theoretical, or outside acceptance | record if useful; never block |
| `REPAIR` | introduced, concrete, verifiable acceptance/correctness/safety/contract defect | one consolidated local repair |
| `REPLAN` | evidence invalidates the slice, acceptance, dependency graph, or approach | return to the earliest invalidated planning move |
| `HOLD` | mutation cannot safely continue automatically and no hard ceiling is spent | pause mutation; one bounded fresh-context helper |
| `ANDON` | human authority is required, or a declared hard time/cost/quota ceiling is spent | stop and ask for the smallest operator decision |

A finding blocks only when it is introduced or newly reachable in the candidate,
concrete and verifiable, and breaks acceptance, correctness, safety, or a claimed
contract. WARN, PARTIAL, reviewer disagreement, generated drift, ordinary test
failure, or retry count alone is not ANDON.

One run-level governor owns attempts, time, token/cost, and helper consumption.
Phase-local retry multipliers are forbidden. Max-attempts, oscillation, or
no-progress enters HOLD and receives exactly one helper consultation. `UNSTUCK`
resumes with a new approach; helper `ESCALATE`, human-only judgment, or a genuinely
spent hard ceiling raises ANDON. The static pawl boundary is
[`docs/contracts/pawls.md`](../contracts/pawls.md); RPI's durable transitions are
the [pull-flow governor](../../skills/rpi/references/pull-flow-governor.md).

## 7. Learn, deliver, verify, and report

`/learn` runs once per tranche, consumes the immutable Validate verdict, and writes the smallest honest
receipt: no change, material plan impact, or terminal. It may copy candidate,
verdict, finding, and recurrence identities; it may not change the verdict,
candidate, delivery authority, or plan. The orchestrator alone consumes
`plan_impact` and chooses the next move.

| Transition | Owner | Authority |
|---|---|---|
| Capture | `learn` | Immutable-verdict observations + plan impact for the orchestrator |

Pattern mining and promotion are off the critical path. A repeated finding earns
a durable check only when it has a named owner, a future consumer, a runnable
activation/holdout example, and rollback or expiry. `/postmortem` is optional and
answers an explicit causal question after Validate and Learn; it is not another
validation pass.

Validation completion and Git delivery are separate transitions.
Delivery consumes the exact candidate, PASS verdict, Learn receipt, and
repository policy; it does not create or upgrade semantic proof. Base movement
triggers only the identity, overlap, mapping, and integration evidence it
invalidates. After delivery, verify the exact remote ref and emit a report
containing landed identity, remote verification, closed leaf, goal status, next
ready leaf, and residual risk. Only that report releases the WIP slot.
Repo-specific commands are in the
[Agent Workflow Reference](../agent-workflow-reference.md).

## Concurrency and software-factory roles

One capable agent plus the local shell is the default. Multiple lanes are useful
only when the operator requests delegation and outputs have independent owners.
Read-only implementer/validator separation may use separate contexts without
multiple writers. Concurrent writers require disjoint paths, separate worktrees,
and explicit integration ownership; a potentially shared path is serialized or
reserved before mutation.

NTM, Agent Mail, Gas City, and councils are optional adapters for durability,
coordination, factory-shaped roles, or higher-rigor judgment. They do not change
the loop, grant write authority, or become mandatory startup infrastructure.

## What the loop deliberately does not own

- the consumer repository's CI provider, PR policy, or merge queue;
- a second tracker or database of duplicate tracker truth;
- automatic agent-runtime startup or a daemon;
- semantic judgment in the CLI;
- full validation in a push hook;
- cumulative final-program review of already validated leaves.

The CLI may provide deterministic transaction and recovery mechanics. Agents own
intent, implementation, and semantic judgment; repositories own delivery policy.
