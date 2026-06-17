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
//      record to the slow-loop sink (escape→shift-left). PENDING is honest until
//      the sink (age-cwo) is wired; do not claim ✓ without an emit.
//   R5 orchestrator-routes-never-reasons ✓  — the script dispatches per-move agents
//      and routes on verdicts; it never does the work it gates.
// ─────────────────────────────────────────────────────────────────────────
```

§6 is an **iff over all five rules**: a script is loop-model-compliant *only* when R1–R5 are each ✓ (HARDENED counts as ✓ — the rule is met, the comment just records that it was tightened and states the no-shell residual). A rule left **PENDING** means the script is **not yet fully conformant** — it is conformant on the rules it marks ✓ and explicitly *not* on the PENDING one, which is an open-loop edge until closed. Do not read PENDING as "compliant with an asterisk." `operating-loop.js` is the honest case: it is conformant on R1–R3/R5 but **not yet fully §6-compliant**, because R4 is PENDING until the slow-loop escape sink (`age-cwo`) is wired. Mark the gap, don't paper over it — and fix any rule you cannot honestly mark ✓ before relying on the workflow.

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

[`.claude/workflows/operating-loop.js`](../../.claude/workflows/operating-loop.js) is the canonical near-conformant reference:

- **(a)** every move (`Shape`, `Plan`, `Pre-flight`, `Implement`, `Capture`) is a dispatched `agent()` returning a schema — the orchestrator never inlines a move.
- **(b)** the slice acceptance gate reads `last.testExitCode === 0` (a captured exit code), **hardened** from a `testNowPasses` self-grade in commit `2f7de41a3`. Its conformance header states the no-shell residual.
- **(c)** `MAX_REPLAN` (pre-flight) and `MAX_SLICE_RETRY` (slices) are plain integers used strictly as backstops; both loops `return` on the grounded verdict first, and each retry carries the prior failure forward.
- **(d)** the script body contains no design or code — only dispatch, verdict-read, and routing.
- **R4 is its one PENDING rule:** the closeout ratchets learnings but emits no escape record (the slow-loop sink is `age-cwo`). Its header marks R4 PENDING honestly — the model for every author: *do not mark a rule ✓ you cannot back with the script.*
