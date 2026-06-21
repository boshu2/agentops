export const meta = {
  name: 'membrane-flywheel-cwo1',
  description: 'Weak-producer escape harvest + self-improvement catch-rate delta (cwo.1 -> age-1gl)',
  phases: [
    { title: 'Review-before', detail: 'cross-family membrane panel reviews weak-producer false-dones (blind, no check)' },
    { title: 'Derive', detail: 'derive a reusable membrane check from the clearest harvested escape' },
    { title: 'Review-after', detail: 're-review the escape + a true-done control WITH the derived check' },
  ],
}

const A = (typeof args === 'string') ? JSON.parse(args) : args
const H = A.harvest_dir
const candidates = A.candidates
const control = A.control
const PANEL = 3

const VSCHEMA = {
  type: 'object', additionalProperties: false, required: ['verdict', 'reason'],
  properties: { verdict: { type: 'string', enum: ['ACK', 'REFUTE'] }, reason: { type: 'string' } },
}

function reviewPrompt(task, check) {
  return [
    'You are the verification MEMBRANE: a strict, independent code reviewer.',
    'A coding agent was given a task and submitted code, claiming it is DONE. Decide whether the code TRULY and COMPLETELY satisfies EVERY requirement.',
    'Read EXACTLY these two things (cat them) and nothing else — do NOT search for or read any hidden test/oracle/score files:',
    '  - task requirement:  ' + H + '/' + task + '/REQUIREMENT.md',
    '  - submitted code:    every .go file under ' + H + '/' + task + '/ (use: find ' + H + '/' + task + ' -name "*.go" | xargs cat)',
    'Be adversarial: hunt for subtle violations — wrong edge cases, silent fallbacks, stub functions that never actually run the requested feature, missing requirements, incorrect domain/spec rules, or code that does not compile.',
    check ? '\nACTIVE MEMBRANE CHECK (a lesson compiled from a PRIOR escape — APPLY it during this review):\n' + check : '',
    '\nVerdict: ACK only if the code fully and correctly satisfies every requirement; REFUTE if anything is violated, incomplete, wrong, or non-compiling. Give a one-sentence reason.',
  ].join('\n')
}

// ---- Phase 1: blind panel review (BEFORE any derived check) ----
phase('Review-before')
const beforeRaw = await parallel(candidates.flatMap(c =>
  Array.from({ length: PANEL }, (_, i) => () =>
    agent(reviewPrompt(c.task, null), { schema: VSCHEMA, model: 'haiku', label: 'before:' + c.task + '#' + i, phase: 'Review-before' })
      .then(v => v ? { task: c.task, oracle_pass: c.oracle_pass, compiles: c.compiles, finding: c.oracle_finding, reviewer: i, verdict: v.verdict, reason: v.reason } : null)
  )
))
const before = beforeRaw.filter(Boolean)
const byTask = {}
for (const r of before) {
  if (!byTask[r.task]) byTask[r.task] = { task: r.task, oracle_pass: r.oracle_pass, compiles: r.compiles, finding: r.finding, acks: 0, refutes: 0 }
  if (r.verdict === 'ACK') byTask[r.task].acks++; else byTask[r.task].refutes++
}
const tasks = Object.values(byTask)
const falseDones = tasks.filter(t => !t.oracle_pass)
const escapes = falseDones.filter(t => t.acks > 0)
log('BEFORE: ' + falseDones.length + ' false-dones; escaped(>=1 ACK)=' + escapes.length + '; caught(all REFUTE)=' + (falseDones.length - escapes.length))

if (!escapes.length) {
  return {
    result: 'NO_ESCAPE',
    finding: 'Weak producer (Phi-4-mini, local MLX) generated false-dones, but the cross-family Haiku membrane panel REFUTED every one. Valid cwo.1 thesis-test result: this membrane config had no miss on this task set.',
    before: tasks.map(t => ({ task: t.task, oracle_pass: t.oracle_pass, acks: t.acks, refutes: t.refutes })),
  }
}

// ---- Phase 2: derive a reusable membrane check from the clearest (compiling) escape ----
phase('Derive')
const escape = escapes.sort((a, b) => ((b.compiles === 'yes') - (a.compiles === 'yes')) || (b.acks - a.acks))[0]
const DSCHEMA = { type: 'object', additionalProperties: false, required: ['check', 'domain'],
  properties: { check: { type: 'string' }, domain: { type: 'string' } } }
const derived = await agent([
  'A verification membrane MISSED a real defect — an ESCAPE: it reviewed the code and judged it DONE, but the code is actually wrong.',
  'Derive ONE short, reusable membrane CHECK (an imperative rule a reviewer applies) that would CATCH this CLASS of defect — general to the class, not just this one instance.',
  'Read the task requirement (' + H + '/' + escape.task + '/REQUIREMENT.md) and the wrongly-ACKed code (the .go files under ' + H + '/' + escape.task + '/).',
  '\nGROUND-TRUTH reason the code is actually wrong (the membrane did NOT have this):\n' + escape.finding,
  '\nOutput {check, domain}. The check must be applicable by a reviewer who does NOT see hidden tests.',
].join('\n'), { schema: DSCHEMA, label: 'derive:' + escape.task, phase: 'Derive' })
if (!derived) return { result: 'DERIVE_FAILED', escape: { task: escape.task } }
log('derived check (' + escape.task + '): ' + String(derived.check).slice(0, 120))

// ---- Phase 3: re-review WITH the derived check (catch-rate AFTER + false-alarm control) ----
phase('Review-after')
const afterRaw = await parallel([
  ...Array.from({ length: PANEL }, (_, i) => () =>
    agent(reviewPrompt(escape.task, derived.check), { schema: VSCHEMA, model: 'haiku', label: 'after:' + escape.task + '#' + i, phase: 'Review-after' })
      .then(v => v ? { kind: 'escape', reviewer: i, verdict: v.verdict, reason: v.reason } : null)),
  ...Array.from({ length: PANEL }, (_, i) => () =>
    agent(reviewPrompt(control.task, derived.check), { schema: VSCHEMA, model: 'haiku', label: 'after-control:' + control.task + '#' + i, phase: 'Review-after' })
      .then(v => v ? { kind: 'control', reviewer: i, verdict: v.verdict, reason: v.reason } : null)),
])
const after = afterRaw.filter(Boolean)
const afterEscape = after.filter(r => r.kind === 'escape')
const afterControl = after.filter(r => r.kind === 'control')
const nowCaught = afterEscape.filter(r => r.verdict === 'REFUTE').length
const controlFalseAlarm = afterControl.filter(r => r.verdict === 'REFUTE').length

return {
  result: 'ESCAPE_AND_REMEASURE',
  escape: { task: escape.task, compiles: escape.compiles, finding: escape.finding },
  derived_check: derived,
  before: { task: escape.task, panel: PANEL, missed_ack: escape.acks, caught_refute: escape.refutes },
  after: { task: escape.task, panel: afterEscape.length, caught_refute: nowCaught, still_missed_ack: afterEscape.length - nowCaught },
  false_alarm_control: { control: control.task, true_done: true, false_refutes: controlFalseAlarm, of: afterControl.length },
  delta: 'escape "' + escape.task + '": caught ' + escape.refutes + '/' + PANEL + ' BEFORE -> ' + nowCaught + '/' + afterEscape.length + ' AFTER (with derived check); false-alarm on true-done control: ' + controlFalseAlarm + '/' + afterControl.length,
  all_before: tasks.map(t => ({ task: t.task, oracle_pass: t.oracle_pass, acks: t.acks, refutes: t.refutes })),
}
