export const meta = {
  name: 'bead-crank',
  description: 'Retired compatibility tombstone for the former ship-beads alias',
  whenToUse: 'Never — this name is kept only so existing invocations fail with a deterministic migration message.',
  phases: [
    { title: 'Migration notice', detail: 'fails immediately with replacement pointers' },
  ],
}

throw new Error(
  'workflows/bead-crank.js is retired along with workflows/ship-beads.js. ' +
    'Use native Git and BD operations under the caller repository policy for delivery, ' +
    'or select a software factory and dispatch through its coordinator. See AGENTS.md.'
)
