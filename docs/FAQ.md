# FAQ

## Why not just use my coding agent directly?

Without AgentOps, every session starts from scratch. Your agent doesn't remember what failed last time, doesn't validate its plan before coding, doesn't check its code with a second opinion, and doesn't capture what it learned. You fill those gaps manually — re-explaining context, reviewing code, tracking what changed. With AgentOps, the system handles context, validation, and memory. You manage the roadmap.

## How does this compare to other approaches?

| Approach | What it does well | What AgentOps adds |
|----------|------------------|--------------------|
| **Direct agent use** (Claude Code, Cursor, Copilot) | Full autonomy, simple to start | Multi-model councils, fresh-context waves, and knowledge that compounds across sessions. A bare agent starts fresh each session; ours extracts learnings and applies them next time. |
| **Custom prompts** (.cursorrules, CLAUDE.md) | Flexible, version-controlled | Static instructions don't compound. The flywheel auto-extracts learnings and injects them back. `/post-mortem` proposes changes to the tools themselves. |
| **Agent orchestrators** (CrewAI, AutoGen, LangGraph) | Multi-language task scheduling | Those choreograph sequential tasks; we compose parallel waves with validation at every stage. No external state backend — all learnings are git-tracked. |
| **CI/CD gates** (GitHub Actions, pre-commit) | Automated, industry standard | Gates run after code is written. Ours run before coding (`/pre-mortem`) and before push (`/vibe`). Failures retry with context, not human escalation. |

## What data leaves my machine?

AgentOps itself stores nothing externally — all state lives in `.agents/` (local, git-ignored by default). No telemetry, no cloud, no external services. Set `AGENTOPS_GITIGNORE_AUTO=0` to commit knowledge artifacts to your repo. Your coding agent's normal API traffic to its LLM provider still applies.

## Can I use this with other AI coding tools?

Yes — Claude Code, Codex CLI, Cursor, OpenCode, anything supporting [Skills](https://skills.sh). The `--mixed` council mode adds Codex judges alongside Claude. Knowledge artifacts are plain markdown.

## What does PRODUCT.md do?

Run `/product` to generate a `PRODUCT.md` describing your mission, personas, and competitive landscape. Once it exists, `/pre-mortem` automatically adds product perspectives (user-value, adoption-barriers) and `/vibe` adds developer-experience perspectives (api-clarity, error-experience) to their council reviews. Your agent understands what matters to your product — not just whether the code compiles.

## What are the current limitations?

- **Single primary author so far.** The system works but hasn't been stress-tested across diverse codebases and team sizes. Looking for early adopters willing to break things.
- **Quality pool can over-promote.** Context-specific patterns sometimes get promoted as general knowledge. Freshness decay helps but doesn't fully solve stale injection.
- **Retry loops cap at 3.** If a council or crank wave fails three times, the system surfaces the failure to you rather than looping forever. This is intentional but means some edge cases need human judgment.
- **Knowledge curation is imperfect.** Freshness decay prevents the worst staleness, but the scoring heuristics (specificity, actionability, novelty) are tuned for one author's workflow. Your mileage may vary.

## How does AgentOps handle subagent nesting?

Claude Code doesn't allow subagents to spawn their own subagents — nesting depth is capped at one level. AgentOps works around this three ways:

- **Teams as flat peers** — `TeamCreate` spawns agents as peers, not nested children. A researcher teammate can spawn its own Task sub-agents because the nesting depth resets at each peer.
- **Wave-based execution** — `/crank` sidesteps the problem entirely. The orchestrator pre-plans waves of parallel work. Wave 1 workers run, complete, and write file artifacts. Wave 2 workers spawn fresh, read those artifacts. No nesting needed — workers are atomic (one task, one spawn, one result) and share work through the filesystem, not through parent context.

Workers are intentionally atomic. Fresh-context isolation per worker prevents contamination between waves. If you need deeper parallelism, decompose into more granular issues — 5 issues across 2 waves becomes 10 issues across 3 waves with finer granularity.

## How do I uninstall?

```bash
rm -rf ~/.claude/skills/ ~/.agents/skills/ ~/.codex/skills/ ~/.codex/plugins/cache/agentops-marketplace/agentops/
brew uninstall agentops  # if installed
```

If Codex is showing duplicate skills and you do not want a full uninstall, remove
or archive stale copies in `~/.agents/skills/` and `~/.codex/skills/`, and keep
the native plugin cache in
`~/.codex/plugins/cache/agentops-marketplace/agentops/` as the source of truth
unless you intentionally need raw-skill mode for your Codex build.
