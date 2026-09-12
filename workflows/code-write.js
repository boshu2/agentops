export const meta = {
  name: 'code-write',
  description:
    'Delegate patterned file writes to cheap code writers: one writer per item reads a required reference file in bounded slices, writes only its target file to satisfy the spec while matching the reference\'s patterns, optionally runs one check, and returns a receipt; the caller never reads the result.',
  whenToUse: 'When boilerplate or patterned code should be written without its content entering the caller\'s context: caller supplies items (spec + required reference + distinct target) via args; one cheap writer per item, receipts only; validation stays elsewhere.',
  phases: [{ title: 'Write', detail: 'one reference-patterned writer per item (parallel, disjoint targets)', model: 'haiku' }],
};

// CONTRACT: a writer returns a receipt about the file it wrote — never the
// content. Independent validation of the written file happens elsewhere.
const WRITER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['key', 'target', 'written', 'lines', 'check_ran', 'check_ok', 'summary'],
  properties: {
    key: { type: 'string' },
    target: { type: 'string' },
    written: { type: 'boolean' },
    lines: { type: 'number' },
    check_ran: { type: 'boolean' },
    check_ok: { type: 'boolean' },
    check_output_tail: { type: 'string' },
    summary: { type: 'string' },
  },
};

function badArgs(detail) {
  throw new Error(
    'code-write: bad args (' + detail + '). Expected ' +
      '{ context?: string, root?: string, model?: string, budgetLines?: positive number, ' +
      'items: [{ key: string, spec: string, reference: string, target: string, check?: string }] } ' +
      '(reference is required; targets must be distinct)'
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!args || typeof input !== 'object') badArgs('args missing');
if (input.context !== undefined && typeof input.context !== 'string') badArgs('context must be a string when given');
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');
if (input.model !== undefined && (typeof input.model !== 'string' || !input.model.trim())) badArgs('model must be a non-empty string when given');
if (input.budgetLines !== undefined && (typeof input.budgetLines !== 'number' || !(input.budgetLines > 0))) {
  badArgs('budgetLines must be a positive number when given');
}
if (!Array.isArray(input.items) || input.items.length === 0) badArgs('items must be a non-empty array');
// CONTRACT: targets are disjoint — writers land files directly in the shared
// working tree with no worktree isolation, so two items on one target would race.
const seenTargets = new Map();
for (const it of input.items) {
  if (!it || typeof it.key !== 'string' || !it.key.trim()) badArgs('every item needs a string key');
  if (typeof it.spec !== 'string' || !it.spec.trim()) badArgs('item "' + it.key + '" needs a non-empty string spec');
  if (typeof it.reference !== 'string' || !it.reference.trim()) {
    badArgs('item "' + it.key + '" needs a non-empty string reference (no reference, no writer)');
  }
  if (typeof it.target !== 'string' || !it.target.trim()) badArgs('item "' + it.key + '" needs a non-empty string target');
  if (it.check !== undefined && typeof it.check !== 'string') badArgs('item "' + it.key + '" check must be a string when given');
  if (seenTargets.has(it.target)) {
    badArgs('duplicate target "' + it.target + '" (items "' + seenTargets.get(it.target) + '" and "' + it.key + '"); targets must be distinct');
  }
  seenTargets.set(it.target, it.key);
}

const model = input.model || 'haiku';
const budgetLines = input.budgetLines || 350;
const where = input.root
  ? 'Work in ' + input.root + '.'
  : 'Work in the current repository (the session working directory).';
const contextBlock = input.context ? '\nContext from the caller:\n' + input.context + '\n' : '';

phase('Write');

const receipts = await parallel(
  input.items.map((item) => () =>
    agent(
      'You are a code writer. You write exactly one file from a spec, matching the patterns of a reference file, and return a receipt. ' +
        'The caller will NOT read the file you write; independent validation happens elsewhere.\n' +
        contextBlock + '\n' +
        'Item key: ' + item.key + '\n' +
        'Reference file (patterns to match): ' + item.reference + '\n' +
        'Target file (the ONLY file you may create or edit): ' + item.target + '\n' +
        'Spec:\n' + item.spec + '\n\n' +
        'Rules:\n' +
        '- ' + where + '\n' +
        '- Read the reference file in slices with the Read tool: Read(file_path, offset, limit) with limit ≤ ' + budgetLines +
        '; advance offset until a slice returns fewer lines than limit. Learn its naming, imports, error handling and test shape. ' +
        'Never an unbounded Read, cat, head or tail (an opt-in read-budget hook may block them).\n' +
        '- Write ONLY the target file so it satisfies the spec while matching the reference\'s patterns. Code only: no markdown fences, no prose outside normal code comments.\n' +
        '- Do not create, edit or delete any other file. Other writers are working in the same tree on other targets.\n' +
        (item.check
          ? '- After writing, run this check ONCE with Bash and report check_ran: true, check_ok (exit status 0) and check_output_tail (the last 20 lines of its output):\n  ' + item.check + '\n'
          : '- No check was given: report check_ran: false and check_ok: false.\n') +
        '- NEVER return the file content. Return a receipt only: key, target, written, lines (line count of the target after writing), ' +
        'the check fields, and a summary of at most 300 characters saying what was written (no code).',
      { label: 'code-write:' + item.key, phase: 'Write', schema: WRITER_SCHEMA, model, effort: 'medium' }
    )
  )
);

// CONTRACT: parallel() resolves failed thunks to null — a dead writer must
// surface as an unwritten target with an explicit error, never as a receipt.
const items = input.items.map((item, i) => {
  const r = receipts[i];
  if (!r) {
    log('code-write[' + item.key + ']: writer failed; nothing written');
    return {
      key: item.key,
      target: item.target,
      written: false,
      lines: 0,
      check_ran: false,
      check_ok: false,
      summary: '',
      error: 'writer agent failed; nothing was written by this lane',
    };
  }
  const out = {
    key: item.key,
    target: item.target,
    written: r.written,
    lines: r.lines,
    check_ran: r.check_ran,
    check_ok: r.check_ok,
    summary: r.summary,
  };
  if (r.check_output_tail) out.check_output_tail = r.check_output_tail;
  log(
    'code-write[' + item.key + ']: ' + (r.written ? 'written, ' + r.lines + ' lines' : 'NOT written') +
      (r.check_ran ? ', check ' + (r.check_ok ? 'ok' : 'FAILED') : ', no check')
  );
  return out;
});

return { items };
