export const meta = {
  name: 'audit-dimensions',
  description:
    'Audit one subject across caller-defined dimensions: a finder per dimension, then a skeptic that re-opens every cited piece of evidence and refutes or downgrades findings before anything is reported.',
  whenToUse: 'When a subject needs a multi-dimension audit: caller supplies dimension charters via args; each dimension gets a finder then a skeptic that confirms/refutes/downgrades every finding.',
  phases: [{ title: 'Audit', detail: 'per-dimension finder → skeptic verification (pipelined)' }],
};

// CONTRACT: findings must carry evidence a skeptic can independently re-open.
const FINDER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['summary', 'findings'],
  properties: {
    summary: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['file', 'severity', 'claim', 'evidence'],
        properties: {
          file: { type: 'string' },
          line: { type: 'number' },
          severity: { enum: ['blocker', 'major', 'minor'] },
          claim: { type: 'string' },
          evidence: { type: 'string' },
          userImpact: { type: 'string' },
          fix: { type: 'string' },
        },
      },
    },
  },
};

const SKEPTIC_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['verdicts'],
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['index', 'verdict'],
        properties: {
          index: { type: 'number' },
          verdict: { enum: ['CONFIRMED', 'REFUTED', 'DOWNGRADED'] },
          severity: { enum: ['blocker', 'major', 'minor'] },
          reason: { type: 'string' },
        },
      },
    },
  },
};

function badArgs(detail) {
  throw new Error(
    'audit-dimensions: bad args (' + detail + '). Expected ' +
      '{ subject: string, dimensions: [{ key: string, charter: string }], ' +
      'bar?: string, root?: string, maxFindingsPerDimension?: positive number }'
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!args || typeof input !== 'object') badArgs('args missing');
if (typeof input.subject !== 'string' || !input.subject.trim()) badArgs('subject must be a non-empty string');
if (!Array.isArray(input.dimensions) || input.dimensions.length === 0) badArgs('dimensions must be a non-empty array');
for (const d of input.dimensions) {
  if (!d || typeof d.key !== 'string' || !d.key.trim()) badArgs('every dimension needs a string key');
  if (typeof d.charter !== 'string' || !d.charter.trim()) badArgs('dimension "' + d.key + '" needs a string charter');
}
if (input.bar !== undefined && typeof input.bar !== 'string') badArgs('bar must be a string when given');
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');
if (
  input.maxFindingsPerDimension !== undefined &&
  (typeof input.maxFindingsPerDimension !== 'number' || !(input.maxFindingsPerDimension > 0))
) {
  badArgs('maxFindingsPerDimension must be a positive number when given');
}

const where = input.root
  ? 'Work in ' + input.root + '.'
  : 'Work in the current repository (the session working directory).';
const barLine = input.bar ? '\nSeverity / judgment bar set by the caller:\n' + input.bar + '\n' : '';
const capLine = input.maxFindingsPerDimension
  ? 'Report at most ' + input.maxFindingsPerDimension + ' findings; prefer the highest-severity, best-evidenced ones.'
  : 'No fixed cap on findings, but prefer the highest-severity, best-evidenced ones over volume.';

phase('Audit');

const findDimension = async (dim) => {
  const found = await agent(
    'You are one auditor in a multi-dimension audit. Audit ONLY your assigned dimension; other dimensions are covered by other auditors.\n\n' +
      'Subject under audit:\n' + input.subject + '\n\n' +
      'Your dimension key: ' + dim.key + '\n' +
      'Your dimension charter (this defines what you look for):\n' + dim.charter + '\n' +
      barLine + '\n' +
      'Rules:\n' +
      '- ' + where + ' Open files and run read-only commands to gather evidence. Do NOT fix anything.\n' +
      '- Every finding must cite concrete evidence (file plus line where applicable, and/or exact command output) that a skeptical reviewer can independently re-open. No evidence, no finding.\n' +
      '- Severity is blocker | major | minor.\n' +
      '- ' + capLine + '\n' +
      '- The summary is 2-4 sentences on the overall health of this dimension.',
    { label: 'find:' + dim.key, phase: 'Audit', schema: FINDER_SCHEMA }
  );
  return { dim, found };
};

const refuteDimension = async (stage) => {
  // CONTRACT: a failed finder stage resolves null — pass it through so the
  // final reconciliation surfaces the dimension as unaudited, never dropped.
  if (!stage || !stage.found) return null;
  const { dim, found } = stage;
  if (!found.findings.length) {
    return { key: dim.key, summary: found.summary, findings: [], refuted: [] };
  }
  const checked = await agent(
    'You are an adversarial skeptic. Another auditor produced the findings below for dimension "' + dim.key +
      '" of this audit subject:\n' + input.subject + '\n\n' +
      'Your job is to try to REFUTE each finding, not to rubber-stamp it. ' + where + '\n\n' +
      'Findings (JSON, judge each by its array index):\n' +
      JSON.stringify(found.findings, null, 2) + '\n\n' +
      'For every index:\n' +
      '- Re-open the cited evidence yourself (open the file at the line, rerun the command). Never trust the finding\'s own wording.\n' +
      '- CONFIRMED: the evidence holds at the stated severity.\n' +
      '- REFUTED: the evidence does not support the claim (wrong file, misread code, stale, fabricated, or the behavior is actually correct). Give the reason.\n' +
      '- DOWNGRADED: real, but overstated — return the corrected severity and the reason.\n' +
      'Return one verdict per index. Read-only; fix nothing.',
    { label: 'refute:' + dim.key, phase: 'Audit', schema: SKEPTIC_SCHEMA, effort: 'high' }
  );

  const byIndex = {};
  for (const v of checked.verdicts) byIndex[v.index] = v;

  const kept = [];
  const refuted = [];
  found.findings.forEach((finding, i) => {
    const v = byIndex[i];
    if (v && v.verdict === 'REFUTED') {
      refuted.push({ claim: finding.claim, file: finding.file, reason: v.reason || 'refuted by skeptic' });
      return;
    }
    if (v && v.verdict === 'DOWNGRADED' && v.severity) {
      kept.push({ ...finding, severity: v.severity });
      return;
    }
    // Skeptic silence is not refutation: an unjudged finding is kept as found.
    kept.push(finding);
  });

  log('audit-dimensions[' + dim.key + ']: ' + kept.length + ' kept, ' + refuted.length + ' refuted');
  return { key: dim.key, summary: found.summary, findings: kept, refuted };
};

const results = await pipeline(input.dimensions, findDimension, refuteDimension);

// CONTRACT: failed stages resolve null — a dead auditor must surface as an
// unaudited dimension with an explicit error, never as silent success.
const byKey = {};
for (const r of results.filter(Boolean)) byKey[r.key] = r;
const dimensions = input.dimensions.map(
  (d) =>
    byKey[d.key] || {
      key: d.key,
      summary: 'auditor agent failed; this dimension was NOT audited',
      findings: [],
      refuted: [],
      error: 'auditor agent failed for this dimension',
    }
);
return { dimensions };
