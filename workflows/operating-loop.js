export const meta = {
  name: 'operating-loop',
  description: 'Retired compatibility tombstone: the seven-move operating-loop workflow was replaced by distinct workflows',
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
// - One bounded experiment, independently judged: workflows/rpi.js
// - Multi-bead repository delivery orchestration: workflows/ship-beads.js,
//   or a caller-selected software factory operated through its own doors.
//
// The former doctrine page moved to docs/architecture/rpi-traversal.md.
throw new Error(
  'workflows/operating-loop.js is retired. ' +
    'For one bounded experiment use workflows/rpi.js; ' +
    'for multi-bead delivery use workflows/ship-beads.js or a caller-selected factory. ' +
    'Arguments are not translated automatically (the seven-move shapes are incompatible with one RPI traversal). ' +
    'See docs/architecture/rpi-traversal.md.'
)
