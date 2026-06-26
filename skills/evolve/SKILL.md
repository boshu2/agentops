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
- compile
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
  tier: execution
  dependencies:
  - rpi
  - post-mortem
  - compile
  triggers:
  - evolve
  - improve everything
  - autonomous improvement
  - run until done
  - postmortem and continue
  - analyze repo and keep going
output_contract: code changes, GOALS.md fitness deltas
---
# /evolve — Goal-Driven Compounding Loop

> Measure what's wrong. Fix the worst thing. Measure again. Compound.

> **Cadence is pawl-gated, not per-tread** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)). Each cycle's heavy validation (full council, `/validate --mixed`, `/pre-land-refuters`) fires at that cycle's **bead-acceptance / merge-to-main pawl** — once per bead, not per slice or wave. The per-cycle regression gate (Step 5) and lightweight in-cycle checks are **chaos**: cheap, run freely, wrong-tolerant between pawls. Do NOT escalate every cycle to a cross-family panel "to be safe" — that re-creates the waterfall the ratchet exists to avoid (`--mixed` is reserved for strategic decisions; see `references/postmortem-checkpoint.md`). The bead is fully validated at its acceptance pawl — that is the ratchet's lock.

**The loop runs as this skill (skills-are-the-runtime).** `evolve` selects work
and invokes complete `/rpi --auto` cycles — that *is* the loop. Each cycle's
post-mortem checkpoint is a **re-plan point, not just stop/continue**: it may
re-scope, reorder, drop, or add to the *remaining* queue/goal from what the cycle
taught (`/rpi`'s [Agile Re-Plan Loop](../rpi/references/agile-replan-loop.md), one
altitude up — agile across cycles, not a fixed backlog run to the letter).
Substrates dispatch the whole `evolve` skill loop as one unit through NTM,
Agent Mail, or `ao agent`; the former RPI CLI wrappers are retired under
ADR-0009.

**Operator cadence:** post-mortem finished work, analyze repo state, select or
create the next highest-value work item, let `rpi` handle research, planning,
pre-mortem, implementation, and validation, then harvest follow-ups and repeat
until a kill switch, max-cycle cap, regression breaker, or real dormancy stops
the run.

Always-on autonomous loop over `rpi`. Work selection order:
1. **Harvested `.agents/rpi/next-work.jsonl` work** (freshest concrete follow-up)
2. **Open ready beads work** (`bd ready`)
3. **Failing goals and directive gaps** (`ao goals measure`)
4. **Testing improvements** (missing/thin coverage, missing regression tests)
5. **Validation tightening and bug-hunt passes** (gates, audits, bug sweeps)
6. **Complexity / TODO / FIXME / drift / dead code / stale docs / stale research mining**
7. **Concrete feature suggestions** derived from repo purpose when no sharper work exists

**Work generators** that feed the selection ladder (auto-invoked, skip with `--no-lifecycle`):
- `Skill(skill="test", args="coverage")` → files with <40% coverage become queue items (Step 3.4)
- `Skill(skill="refactor", args="--sweep all --dry-run")` → functions with CC > 20 become queue items (Step 3.6)
- `Skill(skill="deps", args="audit")` → deps with CVSS >= 7.0 or 2+ major versions behind become queue items (Step 3.5)
- `Skill(skill="perf", args="profile --quick")` → perf findings become queue items when hot paths detected (Step 3.5)

**Dormancy is last resort.** Empty current queues mean "run the generator layers", not "stop". Only go dormant after the queue layers and generator layers come up empty across multiple consecutive passes.

**Live skill edit immune system:** if an evolve cycle edits
`skills/<slug>/SKILL.md`, run
`ao skills edit seal --skill <slug> --actor "${AGENT_NAME:-agent}"` before the
cycle hands off. The seal creates the rollback commit and records the
`Skill-Edit` trailers used by the daily digest. Critical skills listed in
`docs/contracts/critical-skills.txt` reject unattended edits; use
`--allow-critical` only when Bo is supervising that critical edit.

```bash
/evolve                      # Run until kill switch, max-cycles, or real dormancy
/evolve --max-cycles=5       # Cap at 5 cycles
/evolve --dry-run            # Show what would be worked on, don't execute
/evolve --beads-only         # Skip goals measurement, work beads backlog only
/evolve --quality            # Quality-first mode: prioritize post-mortem findings
/evolve --quality --max-cycles=10  # Quality mode with cycle cap
/evolve --compile            # Mine → Defrag warmup before first cycle
/evolve --compile --max-cycles=5 # Warm knowledge base then run 5 cycles
/evolve --test-first         # Default strict-quality /rpi execution path
/evolve --no-test-first      # Explicit opt-out from test-first mode
```

## Delineation vs Nightly Knowledge Compounding

| Lane | Runs | Mutates code? | Mutates corpus? | Outer loop? | Budget |
|------|------|---------------|-----------------|-------------|--------|
| `$curate --mode=dream` | nightly, private local | **No** | **Yes (heavy)** | **Yes (convergence)** | wall-clock + plateau |
| `evolve` | daytime, operator-driven | Yes (via `rpi`) | Yes (light) | Yes | cycle cap |

**The old dream skill is retired**; out-of-session compounding moved to Gas City and the current skill surface is `$curate --mode=dream`. `/evolve` owns the live daytime code-compounding lane. Both still share the fitness-measurement substrate via `corpus.Compute` / `ao goals measure`.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Stop after `N` completed cycles |
| `--dry-run` | off | Show planned cycle actions without executing |
| `--beads-only` | off | Skip goal measurement and run backlog-only selection |
| `--skip-baseline` | off | Skip first-run baseline snapshot |
| `--quality` | off | Prioritize harvested post-mortem findings |
| `--compile` | off | Run `ao mine` + `ao defrag` warmup before cycle 1 |
| `--test-first` | on | Pass strict-quality defaults through to `rpi` |
| `--no-test-first` | off | Explicitly disable test-first passthrough to `rpi` |
| `--no-lifecycle` | off | Skip lifecycle work generators in Steps 3.4-3.6 (/test, /security, /perf, /refactor). Falls back to manual scanning. |
| `--mode=burst\|loop` | burst | Operator-loop; STOP refused. [loop-mode.md](references/loop-mode.md). |

## Execution Steps

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

**FULLY AUTONOMOUS.** Read `references/autonomous-execution.md`. Every `rpi` uses `--auto`. Do NOT ask the user anything. Each cycle = complete 3-phase `rpi` run.

For broad AgentOps 3.0 domain evolution across skills, CLI, hooks, docs, tests,
beads, and knowledge, first read
[references/domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md).
It supplies the BDD/DDD/Hexagonal/TDD/XP control surface and the clean-room
skill-factory guardrails.

### Step 0: Setup

**Stale-checkout survey guard (run FIRST).** Before any tree-reading survey: `git fetch origin && git status -sb`. If the checkout is behind/diverged AND it is a throwaway orchestration tree with no un-pushed work, `git reset --hard origin/main`. **BAN `git pull --rebase` on the survey path** — it silently no-ops against a diverged local `main`, so merged files appear "missing" and the survey investigates already-merged work.

```bash
git fetch origin && git status -sb              # survey guard — never `git pull --rebase` here
mkdir -p .agents/evolve
ao corpus inject --query "autonomous improvement cycle" --limit 5 2>/dev/null || true
bash scripts/evolve-update-session-state.sh 2>/dev/null || true  # refresh derived idle_streak + mode_repeat_streak
```

`ao corpus inject` routes through the typed BC1 `CorpusReaderPort`
(`cli/cmd/ao/corpus_reader_adapter.go`), emitting one ranked `ports.CorpusItem`
JSON record per line from `.agents/learnings/` (soc-y5vh.1 — load-bearing on the
typed port, not an untyped `ao lookup` shell-out).

**Apply retrieved knowledge:** If learnings are returned, check each for applicability to the current improvement cycle. For applicable learnings, cite by filename and record: `ao metrics cite "<path>" --type applied 2>/dev/null || true`

**Prior-failure injection (mandatory):** read the last 3 entries of `.agents/evolve/cycle-history.jsonl`. For any with `gate` containing `FAIL|FAILED|BLOCKED`, extract failure-surface keywords (`registry|bats|markdown|supergate|canary|coverage|toolchain`) and search `.agents/learnings/` for matching learnings. Print the top matches before work selection. Without this read path, the loop accumulates write-only ledgers and re-derives lessons each cycle. See `references/convergence-mechanics.md` for the full recipe.

Before cycle recovery, load the repo execution profile contract when it exists — the source for repo policy; the user prompt supplies mission/objective, not startup reads, validation bundle, tracker rules, or `definition_of_done`.

- Locate `docs/contracts/repo-execution-profile.md` and `docs/contracts/repo-execution-profile.schema.json`.
- Read the ordered `startup_reads` and bootstrap from those paths before selecting work.
- Cache repo `validation_commands`, `tracker_commands`, and `definition_of_done` into session state.
- If present but missing required fields, stop or downgrade with an explicit warning before cycle 1. Do not invent repo policy.
- Read operating-doctrine ADRs (`docs/adr/` or `docs/decisions/`) when present: only operator markers stop the loop; the bead queue is a hypothesis re-confirmed against the goal, not spec; file-a-bead when a candidate is architecture disguised as bounded work.

Then load the repo-local autodev program contract when it exists — `PROGRAM.md` or `AUTODEV.md` is the execution layer for the current improvement loop.

- Locate `PROGRAM.md` and `AUTODEV.md`. `PROGRAM.md` takes precedence.
- Read the resolved program before cycle recovery and cache `program_path`, `mutable_scope`, `immutable_scope`, `validation_commands`, `decision_policy`, and `stop_conditions` into session state.
- If structurally invalid, stop or downgrade with an explicit warning before cycle 1.
- When a program contract exists, prefer work that lands wholly inside mutable scope. Do not silently widen scope around immutable files.

Recover cycle number, generator streaks, and the last claimed work item from disk (survives context compaction). Initialize `CYCLE` from `cycle-history.jsonl`, recover `IDLE_STREAK`, `GENERATOR_EMPTY_STREAK`, `LAST_SELECTED_SOURCE`, and `CLAIMED_WORK_REF` from `session-state.json`.

**Circuit breakers (tunable; also the pawl-escalation governor):** Time-based (60 min no productive work) · max-cycles / max-attempts cap · cost/quota budget · oscillation/no-forward-progress. These are the **same breakers that govern pawl escalation** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md) "Escalation — the circuit-breaker model"): a REFUTED pawl auto-redoes, and a human is pulled in only when one of these breakers trips. Thresholds are configurable (`EVOLVE_KILL_TTL_DAYS`, `--max-cycles`, max-attempts), not hard-coded.

**Oscillation quarantine:** Pre-populate quarantine list from cycle history (scan for goals with 3+ improved-to-fail transitions) — this is the **oscillation / no-forward-progress breaker**. See `references/oscillation.md`.

Parse flags: `--max-cycles=N` (default unlimited), `--dry-run`, `--beads-only`, `--skip-baseline`, `--quality`, `--compile`.

Track cycle-level execution state:

```text
evolve_state = {
  cycle: <current cycle number>,
  mode: <standard|quality|beads-only>,
  test_first: <true by default; false only when --no-test-first>,
  repo_profile_path: <docs/contracts/repo-execution-profile.md or null>,
  startup_reads: <ordered repo bootstrap paths>,
  validation_commands: <ordered repo validation bundle>,
  tracker_commands: <repo tracker shell wrappers>,
  definition_of_done: <repo stop predicates>,
  program_path: <PROGRAM.md|AUTODEV.md or null>,
  program_mutable_scope: <declared mutable paths/globs>,
  program_immutable_scope: <declared immutable paths/globs>,
  program_validation_commands: <ordered program validation bundle>,
  program_decision_policy: <ordered keep/revert rules>,
  program_stop_conditions: <ordered cycle done criteria>,
  generator_empty_streak: <consecutive passes where all generator layers returned nothing>,
  last_selected_source: <harvested|beads|goal|directive|testing|validation|bug-hunt|drift|feature>,
  claimed_work: <null or work reference being worked>,
  queue_refresh_count: <incremented after every /rpi cycle>
}
```

Persist `evolve_state` to `.agents/evolve/session-state.json` at each cycle boundary, after work claims, after release/finalize, and during teardown. `cycle-history.jsonl` is the canonical cycle ledger; `session-state.json` carries resume-only state not yet earning a committed cycle entry. Both are **local-only** (the nested `.agents/.gitignore` denies all paths) — record durable milestones in commit messages too. See `references/cycle-history.md`.

### Step 0.2: Compile Warmup (--compile only)

Skip if `--compile` was not passed or if `--dry-run`. Read `references/knowledge-loop-integration.md` for the full warmup procedure (mine + defrag + signal notes).

### Step 0.5: Baseline (first run only)

Skip if `--skip-baseline` or `--beads-only` or baseline already exists. Read `references/fitness-scoring.md` for the baseline capture procedure.

### Step 1: Kill Switch Check

Run at the TOP of every cycle:

```bash
CYCLE_START_SHA=$(git rev-parse HEAD)
# Mechanical pre-cycle gate (soc-sfjx): markers (KILL/STOP/DORMANT/HANDOFF with
# TTL + soc-5qit non-sticky semantics), goal-regression, and prior-cycle-FAIL.
# This is a SCRIPT the loop MUST run, not prose it can skip — externalized from
# the old inline block so the kill-switch + revert-on-red are enforced, not
# advisory. Adapted from the mt-olympus unbounded-evolve substrate.
if [ -x scripts/evolve/halt-check.sh ]; then
  if ! HALT_OUT=$(bash scripts/evolve/halt-check.sh --json); then
    REASON=$(printf '%s' "$HALT_OUT" | jq -r '.halt_reason // "unknown"')
    if [ "$REASON" = "prior_cycle_fail" ]; then
      export EVOLVE_RESTORATIVE=1   # not terminal: Step 1.5 restricts scope to CI-red reduction
    else
      echo "halt: $REASON"; exit 0  # kill/user_halt/dormant/goal_regression -> stop this cycle
    fi
  fi
else
  # Fallback for repos without the substrate: minimal inline marker check.
  for m in "$HOME/.config/evolve/KILL" .agents/evolve/STOP; do [ -f "$m" ] && { echo "halt: $m"; exit 0; }; done
  [ -f .agents/evolve/DORMANT ] && { [ "$(bd ready --json 2>/dev/null | jq -r 'length // 0')" -gt 0 ] && rm -f .agents/evolve/DORMANT || { echo dormant; exit 0; }; }
  [ -f .agents/evolve/HANDOFF ] && rm -f .agents/evolve/HANDOFF
fi
```

**Agile-first dormancy (soc-5qit):** `DORMANT` is NEVER sticky while ready beads exist — `halt-check.sh` auto-clears it when `bd ready`/harvested work exists. KILL/STOP honor `EVOLVE_KILL_TTL_DAYS` (default 7); stale markers are surfaced and bypassed. `goal_regression` (latest cycle report `goals_passing_after < before`) halts the loop for operator attention. Heavy-context sessions write non-sticky HANDOFF; the next fire clears it and resumes. The gate is mechanical: see `scripts/evolve/halt-check.sh`.

### Step 1.5: Healing-first classifier

Before fitness or work selection, classify the cycle: `ao ci recent --limit 1 2>/dev/null | jq -r '.Conclusion // empty'` (typed BC2 `CIStatusPort`, soc-y5vh.2). If the last push CI was `failure`, this cycle is **restorative-only** — Step 3 takes only CI-red-reducing work (bug-type harvested items, gate-failure-fix beads, generator bug output); no promotions, features, or new-shape work until green. A `gate=FAIL` in cycle-history.jsonl auto-triggers this for cycle N+1 (and `halt-check.sh` surfaces it as `prior_cycle_fail`). See `references/convergence-mechanics.md`.

**Convergence check:** evaluate the STOP predicate via the typed BC3 `ConvergenceCheckPort` — `ao loop converged --green-streak <n> --unconsumed-high-medium <n> [--fitness-baseline]` (soc-y5vh.8). Branch on `.converged` (default: CI green streak ≥ 3, HIGH+MEDIUM next-work ≤ 1, fitness baseline captured); if true, emit teardown and do NOT re-arm wakeup.

### Step 2: Measure Fitness

Skip if `--beads-only`. Run `scripts/evolve-measure-fitness.sh` to produce a rolling fitness snapshot at `.agents/evolve/fitness-latest.json`. Read `references/fitness-scoring.md` for the full measurement procedure, baseline capture, and post-cycle regression detection.

### Step 3: Select Work

Selection is a ladder, not a one-shot check — after every productive cycle, return to the TOP and re-read the queue before considering dormancy. **Read [references/work-selection-ladder.md](references/work-selection-ladder.md) for the full per-rung procedure** (programmatic `ao loop next-work` recommendation, scope filter, metronome gate, the generator rungs with their code blocks, the `--quality` inverted cascade, and the dormancy hard-gate).

Ladder order (standard mode):
- **3.0 Scope filter** (soc-5qit) — split-or-defer oversized candidates via scout-mode; never bail.
- **3.1 Harvested** — `.agents/rpi/next-work.jsonl`, highest-value unconsumed.
- **3.2 Open ready beads** — `bd ready`, highest priority.
- **3.3 Failing goals + directive gaps** — skip if `--beads-only`; skip quarantined oscillators.
- **3.4–3.6 Generators** — `/test` coverage, `/security`+`/perf`, `/refactor`; findings → beads/queue items.
- **3.7 Feature suggestions** grounded in repo purpose.

`--quality` inverts the top (findings before goals/directives). The metronome gate blocks a rung that would repeat the trailing run's `mode` (streak ≥3).

**Agile invariant (soc-5qit):** `bd ready ≥ 1` ⇒ the loop NEVER writes DORMANT and NEVER exits. The only path to DORMANT is a fully empty backlog + dry generators (3 passes). Context exhaustion → HANDOFF, not DORMANT. Under loop mode, `write-stop-marker` refuses → log blocked + operator-wait (ADR-0007).

If `--dry-run`: report what would be worked on and go to Teardown.

### Step 4: Execute

Primary engine: `rpi` for implementation-quality work (all 3 phases mandatory). `/implement` or `/crank` only when a bead has execution-ready scope.

If a repo-local `PROGRAM.md` contract is active, `rpi` will load it automatically. `evolve` must compose with that behavior, not bypass it:
- Do not select work that is obviously outside mutable scope.
- If a bead or goal would require edits under immutable scope, escalate it or convert it into durable follow-up work instead of launching `rpi`.
- When work is plausibly in scope but still uncertain, let `rpi` discovery validate the fit and surface a scope escape explicitly.

For a **harvested item, failing goal, directive gap, testing improvement, validation tightening task, bug-hunt result, drift finding, or feature suggestion**:
```
Invoke /rpi "{normalized work title}" --auto --max-cycles=1
```

For a **beads issue**:
```
Prefer: /rpi "Land {issue_id}: {title}" --auto --max-cycles=1
Fallback: /implement {issue_id}
```
Or for an epic with children: `Invoke /crank {epic_id}`.

If Step 3 created durable work instead of executing it immediately, re-enter Step 3 and let the newly-created bead item win through the normal selection order.

**Mechanical-batch hint:** when the implementation phase identifies > 20 uniform per-file edits, prefer a script (`awk`/`sed`/`for f in $candidates`) over N tool-level Edit calls. See `references/mechanical-batches.md` for the decision rule and the script-first pattern.

**Pre-flight schema check (architectural migrations):** if the selected work is a port/adapter migration that rewires an existing consumer, BEFORE invoking `rpi`, sample two consumer call sites and compare field-use against the target port surface. If the consumer reads > 20% more fields than the port projects, abort and convert into a port-widening cycle instead. Encoded lesson: `docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md`; procedure: `references/pre-flight-schema-check.md`.

**Operator-shape carve-out:** `AskUserQuestion` is permitted ONLY for shape decisions affecting > 50 files OR a schema/contract surface (carrier choice, struct-field shape, frontmatter-key shape). See `references/autonomous-execution.md` for the bound on this exception.

### Step 4.5: Source-surface detection (pre-gate sync)

Before invoking the regression gate, sync downstream artifacts when the staged diff touches binary or embedded surfaces:

- `cli/**/*.go` changed → `cd cli && make build && go install ./cmd/ao`
- `skills/**` or `hooks/**` changed → `cd cli && make sync-hooks`
- `skills-codex/**` changed → `bash scripts/regen-codex-hashes.sh`

Without these, the gate fails on stale-binary or embedded-drift errors that look like real regressions. See `references/gate-hygiene.md` for the detection recipe.

**Adding or modifying a skill?** A skill touches **six derived surfaces** (registry.json SKU catalog, skill-domain-map, context-map, skill counts + the hand-added `SKILL-TIERS.md` row, codex twin, narrative counts) — regenerate them in one shot via `scripts/regen-all.sh` + the codex/count steps, never piecemeal. The most-missed is `registry.json` (stale → `contracts-sync` + `correctness(ubuntu)` fail together). See [references/new-skill-landing.md](references/new-skill-landing.md) for the full procedure, then [references/gate-hygiene.md](references/gate-hygiene.md) for the pre-push diff-scope check + pre-existing-vs-mine red triage.

### Step 5: Regression Gate

After execution, run the project build+test bundle. If the repo execution profile declared `validation_commands`, run them. If a repo-local program contract exists, run its `validation_commands` too, de-duplicated and in declared order after the repo bootstrap checks. Also check `if [ -f scripts/check-wiring-closure.sh ]; then bash scripts/check-wiring-closure.sh; fi`.

Use the program contract's `decision_policy` as the first keep/revert rule set for the cycle:
- if the cycle breached immutable scope, treat it as regressed
- if program validation commands fail, treat it as regressed
- if the decision policy declares a revert rule that fired, revert before consuming claimed work or advancing the queue

Treat program `stop_conditions` as per-cycle done criteria. Do not mark claimed work consumed, completed, or productive until both the stop conditions and the regression gate pass.

If not `--beads-only`, re-measure fitness to `fitness-latest-post.json` and detect regressions. The AgentOps CLI is required for fitness measurement. Read `references/fitness-scoring.md` for the full measurement, regression detection, and revert procedure.

**Gate output parsing:** trust the structural marker `^.*Pass [0-9]+: (FAILED|BLOCKED)` over the trailing status line — the trailing line conflates blocking and advisory results. See `references/gate-hygiene.md`.

Work finalization after the regression gate: claim it first, then keep `consumed: false` until the /rpi cycle succeeds. After the cycle's `/post-mortem` finishes, immediately re-read `.agents/rpi/next-work.jsonl` before selecting the next item. Read `references/knowledge-loop-integration.md` for full claim/release semantics.

### Step 6: Log Cycle + Commit

Two paths: productive cycles get committed, idle cycles are local-only.

**PRODUCTIVE cycles** (result is improved, regressed, or harvested): compute quality score (if `--quality`), log via `scripts/evolve-log-cycle.sh`, commit if real changes exist. See `references/quality-mode.md` for scoring.

**IDLE cycles** (nothing found even after generator layers): log via `evolve-log-cycle.sh` with `--result "unchanged"`. No git add, no commit.

**Record the XP/BDD/TDD trace.** When a cycle worked a product or goal-backed gap, pass `--trace-json` to `evolve-log-cycle.sh` (or `ao loop append`) so the cycle records the continuous-evolution kernel — goal hypothesis → selected gap → Gherkin scenario → first failing proof → red/green evidence → refactor note → validation evidence → ratchet action → goal reshape — letting a reviewer reconstruct the cycle without the transcript. A trivial one-shot cycle records a `trace.exemption_reason` instead. Trace completeness is advisory, never a gate. See `references/cycle-history.md`.

### Step 7: Loop or Stop

```bash
while true; do
  # Step 1 .. Step 6
  # Stop ONLY if: operator override (KILL/STOP), max-cycles, regression-breaker,
  # or genuine stagnation (bd ready=0 AND harvested=0 AND failing-goals=0 AND
  # generators dry across 3 passes). Context exhaustion is NOT a stop — it's a
  # session-handoff signal (HANDOFF marker) that the next cron-fire clears.
  CYCLE=$((CYCLE + 1))
done
```

**Stop reasons (soc-5qit, ALL require genuine reason — never just context size):**

1. **KILL/STOP file present** — operator override.
2. **`--max-cycles=N` cap reached**.
3. **Genuine stagnation** — `bd ready=0 AND harvested-unconsumed=0 AND failing-goals=0 AND GENERATOR_EMPTY_STREAK>=2 AND IDLE_STREAK>=2`. Writes DORMANT, which auto-clears in Step 1 the moment `bd create` adds a new ready bead.
4. **Regression breaker after a revert**.

**Context exhaustion is NOT a stop (soc-5qit).** Heavy-context sessions write `.agents/evolve/HANDOFF` (non-sticky), log `result: "context-handoff"` to cycle-history, and exit the turn cleanly. The next cron-fire (compacted/fresh context) clears HANDOFF in Step 1 and resumes. The loop is continuous across compactions; never write DORMANT for context size. See `references/context-budget.md`.

**Mandatory checkpoint #6 — session-PR threshold (NOT terminal, gates next cycle):** at `session_pr_count >= 5` (soc-waxr default), invoke `/post-mortem --deep`, wait for verdict file. PASS → continue. WARN → continue with caveat in next cycle's `notes`. FAIL or non-convergence → write STOP. Agent MUST NOT self-grade or self-write STOP. Full procedure in `references/postmortem-checkpoint.md` (soc-n75z).

**Self-perpetuation modes:** the terminal-native `evolve` loop and the Claude-Code-harness `ScheduleWakeup` end-of-turn pattern are duals — both drive Step 1..Step 7 repeatedly against the same persisted state. See `references/autonomous-execution.md` for the ScheduleWakeup cadence and the rule that hard stops must NOT re-arm.

Push only when productive work has accumulated **and the pawl gate
CONFIRMS**. A direct `git push` to the shared trunk is the **mutate-shared-trunk
pawl** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)) just as much as a
PR merge — accumulation + a green local gate are necessary but **NOT sufficient**.
Where the repo takes PRs, route through `scripts/reconcile-pr.sh` (which enforces
the verdict). Where a direct push is taken, the same CONFIRMED pawl verdict
([`/pre-land-refuters`](../pre-land-refuters/SKILL.md): all refuters CONFIRMED; the
pawl's diversity floor met — **fresh-context by default** (≥1 refuter in a context
other than the author's; model-agnostic) or **multi-model opt-in** (≥2 distinct
families); real reviewer evidence, `head_sha` == the PR's current head) must exist
first — never push on green-alone. **REFUTED → AUTO-REDO** (the loop re-gates, no
human); a human is pulled in **only when a tunable circuit breaker trips** —
max-attempts, time budget, cost/quota, or oscillation — governed by the evolve
circuit breakers (Step 1 /
[`scripts/evolve/halt-check.sh`](../../scripts/evolve/halt-check.sh)); on a breaker
trip the disposition is `ESCALATE`/`HOLD` and the push is held:
```bash
if [ $((PRODUCTIVE_THIS_SESSION % 5)) -eq 0 ] && [ "$PRODUCTIVE_THIS_SESSION" -gt 0 ]; then
  # mutate-shared-trunk pawl: an evidence-bound, commit-current CONFIRMED pawl
  # verdict (fresh-context default; multi-model opt-in) gates the push. --head
  # pins it to the live commit. FAIL-CLOSED if the head can't be resolved: an
  # empty --head means we can't prove the verdict is commit-current, so HOLD
  # rather than push (pawl-verdict.sh check also refuses an empty --head).
  CUR_HEAD="$(gh pr view "$PR" --json headRefOid -q .headRefOid 2>/dev/null || true)"
  if [ -z "$CUR_HEAD" ]; then
    echo "PAWL-HOLD: could not resolve current head — cannot prove the verdict is commit-current; not pushing the shared trunk" >&2
  elif scripts/pawl-verdict.sh check "$BEAD" "$PR" --head "$CUR_HEAD"; then
    git push
  else
    echo "PAWL-HOLD: no CONFIRMED, evidence-bound, commit-current pawl verdict — not pushing the shared trunk" >&2
  fi
fi
```

**Drive to completion (orchestrator-merge model, soc-2drk).** Where the repo requires PRs, a productive cycle does not stop at "PR opened" — the loop drives each bead to *merged*. Ship the bead from its per-bead worktree as a PR (trailers `Closes-scenario` / `Bounded-context` / `Evidence`), wait for CI, and **squash-merge to main once both gates clear** (`gh pr merge <N> --squash --admin`), then `bd close` the bead and remove the worktree. Merge-to-main is the **mutate-shared-trunk pawl** ([docs/contracts/pawls.md](../../docs/contracts/pawls.md)): clearing it requires **green CI AND the pawl gate** — the pawl review ([`/pre-land-refuters`](../pre-land-refuters/SKILL.md)) must CONFIRM. CI alone never authorizes a merge. **Enforced executably**: `scripts/reconcile-pr.sh` calls `scripts/pawl-verdict.sh check <bead> <pr>` before `gh pr merge` and exits **5 (HOLD)** unless a CONFIRMED pawl verdict tied to this bead+PR exists. On red, fix-and-repush or revert; never merge red; on a **REFUTED pawl the loop AUTO-REDOES** autonomously. A human is escalated **only when a tunable circuit breaker trips** (max-attempts / time budget / cost-quota / oscillation), governed by the same evolve breakers (`scripts/evolve/halt-check.sh`). The loop may dispatch sub-agents and drives their PRs to merge too. The operator stays *on* the loop (intent + STOP marker), not *in* it. **Supersedes "operator is the merge gate"** for the autonomous loop — see [ADR-0008](../../docs/adr/ADR-0008-evolve-intelligent-agile-operating-model.md).

**Confirmed-MERGED gate before `bd close` (hard, not advisory).** Re-confirm `gh pr view <N> --json state -q .state` returns `MERGED` *before* `bd close` — never close on a `gh pr merge` exit code, a log line, or a batch `bd --json` query (those flake to null/0). **Close a parent epic ONLY after every child PR is independently confirmed `MERGED`**; re-query per child first, and one non-merged child aborts the epic close. (Caught two premature epic-closes in the 2026-05-31 crank session — this gate is the governance checkpoint, applied here too.) Enforce via the committed `scripts/reconcile-pr.sh <pr> <bead> [--epic <epic>]` + `scripts/check-epic-children-closed.sh <epic>` (hermetic-tested in `tests/scripts/`), not by hand.

### Teardown

Read `references/knowledge-loop-integration.md` for the full teardown learning extraction procedure (commit staged artifacts, run `/post-mortem`, push, report summary).

A teardown `/post-mortem` is a light-touch session-end retrospective. It does NOT substitute for the mandatory threshold checkpoint (`references/postmortem-checkpoint.md`), which is council-gated and edge-triggered at `session_pr_count >= 5`. Never write `.agents/evolve/STOP` as a substitute for the checkpoint's verdict file — STOP without a verdict is the 2026-05-20 anti-pattern (soc-n75z).

**Release-context teardown (MANDATORY when the loop ran on a release-shaped branch):**

When the current branch matches `release/*`, `v*-prep`, `v*-evolve-run`, or `v\d+\.\d+*`, the teardown report MUST NOT recommend `/release` as the next step. Instead, emit the explicit pre-release checklist below — the operator must run these AND confirm green before tagging:

```
## Pre-release checklist — REQUIRED before /release

The autonomous loop has stopped, but release-readiness gates have NOT been run
during cycles. The operator MUST run the following sequence and confirm green
before invoking /release. Do NOT skip any of these on the basis of "cycles
were green" — fast pre-push gate ≠ full pre-push gate; goals-measure ≠
release readiness.

  [ ] 1. Regenerate ALL derived surfaces if any cobra command/flag changed:
         bash scripts/regen-all.sh          # COMMANDS.md, registry.json, maps
         # ADDING an `ao` command also needs the 2 surfaces regen-all only WARNS
         # about: cli/cmd/ao/cobra_commands_test.go expectedCmds (x2 lists) +
         # cli-command-surface counts in
         # evals/agentops-core/fixtures/cli-command-surface-smoke.sh AND
         # evals/agentops-core/cli-command-surface-matrix.json (top/sub/all).
         # Run the smoke fixture for exact counts. Full procedure:
         # [references/ao-command-landing.md](references/ao-command-landing.md)
         git diff cli/docs/COMMANDS.md registry.json   # commit if non-empty

  [ ] 2. Run the FULL pre-push gate (NOT --fast) with fail-fast OFF, so a
         PRE-EXISTING failure (e.g. corpus-freshness) cannot mask your own
         regressions by stopping the run early:
         PRE_PUSH_FAIL_FAST=false bash scripts/pre-push-gate.sh

  [ ] 3. Run the release-readiness gate:
         bash scripts/ci-local-release.sh

  [ ] 4. (Recommended) Smoke /evolve with the new typed read paths if BC port
         wire-ups changed:
         /evolve --quick --max-cycles=1 --dry-run

Only after [1]–[3] pass: /release <version>

If any check fails, fix the issue, re-run all four, then ship.
```

The handoff artifact (e.g., `.agents/runs/<release>/READY-TO-TAG.md`) MUST contain this checklist verbatim, unchecked, when written by the loop. "Ready to tag" means the boxes are checked, not that the loop ran cleanly.

**Rationale:** a v2.41-evolve-run shipped clean code with all tests green and `ao goals measure` 0/30 failing for three cycles — but never ran the full pre-push gate or `ci-local-release.sh`; the removed CLI flag's reference regen was load-bearing. Per-cycle `--fast` is a smoke test, not release readiness. This checklist makes it mechanical.

## Examples

- `/evolve --max-cycles=5` — re-enters the full selection ladder after every `rpi` cycle and runs producer layers instead of idling on empty queues.
- `/evolve --beads-only` — skips goals measurement and works through `bd ready` backlog.
- `/evolve --dry-run` — shows what would be worked on without executing.
- `/evolve --compile` — runs `ao mine` + `ao defrag` at session start to surface fresh signal (orphaned research, code hotspots, oscillating goals) before cycle 1.
- `evolve` — worked overnight flow through beads → harvested → goals → testing → bug hunt → feature suggestion before dormancy.

See `references/examples.md` for detailed walkthroughs.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Loop exits immediately | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` |
| Stagnation after repeated empty passes | Queue layers and producer layers were empty across multiple passes — dormancy is the fallback outcome |
| `ao goals measure` hangs | Use `--timeout 30 --total-timeout 75` or `--beads-only` to skip |
| Regression gate reverts | Review reverted changes, narrow scope, re-run; claimed work items must be released back to available state |

See `references/cycle-history.md` for advanced troubleshooting.

## References

- [references/evolve.feature](references/evolve.feature) — Executable spec: gated cycles, ladder, bounded slice, never-self-halt
- [references/long-loop-discipline.md](references/long-loop-discipline.md) — Disk-is-truth axiom
- [references/artifacts.md](references/artifacts.md) — Generated files registry
- [references/autonomous-execution.md](references/autonomous-execution.md) — Autonomous-loop rules + operator-shape carve-out
- [references/snapshot-pattern-for-long-cycle-gates.md](references/snapshot-pattern-for-long-cycle-gates.md) — Snapshot pattern for long-cycle gates
- [references/compounding.md](references/compounding.md) — Knowledge flywheel and work harvesting
- [references/context-budget.md](references/context-budget.md) — `CONTEXT_BUDGET_EXHAUSTED` as a third stop reason and handoff protocol
- [references/convergence-mechanics.md](references/convergence-mechanics.md) — Read-path mechanisms for compounding
- [references/domain-evolution-bootstrap.md](references/domain-evolution-bootstrap.md) — BDD/DDD/Hexagonal/TDD/XP control surface for skill/domain evolution
- [references/cycle-history.md](references/cycle-history.md) — JSONL format, recovery protocol, kill switch
- [references/examples.md](references/examples.md) — Detailed usage examples
- [references/fitness-scoring.md](references/fitness-scoring.md) — Baseline capture, regression detection, revert procedure
- [references/gate-hygiene.md](references/gate-hygiene.md) — Pre-gate source-surface detection, structural gate-output parsing, pre-push diff-scope check, pre-existing-vs-mine red triage
- [references/new-skill-landing.md](references/new-skill-landing.md) — The six derived surfaces a new/modified skill must regenerate in one shot to stay one-shot-green
- [references/ao-command-landing.md](references/ao-command-landing.md) — The surfaces a new/renamed `ao` command must regenerate (cobra expectedCmds x2 + cli-command-surface counts that regen-all only WARNS about)
- [references/goals-schema.md](references/goals-schema.md) — GOALS.yaml format and continuous metrics
- [references/knowledge-loop-integration.md](references/knowledge-loop-integration.md) — Claim/release semantics and harvest re-read
- [references/mechanical-batches.md](references/mechanical-batches.md) — Script-first vs per-file Edit for > 20-file uniform batches
- [references/metronome-gate.md](references/metronome-gate.md) — Cross-cycle same-mode-repeat blocker
- [references/oscillation.md](references/oscillation.md) — Oscillation detection and quarantine
- [references/pre-flight-schema-check.md](references/pre-flight-schema-check.md) — Cheap field-fit check before architectural migration cycles
- [references/postmortem-checkpoint.md](references/postmortem-checkpoint.md) — Stop reason #6: session-PR post-mortem checkpoint (soc-n75z)
- [references/parallel-execution.md](references/parallel-execution.md) — Parallel /swarm architecture
- [references/quality-mode.md](references/quality-mode.md) — Quality-first mode: scoring, priority cascade, artifacts
- [references/scout-mode.md](references/scout-mode.md) — Scout-mode as a first-class cycle result; scope filter procedure
- [references/teardown.md](references/teardown.md) — Trajectory computation and session summary

## See Also

- `skills/curate/SKILL.md` — the knowledge compounder; `--mode=harvest` gathers artifacts and `--mode=dream` runs the compounding loop overnight
- `skills/rpi/SKILL.md` — Full lifecycle orchestrator (called per cycle)
- `skills/crank/SKILL.md` — Epic execution (called for beads epics)
- `docs/contracts/autodev-program.md` — Repo-local operational contract for bounded autonomous development
- `GOALS.yaml` — Fitness goals for this repo
- [test](../test/SKILL.md) — Test generation and coverage analysis
- [refactor](../refactor/SKILL.md) — Safe, verified refactoring
- [security](../security/SKILL.md) — Dependency audit and vulnerability scanning (absorbs deps)
- [perf](../perf/SKILL.md) — Performance profiling and benchmarking
