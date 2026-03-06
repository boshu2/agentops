---
name: rpi
description: 'Full RPI lifecycle orchestrator. Discovery (research+plan+pre-mortem) → Implementation (crank) → Validation (vibe+post-mortem). One command, sequential skill invocations with retry gates and fresh phase contexts.'
---


# $rpi — Full RPI Lifecycle Orchestrator

> **Quick Ref:** One command, full lifecycle. Discovery → Implementation → Validation. The session is the lead; sub-skills manage their own teams.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Runtime Rule (Native Orchestration Only)

- `$rpi` MUST orchestrate `$research`, `$plan`, `$pre-mortem`, `$crank`, `$vibe`, and `$post-mortem` directly in-session.
- Do not hand orchestration to external RPI wrapper commands.

## Quick Start

```bash
$rpi "add user authentication"                        # full lifecycle
$rpi --interactive "add user authentication"          # human gates in discovery only
$rpi --from=discovery "add auth"                      # resume discovery
$rpi --from=implementation ag-23k                      # skip to crank with existing epic
$rpi --from=validation                                 # run vibe + post-mortem only
$rpi --loop --max-cycles=3 "add auth"                 # optional iterate-on-fail loop
$rpi "add auth"                                       # strict-quality/test-first is ON by default
$rpi --no-test-first "add auth"                       # explicit opt-out from test-first
```

## Architecture

```
$rpi <goal | epic-id> [--from=<phase>] [--interactive]
  │ (session = lead, no team-create, direct skill chaining only)
  │
  ├── Phase 1: Discovery
  │   ├── $research
  │   ├── $plan
  │   └── $pre-mortem (gate)
  │
  ├── Phase 2: Implementation
  │   └── $crank (autonomous execution)
  │
  └── Phase 3: Validation
      ├── $vibe (gate)
      └── $post-mortem (retro + flywheel)
```

**Human gates (default):** 0 — fully autonomous.
**Human gates (`--interactive`):** discovery approvals in `$research` and `$plan`.
**Retry gates:** pre-mortem FAIL → re-plan, implementation BLOCKED/PARTIAL → re-crank, vibe FAIL → re-crank (max 3 attempts each).
**Optional loop (`--loop`):** post-mortem FAIL can spawn another RPI cycle.

### Phase Data Contracts

All phase transitions use filesystem-based artifacts (no in-memory coupling):

| Transition | Key Artifacts | How Next Phase Reads Them |
|------------|---------------|---------------------------|
| Start -> Discovery | Goal string | Passed as argument to `$research` |
| Discovery -> Implementation | Epic ID, pre-mortem verdict, phase-1 summary | `phased-state.json` + `.agents/rpi/phase-1-summary-*.md` |
| Implementation -> Validation | Crank completion status, phase-2 summary | `bd children <epic-id>` + `.agents/rpi/phase-2-summary-*.md` |
| Validation -> Next Cycle (optional) | Vibe/post-mortem verdicts, harvested follow-up work | Council reports + `.agents/rpi/next-work.jsonl` |

## Execution Steps

Given `$rpi <goal | epic-id> [--from=<phase>] [--interactive]`:

### Step 0: Setup

```bash
mkdir -p .agents/rpi
```

Enforce orchestration mode before selecting a phase:
- Allowed: direct invocations of `$research`, `$plan`, `$pre-mortem`, `$crank`, `$vibe`, `$post-mortem`.
- Disallowed: external RPI wrapper orchestration.

Determine starting phase:
- default: `discovery`
- `--from=implementation` (alias: `crank`)
- `--from=validation` (aliases: `vibe`, `post-mortem`)
- aliases `research`, `plan`, and `pre-mortem` map to `discovery`

If input looks like an epic ID (`ag-*`) and `--from` is not set, start at implementation.

Initialize state:

```
rpi_state = {
  goal: "<goal string>",
  epic_id: null,
  phase: "<discovery|implementation|validation>",
  auto: <true unless --interactive>,
  test_first: <true by default; false only when --no-test-first>,
  complexity: null,
  cycle: 1,
  parent_epic: null,
  verdicts: {}
}
```

### Phase 1: Discovery

Discovery invokes research, planning, and pre-mortem sequentially. Each skill forks into its own subagent context via `context: { window: fork }` and communicates via filesystem artifacts:

```text
$research <goal> [--auto]
$plan <goal> [--auto]
$pre-mortem
```

After discovery completes:
1. Extract epic ID from `bd list --type epic --status open` and store in `rpi_state.epic_id`.
2. Extract pre-mortem verdict (PASS/WARN/FAIL) from latest pre-mortem council report.
3. Store verdict in `rpi_state.verdicts.pre_mortem`.
4. Write summary to `.agents/rpi/phase-1-summary-YYYY-MM-DD-<goal-slug>.md`.
5. Record ratchet and telemetry:

```bash
ao work ratchet record research 2>/dev/null || true
bash scripts/checkpoint-commit.sh rpi "phase-1" "discovery complete" 2>/dev/null || true
bash scripts/log-telemetry.sh rpi phase-complete phase=1 phase_name=discovery 2>/dev/null || true
```

Gate behavior:
- PASS/WARN: proceed to implementation.
- FAIL: re-run `$plan` with findings context (top 5 structured findings: description, fix, ref), then `$pre-mortem` again. Max 3 total attempts. If still FAIL after 3, stop and require manual intervention.

### Phase 2: Implementation

Requires `rpi_state.epic_id`.

```text
$crank <epic-id> [--no-test-first]
```

After implementation completes:
1. Check completion via crank output / epic child statuses.
2. Gate result:
   - DONE: proceed to validation
   - BLOCKED: re-run `$crank` with block context (max 3 total attempts). If still BLOCKED, stop and require manual intervention.
   - PARTIAL: re-run `$crank` with epic-id; it picks up unclosed issues (max 3 total attempts). If still PARTIAL, stop and require manual intervention.
3. Write summary to `.agents/rpi/phase-2-summary-YYYY-MM-DD-<goal-slug>.md`.
4. Record ratchet and telemetry:

```bash
ao work ratchet record implement 2>/dev/null || true
bash scripts/checkpoint-commit.sh rpi "phase-2" "implementation complete" 2>/dev/null || true
bash scripts/log-telemetry.sh rpi phase-complete phase=2 phase_name=implementation 2>/dev/null || true
```

### Phase 3: Validation

Validation runs final review and lifecycle close-out:

```text
$vibe recent            # use --quick recent for low/medium complexity
$post-mortem <epic-id>  # use --quick for low/medium complexity
```

After validation completes:
1. Extract vibe verdict and store `rpi_state.verdicts.vibe`.
2. If present, extract post-mortem verdict and store `rpi_state.verdicts.post_mortem`.
3. Gate result:
   - PASS/WARN: finish RPI
   - FAIL: re-run implementation with findings, then re-run validation (max 3 total attempts)
4. Write summary to `.agents/rpi/phase-3-summary-YYYY-MM-DD-<goal-slug>.md`.
5. Record ratchet and telemetry:

```bash
ao work ratchet record vibe 2>/dev/null || true
bash scripts/checkpoint-commit.sh rpi "phase-3" "validation complete" 2>/dev/null || true
bash scripts/log-telemetry.sh rpi phase-complete phase=3 phase_name=validation 2>/dev/null || true
```

**Optional loop (`--loop`):** If post-mortem verdict is FAIL and `--loop` is enabled, extract 3 concrete fixes from the report and re-invoke `$rpi` from Phase 1 with a tightened goal (up to `--max-cycles` total cycles). PASS/WARN stops the loop.

**Optional spawn-next (`--spawn-next`):** After a PASS/WARN finish, read `.agents/rpi/next-work.jsonl` for harvested follow-up items. Report them to the user with a suggested next `$rpi` command but do NOT auto-invoke.

### Step Final: Report

Read `references/report-template.md` for the final output format and next-work handoff pattern.

Read `references/error-handling.md` for failure semantics and retries.

## Complexity-Aware Ceremony

RPI automatically classifies each goal's complexity at startup and adjusts the ceremony accordingly. This prevents trivial tasks from paying the full validation overhead of a refactor.

### Classification Levels

| Level | Criteria | Behavior |
|-------|----------|----------|
| `fast` | Goal ≤30 chars, no complex/scope keywords | Skips Phase 3 (validation). Runs discovery → implementation only. |
| `standard` | Goal 31–120 chars, or 1 scope keyword | Full 3-phase lifecycle. Gates use `--quick` shortcuts. |
| `full` | Complex-operation keyword (refactor, migrate, rewrite, …), 2+ scope keywords, or >120 chars | Full 3-phase lifecycle. Gates use full council (no shortcuts). |

### Keyword Signals

**Complex-operation keywords** (trigger `full`): `refactor`, `migrate`, `migration`, `rewrite`, `redesign`, `rearchitect`, `overhaul`, `restructure`, `reorganize`, `decouple`, `deprecate`, `split`, `extract module`, `port`

**Scope keywords** (1 → `standard`; 2+ → `full`): `all`, `entire`, `across`, `everywhere`, `every file`, `every module`, `system-wide`, `global`, `throughout`, `codebase`

All keywords are matched as **whole words** to prevent false positives (e.g. "support" does not match "port").

### Logged Output

At RPI start you will see:

```
RPI mode: rpi-phased (complexity: fast)
Complexity: fast — skipping validation phase (phase 3)
```

or for standard/full:

```
RPI mode: rpi-phased (complexity: standard)
```

The complexity level is persisted in `.agents/rpi/phased-state.json` as the `complexity` field.

### Legacy Gate Mapping (Compatibility)

Phase naming in older tuning validation references is mapped as follows:

#### Phase 3: Pre-mortem
- complexity == "low": inline (`--quick`), no spawning
- complexity == "medium": inline fast default (`--quick`)
- complexity == "high": full council path (2-judge minimum), no `--quick`

#### Phase 5: Final Vibe
- complexity == "low": inline (`--quick`), no spawning
- complexity == "medium": inline fast default (`--quick`)
- complexity == "high": full council path (2-judge minimum), no `--quick`

#### Phase 6: Post-mortem
- complexity == "low": inline (`--quick`), no spawning
- complexity == "medium": inline fast default (`--quick`)
- complexity == "high": full council path (2-judge minimum), no `--quick`

### Override

- `--fast-path`: force fast-path regardless of classification (useful for quick patches).
- `--deep`: force full-ceremony regardless of classification (useful for sensitive changes).

## Phase Budgets

Each RPI phase has a time budget scaled by complexity level. Budgets prevent sessions from stalling in research or planning without producing artifacts.

| Phase | `fast` | `standard` | `full` |
|-------|--------|------------|--------|
| Research | 3 min | 5 min | 10 min |
| Plan | 2 min | 5 min | 10 min |
| Pre-mortem | 1 min | 3 min | 5 min |
| Implementation | unlimited | unlimited | unlimited |
| Validation | — (skipped) | 5 min | 10 min |

**Implementation is always unlimited** — crank has its own wave limits (MAX_EPIC_WAVES = 50).

### Budget Enforcement

- **Check at natural pause points:** before spawning agents, before retry loops, between skill invocations
- **On budget expiry:**
  1. Allow in-flight tool calls to complete (soft limit — don't interrupt mid-operation)
  2. Write `[TIME-BOXED]` marker to the phase summary file
  3. Auto-transition to the next phase with whatever artifacts exist
  4. Log: `"Phase <name> time-boxed at <elapsed>s (budget: <budget>s)"`
- **Budget expiry is NOT a retry attempt** — it is orthogonal to the 3-attempt retry gates. A time-boxed phase that produced a FAIL verdict still counts as attempt 1.
- **Override with `--no-budget`** to disable all phase budgets (useful for open-ended research)
- **Override with `--budget=<phase>:<seconds>`** for custom per-phase budgets (e.g., `--budget=research:180,plan:120`)

For detailed budget tables, worked examples, and rationale, read `references/phase-budgets.md`.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--from=<phase>` | `discovery` | Start from `discovery`, `implementation`, or `validation` (aliases accepted) |
| `--interactive` | off | Enable human gates in discovery (`$research`, `$plan`) |
| `--auto` | on | Legacy flag; autonomous is default |
| `--loop` | off | Enable post-mortem FAIL loop into next cycle |
| `--max-cycles=<n>` | `1` | Max cycle count when `--loop` is enabled |
| `--spawn-next` | off | Surface harvested follow-up work after post-mortem |
| `--test-first` | on | Strict-quality default: pass `--test-first` to `$crank` |
| `--no-test-first` | off | Explicit opt-out: do not pass `--test-first` to `$crank` |
| `--fast-path` | auto | Force low-complexity gate mode (`--quick`) |
| `--deep` | auto | Force high-complexity gate mode (full council) |
| `--dry-run` | off | Report actions without mutating next-work queue |
| `--no-budget` | off | Disable all phase time budgets |
| `--budget=<spec>` | auto | Custom per-phase budgets in seconds (e.g., `research:180,plan:120`) |

## Examples

### Full Lifecycle

**User says:** `$rpi "add user authentication"`

**What happens:**
1. Discovery runs `$research`, `$plan`, `$pre-mortem` and produces epic `ag-5k2`.
2. Implementation runs `$crank ag-5k2` until children are complete.
3. Validation runs `$vibe` then `$post-mortem`, extracts learnings, and suggests next work.

### Resume from Implementation

**User says:** `$rpi --from=implementation ag-5k2`

**What happens:**
1. Skips discovery.
2. Runs `$crank ag-5k2`.
3. Runs validation (`$vibe` + `$post-mortem`).

### Interactive Discovery

**User says:** `$rpi --interactive "refactor payment module"`

**What happens:**
1. Discovery runs with human gates in `$research` and `$plan`.
2. Implementation and validation remain autonomous.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Supervisor spiraled branch count | Detached HEAD healing or legacy `codex/auto-rpi-*` naming created detached branches | Keep `--detached-heal` off for supervisor mode (default), prefer detached worktree execution, then use external operator cleanup controls to preview and prune stale branches/worktrees. |
| Discovery retries hit max attempts | Plan has unresolved risks | Review pre-mortem findings, re-run `$rpi --from=discovery` |
| Implementation retries hit max attempts | Epic has blockers or unresolved dependencies | Inspect `bd show <epic-id>`, fix blockers, re-run `$rpi --from=implementation` |
| Validation retries hit max attempts | Vibe found critical defects repeatedly | Apply findings, re-run `$rpi --from=validation` |
| Missing epic ID at implementation start | Discovery did not produce a parseable epic | Verify latest open epic with `bd list --type epic --status open` |
| Large-repo context pressure | Too much context in one window | Use `references/context-windowing.md` and summarize phase outputs aggressively |

### Emergency control (external operator loop)

Use external operator controls from a terminal to cancel in-flight runs and
clean stale worktrees/branches. Keep these controls out-of-band from `$rpi`
skill chaining.

## See Also

- `skills/research/SKILL.md` — discovery exploration
- `skills/plan/SKILL.md` — discovery decomposition
- `skills/pre-mortem/SKILL.md` — discovery risk gate
- `skills/crank/SKILL.md` — implementation execution
- `skills/vibe/SKILL.md` — validation gate
- `skills/post-mortem/SKILL.md` — validation close-out

## Reference Documents

- [references/complexity-scaling.md](references/complexity-scaling.md)
- [references/context-windowing.md](references/context-windowing.md)
- [references/gate-retry-logic.md](references/gate-retry-logic.md)
- [references/gate4-loop-and-spawn.md](references/gate4-loop-and-spawn.md)
- [references/phase-budgets.md](references/phase-budgets.md)
- [references/phase-data-contracts.md](references/phase-data-contracts.md)

## Local Resources

### references/

- [references/complexity-scaling.md](references/complexity-scaling.md)
- [references/context-windowing.md](references/context-windowing.md)
- [references/error-handling.md](references/error-handling.md)
- [references/gate-retry-logic.md](references/gate-retry-logic.md)
- [references/gate4-loop-and-spawn.md](references/gate4-loop-and-spawn.md)
- [references/phase-budgets.md](references/phase-budgets.md)
- [references/phase-data-contracts.md](references/phase-data-contracts.md)
- [references/report-template.md](references/report-template.md)

### scripts/

- `scripts/validate.sh`


