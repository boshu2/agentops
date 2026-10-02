export const meta = {
  name: 'bdd-foundry',
  description: 'Retired compatibility tombstone for the behavior-first planning conveyor',
  whenToUse: 'Never — this name is kept only so existing invocations fail with a deterministic migration message.',
  phases: [
    { title: 'Migration notice', detail: 'fails immediately with replacement pointers' },
  ],
}

throw new Error(
  'workflows/bdd-foundry.js is retired. ' +
    'State accepted behavior in the existing conversation or BD bead, implement and check it natively, ' +
    'then obtain fresh independent judgment of the exact change. ' +
    'Use bd directly for work status and dependencies. See AGENTS.md and docs/agent-workflow-reference.md.'
)
