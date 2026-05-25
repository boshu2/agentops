# ao - AgentOps CLI

`ao` is the automation and control-plane surface for AgentOps. Use it when you
want the operational layer to run headlessly: stage briefings, inspect
bookkeeping, drive repeatable flows, or run overnight compounding without
staying inside an interactive skill.

One important lane inside the CLI is the software-factory surface. That lane
makes the work order, delivery flow, and closeout explicit instead of leaving
them implicit in chat state.

The short version:

```bash
ao quick-start
ao factory start --goal "fix auth startup"
ao rpi phased "fix auth startup"
ao overnight report
```

That lane keeps four concerns explicit:

1. `ao factory start` surfaces a bounded work order by compiling a goal-time
   briefing when the corpus supports it, then running Codex startup.
2. `ao rpi phased` runs the delivery flow with fresh context per phase, and
   `ao rpi status` lets the operator inspect long-running work.
3. `ao overnight start` and `ao overnight report` run the private Dream flow
   against the real local corpus and return a morning packet.
4. `ao codex stop` and `ao flywheel close-loop` close the bookkeeping loop so
   the session leaves behind learnings, citations, and handoff state.

## Install

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

## Quick Start

```bash
# From your repo root: create .agents/, starter knowledge surfaces, and hooks
ao quick-start

# Start a goal with briefing-first runtime context
ao factory start --goal "fix auth startup"

# Run the delivery flow
ao rpi phased "fix auth startup"

# Optional: set up and run a private overnight Dream
ao overnight setup
ao overnight start --goal "tighten auth startup"
ao overnight report
```

If you prefer the skill-first path, use `/rpi "fix auth startup"` after
`ao factory start`, and use `/dream` when you want the same overnight engine
through an interactive skill surface instead of a headless CLI command.

That's it. In Claude Code, `CLAUDE.md` remains the startup surface. The
installed hooks stay silent and only prepare runtime state for the higher-level
flows.

## Operator Surfaces

**SessionStart**: Performs startup maintenance, recovers handoff state, and can
stage `factory-goal.txt` / `factory-briefing.txt` without injecting context.

**UserPromptSubmit**: When startup lacked a goal, the first substantive prompt
can be captured as silent factory intake and staged for later explicit use.

**SessionEnd**: Extracts learnings and updates the feedback loop.

| Command | Purpose |
|---------|---------|
| `ao factory start --goal "<goal>"` | Compile briefing-first startup context, then run explicit Codex start |
| `ao knowledge brief --goal "<goal>"` | Build the task-time briefing directly |
| `ao codex start` | Lower-level hookless Codex startup |
| `ao rpi phased "<goal>"` | CLI-first Discovery -> Implementation -> Validation lane |
| `ao rpi status` | Monitor long-running phased work |
| `ao overnight setup` | Detect host/runtime constraints and persist Dream config |
| `ao overnight start --goal "<goal>"` | Run the local overnight compounding flow |
| `ao overnight report` | Render the latest Dream summary and council state |
| `ao codex stop` | Close the loop explicitly at session end |

## Underlying Primitives

These commands remain important, but they sit below the higher-level flows:

- `ao knowledge` for belief/playbook/briefing refresh and gap reporting
- `ao context assemble` for a five-section task briefing
- `ao lookup` and `ao search` for direct retrieval
- `ao forge transcript` and `ao flywheel close-loop` for manual lifecycle work

## Reference

- [Software Factory Surface](../docs/software-factory.md)
- [Dream Report Contract](../docs/contracts/dream-report.md)
- [Session Lifecycle](../docs/workflows/session-lifecycle.md)
- [CLI Reference](docs/COMMANDS.md)
