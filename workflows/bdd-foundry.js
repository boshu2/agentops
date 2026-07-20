// ─── bdd-foundry v9 (2026-06-17) ──────────────────────────────────────────────
// v9: THINNED to a §6-conformant orchestrator (age-3va.3). The generative discipline
//     (the four behavior-first phases + the adversarial gap dimensions) no longer lives
//     inline — it is DISPATCHED to the behavior-first-planning skill
//     (skills/behavior-first-planning/SKILL.md), the single source of truth, so the
//     workflow can never drift from the skill. The workflow keeps ONLY what an
//     orchestrator owns: the deterministic gates (red-run, the JS-computed beadify gate,
//     the drift-guard), the cross-family validation, the tracker-write-on-clearing-verdict,
//     and the operational guards (dir-misaim, base-snapshot, args de-stringify). It gates
//     and routes; it never reasons about the work. See docs/architecture/
//     workflow-conformance-pattern.md and control-loop-model.md §6.
//
// History (incident patches preserved as orchestrator-level guards): v8 adversarial
// gap dimensions (now in the skill); v7 sentinel-slot guard; v6 args de-stringify;
// v5 dir-misaim + base-snapshot; v4 mechanical drift-guard; v3 truncation fix; v2 red-team.
// CANONICAL: workflows/bdd-foundry.js (git-tracked); project-local .claude/workflows/ is a runtime link minted by `ao workflows link`
//
// ─────────────────────────────────────────────────────────────────────────────
// §6 CONFORMANCE (docs/architecture/control-loop-model.md §6 — loop-model-compliant)
//   R1 deterministic-gates ✓  — every promotion reads ground truth the orchestrator
//      did not author: the EXECUTED red-run (REDRUN_SCHEMA), the JS-computed beadify
//      gate (runnable shape + valid scenario_ref + coverage + cycle-free), and the
//      drift-guard RUN (each acceptance command must list exactly one test). The
//      validate score is one input; the mechanical gates are the hard floor. No
//      free-form self-grade authorizes the tracker write.
//   R2 terminate-on-verdict ✓  — the tracker write fires only on a clearing verdict
//      AND coverage-complete AND cycle-free AND drift-green; otherwise it routes to a
//      single bounded repair pass and stops (the manifest is left for operator
//      re-validation). No unbounded loop.
//   R3 no-self-modification-in-run ✓  — no gate is added/removed/retuned mid-run.
//   R4 escapes→slow-loop ✓  — the Validate phase emits gate-verdicts to the yield
//      ledger: a CONFIRMED at the UPSTREAM beadify gate (the JS-computed bead set passed
//      runnable+valid-ref, cycle-free, fully-covering), and a REFUTED at attempt 2 when the
//      DOWNSTREAM gate (cross-family validate score / mechanical drift-guard) catches a
//      defect the beadify gate passed. That CONFIRMED-then-REFUTED pair for the same bead+run
//      is the escape the slow loop (ao membrane derive-checks, age-zqc) compiles into the
//      catch. EMIT only — derive/govern stays age-cwo. See workflow-conformance-pattern.md
//      'The R4 escape-emit step'.
//   R5 orchestrator-routes-never-reasons ✓  — every generative move is a dispatched
//      agent executing the behavior-first-planning skill; the script body only parses
//      inputs, computes the deterministic gates, and routes on the verdicts.
// ─────────────────────────────────────────────────────────────────────────────
export const meta = {
  name: 'bdd-foundry',
  description: 'Behavior-first planning: intent → Gherkin behaviors → failing acceptance tests (EXECUTED red) → spec → acceptance-gated bead DAG → independent cross-family validation BEFORE tracker write. No runnable acceptance test, no bead. Thin orchestrator over the behavior-first-planning skill.',
  whenToUse: 'When a feature/spec deserves rigorous planning and the beads must be genuinely crank-ready — each carrying a runnable acceptance test that defines "done". The behavior-first successor to plan-foundry: fixes the spec-first failure where beads ship with no done-criteria (the 3/10 problem). Triggers — "bdd-foundry", "plan behavior-first", "acceptance-first planning". Holdout grading per the external measurement register (judges pull holdout scenarios at validate time; implementing lanes never see them — ids deliberately not listed here).',
  phases: [
    { title: 'Behaviors', detail: 'skill phase 1 → frozen Gherkin (happy+edge+error), cross-family adversarial gap-check, dispositioned' },
    { title: 'AcceptanceTests', detail: 'skill phase 2 → runnable FAILING tests; conductor RUNS the suite and verifies red (deterministic gate)' },
    { title: 'Spec', detail: 'skill phase 3 → architecture derived to make the tests pass' },
    { title: 'Beadify', detail: 'skill phase 4 → dep-ordered DAG; conductor gate (JS): runnable acceptance + valid scenario_ref + coverage + cycle-free' },
    { title: 'Validate', detail: 'independent cross-family review of the MANIFEST (author≠reviewer); tracker write ONLY on a clearing verdict + drift-green' },
  ],
}

// ---- inputs (args: string intent, or {intent, tracker, tracker_cmd, dir, score_threshold}) ----
// v6: the skill/name-resolved launch path stringifies args — a JSON object arrives as a
// string, silently defaulting every named param. Parse JSON-looking strings before use.
if (typeof args === 'string' && /^\s*\{/.test(args)) { try { args = JSON.parse(args) } catch (e) { log('args looked like JSON but failed to parse — treating as plain intent string') } }
const intent = (args && (typeof args === 'string' ? args : args.intent)) || null
const TRACKER = (args && typeof args === 'object' && args.tracker) || 'br'
const TRACKER_CMD = (args && typeof args === 'object' && args.tracker_cmd) || TRACKER
const SCORE_THRESHOLD = (args && typeof args === 'object' && args.score_threshold) || 0.7
const slug = (s) => String(s || 'ready-pick').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
const RUN_TAG = (args && typeof args === 'object' && args.run_tag) || slug(intent)
const DRY_RUN = !!(args && typeof args === 'object' && args.dry_run === true) // verification mode: every gate EXCEPT the tracker write
const DIR = (args && typeof args === 'object' && args.dir) || `docs/plans/bdd-foundry/${RUN_TAG}`
const intentClause = intent
  ? `INTENT (the capability to plan): ${intent}`
  : `No intent passed — run \`${TRACKER_CMD} ready\`, pick the top unblocked item, treat its title+description as the intent, and state which id you picked.`
// The orchestrator's standing brief for every dispatched move. The DISCIPLINE lives in
// the skill (single source of truth); this brief only carries the register + the run dir.
const SKILL = 'skills/behavior-first-planning/SKILL.md'
const REGISTER = `Execute the behavior-first-planning skill (${SKILL}) — it is the source of truth for HOW. Keep the project's vocabulary register. Tracker invocation is EXACTLY \`${TRACKER_CMD}\` (never bare \`${TRACKER}\` if that differs; never from a worktree — if \`git rev-parse --git-dir\` ≠ \`git rev-parse --git-common-dir\` you are in a worktree: run tracker commands from the main checkout and STATE the resolved path). LAW: never \`claude -p\`/\`--print\`; \`codex exec\` is the cross-family sub. DO NOT git-commit (the conductor/operator commits).`

// ---- schemas (structured output = what makes the gates mechanical) -----------
const BEHAVIORS_SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['feature', 'scenarios'],
  properties: {
    feature: { type: 'string' },
    scenarios: {
      type: 'array', minItems: 1,
      items: {
        type: 'object', additionalProperties: false,
        required: ['id', 'title', 'kind', 'gherkin'],
        properties: {
          id: { type: 'string' },
          title: { type: 'string' },
          kind: { type: 'string', enum: ['happy', 'edge', 'error'] },
          gherkin: { type: 'string', description: 'Given/When/Then — concrete + testable, no vague "works correctly"' },
        },
      },
    },
  },
}
// behaviors:v1 — same shape plus the OPTIONAL base_protection report slot (committed /
// copied: <path> / fresh-dir — or the DIR-MISAIM sentinel if the preflight aborted).
const BEHAVIORS_V1_SCHEMA = { ...BEHAVIORS_SCHEMA, properties: { ...BEHAVIORS_SCHEMA.properties, base_protection: { type: 'string', description: "pre-run base protection applied: 'committed' / 'copied: <backup path>' / 'fresh-dir' — or exactly 'DIR-MISAIM: <pwd>' when the preflight aborts" } } }
const FROZEN_SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['feature', 'scenarios', 'gap_dispositions'],
  properties: {
    feature: BEHAVIORS_SCHEMA.properties.feature,
    scenarios: BEHAVIORS_SCHEMA.properties.scenarios,
    gap_dispositions: {
      type: 'array',
      items: {
        type: 'object', additionalProperties: false,
        required: ['gap', 'action', 'detail'],
        properties: { gap: { type: 'string' }, action: { type: 'string', enum: ['folded', 'rejected', 'deferred'] }, detail: { type: 'string' } },
      },
    },
  },
}
const REDRUN_SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['run_command', 'exit_code', 'failing', 'passing', 'errored_outside_assertions'],
  properties: {
    run_command: { type: 'string' },
    exit_code: { type: 'integer' },
    failing: { type: 'integer' },
    passing: { type: 'integer' },
    errored_outside_assertions: { type: 'integer', description: 'tests that crashed on harness/syntax rather than failing an assertion' },
  },
}
const BEADS_SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['beads'],
  properties: {
    beads: {
      type: 'array', minItems: 1,
      items: {
        type: 'object', additionalProperties: false,
        required: ['key', 'title', 'why', 'scenario_ref', 'acceptance_test', 'deps'],
        properties: {
          key: { type: 'string', description: 'short stable slug for dep-wiring within this run' },
          title: { type: 'string' },
          why: { type: 'string' },
          scenario_ref: { type: 'string', description: 'id of the behavior scenario this bead delivers' },
          acceptance_test: { type: 'string', description: 'INVOCABLE command + test path that defines done for THIS bead. ONE invocation per test (never multiple positional filters in one call — arg-errors before any test runs); chain with &&. Prose-only acceptance is REJECTED by the gate.' },
          deps: { type: 'array', items: { type: 'string' }, description: 'keys of beads this one depends on' },
        },
      },
    },
  },
}
const VALIDATE_SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['total', 'with_runnable_acceptance', 'crank_ready', 'score', 'biggest_gap'],
  properties: {
    total: { type: 'integer' }, with_runnable_acceptance: { type: 'integer' }, crank_ready: { type: 'integer' },
    score: { type: 'number' }, biggest_gap: { type: 'string' },
  },
}
const DRIFT_SCHEMA = { type: 'object', additionalProperties: false, required: ['all_pass', 'checked', 'failures'], properties: { all_pass: { type: 'boolean' }, checked: { type: 'integer' }, failures: { type: 'array', items: { type: 'object', additionalProperties: false, required: ['key', 'lists', 'reason'], properties: { key: { type: 'string' }, lists: { type: 'integer' }, reason: { type: 'string' } } } } } }

// off-account cross-family pass — the reasoning is Codex's, the wrapper just runs it.
function codexWrap(task, outFile) {
  return `Run EXACTLY this one bash command (the reasoning is Codex's, not yours — do not do it yourself):\n\ntimeout 500 codex exec --skip-git-repo-check --sandbox workspace-write ${JSON.stringify(task)} </dev/null\n\nThen confirm ${outFile} exists and return its first 8 lines. If it failed or produced no file, return exactly "CODEX FAILED".`
}
async function codexPass(task, outFile, label) {
  let r = await agent(codexWrap(task, outFile), { label, phase: label.split(':')[0], model: 'haiku' })
  if (String(r).includes('CODEX FAILED')) { log(`${label}: codex FAILED — retrying once`); r = await agent(codexWrap(task, outFile), { label: `${label}:retry`, model: 'haiku' }) }
  const ok = !String(r).includes('CODEX FAILED')
  if (!ok) log(`${label}: CROSS-FAMILY PASS UNAVAILABLE — downstream consumers must treat ${outFile} as absent`)
  return ok
}

// ---- Phase 1: Behaviors — dispatch skill phase 1; orchestrator keeps the operational guards
phase('Behaviors')
log(`bdd-foundry: behavior-first planning${intent ? ' for: ' + String(intent).slice(0, 100) : ' (picking top ready item)'} → ${DIR} (discipline: ${SKILL})`)
const v1 = await agent(
  `FIRST, before anything: run \`pwd\` and \`git rev-parse --show-toplevel\`. If the resolved plan dir (${DIR}) is relative and cwd is NOT inside a git work tree, STOP and return exactly 'DIR-MISAIM: <pwd>' (your actual pwd) instead of doing any work. (When ${DIR} is absolute this preflight passes trivially — keep it cheap.)\nSECOND: if ${DIR} already exists and contains files, make the base recoverable BEFORE any write: if inside a git work tree, \`git add ${DIR}\` and commit with message 'docs(plans): bdd-foundry pre-run-N base snapshot' IF the repo allows commits (this snapshot commit is the ONE sanctioned commit; \`git stash push --staged\` is FORBIDDEN — it loses worktree state), OTHERWISE copy the dir to ${DIR}.pre-run-backup-<HHMMSS>/. State in your reply (base_protection) which protection you applied (committed / copied / fresh-dir).\n${REGISTER}\nTHIRD: mkdir -p ${DIR}. If in a git worktree, WARN that plan artifacts are landing on this branch.\n${intentClause}\n\nExecute PHASE 1 (Behaviors) of the skill: produce the desired behaviors as concrete, TESTABLE Gherkin (happy + edge + error), write them to ${DIR}/behaviors.md, and return the structured set.`,
  { label: 'behaviors:v1', phase: 'Behaviors', schema: BEHAVIORS_V1_SCHEMA })
// v5/v7 dir-misaim guard: die LOUDLY at phase 1, keyed to the sentinel SLOTS (not a
// whole-payload substring — self-referential planning content legitimately contains it).
const misaim = (typeof v1 === 'string' && v1.trim().startsWith('DIR-MISAIM'))
  ? v1.trim()
  : (v1 && typeof v1 === 'object' && typeof v1.base_protection === 'string' && v1.base_protection.trim().startsWith('DIR-MISAIM')) ? v1.base_protection.trim() : null
if (misaim) throw new Error(`DIR mis-aim: DIR '${DIR}' resolved with cwd OUTSIDE any git work tree — agent reported ${misaim.slice(0, 300)} — aborting at phase 1; pass an absolute dir or launch from inside the repo`)
log(`pre-run base protection: ${(v1 && typeof v1 === 'object' && v1.base_protection) || 'UNREPORTED'}`)
// Cross-family adversarial gap-check — the dimensions + the gate-findings ratchet are the skill's discipline.
const gapsOk = await codexPass(
  `Read ${DIR}/behaviors.md (Gherkin behaviors). You are an INDEPENDENT cross-family ADVERSARY (you did not write them). Execute the cross-family adversarial gap-check from the behavior-first-planning skill (${SKILL}, Phase 1): apply its standing adversarial dimension checklist to every input / trust boundary / mutation / failure path / external-state behavior, AND read any repo-local gate-findings ledger (try docs/gate/findings-ledger.md) and apply its Standing Review Dimensions (the ratchet). List the TOP 15 MISSING scenarios RANKED by how badly an implementer could ship broken while green — concrete (the exact bypass + a runnable repro shape), not an exhaustive dump. Write to ${DIR}/behaviors-codex-gaps.md.`,
  `${DIR}/behaviors-codex-gaps.md`, 'Behaviors:codex-gaps')
const behaviors = await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md${gapsOk ? ` AND the cross-family gap list ${DIR}/behaviors-codex-gaps.md. Disposition EVERY gap explicitly: folded / rejected (one-line reason) / deferred (say where it goes).` : '. NOTE: the cross-family gap pass FAILED — say so; the freeze is single-family this run.'} Rewrite ${DIR}/behaviors.md as the **FROZEN definition of done** and return the complete set${gapsOk ? ' including gap_dispositions for every gap' : ' with gap_dispositions: []'}.`,
  { label: 'behaviors:frozen', phase: 'Behaviors', schema: FROZEN_SCHEMA })
const scenarios = (behaviors && behaviors.scenarios) || []
const scenarioCount = scenarios.length
const dispositions = (behaviors && behaviors.gap_dispositions) || []
if (!scenarioCount) throw new Error('behaviors:frozen returned zero scenarios — nothing to plan')
log(`behaviors frozen: ${scenarioCount} scenarios; gaps dispositioned: ${dispositions.length} (folded ${dispositions.filter(d=>d.action==='folded').length} / rejected ${dispositions.filter(d=>d.action==='rejected').length} / deferred ${dispositions.filter(d=>d.action==='deferred').length})`)

// ---- Phase 2: Acceptance tests — skill phase 2 + the EXECUTED-red deterministic gate
phase('AcceptanceTests')
await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md (the frozen behaviors). Execute PHASE 2 (Acceptance Tests) of the skill: turn EACH scenario into a RUNNABLE test in the project's framework — currently FAILING (red). Write the tests under ${DIR}/acceptance-tests/ and a ${DIR}/acceptance-tests.md index mapping scenario id → test name/path, INCLUDING the exact one-line command that runs the whole suite. Return a one-line summary.`,
  { label: 'acceptance-tests:author', phase: 'AcceptanceTests' })
// THE RED GATE — red is OBSERVED, not asserted (R1 deterministic ground truth).
const redrun = await agent(
  `${REGISTER}\nYou did NOT write these tests. Read ${DIR}/acceptance-tests.md, run its suite command VERBATIM, and report the mechanical result. A test that crashes on syntax/harness errors is NOT a valid red — count those separately.`,
  { label: 'acceptance-tests:red-run', phase: 'AcceptanceTests', schema: REDRUN_SCHEMA })
if (redrun.passing > 0) log(`RED-GATE WARNING: ${redrun.passing} test(s) already PASS at plan time — they assert nothing new; flagged for the spec author`)
if (redrun.errored_outside_assertions > 0) log(`RED-GATE WARNING: ${redrun.errored_outside_assertions} test(s) crash on harness errors — not valid reds`)
if (redrun.failing + redrun.passing + redrun.errored_outside_assertions < scenarioCount) log(`RED-GATE WARNING: suite covers ${redrun.failing + redrun.passing} runnable tests for ${scenarioCount} scenarios`)
log(`red-run observed: ${redrun.failing} red / ${redrun.passing} green / ${redrun.errored_outside_assertions} harness-errored (cmd: ${redrun.run_command.slice(0,80)})`)

// ---- Phase 3: Spec — skill phase 3, derived to make the tests pass
phase('Spec')
await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md + ${DIR}/acceptance-tests.md. Red-run ground truth: ${redrun.failing} red, ${redrun.passing} already-green (fix those tests' assertions if your design makes them meaningful), ${redrun.errored_outside_assertions} harness-broken (repair them). Execute PHASE 3 (Spec) of the skill: design the architecture/spec whose ONLY job is to make those acceptance tests pass — DERIVED from the behaviors+tests, not free-form. Keep it tight. Write to ${DIR}/spec.md. Return only the path.`,
  { label: 'spec', phase: 'Spec' })

// ---- Phase 4: Beadify — skill phase 4 + the JS-computed mechanical gate; NO tracker write yet
phase('Beadify')
const beadResult = await agent(
  `${REGISTER}\nRead ${DIR}/spec.md + ${DIR}/behaviors.md + ${DIR}/acceptance-tests.md. Execute PHASE 4 (Beadify) of the skill: decompose into a dependency-ordered DAG of beads. EVERY bead MUST carry a real scenario_ref and an INVOCABLE acceptance_test (one invocation per test, chained with && when several; never multiple positional filters). Prose-only acceptance is REJECTED by the conductor gate. Cover every frozen scenario. ALSO write the complete bead list as JSON to ${DIR}/beads.json (downstream stages read the FILE). Return the same structured bead list. DO NOT write to the tracker — validation runs first.`,
  { label: 'beadify', phase: 'Beadify', schema: BEADS_SCHEMA })
// THE GATE — conductor-owned, mechanical (R1): runnable shape + real scenario_ref + coverage + cycle-free.
const allBeads = (beadResult && beadResult.beads) || []
const idSet = new Set(scenarios.map((s) => s.id))
const looksRunnable = (t) => /(\bbats\b|\bgo test\b|\bnpm (test|run)\b|\bpytest\b|\bmake \b|\bbash \b|\.bats\b|_test\.|\btest-|acceptance-tests\/)/.test(t || '')
const passed = allBeads.filter((b) => b.acceptance_test && b.acceptance_test.trim().length > 20 && looksRunnable(b.acceptance_test) && idSet.has(b.scenario_ref))
const rejected = allBeads.filter((b) => !passed.includes(b))
const coveredIds = new Set(passed.map((b) => b.scenario_ref))
const uncovered = scenarios.map((s) => s.id).filter((id) => !coveredIds.has(id))
const keys = new Set(passed.map((b) => b.key))
const indeg = {}; const adj = {}
for (const b of passed) { indeg[b.key] = indeg[b.key] || 0; adj[b.key] = adj[b.key] || [] }
for (const b of passed) for (const d of b.deps || []) if (keys.has(d)) { adj[d].push(b.key); indeg[b.key]++ }
let queue = Object.keys(indeg).filter((k) => indeg[k] === 0); let visited = 0
while (queue.length) { const k = queue.shift(); visited++; for (const n of adj[k]) if (--indeg[n] === 0) queue.push(n) }
const cycleFree = visited === passed.length
log(`beadify GATE: ${passed.length}/${allBeads.length} pass (runnable+valid-ref); ${rejected.length} REJECTED; coverage holes: ${uncovered.length ? uncovered.join(',') : 'none'}; DAG ${cycleFree ? 'cycle-free' : 'HAS CYCLES'}`)
if (rejected.length) log(`rejected: ${rejected.map((b) => b.title).join(' | ').slice(0, 300)}`)
if (!passed.length) throw new Error('beadify gate rejected every bead — no acceptance-bearing units to validate')
await agent(
  `${REGISTER}\nWrite ${DIR}/beads-manifest.md describing this PROPOSED bead set (NOT yet in the tracker): for each bead — title, why, scenario_ref, the ACCEPTANCE section (its runnable command), deps by key. Include the conductor's mechanical findings verbatim: coverage holes [${uncovered.join(',') || 'none'}], cycle_free=${cycleFree}, rejected=[${rejected.map((b)=>b.title).join('; ').slice(0,200)}]. Read the FULL bead bodies from ${DIR}/beads.json and describe ONLY these gate-PASSED keys: ${passed.map((b) => b.key).join(', ')}. Return the path.`,
  { label: 'beadify:manifest', phase: 'Beadify' })

// ---- Phase 5: Validate FIRST, write to tracker ONLY on a clearing verdict (R2 terminate-on-verdict)
phase('Validate')
await codexPass(
  `Independently grade the PROPOSED bead set in ${DIR}/beads-manifest.md against ${DIR}/behaviors.md. For each bead: is the acceptance a CONCRETE INVOCABLE test (not prose, not "see spec")? Output 'X/N crank-ready', list the thin ones, and the single biggest systemic gap. Write to ${DIR}/validate-codex.md.`,
  `${DIR}/validate-codex.md`, 'Validate:codex')
const verdict = await agent(
  `${REGISTER}\nPHASE 5 — INDEPENDENT VALIDATION (you did NOT write these beads; they are NOT in the tracker yet). Read ${DIR}/beads-manifest.md, ${DIR}/behaviors.md, and the cross-family pass ${DIR}/validate-codex.md (if absent, say so). Conductor ground truth (do not re-derive, do audit): coverage holes=[${uncovered.join(',') || 'none'}], cycle_free=${cycleFree}, red-run=${redrun.failing}r/${redrun.passing}g. Pull holdout grading scenarios via \`ao scenario\` if available and grade against them; report which you used. Verify each bead's acceptance is invocable (spot-run at least 2). Score crank-readiness 0-1 and name the biggest gap.`,
  { label: 'validate:judge', phase: 'Validate', schema: VALIDATE_SCHEMA })
const score = (verdict && verdict.score) || 0
// THE MECHANICAL DRIFT-GUARD (R1) — each ACCEPTANCE command must RESOLVE TO EXACTLY ONE
// unignored test, RUN (listed) not asserted. The gate is the run, not a prompt.
const drift = await agent(
  `${REGISTER}\nMECHANICAL DRIFT-GUARD — the gate is the RUN, not your word. Read ${DIR}/beads.json (if ABSENT/unparseable: return all_pass=false, checked=0, failures=[{key:'-',lists:0,reason:'beads.json missing/invalid'}]). For EACH gate-passed key [${passed.map((b) => b.key).join(', ')}], extract its acceptance command and RUN IT IN LIST MODE ONLY (cargo: \`-- --list\`, pytest: \`--collect-only -q\`, go: \`-list '.*'\`, bats: \`--count\`); do NOT run the tests. Assert each resolves to EXACTLY ONE unignored test — not 0, not 2+, not an arg/parse error. Also assert 1:1 parity: gate-passed keys == beads in ${DIR}/beads-manifest.md == distinct tests named. all_pass=true ONLY if every command lists exactly one AND parity holds; put every offender in failures with its listed count and exact error.`,
  { label: 'drift-guard', phase: 'Validate', schema: DRIFT_SCHEMA })
const driftOk = !!(drift && drift.all_pass === true)
if (!driftOk) log(`DRIFT-GUARD FAILED (${(drift && drift.failures || []).length} offenders) — tracker write BLOCKED: ${(drift && drift.failures || []).map((f) => `${f.key}:lists=${f.lists}`).join(' ').slice(0, 200)}`)
const clears = score >= SCORE_THRESHOLD && cycleFree && uncovered.length === 0 && driftOk
if (DRY_RUN) {
  log(`DRY-RUN: gates score=${score}/${SCORE_THRESHOLD}, drift_guard=${driftOk}, coverage_holes=${uncovered.length}, cycle_free=${cycleFree} — verified, NOT writing to the tracker`)
} else if (clears) {
  await agent(
    `${REGISTER}\nVerdict cleared (${score} ≥ ${SCORE_THRESHOLD}). Write the ${passed.length} gate-PASSED beads into the tracker — read full bodies from ${DIR}/beads.json (only these keys cleared the gate: ${passed.map((b) => b.key).join(', ')}); ${DIR}/beads-manifest.md is the human summary. Use EXACTLY \`${TRACKER_CMD}\` (resolve to the main checkout if in a worktree — never fork the DB). Each self-contained: title, why, scenario, explicit **ACCEPTANCE** section. Wire deps per the manifest. Overlap-check existing open beads first; merge, don't duplicate. Update ${DIR}/beads-manifest.md with created ids. Return the count created.`,
    { label: 'tracker:write', phase: 'Validate' })
} else {
  // R2: route to a single bounded repair pass, then STOP (no unbounded loop) — operator re-validates.
  await agent(
    `${REGISTER}\nVerdict did NOT clear (score=${score}/${SCORE_THRESHOLD}, coverage holes=${uncovered.length}, cycle_free=${cycleFree}, drift_guard=${driftOk ? 'pass' : 'FAIL'}).${driftOk ? '' : ` DRIFT offenders — each ACCEPTANCE command must list exactly ONE test: ${(drift && drift.failures || []).map((f) => `${f.key}(listed ${f.lists}: ${f.reason})`).join('; ').slice(0, 300)}. Fix these to one-invocation-per-test FIRST.`} Biggest gap: ${verdict && verdict.biggest_gap}. Do ONE repair pass on ${DIR}/beads-manifest.md: rewrite the thin acceptance sections named in ${DIR}/validate-codex.md, fix coverage/cycles. Mark the manifest header 'REPAIRED — needs operator re-validation before tracker write'. Do NOT write to the tracker. Return what you changed.`,
    { label: 'validate:repair', phase: 'Validate' })
  log(`verdict ${score} below threshold ${SCORE_THRESHOLD} (or gate fail) — beads NOT written; manifest repaired for operator review`)
}

// ---- R4 escapes→slow-loop — EMIT only (the derive/govern half is age-cwo).
// The yield ledger pairs an UPSTREAM CONFIRMED (the JS-computed beadify gate accepted the
// bead set) with a DOWNSTREAM REFUTED at a higher attempt when the cross-family validate
// score or the mechanical drift-guard catches a defect the beadify gate passed. That
// CONFIRMED→REFUTED pair for the same bead+run is what `ao membrane derive-checks` (age-zqc)
// surfaces and compiles into the check that would have caught it. The gate-verdict body is
// COMMIT-BOUND (ao validates pawl_verdict_ref + head_sha ≥7 + mode + author_family), so the
// emit agent captures the run's current HEAD as the sha. Skipped under DRY_RUN (pure
// verification — no persistent-state mutation). EMIT only; never derive or tune a gate (R3).
const r4Bead = `bdd-foundry-${RUN_TAG}`
const beadifyGreen = passed.length > 0 && cycleFree && uncovered.length === 0
// pairGuard (optional) is the orphan-escape guard for the downstream REFUTED@2: emit is
// fail-open observability, so the orchestrator cannot branch on a prior emit's exit code
// (the agent returns text, not a status). The robust place to prevent an orphaned escape
// (a REFUTED@2 with no paired CONFIRMED@1, which DetectEscapes would silently drop) is the
// emit layer itself — the agent re-asserts the attempt-1 CONFIRMED before the REFUTED, and
// `ao yield emit` is append-idempotent-safe to re-run. (Strict improvement over the
// operating-loop.js reference idiom; backport candidate.)
const emitVerdict = (disposition, attempt, ctx, note, label, pairGuard) => agent(
  `${REGISTER}\nR4 gate-verdict (workflow-side EMIT only). ${note}\nDo exactly this, nothing else:${pairGuard ? ` (0) ${pairGuard}` : ''} (1) capture the current commit with \`SHA=$(git rev-parse HEAD)\`; (2) run\n  ao yield emit gate-verdict --bead ${JSON.stringify(r4Bead)} --run ${JSON.stringify(r4Bead)} --json '{"difficulty":2,"pawl_verdict_ref":{"bead_id":${JSON.stringify(r4Bead)},"head_sha":"'"$SHA"'"},"disposition":"${disposition}","head_sha":"'"$SHA"'","attempt":${attempt},"mode":"deterministic","author_context_id":"${ctx}","refuter_families":[],"author_family":"bdd-foundry-gate","cross_family":false,"author_ne_reviewer":true,"evidence_present":true}'\nReturn the command's exit code (0 = appended). Do NOT run \`ao membrane derive-checks\` or otherwise build/tune the sink — that is the slow loop's job (age-cwo).`,
  { label, phase: 'Validate' })
let escapeEmitted = false
if (DRY_RUN) {
  log(`R4: DRY-RUN — gate-verdict emit skipped (no persistent-state mutation in verification mode)`)
} else if (beadifyGreen) {
  // Upstream confirm: the JS-computed beadify gate accepted the whole bead set.
  await emitVerdict('CONFIRMED', 1, 'bdd-foundry-beadify-gate', `The beadify gate accepted the bead set for ${r4Bead} (runnable+valid-ref, cycle-free, fully covering) — record the upstream CONFIRM.`, 'r4:confirm')
  if (!clears) {
    // Downstream catch of an upstream-confirmed unit = an ESCAPE. Emit REFUTED at attempt 2.
    const why = !driftOk ? `drift-guard REFUTED (${(drift && drift.failures || []).length} acceptance command(s) did not resolve 1:1)` : `cross-family validate score ${score} < ${SCORE_THRESHOLD}`
    await emitVerdict('REFUTED', 2, 'bdd-foundry-validate', `The beadify gate accepted the bead set but the DOWNSTREAM validate phase caught a defect: ${why}. This is an escape — a unit the upstream beadify gate CONFIRMED that a stricter downstream gate REFUTED at a higher attempt; the CONFIRMED→REFUTED pair is what the slow loop consumes.`, 'r4:escape-emit', `ORPHAN-ESCAPE GUARD: first confirm the attempt-1 CONFIRMED gate-verdict for bead ${JSON.stringify(r4Bead)} run ${JSON.stringify(r4Bead)} is already in the yield ledger (it was just emitted upstream). If it is ABSENT (the upstream emit failed), re-emit that exact attempt-1 CONFIRMED body FIRST so this REFUTED@2 is never an orphan with no pair; re-emitting is append-safe.`)
    escapeEmitted = true
    log(`R4: escape emitted — the downstream validate phase caught a defect the beadify gate passed (${why}); REFUTED gate-verdict (attempt 2) paired with the attempt-1 CONFIRMED for ${r4Bead}. Slow loop (ao membrane derive-checks --run ${r4Bead}) derives the catch.`)
  } else {
    log(`R4: no escape — beadify gate accepted and the validate phase cleared for ${r4Bead}; single upstream CONFIRM recorded.`)
  }
} else {
  // Beadify gate did NOT fully accept (coverage holes / cycles) — a direct fail, not an
  // escape (no upstream CONFIRM to pair against). Record the downstream REFUTED at attempt 1
  // so the deterministic catch still counts.
  await emitVerdict('REFUTED', 1, 'bdd-foundry-validate', `The beadify gate did NOT accept the bead set for ${r4Bead} (coverage holes / cycles) — a direct fail, not an escape.`, 'r4:refute')
}

return {
  escapeEmitted,
  verdict, score_threshold: SCORE_THRESHOLD, dry_run: DRY_RUN, tracker_written: !DRY_RUN && score >= SCORE_THRESHOLD && cycleFree && uncovered.length === 0 && driftOk, drift_guard: driftOk, drift_failures: (drift && drift.failures) || [],
  gate: { passed: passed.length, rejected: rejected.map((b) => b.title), coverage_holes: uncovered, cycle_free: cycleFree },
  red_run: redrun, scenarios: scenarioCount, gaps_dispositioned: dispositions.length, codex_gap_pass_ok: gapsOk,
}
