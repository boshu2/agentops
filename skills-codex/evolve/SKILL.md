---
name: evolve
description: Run autonomous improvement loops.
---
# $evolve — Goal-Driven Autonomous Loop

> Measure what's wrong. Fix the worst thing. Measure again. **Whether the fixes *compound* into a durable knowledge moat is a tracked hypothesis, not a promise** — DEMOTED to unproven by [ADR-0004](../../docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md) and [ADR-0011](../../docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md). The proven product is the per-cycle verification (**no verdict = not done**), not the compounding. Do not market the flywheel ahead of the ruler.
> **Experimental tier.** Autonomous long-loop; run attended or dispatched onto a substrate, never as an in-repo daemon (ADR-0009).

**Cycle feedback is explicit.** Each completed execution unit routes `Validate -> Learn -> orchestrator`; neither proof nor bookkeeping controls retry or delivery.

**The loop runs as this skill.** `evolve` selects work and invokes complete `$rpi --auto` cycles — that *is* the loop. A material Learn packet returns to the orchestrator, which changes the remaining plan through Discovery and sends only that changed plan through Premortem. `no_change` permits an explicit continue/retry/stop/escalate decision; `terminal` closes the cycle. Substrates dispatch the whole loop as one unit; the former RPI CLI wrappers are retired (ADR-0009).

**Operator cadence:** measure repo state → select the next highest-value item → Discovery → Premortem → Crank → Validate → Learn → orchestrator decision → repeat until a kill switch, max-cycle cap, regression breaker, or real dormancy stops it.

## Constraints

- Run one complete `rpi --auto` cycle per selected item and re-read the work ladder afterward, because partial phases and fixed backlogs break the feedback loop.
- Never let Validate or Learn push, close work, mutate the plan, or choose the next cycle; those are separate adapter/orchestrator decisions.
- Treat breaker trips with one bounded helper pass before escalation; only judgment, refusal, spent budgets, or a failed helper reach a human, because ordinary blockers return to the earliest invalidated loop move.

## Work selection ladder

Selection is a ladder re-read from the TOP after every productive cycle — never a one-shot check. Full per-rung procedure (`ao loop next-work` recommendation, scope filter, generator code, `--quality` cascade, dormancy hard-gate): [references/work-selection-ladder.md](references/work-selection-ladder.md).

1. **Harvested** — `.agents/rpi/next-work.jsonl`, freshest unconsumed follow-up
2. **Open ready beads** — `ao beads exec ready`, highest priority
3. **Failing goals + directive gaps** — `ao goals measure` (skip if `--beads-only`; skip quarantined oscillators)
4. **Generators** — coverage / security / perf / refactor findings → beads or queue items (below)
5. **Complexity / TODO / drift / dead-code / stale-doc / stale-research mining**
6. **Feature suggestions** grounded in repo purpose when nothing sharper exists

`--quality` inverts the top (findings before goals). The metronome gate blocks a rung that would repeat the trailing run's `mode` (streak ≥3). **Dormancy is last resort** — empty queues mean "run the generators", not "stop"; go dormant only after queue AND generator layers come up empty across multiple consecutive passes.

**Work generators** (auto-invoked; skip with `--no-lifecycle`, which falls back to manual scanning):
- `$test coverage` → files with <40% coverage become queue items
- `$refactor --sweep all --dry-run` → functions with CC > 20 become queue items
- `$security audit` → deps with CVSS ≥ 7.0 or 2+ majors behind
- `Skill(skill="perf", args="profile --quick")` → hot-path perf findings

**Live skill-edit immune system:** if a cycle edits `skills/<slug>/SKILL.md`, run `ao skills edit seal --skill <slug> --actor "${AGENT_NAME:-agent}"` before handoff — the seal creates the rollback commit and records the `Skill-Edit` trailers for the daily digest. Critical skills in `docs/contracts/critical-skills.txt` reject unattended edits; `--allow-critical` only under supervision.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Stop after `N` completed cycles |
| `--dry-run` | off | Show planned cycle actions without executing |
| `--beads-only` | off | Skip goal measurement and run backlog-only selection |
| `--skip-baseline` | off | Skip first-run baseline snapshot |
| `--quality` | off | Prioritize harvested postmortem findings |
| `--compile` | off | Run `ao compile` knowledge warmup before cycle 1 |
| `--test-first` | on | Pass strict-quality defaults through to `rpi` |
| `--no-test-first` | off | Explicitly disable test-first passthrough to `rpi` |
| `--no-lifecycle` | off | Skip lifecycle work generators (falls back to manual scanning) |
| `--mode=burst\|loop` | burst | Operator-loop; STOP refused ([references/loop-mode.md](references/loop-mode.md)) |

## Execution Steps

**YOU MUST EXECUTE THIS WORKFLOW — do not just describe it.** **FULLY AUTONOMOUS:** every `rpi` uses `--auto`; do NOT ask the user anything (read `references/autonomous-execution.md` for the narrow operator-shape carve-out). Each cycle = one complete 3-phase `rpi` run. For broad AgentOps-domain evolution (skills, CLI, docs, tests, beads, knowledge) first read [references/domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md) — the BDD/DDD/Hexagonal/TDD/XP control surface + clean-room skill-factory guardrails.

### Step 0: Setup

**Stale-checkout survey guard (run FIRST):** `git fetch origin && git status -sb`. If behind/diverged AND a throwaway orchestration tree with no un-pushed work, `git reset --hard origin/main`. **Never `git pull --rebase` on the survey path** — it silently no-ops against a diverged local `main`, so merged files appear "missing".

```bash
git fetch origin && git status -sb              # survey guard — never `git pull --rebase` here
mkdir -p .agents/evolve
ao corpus inject --query "autonomous improvement cycle" --limit 5 2>/dev/null || true
# session state (idle_streak + mode_repeat_streak in .agents/evolve/session-state.json) is refreshed inline by the loop
```

Recover cycle state from disk (survives compaction): `CYCLE`, `IDLE_STREAK`, `GENERATOR_EMPTY_STREAK`, `LAST_SELECTED_SOURCE`, `CLAIMED_WORK_REF` from `.agents/evolve/session-state.json`; the canonical cycle ledger is `cycle-history.jsonl` (both **local-only** — the nested `.agents/.gitignore` denies all paths, so record durable milestones in commit messages too). **Prior-failure injection (mandatory):** read the last 3 `cycle-history.jsonl` entries; for any `gate` containing `FAIL|BLOCKED`, extract failure keywords and grep `.agents/learnings/` before selecting work — without this the loop re-derives the same lessons each cycle. Detail: `references/cycle-history.md`, `references/convergence-mechanics.md`.

**Repo-local contracts.** If `docs/contracts/repo-execution-profile.md` exists, read its ordered `startup_reads` and bootstrap from them before selecting work; cache `validation_commands`, `tracker_commands`, `definition_of_done`. If a repo-local `PROGRAM.md` (or `AUTODEV.md` alias — `PROGRAM.md` wins) contract exists, `rpi` loads it automatically — cache its `mutable_scope`, `validation_commands`, `decision_policy`, `stop_conditions`; prefer work inside mutable scope, never silently widen it around immutable files. The PROGRAM.md contract is the legacy autodev lane (built only under `-tags legacy`); its spec + repair guidance live in [docs/contracts/autodev-program.md](../../docs/contracts/autodev-program.md), with executable specs `references/autodev.feature` and `references/autodev-cli.feature`.

**Circuit breakers (tunable):** time-based (60 min no productive work) · max-cycles/max-attempts cap · cost/quota budget · oscillation. Ordinary REFUTED results auto-redo; a tripped breaker takes one bounded fresh-context helper pass, and only a failed helper or a skip class (refusal, explicit judgment, spent ceiling) reaches the human. Thresholds are configurable (`EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts), not hard-coded. **Oscillation quarantine:** pre-populate from cycle history (goals with 3+ improved→fail transitions). See `references/oscillation.md`.

### Step 0.2 / 0.5: Warmup + baseline

**Checkpoint:** `--compile` only (skip on `--dry-run`): run `ao compile` before cycle 1 per `references/knowledge-loop-integration.md`. On the first eligible run, capture the fitness baseline per `references/fitness-scoring.md`.

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
      echo "halt: $REASON"; exit 0  # kill/user_halt/dormant/goal_regression -> stop this cycle
    fi
  fi
fi
```

**Agile-first dormancy:** `DORMANT` is NEVER sticky while ready beads exist — `halt-check.sh` auto-clears it when `ao beads exec ready` / harvested work exists. KILL/STOP honor `EVOLVE_KILL_TTL_DAYS` (default 7); stale markers are surfaced and bypassed. `goal_regression` (`goals_passing_after < before`) halts for operator attention.

### Step 1.5: Healing-first classifier

`ao ci recent --limit 1` (typed BC2 `CIStatusPort`) → if the last push CI was `failure`, this cycle is **restorative-only**: Step 3 takes only CI-red-reducing work (harvested bugs, gate-fix beads, generator bug output) — no promotions, features, or new-shape work until green. A `gate=FAIL` in cycle-history auto-triggers this for cycle N+1. **Convergence check:** `ao loop converged --green-streak <n> --unconsumed-high-medium <n> [--fitness-baseline]` (typed BC3 `ConvergenceCheckPort`); branch on `.converged` (default: CI green streak ≥ 3, HIGH+MEDIUM next-work ≤ 1, baseline captured) — if true, emit teardown and do NOT re-arm. See `references/convergence-mechanics.md`.

### Step 2: Measure fitness

Skip if `--beads-only`. Run `ao goals measure` → `.agents/evolve/fitness-latest.json`. Full measurement, baseline capture, and post-cycle regression detection: `references/fitness-scoring.md`.

### Step 3: Select work

Run the ladder above; read [references/work-selection-ladder.md](references/work-selection-ladder.md) for the per-rung code. **Agile invariant:** `ao beads exec ready ≥ 1` ⇒ the loop NEVER writes DORMANT and NEVER exits — the only path to DORMANT is a fully empty backlog + dry generators (3 passes); context exhaustion → HANDOFF, not DORMANT. If `--dry-run`: report what would be worked on and go to Teardown.

### Step 4: Execute

Primary engine: `$rpi` (all 3 phases mandatory). `$implement` or `$crank` only when a bead has execution-ready scope.

```
Invoke $rpi "{normalized work title}" --auto --max-cycles=1     # harvested / goal / gap / testing / bug / drift / feature
Invoke $rpi "Complete {issue_id}: {title}" --auto --max-cycles=1 # a bead (fallback: $implement {issue_id})
Invoke $crank {epic_id}                                         # epic with children
```

If Step 3 created durable work instead of executing it, re-enter Step 3 and let the new bead win through normal selection. **Mechanical-batch hint:** > 20 uniform per-file edits → a script (`awk`/`sed`/`for`), not N Edit calls (`references/mechanical-batches.md`). **Pre-flight schema check:** a port/adapter migration whose consumer reads > 20% more fields than the target port projects → abort, convert to a port-widening cycle (`references/pre-flight-schema-check.md`). **Operator-shape carve-out:** `AskUserQuestion` permitted ONLY for shape decisions affecting > 50 files OR a schema/contract surface (`references/autonomous-execution.md`).

### Step 4.5: Source-surface sync (pre-gate)

Sync binaries and generated surfaces before the gate: Go CLI changes require build/install; Codex skill changes require hash regeneration; skill inventory changes require the full one-shot regeneration. Use [gate hygiene](references/gate-hygiene.md) and [new-skill landing](references/new-skill-landing.md), never piecemeal regen.

### Step 5: Regression gate

**Checkpoint:** run the project tests plus ordered repo-profile / PROGRAM.md `validation_commands` and wiring closure. Apply `decision_policy`, immutable scope, and `stop_conditions`; re-measure to `fitness-latest-post.json` and revert regressions. Keep claimed work `consumed: false` until the cycle succeeds, then re-read `.agents/rpi/next-work.jsonl`. Details: [fitness scoring](references/fitness-scoring.md), [gate hygiene](references/gate-hygiene.md), [knowledge integration](references/knowledge-loop-integration.md).

### Step 6: Log cycle + commit

**PRODUCTIVE** (improved / regressed / harvested): append the cycle record to `.agents/evolve/cycle-history.jsonl`, commit real changes. **IDLE** (nothing found even after generators): append a record with `result: "unchanged"`; no git add, no commit. Record the XP/BDD/TDD trace in the cycle record's `trace` field when a cycle worked a product or goal-backed gap (goal hypothesis → gap → Gherkin → failing proof → red/green → refactor → validation → ratchet → goal reshape); trivial one-shot cycles record a `trace.exemption_reason`. Trace completeness is advisory, never a gate. See `references/cycle-history.md`, `references/quality-mode.md`.

### Step 7: Optional deterministic delivery

After the immutable Validate proof passes through Learn and the orchestrator accepts the cycle, invoke the repository-selected deterministic `$push` adapter only when repo policy or the operator authorizes delivery. Pass the exact source SHA, destination, and deterministic check results. Push cannot change the verdict, close tracker state, or complete the lifecycle; without delivery authority, return the prepared SHA and evidence. A delivery failure returns the unchanged proof to the orchestrator.

### Step 7 loop / stop

After the caller records the delivery or prepared-handoff result, increment `CYCLE` and return to Step 1.

**Stop ONLY on** (all require a genuine reason — never just context size): (1) **KILL/STOP marker** — operator override; (2) **`--max-cycles` cap**; (3) **genuine stagnation** — `ao beads exec ready=0 AND harvested=0 AND failing-goals=0 AND GENERATOR_EMPTY_STREAK ≥ 2 AND IDLE_STREAK ≥ 2` → writes DORMANT, which auto-clears the moment `ao beads exec create` adds a ready bead; (4) **regression breaker after a revert**. **Context exhaustion is NOT a stop** — write `.agents/evolve/HANDOFF` (non-sticky), log `result: "context-handoff"`, exit the turn; the next fire clears HANDOFF in Step 1 and resumes (`references/context-budget.md`).

**Mandatory checkpoint — session-PR threshold (gates next cycle, NOT terminal):** at `session_pr_count >= 5`, invoke `$postmortem --deep` and wait for the verdict file. PASS → continue; WARN → continue with a caveat in the next cycle's `notes`; FAIL / non-convergence → write STOP. The agent MUST NOT self-grade or self-write STOP — STOP without a verdict is the 2026-05-20 anti-pattern (`references/postmortem-checkpoint.md`).

### Teardown

Commit any staged `cycle-history.jsonl`, run `$postmortem "evolve session: N cycles"` (a light session-end retrospective — it does NOT substitute for the council-gated threshold checkpoint), invoke `$push` for unpushed commits only when authorized, and report the summary (cycles, productive/regressed/idle counts, stop reason). Full procedure: `references/knowledge-loop-integration.md`, `references/teardown.md`. Never write `.agents/evolve/STOP` as a substitute for the checkpoint's verdict file.

Release-shaped branches must follow [the release teardown contract](references/teardown.md#release-shaped-teardown): never recommend `$release` from per-cycle `--fast`, carry the unchecked checklist into the handoff, and require the full release gate before tagging.

## Output Specification

- **Path:** emit the cycle summary to stdout; append `.agents/evolve/cycle-history.jsonl`; write `.agents/evolve/{fitness-latest.json,session-state.json}` and control files `{STOP,DORMANT,HANDOFF}`.
- **Filename:** cycle history is `cycle-history.jsonl`; current fitness and resumable state use the fixed filenames above.
- **Format:** stdout is Markdown; state and fitness use JSON; cycle history uses JSONL following `references/cycle-history.md`.
- **Validation command:** run repo/profile tests and `ao gate check --fast --scope head` (which subsumes wiring closure); if delivery is selected, also require `bash skills/push/scripts/validate.sh`.
- **Downstream handoff:** return cycle counts, fitness delta, result, stop reason, changed paths, immutable Validate verdict, and any deterministic delivery result; the next cycle consumes persisted state and unconsumed work.

## Quality Checklist

- Selected work follows the ladder and remains inside declared mutable scope.
- A productive cycle has deterministic validation plus an immutable candidate-current Validate verdict.
- Regressions revert, queue items stay unconsumed until authorized delivery succeeds or the caller releases them, and breaker handling follows helper-before-human policy.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Loop exits immediately | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` |
| Stagnation after repeated empty passes | Queue + producer layers empty across multiple passes — dormancy is the fallback outcome |
| `ao goals measure` hangs | Use `--timeout 30 --total-timeout 75`, or `--beads-only` to skip |
| Regression gate reverts | Review reverted changes, narrow scope, re-run; release claimed work back to available |

## References

- **Loop mechanics** — [work-selection-ladder.md](references/work-selection-ladder.md) (per-rung selection), [fitness-scoring.md](references/fitness-scoring.md) (baseline / regression / revert), [convergence-mechanics.md](references/convergence-mechanics.md) (healing-first classifier), [cycle-history.md](references/cycle-history.md) (JSONL, recovery, trace), [oscillation.md](references/oscillation.md), [metronome-gate.md](references/metronome-gate.md), [scout-mode.md](references/scout-mode.md), [long-loop-discipline.md](references/long-loop-discipline.md)
- **Gating + delivery preparation** — [gate-hygiene.md](references/gate-hygiene.md) (source-surface, red triage), [new-skill-landing.md](references/new-skill-landing.md) (six derived surfaces), [ao-command-landing.md](references/ao-command-landing.md), [postmortem-checkpoint.md](references/postmortem-checkpoint.md), [pre-flight-schema-check.md](references/pre-flight-schema-check.md), [mechanical-batches.md](references/mechanical-batches.md), [snapshot-pattern-for-long-cycle-gates.md](references/snapshot-pattern-for-long-cycle-gates.md)
- **Autonomy + knowledge** — [autonomous-execution.md](references/autonomous-execution.md) (loop rules + operator-shape carve-out), [context-budget.md](references/context-budget.md), [knowledge-loop-integration.md](references/knowledge-loop-integration.md) (claim/release, teardown), [compounding.md](references/compounding.md) (hypothesis-posture per ADR-0004/0011), [domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md), [quality-mode.md](references/quality-mode.md), [parallel-execution.md](references/parallel-execution.md), [teardown.md](references/teardown.md), [artifacts.md](references/artifacts.md)
## Behavioral contract anchors (validated by scripts/validate.sh)

The trim moved procedure to references/, but these invariants stay inline — the skill's own validator greps them, and they are the loop's load-bearing behavior:

- **Continuous values, not booleans:** every fitness metric reports a continuous value against a threshold (value/threshold), never a bare pass/fail.
- **Oscillation sweep (always-on, Step 0):** Pre-populate quarantine list from `ao compile`'s oscillation report before selecting a goal.
- **Wiring pre-flight (Step 5):** `if ao gate check --fast --scope head; then proceed; else fix wiring first; fi` — never ship a cycle over broken wiring (wiring closure folded into the gate after the always.wiring-closure meta-gate was retired).
- **The CLI is required for fitness measurement** — `ao goals measure` is the instrument; prose self-grades are not fitness.
- **Harvested-first selection order:** Harvested `.agents/rpi/next-work.jsonl` work outranks generated candidates; drain the harvest before generating.
- **Generator ladder (when the harvest is dry):** Testing improvements → Validation tightening and bug-hunt passes → Concrete feature suggestions.
- **Queue claim before consume:** claim it first (set the claim marker), keep `consumed: false` until authorized delivery succeeds or the caller finalizes the handoff; a crash between claim and consume must leave the row re-runnable.
- **Immediate queue reread:** after each $rpi turn, immediately re-read `.agents/rpi/next-work.jsonl` — the turn may have harvested new work.
- **Repo execution profile:** honor `docs/contracts/repo-execution-profile.md` (`startup_reads`, `validation_commands`) when present.

- **Specs + schemas** — [evolve.feature](references/evolve.feature) (gated cycles, ladder, never-self-halt), [goals-schema.md](references/goals-schema.md), [loop-mode.md](references/loop-mode.md), [examples.md](references/examples.md), [autodev.feature](references/autodev.feature) + [autodev-cli.feature](references/autodev-cli.feature) (legacy autodev lane, `-tags legacy`)

## Examples

`$evolve` runs until a genuine stop; `$evolve --max-cycles=3` bounds it; `$evolve --dry-run` reports selection without mutation. Full walkthroughs: [references/examples.md](references/examples.md).
- `skills/rpi/SKILL.md` — full lifecycle orchestrator (called per cycle)
- `skills/crank/SKILL.md` — epic execution (called for beads epics)
- `skills/postmortem/SKILL.md` — learning extraction + mining surface; absorbed the retired `/curate`, `/compile`, and `/flywheel` skills (mechanical surfaces are the `ao compile` and `ao flywheel status` CLI, not skills)
- `docs/contracts/autodev-program.md` — repo-local PROGRAM.md contract (legacy autodev lane)
- `GOALS.yaml` — fitness goals for this repo
- [test](../test/SKILL.md) · [refactor](../refactor/SKILL.md) · [security](../security/SKILL.md) · [validate](../validate/SKILL.md) — the work generators
