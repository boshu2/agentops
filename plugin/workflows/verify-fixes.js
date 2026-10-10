export const meta = {
  name: 'verify-fixes',
  description:
    'Adversarially verify claimed fixes: one verifier per group tries to refute every claim with fresh evidence, returning RESOLVED | INCOMPLETE | REGRESSED per item plus residuals.',
  whenToUse: 'When claimed fixes need fresh-context adversarial verification: caller supplies claim groups via args; one refuting verifier per group.',
  phases: [{ title: 'Verify', detail: 'one adversarial verifier per claim group (parallel)' }],
};

const VERIFIER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['verdicts', 'residuals'],
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['item', 'verdict', 'evidence'],
        properties: {
          item: { type: 'string' },
          verdict: { enum: ['RESOLVED', 'INCOMPLETE', 'REGRESSED'] },
          evidence: { type: 'string' },
        },
      },
    },
    residuals: { type: 'array', items: { type: 'string' } },
  },
};

function badArgs(detail) {
  throw new Error(
    'verify-fixes: bad args (' + detail + '). Expected ' +
      '{ context: string, root?: string, groups: [{ key: string, items: [string, ...] }] }'
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!args || typeof input !== 'object') badArgs('args missing');
if (typeof input.context !== 'string' || !input.context.trim()) badArgs('context must be a non-empty string');
if (!Array.isArray(input.groups) || input.groups.length === 0) badArgs('groups must be a non-empty array');
for (const g of input.groups) {
  if (!g || typeof g.key !== 'string' || !g.key.trim()) badArgs('every group needs a string key');
  if (!Array.isArray(g.items) || g.items.length === 0 || g.items.some((i) => typeof i !== 'string' || !i.trim())) {
    badArgs('group "' + (g && g.key) + '" needs a non-empty items array of non-empty strings');
  }
}
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');

const where = input.root
  ? 'Work in ' + input.root + '.'
  : 'Work in the current repository (the session working directory).';

phase('Verify');

const results = await parallel(
  input.groups.map((group) => () =>
    agent(
      'You are an adversarial verifier. Your job is to REFUTE the claims below, not to confirm them. ' +
        'A claim earns RESOLVED only when your own fresh evidence fails to break it. ' +
        'Unverified work is unfinished work.\n\n' +
        'Context — what was changed, and where:\n' + input.context + '\n\n' +
        'Claims to attack (group "' + group.key + '"):\n' +
        group.items.map((it, i) => '  ' + (i + 1) + '. ' + it).join('\n') + '\n\n' +
        'For each claim:\n' +
        '- ' + where + ' Re-derive the evidence yourself: open the files, rerun the tests or commands. Never trust the claim\'s own wording or any prior report.\n' +
        '- RESOLVED: you actively tried to break it and could not; cite the exact evidence (paths, commands, output).\n' +
        '- INCOMPLETE: a fix exists but does not fully discharge the claim, or you could not actually check it (say so in the evidence).\n' +
        '- REGRESSED: the change broke something this area previously got right.\n' +
        '- Return each verdict\'s item as the claim text you judged.\n' +
        'Record anything adjacent you found broken under residuals. Read-only: fix nothing, commit nothing.',
      { label: 'verify:' + group.key, phase: 'Verify', schema: VERIFIER_SCHEMA, effort: 'high' }
    )
  )
);

// CONTRACT: parallel() resolves failed thunks to null — a dead verifier must
// surface as unverified work, never as silent success.
const groups = input.groups.map((group, i) => {
  const r = results[i];
  if (!r) {
    return {
      key: group.key,
      verdicts: group.items.map((item) => ({
        item,
        verdict: 'INCOMPLETE',
        evidence: 'verifier agent failed; claim was never checked',
      })),
      residuals: [],
      error: 'verifier agent failed for this group',
    };
  }
  return { key: group.key, verdicts: r.verdicts, residuals: r.residuals };
});

return { groups };
