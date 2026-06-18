# Workflow Conformance Pattern

> The copy-paste idiom for authoring a `.claude/workflows/*.js` script that is a **traversable control system**, not an open-loop DAG wearing the loop's vocabulary. This is the *how-to* companion to [control-loop-model.md §6](control-loop-model.md) (the conformance *contract*) — that doc says what compliance **is**; this doc says how to **build it** and how to **self-check** it before you ship.
>
> **Discipline of this doc:** a tight idiom, not a framework. There is deliberately no shared runtime, no base class, no `ao loop step` primitive — a Workflow script has no shell, no filesystem, and no imports (it can only dispatch agents), so the bounding logic ships as a **documented copy-paste idiom**. Reach this doc from [`skills/workflow-builder/SKILL.md`](../../skills/workflow-builder/SKILL.md) once `automation-shape-routing` has confirmed the shape is **Workflow**.

## Why a workflow needs this

A Workflow script is an **orchestrator**: it decides *what runs next and whether to promote*. The failure mode it keeps falling into (the `bdd-foundry.js` DNA) is to re-roll its own control — inline the generative work, grade that work with a free-form LLM self-check, and march forward in fixed order regardless of the verdict. That is an open-loop DAG. It cannot converge, because the thing deciding "is this good enough?" is stochastic and the thing producing the work is the thing grading it.

The four idioms below turn an orchestrator into a control system. The [worked example](#worked-example-operating-loopjs) is `operating-loop.js`.

## The four idioms

### (a) Dispatch skills as black-box agents that return a structured schema

The orchestrator never inlines the generative discipline. Each move is **one `agent()` call** that dispatches a skill (or the skill's discipline) as a black box and returns a **validated JSON schema** — never free prose the orchestrator then has to re-interpret. The schema is what makes the gate mechanical: the orchestrator reads typed fields, not sentences.

```js
const VERDICT = {
  type: 'object', additionalProperties: false,
  required: ['testExitCode', 'blocked'],
  properties: {
    testExitCode: { type: 'integer', description: 'captured exit code of the test command; 0 = green. The §6.1 ground truth — transcribe the real value.' },
    blocked: { type: 'boolean' },
  },
}
const result = await agent(`${DOCTRINE}\n\n<dispatch the skill as a black box>`, { schema: VERDICT, phase: 'Implement' })
```

If you find yourself writing *the work itself* in the prompt (the actual design, the actual code) rather than *dispatching* it, you have violated idiom (d) — see below.

### (b) Gate on a DELEGATED deterministic verdict — never a free-form self-grade

The promotion decision is read from **ground truth the orchestrator did not author**: a captured test exit code (`result.testExitCode === 0`), `ao validate` / `ao gate check`, `ao yield gauge`, or a fixed-rubric quorum. It is **never** a free-form `testNowPasses: true` field the same agent set by grading itself.

```js
// RIGHT — gate on the captured exit code (deterministic ground truth)
if (result.testExitCode === 0 && !result.blocked) return result
// WRONG — gate on a self-graded boolean (§6.1's #1 non-convergence cause)
if (result.testNowPasses) return result          // a stochastic gate flaps pass→fail→pass
```

**The residual you must state honestly:** a Workflow script has no shell, so it cannot *run* the test itself — the agent transcribes the exit code. The in-run verdict is therefore *provisional, best-grounded on captured evidence*; the **authoritative** deterministic gate is `ao gate check` run **post-workflow** (the pre-push windshield). Write that residual into your conformance header (see the worked example, commit `2f7de41a3`, which hardened `operating-loop.js`'s slice gate from a self-grade to a captured exit code).

### (c) The bounded-loop guard — a copy-paste idiom, NOT a module

The fast loop **terminates on a grounded verdict**; the attempt-cap and wall-clock are a **backstop** that catches a thrash the verdict missed, never the primary terminator. Because scripts cannot import a shared helper, paste this shape and adapt the verdict predicate:

```js
const MAX_RETRY = 2                       // BACKSTOP only — never the terminator
const runToVerdict = async (item) => {
  let last = null
  for (let k = 0; k <= MAX_RETRY; k++) {
    last = await attempt(item, k > 0 ? last.whyItFailed : null)  // carry the prior failure forward
    if (last.testExitCode === 0 && !last.blocked) return { ...last, attempts: k + 1 }  // GROUNDED verdict → done
    if (k < MAX_RETRY) log(`${item.id} not green (attempt ${k + 1}/${MAX_RETRY + 1}) — self-repair retry.`)
  }
  return { ...last, attempts: MAX_RETRY + 1, backstopExhausted: true }   // backstop tripped → escalate, don't loop
}
```

Three rules this shape encodes (from §3 of the control-loop model):

- **Terminate on the verdict, not the counter.** The `return` inside the loop fires on `testExitCode === 0`. The post-loop `return` is the backstop.
- **Carry WHY the last attempt failed forward** (`last.whyItFailed`). A retry without the prior failure re-derives the same dead end (degeneration-of-thought).
- **The counter is a plain V8 integer** — that is *why* it is deterministic. Do not route the bound through an agent round-trip.

For unknown-size discovery, swap the fixed cap for a `loop-until-budget` / `loop-until-dry` guard (see `workflow-builder`), but the principle is identical: terminate on a grounded signal, bound as backstop.

### (d) The orchestrator routes and gates — it NEVER reasons about the work

The orchestrator's only jobs are: **read the verdict, apply the bounding primitives, route to the next altitude (or escalate).** The moment the orchestrator starts *doing* the work it is supposed to *gate*, the membrane has a conflict of interest and the control system is broken. Concretely: no design decisions, no code, no "let me just fix this inline" in the script body — those go inside a dispatched `agent()`. The script is the conductor; the agents are the orchestra.

## The §6 five-rule self-check (paste into your workflow header)

Every conformant workflow carries this block at the top, with each rule marked ✓ / HARDENED / PENDING and a one-line justification grounded in the script. This is both documentation and the **self-check** an author runs before shipping (and a reviewer reads to confirm compliance). Mirror `operating-loop.js`:

```js
// ─────────────────────────────────────────────────────────────────────────
// §6 CONFORMANCE (docs/architecture/control-loop-model.md §6 — loop-model-compliant)
//   R1 deterministic-gates ✓/HARDENED  — promotion reads ground truth (a captured
//      test exit code / ao validate / fixed-rubric quorum), never a free-form self-grade.
//      State the residual: a workflow has no shell, so the authoritative gate is
//      `ao gate check` post-workflow; the in-run verdict is provisional.
//   R2 terminate-on-verdict ✓  — loop returns on the grounded verdict; the
//      attempt-cap / wall-clock is a BACKSTOP, not the terminator.
//   R3 no-self-modification-in-run ✓  — no gate is added/removed/retuned mid-run.
//   R4 escapes→slow-loop ✓/PENDING  — a downstream-caught defect emits an escape
//      record (a REFUTED gate-verdict at a higher attempt, paired with the upstream
//      CONFIRMED) to the slow-loop sink. See "The R4 escape-emit step" below; the
//      sink exists (age-zqc), so ✓ requires the emit — PENDING only until it is wired.
//   R5 orchestrator-routes-never-reasons ✓  — the script dispatches per-move agents
//      and routes on verdicts; it never does the work it gates.
// ─────────────────────────────────────────────────────────────────────────
```

§6 is an **iff over all five rules**: a script is loop-model-compliant *only* when R1–R5 are each ✓ (HARDENED counts as ✓ — the rule is met, the comment just records that it was tightened and states the no-shell residual). A rule left **PENDING** means the script is **not yet fully conformant** — it is conformant on the rules it marks ✓ and explicitly *not* on the PENDING one, which is an open-loop edge until closed. Do not read PENDING as "compliant with an asterisk." Mark the gap, don't paper over it — and fix any rule you cannot honestly mark ✓ before relying on the workflow.

## The R4 escape-emit step — route a downstream catch to the slow loop

R4 is the rule a workflow most often leaves PENDING, because it needs a sink to emit into. That sink now exists (`ao membrane` / `yieldledger.DetectEscapes`, landed `age-zqc`), so a conformant workflow gains one more step: **when a downstream gate catches a defect an upstream gate had already passed, emit an escape record.** An escape is the membrane's own miss — a unit a gate CONFIRMED that a *later, stricter* gate REFUTED. The slow loop turns each escape into the check that would have caught it (`escape → finding → membrane check`); the workflow's only job is to **emit** it, never to derive or govern (that is `age-cwo`).

The mechanism is the yield ledger. `DetectEscapes` is run-scoped and keys on the pattern **CONFIRMED-then-REFUTED-at-a-higher-attempt for the same bead**. So the workflow emits two gate-verdicts across its gates. The `ao yield emit gate-verdict` body is **commit-bound** — `ao` validates a `pawl_verdict_ref` (bead_id + a `head_sha` ≥7 chars), a top-level `head_sha` ≥7 chars, a valid `mode`, and a non-empty `author_family`; a minimal body is rejected (exit 1). So the emit step must build the full body, capturing the run's current HEAD as the sha (the base commit the work sits on):

```js
// The emit agent captures SHA = `git rev-parse HEAD`, then runs a body of this shape
// (deterministic tier: cross_family=false so it never inflates catch_rate_cross_family):
//   ao yield emit gate-verdict --bead <id> --run <runId> --json '{
//     "difficulty":2,"pawl_verdict_ref":{"bead_id":"<id>","head_sha":"<SHA>"},
//     "disposition":"CONFIRMED","head_sha":"<SHA>","attempt":1,"mode":"deterministic",
//     "author_context_id":"<upstream-gate>","refuter_families":[],
//     "author_family":"<workflow>-gate","cross_family":false,
//     "author_ne_reviewer":true,"evidence_present":true }'
const emitVerdict = (disposition, attempt, ctx, note, label) => agent(
  `R4 gate-verdict (EMIT only). ${note}\nDo exactly this: (1) SHA=$(git rev-parse HEAD); ` +
  `(2) ao yield emit gate-verdict --bead "<id>" --run "<runId>" --json '{...full body above with "$SHA"...}'. ` +
  `Return the exit code. Do NOT run ao membrane derive-checks or tune the sink — that is age-cwo.`,
  { label, phase: 'Capture' })

await emitVerdict('CONFIRMED', 1, 'slice-gate', 'The upstream gate passed green.', 'r4:confirm')
if (!closeout.accepted)                                  // downstream caught a defect upstream passed
  await emitVerdict('REFUTED', 2, 'acceptance-rollup', `Caught: ${JSON.stringify(failed)}`, 'r4:escape-emit')
```

The pairing (`attempt:1` CONFIRMED → `attempt:2` REFUTED, same bead+run) is what `DetectEscapes` surfaces; a clean run emits only the single CONFIRMED. The slow loop then runs `ao membrane derive-checks --run <id>` out-of-band. **Boundary:** the workflow emits; it never derives the check or tunes a gate mid-run (R3) — keep R4 strictly the emit, or it front-runs `age-cwo` and breaks the two-timescale separation.

**Orphan-escape guard (the REFUTED@2 emit is fail-open).** The two emits are independent agent calls, so a failed upstream `CONFIRMED@1` append leaves a `REFUTED@2` with no pair — and `DetectEscapes` keys on the *pair*, so it silently drops a lone REFUTED, losing the escape. Make the downstream REFUTED@2 self-heal: before appending it, re-assert the attempt-1 CONFIRMED is in the yield ledger and re-emit that exact body first if absent (`ao yield emit` is append-safe, so re-emitting is idempotent-safe). Thread it as an optional `pairGuard` on the escape call — `emitVerdict('REFUTED', 2, …, 'r4:escape-emit', ORPHAN_GUARD)` — so a clean CONFIRMED path is unaffected. Carried by both `bdd-foundry.js` and `operating-loop.js`; landed `age-g0y` / `age-1vy`.

## Governance — the workflow must be born registered

A new workflow `.js` is not done until `scripts/check-workflow-governance.sh` passes. That gate asserts a **bidirectional identity match** between every `.claude/workflows/*.js` and the top-level `workflows:` section of [`docs/contracts/skill-dispositions.yaml`](../contracts/skill-dispositions.yaml), and that each ledger row carries the DDD identity triple. Add a row keyed by your workflow's `meta.name`:

```yaml
workflows:
  my-workflow:
    kind:            workflow          # REQUIRED — exact string
    domain:          "BC3 Loop"        # REQUIRED — a Bounded Context (see bounded-contexts.yaml)
    hexagonal_role:  driving-adapter   # REQUIRED — a hexagonal role
    runtime_targets: [claude]          # workflows are Claude-only (no skills-codex twin)
    parity_policy:   exempt
    capability_class: orchestration    # or planning / etc.
    aliases:         []
    path:            .claude/workflows/my-workflow.js
    supersedes:      null
    rationale:       "one-line justification"
```

Then run `make regen-all` (registry projection) and `node --check .claude/workflows/my-workflow.js` (the script parses). `make regen-check` is the drift backstop.

## Worked example: operating-loop.js

[`.claude/workflows/operating-loop.js`](../../.claude/workflows/operating-loop.js) is the canonical fully-conformant reference:

- **(a)** every move (`Shape`, `Plan`, `Pre-flight`, `Implement`, `Capture`) is a dispatched `agent()` returning a schema — the orchestrator never inlines a move.
- **(b)** the slice acceptance gate reads `last.testExitCode === 0` (a captured exit code), **hardened** from a `testNowPasses` self-grade in commit `2f7de41a3`. Its conformance header states the no-shell residual.
- **(c)** `MAX_REPLAN` (pre-flight) and `MAX_SLICE_RETRY` (slices) are plain integers used strictly as backstops; both loops `return` on the grounded verdict first, and each retry carries the prior failure forward.
- **(d)** the script body contains no design or code — only dispatch, verdict-read, and routing.
- **R4** — the Capture phase emits the escape pair: a CONFIRMED at the upstream slice gate (all slices green) and a REFUTED at attempt 2 when the downstream acceptance roll-up catches a defect the slice gate passed. That pair is what the slow loop (`ao membrane derive-checks`, `age-zqc`) consumes. With R4 wired, `operating-loop.js` satisfies all five rules.
