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

AgentOps 3.0 is **hookless**: the factory runs identically on every runtime
(Claude Code, Codex, Cursor, OpenCode) because it does not depend on any
harness-specific hook surface. Workflow is guided by skills + the `ao` CLI, and
CI is the authoritative gate. The operator lane is the same everywhere:

```bash
/rpi "fix auth startup"            # interactive, any harness
ao daemon submit --kind rpi --goal "fix auth startup"   # pipeline-resident
```

What used to be hook responsibilities are now explicit, pulled surfaces:

| Concern | Hookless surface |
|---------|------------------|
| Startup context | `ao knowledge brief` / `ao context assemble` (pulled, not injected on every event) |
| Validation gates | CI (`.github/workflows/validate.yml`) + skill-level checks run in-band |
| Code quality | `cd cli && make test`, `go vet`, complexity budget — enforced by CI |
| Flywheel closure | `ao flywheel close-loop` / `/retro` / `/forge` at session close |
| Execution discipline | Execution-packet `next_action` + skill instructions |

Both interactive and daemon paths exist because people use Codex or they use
Claude Code — but neither relies on hooks.

## Surface Map

| Layer | Purpose | Primary surfaces |
|------|---------|------------------|
| Operator | What the human or lead agent should touch first | `ao factory start`, `/rpi`, `ao rpi phased`, `ao rpi status`, `ao daemon submit` |
| Briefing + runtime | Bounded startup context and thread-time state | `ao knowledge brief`, `ao context assemble`, `ao daemon submit` |
| Delivery line | Research, planning, execution, validation | `/discovery`, `/plan`, `/crank`, `/validation`, `/rpi` |
| Learning loop | Convert completed work into future advantage | `ao knowledge activate`, `ao flywheel close-loop`, `/retro`, `/forge` |
| Enforcement | Automatic quality gates and execution discipline | CI (`.github/workflows/validate.yml`), skill-level checks, `cd cli && make test` |
| Substrate | Retrieval, provenance, packetization, and promotion machinery | `.agents/packets/`, `.agents/topics/`, `.agents/briefings/`, `.agents/findings/`, builder logic |

## Enforcement — No Hooks Required

AgentOps 3.0 ships **zero hooks**. The factory's design rules are enforced by
explicit, pulled surfaces rather than automatic lifecycle scripts:

- **Validation gates** — CI (`.github/workflows/validate.yml`) is the
  authoritative gate; skills run their own checks in-band before claiming work
  complete.
- **Ratchet checkpoints** — `ao flywheel close-loop` / `/retro` / `/forge`
  persist learnings at session close.
- **Execution discipline** — execution-packet `next_action` and skill
  instructions keep the agent producing artifacts instead of stalling.
- **Code quality** — `cd cli && make test`, `go vet`, and the complexity budget
  are enforced by CI on every push.

Operators who *want* runtime hooks can author their own with the
`hooks-authoring` skill — they are opt-in and not part of the default product
surface.

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

<!-- agentops:claim:AOP-CLAIM-SOFTWARE-FACTORY-THIN-TOPICS -->
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
