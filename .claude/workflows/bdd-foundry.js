// ─── bdd-foundry v8 (2026-06-12) ──────────────────────────────────────────────
// v8: ADVERSARIAL-BY-CONSTRUCTION GAP-CHECK — the Behaviors cross-family gap pass was
//     generic ('top 15 missing edge cases') and let security/correctness BYPASS classes slip
//     to the gate, where each costs a full review+re-fix+re-gate PER BEAD. The pass is now an
//     ADVERSARY applying a standing dimension checklist (fail-closed-every-path / no-forgeable-
//     marker / no-raw-untrusted-string / enforce-at-sink / no-overclaiming-test / input-channel-
//     variants) to every trust boundary, AND reads docs/gate/findings-ledger.md so every gate
//     finding permanently upgrades the factory (the ratchet). Catch when tokens are cheap.
//     [ref mto-e4pv; orig 3b7ab6d0a by the concurrent lane, Co-Authored Opus 4.8]
// v7: SENTINEL-SLOT GUARD — the v5 DIR-misaim check substring-matched 'DIR-MISAIM'
//     across the WHOLE structured payload, so a SELF-REFERENTIAL run (planning content
//     ABOUT this guard — its Gherkin greps for the sentinel) false-positived and aborted
//     phase 1 while base_protection='fresh-dir' (preflight had PASSED). The check now
//     keys to the sentinel SLOTS only: a bare-string reply or base_protection starting
//     with 'DIR-MISAIM'. [observed live, canonicalization run-3 wf_b26ef26c 2026-06-12]
// v6: ARGS DE-STRINGIFY — the skill/name-resolved launch path delivers args as a STRING
//     even when the caller passed a JSON object, silently defaulting every named param
//     (tracker_cmd → bare br, dir → cwd-relative, threshold, run_tag). JSON-looking
//     string args are now parsed before use. [observed live, run-2 2026-06-12]
// v5: DIR MIS-AIM GUARD — conductors have no cwd/fs, so the FIRST agent call runs a
//     pwd + git-toplevel preflight; relative DIR outside a git work tree returns
//     'DIR-MISAIM: <pwd>' and the conductor THROWS at phase 1 (no artifacts built in
//     the wrong place). Plus RE-RUN BASE SNAPSHOT — a pre-existing non-empty DIR is
//     made recoverable BEFORE any write: committed ('docs(plans): bdd-foundry
//     pre-run-N base snapshot' — the ONE sanctioned agent commit; staged-stash
//     FORBIDDEN) or copied to <DIR>.pre-run-backup-<HHMMSS>/; protection logged
//     (committed/copied/fresh-dir). [both from live incidents 2026-06-12]
// v4: MECHANICAL DRIFT-GUARD — before any tracker write, each ACCEPTANCE command is RUN
//     in list-mode and must resolve to EXACTLY ONE unignored test (multi-filter commands,
//     manifest<->test desync, and an absent beads.json all caught); br write BLOCKED on
//     failure. The pawl is a RUN, not a prompt. [red-team-until-dry doctrine]
// v3: truncation fix COMPLETE — beadify writes <DIR>/beads.json; the manifest AND
//     tracker-write stages now read that FILE filtered to the gate-passed keys (the
//     slice(0,14000) prompt payload is gone, so coverage no longer silently drops
//     past ~14KB — the bug that dropped pillars D-G in the first dogfood run).
//     Plus the one-invocation-per-test command rule (multi-filter cargo/pytest/
//     go-test calls arg-error before any test runs).
// v2: red-team patchset — mechanical gate, executed red-run, validate-before-write,
//     codex-failure handling, per-run dirs.
// CANONICAL: .claude/workflows/bdd-foundry.js (agentops repo, git-tracked); ~/.claude/workflows/bdd-foundry.js is a symlink/copy installed via scripts/install-workflows.sh
export const meta = {
  name: 'bdd-foundry',
  description: 'Behavior-first planning: intent → Gherkin behaviors → failing acceptance tests (EXECUTED red) → spec → acceptance-gated bead DAG → independent cross-family validation BEFORE tracker write. No runnable acceptance test, no bead.',
  whenToUse: 'When a feature/spec deserves rigorous planning and the beads must be genuinely crank-ready — each carrying a runnable acceptance test that defines "done". The behavior-first successor to plan-foundry: fixes the spec-first failure where beads ship with no done-criteria (the 3/10 problem). Triggers — "bdd-foundry", "plan behavior-first", "acceptance-first planning". Holdout grading per the external measurement register (judges pull holdout scenarios at validate time; implementing lanes never see them — ids deliberately not listed here).',
  phases: [
    { title: 'Behaviors', detail: 'intent → Gherkin (happy+edge+error), cross-family gap-check (top-15, dispositioned) → frozen definition of done' },
    { title: 'AcceptanceTests', detail: 'each scenario → a runnable FAILING test — then the conductor RUNS the suite and verifies red' },
    { title: 'Spec', detail: 'architecture derived to make the tests pass (not free-form)' },
    { title: 'Beadify', detail: 'dep-ordered DAG; conductor gate: runnable acceptance + valid scenario_ref + coverage + cycle-free, computed in JS' },
    { title: 'Validate', detail: 'independent cross-family review of the MANIFEST (author≠reviewer); tracker write happens ONLY after the verdict clears threshold' },
  ],
}

// ---- inputs (args: string intent, or {intent, tracker, tracker_cmd, dir, score_threshold}) ----
// v6: the skill/name-resolved launch path stringifies args — a JSON object arrives as a
// string, silently defaulting EVERY named param (observed live: run-2's dir slug was the
// slug of the whole JSON blob). Parse JSON-looking strings before treating args as intent.
if (typeof args === 'string' && /^\s*\{/.test(args)) { try { args = JSON.parse(args) } catch (e) { log('args looked like JSON but failed to parse — treating as plain intent string') } }
const intent = (args && (typeof args === 'string' ? args : args.intent)) || null
const TRACKER = (args && typeof args === 'object' && args.tracker) || 'br'
// tracker_cmd: the FULL invocation incl. env/path quirks (e.g. 'BEADS_DIR=/abs/path/_beads br').
// Worktree rule: br from a worktree forks the bead DB — tracker_cmd should point at the main checkout.
const TRACKER_CMD = (args && typeof args === 'object' && args.tracker_cmd) || TRACKER
const SCORE_THRESHOLD = (args && typeof args === 'object' && args.score_threshold) || 0.7
const slug = (s) => String(s || 'ready-pick').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
const RUN_TAG = (args && typeof args === 'object' && args.run_tag) || slug(intent) // per-run dir: no cross-run clobber
const DRY_RUN = !!(args && typeof args === 'object' && args.dry_run === true) // verification mode: run every gate EXCEPT the tracker write (no bead pollution from test runs)
const DIR = (args && typeof args === 'object' && args.dir) || `docs/plans/bdd-foundry/${RUN_TAG}`
const intentClause = intent
  ? `INTENT (the capability to plan): ${intent}`
  : `No intent passed — run \`${TRACKER_CMD} ready\`, pick the top unblocked item, treat its title+description as the intent, and state which id you picked.`
const REGISTER = `Keep the project's vocabulary register. Tracker invocation is EXACTLY \`${TRACKER_CMD}\` (never bare \`${TRACKER}\` if that differs; never from a worktree — if \`git rev-parse --git-dir\` ≠ \`git rev-parse --git-common-dir\` you are in a worktree: run tracker commands from the main checkout and STATE the resolved path). LAW: never \`claude -p\`/\`--print\`; \`codex exec\` is the cross-family sub. DO NOT git-commit (the conductor/operator commits).`

// ---- schemas (structured output = what makes the gate mechanical) -----------
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

// v5: behaviors:v1 only — same shape plus an OPTIONAL base_protection report slot
// (which pre-run protection was applied: committed / copied: <path> / fresh-dir —
// or the DIR-MISAIM sentinel if the preflight aborted). Not required: v4 consumers unaffected.
const BEHAVIORS_V1_SCHEMA = { ...BEHAVIORS_SCHEMA, properties: { ...BEHAVIORS_SCHEMA.properties, base_protection: { type: 'string', description: "pre-run base protection applied: 'committed' / 'copied: <backup path>' / 'fresh-dir' — or exactly 'DIR-MISAIM: <pwd>' when the preflight aborts" } } }

// freeze must account for every cross-family gap — silent drops are detectable
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
        properties: {
          gap: { type: 'string' },
          action: { type: 'string', enum: ['folded', 'rejected', 'deferred'] },
          detail: { type: 'string' },
        },
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
          acceptance_test: { type: 'string', description: 'INVOCABLE command + test path (under the acceptance-tests dir or the project test tree) that defines done for THIS bead. ONE invocation per test — never pass multiple positional test-name filters to a single cargo/pytest/go-test call (arg error before any test runs); chain with && if a bead has several. Prose Gherkin alone is REJECTED by the gate.' },
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
    total: { type: 'integer' },
    with_runnable_acceptance: { type: 'integer' },
    crank_ready: { type: 'integer' },
    score: { type: 'number' },
    biggest_gap: { type: 'string' },
  },
}

// off-account cross-family pass — the reasoning is Codex's, the wrapper just runs it.
// workspace-write (NOT danger-full-access): review tasks read one file and write one file.
function codexWrap(task, outFile) {
  return `Run EXACTLY this one bash command (the reasoning is Codex's, not yours — do not do it yourself):\n\ntimeout 500 codex exec --skip-git-repo-check --sandbox workspace-write ${JSON.stringify(task)} </dev/null\n\nThen confirm ${outFile} exists and return its first 8 lines. If it failed or produced no file, return exactly "CODEX FAILED".`
}
// conductor-owned cross-family enforcement: retry once, then degrade LOUDLY
async function codexPass(task, outFile, label) {
  let r = await agent(codexWrap(task, outFile), { label, phase: label.split(':')[0], model: 'haiku' })
  if (String(r).includes('CODEX FAILED')) {
    log(`${label}: codex FAILED — retrying once`)
    r = await agent(codexWrap(task, outFile), { label: `${label}:retry`, model: 'haiku' })
  }
  const ok = !String(r).includes('CODEX FAILED')
  if (!ok) log(`${label}: CROSS-FAMILY PASS UNAVAILABLE — downstream consumers must treat ${outFile} as absent`)
  return ok
}

// ---- Phase 1: Behaviors — define DONE first, cross-family gap-checked early --
phase('Behaviors')
log(`bdd-foundry: behavior-first planning${intent ? ' for: ' + String(intent).slice(0, 100) : ' (picking top ready item)'} → ${DIR}`)
const v1 = await agent(
  `FIRST, before anything: run \`pwd\` and \`git rev-parse --show-toplevel\`. If the resolved plan dir (${DIR}) is relative and cwd is NOT inside a git work tree, STOP and return exactly 'DIR-MISAIM: <pwd>' (your actual pwd) instead of doing any work. (When ${DIR} is absolute this preflight passes trivially — keep it cheap.)\nSECOND: if ${DIR} already exists and contains files, make the base recoverable BEFORE any write: if the files are inside a git work tree, \`git add <DIR> && git stash push --staged -m 'bdd-foundry pre-run base'\` is FORBIDDEN (loses worktree state) — instead \`git add ${DIR}\` and commit them with message 'docs(plans): bdd-foundry pre-run-N base snapshot' IF the repo allows commits (this snapshot commit is the ONE sanctioned commit; everything else still forbidden — the DO-NOT-git-commit rule below holds for all other work), OTHERWISE copy the dir to ${DIR}.pre-run-backup-<HHMMSS>/. State in your reply (base_protection) which protection you applied (committed / copied / fresh-dir).\n${REGISTER}\nTHIRD: mkdir -p ${DIR}. If you are in a git worktree (check \`git rev-parse --git-dir\` vs \`--git-common-dir\`), WARN in your reply that plan artifacts are landing on this branch.\n${intentClause}\n\nPhase 1 — BEHAVIORS. Define what DONE means BEFORE any design. Produce the desired behaviors as concrete, TESTABLE Gherkin scenarios — happy paths AND edge AND error cases. Every Given/When/Then must be specific enough to become a runnable test (no vague "works correctly"). Write them to ${DIR}/behaviors.md and return the structured set.`,
  { label: 'behaviors:v1', phase: 'Behaviors', schema: BEHAVIORS_V1_SCHEMA })
// v5 DIR mis-aim guard: die LOUDLY at phase 1 instead of building artifacts in the wrong place.
// v7: keyed to the sentinel SLOTS (bare-string reply / base_protection), NOT a whole-payload
// substring — self-referential planning content legitimately contains 'DIR-MISAIM'.
const misaim = (typeof v1 === 'string' && v1.trim().startsWith('DIR-MISAIM'))
  ? v1.trim()
  : (v1 && typeof v1 === 'object' && typeof v1.base_protection === 'string' && v1.base_protection.trim().startsWith('DIR-MISAIM'))
    ? v1.base_protection.trim()
    : null
if (misaim) {
  throw new Error(`DIR mis-aim: DIR '${DIR}' resolved with cwd OUTSIDE any git work tree — agent reported ${misaim.slice(0, 300)} — aborting at phase 1; pass an absolute dir or launch from inside the repo`)
}
// v5 re-run base snapshot: log which protection the agent applied to a pre-existing DIR
log(`pre-run base protection: ${(v1 && typeof v1 === 'object' && v1.base_protection) || 'UNREPORTED (agent did not state committed/copied/fresh-dir)'}`)
const gapsOk = await codexPass(
  `Read ${DIR}/behaviors.md (Gherkin behaviors). You are an INDEPENDENT cross-family ADVERSARY (you did not write them). Your job: find where an implementer ships BROKEN while satisfying every written scenario — ESPECIALLY security/correctness BYPASSES a green test would miss (these are expensive at the gate, cheap here).\n\nApply this ADVERSARIAL DIMENSION CHECKLIST to EVERY behavior touching an input, a trust boundary, a mutation/write surface, a failure path, or external state. For each applicable class emit the MISSING attack-vector scenario:\n - FAIL-CLOSED on EVERY failure path: unparseable input, missing/absent dependency, substrate/IO error, partial/malformed/timeout response → ABORT/BLOCK, never silent-pass or silent-skip-and-continue.\n - NO FORGEABLE TRUST MARKER: never trust a caller-settable signal (env var, flag, header, marker file) as proof a check ran — re-derive/verify provenance.\n - NO RAW UNTRUSTED STRING past a boundary: canonicalize/encode before display, argv, or serialization (a value with quote, backslash, newline, shell metachar, unicode/case).\n - ENFORCE AT THE SINK, not the source: last-wins / passthrough-after-'--' / override vectors; the component that ACTS must be the one that validates.\n - NO OVERCLAIMING TEST: a property proven only under harness conditions (injected PATH/env, scratch stub) is NOT a live/production proof — the production path needs its own scenario.\n - INPUT-CHANNEL variants of every surface: stdin vs argv, heredoc, file vs inline, symlink/case/unicode path aliases, TOCTOU lookup-to-write race, nesting/depth.\nALSO read any repo-local gate-findings ledger (try docs/gate/findings-ledger.md) and apply its Standing Review Dimensions — those are real defects this gate already caught; do not let them recur (this is the ratchet: every gate finding upgrades this factory).\n\nList the TOP 15 MISSING scenarios RANKED by how badly an implementer could ship broken while green; concrete and specific (the exact bypass + a runnable repro shape), not an exhaustive dump. Write to ${DIR}/behaviors-codex-gaps.md.`,
  `${DIR}/behaviors-codex-gaps.md`, 'Behaviors:codex-gaps')
const behaviors = await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md${gapsOk ? ` AND the cross-family gap list ${DIR}/behaviors-codex-gaps.md. Disposition EVERY gap explicitly: folded (add the scenario), rejected (one-line reason), or deferred (out of scope, say where it goes).` : '. NOTE: the cross-family gap pass FAILED — say so in the doc; the freeze is single-family this run.'} Rewrite ${DIR}/behaviors.md as the **FROZEN definition of done** and return the complete set${gapsOk ? ' including gap_dispositions for every gap in the list' : ' with gap_dispositions: []'}.`,
  { label: 'behaviors:frozen', phase: 'Behaviors', schema: FROZEN_SCHEMA })
const scenarios = (behaviors && behaviors.scenarios) || []
const scenarioCount = scenarios.length
const dispositions = (behaviors && behaviors.gap_dispositions) || []
if (!scenarioCount) throw new Error('behaviors:frozen returned zero scenarios — nothing to plan')
log(`behaviors frozen: ${scenarioCount} scenarios; gaps dispositioned: ${dispositions.length} (folded ${dispositions.filter(d=>d.action==='folded').length} / rejected ${dispositions.filter(d=>d.action==='rejected').length} / deferred ${dispositions.filter(d=>d.action==='deferred').length})`)

// ---- Phase 2: Acceptance tests — Gherkin → runnable FAILING tests, EXECUTED --
phase('AcceptanceTests')
await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md (the frozen behaviors). Phase 2 — ACCEPTANCE TESTS (ATDD, test-first). Turn EACH scenario into a RUNNABLE test in the project's test framework — currently FAILING (red), because the feature isn't built yet. The test IS the executable definition of done. Write the tests under ${DIR}/acceptance-tests/ and a ${DIR}/acceptance-tests.md index mapping scenario id → test name/path, INCLUDING the exact one-line command that runs the whole suite. Return a one-line summary.`,
  { label: 'acceptance-tests:author', phase: 'AcceptanceTests' })
// THE RED GATE — red is observed, not asserted
const redrun = await agent(
  `${REGISTER}\nYou did NOT write these tests. Read ${DIR}/acceptance-tests.md, run its suite command VERBATIM, and report the mechanical result. A test that crashes on syntax/harness errors is NOT a valid red — count those separately.`,
  { label: 'acceptance-tests:red-run', phase: 'AcceptanceTests', schema: REDRUN_SCHEMA })
if (redrun.passing > 0) log(`RED-GATE WARNING: ${redrun.passing} test(s) already PASS at plan time — they assert nothing new; flagged for the spec author`)
if (redrun.errored_outside_assertions > 0) log(`RED-GATE WARNING: ${redrun.errored_outside_assertions} test(s) crash on harness errors — not valid reds`)
if (redrun.failing + redrun.passing + redrun.errored_outside_assertions < scenarioCount) log(`RED-GATE WARNING: suite covers ${redrun.failing + redrun.passing} runnable tests for ${scenarioCount} scenarios`)
log(`red-run observed: ${redrun.failing} red / ${redrun.passing} green / ${redrun.errored_outside_assertions} harness-errored (cmd: ${redrun.run_command.slice(0,80)})`)

// ---- Phase 3: Spec — derived to make the tests pass -------------------------
phase('Spec')
await agent(
  `${REGISTER}\nRead ${DIR}/behaviors.md + ${DIR}/acceptance-tests.md. Red-run ground truth: ${redrun.failing} red, ${redrun.passing} already-green (fix those tests' assertions if your design makes them meaningful), ${redrun.errored_outside_assertions} harness-broken (repair them). Phase 3 — SPEC. Design the architecture/spec whose ONLY job is to make those acceptance tests pass — DERIVED from the behaviors+tests, not free-form. For each behavior, name the components/changes that satisfy it. Keep it tight: a spec, not a monument. Write to ${DIR}/spec.md. Return only the path.`,
  { label: 'spec', phase: 'Spec' })

// ---- Phase 4: Beadify — conductor-computed gates; NO tracker write yet -------
phase('Beadify')
const beadResult = await agent(
  `${REGISTER}\nRead ${DIR}/spec.md + ${DIR}/behaviors.md + ${DIR}/acceptance-tests.md. Phase 4 — BEADIFY. Decompose into a dependency-ordered DAG of beads. EVERY bead MUST carry: the scenario_ref it delivers (a real frozen scenario id), and an acceptance_test that is an INVOCABLE command + test path (from ${DIR}/acceptance-tests.md or the project test tree) — ONE invocation per test, chained with && when a bead has several; never multiple positional test-name filters in one call. Prose-only acceptance is REJECTED by the conductor gate. Cover every frozen scenario. ALSO write the complete bead list as JSON to ${DIR}/beads.json (downstream stages read the FILE — prompt payloads truncate). Return the same structured bead list. DO NOT write to the tracker — validation runs first.`,
  { label: 'beadify', phase: 'Beadify', schema: BEADS_SCHEMA })

// THE GATE — conductor-owned, mechanical: runnability shape + real scenario_ref
const allBeads = (beadResult && beadResult.beads) || []
const idSet = new Set(scenarios.map((s) => s.id))
const looksRunnable = (t) => /(\bbats\b|\bgo test\b|\bnpm (test|run)\b|\bpytest\b|\bmake \b|\bbash \b|\.bats\b|_test\.|\btest-|acceptance-tests\/)/.test(t || '')
const passed = allBeads.filter((b) => b.acceptance_test && b.acceptance_test.trim().length > 20 && looksRunnable(b.acceptance_test) && idSet.has(b.scenario_ref))
const rejected = allBeads.filter((b) => !passed.includes(b))
// coverage + cycle check: computed, not self-reported
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
// manifest written by an agent (conductor has no fs) — tracker still untouched
await agent(
  `${REGISTER}\nWrite ${DIR}/beads-manifest.md describing this PROPOSED bead set (NOT yet in the tracker): for each bead — title, why, scenario_ref, the ACCEPTANCE section (its runnable command), deps by key. Include the conductor's mechanical findings verbatim: coverage holes [${uncovered.join(',') || 'none'}], cycle_free=${cycleFree}, rejected=[${rejected.map((b)=>b.title).join('; ').slice(0,200)}]. Beads JSON:\n(read the FULL bead bodies from ${DIR}/beads.json and describe ONLY these gate-PASSED keys: ${passed.map((b) => b.key).join(', ')})\nReturn the path.`,
  { label: 'beadify:manifest', phase: 'Beadify' })

// ---- Phase 5: Validate FIRST, write to tracker ONLY on a clearing verdict ----
phase('Validate')
await codexPass(
  `Independently grade the PROPOSED bead set in ${DIR}/beads-manifest.md against ${DIR}/behaviors.md. For each bead: is the acceptance a CONCRETE INVOCABLE test (not prose, not "see spec")? Output 'X/N crank-ready', list the thin ones, and the single biggest systemic gap. Write to ${DIR}/validate-codex.md.`,
  `${DIR}/validate-codex.md`, 'Validate:codex')
const verdict = await agent(
  `${REGISTER}\nPhase 5 — INDEPENDENT VALIDATION (you did NOT write these beads; they are NOT in the tracker yet). Read ${DIR}/beads-manifest.md, ${DIR}/behaviors.md, and the cross-family pass ${DIR}/validate-codex.md (if absent, say so). Conductor ground truth (do not re-derive, do audit): coverage holes=[${uncovered.join(',') || 'none'}], cycle_free=${cycleFree}, red-run=${redrun.failing}r/${redrun.passing}g. Pull holdout grading scenarios via \`ao scenario\` if available and grade against them; report which you used. Verify each bead's acceptance is invocable (spot-run at least 2). Score crank-readiness 0-1 and name the biggest gap.`,
  { label: 'validate:judge', phase: 'Validate', schema: VALIDATE_SCHEMA })
const score = (verdict && verdict.score) || 0

// THE MECHANICAL DRIFT-GUARD (v4) — each ACCEPTANCE command must RESOLVE TO EXACTLY ONE
// unignored test, RUN (listed) not asserted: catches multi-filter commands, missing tests,
// manifest<->test desync, and an absent beads.json. The gate is the run, not a prompt.
const DRIFT_SCHEMA = { type: 'object', additionalProperties: false, required: ['all_pass', 'checked', 'failures'], properties: { all_pass: { type: 'boolean' }, checked: { type: 'integer' }, failures: { type: 'array', items: { type: 'object', additionalProperties: false, required: ['key', 'lists', 'reason'], properties: { key: { type: 'string' }, lists: { type: 'integer' }, reason: { type: 'string' } } } } } }
const drift = await agent(
  `${REGISTER}\nMECHANICAL DRIFT-GUARD — the gate is the RUN, not your word. Read ${DIR}/beads.json (if ABSENT or unparseable: return all_pass=false, checked=0, failures=[{key:'-',lists:0,reason:'beads.json missing/invalid'}]). For EACH gate-passed key [${passed.map((b) => b.key).join(', ')}], extract its acceptance command and RUN IT IN LIST MODE ONLY (append the framework's list flag — cargo: \`-- --list\`, pytest: \`--collect-only -q\`, go: \`-list '.*'\`, bats: \`--count\`); do NOT run the tests. Assert each resolves to EXACTLY ONE unignored test — not 0, not 2+, not an arg/parse error (the v2 multi-filter bug errors instead of listing). Also assert 1:1 parity: gate-passed keys == beads in ${DIR}/beads-manifest.md == distinct tests named. all_pass=true ONLY if every command lists exactly one AND parity holds; put every offender in failures with its listed count and exact error.`,
  { label: 'drift-guard', phase: 'Validate', schema: DRIFT_SCHEMA })
const driftOk = !!(drift && drift.all_pass === true)
if (!driftOk) log(`DRIFT-GUARD FAILED (${(drift && drift.failures || []).length} offenders) — tracker write BLOCKED: ${(drift && drift.failures || []).map((f) => `${f.key}:lists=${f.lists}`).join(' ').slice(0, 200)}`)
if (DRY_RUN) {
  log(`DRY-RUN: gates score=${score}/${SCORE_THRESHOLD}, drift_guard=${driftOk}, coverage_holes=${uncovered.length}, cycle_free=${cycleFree} — verified, NOT writing to the tracker (no bead pollution from a verification run)`)
} else if (score >= SCORE_THRESHOLD && cycleFree && uncovered.length === 0 && driftOk) {
  await agent(
    `${REGISTER}\nVerdict cleared (${score} ≥ ${SCORE_THRESHOLD}). Write the ${passed.length} gate-PASSED beads into the tracker — read full bodies from ${DIR}/beads.json (only these keys cleared the gate: ${passed.map((b) => b.key).join(', ')}); ${DIR}/beads-manifest.md is the human summary using EXACTLY \`${TRACKER_CMD}\` (resolve to the main checkout if in a worktree — never fork the DB). Each self-contained: title, why, scenario, explicit **ACCEPTANCE** section. Wire deps per the manifest. Overlap-check existing open beads first; merge, don't duplicate. Update ${DIR}/beads-manifest.md with created ids. Return the count created.`,
    { label: 'tracker:write', phase: 'Validate' })
} else {
  await agent(
    `${REGISTER}\nVerdict did NOT clear (score=${score}/${SCORE_THRESHOLD}, coverage holes=${uncovered.length}, cycle_free=${cycleFree}, drift_guard=${driftOk ? 'pass' : 'FAIL'}).${driftOk ? '' : ` DRIFT offenders — each ACCEPTANCE command must list exactly ONE test: ${(drift && drift.failures || []).map((f) => `${f.key}(listed ${f.lists}: ${f.reason})`).join('; ').slice(0, 300)}. Fix these to one-invocation-per-test FIRST.`} Biggest gap: ${verdict && verdict.biggest_gap}. Do ONE repair pass on ${DIR}/beads-manifest.md: rewrite the thin acceptance sections named in ${DIR}/validate-codex.md, fix coverage/cycles. Mark the manifest header 'REPAIRED — needs operator re-validation before tracker write'. Do NOT write to the tracker. Return what you changed.`,
    { label: 'validate:repair', phase: 'Validate' })
  log(`verdict ${score} below threshold ${SCORE_THRESHOLD} — beads NOT written to tracker; manifest repaired for operator review`)
}

return {
  verdict, score_threshold: SCORE_THRESHOLD, dry_run: DRY_RUN, tracker_written: !DRY_RUN && score >= SCORE_THRESHOLD && cycleFree && uncovered.length === 0 && driftOk, drift_guard: driftOk, drift_failures: (drift && drift.failures) || [],
  gate: { passed: passed.length, rejected: rejected.map((b) => b.title), coverage_holes: uncovered, cycle_free: cycleFree },
  red_run: redrun, scenarios: scenarioCount, gaps_dispositioned: dispositions.length, codex_gap_pass_ok: gapsOk,
}
