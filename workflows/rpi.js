export const meta = {
  name: 'rpi',
  description:
    'One pass of the AgentOps core loop: Plan shapes one behavior and snapshots intent identity, Implement runs one bounded RED->GREEN experiment, then a structurally fresh Validate context judges the exact content and persists verdict.v2. No retry, no revision, no lifecycle.',
  whenToUse: 'When a caller intent (behavior request or bead text) should be driven through Plan -> Implement -> fresh Validate exactly once, ending in a durable PASS | FAIL | NOT_PROVEN verdict. Not for multi-lane waves (implement-wave) or verdict-only re-checks (verify-fixes).',
  phases: [
    { title: 'Plan', detail: 'shape one active behavior; snapshot exact intent bytes under SHA-256 identity' },
    { title: 'Implement', detail: 'one bounded RED->GREEN experiment strictly inside write scope' },
    { title: 'Validate', detail: 'fresh context re-verifies identity, judges acceptance, persists verdict.v2' },
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

const VALIDATE_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['verdict', 'verdictPath', 'criteria', 'validatorContextId'],
  properties: {
    verdict: { enum: ['PASS', 'FAIL', 'NOT_PROVEN'] },
    verdictPath: { type: 'string' },
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

function badArgs(detail) {
  throw new Error(
    'rpi: bad args (' + detail + '). Expected ' +
      '{ intent: string, root?: string, writeScope?: [string, ...], acceptance?: string, ' +
      "validator?: { kind: 'spawned' | 'command', command?: string } }"
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
  'You are the Plan stage of a one-pass Plan -> Implement -> fresh Validate loop.\n\n' +
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

phase('Implement');

const impl = await agent(
  'You are the Implement stage of a one-pass Plan -> Implement -> fresh Validate loop. ' +
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

phase('Validate');

// FRESHNESS WALL: this prompt is built ONLY from Plan outputs plus the
// implement stage's derived facts (changedPaths, checkReceipts, contextId).
// The author's narrative (filesSummary or anything else) must never cross —
// the context that authors a candidate cannot issue its binding PASS. The
// wall is identical in both modes: in external-judge mode the same fresh
// context brokers the same packet to the caller-supplied command and
// transcribes its ruling without interpretation.
let validation;
if (externalJudge === null) {
validation = await agent(
  'You are a fresh, independent Validate context. You did not author the candidate and you have not seen ' +
    'the author\'s reasoning — only the facts below. Judge the exact content; the author\'s claims do not exist ' +
    'for you.\n\n' +
    'Intent identity:\n' +
    '- intentDigest: ' + plan.intentDigest + '\n' +
    '- intent snapshot path: ' + plan.intentPath + '\n\n' +
    'Acceptance criteria to judge:\n' + acceptance + '\n\n' +
    'Declared write scope:\n' + writeScope.map((s) => '  - ' + s).join('\n') + '\n\n' +
    'Author context id: ' + impl.contextId + '\n\n' +
    'Changed paths (derived, judge these exact files):\n' +
    (impl.changedPaths.length ? impl.changedPaths.map((p) => '  - ' + p).join('\n') : '  (none reported)') + '\n\n' +
    'Check receipts (factual command outcomes, re-derivable — rerun them yourself):\n' +
    JSON.stringify(impl.checkReceipts, null, 2) + '\n\n' +
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
    'evidence of a criteria entry.',
  { label: 'validate', phase: 'Validate', schema: VALIDATE_SCHEMA, effort: 'high' }
);
} else {
// CONTRACT (external judge mode): the fresh context is a BROKER, not the
// judge. Load-bearing honesty rules: no verdict laundering (the persisted
// verdict is exactly the external ruling; ambiguity degrades to NOT_PROVEN),
// no silent fallback (a dead command is NOT_PROVEN, never re-judged in-run),
// and the broker attests under its own id, distinct from author and judge.
validation = await agent(
  'You are a fresh, independent Validate context acting as the BROKER for an external judge. You did not ' +
    'author the candidate and you have not seen the author\'s reasoning — only the facts below. You do NOT ' +
    'judge the acceptance yourself: the external judge command below rules, and you transcribe its ruling ' +
    'faithfully.\n\n' +
    'External judge command (opaque caller input — assume nothing about which tool it is):\n' +
    '  ' + externalJudge + '\n\n' +
    'Evidence packet for the judge (these facts and NOTHING more may reach it — the freshness wall is ' +
    'unchanged):\n' +
    '- intentDigest: ' + plan.intentDigest + '\n' +
    '- intent snapshot path: ' + plan.intentPath + '\n' +
    '- Acceptance criteria to judge:\n' + acceptance + '\n' +
    '- Declared write scope:\n' + writeScope.map((s) => '  - ' + s).join('\n') + '\n' +
    '- Author context id: ' + impl.contextId + '\n' +
    '- Changed paths (derived):\n' +
    (impl.changedPaths.length ? impl.changedPaths.map((p) => '  - ' + p).join('\n') : '  (none reported)') + '\n' +
    '- Check receipts (factual command outcomes):\n' + JSON.stringify(impl.checkReceipts, null, 2) + '\n\n' +
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
    'evidence of a criteria entry.',
  { label: 'validate', phase: 'Validate', schema: VALIDATE_SCHEMA, effort: 'high' }
);
}

if (!validation) {
  return {
    verdict: 'NOT_PROVEN',
    verdictPath: null,
    intentDigest: plan.intentDigest,
    changedPaths: impl.changedPaths,
    filesSummary: impl.filesSummary,
    criteria: [],
    error: 'Validate stage failed; the candidate exists but was never freshly judged',
  };
}

// Report and stop: script-assembled, no fourth agent, no next action.
// filesSummary reaches only this caller-facing report — never the validator.
return {
  verdict: validation.verdict,
  verdictPath: validation.verdictPath,
  intentDigest: plan.intentDigest,
  changedPaths: impl.changedPaths,
  filesSummary: impl.filesSummary,
  criteria: validation.criteria,
};
