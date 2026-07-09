# Templates

## Operating-loop artifacts

These templates carry one turn of the [operating loop](../architecture/operating-loop.md): BDD intent → vertical slices → conflict-free wave → bead acceptance → evidence.

- [Intent Issue (BDD-shaped)](./intent-issue.md) — produced by `/discovery` (or `/brainstorm` for the earlier free-text → structured pass). The intent issue is not ready until acceptance examples are testable.
- [Goal Design Intent](./goal-design-intent.md) — schema-backed `.agents/goal-design/<slug>/intent.md` artifact for objective, BDD behavior, boundaries, evidence, and hard rules.
- [Goal Design Driver](./goal-design-driver.md) — schema-backed `.agents/goal-design/<slug>/driver.md` artifact for four-loop routing, candidate beads, route-back rules, digest integrity, and validation policy.
- [Slice Validation Plan](./slice-validation.md) — produced by `/plan`, executed by `/validation`. One row per vertical slice; roll-up proves the bead's acceptance examples.

## Authoring templates

- [Workflow Template](./workflow.template.md)
- [Agent Template](./agent.template.md)
- [Skill Template](./skill.template.md)
- [Command Template](./command.template.md)
- [Kernel Template](./kernel.template.md)

Back: [Docs Index](../documentation-index.md)
