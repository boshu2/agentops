# AgentOps — One-Page Brief

AgentOps is a local verification and bookkeeping layer for coding agents. The
agent and its reasoning do the work; AgentOps supplies small contracts, durable
evidence, independent judgment, and deterministic checks where code can prove
facts. The proven product is the verification membrane: **no verdict = not
done**. Claims that the corpus compounds into a moat remain explicitly
[unproven](adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md).

## The operating loop

```text
Orient → shape BDD acceptance → track/isolate → slice → build
       → Validate → Learn → orchestrator decision → land
```

Discovery shapes an execution packet. Crank executes behavior-sized slices.
Validate produces an immutable evidence-bound verdict. Learn is the only
post-verdict handoff: it copies structured observations, reconciles recurrence,
and emits `plan_impact` without changing proof, the plan, delivery state, or
constraints. The orchestrator alone retries, re-plans, stops, or closes.
Postmortem is optional and only tests an explicit retrospective causal question
after Validate and Learn.

## Four product layers

<!-- agentops:claim:AOP-CLAIM-BRIEF-FOUR-LAYERS -->

| Layer | Purpose | Canonical surfaces |
|---|---|---|
| Bookkeeping | Preserve the objective, attempts, evidence, and verdict | beads, RPI receipts, provenance |
| Local context | Load only the contracts and evidence the current phase needs | skills, execution packets, `.agents/` |
| Validation membrane | Challenge plans and completion claims independently | `/premortem`, `/validate`, `/council`, pawl, `ao gate` |
| Learning ratchet | Turn verdict observations into bounded future-facing candidates | `/learn`, `/pattern-mining`, `/operationalize` |

`.agents/` is local runtime evidence, not public repository truth. Skills are the
front door. The `ao` CLI is the supporting control plane for tracking,
provenance, gates, and landing; it is not a substitute agent runtime.

## Architecture

The system uses six DDD bounded contexts: Corpus, Validation, Loop, Factory,
Runtime, and Orchestration. Go interfaces in `cli/internal/ports/` define the
hexagonal seams. Skills are domain or driving-adapter contracts; the CLI and
external runtimes are adapters. Generated inventories expose the current
skill, command, and ownership matrix without copying volatile totals into prose.

The active in-session waist is:

```text
skills + local agent reasoning
  → operating loop
  → immutable Validate verdict
  → Learn receipt
  → ao gate / pawl evidence
  → land
```

Out-of-session execution is delegated to an operator-selected substrate such as
NTM, Agent Mail, managed agents, or Gas City. AgentOps ships no daemon and does
not auto-route work into a substrate.

## What machines prove and agents judge

Deterministic code owns schemas, generated-file drift, link integrity, command
existence, exact port ownership, tests, and commit-bound evidence. Agents own
semantic review: whether the plan addresses the intent, whether evidence proves
the requested behavior, what risks were missed, and whether a causal claim is
credible. Scripts do not approximate those judgments with keyword counts.

Mechanical candidates start advisory. Activation requires deterministic replay
over stored positives, explicit negative controls, and warn-only shadow evidence
meeting the precision threshold. Learn never activates a constraint.

## Honest posture

<!-- agentops:claim:AOP-CLAIM-BRIEF-VALIDATED-PATTERNS -->

AgentOps can preserve validated patterns and make them available to later work.
It has not yet proven general live-agent uplift or a compounding corpus moat.
The repo tracks those hypotheses separately from the verified membrane so
marketing cannot outrun measurement.

## Canonical next reads

- [Operating loop](architecture/operating-loop.md)
- [Codebase overview](architecture/codebase-overview.md)
- [Skills and CLI matrix](skills-matrix.md)
- [Ports and adapters](architecture/ports-and-adapters.md)
- [Product claims and goals](../PRODUCT.md) · [fitness measures](../GOALS.md)
