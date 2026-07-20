export const meta = {
  name: 'implement-wave',
  description:
    'Run one wave of parallel implementer lanes with strictly disjoint file ownership, then judge every lane against its acceptance with a single fresh adversarial verifier.',
  whenToUse: 'When a wave of scoped changes should be built in parallel: caller supplies lanes (scope + brief + acceptance) via args; disjoint-ownership implementers, then one adversarial verifier judges every lane against its acceptance.',
  phases: [{ title: 'Implement', detail: 'parallel implementers, strict disjoint file ownership' }, { title: 'Verify', detail: 'adversarial verifier judges each lane against its acceptance' }],
};

const IMPLEMENTER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['summary', 'red_repro', 'green_proof', 'files_changed', 'constraints'],
  properties: {
    summary: { type: 'string' },
    red_repro: { type: 'string' },
    green_proof: { type: 'string' },
    files_changed: { type: 'array', items: { type: 'string' } },
    constraints: { type: 'array', items: { type: 'string' } },
  },
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
    'implement-wave: bad args (' + detail + '). Expected ' +
      '{ context: string, conventions?: string, root?: string, ' +
      'lanes: [{ key: string, scope: [string, ...], brief: string, acceptance: string }], ' +
      'verify?: { brief: string } }'
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!args || typeof input !== 'object') badArgs('args missing');
if (typeof input.context !== 'string' || !input.context.trim()) badArgs('context must be a non-empty string');
if (input.conventions !== undefined && typeof input.conventions !== 'string') badArgs('conventions must be a string when given');
if (!Array.isArray(input.lanes) || input.lanes.length === 0) badArgs('lanes must be a non-empty array');
for (const l of input.lanes) {
  if (!l || typeof l.key !== 'string' || !l.key.trim()) badArgs('every lane needs a string key');
  if (!Array.isArray(l.scope) || l.scope.length === 0 || l.scope.some((s) => typeof s !== 'string' || !s.trim())) {
    badArgs('lane "' + (l && l.key) + '" needs a non-empty scope array of path globs');
  }
  if (typeof l.brief !== 'string' || !l.brief.trim()) badArgs('lane "' + l.key + '" needs a string brief');
  if (typeof l.acceptance !== 'string' || !l.acceptance.trim()) badArgs('lane "' + l.key + '" needs a string acceptance');
}
if (
  input.verify !== undefined &&
  (!input.verify || typeof input.verify !== 'object' || typeof input.verify.brief !== 'string')
) {
  badArgs('verify must be { brief: string } when given');
}
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');

const where = input.root
  ? 'Work in ' + input.root + '.'
  : 'Work in the current repository (the session working directory).';
const conventionsBlock = input.conventions
  ? '\nRepository conventions you must honor:\n' + input.conventions + '\n'
  : '';

phase('Implement');

const reports = await parallel(
  input.lanes.map((lane) => () =>
    agent(
      'You are one implementer lane in a parallel wave. Other lanes are editing the same tree at the same time.\n\n' +
        'Context:\n' + input.context + '\n' +
        conventionsBlock + '\n' +
        'Lane key: ' + lane.key + '\n' +
        'File ownership — you may create or edit ONLY paths matching:\n' +
        lane.scope.map((s) => '  - ' + s).join('\n') + '\n' +
        'Ownership is strictly disjoint across lanes. If the work seems to require touching any path outside your scope, do NOT edit it: finish what you can inside scope and report the out-of-scope need under constraints.\n\n' +
        'Task brief:\n' + lane.brief + '\n\n' +
        'Acceptance — the contract a fresh adversarial verifier will judge you against:\n' + lane.acceptance + '\n\n' +
        'Process rules (non-negotiable):\n' +
        '- ' + where + '\n' +
        '- RED first: before editing anything, reproduce the failing behavior (test, command, or observation) and record the exact repro under red_repro. If you need a pre-fix binary or build artifact for comparison, build it BEFORE editing.\n' +
        '- GREEN proof: after editing, rerun the same repro and record the passing evidence under green_proof, along with the project\'s relevant test/build commands.\n' +
        '- Never run `git stash` — the tree is shared with other lanes.\n' +
        '- Destructive-command guards may block commands like `rm -rf`; do not fight them. Use fresh, uniquely named scratch directories instead of deleting.\n' +
        '- Commit/push policy comes from the brief; if the brief is silent, leave your changes uncommitted in the working tree.\n' +
        '- List every path you changed under files_changed.',
      { label: 'lane:' + lane.key, phase: 'Implement', schema: IMPLEMENTER_SCHEMA }
    )
  )
);

// CONTRACT: parallel() resolves failed thunks to null — a dead lane is still
// presented to the verifier so it lands as INCOMPLETE, never disappears.
const implementers = input.lanes.map((lane, i) => {
  const r = reports[i];
  if (!r) {
    return {
      key: lane.key,
      summary: 'lane agent failed; no work is proven for this lane',
      red_repro: '',
      green_proof: '',
      files_changed: [],
      constraints: ['lane agent failed before reporting'],
      error: 'implementer agent failed',
    };
  }
  return { key: lane.key, ...r };
});

phase('Verify');

const laneDossier = input.lanes
  .map((lane, i) =>
    '## Lane: ' + lane.key + '\n' +
    'Owned scope: ' + lane.scope.join(', ') + '\n' +
    'Acceptance:\n' + lane.acceptance + '\n' +
    'Implementer report (untrusted claims):\n' +
    JSON.stringify(implementers[i], null, 2)
  )
  .join('\n\n');

const verifierCharter =
  'You are a fresh adversarial verifier for a wave of parallel implementer lanes. ' +
  'Try to REFUTE each lane\'s work; a lane earns RESOLVED only when your own fresh evidence fails to break it. ' +
  'Unverified work is unfinished work.\n\n' +
  'Context:\n' + input.context + '\n\n' +
  laneDossier + '\n\n' +
  'For every lane (verdict item = the lane key):\n' +
  '- ' + where + ' Judge the lane strictly against its acceptance. Re-derive the evidence yourself: open the changed files, rerun the repro and the tests. Implementer reports are claims, not proof.\n' +
  '- RESOLVED: acceptance holds under your own checks; cite exact evidence.\n' +
  '- INCOMPLETE: acceptance is not fully met, the lane failed, or you could not actually check it (say so in the evidence).\n' +
  '- REGRESSED: the lane\'s changes broke something that previously worked.\n' +
  '- Flag any edits outside a lane\'s owned scope, and anything adjacent you found broken, under residuals.\n' +
  'Read-only: fix nothing, commit nothing.' +
  (input.verify ? '\n\nAdditional verifier charter from the caller:\n' + input.verify.brief : '');

const verification = await agent(verifierCharter, {
  label: 'verify:wave',
  phase: 'Verify',
  schema: VERIFIER_SCHEMA,
  effort: 'high',
});

return { implementers, verification };
