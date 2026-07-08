---
name: evolve
description: Run autonomous improvement loops.
practices:
- lean-startup
- dora-metrics
- agile-manifesto
hexagonal_role: domain
consumes:
- rpi
- goals
- post-mortem
produces:
- git-changes
- goals-fitness-delta
context_rel:
- kind: customer-of
  with: rpi
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  tier: experimental
  dependencies:
  - rpi
  - post-mortem
  triggers:
  - evolve
  - improve everything
  - autonomous improvement
  - run until done
  - postmortem and continue
  - analyze repo and keep going
output_contract: code changes, GOALS.md fitness deltas
---
# /evolve — Goal-Driven Autonomous Loop

> Measure what's wrong. Fix the worst thing. Measure again. **Whether the fixes *compound* into a durable knowledge moat is a tracked hypothesis, not a promise** — DEMOTED to unproven by [ADR-0004](../../docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md) and [ADR-0011](../../docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md). The proven product is the per-cycle verification (**no verdict = not done**), not the compounding. Do not market the flywheel ahead of the ruler.

> **Experimental tier.** Autonomous long-loop; run attended or dispatched onto a substrate, never as an in-repo daemon (ADR-0009).

**Cadence is pawl-gated, not per-tread** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)). Each cycle's heavy validation (pawl review, `/validate --mixed`, `/pre-land-refuters`) fires ONCE at the cycle's **bead-acceptance / land pawl** — not per slice or wave. The per-cycle regression gate (Step 5) is **chaos**: cheap and wrong-tolerant between pawls. Do NOT escalate every cycle to a cross-family panel "to be safe" — that re-creates the waterfall the ratchet avoids.

**The loop runs as this skill.** `evolve` selects work and invokes complete `/rpi --auto` cycles — that *is* the loop. Each cycle's post-mortem checkpoint is a **re-plan point** (re-scope / reorder / drop / add to the remaining queue from what the cycle taught), one altitude up from `/rpi`'s [agile re-plan loop](../rpi/references/agile-replan-loop.md) — agile across cycles, not a fixed backlog. Substrates dispatch the whole loop as one unit through NTM, Agent Mail, or `ao agent`; the former RPI CLI wrappers are retired (ADR-0009).

**Operator cadence:** post-mortem finished work → measure repo state → select the next highest-value item → let `rpi` run research → plan → pre-mortem → implement → validate → harvest follow-ups → repeat until a kill switch, max-cycle cap, regression breaker, or real dormancy stops it.

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
- `Skill(skill="test", args="coverage")` → files with <40% coverage become queue items
- `Skill(skill="refactor", args="--sweep all --dry-run")` → functions with CC > 20 become queue items
- `Skill(skill="security", args="audit")` → deps with CVSS ≥ 7.0 or 2+ majors behind
- `Skill(skill="perf", args="profile --quick")` → hot-path perf findings

**Live skill-edit immune system:** if a cycle edits `skills/<slug>/SKILL.md`, run `ao skills edit seal --skill <slug> --actor "${AGENT_NAME:-agent}"` before handoff — the seal creates the rollback commit and records the `Skill-Edit` trailers for the daily digest. Critical skills in `docs/contracts/critical-skills.txt` reject unattended edits; `--allow-critical` only under supervision.

```bash
/evolve                      # Run until kill switch, max-cycles, or real dormancy
/evolve --max-cycles=5       # Cap at 5 cycles
/evolve --dry-run            # Show what would be worked on, don't execute
/evolve --quality            # Quality-first: prioritize post-mortem findings
/evolve --compile            # ao compile knowledge warmup before cycle 1
# Flags compose (e.g. --quality --max-cycles=10); full table below.
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Stop after `N` completed cycles |
| `--dry-run` | off | Show planned cycle actions without executing |
| `--beads-only` | off | Skip goal measurement and run backlog-only selection |
| `--skip-baseline` | off | Skip first-run baseline snapshot |
| `--quality` | off | Prioritize harvested post-mortem findings |
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
bash scripts/evolve-update-session-state.sh 2>/dev/null || true  # refresh idle_streak + mode_repeat_streak
```

Recover cycle state from disk (survives compaction): `CYCLE`, `IDLE_STREAK`, `GENERATOR_EMPTY_STREAK`, `LAST_SELECTED_SOURCE`, `CLAIMED_WORK_REF` from `.agents/evolve/session-state.json`; the canonical cycle ledger is `cycle-history.jsonl` (both **local-only** — the nested `.agents/.gitignore` denies all paths, so record durable milestones in commit messages too). **Prior-failure injection (mandatory):** read the last 3 `cycle-history.jsonl` entries; for any `gate` containing `FAIL|BLOCKED`, extract failure keywords and grep `.agents/learnings/` before selecting work — without this the loop re-derives the same lessons each cycle. Detail: `references/cycle-history.md`, `references/convergence-mechanics.md`.

**Repo-local contracts.** If `docs/contracts/repo-execution-profile.md` exists, read its ordered `startup_reads` and bootstrap from them before selecting work; cache `validation_commands`, `tracker_commands`, `definition_of_done`. If a repo-local `PROGRAM.md` (or `AUTODEV.md` alias — `PROGRAM.md` wins) contract exists, `rpi` loads it automatically — cache its `mutable_scope`, `validation_commands`, `decision_policy`, `stop_conditions`; prefer work inside mutable scope, never silently widen it around immutable files. The PROGRAM.md contract is the legacy autodev lane (built only under `-tags legacy`); its spec + repair guidance live in [docs/contracts/autodev-program.md](../../docs/contracts/autodev-program.md), with executable specs `references/autodev.feature` and `references/autodev-cli.feature`.

**Circuit breakers (tunable — also the pawl-escalation governor):** time-based (60 min no productive work) · max-cycles/max-attempts cap · cost/quota budget · oscillation. These are the **same breakers that govern pawl escalation** — a REFUTED pawl auto-redoes; a human is pulled in only when a breaker trips. Thresholds are configurable (`EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts), not hard-coded. **Oscillation quarantine:** pre-populate from cycle history (goals with 3+ improved→fail transitions). See `references/oscillation.md`.

### Step 0.2 / 0.5: Warmup + baseline

`--compile` only (skip on `--dry-run`): `ao compile` knowledge warmup before cycle 1 — mine + signal notes per `references/knowledge-loop-integration.md`. First run only (skip on `--skip-baseline` / `--beads-only` / existing baseline): capture the fitness baseline per `references/fitness-scoring.md`.

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

Skip if `--beads-only`. Run `scripts/evolve-measure-fitness.sh` → `.agents/evolve/fitness-latest.json`. Full measurement, baseline capture, and post-cycle regression detection: `references/fitness-scoring.md`.

### Step 3: Select work

Run the ladder above; read [references/work-selection-ladder.md](references/work-selection-ladder.md) for the per-rung code. **Agile invariant:** `ao beads exec ready ≥ 1` ⇒ the loop NEVER writes DORMANT and NEVER exits — the only path to DORMANT is a fully empty backlog + dry generators (3 passes); context exhaustion → HANDOFF, not DORMANT. If `--dry-run`: report what would be worked on and go to Teardown.

### Step 4: Execute

Primary engine: `/rpi` (all 3 phases mandatory). `/implement` or `/crank` only when a bead has execution-ready scope.

```
Invoke /rpi "{normalized work title}" --auto --max-cycles=1     # harvested / goal / gap / testing / bug / drift / feature
Invoke /rpi "Land {issue_id}: {title}" --auto --max-cycles=1    # a bead (fallback: /implement {issue_id})
Invoke /crank {epic_id}                                         # epic with children
```

If Step 3 created durable work instead of executing it, re-enter Step 3 and let the new bead win through normal selection. **Mechanical-batch hint:** > 20 uniform per-file edits → a script (`awk`/`sed`/`for`), not N Edit calls (`references/mechanical-batches.md`). **Pre-flight schema check:** a port/adapter migration whose consumer reads > 20% more fields than the target port projects → abort, convert to a port-widening cycle (`references/pre-flight-schema-check.md`). **Operator-shape carve-out:** `AskUserQuestion` permitted ONLY for shape decisions affecting > 50 files OR a schema/contract surface (`references/autonomous-execution.md`).

### Step 4.5: Source-surface sync (pre-gate)

Sync downstream artifacts when the staged diff touches binary or embedded surfaces, or the gate fails on stale-binary / drift errors that look like real regressions (`references/gate-hygiene.md`):
- `cli/**/*.go` changed → `cd cli && make build && go install ./cmd/ao`
- `skills-codex/**` changed → `bash scripts/regen-codex-hashes.sh`

A skill touches **six derived surfaces** (registry.json, skill-domain-map, context-map, counts + the `SKILL-TIERS.md` row, codex twin, narrative counts) — regenerate in one shot via `scripts/regen-all.sh` + the codex/count steps, never piecemeal. Most-missed: `registry.json` (stale → `contracts-sync` + `correctness` fail together). Full procedure: [references/new-skill-landing.md](references/new-skill-landing.md).

### Step 5: Regression gate

After execution, run the project build+test bundle plus any repo-profile / PROGRAM.md `validation_commands` (de-duplicated, declared order, after the repo bootstrap checks), and `bash scripts/check-wiring-closure.sh` if present. A PROGRAM.md `decision_policy` is the cycle's first keep/revert rule set (breached immutable scope ⇒ regressed; failed program validation ⇒ regressed; a fired revert rule ⇒ revert before consuming claimed work). Treat `stop_conditions` as per-cycle done criteria — main tests green alone never marks a cycle successful. If not `--beads-only`, re-measure fitness → `fitness-latest-post.json` and revert on regression (`references/fitness-scoring.md`). Trust the structural marker `^.*Pass [0-9]+: (FAILED|BLOCKED)` over the trailing status line (`references/gate-hygiene.md`). Claim work first, keep `consumed: false` until the `/rpi` cycle succeeds, then re-read `.agents/rpi/next-work.jsonl` (`references/knowledge-loop-integration.md`).

### Step 6: Log cycle + commit

**PRODUCTIVE** (improved / regressed / harvested): log via `scripts/evolve-log-cycle.sh`, commit real changes. **IDLE** (nothing found even after generators): log `--result "unchanged"`; no git add, no commit. Record the XP/BDD/TDD trace via `--trace-json` when a cycle worked a product or goal-backed gap (goal hypothesis → gap → Gherkin → failing proof → red/green → refactor → validation → ratchet → goal reshape); trivial one-shot cycles record a `trace.exemption_reason`. Trace completeness is advisory, never a gate. See `references/cycle-history.md`, `references/quality-mode.md`.

### Step 7: Land — worktree → gate → pawl → push

Push to the shared trunk is the **mutate-shared-trunk pawl** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)): accumulation + a green local gate are necessary but **NOT sufficient** — a CONFIRMED, commit-current pawl verdict must exist first. Per productive bead, run the live land path from a per-cycle worktree:

```bash
git worktree add wt-<bead> -b <type>/<bead>-<slug>   # per-cycle worktree; never edit the shared checkout
# ...implement + Step 5 regression gate...
ao gate check --fast --scope head                    # smart Go cockpit gate — fail fast locally
scripts/pawl-review.sh <bead>                         # cross-family codex refuter vs the commit; on
                                                       # CONFIRMED it writes the commit-bound verdict the pre-push gate requires
scripts/pawl-land.sh <bead>                           # fetch+rebase, restamp the verdict onto the feat, single-shot push
```

`pawl-review.sh` REFUSES a same-family author (review codex-authored work with a different family). **Push is refused without a CONFIRMED verdict** (`scripts/check-pawl-pre-push.sh`; a `#trivial` provenance-only commit is the only waiver). **REFUTED → AUTO-REDO** — the loop re-gates with no human; it prints the defects to fix, then re-runs. A human is pulled in only when a Step-0 circuit breaker trips (max-attempts, time, cost/quota, oscillation); the disposition is then `ESCALATE`/`HOLD` and the push is held. The operator stays *on* the loop (intent + STOP marker), not *in* it ([ADR-0008](../../docs/adr/ADR-0008-evolve-intelligent-agile-operating-model.md)). Never `claude -p` to redo (LAW 0).

### Step 7 loop / stop

```bash
while true; do
  # Step 1 .. Step 7
  CYCLE=$((CYCLE + 1))
done
```

**Stop ONLY on** (all require a genuine reason — never just context size): (1) **KILL/STOP marker** — operator override; (2) **`--max-cycles` cap**; (3) **genuine stagnation** — `ao beads exec ready=0 AND harvested=0 AND failing-goals=0 AND GENERATOR_EMPTY_STREAK ≥ 2 AND IDLE_STREAK ≥ 2` → writes DORMANT, which auto-clears the moment `ao beads exec create` adds a ready bead; (4) **regression breaker after a revert**. **Context exhaustion is NOT a stop** — write `.agents/evolve/HANDOFF` (non-sticky), log `result: "context-handoff"`, exit the turn; the next fire clears HANDOFF in Step 1 and resumes (`references/context-budget.md`).

**Mandatory checkpoint — session-PR threshold (gates next cycle, NOT terminal):** at `session_pr_count >= 5`, invoke `/post-mortem --deep` and wait for the verdict file. PASS → continue; WARN → continue with a caveat in the next cycle's `notes`; FAIL / non-convergence → write STOP. The agent MUST NOT self-grade or self-write STOP — STOP without a verdict is the 2026-05-20 anti-pattern (`references/postmortem-checkpoint.md`).

### Teardown

Commit any staged `cycle-history.jsonl`, run `/post-mortem "evolve session: N cycles"` (a light session-end retrospective — it does NOT substitute for the council-gated threshold checkpoint), push only if unpushed commits exist, and report the summary (cycles, productive/regressed/idle counts, stop reason). Full procedure: `references/knowledge-loop-integration.md`, `references/teardown.md`. Never write `.agents/evolve/STOP` as a substitute for the checkpoint's verdict file.

**Release-shaped branches** (`release/*`, `v*-prep`, `v*-evolve-run`, `v\d+\.\d+*`): the teardown MUST NOT recommend `/release`. Per-cycle `--fast` is a smoke test, not release readiness — the operator runs the **full** Go gate and confirms green before tagging:

```
## Pre-release checklist — REQUIRED before /release

[ ] 1. Regenerate derived surfaces if any cobra command/flag changed:
       bash scripts/regen-all.sh          # COMMANDS.md, registry.json, maps
       # Adding an `ao` command also needs cobra_commands_test.go expectedCmds (x2)
       # + the CLI-command-surface counts — references/ao-command-landing.md
       git diff cli/docs/COMMANDS.md registry.json   # commit if non-empty
[ ] 2. Full release gate (every check, routing ignored):
       ao gate check --full --workflow-coverage --require-workflow-parity
[ ] 3. Smoke /evolve with new typed read paths if BC port wire-ups changed:
       /evolve --dry-run --max-cycles=1

Only after [1]–[2] pass: /release <version>
```

The handoff artifact (e.g. `.agents/runs/<release>/READY-TO-TAG.md`) MUST contain this checklist verbatim, unchecked. "Ready to tag" means the boxes are checked, not that the loop ran cleanly. (Rationale: a v2.41-evolve-run shipped green code for three cycles but never ran the full gate; a removed CLI flag's reference regen was load-bearing.)

## Output

Per-cycle markdown summary to stdout (goals fixed, fitness delta, result); appends `.agents/evolve/cycle-history.jsonl`; writes `fitness-latest.json` + `session-state.json`; honors control files `.agents/evolve/{STOP,DORMANT,HANDOFF}`. Resume a paused cycle via `/evolve --resume`.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Loop exits immediately | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` |
| Stagnation after repeated empty passes | Queue + producer layers empty across multiple passes — dormancy is the fallback outcome |
| `ao goals measure` hangs | Use `--timeout 30 --total-timeout 75`, or `--beads-only` to skip |
| Regression gate reverts | Review reverted changes, narrow scope, re-run; release claimed work back to available |

See `references/cycle-history.md` for advanced troubleshooting.

## References

- **Loop mechanics** — [work-selection-ladder.md](references/work-selection-ladder.md) (per-rung selection), [fitness-scoring.md](references/fitness-scoring.md) (baseline / regression / revert), [convergence-mechanics.md](references/convergence-mechanics.md) (healing-first classifier), [cycle-history.md](references/cycle-history.md) (JSONL, recovery, trace), [oscillation.md](references/oscillation.md), [metronome-gate.md](references/metronome-gate.md), [scout-mode.md](references/scout-mode.md), [long-loop-discipline.md](references/long-loop-discipline.md)
- **Gating + landing** — [gate-hygiene.md](references/gate-hygiene.md) (source-surface, red triage), [new-skill-landing.md](references/new-skill-landing.md) (six derived surfaces), [ao-command-landing.md](references/ao-command-landing.md), [postmortem-checkpoint.md](references/postmortem-checkpoint.md), [pre-flight-schema-check.md](references/pre-flight-schema-check.md), [mechanical-batches.md](references/mechanical-batches.md), [snapshot-pattern-for-long-cycle-gates.md](references/snapshot-pattern-for-long-cycle-gates.md)
- **Autonomy + knowledge** — [autonomous-execution.md](references/autonomous-execution.md) (loop rules + operator-shape carve-out), [context-budget.md](references/context-budget.md), [knowledge-loop-integration.md](references/knowledge-loop-integration.md) (claim/release, teardown), [compounding.md](references/compounding.md) (hypothesis-posture per ADR-0004/0011), [domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md), [quality-mode.md](references/quality-mode.md), [parallel-execution.md](references/parallel-execution.md), [teardown.md](references/teardown.md), [artifacts.md](references/artifacts.md)
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
- **Immediate queue reread:** after each /rpi turn, immediately re-read `.agents/rpi/next-work.jsonl` — the turn may have harvested new work.
- **Repo execution profile:** honor `docs/contracts/repo-execution-profile.md` (`startup_reads`, `validation_commands`) when present.

## Examples

```bash
/evolve                          # one gated improvement cycle against GOALS.md
/evolve --max-cycles=3           # bounded ladder run
/evolve --dry-run                # report the selected goal + plan, change nothing
```

Full walkthroughs: [references/examples.md](references/examples.md).

- **Specs + schemas** — [evolve.feature](references/evolve.feature) (gated cycles, ladder, never-self-halt), [goals-schema.md](references/goals-schema.md), [loop-mode.md](references/loop-mode.md), [examples.md](references/examples.md), [autodev.feature](references/autodev.feature) + [autodev-cli.feature](references/autodev-cli.feature) (legacy autodev lane, `-tags legacy`)

## See Also

- `skills/rpi/SKILL.md` — full lifecycle orchestrator (called per cycle)
- `skills/crank/SKILL.md` — epic execution (called for beads epics)
- `skills/post-mortem/SKILL.md` — learning extraction + mining surface; absorbed the retired `/curate`, `/compile`, and `/flywheel` skills (mechanical surfaces are the `ao compile` and `ao flywheel status` CLI, not skills)
- `docs/contracts/autodev-program.md` — repo-local PROGRAM.md contract (legacy autodev lane)
- `GOALS.yaml` — fitness goals for this repo
- [test](../test/SKILL.md) · [refactor](../refactor/SKILL.md) · [security](../security/SKILL.md) · [validate](../validate/SKILL.md) — the work generators
