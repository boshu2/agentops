# Software Factory Surface

AgentOps is a software-factory operating contract for coding agents. It turns
accepted intent into independently judged evidence while keeping runtimes,
trackers, and repository delivery replaceable.

## Four umbrellas

| Umbrella | Factory role | Output |
|---|---|---|
| Discovery | Orchestrator shapes behavior and consumes Premortem when needed | accepted plan and tranche packet |
| Crank | One writer implements one bounded tranche | frozen candidate and deterministic receipts |
| Validate | A fresh author-distinct context judges the complete claim set once | immutable PASS or FAIL verdict |
| Learn | The orchestrator records the smallest useful consequence | no-change, plan-impact, or terminal receipt |

Postmortem is optional after Learn and answers an explicit causal question. It
is not a fifth lifecycle gate.

## Roles and runtime variants

The role split is orchestrator, implementer, and validator. One model can fill
all three roles only through separate contexts; the candidate's author cannot
be its validator. One fresh validator is the default. A council or multiple
model families are explicit higher-rigor strategies.

Native Codex, another local agent, NTM-managed panes, managed agents, and cloud
workers all run the same loop. Runtime adapters may provide process durability,
mailboxes, or isolation; they do not change role authority or lifecycle state.

## CLI boundary

The final `ao` CLI is a deterministic transaction kernel and flight recorder.
It may pull one tracked leaf, freeze candidate identity, run factual checks,
record an external verdict, reduce Learn bookkeeping, record delivery facts,
and close a report after remote verification. It does not decide intent, drive
a model, issue semantic judgment, choose Git policy, or operate a merge queue.

The current executable still contains transitional commands and build profiles.
They are removed through exact same-owner K, CLI, and F leaves; this document
does not treat those leftovers as product variants.

## Repository-owned delivery

After Validate and Learn, the consumer repository chooses direct push, a PR,
hosted CI, a dedicated merger, or another adapter. The choice is identical for
local and cloud agents. Delivery consumes proof but cannot upgrade it. AgentOps
records the adapter, target, result, and remote identity.

## Concurrency

One writer and one active leaf are the default. An explicitly requested swarm
may parallelize read-only research. Concurrent write lanes require separate
worktrees, independently checked disjoint manifests, one owner per lane, and a
lead-owned integration order.

## Learning boundary

Learn runs once in the orchestrator's existing context. A repeated defect may
become a proposed check only after two distinct objectives plus runnable
positives, negative controls, shadow evidence, an owner, rollback, and expiry.
Pattern mining and promotion stay off the tranche critical path.

## Design rules

<!-- agentops:claim:AOP-CLAIM-SOFTWARE-FACTORY-THIN-TOPICS -->
- Prefer briefings over giant startup dumps.
- Keep substrate and operator surfaces distinct.
- Let external validation outrank self-report.
- Treat thin topics as discovery-only until evidence improves.
- Keep deterministic mechanisms separate from semantic judgment.

## Related docs

- [Operating Loop](architecture/operating-loop.md)
- [Agent Workflow Reference](agent-workflow-reference.md)
- [Go CLI Architecture Guide](architecture/go-cli-architecture-guide.md)
- [Context Packet](context-packet.md)
