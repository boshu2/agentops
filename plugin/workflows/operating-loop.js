export const meta = {
  name: 'operating-loop',
  description: 'Retired compatibility tombstone for the seven-move operating-loop conveyor',
  whenToUse: 'Never — this name is kept only so existing invocations fail with a deterministic migration message instead of silently running retired doctrine.',
  phases: [
    { title: 'Migration notice', detail: 'fails immediately with replacement pointers' },
  ],
}

// The seven-move operating-loop conveyor is retired. Its arguments (shape /
// wave / ratchet moves, replan and retry budgets) do not map onto the RPI
// traversal's one-experiment contract, so nothing is translated automatically:
// choosing the replacement shape is the caller's decision.
//
// - One bounded experiment: native implementation, checks and fresh judgment;
//   the `rpi` skill (skills/rpi/SKILL.md) is optional guidance.
// - Repository delivery: native Git and BD operations under the caller's
//   repository policy, or a caller-selected factory via its coordinator.
//
// The former doctrine page moved to docs/architecture/rpi-traversal.md.
throw new Error(
  'workflows/operating-loop.js is retired. ' +
    'For one bounded experiment use native implementation, checks and fresh independent judgment; ' +
    'the rpi skill (skills/rpi/SKILL.md) is optional guidance. ' +
    'For repository delivery use native Git and BD operations under the caller repository policy, ' +
    'or select a software factory and dispatch through its coordinator. ' +
    'Arguments are not translated automatically (the seven-move shapes are incompatible with one RPI traversal). ' +
    'See docs/architecture/rpi-traversal.md.'
)
