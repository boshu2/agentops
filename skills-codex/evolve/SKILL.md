---
name: evolve
description: 'Run autonomous improvement loops. Triggers: "evolve", "improve everything", "autonomous improvement".'
---
# $evolve — Goal-Driven Autonomous Loop

> Measure what's wrong. Fix the worst thing. Measure again. **Whether the fixes *compound* into a durable knowledge moat is a tracked hypothesis, not a promise** — DEMOTED to unproven by [ADR-0004](../../docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md) and [ADR-0011](../../docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md). The proven product is the per-cycle verification (**no verdict = not done**), not the compounding. Do not market the flywheel ahead of the ruler.

> **Experimental tier.** Autonomous long-loop; run attended or dispatched onto a substrate, never as an in-repo daemon (ADR-0009).

**Codex orchestration default:** keep the skill name `$evolve`. In Codex, run the loop by chaining Codex skills — `$evolve` selects work and invokes complete `$rpi --auto` cycles. Each cycle's post-mortem checkpoint is a **re-plan point** (re-scope / reorder / drop / add to the remaining queue), one altitude up from `$rpi`'s [agile re-plan loop](../rpi/references/agile-replan-loop.md) — agile across cycles, not a fixed backlog. Substrates dispatch the whole loop as one unit through NTM, Agent Mail, or `ao agent`; former RPI CLI wrappers are retired (ADR-0009).

**Cadence is pawl-gated, not per-tread** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)). Each cycle's heavy validation (`$validate`, `$pawl-review`, then `ao pawl`) fires ONCE at the cycle's **bead-acceptance / land pawl** — not per slice or wave. The per-cycle regression gate (Step 5) is **chaos**: cheap, wrong-tolerant between pawls. Do NOT escalate every cycle to a cross-family panel "to be safe".

**Operator cadence:** post-mortem finished work → measure repo state → select the next highest-value item → let `$rpi` run research → plan → pre-mortem → implement → validate → harvest follow-ups → repeat until a kill switch, max-cycle cap, regression breaker, or real dormancy stops it.

## Constraints

- Run one complete `$rpi --auto` cycle per selected item and re-read the ladder afterward, because partial phases and fixed backlogs break feedback.
- Never push without a commit-current CONFIRMED pawl verdict, because a green producer gate is necessary but not sufficient proof.
- Treat breaker trips with one bounded helper pass before escalation; only judgment, refusal, spent budgets, or a failed helper reach a human, because ordinary blockers belong to pawl recovery.

## Work selection ladder

A ladder re-read from the TOP after every productive cycle — never a one-shot check. Full per-rung procedure: [references/work-selection-ladder.md](references/work-selection-ladder.md).

1. **Harvested** — `.agents/rpi/next-work.jsonl`, freshest unconsumed follow-up
2. **Open ready beads** — `ao beads exec ready`, highest priority
3. **Failing goals + directive gaps** — `ao goals measure` (skip if `--beads-only`; skip quarantined oscillators)
4. **Generators** — coverage / security / perf / refactor findings → beads or queue items (below)
5. **Complexity / TODO / drift / dead-code / stale-doc / stale-research mining**
6. **Feature suggestions** grounded in repo purpose when nothing sharper exists

`--quality` inverts the top (findings before goals). The metronome gate blocks a rung that repeats the trailing run's `mode` (streak ≥3). **Dormancy is last resort** — empty queues mean "run the generators", not "stop"; go dormant only after queue AND generator layers come up empty across multiple consecutive passes.

**Work generators** (auto-invoked; skip with `--no-lifecycle`):
- `$test --coverage` → files with <40% coverage become queue items
- `$refactor --sweep` → functions with CC > 20 become queue items
- `$security audit` → deps with CVSS ≥ 7.0 or 2+ majors behind
- `$perf profile` → hot-path perf findings

**Live skill-edit immune system:** if a cycle edits `skills/<slug>/SKILL.md`, run `ao skills edit seal --skill <slug> --actor "${AGENT_NAME:-agent}"` before handoff — the seal creates the rollback commit and records the `Skill-Edit` trailers. Critical skills in `docs/contracts/critical-skills.txt` reject unattended edits; `--allow-critical` only under supervision.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Stop after `N` completed cycles |
| `--dry-run` | off | Show planned cycle actions without executing |
| `--beads-only` | off | Skip goal measurement, run backlog-only selection |
| `--skip-baseline` | off | Skip first-run baseline snapshot |
| `--quality` | off | Prioritize harvested post-mortem findings |
| `--compile` | off | Run `ao compile` knowledge warmup before cycle 1 |
| `--test-first` | on | Pass strict-quality defaults through to `$rpi` |
| `--no-test-first` | off | Explicitly disable test-first passthrough to `$rpi` |

## Execution Steps

**YOU MUST EXECUTE THIS WORKFLOW — do not just describe it.** **FULLY AUTONOMOUS:** every `$rpi` uses `--auto`; do NOT ask the user anything, do NOT pause between cycles (the operator-shape carve-out in [references/autonomous-execution.md](references/autonomous-execution.md) is the only exception). Each cycle = one complete 3-phase `$rpi` run. For broad AgentOps-domain evolution first read [references/domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md) — the BDD/DDD/Hexagonal/TDD/XP control surface.

### Anti-Patterns (DO NOT)

| Anti-Pattern | Correct Behavior |
|--------------|------------------|
| Ask the user anything during execution | Make best judgment, report in teardown |
| Stop after one `$rpi` cycle and summarize | Increment cycle, re-enter Step 1 |
| Run `$rpi` without `--auto` | Always pass `--auto` (non-auto has human gates) |
| Run partial `$rpi` (skip validation) | Let `$rpi` run all 3 phases |
| Treat "no queued work" as "stop" | Run all generator layers before dormancy |

### Step 0: Setup

**Stale-checkout survey guard (run FIRST):** `git fetch origin && git status -sb`. If behind/diverged AND a throwaway tree with no un-pushed work, `git reset --hard origin/main`. **Never `git pull --rebase` on the survey path** — it no-ops against a diverged local `main`, so merged files appear "missing".

```bash
git fetch origin && git status -sb
mkdir -p .agents/evolve
ao corpus inject --query "autonomous improvement cycle" --limit 5 2>/dev/null || true
```

Recover cycle state from disk (survives compaction): `CYCLE`, `IDLE_STREAK`, `GENERATOR_EMPTY_STREAK`, `LAST_SELECTED_SOURCE`, `CLAIMED_WORK_REF` from `.agents/evolve/session-state.json`; canonical ledger is `cycle-history.jsonl` (both **local-only** — the nested `.agents/.gitignore` denies all paths). **Prior-failure injection (mandatory):** read the last 3 `cycle-history.jsonl` entries; for any `gate` containing `FAIL|BLOCKED`, extract failure keywords and grep `.agents/learnings/` before selecting work. Detail: `references/cycle-history.md`, `references/convergence-mechanics.md`.

**Repo-local contracts.** If `docs/contracts/repo-execution-profile.md` exists, read its ordered `startup_reads` and bootstrap before selecting work; cache `validation_commands`, `tracker_commands`, `definition_of_done`. If a repo-local `PROGRAM.md` (or `AUTODEV.md` alias — `PROGRAM.md` wins) contract exists, `$rpi` loads it automatically — cache its `mutable_scope`, `validation_commands`, `decision_policy`, `stop_conditions`; prefer work inside mutable scope, never silently widen it. The PROGRAM.md contract is the legacy autodev lane (built only under `-tags legacy`); spec + repair guidance: [docs/contracts/autodev-program.md](../../docs/contracts/autodev-program.md) and executable specs [references/autodev.feature](references/autodev.feature) / [references/autodev-cli.feature](references/autodev-cli.feature).

**Circuit breakers (tunable — also the pawl-escalation governor):** time-based (60 min no productive work) · max-cycles/max-attempts cap · cost/quota budget · oscillation. Same breakers govern pawl escalation — a REFUTED pawl auto-redoes; a tripped breaker takes one bounded helper pass (fresh context, cross-family model, or council — [pawls.md §Escalation](../../docs/contracts/pawls.md#escalation-the-circuit-breaker-model)); a human is pulled in only past the helper or on the skip classes (refusal lane, explicit judgment flag, spent ceiling). Thresholds: `EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts. **Oscillation quarantine:** pre-populate from cycle history (goals with 3+ improved→fail transitions). See `references/oscillation.md`.

### Step 0.2 / 0.5: Warmup + baseline

**Checkpoint:** `--compile` only (skip on `--dry-run`): run `ao compile` before cycle 1. On the first eligible run, capture the fitness baseline via `scripts/evolve-capture-baseline.sh`.

### Step 1: Kill-switch check (TOP of every cycle)

```bash
CYCLE_START_SHA=$(git rev-parse HEAD)
# Mechanical pre-cycle gate: KILL/STOP/DORMANT/HANDOFF markers (TTL + non-sticky),
# goal-regression, prior-cycle-FAIL. A SCRIPT the loop MUST run, not skippable prose.
if [ -x scripts/evolve/halt-check.sh ]; then
  if ! HALT_OUT=$(bash scripts/evolve/halt-check.sh --json); then
    REASON=$(printf '%s' "$HALT_OUT" | jq -r '.halt_reason // "unknown"')
    if [ "$REASON" = "prior_cycle_fail" ]; then
      export EVOLVE_RESTORATIVE=1   # not terminal: Step 1.5 restricts scope to CI-red reduction
    else
      echo "halt: $REASON"; exit 0
    fi
  fi
fi
```

**Agile-first dormancy:** `DORMANT` is NEVER sticky while ready beads exist — `halt-check.sh` auto-clears it when `ao beads exec ready` / harvested work exists. KILL/STOP honor `EVOLVE_KILL_TTL_DAYS` (default 7); stale markers are surfaced and bypassed. `goal_regression` halts for operator attention.

### Step 1.5: Healing-first classifier

`ao ci recent --limit 1` → if the last push CI was `failure`, this cycle is **restorative-only**: Step 3 takes only CI-red-reducing work (harvested bugs, gate-fix beads, generator bug output) — no promotions/features until green. A `gate=FAIL` in cycle-history auto-triggers this for cycle N+1. **Convergence check:** `ao loop converged --green-streak <n> --unconsumed-high-medium <n> [--fitness-baseline]`; branch on `.converged` (default: CI green streak ≥ 3, HIGH+MEDIUM ≤ 1, baseline captured) — if true, emit teardown and do NOT re-arm. See `references/convergence-mechanics.md`.

### Step 2: Measure fitness

Skip if `--beads-only`. Run `scripts/evolve-measure-fitness.sh --output .agents/evolve/fitness-latest.json --timeout 60 --total-timeout 75`. The AgentOps CLI is required (the wrapper shells out to `ao goals measure`). Full procedure: `references/goals-schema.md`.

### Step 3: Select work

Run the ladder above; read [references/work-selection-ladder.md](references/work-selection-ladder.md) for the per-rung code, `--quality` cascade, and dormancy hard-gate. **Agile invariant:** `ao beads exec ready ≥ 1` ⇒ the loop NEVER writes DORMANT and NEVER exits — the only path to DORMANT is a fully empty backlog + dry generators (3 passes); context exhaustion → HANDOFF, not DORMANT. Pick harvested items with a claim (`claim_status: in_progress`, `claimed_by: evolve:cycle-N`, `consumed: false` until success). If `--dry-run`: report and go to Teardown.

### Step 4: Execute

Primary engine: `$rpi` (all 3 phases mandatory). `$implement` / `$crank` only when a bead has execution-ready scope.

```
Invoke $rpi "{normalized work title}" --auto --max-cycles=1     # harvested / goal / gap / testing / bug / drift / feature
Invoke $rpi "Land {issue_id}: {title}" --auto --max-cycles=1    # a bead (fallback: $implement {issue_id})
Invoke $crank {epic_id}                                         # epic with children
```

If Step 3 created durable work instead of executing it, re-enter Step 3 and let the new bead win. **Mechanical-batch hint:** > 20 uniform per-file edits → a script, not N tool calls. **Pre-flight schema check:** a port/adapter migration reading > 20% more fields than the target port projects → abort, convert to a port-widening cycle. **Operator-shape carve-out:** `AskUserQuestion` permitted ONLY for shape decisions > 50 files OR a schema/contract surface.

### Step 4.5: Source-surface sync (pre-gate)

Sync downstream artifacts when the staged diff touches binary/embedded surfaces, or the gate fails on stale-binary / drift errors that look like regressions (`references/gate-hygiene.md` in source tree):
- `cli/**/*.go` changed → `cd cli && make build && go install ./cmd/ao`
- `skills-codex/**` changed → `bash scripts/regen-codex-hashes.sh`

A skill touches **six derived surfaces** (registry.json, skill-domain-map, context-map, counts + `SKILL-TIERS.md`, codex twin, narrative counts) — regenerate via `scripts/regen-all.sh` + codex/count steps, never piecemeal.

### Step 5: Regression gate

**Checkpoint:** run project tests plus ordered repo-profile / PROGRAM.md `validation_commands` and wiring closure. Apply `decision_policy`, immutable scope, and `stop_conditions`; re-measure to `fitness-latest-post.json` and revert regressions. Keep claimed work `consumed: false` until `$rpi` succeeds, then re-read `.agents/rpi/next-work.jsonl`.

### Step 6: Log cycle + commit

**PRODUCTIVE** (improved / regressed / harvested): log via `scripts/evolve-log-cycle.sh`, commit real changes. **IDLE:** log `--result "unchanged"`; no git add, no commit. Record the XP/BDD/TDD trace via `--trace-json` when a cycle worked a product or goal-backed gap (goal hypothesis → gap → Gherkin → failing proof → red/green → refactor → validation → ratchet → goal reshape); trivial one-shot cycles record a `trace.exemption_reason`. Trace completeness is advisory, never a gate. See `references/cycle-history.md`, `references/quality-mode.md`.

### Step 7: Land — worktree → gate → pawl → push

Push is the **mutate-shared-trunk pawl**: work in a per-cycle worktree, run `ao gate check --fast --scope head`, obtain a cross-family commit-current verdict with `scripts/pawl-review.sh`, then use `scripts/pawl-land.sh`. Same-family review and pushes without CONFIRMED are refused. REFUTED auto-redoes; breaker trips get one bounded helper pass before human-only skip classes. Obey LAW 0: never invoke a forbidden print-mode runtime.

### Step 7 loop / stop

After landing, increment `CYCLE` and return to Step 1.

**Stop ONLY on:** (1) **KILL/STOP marker**; (2) **`--max-cycles` cap**; (3) **genuine stagnation** after empty backlog, goals, harvest, and generators; (4) **regression breaker after a revert**. **Context exhaustion is NOT a stop** — write non-sticky `.agents/evolve/HANDOFF`, log `context-handoff`, and resume on the next fire.

**Mandatory checkpoint — session-PR threshold (gates next cycle, NOT terminal):** at `session_pr_count >= 5`, invoke `$post-mortem --deep` and wait for the verdict file. PASS → continue; WARN → continue with a caveat; FAIL / non-convergence → write STOP. The agent MUST NOT self-grade or self-write STOP (`references/postmortem-checkpoint.md`).

### Teardown

Commit any staged `cycle-history.jsonl`, run `$post-mortem "evolve session: N cycles"` (a light session-end retrospective — it does NOT substitute for the council-gated threshold checkpoint), push only if unpushed commits exist, and report the summary (cycles, productive/regressed/idle counts, stop reason). Never write `.agents/evolve/STOP` as a substitute for the checkpoint's verdict file.

Release-shaped branches follow [the release teardown contract](references/teardown.md#release-shaped-teardown): never recommend `$release` from per-cycle `--fast`, carry the unchecked checklist into handoff, and require the full release gate before tagging.

## Output Specification

- **Path:** emit summary to stdout; append `.agents/evolve/cycle-history.jsonl`; write `.agents/evolve/{fitness-latest.json,session-state.json}` and control files.
- **Filename:** history is `cycle-history.jsonl`; current fitness and resumable state use the fixed filenames above.
- **Format:** stdout is Markdown; state and fitness use JSON; cycle history uses JSONL per `references/cycle-history.md`.
- **Validation command:** run repo/profile tests, wiring closure, and `ao gate check --fast --scope head`; require a commit-current pawl verdict before land.
- **Downstream handoff:** return cycle counts, fitness delta, result, stop reason, changed paths, and verdict; the next cycle consumes persisted state and unconsumed work.

## Quality Checklist

- Selected work follows the ladder and declared mutable scope.
- A productive cycle has deterministic validation plus a commit-current independent verdict.
- Regressions revert, work stays unconsumed until land, and breaker handling follows helper-before-human policy.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Loop exits immediately | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` |
| Stagnation after repeated empty passes | Queue + producer layers empty across multiple passes — dormancy is the fallback |
| `ao goals measure` hangs | Use `--timeout 30 --total-timeout 75`, or `--beads-only` to skip |
| Regression gate reverts | Review reverts, narrow scope, re-run; release claimed work back to available |

## References

- **Loop mechanics** — [cycle-history](references/cycle-history.md) (JSONL, recovery, trace), [convergence-mechanics](references/convergence-mechanics.md) (healing-first classifier), [oscillation](references/oscillation.md), [quality-mode](references/quality-mode.md), [goals-schema](references/goals-schema.md), [work-selection-ladder](references/work-selection-ladder.md), [fitness-scoring](references/fitness-scoring.md)
- **Autonomy + knowledge** — [autonomous-execution](references/autonomous-execution.md) (loop rules + carve-out), [compounding](references/compounding.md) (hypothesis-posture per ADR-0004/0011), [domain-evolution-bootstrap](references/domain-evolution-bootstrap.md), [postmortem-checkpoint](references/postmortem-checkpoint.md), [parallel-execution](references/parallel-execution.md) (`$swarm`), [teardown](references/teardown.md), [examples](references/examples.md), [artifacts](references/artifacts.md)
- **Gating + specs** — [gate-hygiene](references/gate-hygiene.md), [new-skill-landing](references/new-skill-landing.md), [knowledge-loop-integration](references/knowledge-loop-integration.md), [evolve.feature](references/evolve.feature), [autodev.feature](references/autodev.feature) + [autodev-cli.feature](references/autodev-cli.feature) (legacy autodev lane, `-tags legacy`)

## See Also

- `skills/rpi/SKILL.md` — full lifecycle orchestrator (called per cycle)
- `skills/crank/SKILL.md` — epic execution for beads epics
- `skills/post-mortem/SKILL.md` — learning extraction + mining surface; absorbed the retired curate/compile/flywheel skills (mechanical surfaces are the `ao compile` and `ao flywheel status` CLI, not skills)
- `docs/contracts/autodev-program.md` — repo-local PROGRAM.md contract (legacy autodev lane)
- `GOALS.yaml` — fitness goals
- [test](../test/SKILL.md) · [refactor](../refactor/SKILL.md) · [security](../security/SKILL.md) · [validate](../validate/SKILL.md) — the work generators

<!-- Lifecycle integration wired: 2026-03-28. Trimmed 2026-07-07 (age-skills-audit-fable-l6ic.4) — mirrors skills/evolve/SKILL.md hard-trim. -->

## Behavioral contract anchors (validated by scripts/validate.sh)

The trim moved procedure to references/, but these invariants stay inline — the skill's
own validator greps them, and they are the loop's load-bearing behavior:

- **Continuous values, not booleans:** every fitness metric reports a continuous value against a threshold (value/threshold), never a bare pass/fail.
- **Oscillation sweep (always-on, Step 0):** Pre-populate quarantine list from `ao compile`'s oscillation report before selecting a goal.
- **Wiring pre-flight (Step 5):** `if bash scripts/check-wiring-closure.sh; then proceed; else fix wiring first; fi` — never ship a cycle over broken wiring.
- **The CLI is required for fitness measurement** — `ao goals measure` is the instrument; prose self-grades are not fitness.
- **Harvested-first selection order:** Harvested `.agents/rpi/next-work.jsonl` work outranks generated candidates; drain the harvest before generating.
- **Generator ladder (when the harvest is dry):** Testing improvements → Validation tightening and bug-hunt passes → Concrete feature suggestions.
- **Queue claim before consume:** claim it first (set the claim marker), keep `consumed: false` until the work actually lands; a crash between claim and consume must leave the row re-runnable.
- **Immediate queue reread:** after each $rpi turn, immediately re-read `.agents/rpi/next-work.jsonl` — the turn may have harvested new work.
- **Repo execution profile:** honor `docs/contracts/repo-execution-profile.md` (`startup_reads`, `validation_commands`) when present.

## Examples

```bash
$evolve                          # one gated improvement cycle against GOALS.md
$evolve --max-cycles=3           # bounded ladder run
$evolve --dry-run                # report the selected goal + plan, change nothing
```

## Contract management (absorbed from $autodev)

Treat retired CLI wrappers as terminal: the old ao-evolve / ao-rpi CLI commands are deleted
(ag-llni) — never invoke them as Codex defaults; they exist only behind `-tags legacy`.
In Codex, `$autodev` hands work to `$evolve` or `$rpi` as skill invocations. Repo-local
PROGRAM.md/AUTODEV.md contracts load per the pointer in the body above.
