export const meta = {
  name: 'bulk-read',
  description:
    'Delegate large or many files to cheap bulk readers: one reader per file reads the whole file in bounded slices and answers one question with line-referenced bullets only; the file bytes never enter the caller\'s context.',
  whenToUse: 'When a file exceeds the read budget (or an opt-in read-budget guard blocked a Read) and the caller needs an answer about its contents, not the contents: caller supplies the question and file paths via args; one cheap reader per file, bullets only.',
  phases: [{ title: 'Read', detail: 'one bounded-slice reader per file (parallel), bullets only', model: 'haiku' }],
};

// CONTRACT: a reader returns bullets that cite file:line refs; the caller sees
// this structure and nothing else — never the file bytes.
const READER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['file', 'bullets', 'lines_covered', 'complete'],
  properties: {
    file: { type: 'string' },
    bullets: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['ref', 'text'],
        properties: {
          ref: { type: 'string' },
          text: { type: 'string', maxLength: 200, pattern: '^[^\\r\\n\\u0085\\u2028\\u2029]*$' },
        },
      },
    },
    lines_covered: { type: 'integer', minimum: 0 },
    complete: { type: 'boolean' },
    note: { type: 'string', maxLength: 300, pattern: '^[^\\r\\n\\u0085\\u2028\\u2029]*$' },
  },
};

function badArgs(detail) {
  throw new Error(
    'bulk-read: bad args (' + detail + '). Expected ' +
      '{ question: string, files: [string, ...], root?: string, model?: string, ' +
      'maxBullets?: positive integer, budgetLines?: positive integer }'
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!input || typeof input !== 'object' || Array.isArray(input)) badArgs('args missing');
if (typeof input.question !== 'string' || !input.question.trim()) badArgs('question must be a non-empty string');
if (!Array.isArray(input.files) || input.files.length === 0 || input.files.some((f) => typeof f !== 'string' || !f.trim())) {
  badArgs('files must be a non-empty array of non-empty path strings');
}
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');
if (input.model !== undefined && (typeof input.model !== 'string' || !input.model.trim())) badArgs('model must be a non-empty string when given');
if (input.maxBullets !== undefined && (!Number.isSafeInteger(input.maxBullets) || input.maxBullets <= 0)) {
  badArgs('maxBullets must be a positive safe integer when given');
}
if (input.budgetLines !== undefined && (!Number.isSafeInteger(input.budgetLines) || input.budgetLines <= 0)) {
  badArgs('budgetLines must be a positive safe integer when given');
}

const model = input.model || 'haiku';
const maxBullets = input.maxBullets || 40;
const budgetLines = input.budgetLines || 350;
const where = input.root
  ? 'Work in ' + input.root + '.'
  : 'Work in the current repository (the session working directory).';
const basename = (p) => p.split('/').filter(Boolean).pop() || p;
const singleLine = (value, cap) => typeof value === 'string' && value.length <= cap && !/[\r\n\u0085\u2028\u2029]/.test(value);
function validResult(r, file) {
  if (!r || typeof r !== 'object' || Array.isArray(r) || r.file !== file ||
      !Array.isArray(r.bullets) || !Number.isSafeInteger(r.lines_covered) || r.lines_covered < 0 ||
      typeof r.complete !== 'boolean' || (r.note !== undefined && !singleLine(r.note, 300)) ||
      Object.keys(r).some((key) => !['file', 'bullets', 'lines_covered', 'complete', 'note'].includes(key))) return false;
  return r.bullets.every((b) => {
    if (!b || typeof b !== 'object' || Array.isArray(b) || !singleLine(b.text, 200) ||
        typeof b.ref !== 'string' || !b.ref.startsWith(file + ':') ||
        Object.keys(b).some((key) => !['ref', 'text'].includes(key))) return false;
    const range = /^([1-9][0-9]*)(?:-([1-9][0-9]*))?$/.exec(b.ref.slice(file.length + 1));
    if (!range) return false;
    const start = Number(range[1]);
    const end = Number(range[2] || range[1]);
    return Number.isSafeInteger(start) && Number.isSafeInteger(end) && end >= start && end <= r.lines_covered;
  });
}

phase('Read');

const results = await parallel(
  input.files.map((file) => () =>
    agent(
      'You are a bulk reader. Your structured return is the ONLY thing the caller sees; the file bytes never reach the caller.\n\n' +
        'File to read: ' + file + '\n' +
        'Question to answer about it:\n' + input.question + '\n\n' +
        'Rules:\n' +
        '- ' + where + '\n' +
        '- Read the file COMPLETELY in slices with the Read tool: Read(file_path, offset, limit) with limit ≤ ' + budgetLines +
        '. This is a PER-CALL limit, not a total reading budget. Start with offset: 1 and supply both offset and limit on every Read. ' +
        'Continue from the line after the last line actually received until EOF. A short response proves EOF only if it is untruncated and no remaining lines are indicated. ' +
        'If output is truncated, retry from the first unread line with a smaller limit; never skip unseen lines or treat truncation as EOF. ' +
        'Never an unbounded Read, cat, head or tail (an opt-in read-budget hook may block them). A blocked read is not coverage.\n' +
        '- The bullet cap limits the answer, not how many lines to read. An early answer does not end the read: later lines may revise it. ' +
        'If you cannot reach EOF, report partial coverage and do not present an early answer as the final file-wide one.\n' +
        '- Answer the question with bullets only: each bullet is { ref: "<file>:<line>" or "<file>:<start>-<end>", text: one line of at most 200 characters }, ' +
        'most relevant first, at most ' + maxBullets + ' bullets. No prose, no preamble, no multi-line code.\n' +
        '- Read-only: no Write, no Edit, no mutating Bash.\n' +
        '- Report lines_covered (lines you actually read) and complete (true only when every line was read) truthfully. ' +
        'Use the Read tool source line-number labels for citations and coverage; exclude tool wrappers, system reminders and a nonexistent EOF line. ' +
        'For a complete read from line 1, lines_covered is the last actual source line number (0 for an empty file). ' +
        'Never approximate counts or add requested slice limits. If exact coverage cannot be established, report only verified lines, complete: false and a note. ' +
        'A missing, binary or unreadable file gets zero bullets and a note saying why (one line, at most 300 characters). ' +
        'Summarize; do not copy source code or file content into text or note.\n' +
        '- Return file as the path given above.',
      { label: 'bulk-read:' + basename(file), phase: 'Read', schema: READER_SCHEMA, model, effort: 'low' }
    )
  )
);

// CONTRACT: parallel() resolves failed thunks to null. A dead reader or invalid
// result yields an explicit error; missing output cannot establish coverage.
const files = input.files.map((file, i) => {
  const r = results[i];
  if (!r || !validResult(r, file)) {
    const error = r ? 'reader returned an invalid result; coverage unknown' : 'reader agent failed; coverage unknown';
    log('bulk-read[' + basename(file) + ']: ' + error);
    return { file, bullets: [], lines_covered: null, complete: false, error };
  }
  // The prompt caps bullets at maxBullets; enforce the cap here too so an
  // over-eager reader cannot push more than the caller asked for into context.
  const out = { file, bullets: r.bullets.slice(0, maxBullets), lines_covered: r.lines_covered, complete: r.complete };
  if (r.note) out.note = r.note;
  // No silent caps: say what the cap dropped, or the result reads as complete coverage.
  const dropped = r.bullets.length - out.bullets.length;
  if (dropped > 0) log('bulk-read[' + basename(file) + ']: dropped ' + dropped + ' bullet(s) over maxBullets=' + maxBullets);
  log(
    'bulk-read[' + basename(file) + ']: ' + out.bullets.length + ' bullets, ' + r.lines_covered + ' lines covered' +
      (r.complete ? '' : ' (incomplete)')
  );
  return out;
});

const bullets_total = files.reduce((n, f) => n + f.bullets.length, 0);
log('bulk-read: ' + bullets_total + ' bullets across ' + files.length + ' file(s)');

return { question: input.question, files, bullets_total };
