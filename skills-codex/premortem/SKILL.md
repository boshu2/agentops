---
name: premortem
description: 'Stress-test plans before work. Use when: a'
---
# Premortem Skill

> **Purpose:** Is this plan/spec good enough to implement?
> **Admission rule:** one fresh-context Premortem binds the plan before Crank.
> Quick mode narrows claims; it never lets the plan author grade their own work.

## Constraints

- **Judge the plan, never the implementation.** This keeps the plan-pawl separate from the acceptance-test and finished-diff pawls, because one verdict cannot prove all three artifacts.
- **Use an independent judge.** The author must not grade their own plan, because shared assumptions make self-review autocorrelated; one-way doors additionally require a different model family.
- **Pre-register kill conditions for irreversible work.** A strategy, experiment, or one-way-door plan must say what evidence changes the decision before deliberation, because an unfalsifiable review is ceremony.
- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED is repair evidence: revise the plan and rerun automatically. Raise the andon and route one helper only for a true breaker such as missing authority, unavailable required trust domain after retry, or an impossible invariant.
- **Bound the repair loop.** Apply one consolidated repair to the exact plan; a second distinct acceptance repair returns `REPLAN` for re-slicing, while the RPI governor owns disposition and breaker/helper state.
- Bind the verdict to a plan digest plus acceptance, dependency shape, write
  scope, and risk class. Reuse it across intermediate waves while those inputs
  remain unchanged. Mechanical packet inconsistencies are local `REPAIR`; only
  material plan/risk change requires another independent Premortem.
  That repeat requires an explicit orchestrator request carrying the changed plan;
  no downstream phase may silently manufacture the request.

## Loop position

Pre-flight check between moves **3 (slice plan)** and **4 (TDD per slice)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces a PASS/WARN/FAIL verdict on the plan AND on the wave-validity rows (distinct write scopes, no shared migration/contract/CLI surface, owner per slice, discard path per slice). A wave can only be claimed parallel if premortem confirms every conflict-free row. FAIL on wave-validity → run slices sequential or send the plan back to `$plan` for re-slicing. Between waves, the orchestrator reuses the bound verdict when inputs are identical. Only an explicit material change returns the exact changed plan for one new review. Validate and Learn cannot invoke Premortem directly.

Run `$council validate` only when a named one-way door or explicitly contested
decision earns multi-model judgment. Routine reversible work uses one concise
fresh-context judge.

## Quick Start

```bash
$premortem                                         # one fresh judge on the most recent plan
$premortem path/to/PLAN.md                         # one fresh judge on this plan
$premortem --deep path/to/SPEC.md                  # 4 judges (thorough review, spawns agents)
$premortem --mixed path/to/PLAN.md                 # cross-vendor (Claude + Codex)
$premortem --preset=architecture path/to/PLAN.md   # architecture-focused review
$premortem --explorers=3 path/to/SPEC.md           # deep investigation of plan
$premortem --debate path/to/PLAN.md                # two-round adversarial review
```

## Execution Steps

### Steps 0-1: Resolve one current plan

Use the supplied path or latest plan/spec. For a full, older-than-seven-day, or
prior-session bead, run `ao beads verify <id>` first and stop on stale citations.

### Step 1.4: Retrieve matched compiled prevention once

Routine quick mode reads only directly matched compiled checks from
`.agents/premortem-checks/*.md` (fall back to
`.agents/findings/registry.jsonl`) and carries them as `known_risks`. It does not
run a broad flywheel search, metrics write, or registry mutation. Deep modes may
run `ao lookup` for the plan's domain. Full fail-open and ranking rules live in
[references/compiled-prevention.md](references/compiled-prevention.md).

Fail-open reader behavior is mandatory: missing or empty compiled prevention inputs skip silently; malformed line -> warn and ignore that line; unreadable file -> warn once and continue without findings.

### Step 1.5: Fast Path (--quick mode)

**By default, premortem runs `--quick` with exactly one fresh-context judge
distinct from the plan author.** It emits one complete blocker set and no
council fan-out. The orchestrator may repair packet wording locally and request
one narrow recheck; a second distinct acceptance repair is `REPLAN`.

In `--quick` mode, skip Steps 1a and 1b as standalone pre-processing phases.
Load only product constraints directly implicated by acceptance. `--deep`,
`--mixed`, `--debate`, and `--explorers` require a named irreversible or
high-blast-radius reason before they add council fan-out.

To escalate to full multi-judge council, use `--deep` (4 judges) or `--mixed` (cross-vendor).

### Steps 1.5.1-1.7: Size and focus the gate

State reversibility and blast radius in one sentence, select
`scope_mode: expansion|hold|reduction`, and load only applicable patterns from
[scope-mode.md](references/scope-mode.md) and
[council-fail-patterns.md](references/council-fail-patterns.md). One fresh judge
is the reversible default; deep/mixed council requires a named one-way door.

### Step 2: Run one independent judgment

For reversible work, dispatch one runtime-native fresh judge directly against
the bound plan packet; do not start `$council`. Use `$council --deep
--preset=plan-review`, `$council --mixed --preset=plan-review`, explorers, or
debate only for a named one-way door, contested judgment, or explicit operator
request. Mode composition and judge roles are in
[references/mandatory-checks.md](references/mandatory-checks.md#steps-2911-independent-adjudication-and-plan-pawl).

**Checkpoint:** before deliberation, confirm the packet records `scope_mode`, blast radius/reversibility, `author_id`, a distinct `judge_id`, and any required pre-registered `decision_rule`. Do not emit PASS while an invariant is missing.

### Steps 2.4–2.8: Check the plan once

Apply the triggered temporal, rescue, test-shape, input, migration-manifest,
scope-predicate, one-behavior, RED-proof, refactor-separation, and existing-
capability checks from [mandatory-checks.md](references/mandatory-checks.md).
Return one complete blocker set; do not write one report per checklist row.

### Steps 2.9–2.11: Independent adjudication and plan-pawl

Apply the no-self-grading rule, cross-family rule for one-way doors, pre-registered decision rule, and discovery plan-pawl equivalence exactly as specified in [references/mandatory-checks.md](references/mandatory-checks.md#steps-2911-independent-adjudication-and-plan-pawl). A completed discovery plan-pawl duel is the premortem verdict for fanout-class discovery; do not run a duplicate council.

### Step 3: Interpret Council Verdict

| Council Verdict | Premortem Result | Action |
|-----------------|-------------------|--------|
| PASS | Ready to implement | Proceed |
| WARN | Review concerns | Address warnings or accept risk |
| FAIL | Not ready | Fix issues before implementing |

### Step 4: Write one Premortem verdict

The canonical output is one schema-valid verdict record bound to the plan
digest and review inputs. While legacy consumers still require it, write a
concise `.agents/council/YYYY-MM-DD-premortem-<topic>.md` projection containing
identity, verdict, complete blockers, and next action. Use the full report
template in [references/write-premortem-output.md](references/write-premortem-output.md)
only for deep/contested work; routine quick mode does not create a second
analysis, pseudocode dossier, or plan-copy.

Only a genuinely reusable finding takes the off-path Step 4.5 route. When it
does, include `dedup_key`; do not invoke a repository hook or activate a
constraint. `ao membrane digest` refreshes the canonical recurring-catch
advisory sink. Routine PASS and packet repair create no registry entry.

The generated report must preserve this exact heading because downstream validators and ledger readers extract verdicts with a regex anchored to it:

## Council Verdict: PASS / WARN / FAIL

## Output Specification

- **Artifact path:** `.agents/council/`.
- **Filename convention:** `YYYY-MM-DD-premortem-<topic>.md`.
- **Serialization/schema format:** canonical verdict data conforms to
  `skills/council/schemas/verdict.json`; routine Markdown is a concise
  compatibility projection, while deep mode may use
  [references/write-premortem-output.md](references/write-premortem-output.md).
- **Validator command:** `bash skills/premortem/scripts/validate.sh && grep -Eq '^## Council Verdict: (PASS|WARN|FAIL)$' .agents/council/YYYY-MM-DD-premortem-<topic>.md`.
- **Downstream handoff:** PASS proceeds to `$implement`; WARN or FAIL returns the plan to its author for repair and automatic re-review. Only a breaker raises the andon or routes one helper.

### Step 5: Record ratchet progress off the critical path

```bash
ao ratchet record premortem 2>/dev/null || true # only when a reusable finding exists
```

### Step 6: Report to User

Tell the user:
1. Council verdict (PASS/WARN/FAIL)
2. Key concerns (if any)
3. Recommendation
4. Location of premortem report

## Integration with Workflow

```
$plan epic-123
    │
    ▼
$premortem                    ← You are here
    │
    ├── PASS → $implement
    ├── WARN → Review, then $implement or fix
    └── FAIL → Fix plan, re-run $premortem
```

## Quality Checklist

- Every verdict cites concrete plan text and names the failure mode or proof that resolved it.
- Every wave-validity row has non-overlapping write scope, one owner, and a discard path before parallel execution.
- Every irreversible decision has an independent cross-family judge and a decision rule recorded before deliberation.
- WARN, FAIL, and REFUTED routes repair and rerun; only a breaker routes the andon/helper path.

## Examples

See [references/examples.md](references/examples.md) for worked examples (one
fresh judge by default, explicit `--mixed`, auto-find recent, and `--deep`
high-stakes) plus focused troubleshooting.

## Troubleshooting

Use the structured troubleshooting table in [references/examples.md](references/examples.md); repair ordinary verdicts in place and reserve escalation for a breaker.

## See Also

- `skills/council/SKILL.md` — Multi-model validation council
- [`pawl-review`](../pawl-review/SKILL.md) — fresh reviewer execution for the finished diff; this skill attacks the plan before work
- `skills/plan/SKILL.md` — Create implementation plans
- `skills/validate/SKILL.md` — Validate code after implementation

## Reference Documents

- [references/premortem.feature](references/premortem.feature) — Executable spec: plan PASS/WARN/FAIL verdict before work, wave-validity gates parallelism, one fresh --quick judge (soc-qk4b)
- [references/compiled-prevention.md](references/compiled-prevention.md)
- [references/scope-mode.md](references/scope-mode.md)
- [references/mandatory-checks.md](references/mandatory-checks.md)
- [references/scope-predicate-positive-negative-cases.md](references/scope-predicate-positive-negative-cases.md)
- [references/write-premortem-output.md](references/write-premortem-output.md)
- [references/examples.md](references/examples.md)
- [references/council-fail-patterns.md](references/council-fail-patterns.md)
- [references/enhancement-patterns.md](references/enhancement-patterns.md)
- [references/error-rescue-map-template.md](references/error-rescue-map-template.md)
- [references/failure-taxonomy.md](references/failure-taxonomy.md)
- [references/simulation-prompts.md](references/simulation-prompts.md)
- [references/prediction-tracking.md](references/prediction-tracking.md)
- [references/spec-verification-checklist.md](references/spec-verification-checklist.md)
- [references/temporal-interrogation.md](references/temporal-interrogation.md)
