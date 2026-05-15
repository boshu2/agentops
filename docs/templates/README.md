# Templates

## Operating-loop artifacts

These templates carry one turn of the [operating loop](../architecture/operating-loop.md): BDD intent → vertical slices → conflict-free wave → bead acceptance → evidence.

- [Intent Issue (BDD-shaped)](./intent-issue.md) — produced by `/discovery` (or `/brainstorm` for the earlier free-text → structured pass). The intent issue is not ready until acceptance examples are testable.
- [Slice Validation Plan](./slice-validation.md) — produced by `/plan`, executed by `/validation`. One row per vertical slice; roll-up proves the bead's acceptance examples.

## Authoring templates

- [Workflow Template](./workflow.template.md)
- [Agent Template](./agent.template.md)
- [Skill Template](./skill.template.md)
- [Command Template](./command.template.md)
- [Kernel Template](./kernel.template.md)

## Configuration examples

- [Dark Factory Schedule Example](./dark-factory-schedule.yaml.example)
- [Schedule Example (`schedule.yaml.example`)](./schedule.yaml.example) — stock starter schedule for `ao daemon` continuous workers; copy to `.agents/schedule.yaml` or run `ao init --with-schedule`.

Back: [Docs Index](../documentation-index.md)
