export const meta = {
  name: 'operating-loop',
  description: 'Run one capability through the AgentOps seven-move operating loop (shape → bead → slice/wave → pre-flight → TDD per slice → acceptance → ratchet)',
  whenToUse: 'When you want a feature/capability driven end-to-end through the repo doctrine in docs/architecture/operating-loop.md, with subagents standing in for the loop-move skills (discovery/design, plan, pre-mortem/council, implement, validation, post-mortem/flywheel).',
  phases: [
    { title: 'Shape', detail: 'brainstorm/discovery/design lenses → one BDD intent issue' },
    { title: 'Plan', detail: 'vertical slices + wave-validity (conflict-free) grouping' },
    { title: 'Pre-flight', detail: 'pre-mortem + council judge panel → go/no-go' },
    { title: 'Implement', detail: 'TDD per slice; waves sequential, slices parallel only when wave is conflict-free' },
    { title: 'Capture', detail: 'acceptance roll-up + ratcheted learnings' },
  ],
}

// ---------------------------------------------------------------------------
// §6 CONFORMANCE (docs/architecture/control-loop-model.md §6 — loop-model-compliant)
//   R2 terminate-on-verdict ✓  pre-flight breaks on `go`; slices return on green;
//      MAX_REPLAN/MAX_SLICE_RETRY are BACKSTOPS, not the terminator.
//   R3 no-self-modification-in-run ✓  no gate is added/removed/retuned mid-run.
//   R5 orchestrator-routes-never-reasons ✓  this script dispatches per-move agents
//      and routes on verdicts; it never does the work it gates.
//   R1 deterministic-gates — HARDENED here: slice acceptance gates on a CAPTURED
//      test exit code (testExitCode === 0), NOT a free-form `testNowPasses` self-grade
//      (§6.1's #1 non-convergence cause). RESIDUAL: a Workflow script has no shell,
//      so it cannot run the test itself — the agent transcribes the exit code. The
//      AUTHORITATIVE deterministic gate is `ao gate check` run post-workflow (the
//      pre-push windshield); the in-run verdict is provisional, best-grounded on
//      captured evidence.
//   R4 escapes→slow-loop ✓  — the Capture phase emits gate-verdicts to the yield
//      ledger: a CONFIRMED at the upstream slice gate (all slices green), and a
//      REFUTED at attempt 2 when the DOWNSTREAM acceptance roll-up catches a defect
//      the slice gate passed. That CONFIRMED-then-REFUTED pair is the escape the
//      slow loop's `ao membrane derive-checks` / DetectEscapes (age-zqc) consumes.
//      Workflow-side EMIT only; the derive/govern half is age-cwo (no front-running).
// ---------------------------------------------------------------------------

// ---- The capability to run through the loop -------------------------------
// args may be a string (the intent text) or {intent, beadId}. If absent, the
// shape agents fall back to the ready queue and pick the top item.
const intent = (args && (typeof args === 'string' ? args : args.intent)) || null
const intentClause = intent
  ? `The capability/intent to run through the loop is:\n\n${intent}`
  : `No intent was passed. Run \`BEADS_DIR="$(ao beads dir)" br ready\` (bd/Dolt is retired), pick the highest-priority unblocked bead, and treat its title+description as the intent. State which bead id you picked.`

// S2: run mode — 'full' (default) runs all five phases; 'plan' stops after
// Pre-flight, returning the hardened plan + verdicts (the safe planning half).
const mode = args && typeof args === 'object' && args.mode === 'plan' ? 'plan' : 'full'
// S1: changes the pre-flight panel asks for but that survive MAX_REPLAN re-plans.
let unresolvedChanges = []
// S5a: scale verification fan-out (refutation skeptics per risk) to the turn's
// token target — more independent checks when budget allows, exactly 1 otherwise.
const FANOUT = budget && budget.total ? Math.min(3, Math.max(1, Math.floor(budget.total / 200000))) : 1
// S5b: simple framing lenses run on a cheap model; synthesis/judging/validation
// keep the default (inherited) model so reasoning quality is preserved.
const FRAMING_MODEL = 'haiku'
// S6: how many times a BLOCKED/failing slice is re-attempted before giving up.
const MAX_SLICE_RETRY = 2

const DOCTRINE = `You are executing one move of the AgentOps seven-move operating loop documented at docs/architecture/operating-loop.md. Read that file if you need the exact gate definitions. Governing principles: behavior (a Given/When/Then) is the unit of work, not a layer; the first failing test is a slice's contract; parallelism requires explicit non-colliding write scopes; artifacts only exist if they advance the loop. Anchor domain terms to skills/domain/references/.`

// ---------------------------------------------------------------------------
// SCHEMAS
// ---------------------------------------------------------------------------
const INTENT_SCHEMA = {
  type: 'object',
  required: ['feature', 'acceptance', 'boundedContext', 'nonGoals', 'rollback', 'evidenceNeeded'],
  properties: {
    feature: { type: 'string', description: 'Feature/capability name' },
    beadId: { type: 'string', description: 'br bead id if one was picked/created, else empty' },
    acceptance: {
      type: 'array',
      minItems: 2,
      description: 'Given/When/Then examples: at least one happy path + one edge',
      items: {
        type: 'object',
        required: ['name', 'given', 'when', 'then'],
        properties: {
          name: { type: 'string' },
          given: { type: 'string' },
          when: { type: 'string' },
          then: { type: 'string' },
        },
      },
    },
    domainTerms: { type: 'array', items: { type: 'string' } },
    boundedContext: { type: 'string', description: 'e.g. BC1-Corpus … BC5-Runtime per docs/contracts/context-map.md' },
    nonGoals: { type: 'array', items: { type: 'string' } },
    rollback: { type: 'string', description: 'Rollback / containment path' },
    evidenceNeeded: { type: 'array', items: { type: 'string' }, description: 'Test names, snapshot keys, eval suites, council verdicts' },
  },
}

const PLAN_SCHEMA = {
  type: 'object',
  required: ['slices', 'waves'],
  properties: {
    slices: {
      type: 'array',
      minItems: 1,
      items: {
        type: 'object',
        required: ['id', 'behavior', 'firstFailingTest', 'writeScope', 'boundedContext'],
        properties: {
          id: { type: 'string', description: 'short slice id e.g. S1' },
          behavior: { type: 'string', description: 'the single Given/When/Then row this slice demonstrates' },
          firstFailingTest: { type: 'string', description: 'name of the first failing test that is this slice contract' },
          writeScope: { type: 'array', items: { type: 'string' }, description: 'files/dirs this slice modifies' },
          boundedContext: { type: 'string' },
        },
      },
    },
    waves: {
      type: 'array',
      minItems: 1,
      description: 'Slices grouped into waves. A wave with >1 slice MUST pass the seven-row conflict-free check.',
      items: {
        type: 'object',
        required: ['sliceIds', 'parallelizable', 'conflictFreeCheck'],
        properties: {
          sliceIds: { type: 'array', items: { type: 'string' } },
          parallelizable: { type: 'boolean', description: 'true only if every conflict-free row passes' },
          conflictFreeCheck: {
            type: 'object',
            description: 'the seven wave-validity rows; each true=pass',
            required: ['distinctWriteScopes', 'distinctTestTargets', 'noSharedMigration', 'noSharedCliSurface', 'integrationOrderDeclared', 'ownerPerSlice', 'discardPathPerSlice'],
            properties: {
              distinctWriteScopes: { type: 'boolean' },
              distinctTestTargets: { type: 'boolean' },
              noSharedMigration: { type: 'boolean' },
              noSharedCliSurface: { type: 'boolean' },
              integrationOrderDeclared: { type: 'boolean' },
              ownerPerSlice: { type: 'boolean' },
              discardPathPerSlice: { type: 'boolean' },
            },
          },
        },
      },
    },
  },
}

const VERDICT_SCHEMA = {
  type: 'object',
  required: ['verdict', 'rationale', 'blockingRisks'],
  properties: {
    lens: { type: 'string' },
    verdict: { type: 'string', enum: ['go', 'go-with-changes', 'no-go'] },
    rationale: { type: 'string' },
    blockingRisks: { type: 'array', items: { type: 'string' } },
    requiredChanges: { type: 'array', items: { type: 'string' } },
  },
}

const SLICE_RESULT_SCHEMA = {
  type: 'object',
  required: ['sliceId', 'testFirstFailed', 'testNowPasses', 'testCommand', 'testExitCode', 'evidence'],
  properties: {
    sliceId: { type: 'string' },
    testFirstFailed: { type: 'boolean', description: 'did the first failing test fail for the right reason before implementation' },
    testNowPasses: { type: 'boolean', description: 'agent self-report (human-readable); NOT the gate — the acceptance gate reads testExitCode === 0' },
    testCommand: { type: 'string', description: 'the exact test command run against the working tree (e.g. `cd cli && go test ./internal/foo/...`)' },
    testExitCode: { type: 'integer', description: 'captured exit code of testCommand; 0 = green. This is the §6.1 ground-truth the acceptance gate reads — transcribe the real value, do not assert a 0 you cannot show.' },
    refactorCommitSeparate: { type: 'boolean' },
    filesChanged: { type: 'array', items: { type: 'string' } },
    evidence: { type: 'string', description: 'test names + commands run + raw result tail' },
    blocked: { type: 'boolean' },
    blockerReason: { type: 'string' },
  },
}

const CLOSEOUT_SCHEMA = {
  type: 'object',
  required: ['accepted', 'acceptanceMap', 'learnings'],
  properties: {
    accepted: { type: 'boolean', description: 'true only if every acceptance GWT maps to a passing test and every non-goal is untouched' },
    acceptanceMap: {
      type: 'array',
      items: {
        type: 'object',
        required: ['example', 'provedBy', 'passed'],
        properties: { example: { type: 'string' }, provedBy: { type: 'string' }, passed: { type: 'boolean' } },
      },
    },
    residualGaps: { type: 'array', items: { type: 'string' } },
    learnings: {
      type: 'array',
      description: 'Ratchet candidates with promotion target per the operating-loop ratchet table',
      items: {
        type: 'object',
        required: ['learning', 'promoteTo'],
        properties: {
          learning: { type: 'string' },
          promoteTo: { type: 'string', enum: ['handoff', '.agents/learnings', 'SKILL.md/template', 'validation-gate', 'PRODUCT/GOALS/cdlc'] },
        },
      },
    },
  },
}

// ===========================================================================
// MOVE 1 — Shape intent as BDD (fan out 3 lenses → synthesize one intent)
// ===========================================================================
phase('Shape')
const LENSES = [
  { key: 'brainstorm', ask: 'Generate the option space: what is the smallest valuable behavior to ship, and 2-3 alternative framings. Winnow to one.' },
  { key: 'discovery', ask: 'Scope it: constraints, the bounded context it touches, non-goals, and the rollback/containment path.' },
  { key: 'design', ask: 'Validate product fit: who consumes this, what evidence proves it works, and what would make it the wrong thing to build.' },
]
const framings = (await parallel(LENSES.map((l) => () =>
  agent(`${DOCTRINE}\n\nMove 1 (Shape intent), ${l.key} lens.\n${intentClause}\n\nTask: ${l.ask}\nReturn a concise framing (no code edits).`,
    { label: `shape:${l.key}`, phase: 'Shape', model: FRAMING_MODEL })
))).filter(Boolean)

const intentIssue = await agent(
  `${DOCTRINE}\n\nMove 1 (Shape intent) — SYNTHESIZER.\n${intentClause}\n\nThree framing lenses produced these notes:\n\n${framings.map((f, i) => `### ${LENSES[i].key}\n${f}`).join('\n\n')}\n\nProduce ONE BDD-shaped intent issue. The acceptance examples MUST be testable (at least one happy path + one edge). Anchor domain terms to skills/domain/references/ and name the bounded context per docs/contracts/context-map.md.`,
  { label: 'shape:synthesize', phase: 'Shape', schema: INTENT_SCHEMA }
)
log(`Intent shaped: "${intentIssue.feature}" · ${intentIssue.acceptance.length} acceptance examples · ${intentIssue.boundedContext}`)

// ===========================================================================
// MOVE 3 — Slice vertically + group into waves (conflict-free gate)
// (Move 2 "track as bead" is a side-effect the synthesizer records in beadId.)
// ===========================================================================
phase('Plan')
let plan = await agent(
  `${DOCTRINE}\n\nMove 3 (Slice + wave plan).\n\nIntent issue:\n${JSON.stringify(intentIssue, null, 2)}\n\nDecompose into vertical slices: each slice demonstrates exactly ONE Given/When/Then row, has a nameable first failing test, a review-in-one-pass write scope, and touches one bounded context. "Refactor then feature" is TWO slices.\n\nThen group slices into waves. A wave may hold >1 slice ONLY if it passes ALL seven conflict-free rows (distinct write scopes, distinct test targets, no shared migration, no shared CLI surface, integration order declared, owner per slice, discard path per slice). If any row fails, those slices go in single-slice (sequential) waves. Set parallelizable accordingly.`,
  { label: 'plan:slice', phase: 'Plan', schema: PLAN_SCHEMA }
)
log(`Plan: ${plan.slices.length} slices in ${plan.waves.length} waves (${plan.waves.filter((w) => w.parallelizable).length} parallelizable)`)

// ===========================================================================
// MOVE 5 pre-flight — pre-mortem + council judge panel (BARRIER: need all to decide)
// ===========================================================================
phase('Pre-flight')
const JUDGES = [
  { key: 'pre-mortem', ask: 'Assume this plan shipped and failed. What are the most likely failure modes? Are any acceptance examples untestable? Is any "parallelizable" wave actually colliding?' },
  { key: 'council-correctness', ask: 'Does each slice really map to one behavior with a genuine first failing test? Flag slices that are layers, not behaviors.' },
  { key: 'council-scope', ask: 'Do the slices stay inside the declared bounded context and non-goals? Flag scope creep or cross-context slices.' },
]
const runJudges = async (planArg) => (await parallel(JUDGES.map((j) => () =>
  agent(`${DOCTRINE}\n\nMove 5 pre-flight — ${j.key}. Be adversarial; default to skepticism.\n\nIntent:\n${JSON.stringify(intentIssue, null, 2)}\n\nPlan:\n${JSON.stringify(planArg, null, 2)}\n\n${j.ask}\nReturn a verdict; put any concrete fixes in requiredChanges.`,
    { label: `preflight:${j.key}`, phase: 'Pre-flight', schema: { ...VERDICT_SCHEMA, properties: { ...VERDICT_SCHEMA.properties, lens: { type: 'string', enum: [j.key] } } } })
    .then((v) => ({ ...v, lens: j.key }))
))).filter(Boolean)

// S4: a single judge's `no-go` does not halt the run — N independent skeptics try
// to REFUTE each blocking risk; halt only if a majority survive refutation.
const survivesRefutation = async (risks) => {
  if (!risks.length) return { survived: false, real: [] }
  const refuteOne = (r) =>
    agent(`${DOCTRINE}\n\nAdversarial verification. A pre-flight judge raised this BLOCKING risk:\n\n"${r}"\n\nTry hard to REFUTE it against the actual repo. Is it real, verifiable, AND genuinely blocking? Default refuted=true if you cannot confirm all three.`,
      { label: 'refute', phase: 'Pre-flight', schema: { type: 'object', required: ['refuted', 'why'], properties: { refuted: { type: 'boolean' }, why: { type: 'string' } } } })
  // S5a: FANOUT independent skeptics per risk. A risk survives (stays blocking)
  // unless a majority of the skeptics that ANSWERED refute it — so missing
  // answers fail CLOSED (the risk is kept), never silently toward "proceed".
  const perRisk = (await parallel(risks.map((r) => () =>
    parallel(Array.from({ length: FANOUT }, () => () => refuteOne(r)))
      .then((votes) => {
        const v = votes.filter(Boolean)
        const refutedByMajority = v.filter((c) => c.refuted).length >= Math.ceil((v.length || 1) / 2)
        return { why: (v.find((c) => !c.refuted) || v[0] || {}).why || r, survived: !refutedByMajority }
      })
  ))).filter(Boolean)
  const real = perRisk.filter((x) => x.survived)
  return { survived: real.length >= Math.ceil(risks.length / 2), real }
}

// S1: close the pre-flight -> plan feedback loop. A `go-with-changes` verdict (or a
// `no-go` that survives refutation's requiredChanges) is folded back into a re-plan,
// bounded to MAX_REPLAN passes so it can never loop forever.
const MAX_REPLAN = 2
let verdicts = await runJudges(plan)
let replanned = 0
while (true) {
  const noGo = verdicts.filter((v) => v.verdict === 'no-go')
  if (noGo.length) {
    const { survived, real } = await survivesRefutation(noGo.flatMap((v) => v.blockingRisks || []))
    if (survived) {
      log(`PRE-FLIGHT BLOCKED: ${real.length} risk(s) survived refutation.`)
      return { stoppedAt: 'pre-flight', mode, intent: intentIssue, plan, verdicts, blockingRisks: real.map((r) => r.why) }
    }
    log(`Pre-flight: no-go verdict(s) refuted as non-blocking — continuing.`)
  }
  const changes = verdicts.filter((v) => v.verdict !== 'go').flatMap((v) => v.requiredChanges || [])
  if (changes.length === 0 || replanned >= MAX_REPLAN) {
    unresolvedChanges = changes
    break
  }
  log(`Pre-flight: folding ${changes.length} required change(s) into the plan (re-plan ${replanned + 1}/${MAX_REPLAN}).`)
  const replannedPlan = await agent(`${DOCTRINE}\n\nMove 3 RE-PLAN. The pre-flight panel returned required changes; produce a corrected plan (same schema) that applies ALL of them. Keep every slice unless a change says to drop it.\n\nCurrent plan:\n${JSON.stringify(plan, null, 2)}\n\nRequired changes:\n${JSON.stringify(changes, null, 2)}`,
    { label: `replan:${replanned + 1}`, phase: 'Pre-flight', schema: PLAN_SCHEMA })
  // Guard: if the re-plan returns null/invalid, keep the prior plan and stop
  // re-planning rather than crashing the Implement phase on plan.slices.
  if (!replannedPlan || !Array.isArray(replannedPlan.slices)) {
    log('Pre-flight: re-plan returned an invalid plan — keeping the prior plan, leaving changes unresolved.')
    unresolvedChanges = changes
    break
  }
  plan = replannedPlan
  verdicts = await runJudges(plan)
  replanned++
}
log(`Pre-flight: ${verdicts.map((v) => `${v.lens}=${v.verdict}`).join(', ')}${unresolvedChanges.length ? ` · ${unresolvedChanges.length} change(s) unresolved after ${MAX_REPLAN} re-plans` : ''}`)

// S2: plan-only mode returns the hardened plan + verdicts here (the safe half).
if (mode === 'plan') {
  return { stoppedAt: 'plan-mode', mode, intent: intentIssue, plan, verdicts, unresolvedChanges }
}

// ===========================================================================
// MOVE 4 + 6 — TDD per slice, then per-slice validation.
// Waves run SEQUENTIALLY. Slices inside a conflict-free wave run in PARALLEL.
// Collision-safety rests on move 5's wave-validity gate (disjoint write scopes),
// NOT on worktree isolation — isolation would strand every change in a throwaway
// worktree instead of landing it in the checkout. Agents write to the working
// tree but DO NOT commit: committing is the human/PR discipline's job (every
// change is a PR that cites a bead), and not-committing also avoids a git index
// race between parallel agents sharing one checkout.
// ===========================================================================
phase('Implement')
const sliceById = Object.fromEntries(plan.slices.map((s) => [s.id, s]))
const sliceResults = []
for (const wave of plan.waves) {
  const slices = wave.sliceIds.map((id) => sliceById[id]).filter(Boolean)
  // A slice that errors must surface as a BLOCKED result, never be silently
  // dropped — an empty sliceResults makes the capture move hallucinate success.
  const blocked = (s, err) => ({ sliceId: s.id, testFirstFailed: false, testNowPasses: false, blocked: true, blockerReason: `slice errored before producing a result: ${String(err && err.message || err)}`, evidence: '(none — slice did not complete)' })
  // One TDD+validate attempt. Move 4 implements; Move 6 validates via the
  // code-reviewer agent (S5b). priorFailure feeds the last blocker into a retry.
  const attemptSlice = (s, priorFailure) =>
    agent(`${DOCTRINE}\n\nMove 4 (TDD for one slice). Write to the working tree; DO NOT git commit or git add (the human reviews and commits under a bead+PR).\n\nSlice ${s.id}: ${s.behavior}\nFirst failing test (your contract): ${s.firstFailingTest}\nWrite scope: ${(s.writeScope || []).join(', ')}\nBounded context: ${s.boundedContext}${priorFailure ? `\n\nA PRIOR ATTEMPT FAILED: ${priorFailure}\nDiagnose why and try a DIFFERENT approach.` : ''}\n\nDo it in order: (1) write the first failing test and confirm it fails for the RIGHT reason (missing behavior, not syntax); (2) make the smallest change that flips it to green; (3) refactor under green. Follow .claude/rules/go.md / python.md. Stay STRICTLY inside the write scope — touch no file outside it.`,
      { label: `tdd:${s.id}`, phase: 'Implement' })
      .then((impl) =>
        agent(`${DOCTRINE}\n\nMove 6 (slice acceptance) for slice ${s.id}: ${s.behavior}\n\nImplementer reported:\n${impl}\n\nRun the slice's tests against the working tree and prove the behavior. Confirm the first failing test now passes, the change is present in the working tree, and (via \`git status --short\`) nothing outside the declared write scope changed. Report the EXACT command you ran in testCommand, its CAPTURED exit code in testExitCode (0 = green), and the raw result tail in evidence. The gate reads testExitCode, not testNowPasses — transcribe the real exit code; do not assert a green you cannot show.`,
          { label: `validate:${s.id}`, phase: 'Implement', schema: SLICE_RESULT_SCHEMA, agentType: 'agentops:code-reviewer' })
          .then((r) => ({ ...r, sliceId: s.id }))
      )
  // S6: bounded self-repair — re-attempt a BLOCKED/failing slice up to
  // MAX_SLICE_RETRY times before recording it blocked, feeding each failure forward.
  const runSlice = (s) => async () => {
    let last
    for (let k = 0; k <= MAX_SLICE_RETRY; k++) {
      try {
        last = await attemptSlice(s, k > 0 && last ? last.blockerReason || last.evidence : null)
      } catch (err) {
        last = blocked(s, err)
      }
      // §6.1: green is the CAPTURED exit code, not the agent's testNowPasses self-grade.
      if (last && last.testExitCode === 0 && !last.blocked) return { ...last, attempts: k + 1 }
      if (k < MAX_SLICE_RETRY) log(`Slice ${s.id} not green (attempt ${k + 1}/${MAX_SLICE_RETRY + 1}) — self-repair retry.`)
    }
    return { ...last, attempts: MAX_SLICE_RETRY + 1, selfRepairExhausted: true }
  }
  let waveResults
  if (wave.parallelizable) {
    // parallel() maps a thrown thunk to null; recover which slice each is by index.
    const settled = await parallel(slices.map((s) => () => runSlice(s)().catch((err) => blocked(s, err))))
    waveResults = settled.map((r, i) => r || blocked(slices[i], 'unknown (parallel returned null)'))
  } else {
    waveResults = []
    for (const s of slices) {
      try { waveResults.push(await runSlice(s)()) } catch (err) { waveResults.push(blocked(s, err)) }
    }
  }
  sliceResults.push(...waveResults)
  const greens = waveResults.filter((r) => r.testExitCode === 0 && !r.blocked).length
  const blocks = waveResults.filter((r) => r.blocked).length
  log(`Wave [${wave.sliceIds.join(',')}] done · ${greens}/${slices.length} green${blocks ? ` · ${blocks} BLOCKED` : ''}`)
}

// ===========================================================================
// MOVE 7 — Close the bead by proving acceptance, then ratchet learnings.
// ===========================================================================
phase('Capture')
const closeout = await agent(
  `${DOCTRINE}\n\nMove 6 roll-up + Move 7 (capture & ratchet).\n\nIntent acceptance examples:\n${JSON.stringify(intentIssue.acceptance, null, 2)}\n\nNon-goals (must remain untouched):\n${JSON.stringify(intentIssue.nonGoals, null, 2)}\n\nPer-slice results:\n${JSON.stringify(sliceResults, null, 2)}\n\nMap every acceptance example to the test/gate that proved it. accepted=true ONLY if every example maps to a passing test AND every non-goal is untouched. Then propose learnings, each tagged with a promotion target from the ratchet table (handoff dies at age-out; repeats-twice → .agents/learnings; changes-future-behavior → SKILL.md/template; must-never-regress → validation-gate; core-doctrine → PRODUCT/GOALS/cdlc). Promote nothing that was only noticed once.`,
  { label: 'capture:closeout', phase: 'Capture', schema: CLOSEOUT_SCHEMA }
)

// R4 — escapes route to the slow loop. The yield ledger pairs a CONFIRMED at the
// UPSTREAM gate (slices green) with a REFUTED at a higher attempt from the DOWNSTREAM
// gate (acceptance roll-up) → DetectEscapes surfaces it and `ao membrane derive-checks`
// (age-zqc) compiles the check that would have caught it. We EMIT only; the derive/
// govern half is age-cwo. The gate-verdict body is COMMIT-BOUND (ao validates
// pawl_verdict_ref + a head_sha ≥7 chars + mode + author_family), so the emit agent
// captures the run's current HEAD as the sha — the base commit the bead's work sits on.
const beadId = (intentIssue && intentIssue.beadId) || ''
const runId = (args && typeof args === 'object' && args.runId) || (beadId ? `operating-loop-${beadId}` : '')
const slicesAllGreen = sliceResults.length > 0 && sliceResults.every((s) => s && s.testExitCode === 0 && !s.blocked)
// Build a VALID gate-verdict emit prompt. The agent substitutes <SHA> with the real
// `git rev-parse HEAD`; the body shape matches the ao yield gate-verdict contract
// (deterministic tier: cross_family=false so it never inflates catch_rate_cross_family).
// pairGuard (optional) mirrors bdd-foundry.js's orphan-escape guard: the downstream
// REFUTED@2 emit is fail-open — a failed upstream CONFIRMED@1 append would leave a
// REFUTED@2 with no paired CONFIRMED@1, which DetectEscapes silently drops. When set,
// the agent re-asserts the attempt-1 CONFIRMED is in the yield ledger before the REFUTED
// (re-emitting is append-safe). Emit-only; derive/govern stays age-cwo.
const emitVerdict = (disposition, attempt, ctx, note, label, pairGuard) => agent(
  `${DOCTRINE}\n\nR4 gate-verdict (workflow-side EMIT only). ${note}\nDo exactly this, nothing else:${pairGuard ? ` (0) ${pairGuard}` : ''} (1) capture the current commit with \`SHA=$(git rev-parse HEAD)\`; (2) run\n  ao yield emit gate-verdict --bead ${JSON.stringify(beadId)} --run ${JSON.stringify(runId)} --json '{"difficulty":2,"pawl_verdict_ref":{"bead_id":${JSON.stringify(beadId)},"head_sha":"'"$SHA"'"},"disposition":"${disposition}","head_sha":"'"$SHA"'","attempt":${attempt},"mode":"deterministic","author_context_id":"${ctx}","refuter_families":[],"author_family":"operating-loop-gate","cross_family":false,"author_ne_reviewer":true,"evidence_present":true}'\nReturn the command's exit code (0 = appended). Do NOT run \`ao membrane derive-checks\` or otherwise build/tune the sink — that is the slow loop's job (age-cwo).`,
  { label, phase: 'Capture' })
let escapeEmitted = false
if (beadId && runId && slicesAllGreen) {
  // Upstream confirm: the per-slice gate passed green for the whole bead.
  await emitVerdict('CONFIRMED', 1, 'operating-loop-slice-gate', `The per-slice gate passed green for bead ${beadId} — record the upstream CONFIRM.`, 'capture:r4-confirm')
  if (!closeout.accepted) {
    // Downstream catch of an upstream-confirmed unit = an ESCAPE. Emit REFUTED at a higher attempt.
    const failed = (closeout.acceptanceMap || []).filter((m) => m && !m.passed).map((m) => m.example)
    await emitVerdict('REFUTED', 2, 'operating-loop-acceptance-rollup', `The per-slice gate passed green but the DOWNSTREAM acceptance roll-up caught defect(s): ${JSON.stringify(failed)} (residual gaps: ${JSON.stringify(closeout.residualGaps || [])}). This is an escape — a unit an upstream gate CONFIRMED that a stricter downstream gate REFUTED at a higher attempt; the CONFIRMED→REFUTED pair is what the slow loop consumes.`, 'capture:r4-escape-emit', `ORPHAN-ESCAPE GUARD: first confirm the attempt-1 CONFIRMED gate-verdict for bead ${JSON.stringify(beadId)} run ${JSON.stringify(runId)} is already in the yield ledger (it was just emitted upstream). If it is ABSENT (the upstream emit failed), re-emit that exact attempt-1 CONFIRMED body FIRST so this REFUTED@2 is never an orphan with no pair; re-emitting is append-safe.`)
    escapeEmitted = true
    log(`R4: escape emitted — acceptance roll-up caught ${failed.length} defect(s) the slice gate passed green; REFUTED gate-verdict (attempt 2) paired with the attempt-1 CONFIRMED for ${beadId}@${runId}. Slow loop (ao membrane derive-checks --run ${runId}) derives the catch.`)
  }
} else if (beadId && runId && !closeout.accepted) {
  // Slices were not all green AND acceptance failed — a direct fail, not an escape (no
  // upstream CONFIRM to pair against). Record the downstream REFUTED at attempt 1 so the
  // catch_rate gauge still counts the deterministic catch.
  await emitVerdict('REFUTED', 1, 'operating-loop-acceptance-rollup', `The acceptance roll-up did NOT accept bead ${beadId} (slices were not all green — a direct fail, not an escape).`, 'capture:r4-refute')
} else if (beadId && runId) {
  log(`R4: no escape — acceptance accepted (or no bead id); upstream CONFIRM recorded for ${beadId}@${runId}.`)
}

return {
  escapeEmitted,
  stoppedAt: null,
  mode,
  intent: intentIssue,
  plan,
  preflight: verdicts,
  unresolvedChanges,
  slices: sliceResults,
  closeout,
}
