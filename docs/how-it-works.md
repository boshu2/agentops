# How It Works

> Agent output quality is determined by context input quality. Every pattern below — fresh context per worker, ratcheted progress, least-privilege loading — exists to ensure the right information is in the right window at the right time.

Parallel agents produce noisy output; councils filter it; ratchets lock progress so it can never regress.

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

## Ralph Wiggum Pattern — Fresh Context Every Wave

*Named after Ralph Wiggum's "I'm helping!" -- each worker starts fresh with no memory of previous workers, ensuring complete isolation between waves.*

```
  Wave 1:  spawn 3 workers → write files → lead validates → lead commits
  Wave 2:  spawn 2 workers → ...same pattern, zero accumulated context
```

Every wave gets a fresh worker set. Every worker gets clean context. No bleed-through between waves. The lead is the only one who commits.

Supports both Codex sub-agents (`spawn_agent`) and Claude agent teams (`TeamCreate`).

Operational contract reference: `skills/shared/references/ralph-loop-contract.md` (reverse-engineered from `ghuntley/how-to-ralph-wiggum` and mapped to AgentOps primitives).

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

## Hooks — The Workflow Enforces Itself

12 hooks. All have a kill switch: `AGENTOPS_HOOKS_DISABLED=1`.

| Hook | Trigger | What it does |
|------|---------|-------------|
| Push gate | `git push` | Blocks push if `/vibe` hasn't passed |
| Pre-mortem gate | `/crank` invocation | Blocks `/crank` if `/pre-mortem` hasn't passed |
| Worker guard | `git commit` | Blocks workers from committing (lead-only) |
| Dangerous git guard | `force-push`, `reset --hard` | Blocks destructive git commands |
| Standards injector | Write/Edit | Auto-injects language-specific coding rules |
| Ratchet nudge | Any prompt | "Run /vibe before pushing" |
| Task validation | Task completed | Validates metadata before accepting |
| Session start | Session start | Knowledge injection, stale state cleanup |
| Ratchet advance | After Bash | Locks progress gates |
| Stop team guard | Session stop | Prevents premature stop with active teams |
| Precompact snapshot | Before compaction | Saves state before context compaction |
| Pending cleaner | Session start | Cleans stale pending state |

All hooks use `lib/hook-helpers.sh` for structured error recovery — failures include suggested next actions and auto-handoff context.

## Context Windowing — Bounded Execution for Large Codebases

For repos over ~1500 files, `/rpi` uses deterministic shards to keep each worker's context window bounded. Run `scripts/rpi/context-window-contract.sh` before `/rpi` to enable sharding. This prevents context overflow and keeps worker quality consistent regardless of codebase size.

## See Also

- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Glossary](GLOSSARY.md) — Definitions of key terms and metaphors
