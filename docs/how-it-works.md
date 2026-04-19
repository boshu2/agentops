# How It Works

> Agent output quality is determined by context input quality. Every pattern below — fresh context per worker, ratcheted progress, least-privilege loading — exists to ensure the right information is in the right window at the right time.

Parallel agents produce noisy output; councils filter it; ratchets lock progress so it can never regress.

Publicly, AgentOps sells bookkeeping, validation, primitives, and flows. This
page explains the internal mechanics beneath that story: the proof gaps those
promises must close, the Brownian Ratchet, and the flywheel that makes sessions
compound.

Think of the mechanics below as the substrate under the operator surface:
briefings and startup context prepare the work order, RPI phases run the
delivery lane, and the flywheel closes the learning loop. See
[Software Factory Surface](software-factory.md).

The same split now applies to Dream:

- skills remain the interactive operator surface
- the `ao` binary is the headless automation surface
- shared config is the control plane

That matters most for overnight work. GitHub nightly is the public proof harness. `ao overnight` is the private local compounding engine.

## The Proof Gaps

AgentOps exists because most agent tooling leaves three gaps open after prompt
construction and routing are solved. The runtime mechanics described on this
page are organized around proving they are actually closed:

1. **Validation gap** (internal label: judgment validation) — agents ship
   without risk context. Hooks and skills challenge plans and implementations
   before they land (pre-mortem gate, `/vibe`, `/council`, task-validation
   gate).
2. **Bookkeeping gap** (internal label: durable learning) — solved problems
   recur. The knowledge flywheel extracts, scores, promotes, and retrieves
   learnings so the same lesson is never re-paid (session-end forging,
   `ao forge`, `ao lookup`, maturity controls).
3. **Closure gap** (internal label: loop closure) — completed work does not
   produce better next work. Post-mortems, finding registries, compiled
   constraints, and the flywheel close hook ensure every session leaves the
   environment smarter than it found it.

The canonical contract is in [Context Lifecycle Contract](context-lifecycle.md). The sections below show how each runtime mechanism maps to one or more of these gaps.

## The Brownian Ratchet

*A mechanism borrowed from molecular physics: random motion is captured by one-way gates, converting chaos into forward progress.*

Chaos in, locked progress out.

```
  ╭─ agent-1 ─→ ✓ ─╮
  ├─ agent-2 ─→ ✗ ─┤   3 attempts, 1 fails
  ├─ agent-3 ─→ ✓ ─┤   council catches it
  ╰─ council ──→ PASS   ratchet locks the result
                  ↓
          can't go backward
```

Spawn parallel agents (chaos), validate with multi-model council (filter), merge to main (ratchet). Failed agents are cheap — fresh context means no contamination.

See also: [Brownian Ratchet (deep dive)](brownian-ratchet.md)

## The Stigmergic Spiral in Runtime Terms

The repo now expresses the Stigmergic Spiral as executable mechanics:

- **Spiral macro-cycle:** `Discovery -> Implementation -> Validation`
- **OODA micro-cycles:** each wave repeatedly observes state, orients with repo artifacts, decides a bounded move, and acts
- **Stigmergic memory:** `.agents/`, finding registries, contracts, handoffs, and commits carry state forward
- **Degraded operation:** fresh workers, disk-backed recovery, and hook-enforced checkpoints assume context loss and tool drift are normal

The important shift is where intelligence lives. Agent sessions are disposable. The environment compounds. See [The Knowledge Flywheel](knowledge-flywheel.md) for the full 6-stage pipeline that makes this happen automatically.

## Ralph Wiggum Pattern — Fresh Context Every Wave

*Named after Ralph Wiggum's "I'm helping!" -- each worker starts fresh with no memory of previous workers, ensuring complete isolation between waves.*

```
  Wave 1:  spawn 3 workers → write files → lead validates → lead commits
  Wave 2:  spawn 2 workers → ...same pattern, zero accumulated context
```

Every wave gets a fresh worker set. Every worker gets clean context. No bleed-through between waves. The lead is the only one who commits.

Supports both Codex sub-agents (`spawn_agent`) and Claude agent teams (`TeamCreate`).

Operational contract reference: `skills/shared/references/ralph-loop-contract.md` (reverse-engineered from `ghuntley/how-to-ralph-wiggum` and mapped to AgentOps primitives).

## Two-Tier Execution Model

The target model is: **keep orchestration visible in the main session, and let spawned workers carry the isolated context.** Most current meta-skills follow that shape, but a few SKILL contracts still declare `context.window: fork` while the runtime behavior has shifted toward visible orchestration. When docs and contracts disagree, treat the live `SKILL.md` as authoritative.

| Tier | Skills | Behavior |
|------|--------|----------|
| **NO-FORK** (orchestrators) | evolve, rpi, crank, vibe, post-mortem, pre-mortem | Stay in main session — operator sees progress and can intervene |
| **FORK** (worker spawners) | council, codex-team | Fork into subagents — results merge back via filesystem |

This was learned through production experience: orchestration that disappears into a fork becomes hard to supervise. The long-term direction is to keep macro progress visible and isolate only the worker layer, but the repo still contains a small amount of contract drift that has not been fully normalized.

`/swarm` is a special case — it's an orchestrator (no fork) that spawns runtime workers via `TeamCreate`/`spawn_agent`. The workers are runtime sub-agents, not SKILL.md skills.

Full classification: [`SKILL-TIERS.md`](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md)

## Agent Backends — Runtime-Native Orchestration

Skills auto-select the best available backend:

1. Runtime-native backend first:
   Claude sessions → Claude native teams (`TeamCreate` + `SendMessage`)
   Codex sessions → Codex sub-agents (`spawn_agent`)
2. Secondary/mixed backend only when explicitly requested
3. Background task fallback (`Task(run_in_background=true)`)

```
  Council:                               Swarm:
  ╭─ judge-1 ──╮                  ╭─ worker-1 ──╮
  ├─ judge-2 ──┼→ consolidate     ├─ worker-2 ──┼→ validate + commit
  ╰─ judge-3 ──╯                  ╰─ worker-3 ──╯
```

**Claude teams setup** (optional):
```json
// ~/.claude/settings.json
{
  "teammateMode": "tmux",
  "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" }
}
```

## Hooks — The Operational Layer Enforces Itself

The active runtime manifest currently declares **7 hook event sections** in `hooks/hooks.json`. All have a kill switch: `AGENTOPS_HOOKS_DISABLED=1`.

### Lifecycle anchors

| Hook surface | Trigger | What it does | Gap closed |
|--------------|---------|--------------|------------|
| Session start | `SessionStart` | Runs `session-start.sh` — startup maintenance, handoff recovery, and silent factory-state staging | Runtime continuity |
| Session end maintenance | `SessionEnd` | Runs `session-end-maintenance.sh` (transcript mining, maturity management) and `compile-session-defrag.sh` (knowledge deduplication and defrag) | Durable learning (extraction), Loop closure |
| Flywheel close | `Stop` | Runs `ao-flywheel-close.sh` — closes the feedback loop via `ao flywheel close-loop` | Loop closure |

### Guardrails and continuity surfaces

| Hook surface | Trigger | What it does | Gap closed |
|--------------|---------|--------------|------------|
| Prompt guidance | `UserPromptSubmit` | Runs `factory-router.sh` (captures first-goal intake when startup had none), `new-user-welcome.sh` (one-time fresh-repo onboarding), `prompt-nudge.sh` (ratchet nudges), and `intent-echo.sh` (confirms high-stakes intent) without adding startup briefings to the conversation | Judgment validation |
| Pre-tool gates | `PreToolUse` | `pre-mortem-gate.sh` (blocks `/crank` without plan review), `commit-review-gate.sh` (pre-commit checks), `go-test-precommit.sh`, `git-worker-guard.sh` (worker isolation), `edit-knowledge-surface.sh`, `codex-parity-warn.sh` | Judgment validation |
| Post-tool checks | `PostToolUse` | `write-time-quality.sh` (edit quality), `go-complexity-precommit.sh`, `go-vet-post-edit.sh`, `research-loop-detector.sh` (detects stalled loops), `context-monitor.sh` | Judgment validation, Loop closure |
| Task completion gate | `TaskCompleted` | Runs `task-validation-gate.sh` — executes compiled constraints from `.agents/constraints/index.json` before accepting task completion | Judgment validation, Loop closure |

All hooks use `lib/hook-helpers.sh` for structured error recovery — failures include suggested next actions and auto-handoff context.

## Compaction Resilience — Long Runs That Don't Lose State

LLM context compaction can destroy loop state mid-run. Any skill that runs for hours (especially `/evolve`) must store state on disk, not in LLM memory.

The pattern:
1. **Write state to disk after every step** — `cycle-history.jsonl`, fitness snapshots, heartbeat
2. **Recover from disk on every resume** — read last cycle number from JSONL, not from conversation context
3. **Verify writes succeeded** — read back the entry, compare, stop if mismatch

Hard gates in `/evolve`:
- Pre-cycle: fitness snapshot must exist and be valid JSON before the regression gate runs
- Post-cycle: cycle-history.jsonl write is verified; failure = stop
- Loop entry: continuity check confirms cycle N was logged before starting N+1

This was validated in production: 116 evolve cycles ran ~7 hours overnight. The first run revealed that without disk-based recovery, context compaction silently broke tracking after cycle 1 — the agent continued producing valuable work but without formal regression gating. The hardening above prevents this class of failure.

## Context Windowing — Bounded Execution for Large Codebases

For repos over ~1500 files, `/rpi` uses deterministic shards to keep each worker's context window bounded. Run `scripts/rpi/context-window-contract.sh` before `/rpi` to enable sharding. This prevents context overflow and keeps worker quality consistent regardless of codebase size.

## Phased RPI — Fresh Context Per Phase

`ao rpi phased "goal"` runs each phase in its own session — no context bleed between phases. Use `/rpi` when context fits in one session. Use `ao rpi phased` when you need phase-level resume control. For autonomous control-plane operation, use the canonical path `ao rpi loop --supervisor`.

## Parallel RPI — N Epics in Isolated Worktrees

`ao rpi parallel` runs multiple epics concurrently, each in its own git worktree. Every epic gets a full 3-phase lifecycle (discovery → implementation → validation) with zero cross-contamination, then merges back sequentially.

```
ao rpi parallel --manifest epics.json        # Named epics with merge order
ao rpi parallel "add auth" "add logging"     # Inline goals (auto-named)
ao rpi parallel --no-merge --manifest m.json # Leave worktrees for manual review
```

```
                   ao rpi parallel
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
   ┌───────────┐   ┌───────────┐   ┌───────────┐
   │ worktree  │   │ worktree  │   │ worktree  │
   │  epic/A   │   │  epic/B   │   │  epic/C   │
   ├───────────┤   ├───────────┤   ├───────────┤
   │ 1 discover│   │ 1 discover│   │ 1 discover│
   │ 2 build   │   │ 2 build   │   │ 2 build   │
   │ 3 validate│   │ 3 validate│   │ 3 validate│
   └─────┬─────┘   └─────┬─────┘   └─────┬─────┘
         └───────────────┼───────────────┘
                         ▼
            merge  A → B → C  (in order)
                         │
                   gate script (CI)
```

Each phase spawns a fresh session — no context bleed. Worktree isolation means parallel epics can touch the same files without conflicts. The merge order is configurable (manifest `merge_order` or `--merge-order` flag) so dependency-heavy epics land first.

## Dream — Private Overnight Operator Mode

Dream is the overnight expression of the same control-plane model:

- **interactive surface:** `$dream` for setup, bedtime runs, and morning reports
- **automation surface:** `ao overnight setup|start|run|report`
- **control plane:** shared `dream.*` config plus explicit output artifacts

The first shipped wave is intentionally bounded. `ao overnight` runs locally against the real `.agents` corpus, writes `summary.json` and `summary.md`, and keeps runtime behavior honest:

- no fake scheduler guarantees on sleeping laptops
- no tracked source-code edits by default
- optional bounded Dream Council synthesis through independent runner reports
- DreamScape terrain rendering inside the shared report contract

GitHub nightly remains useful, but for a different job: it proves that Dream's report contract and flywheel primitives still work in CI. It does not replace a private local bedtime run.

## See Also

- [Context Lifecycle Contract](context-lifecycle.md) — The three gaps this runtime is built to close
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Glossary](GLOSSARY.md) — Definitions of key terms and metaphors
