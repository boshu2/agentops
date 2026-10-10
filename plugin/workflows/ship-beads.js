export const meta = {
  name: 'ship-beads',
  description: 'Retired compatibility tombstone for the repository-delivery conveyor',
  whenToUse: 'Never — this name is kept only so existing invocations fail with a deterministic migration message.',
  phases: [
    { title: 'Migration notice', detail: 'fails immediately with replacement pointers' },
  ],
}

throw new Error(
  'workflows/ship-beads.js is retired. ' +
    'Use native Git and BD operations under the caller repository policy for delivery, ' +
    'or select a software factory and dispatch through its coordinator. ' +
    'AgentOps does not own merge or tracker closure. See AGENTS.md.'
)
