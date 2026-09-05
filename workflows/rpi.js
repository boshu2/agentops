export const meta = {
  name: 'rpi',
  description:
    'One RPI traversal: Plan shapes one behavior and snapshots intent identity, a premortem judge reads the frozen plan when the write scope is risky, Implement runs one bounded RED->GREEN experiment, then a structurally fresh Validate context judges the exact content and persists verdict.v2. A FAIL or NOT_PROVEN enters a bounded repair phase that stops when converged, stopped by the convergence law, or out of repairRounds. No lifecycle.',
  whenToUse: 'When a caller intent (behavior request or bead text) should be driven through Plan -> Implement -> fresh Validate -> bounded repair, ending in a durable PASS | FAIL | NOT_PROVEN verdict. Not for multi-lane waves (implement-wave) or verdict-only re-checks (verify-fixes).',
  phases: [
    { title: 'Plan', detail: 'shape one active behavior; snapshot exact intent bytes under SHA-256 identity' },
    { title: 'Premortem', detail: 'on a risky write scope only: one fresh judge reads the frozen plan and names what blocks' },
    { title: 'Implement', detail: 'one bounded RED->GREEN experiment strictly inside write scope' },
    { title: 'Validate', detail: 'fresh context re-verifies identity, judges acceptance, persists verdict.v2' },
    { title: 'Repair', detail: 'bounded repair rounds under the convergence law; stop when converged, stopped by the law, or out of budget' },
  ],
};

const PLAN_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['acceptance', 'writeScope', 'intentDigest', 'intentPath'],
  properties: {
    acceptance: { type: 'string' },
    writeScope: { type: 'array', items: { type: 'string' } },
    intentDigest: { type: 'string' },
    intentPath: { type: 'string' },
  },
};

// CONTRACT: no free narrative field beyond filesSummary — and the script never
// forwards filesSummary to the validator. The author's story must not be able
// to reach the judging context.
const IMPLEMENT_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['contextId', 'changedPaths', 'checkReceipts', 'filesSummary'],
  properties: {
    contextId: { type: 'string' },
    changedPaths: { type: 'array', items: { type: 'string' } },
    checkReceipts: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['name', 'command', 'outcome'],
        properties: {
          name: { type: 'string' },
          command: { type: 'string' },
          outcome: { type: 'string' },
        },
      },
    },
    filesSummary: { type: 'string' },
  },
};

// CONTRACT (ADR-0017): findings[].id is the convergence law's key. Ids must be
// STABLE across rounds — the same defect keeps its id however the summary is
// reworded — because "the open set did not grow" and "nothing reopened" are
// meaningless over unstable ids. subjectDigest is the law's progress signal.
const VALIDATE_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['verdict', 'verdictPath', 'criteria', 'validatorContextId', 'subjectDigest', 'findings', 'evidenceRefs', 'derivedChangedPaths'],
  properties: {
    verdict: { enum: ['PASS', 'FAIL', 'NOT_PROVEN'] },
    verdictPath: { type: 'string' },
    subjectDigest: { type: 'string' },
    // CONTRACT (ADR-0017, condition 4): evidence is a BINDING, not a label. An
    // entry admits an unchanged subject only when its subjectDigest equals the
    // round's digest and it names a finding id the round actually closed.
    evidenceRefs: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['ref', 'subjectDigest', 'resolves'],
        properties: {
          ref: { type: 'string' },
          subjectDigest: { type: 'string' },
          resolves: { type: 'array', items: { type: 'string' } },
        },
      },
    },
    // CONTRACT: the validator derives the changed set itself (git status and
    // diff against the clean pre-run tree) so author-reported paths cannot hide
    // an edit from coverage or from risky-surface classification.
    derivedChangedPaths: { type: 'array', items: { type: 'string' } },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'summary'],
        properties: {
          id: { type: 'string' },
          // CONTRACT (ADR-0017): the CLASS is the law's second key. Ids alone
          // cannot see a repair phase that mints a fresh id for the same KIND
          // of defect every round; the class can, and does.
          class: { type: 'string' },
          summary: { type: 'string' },
        },
      },
    },
    criteria: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['criterion', 'result', 'evidence'],
        properties: {
          criterion: { type: 'string' },
          result: { type: 'string' },
          evidence: { type: 'string' },
        },
      },
    },
    validatorContextId: { type: 'string' },
  },
};

// The repair stage returns the same derived facts as Implement: the freshness
// wall is identical, so a repair round's narrative never reaches the validator.
const REPAIR_SCHEMA = IMPLEMENT_SCHEMA;

function badArgs(detail) {
  throw new Error(
    'rpi: bad args (' + detail + '). Expected ' +
      '{ intent: string, root?: string, writeScope?: [string, ...], acceptance?: string, ' +
      "repairRounds?: integer >= 0, validator?: { kind: 'spawned' | 'command', command?: string }, " +
      "crossFamily?: { command: string }, premortem?: 'auto' | 'skip', " +
      "bindingJudge?: 'primary' | 'cross' }"
  );
}

// The harness may deliver args as a JSON-encoded string (see Workflow tool
// docs); normalize before validating so both shapes work.
const input = typeof args === 'string' ? JSON.parse(args) : args;
log('args received as ' + (typeof args) + (input ? ' (normalized ok)' : ' (empty)'));
if (!input || typeof input !== 'object') badArgs('args missing');
if (typeof input.intent !== 'string' || !input.intent.trim()) badArgs('intent must be a non-empty string');
if (input.root !== undefined && typeof input.root !== 'string') badArgs('root must be a string when given');
if (
  input.writeScope !== undefined &&
  (!Array.isArray(input.writeScope) || input.writeScope.length === 0 ||
    input.writeScope.some((s) => typeof s !== 'string' || !s.trim()))
) {
  badArgs('writeScope must be a non-empty array of path globs when given');
}
if (input.acceptance !== undefined && (typeof input.acceptance !== 'string' || !input.acceptance.trim())) {
  badArgs('acceptance must be a non-empty string when given');
}
// CONTRACT (ADR-0017): the repair bound is the CALLER's declaration, not the
// workflow's budget. It is never widened at runtime; 0 reproduces the old
// single-pass behavior exactly.
if (
  input.repairRounds !== undefined &&
  (!Number.isInteger(input.repairRounds) || input.repairRounds < 0)
) {
  badArgs('repairRounds must be a non-negative integer when given');
}
const repairRounds = input.repairRounds === undefined ? 2 : input.repairRounds;
// CONTRACT (ADR-0017): on a risky surface validation is cross-family by default.
// The second leg is an external judge command from ANOTHER model family (LAW 0:
// a headless Claude leg is never legal, so from a Claude session this is a
// read-only `codex exec` command). Absent or same as the primary judge means
// diversity_unsatisfied, and a risky-surface PASS then degrades to NOT_PROVEN.
if (input.crossFamily !== undefined) {
  const cf = input.crossFamily;
  if (typeof cf !== 'object' || cf === null || Array.isArray(cf) || typeof cf.command !== 'string' || !cf.command.trim()) {
    badArgs('crossFamily must be { command: non-empty string } when given');
  }
}
const crossFamilyCommand = input.crossFamily !== undefined ? input.crossFamily.command.trim() : null;
// Explicit list; the skill contract names the same paths. (security-gate.sh
// scans the whole repository, so "anything it scans" is not a usable predicate.)
const RISKY_SURFACE = [
  /^cli\/internal\/gates\//, /^scripts\/check-[^/]*\.sh$/, /^tests\//, /^skills\/[^/]+\/scripts\//,
  /^skills\/cc-hooks\/policies\//, /^lib\//, /^\.github\/workflows\//, /^scripts\/security-gate\.sh$/,
];
const isRiskySurface = (paths) => paths.some((p) => RISKY_SURFACE.some((re) => re.test(p)));
// CONTRACT: a risky write scope buys one premortem judge BEFORE any code is
// written. The 2026-09-03 run shipped a risky-surface design with no premortem
// and paid six repair passes for defects a frozen-plan reading would have named
// first. `premortem: 'skip'` is the caller's explicit waiver, recorded in the
// report so a skipped premortem is never mistaken for a clean one.
if (input.premortem !== undefined && input.premortem !== 'skip' && input.premortem !== 'auto') {
  badArgs("premortem must be 'auto' or 'skip' when given");
}
const premortemMode = input.premortem === undefined ? 'auto' : input.premortem;
// CONTRACT: when two judge legs disagree the tie-break is DECLARED IN ADVANCE
// or decided by a third judge, never named by the caller after seeing which
// leg they preferred. Absent is the honest default: no tie-break was declared.
if (input.bindingJudge !== undefined && input.bindingJudge !== 'primary' && input.bindingJudge !== 'cross') {
  badArgs("bindingJudge must be 'primary' or 'cross' when given");
}
const bindingJudge = input.bindingJudge === undefined ? null : input.bindingJudge;
// CONTRACT: validator selects who judges — the default spawned fresh context,
// or an external judge command brokered by that fresh context. The command is
// opaque caller input; invalid shapes die here, never as a runtime surprise.
if (input.validator !== undefined) {
  const v = input.validator;
  if (typeof v !== 'object' || v === null || Array.isArray(v)) {
    badArgs('validator must be an object when given');
  }
  if (v.kind !== 'spawned' && v.kind !== 'command') {
    badArgs("validator.kind must be 'spawned' or 'command'");
  }
  if (v.command !== undefined && (typeof v.command !== 'string' || !v.command.trim())) {
    badArgs('validator.command must be a non-empty string when given');
  }
  if (v.kind === 'command' && typeof v.command !== 'string') {
    badArgs("validator.kind 'command' requires validator.command");
  }
}
const externalJudge =
  input.validator !== undefined && input.validator.kind === 'command'
    ? input.validator.command.trim()
    : null;

// CONTRACT: cwd pinning is load-bearing. The first cross-vendor smoke FAILed
// because an implement-stage receipt ran `git status` from the session cwd
// instead of the subject repo, and the external judge (correctly) refused the
// inconsistent evidence. Every command in every stage must run from the
// subject root — receipts gathered elsewhere describe the wrong repository.
const where = input.root
  ? 'Work in ' + input.root + '. Before anything else, cd into that exact ' +
    'directory; EVERY command you run — builds, checks, git status, tooling — ' +
    'must execute with it as the working directory. A receipt gathered from ' +
    'any other directory describes the wrong repository and is invalid.'
  : 'Work in the current repository (the session working directory). Every ' +
    'command you run must execute from its root.';

// The product's deterministic tooling does identity and persistence; agents
// drive it, never hand-roll it. Every stage prompt carries the same locator.
// CONTRACT: the skill-root example below is split mid-token so the source file
// carries no vendor-name substring (the script is vendor-agnostic; the judge
// command is opaque caller input) while the constructed prompt bytes remain
// identical to the pre-validator-field default path.
const toolingBlock =
  'Deterministic tooling (drive it, never hand-roll identity or persistence): ' +
  'the installed validate skill ships scripts/validate.py with subcommands ' +
  'snapshot-intent, manifest, verify-manifest, digest, store-verdict. Locate it at the installed ' +
  'skill root (e.g. ~/.cl' + 'aude/skills/validate/scripts/validate.py) or at ' +
  'skills/validate/scripts/validate.py in a checkout. If you cannot find or run it, do not ' +
  'improvise a substitute: report the stage as failed with the real error text.';

phase('Plan');

const plan = await agent(
  'You are the Plan stage of one RPI traversal (Plan -> Implement -> fresh Validate -> bounded repair -> report).\n\n' +
    'Caller intent (behavior request or bead text):\n' + input.intent + '\n\n' +
    (input.acceptance
      ? 'The caller fixed the acceptance criteria — adopt them verbatim as your acceptance output:\n' + input.acceptance + '\n\n'
      : '') +
    (input.writeScope
      ? 'The caller fixed the write scope — adopt these globs verbatim as your writeScope output:\n' +
        input.writeScope.map((s) => '  - ' + s).join('\n') + '\n\n'
      : '') +
    'Your job:\n' +
    '- ' + where + '\n' +
    '- Shape ONE active behavior from the intent: read whatever you need to understand the subject, ' +
    'then produce acceptance criteria (normal case AND edge case, each independently checkable) ' +
    (input.acceptance ? '(caller-fixed above) ' : '') +
    'and a write scope of path globs the implementation may touch ' +
    (input.writeScope ? '(caller-fixed above).\n' : '(as tight as honestly possible).\n') +
    '- Snapshot the EXACT intent bytes: write the caller intent text verbatim to a scratch file and run the ' +
    'validate tooling\'s snapshot-intent subcommand on it, so the bytes persist under their SHA-256 content ' +
    'identity. Return the digest as intentDigest and the persisted snapshot path as intentPath.\n' +
    '- ' + toolingBlock + '\n' +
    '- Read-only otherwise: the intent snapshot is the only thing you persist. Do not implement, do not edit ' +
    'the subject, do not commit.',
  { label: 'plan', phase: 'Plan', schema: PLAN_SCHEMA }
);

// CONTRACT: a failed stage degrades to NOT_PROVEN — never to a retry.
if (!plan) {
  return {
    verdict: 'NOT_PROVEN',
    verdictPath: null,
    intentDigest: null,
    changedPaths: [],
    criteria: [],
    error: 'Plan stage failed; no acceptance or intent identity exists, so nothing can be proven',
  };
}

// CONTRACT: caller-fixed acceptance/writeScope are pinned by the script, not
// by trusting the Plan agent to have echoed them verbatim.
const acceptance = input.acceptance !== undefined ? input.acceptance : plan.acceptance;
const writeScope = input.writeScope !== undefined ? input.writeScope : plan.writeScope;

// PREMORTEM LEG: one fresh judge reads the FROZEN plan on a risky write scope
// and names only what blocks. Blocking findings end the traversal as
// NOT_PLANNED. The cheapest place to kill a bad design is before Implement,
// not in the sixth repair round.
const PREMORTEM_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['blocking', 'notes'],
  properties: {
    blocking: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'summary'],
        properties: {
          id: { type: 'string' },
          class: { type: 'string' },
          summary: { type: 'string' },
        },
      },
    },
    notes: { type: 'array', items: { type: 'string' } },
  },
};

// Risk is classified over the DECLARED scope here: no code exists yet, so the
// globs the plan authorized are the only surface there is.
const plannedRisky = isRiskySurface(writeScope);
let premortem = { status: 'not-required', blocking: [], notes: [] };
if (plannedRisky && premortemMode === 'skip') {
  premortem = { status: 'skipped', blocking: [], notes: ["caller declared premortem: 'skip'"] };
} else if (plannedRisky) {
  phase('Premortem');
  const pm = await agent(
    'You are a PREMORTEM judge: one fresh context that did not write the plan below and will not implement ' +
      'it. The plan is FROZEN: you do not edit it, improve it, or propose an alternative. Assume the work ' +
      'shipped exactly as planned and then failed. Name only what BLOCKS.\n\n' +
      'Frozen plan\n' +
      '- intentDigest: ' + plan.intentDigest + '\n' +
      '- intent snapshot path: ' + plan.intentPath + '\n' +
      '- Caller intent:\n' + input.intent + '\n' +
      '- Acceptance the fresh validator will judge against:\n' + acceptance + '\n' +
      '- Write scope (a risky surface: gates, tests, hook policies, shared libraries, skill scripts, or CI):\n' +
      writeScope.map((s) => '  - ' + s).join('\n') + '\n\n' +
      'Procedure:\n' +
      '- ' + where + '\n' +
      '- Read the exact files the write scope names before ruling. A premortem from the plan text alone is a ' +
      'guess, and a guess blocks nothing.\n' +
      '- Return blocking: one entry {id, class, summary} per defect that must be fixed in the PLAN before any ' +
      'code is written: a failure this plan produces on this surface that this acceptance would not catch. ' +
      'Ids are stable keys <surface-slug>:<defect-slug>; class names the KIND so a later repair round cannot ' +
      'rename it. Return an empty list when nothing blocks: a premortem that always blocks is ceremony.\n' +
      '- Return notes: everything that does not block. Notes never stop the traversal.\n' +
      '- Read-only: change nothing, persist nothing, commit nothing.',
    { label: 'premortem', phase: 'Premortem', schema: PREMORTEM_SCHEMA, effort: 'high' }
  );
  if (!pm) {
    return {
      verdict: 'NOT_PROVEN',
      verdictPath: null,
      intentDigest: plan.intentDigest,
      changedPaths: [],
      criteria: [],
      findings: [],
      premortem: { status: 'failed', blocking: [], notes: [] },
      error: 'Premortem stage failed on a risky write scope; no candidate was built and nothing was judged',
    };
  }
  premortem = {
    status: pm.blocking.length > 0 ? 'blocked' : 'clean',
    blocking: pm.blocking,
    notes: pm.notes,
  };
  if (pm.blocking.length > 0) {
    return {
      verdict: 'NOT_PLANNED',
      verdictPath: null,
      intentDigest: plan.intentDigest,
      changedPaths: [],
      criteria: [],
      findings: pm.blocking,
      premortem,
      error: 'Premortem named blocking findings on a risky write scope; Implement was never dispatched',
    };
  }
}

phase('Implement');

const impl = await agent(
  'You are the Implement stage of one RPI traversal (Plan -> Implement -> fresh Validate -> bounded repair -> report). ' +
    'A separate fresh context will judge your work against the acceptance below using only derived facts — ' +
    'it will never see your narrative, so evidence lives in receipts, not prose.\n\n' +
    'Caller intent:\n' + input.intent + '\n\n' +
    'Acceptance — the contract the fresh validator will judge against:\n' + acceptance + '\n\n' +
    'Write scope — you may create or edit ONLY paths matching:\n' +
    writeScope.map((s) => '  - ' + s).join('\n') + '\n\n' +
    'Process rules (non-negotiable):\n' +
    '- ' + where + '\n' +
    '- One bounded RED -> GREEN experiment: before editing, reproduce the failing behavior (test, command, ' +
    'or observation); after editing, rerun the same repro and the repository\'s real checks (its own test/build/lint ' +
    'commands, not ad-hoc substitutes). Record each as a checkReceipts entry {name, command, outcome} with the ' +
    'exact command and its observed outcome.\n' +
    '- Stay strictly inside the write scope. If the work seems to require touching any path outside it, do NOT ' +
    'edit it — finish what you can inside scope and record the unmet need as a checkReceipts entry whose outcome ' +
    'names the out-of-scope path.\n' +
    '- Generate your own author context id: read random bytes via bash (e.g. from /dev/urandom, hex-encoded) and ' +
    'return it as contextId. The validator asserts its own id is distinct from yours.\n' +
    '- List every path you changed under changedPaths.\n' +
    '- filesSummary is a short human-facing note for the caller only; the validator never receives it.\n' +
    '- No lifecycle: do not commit, push, tag, or close anything. Leave changes in the working tree.',
  { label: 'implement', phase: 'Implement', schema: IMPLEMENT_SCHEMA }
);

if (!impl) {
  return {
    verdict: 'NOT_PROVEN',
    verdictPath: null,
    intentDigest: plan.intentDigest,
    changedPaths: [],
    criteria: [],
    error: 'Implement stage failed; no candidate exists to judge',
  };
}

// ORPHANED-EVIDENCE RECEIPT: every harness edit orphans the evidence bound to
// the old harness, and on 2026-09-03 nobody knew until a later verify failed.
// This is a receipt, not a gate: it reports what the repository's own detector
// says about the changed paths, and says plainly when there is no detector.
const ORPHANS_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['scriptPresent'],
  properties: {
    scriptPresent: { type: 'boolean' },
    json: { type: 'string' },
    error: { type: 'string' },
  },
};

const orphans = await agent(
  'You are a read-only receipt step after Implement. Do exactly one thing: report whether this repository ' +
    'ships scripts/evidence-orphans.sh and, if it does, what it says about the changed paths below. You judge ' +
    'nothing.\n\n' +
    'Changed paths:\n' +
    (impl.changedPaths.length ? impl.changedPaths.map((p) => '  - ' + p).join('\n') : '  (none reported)') + '\n\n' +
    'Procedure:\n' +
    '- ' + where + '\n' +
    '- Test for scripts/evidence-orphans.sh. If it is absent, return scriptPresent false and nothing else. ' +
    'Never substitute another script, never write one, and never approximate its output yourself.\n' +
    '- If it is present, run it ONCE with the changed paths above as its arguments and capture its stdout ' +
    'verbatim. Return that raw text as json, unedited.\n' +
    '- If it exits nonzero or prints nothing, return scriptPresent true with the real error text as error and ' +
    'omit json. An invented result is worse than no result.\n' +
    '- Read-only: fix nothing, create nothing, commit nothing.',
  { label: 'orphans', phase: 'Implement', schema: ORPHANS_SCHEMA }
);

let orphanedEvidence = null;
let orphanedEvidenceReason = null;
if (!orphans) {
  orphanedEvidenceReason = 'receipt-leg-failed';
} else if (!orphans.scriptPresent) {
  orphanedEvidenceReason = 'script-absent';
} else if (typeof orphans.json === 'string' && orphans.json.trim()) {
  try {
    orphanedEvidence = JSON.parse(orphans.json);
  } catch (err) {
    orphanedEvidenceReason = 'unparsable-output: ' + err.message;
  }
} else {
  orphanedEvidenceReason = orphans.error ? 'script-failed: ' + orphans.error : 'no-output';
}

phase('Validate');

// FRESHNESS WALL: this prompt is built ONLY from Plan outputs plus the
// implement stage's derived facts (changedPaths, checkReceipts, contextId).
// The author's narrative (filesSummary or anything else) must never cross —
// the context that authors a candidate cannot issue its binding PASS. The
// wall is identical in both modes: in external-judge mode the same fresh
// context brokers the same packet to the caller-supplied command and
// transcribes its ruling without interpretation.

// CONTRACT (ADR-0017): validators return the subject-manifest digest and the open
// findings under STABLE ids so the repair phase can apply the convergence law.
const FINDINGS_BLOCK =
  '\n- Return subjectDigest: the manifest digest you computed over the changed paths.\n' +
  '- Return findings: one entry {id, class, summary} per open defect that blocks PASS (empty on PASS). Ids are ' +
  'STABLE keys of the form <criterion-slug>:<defect-slug> so the same defect keeps its id across rounds ' +
  'however you reword the summary; never mint a new id for a defect you already named.\n' +
  '- class names the KIND of defect ("seal.pinning", "scope.coverage"), stable across rounds and shared by ' +
  'every finding of that kind. It is how the convergence law sees a repair phase that keeps renaming one ' +
  'defect: a NEW id carrying a class an earlier round closed stops the run. Omit it only when the defect ' +
  'genuinely belongs to no kind you can name.\n' +
  '- Return evidenceRefs as bindings {ref, subjectDigest, resolves}: ref is the receipt/transcript/verdict path, ' +
  'subjectDigest is the manifest digest it was produced against, resolves lists the finding ids it closes (empty if none).\n' +
  '- Return derivedChangedPaths: derive the changed set YOURSELF from the working tree (git status --porcelain and ' +
  'git diff --name-only against HEAD, including untracked files); do not copy the author list. Any path you derive that ' +
  'the author did not report is a coverage finding (id coverage:unreported:<path>).';

async function spawnedLeg(facts) {
  return await agent(
  'You are a fresh, independent Validate context. You did not author the candidate and you have not seen ' +
    'the author\'s reasoning — only the facts below. Judge the exact content; the author\'s claims do not exist ' +
    'for you.\n\n' +
    'Intent identity:\n' +
    '- intentDigest: ' + plan.intentDigest + '\n' +
    '- intent snapshot path: ' + plan.intentPath + '\n\n' +
    'Acceptance criteria to judge:\n' + acceptance + '\n\n' +
    'Declared write scope:\n' + writeScope.map((s) => '  - ' + s).join('\n') + '\n\n' +
    'Author context id: ' + facts.contextId + '\n\n' +
    'Changed paths (derived, judge these exact files):\n' +
    (facts.changedPaths.length ? facts.changedPaths.map((p) => '  - ' + p).join('\n') : '  (none reported)') + '\n\n' +
    'Check receipts (factual command outcomes, re-derivable — rerun them yourself):\n' +
    JSON.stringify(facts.checkReceipts, null, 2) + '\n\n' +
    'Procedure:\n' +
    '- ' + where + '\n' +
    '- ' + toolingBlock + '\n' +
    '- Verify intent identity: read the snapshot at the path above and confirm its bytes hash to intentDigest ' +
    '(the tooling\'s digest subcommand). A mismatch or missing snapshot is NOT_PROVEN.\n' +
    '- Compute the subject manifest over the changed paths with the tooling\'s manifest subcommand, so the ' +
    'verdict binds to exact content.\n' +
    '- Judge EVERY acceptance criterion with your own fresh evidence: open the files, rerun the receipts\' ' +
    'commands and the repository\'s real checks. Record each as a criteria entry {criterion, result, evidence}.\n' +
    '- Coverage: every changed path must fall inside the declared write scope and be accounted for by the ' +
    'acceptance. A PROVEN out-of-scope change is FAIL. Empty checked scope, missing identity, or coverage you ' +
    'could not establish is NOT_PROVEN.\n' +
    '- Generate your own validator context id (random bytes via bash, hex-encoded) and assert it is distinct ' +
    'from the author context id above; if you cannot assert distinctness, the verdict is NOT_PROVEN.\n' +
    '- PASS only when: the intent digest verifies, checked scope is nonempty, changed-path coverage is complete ' +
    'inside write scope, and every criterion has evidence.\n' +
    '- Persist the verdict as verdict.v2 via the tooling\'s store-verdict subcommand — read its --help for the ' +
    'exact flags and input shape rather than guessing. Record BOTH context ids and an explicit freshness ' +
    'attestation (you judged from a fresh context, not the authoring one). Return the persisted path as ' +
    'verdictPath.\n' +
    '- Read-only over the subject: fix nothing, commit nothing. The verdict file is the only thing you persist.\n' +
    '- If the tooling is absent or persistence fails, the verdict is NOT_PROVEN with the real error in the ' +
    'evidence of a criteria entry.' + FINDINGS_BLOCK,
  { label: 'validate', phase: 'Validate', schema: VALIDATE_SCHEMA, effort: 'high' }
);
}

async function brokerLeg(facts, command) {
  return await agent(
  'You are a fresh, independent Validate context acting as the BROKER for an external judge. You did not ' +
    'author the candidate and you have not seen the author\'s reasoning — only the facts below. You do NOT ' +
    'judge the acceptance yourself: the external judge command below rules, and you transcribe its ruling ' +
    'faithfully.\n\n' +
    'External judge command (opaque caller input — assume nothing about which tool it is):\n' +
    '  ' + command + '\n\n' +
    'Evidence packet for the judge (these facts and NOTHING more may reach it — the freshness wall is ' +
    'unchanged):\n' +
    '- intentDigest: ' + plan.intentDigest + '\n' +
    '- intent snapshot path: ' + plan.intentPath + '\n' +
    '- Acceptance criteria to judge:\n' + acceptance + '\n' +
    '- Declared write scope:\n' + writeScope.map((s) => '  - ' + s).join('\n') + '\n' +
    '- Author context id: ' + facts.contextId + '\n' +
    '- Changed paths (derived):\n' +
    (facts.changedPaths.length ? facts.changedPaths.map((p) => '  - ' + p).join('\n') : '  (none reported)') + '\n' +
    '- Check receipts (factual command outcomes):\n' + JSON.stringify(facts.checkReceipts, null, 2) + '\n\n' +
    'Procedure:\n' +
    '- ' + where + '\n' +
    '- ' + toolingBlock + '\n' +
    '- Learn the judge command\'s invocation shape by reading its --help output, then invoke it once. Pass it ' +
    'a validator charter (judge the changed content against the acceptance criteria; rule exactly one of ' +
    'PASS, FAIL, or NOT_PROVEN with per-criterion findings) plus the evidence packet above, via the ' +
    'command\'s supported input mechanism. Capture its RAW stdout and stderr, unedited, to a transcript file ' +
    'under the run\'s .agents/ao/ evidence area.\n' +
    '- No verdict laundering: the persisted verdict field must be EXACTLY the external judge\'s ruling. If ' +
    'the raw output does not contain an unambiguous PASS, FAIL, or NOT_PROVEN ruling, the verdict is ' +
    'NOT_PROVEN with the parse problem named in the evidence — you never interpret an ambiguous ruling into ' +
    'a verdict.\n' +
    '- No silent fallback: if the judge command is absent, non-executable, or exits without producing ' +
    'output, the verdict is NOT_PROVEN naming the command failure. You never judge the acceptance yourself ' +
    'in its place and never substitute any other judge.\n' +
    '- External validator identity: validatorContextId is the judge\'s own run/session identity when its ' +
    'output provides one, otherwise the SHA-256 of the raw transcript file (the tooling\'s digest ' +
    'subcommand).\n' +
    '- Attester identity: generate your own broker context id (random bytes via bash, hex-encoded) and ' +
    'record it in the verdict attestation as the attester — distinct from BOTH the author context id above ' +
    'and the external validator id.\n' +
    '- Transcribe the criteria list from the external judge\'s ruling as criteria entries ' +
    '{criterion, result, evidence}.\n' +
    '- Persist the verdict as verdict.v2 via the tooling\'s store-verdict subcommand — read its --help for ' +
    'the exact flags and input shape rather than guessing — and record the raw transcript path in ' +
    'evidence_refs so the ruling stays auditable. Return the persisted path as verdictPath.\n' +
    '- Read-only over the subject: fix nothing, commit nothing. The transcript and the verdict file are the ' +
    'only things you persist.\n' +
    '- If the tooling is absent or persistence fails, the verdict is NOT_PROVEN with the real error in the ' +
    'evidence of a criteria entry.' + FINDINGS_BLOCK,
  { label: 'validate', phase: 'Validate', schema: VALIDATE_SCHEMA, effort: 'high' }
);
}

// COUNCIL LEG: a third fresh judge, convened only on an undeclared split over a
// risky surface. It rules BETWEEN the two legs: it re-reads the subject and
// picks one of the two verdicts already on the table, never a third, so a
// disagreement is resolved by a judge, not by whichever leg the caller liked.
const councilSchema = (legs) => ({
  type: 'object',
  additionalProperties: false,
  required: ['verdict', 'reason'],
  properties: {
    verdict: { enum: [...new Set(legs.map((l) => l.result.verdict))] },
    reason: { type: 'string' },
  },
});

async function councilLeg(legs) {
  return await agent(
    'You are the COUNCIL: a third fresh, independent judge convened because two validator legs ruled ' +
      'differently on the same subject. You did not author the candidate and you did not write either ruling.\n\n' +
      'The rulings on the table:\n' +
      legs
        .map(
          (l) =>
            '- ' + l.family + ' judged ' + l.result.verdict + ' with findings:\n' +
            ((l.result.findings || []).length
              ? (l.result.findings || []).map((f) => '    - ' + f.id + ': ' + f.summary).join('\n')
              : '    (none)')
        )
        .join('\n') + '\n\n' +
      'Subject digest both legs judged: ' + (legs[0].result.subjectDigest || '(none)') + '\n\n' +
      'Acceptance criteria:\n' + acceptance + '\n\n' +
      'Procedure:\n' +
      '- ' + where + '\n' +
      '- Re-derive the disputed facts YOURSELF: open the files the findings name and rerun the checks. ' +
      'Neither leg\'s reasoning is evidence for you.\n' +
      '- Return verdict: exactly ONE of the two verdicts above. You do not mint a third verdict, and you do ' +
      'not average them. If the disputed findings reproduce, the leg that named them was right.\n' +
      '- Return reason: one plain statement of what you re-derived and which leg it supports.\n' +
      '- Read-only: fix nothing, persist nothing, commit nothing.',
    { label: 'council', phase: 'Validate', schema: councilSchema(legs), effort: 'high' }
  );
}

// Worst-of ordering for merged legs: a FAIL anywhere is a FAIL; a NOT_PROVEN
// anywhere without a FAIL is NOT_PROVEN; PASS needs every leg to PASS.
const VERDICT_RANK = { PASS: 0, NOT_PROVEN: 1, FAIL: 2 };
const dissentOf = (leg) => ({
  family: leg.family,
  verdict: leg.result.verdict,
  findings: (leg.result.findings || []).map((f) => ({ id: f.id, class: f.class, summary: f.summary })),
});

// A SPLIT is a disagreement on the VERDICT, not merely on which findings each
// leg named. Resolution is declared (bindingJudge) or judged (council); it is
// never invented here, and it never silently becomes PASS.
async function mergeLegs(allLegs, risky, diversity, council) {
  let legs = allLegs;
  let dissent = null;
  let councilRecord = null;
  let tieBreak = null;
  const split = allLegs.length === 2 && allLegs[0].result.verdict !== allLegs[1].result.verdict;
  if (split && bindingJudge !== null) {
    const index = bindingJudge === 'primary' ? 0 : 1;
    legs = [allLegs[index]];
    dissent = [dissentOf(allLegs[1 - index])];
    tieBreak = 'bindingJudge:' + bindingJudge;
  } else if (split && risky) {
    const ruling = await council(allLegs);
    if (!ruling) {
      tieBreak = 'council-unavailable';
    } else {
      councilRecord = {
        verdict: ruling.verdict,
        reason: ruling.reason,
        legs: allLegs.map((l) => ({ family: l.family, verdict: l.result.verdict })),
      };
      const ruled = allLegs.filter((l) => l.result.verdict === ruling.verdict);
      // The council picks one of the two verdicts on the table; a ruling that
      // matches neither leg is a broken council, not a third opinion.
      if (ruled.length > 0) {
        legs = ruled;
        const others = allLegs.filter((l) => l.result.verdict !== ruling.verdict);
        dissent = others.map(dissentOf);
        tieBreak = 'council';
      } else {
        tieBreak = 'council-invalid-ruling';
      }
    }
  } else if (split) {
    // Non-risky split: worst-of still stands, so a single-family FAIL stands.
    tieBreak = 'worst-of';
  }
  let verdict = 'PASS';
  const findings = new Map();
  const evidence = new Set();
  const criteria = [];
  let digest = null;
  let digestConflict = false;
  for (const leg of legs) {
    const r = leg.result;
    if (VERDICT_RANK[r.verdict] > VERDICT_RANK[verdict]) verdict = r.verdict;
    for (const f of r.findings || []) findings.set(f.id, { id: f.id, class: f.class, summary: f.summary, family: leg.family });
    for (const e of r.evidenceRefs || []) evidence.add(JSON.stringify(e));
    for (const c of r.criteria || []) criteria.push(c);
    if (digest === null) digest = r.subjectDigest;
    else if (r.subjectDigest !== digest) digestConflict = true;
  }
  // Identity is asserted over EVERY leg, including a dissenting one: two judges
  // that measured different bytes never proved anything, whoever was binding.
  for (const leg of allLegs) {
    if (digest !== null && leg.result.subjectDigest !== digest) digestConflict = true;
  }
  if (tieBreak === 'council-unavailable' || tieBreak === 'council-invalid-ruling') {
    verdict = 'NOT_PROVEN';
    const id = 'council:' + (tieBreak === 'council-unavailable' ? 'unavailable' : 'invalid-ruling');
    findings.set(id, {
      id,
      class: 'council.tie-break',
      summary: tieBreak === 'council-unavailable'
        ? 'validator legs split on a risky surface and the council judge did not rule'
        : 'the council returned a verdict neither leg had ruled',
      family: 'merge',
    });
    for (const leg of allLegs) for (const f of leg.result.findings || []) {
      if (!findings.has(f.id)) findings.set(f.id, { id: f.id, class: f.class, summary: f.summary, family: leg.family });
    }
  }
  if (digestConflict) {
    verdict = 'NOT_PROVEN';
    findings.set('identity:digest-disagreement', { id: 'identity:digest-disagreement', summary: 'validator legs computed different subject digests', family: 'merge' });
  }
  if (verdict === 'PASS' && findings.size > 0) {
    // A PASS that names open defects is malformed; it cannot certify.
    verdict = 'NOT_PROVEN';
  }
  if (risky && diversity !== 'satisfied' && verdict === 'PASS') {
    verdict = 'NOT_PROVEN';
    findings.set('diversity:unsatisfied', { id: 'diversity:unsatisfied', summary: 'risky surface judged by one model family only; no authorized cross-family leg', family: 'merge' });
  }
  // Coverage is derived from EVERY leg: a dissenting leg's derived path is
  // still a changed path, and a tie-break must never shrink the checked set.
  const derived = new Set();
  for (const leg of allLegs) for (const p of leg.result.derivedChangedPaths || []) derived.add(p);
  // A persisted verdict path is exposed only when every leg, dissent included,
  // agrees with the aggregate; a PASS file behind a FAIL/NOT_PROVEN result, or
  // behind a resolved split, would launder it.
  const allAgree = allLegs.every((l) => l.result.verdict === verdict);
  const persistedMatches = allAgree && legs[0].result.verdict === verdict;
  return {
    verdict,
    verdictPath: persistedMatches ? legs[0].result.verdictPath : null,
    legVerdictPaths: allLegs.map((l) => ({ family: l.family, verdict: l.result.verdict, verdictPath: l.result.verdictPath })),
    validatorContextId: allLegs.map((l) => l.result.validatorContextId).join('+'),
    subjectDigest: digest,
    findings: [...findings.values()],
    evidenceRefs: [...evidence].map((e) => JSON.parse(e)),
    derivedChangedPaths: [...derived],
    criteria,
    families: allLegs.map((l) => l.family),
    risky,
    diversity,
    tieBreak,
    dissent,
    council: councilRecord,
  };
}

// Family distinctness is asserted by the CALLER's choice of crossFamily.command
// (a different vendor's read-only CLI); this script cannot verify a model's
// identity and does not pretend to. From a Claude session the legal second leg
// is a read-only `codex exec` (LAW 0: never a headless Claude leg).
async function validateOnce(facts) {
  const legs = [];
  const primary = externalJudge === null ? await spawnedLeg(facts) : await brokerLeg(facts, externalJudge);
  if (!primary) return null;
  legs.push({ family: externalJudge === null ? 'spawned' : 'external', result: primary });
  // Risk is classified over the union of author-reported and validator-derived
  // paths, so an unreported edit on a gate or test cannot dodge cross-family.
  const allPaths = new Set([...facts.changedPaths, ...(primary.derivedChangedPaths || [])]);
  const risky = isRiskySurface([...allPaths]);
  const elected = crossFamilyCommand !== null && crossFamilyCommand !== externalJudge;
  let diversity = risky ? 'unsatisfied' : 'not-required';
  if (elected) {
    const second = await brokerLeg(facts, crossFamilyCommand);
    if (!second) return null;
    legs.push({ family: 'cross-family', result: second });
    diversity = risky ? 'satisfied' : 'elected';
  }
  const merged = await mergeLegs(legs, risky, diversity, councilLeg);
  const unreported = merged.derivedChangedPaths.filter((p) => !facts.changedPaths.includes(p));
  if (unreported.length > 0) {
    const byId = new Map(merged.findings.map((f) => [f.id, f]));
    for (const p of unreported) {
      const id = 'coverage:unreported:' + p;
      if (!byId.has(id)) byId.set(id, { id, summary: 'changed path not reported by the author', family: 'merge' });
    }
    merged.findings = [...byId.values()];
    if (merged.verdict === 'PASS') return degrade(merged, 'NOT_PROVEN');
  }
  return merged;
}

// CONTRACT: every aggregate verdict mutation goes through here so a persisted
// verdict file whose ruling differs from the reported verdict is never exposed.
function degrade(result, verdict) {
  if (result.verdict === verdict) return result;
  return { ...result, verdict, verdictPath: null };
}

let validation = await validateOnce(impl);

if (!validation) {
  return {
    verdict: 'NOT_PROVEN',
    verdictPath: null,
    intentDigest: plan.intentDigest,
    changedPaths: impl.changedPaths,
    filesSummary: impl.filesSummary,
    criteria: [],
    premortem,
    orphanedEvidence,
    orphanedEvidenceReason,
    error: 'Validate stage failed; the candidate exists but was never freshly judged',
  };
}

// REPAIR PHASE (ADR-0017): bounded by the caller's repairRounds and stopped by
// the convergence law. The fix step is an Implement-shaped agent that sees the
// open findings; every judge leg stays non-mutating. Law, per round:
//   1. roundsUsed < repairRounds
//   2. open finding set (stable ids) is not larger than the previous round's
//   3. no id closed in an earlier round reopens
//   4. the subject digest changed, or the round resolved NOT_PROVEN with new evidence
//   3b. no CLASS closed in an earlier round is reopened by a new id
const openIds = (v) => new Set((v.findings || []).map((f) => f.id));
const findingClass = (f) => (typeof f.class === 'string' && f.class.trim() ? f.class.trim() : null);
const repairLog = [];
const closedIds = new Set();
// class -> the finding id whose closure retired it. A class is retired only
// once no open finding still carries it, so a surviving sibling never makes
// its own class a violation.
const closedClasses = new Map();
let facts = { contextId: impl.contextId, changedPaths: impl.changedPaths.slice(), checkReceipts: impl.checkReceipts };
let roundsUsed = 0;
let stoppedBy = null;
// A diversity gap is not a defect in the subject and cannot be repaired by
// editing it. Real findings on a risky surface still enter repair (the
// contract: FAIL with findings repairs); only when the remaining findings are
// diversity-only does the traversal stop as NOT_PROVEN (diversity_unsatisfied)
// without spending a repair round.
const repairable = (v) => (v.findings || []).some((f) => !String(f.id).startsWith('diversity:'));
const diversityOnly = (v) => v && v.risky && v.diversity === 'unsatisfied' && !repairable(v);
if (diversityOnly(validation)) {
  stoppedBy = 'diversity_unsatisfied: risky surface, no authorized cross-family leg';
}
while (
  stoppedBy === null &&
  validation &&
  (validation.verdict === 'FAIL' || validation.verdict === 'NOT_PROVEN') &&
  repairable(validation)
) {
  if (roundsUsed >= repairRounds) { stoppedBy = 'out of repair_rounds (' + repairRounds + ')'; break; }
  phase('Repair');
  const prevIds = openIds(validation);
  const prevDigest = validation.subjectDigest;
  const repair = await agent(
    'You are the Repair stage of one RPI traversal, round ' + (roundsUsed + 1) + ' of at most ' + repairRounds +
      '. A fresh validator judged the current candidate and returned ' + validation.verdict +
      ' with these open findings; fix exactly these, nothing else, inside the write scope.\n\n' +
      'Open findings (stable ids, class in brackets):\n' +
      (validation.findings || [])
        .map((f) => '  - ' + f.id + (findingClass(f) ? ' [' + findingClass(f) + ']' : '') + ': ' + f.summary)
        .join('\n') + '\n\n' +
      'Acceptance the validator judges against:\n' + acceptance + '\n\n' +
      'Write scope — you may create or edit ONLY paths matching:\n' +
      writeScope.map((s) => '  - ' + s).join('\n') + '\n\n' +
      'Paths changed so far:\n' + facts.changedPaths.map((p) => '  - ' + p).join('\n') + '\n\n' +
      'Process rules (non-negotiable):\n' +
      '- ' + where + '\n' +
      '- Repair the named findings; rerun the repository\'s real checks and record each as a checkReceipts ' +
      'entry {name, command, outcome}. Do not weaken tests, gates, or acceptance to obtain green.\n' +
      '- Stay strictly inside the write scope; record any unmet out-of-scope need as a receipt, never an edit.\n' +
      '- Generate your own author context id (random bytes via bash, hex-encoded) and return it as contextId.\n' +
      '- List every path you changed under changedPaths. filesSummary is for the caller only.\n' +
      '- No lifecycle: do not commit, push, tag, or close anything.',
    { label: 'repair:' + (roundsUsed + 1), phase: 'Repair', schema: REPAIR_SCHEMA }
  );
  roundsUsed += 1;
  if (!repair) {
    // A failed repair may have mutated the subject: no stale verdict identity survives it.
    return {
      verdict: 'NOT_PROVEN', verdictPath: null, intentDigest: plan.intentDigest,
      changedPaths: facts.changedPaths, filesSummary: impl.filesSummary, criteria: [],
      findings: validation.findings || [], converged: false, repairRoundsUsed: roundsUsed,
      repairLog: repairLog.concat(['repair round ' + roundsUsed + ': repair stage failed']),
      stoppedBy: 'repair stage failed in round ' + roundsUsed, subjectDigest: null,
      premortem, orphanedEvidence, orphanedEvidenceReason,
      error: 'Repair stage failed after the subject may have changed; the prior verdict no longer binds',
    };
  }
  const union = new Set([...facts.changedPaths, ...(validation.derivedChangedPaths || [])]);
  for (const p of repair.changedPaths) union.add(p);
  facts = { contextId: repair.contextId, changedPaths: [...union], checkReceipts: repair.checkReceipts };
  const next = await validateOnce(facts);
  if (!next) {
    return {
      verdict: 'NOT_PROVEN', verdictPath: null, intentDigest: plan.intentDigest,
      changedPaths: facts.changedPaths, filesSummary: impl.filesSummary, criteria: [],
      findings: validation.findings || [], converged: false, repairRoundsUsed: roundsUsed,
      repairLog: repairLog.concat(['repair round ' + roundsUsed + ': validate stage failed']),
      stoppedBy: 'validate stage failed in round ' + roundsUsed, subjectDigest: null,
      premortem, orphanedEvidence, orphanedEvidenceReason,
      error: 'Validate stage failed after a repair; the repaired candidate was never freshly judged',
    };
  }
  const nextIds = openIds(next);
  const resolved = [...prevIds].filter((id) => !nextIds.has(id));
  const prevEvidence = new Set((validation.evidenceRefs || []).map((e) => e.ref));
  const resolvedSet = new Set(resolved);
  const bindingEvidence = (next.evidenceRefs || []).filter(
    (e) => !prevEvidence.has(e.ref) && e.subjectDigest === next.subjectDigest && (e.resolves || []).some((id) => resolvedSet.has(id))
  );
  let violation = null;
  let classStop = false;
  const reopened = [...nextIds].filter((id) => closedIds.has(id));
  // Condition 3b: a NEW id carrying a class an earlier round closed. Counting
  // ids alone cannot see it, and that is exactly how the 2026-09-03 run kept a
  // flat open set for three rounds while the same class stayed unenforced.
  const classReopened = (next.findings || [])
    .filter((f) => !prevIds.has(f.id) && findingClass(f) !== null && closedClasses.has(findingClass(f)))
    .map((f) => findingClass(f) + ': ' + f.id + ' reopens the class closed by ' + closedClasses.get(findingClass(f)));
  // Most specific regression first: a reopened id, then a reopened class, then
  // the generic grown set.
  if (reopened.length > 0) violation = 'a closed finding reopened (' + reopened.join(', ') + ')';
  else if (classReopened.length > 0) {
    violation = 'a closed finding class reopened (' + classReopened.join('; ') + ')';
    classStop = true;
  } else if (nextIds.size > prevIds.size) violation = 'open finding set grew (' + prevIds.size + ' -> ' + nextIds.size + ')';
  else if (next.subjectDigest === prevDigest) {
    // Condition 4: unchanged bytes are admitted only when a NOT_PROVEN round
    // was resolved by new evidence that closed at least one finding; a FAIL is
    // never rescued by evidence, and a PASS over unchanged bytes is a flip.
    const evidenceResolved =
      validation.verdict === 'NOT_PROVEN' && next.verdict !== 'FAIL' && bindingEvidence.length > 0;
    if (!evidenceResolved) violation = next.verdict === 'PASS'
      ? 'verdict flipped to PASS over an unchanged subject'
      : 'subject digest unchanged without new resolving evidence';
  }
  const stillOpen = new Set((next.findings || []).map(findingClass).filter((c) => c !== null));
  for (const id of resolved) {
    closedIds.add(id);
    const closed = (validation.findings || []).find((f) => f.id === id);
    const cls = closed ? findingClass(closed) : null;
    if (cls !== null && !stillOpen.has(cls) && !closedClasses.has(cls)) closedClasses.set(cls, id);
  }
  repairLog.push('repair round ' + roundsUsed + ': ' + nextIds.size + ' open findings' + (violation ? ' — stopped by the law: ' + violation : ''));
  validation = next;
  if (violation) {
    stoppedBy = 'law: ' + violation;
    // A class reopen means every round named a different id for the same kind
    // of defect, so no round's ruling binds to a converging subject: the honest
    // outcome is NOT_PROVEN, never the churning round's own status.
    if (validation.verdict === 'PASS' || classStop) validation = degrade(validation, 'NOT_PROVEN');
    break;
  }
  if (diversityOnly(validation)) {
    stoppedBy = 'diversity_unsatisfied: risky surface, no authorized cross-family leg';
    break;
  }
}
const converged = !!validation && validation.verdict === 'PASS' && stoppedBy === null;
if (!converged && stoppedBy === null && validation && openIds(validation).size === 0 && validation.verdict !== 'PASS') {
  stoppedBy = 'validator returned ' + validation.verdict + ' with no findings to repair';
}

// Report: script-assembled, no extra agent, no next action. The caller owns
// continuation; repairRounds was the caller's declaration and is never widened.
// filesSummary reaches only this caller-facing report — never the validator.
return {
  verdict: validation.verdict,
  verdictPath: validation.verdictPath,
  intentDigest: plan.intentDigest,
  changedPaths: facts.changedPaths,
  filesSummary: impl.filesSummary,
  criteria: validation.criteria,
  findings: validation.findings || [],
  subjectDigest: validation.subjectDigest || null,
  validatorFamilies: validation.families || [],
  diversity: validation.diversity || 'not-required',
  premortem,
  orphanedEvidence,
  orphanedEvidenceReason,
  tieBreak: validation.tieBreak || null,
  dissent: validation.dissent || null,
  council: validation.council || null,
  converged,
  repairRoundsUsed: roundsUsed,
  repairLog,
  stoppedBy,
};
