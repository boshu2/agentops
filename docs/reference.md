# AgentOps Reference Router

This page used to duplicate the command, skill, lifecycle, and runtime manuals.
Those copies drifted. Use the source owner for the question instead.

| Need | Canonical source |
|---|---|
| Product and current claims | [README](../README.md), [PRODUCT](../PRODUCT.md), [GOALS](../GOALS.md) |
| Beginning-to-done workflow | [Operating loop](architecture/operating-loop.md) |
| Skill selection | [SKILLS](SKILLS.md), [Skill Router](SKILL-ROUTER.md), [Skills Matrix](skills-matrix.md) |
| Four RPI umbrellas | [`/rpi`](../skills/rpi/SKILL.md): Discovery → Crank → Validate → Learn |
| Post-verdict bookkeeping | [`/learn`](../skills/learn/SKILL.md) |
| Retrospective causal analysis | [`/postmortem`](../skills/postmortem/SKILL.md), only after Validate and Learn |
| CLI commands and flags | [Generated CLI reference](../cli/docs/COMMANDS.md) |
| DDD bounded contexts | [Component Map](architecture/component-map.md) |
| Hexagonal ports and adapters | [Ports and Adapters](architecture/ports-and-adapters.md) |
| Gate and release behavior | [CI/CD](CI-CD.md), [Agent Workflow Reference](agent-workflow-reference.md) |
| Installation and upgrades | [Getting Started](getting-started/index.md), [Upgrading](UPGRADING.md) |

The active in-session sequence is:

```text
shape acceptance → build a vertical slice → Validate → Learn
                 → orchestrator decision → gate/pawl evidence → land
```

Postmortem does not validate completion, capture arbitrary insights, operate the
plan, or activate constraints. Mechanical candidates begin advisory and require
replay, negative controls, and measured shadow precision before activation.

For a complete catalog of narrative docs, see the
[Documentation Index](documentation-index.md). Generated counts and inventories
belong to `registry.json`, `skills/catalog.json`, `docs/cli-surface.json`, and
the generated CLI reference; do not copy them into this router.
