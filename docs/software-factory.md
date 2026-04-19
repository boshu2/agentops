# Software Factory Surface

Publicly, AgentOps is the operational layer for coding agents. This document
names the operator surface and software-factory mechanics beneath that public
story so users do not have to infer them from skills, hooks, CLI commands, and
internal artifacts.

## Thesis

AgentOps gives coding agents four things by default:

- bookkeeping
- validation
- primitives
- flows

This page explains the operator surface beneath that promise. Internally,
AgentOps is best understood as a **software-factory control plane**.

The environment carries:

- bounded briefing and context assembly
- tracked planning and scoped execution
- validation gates and ratchet checkpoints
- bookkeeping and flywheel closure between sessions
- isolated work lanes for long-running or parallel work

The workers remain replaceable. The environment carries continuity.

This follows the repo's stateful-environment/stateless-agents theory and its
own lifecycle/flywheel contracts: briefings and runtime state are the operator
surface; packets, chunks, topics, and builders are substrate.

## Runtime Variants

The factory runs on two classes of runtime. The capability gap between them is
the hooks surface: Claude Code has it natively; Codex does not.

### Claude Code (hook-native)

Claude Code provides `hooks.json` — a declarative surface that fires shell
scripts at lifecycle events (`SessionStart`, `PreToolUse`, `PostToolUse`,
`UserPromptSubmit`, `Stop`, etc.). This gives the factory automatic enforcement
with zero operator action:

```
SessionStart  →  session-start.sh injects context
UserPromptSubmit  →  factory-router.sh, prompt-nudge.sh, quality-signals.sh
PreToolUse    →  pre-mortem-gate.sh blocks unvalidated /crank
PostToolUse   →  go-vet, complexity, research-loop-detector
Stop          →  ao-flywheel-close.sh persists learnings
```

In Claude Code the operator lane is simply:

```bash
/rpi "fix auth startup"
```

Hooks handle startup context, validation gates, execution discipline, and
flywheel closure automatically around whatever the agent does.

### Codex (hookless — explicit lifecycle)

Codex has no hooks surface. The factory compensates with an explicit CLI
lifecycle that skills auto-invoke to simulate what hooks provide natively:

```bash
ao codex start          # Replaces SessionStart hooks — assembles briefing,
                        # writes startup-context.md, runs safe maintenance
/rpi "fix auth startup" # Skills invoke ao codex ensure-start internally
ao codex stop           # Replaces Stop hooks — close-loop, learnings, handoff
```

Key differences from the hook-native path:

| Concern | Claude Code | Codex |
|---------|-------------|-------|
| Startup context | `session-start.sh` fires automatically | `ao codex start` must be called (or skills call `ao codex ensure-start`) |
| Validation gates | `pre-mortem-gate.sh` blocks tool calls | Skills check gate status themselves |
| Code quality | `go-vet-post-edit.sh`, `go-complexity-precommit.sh` fire after edits | Must run `cd cli && make test` manually or via skill |
| Flywheel closure | `ao-flywheel-close.sh` fires on Stop | `ao codex stop` must be called explicitly |
| Execution nudges | `prompt-nudge.sh`, `research-loop-detector.sh` | No equivalent — relies on skill discipline |

The operator lane for Codex wraps this:

```bash
ao factory start --goal "fix auth startup"
/rpi "fix auth startup"
ao codex stop
```

`ao factory start` compiles a goal-time briefing and runs `ao codex start` so
the session begins with bounded context. `ao codex stop` closes the flywheel
so learnings, citations, and handoff state persist.

Both paths exist because people use Codex or they use Claude Code.

## Surface Map

| Layer | Purpose | Primary surfaces |
|------|---------|------------------|
| Operator | What the human or lead agent should touch first | `ao factory start`, `/rpi`, `ao rpi phased`, `ao rpi status`, `ao codex stop` |
| Briefing + runtime | Bounded startup context and thread-time state | `ao knowledge brief`, `ao codex start`, `ao codex ensure-start`, `ao context assemble` |
| Delivery line | Research, planning, execution, validation | `/discovery`, `/plan`, `/crank`, `/validation`, `/rpi` |
| Learning loop | Convert completed work into future advantage | `ao codex stop`, `ao knowledge activate`, `ao flywheel close-loop`, `/retro`, `/forge` |
| Hooks | Automatic enforcement, quality gates, and execution discipline | `hooks/hooks.json`, `hooks/*.sh`, kill switch |
| Substrate | Retrieval, provenance, packetization, and promotion machinery | `.agents/packets/`, `.agents/topics/`, `.agents/briefings/`, `.agents/findings/`, builder logic |

## Hooks — The Automation Layer

Hooks (`hooks/hooks.json`) are shell scripts that fire automatically at lifecycle
events. They are the factory's invisible enforcement and hygiene layer — they
run without operator action and keep the conveyor belt honest.

| Event | Hook | Purpose |
|-------|------|---------|
| **SessionStart** | `session-start.sh` | Cleans stale runs, injects startup context |
| **SessionEnd** | `session-end-maintenance.sh` | Post-session cleanup and state persistence |
| | `compile-session-defrag.sh` | Knowledge defragmentation pass |
| **Stop** | `ao-flywheel-close.sh` | Closes the flywheel loop (learnings, citations, handoff) |
| **UserPromptSubmit** | `factory-router.sh` | Routes operator intent to the correct lane |
| | `new-user-welcome.sh` | Gives fresh repos a one-time path into `/research`, `/implement`, or `/council` |
| | `prompt-nudge.sh` | Reminds about pending gates (e.g. pre-mortem) |
| | `intent-echo.sh` | Injects intent-echo discipline |
| | `quality-signals.sh` | Surfaces quality context before work begins |
| **PreToolUse** | `pre-mortem-gate.sh` | Blocks `/crank` or `/implement` without pre-mortem |
| | `go-test-precommit.sh` | Requires Go tests pass before commits |
| | `commit-review-gate.sh` | Reviews commit content before `git commit` |
| | `git-worker-guard.sh` | Prevents destructive git operations |
| | `edit-knowledge-surface.sh` | Warns on edits to knowledge-surface files |
| | `codex-parity-warn.sh` | Warns when skill edits may drift from Codex copies |
| **PostToolUse** | `write-time-quality.sh` | Checks quality of written/edited files |
| | `go-complexity-precommit.sh` | Enforces cyclomatic complexity budget |
| | `go-vet-post-edit.sh` | Runs `go vet` after Go file edits |
| | `research-loop-detector.sh` | Detects stalling in research without output |
| | `context-monitor.sh` | Tracks context window consumption |
| **TaskCompleted** | `task-validation-gate.sh` | Validates task output before marking complete |

Every hook checks the kill switch (`AGENTOPS_HOOKS_DISABLED=1`) and produces
structured JSON on stdout. Exit code `2` blocks the operation (PreToolUse only);
`0` passes.

Hooks enforce the factory's design rules automatically:

- **Validation gates** — `pre-mortem-gate.sh`, `go-test-precommit.sh`,
  `commit-review-gate.sh`, and `task-validation-gate.sh` prevent unvalidated
  work from advancing.
- **Ratchet checkpoints** — `ao-flywheel-close.sh` ensures learnings persist
  after each session.
- **Execution discipline** — `research-loop-detector.sh` and `prompt-nudge.sh`
  keep the agent producing artifacts instead of stalling.
- **Code quality** — `go-complexity-precommit.sh`, `go-vet-post-edit.sh`, and
  `write-time-quality.sh` catch regressions at edit time, not CI time.

## Why This Surface Exists

The factory framing matters because the repo already has the hard parts:

- RPI provides the conveyor belt.
- Context packets and briefings provide bounded work orders.
- The flywheel provides bookkeeping and closure between sessions.
- Codex lifecycle commands provide explicit runtime boundaries where hooks do
  not exist.

Without an explicit operator lane, users see a powerful collection of
primitives. With it, they see one product surface.

## Design Rules

- Prefer briefings over giant startup dumps.
- Keep substrate and operator surfaces distinct.
- Let external validation outrank self-report.
- Treat thin topics as discovery-only until evidence improves.
- Keep `compile` scoped to hygiene, not full operator-surface activation.

## Related Docs

- [How It Works](how-it-works.md)
- [Context Packet](context-packet.md)
- [Knowledge Flywheel](knowledge-flywheel.md)
- [Session Lifecycle](workflows/session-lifecycle.md)
- [CLI Reference](cli/commands.md)
