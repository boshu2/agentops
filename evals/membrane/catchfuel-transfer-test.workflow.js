export const meta = {
  name: 'catchfuel-transfer-test',
  description: 'Catch-fuel VALUE half (cwo.1 transfer test): the Haiku PANEL caught 2 subtle compiling schema bugs 3/3. Does a derived check help a CHEAPER tier (a single terse reviewer) catch what it misses unaided? Measures catch-rate lift WITHOUT vs WITH the check + a true-done false-alarm control.',
  phases: [ { title: 'Transfer', detail: 'cheap single-reviewer tier reviews the caught bugs + a true-done, WITHOUT vs WITH the derived check' } ],
}

const A = (typeof args === 'string') ? JSON.parse(args) : args
const H = A.harvest_dir
const BUGS = A.bugs          // [{task, check}]  the caught schema false-dones + a derived check
const CONTROL = A.control    // a true-done task
const N = 3

const VSCHEMA = {
  type: 'object', additionalProperties: false, required: ['verdict', 'reason'],
  properties: { verdict: { type: 'string', enum: ['ACK', 'REFUTE'] }, reason: { type: 'string' } },
}

// A deliberately CHEAP/terse single reviewer (simulating a cheaper deployed membrane
// tier than the 3-Haiku panel): a fast skim, not an exhaustive adversarial audit.
function cheapPrompt(task, check) {
  return [
    'You are a FAST code reviewer doing a quick sanity pass (not an exhaustive audit). A coding agent says this task is DONE.',
    'Skim the requirement and the code and give a quick verdict — ACK if it looks fine, REFUTE if you spot a problem.',
    '  - requirement: ' + H + '/' + task + '/REQUIREMENT.md',
    '  - code: find ' + H + '/' + task + ' -name "*.go" | xargs cat',
    check ? '\nQUICK CHECK to apply (a lesson from a prior catch):\n' + check : '',
    '\nOne-line reason + verdict ACK/REFUTE.',
  ].join('\n')
}

phase('Transfer')
const thunks = []
for (const b of BUGS) {
  for (let i = 0; i < N; i++) {
    thunks.push(() => agent(cheapPrompt(b.task, null), { schema: VSCHEMA, model: 'haiku', label: 'bug-without:' + b.task + '#' + i, phase: 'Transfer' })
      .then(v => v ? { kind: 'bug', cond: 'without', task: b.task, verdict: v.verdict } : null))
    thunks.push(() => agent(cheapPrompt(b.task, b.check), { schema: VSCHEMA, model: 'haiku', label: 'bug-with:' + b.task + '#' + i, phase: 'Transfer' })
      .then(v => v ? { kind: 'bug', cond: 'with', task: b.task, verdict: v.verdict } : null))
  }
}
for (let i = 0; i < N; i++) {
  const allChecks = BUGS.map(b => b.check).join('\n')
  thunks.push(() => agent(cheapPrompt(CONTROL, null), { schema: VSCHEMA, model: 'haiku', label: 'ctl-without#' + i, phase: 'Transfer' })
    .then(v => v ? { kind: 'control', cond: 'without', task: CONTROL, verdict: v.verdict } : null))
  thunks.push(() => agent(cheapPrompt(CONTROL, allChecks), { schema: VSCHEMA, model: 'haiku', label: 'ctl-with#' + i, phase: 'Transfer' })
    .then(v => v ? { kind: 'control', cond: 'with', task: CONTROL, verdict: v.verdict } : null))
}
const raw = (await parallel(thunks)).filter(Boolean)

function caught(kind, cond) {
  const rs = raw.filter(r => r.kind === kind && r.cond === cond)
  return { of: rs.length, refute: rs.filter(r => r.verdict === 'REFUTE').length, ack: rs.filter(r => r.verdict === 'ACK').length }
}
const bw = caught('bug', 'without'), bwith = caught('bug', 'with')
const cw = caught('control', 'without'), cwith = caught('control', 'with')

return {
  result: 'TRANSFER_TEST',
  bugs: BUGS.map(b => b.task),
  control: CONTROL,
  bug_caught_without: bw.refute + '/' + bw.of,
  bug_caught_with: bwith.refute + '/' + bwith.of,
  control_false_alarm_without: cw.refute + '/' + cw.of,
  control_false_alarm_with: cwith.refute + '/' + cwith.of,
  transfer_lift: (bwith.refute - bw.refute),
  verdict: bw.refute < bwith.refute && cwith.refute <= cw.refute
    ? 'TRANSFER VALUE — the derived check helped the cheap tier catch more, without new false-alarms (supports catch-fuel value)'
    : (bw.refute >= bwith.refute
      ? 'NO LIFT — the cheap tier caught the same with/without the check (no demonstrable transfer value on these bugs; likely the bug is not a cheap-tier blindspot)'
      : 'FALSE-ALARM REGRESSION — the check induced false-alarms on the control'),
  all: raw,
}