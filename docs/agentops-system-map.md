# AgentOps System Map

AgentOps is a local verification membrane around an operator-selected coding
agent. Skills carry domain contracts; the `ao` CLI supplies deterministic
bookkeeping, provenance, gates, and landing mechanics.

## One loop

```text
operator intent
  → Discovery: BDD acceptance + execution packet
  → Crank: behavior-sized slices and waves
  → Validate: immutable evidence-bound verdict
  → Learn: observations + recurrence bookkeeping + plan impact
  → orchestrator: retry | re-plan | stop | terminal
  → pawl / ao gate: commit-bound evidence
  → land
```

`/postmortem` is an optional side path after Validate and Learn. It answers one
explicit retrospective causal question; it does not validate completion,
harvest general learnings, mutate the plan, or activate constraints.

## Six bounded contexts

| Context | Owns |
|---|---|
| BC1 Corpus | local evidence, retrieval, pattern mining, operationalization |
| BC2 Validation | verdicts, gates, pawls, plan risk review |
| BC3 Loop | Discovery, Crank, Learn, goals, execution state |
| BC4 Factory | skill generation, registries, standards, dispositions |
| BC5 Runtime | CLI, installers, plugin/runtime packaging |
| BC6 Orchestration | substrate-neutral whole-skill dispatch and coordination |

The exact skill and Go-interface ownership is generated from
`docs/contracts/skill-dispositions.yaml`, `docs/contracts/bounded-contexts.yaml`,
and `cli/internal/ports/`. See the [Component Map](architecture/component-map.md)
and [Ports and Adapters](architecture/ports-and-adapters.md).

## Authority flow

```text
skills/**/SKILL.md               declared behavior
cli/cmd/ao + cli/internal        executable behavior
schemas + docs/contracts         machine-readable boundaries
registry/catalog/CLI docs        generated projections
docs narrative                   explanation and routing
```

When these disagree, executable and declared contracts win; report and repair
the stale consumer rather than teaching both versions.

## Prevention ratchet

```text
Validate observation
  → Learn advisory candidate
  → replay over stored positives + negative controls
  → warn-only shadow evidence
  → measured precision threshold
  → operator/CLI activation
```

Generated detector scripts are compatibility metadata, not proof. Learn and
Postmortem never activate a constraint.

## Runtime boundary

AgentOps 3.0 is hookless and ships no daemon. One local agent plus the shell is
the default. Out-of-session or parallel execution is an explicit
operator-selected substrate—NTM, Agent Mail, managed agents, Gas City, or
another adapter—that dispatches whole skill contracts without reimplementing
the loop.

## Current inventories

Do not copy totals into this map. Read:

- `registry.json` and `skills/catalog.json` for skills and relationships;
- `cli/docs/COMMANDS.md` and `docs/cli-surface.json` for the CLI;
- `docs/reference/agentops-skill-domain-map.md` for generated BC ownership;
- `docs/skills-matrix.md` for the curated human-facing loop matrix.
