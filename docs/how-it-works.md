# How It Works

> Agent output quality is determined by context input quality. AgentOps is the SDLC control plane that keeps that context small, bounded, verifiable, and compounding.

Parallel agents produce noisy output; councils filter it; ratchets lock progress so it can never regress.

AgentOps delivers four product layers: Bookkeeping, the Context Compiler,
Validation Gates, and the Knowledge Flywheel. This page explains the internal
mechanics beneath those layers: the proof gaps they must close, the Brownian
Ratchet, and the flywheel that makes sessions compound.

Think of the mechanics below as the substrate under the operator surface:
briefings and startup context prepare the work order, RPI phases run the
delivery lane, and the flywheel closes the learning loop. See
[Software Factory Surface](software-factory.md).

The same split applies to Dream's in-session surface:

- skills (`/dream`) are the interactive operator surface for a compounding session
- the `ao` CLI primitives (`ao compile`, `ao maturity`, `ao flywheel close-loop`) are the headless work the loop performs

Running that compounding *out of session* — always-on, scheduled, unattended — is delegated to an orchestration substrate (the reference is NTM + MCP + managed-agents), not an AgentOps daemon or scheduler; those surfaces were deleted in 3.0. GitHub nightly remains the public CI proof harness for the report contract; the private always-on compounding engine is the substrate driving the in-session loop.

## The Proof Gaps

AgentOps exists because most agent tooling leaves three gaps open after prompt
construction and routing are solved. The runtime mechanics described on this
page are organized around proving they are actually closed:

1. **Validation gap** (internal label: judgment validation) — agents ship
   without risk context. **Validation Gates** (Layer 2) challenge plans and
   implementations before they land (premortem gate, `/validate`, `/council`,
   task-validation gate).
2. **Bookkeeping gap** (internal label: durable learning) — solved problems
   recur. The **Knowledge Flywheel** (Layer 3) extracts, scores, promotes,
   and retrieves learnings so the same lesson is never re-paid (session-end
   forging, `ao forge`, `ao lookup`, maturity controls).
3. **Closure gap** (internal label: loop closure) — completed work does not
   produce better next work. Postmortems, finding registries, compiled
   constraints, and the flywheel close hook ensure every session leaves the
   environment smarter than it found it. The **Context Compiler** (Layer 1)
   loads these learnings at session start.

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

Run bounded workers, validate each completed slice in fresh context, and route the verdict through Learn. A multi-model council is an optional filter for higher-risk work; Git delivery remains a separate repository decision.

See also: [Brownian Ratchet (deep dive)](brownian-ratchet.md)

## The Stigmergic Spiral in Runtime Terms

The repo now expresses the Stigmergic Spiral as executable mechanics:

- **Spiral macro-cycle:** `Discovery -> Crank -> Validate -> Learn`
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

## Execution Isolation Model

The target model is: **keep lifecycle orchestration visible in the main
session, and isolate expensive execution behind skill contracts.** The main
session should show phase order, retry decisions, and operator intervention
points. Phase and worker contexts should die after they write bounded
artifacts.

| Tier | Skills | Behavior |
|------|--------|----------|
| **Visible orchestration** | evolve, rpi | Stay in main session - operator sees progress and can intervene |
| **Phase isolation** | discovery, crank, validation when called by rpi | Execute the declared phase skill contract in an isolated phase context; return artifact path, verdict, and next action |
| **Worker isolation** | council, codex-team, swarm workers | Fork into subagents or equivalent workers; results merge back via filesystem |

This was learned through production experience: orchestration that disappears
into a fork becomes hard to supervise. The refined direction is visible
orchestration plus isolated execution, not direct agent work replacing skill
contracts.

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

## Hookless — The Operational Layer Enforces Itself

AgentOps 3.0 ships **zero hooks**. Everything a hook used to do is now an
explicit, pulled surface — which means the operating loop works identically on
every harness (Claude Code, Codex, Cursor, OpenCode), and the local pre-push Go
gate (`ao gate check`) is the release authority. CI (`validate.yml`) is a
tag/PR/manual backstop, not the gate on every `main` push.

| Former hook responsibility | Hookless surface | Gap closed |
|----------------------------|------------------|------------|
| Startup maintenance / handoff recovery / factory-state staging | `ao knowledge brief`, `ao context assemble`, `ao session handoff` | Runtime continuity |
| Transcript mining / maturity management / defrag | `/curate --mode=forge`, `ao maturity`, `ao compile` at session close | Durable learning, Loop closure |
| Flywheel close | `ao flywheel close-loop` / `/retro` | Loop closure |
| Prompt guidance / context pressure | `ao lookup`, factory briefings (pulled, not injected) | Judgment validation |
| Validation gates / quality / completion | local pre-push Go gate (`ao gate check --fast` via the pre-push hook) + skill-level checks + `cd cli && make test`; CI (`.github/workflows/validate.yml`) as a tag/PR/manual backstop | Judgment validation, Loop closure |

Operators who *want* runtime hooks can author their own with the
`hooks-authoring` skill; they are opt-in and not part of the default product.

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

`/rpi "goal"` runs the seven-move loop inside the current session when context fits. For phase-level resume control or unattended operation, dispatch the whole loop through the out-of-session substrate (NTM + MCP Agent Mail + managed-agents), not an AgentOps daemon.

## Parallel RPI — N Epics in Isolated Worktrees

`/crank` runs multiple epics concurrently, each in its own git worktree. Every epic gets the same four-umbrella lifecycle (Discovery → Crank → Validate → Learn) with zero cross-contamination. Delivery happens afterward under repository policy.

Use `/crank` to split independent epics into coordinated lanes with explicit ownership and merge order.

```
                   /crank
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

## Dream — Compounding Run, In Session and Out

Dream is the compounding expression of the same loop model:

- **interactive surface:** `/dream` for a compounding session, run in the foreground against the real `.agents` corpus
- **work the loop performs:** `ao compile`, `ao maturity --scan`, `ao flywheel close-loop` — the CLI primitives the session drives
- **report contract:** `summary.json` and `summary.md` plus DreamScape terrain rendering

A foreground `/dream` session keeps runtime behavior honest: no tracked source-code edits by default, optional bounded Dream Council synthesis through independent runner reports, and a shared report contract.

Running Dream **unattended** — always-on, scheduled, queue-driven — is out-of-session orchestration, which AgentOps delegates to a substrate (the reference is NTM + MCP + managed-agents) rather than shipping its own daemon or scheduler. The pattern is a long-lived agent — an NTM swarm pane or a managed-agent driver — running `/dream`-equivalent maintenance (`ao compile`, `ao maturity --scan`) on a cron schedule. There are no fake scheduler guarantees on a sleeping laptop; the substrate owns when and where.

GitHub nightly remains useful for a different job: it proves that Dream's report contract and flywheel primitives still work in CI. It does not replace a substrate-driven unattended run.

## See Also

- [Context Lifecycle Contract](context-lifecycle.md) — The three gaps this runtime is built to close
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Glossary](GLOSSARY.md) — Definitions of key terms and metaphors
