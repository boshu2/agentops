export const meta = {
  name: 'catchfuel-falsealarm-probe',
  description: 'Council TIERED_MIDDLE falsification (precision half): does injecting catch-derived ADVISORY findings make the membrane over-REFUTE genuinely-correct true-dones? If REFUTE rate rises WITH findings, catch-fuel pollutes precision -> escape-only; if ~0, precision-safe.',
  phases: [ { title: 'Probe', detail: 'true-dones reviewed WITHOUT vs WITH catch-findings injected' } ],
}

const A = (typeof args === 'string') ? JSON.parse(args) : args
const PROBE = A.probe_dir
const TRUEDONES = A.true_dones
const CHECKS = A.catch_checks
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
    '  - task requirement:  ' + PROBE + '/' + task + '/REQUIREMENT.md',
    '  - submitted code:    every .go file under ' + PROBE + '/' + task + '/ (use: find ' + PROBE + '/' + task + ' -name "*.go" | xargs cat)',
    'Be adversarial: hunt for subtle violations — wrong edge cases, silent fallbacks, stub functions, missing requirements, incorrect domain/spec rules, or code that does not compile.',
    check ? '\nACTIVE MEMBRANE CHECKS (advisory domain-memory compiled from PRIOR catches — APPLY them during this review):\n' + check : '',
    '\nVerdict: ACK only if the code fully and correctly satisfies every requirement; REFUTE if anything is violated, incomplete, wrong, or non-compiling. Give a one-sentence reason.',
  ].join('\n')
}

phase('Probe')
const raw = (await parallel(TRUEDONES.flatMap(td => [
  ...Array.from({ length: PANEL }, (_, i) => () =>
    agent(reviewPrompt(td, null), { schema: VSCHEMA, model: 'haiku', label: 'without:' + td + '#' + i, phase: 'Probe' })
      .then(v => v ? { task: td, cond: 'without', verdict: v.verdict, reason: v.reason } : null)),
  ...Array.from({ length: PANEL }, (_, i) => () =>
    agent(reviewPrompt(td, CHECKS), { schema: VSCHEMA, model: 'haiku', label: 'with:' + td + '#' + i, phase: 'Probe' })
      .then(v => v ? { task: td, cond: 'with', verdict: v.verdict, reason: v.reason } : null)),
]))).filter(Boolean)

function tally(cond) {
  const rs = raw.filter(r => r.cond === cond)
  return { panel: rs.length, refute_false_alarm: rs.filter(r => r.verdict === 'REFUTE').length, ack: rs.filter(r => r.verdict === 'ACK').length }
}
const without = tally('without')
const wit = tally('with')
const refuteReasons = raw.filter(r => r.cond === 'with' && r.verdict === 'REFUTE').map(r => ({ task: r.task, reason: r.reason }))

return {
  result: 'FALSE_ALARM_PROBE',
  true_dones: TRUEDONES,
  without_findings: without,
  with_findings: wit,
  false_alarm_delta: 'REFUTE on CORRECT work: ' + without.refute_false_alarm + '/' + without.panel + ' WITHOUT -> ' + wit.refute_false_alarm + '/' + wit.panel + ' WITH catch-findings',
  verdict: wit.refute_false_alarm > without.refute_false_alarm
    ? 'PRECISION REGRESSION — catch-findings induced false-alarms on correct work (supports KEEP_ESCAPE_ONLY / strong decay+dedup needed)'
    : 'PRECISION-SAFE — catch-findings did NOT increase false-alarms on correct work (supports TIERED_MIDDLE advisory catch-fuel)',
  with_refute_reasons: refuteReasons,
  all: raw,
}