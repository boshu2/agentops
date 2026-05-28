# Agent-Genie Coordination Contract

When two or more agent streams work in parallel against the same repo,
**each stream MUST state its coordination contract before landing any
commit**. The contract is not chat memory — it's an artifact each stream
writes and checks. Without it, parallel streams collide on generated
artifacts (registry.json, mkdocs nav, CLI docs, embedded copies) and one
stream's "final regen" silently overwrites the other's work.

## The Contract — Eight Required Fields

Each parallel agent stream declares, before claiming any work:

| Field | Example |
|---|---|
| **Stream name** | `doctor`, `rpi`, `wireup` |
| **Branch name** | `feat/<type>/<bead>-<slug>` |
| **Base SHA** | `git rev-parse origin/main` at stream start |
| **Owned paths** | `cli/cmd/ao/doctor*.go`, `cli/internal/doctor/**` |
| **Forbidden paths** | `cli/cmd/ao/loop*.go` (owned by sibling stream) |
| **Shared generated files** | `registry.json`, `docs/contracts/context-map.md` — declare which stream regenerates last |
| **Handoff triggers** | "ping `<sibling>` when `tests/doctor/*` green" |
| **Closeout conditions** | "PR merged + bead closed + `<sibling>` consumes my artifact" |

Write these to a coordination file the sibling stream can read — Agent
Mail (`mcp__mcp-agent-mail__send_message`), a `.agents/coordination/<topic>.md`,
or pinned in a shared NTM pane. **Not in chat memory.** Chat memory
doesn't survive the handoff.

## Why This Matters

Generated artifacts (registry.json, mkdocs nav, CLI docs, embedded
hook copies) are produced by scripts that read the *whole* state of
the repo. If two streams both run the regenerator at end-of-stream, the
later run overwrites the earlier — silently, with a green diff. The
"shared generated files" field is the load-bearing one: it names which
stream is allowed to regen last, and the other stream must commit its
work *before* the regen and skip the regen entirely.

## Evidence (anchored)

> "Parallel agent streams work reliably when each agent states branch
> name, base SHA, owned paths, forbidden paths, shared generated files,
> handoff triggers, and closeout conditions before landing. … This
> prevents collisions on generated artifacts and lets one stream
> unblock another without relying on ad hoc chat memory."
— `.agents/learnings/2026-05-16-agent-genie-coordination-contract.md`
(post-mortem for soc-z3qo.1 / PR #285)

The empirical anchor: PR #285. The doctor stream owned doctor/canary
drift; the RPI stream owned execution-packet files plus final
`registry.json` regeneration. Each stream's contract was explicit
about lanes. The streams committed in series; the registry regen
happened exactly once at the end of the RPI stream. No collisions.

## How To Apply

### Before claiming work (each stream)

1. **Read the bead** for "owned files" / acceptance criteria.
2. **Write your contract** to one of:
   - Agent Mail topic (`macro_prepare_thread` then `send_message`)
   - `.agents/coordination/<topic>.md` (committed; visible to all streams
     via the file system)
   - NTM pane title or pinned message
3. **Confirm receipt** with sibling streams. Don't proceed until they
   have read and replied.

### Sample contract block

```markdown
## Stream: doctor (soc-z3qo.1)
- Branch:           feat/doctor-soc-z3qo.1-rebuild
- Base SHA:         abc1234
- Owned:            cli/cmd/ao/doctor*.go, cli/internal/doctor/**, tests/doctor/**
- Forbidden:        cli/cmd/ao/rpi*.go (rpi stream), .agents/rpi/** (rpi stream)
- Shared (I do NOT regen): registry.json, cli/docs/COMMANDS.md
- Handoff:          ping #rpi-stream when tests/doctor/* green
- Closeout:         PR merged + bead closed + rpi stream consumes my Healable trait
```

### During work

- **Stay in your lane.** If you discover scope outside your owned
  paths, write a scope-escape note (see
  [scope-escape-template.md](scope-escape-template.md)). Do not edit.
- **Don't run shared-file regenerators** unless your contract names you
  as the regen owner. Commit your non-generated edits, hand off, let
  the regen owner do the final pass.
- **Re-read sibling contracts** at each commit. If a sibling has moved
  their boundary, your forbidden-paths list may have changed.

### At closeout

- Confirm closeout conditions (PR merged, bead closed, artifact
  consumed by sibling).
- Archive the contract file or mark the Agent Mail thread complete.
- If your stream produced a shared-file delta, name the SHA so the
  sibling can rebase.

## Failure Modes

- **Implicit ownership.** "I assumed they wouldn't touch registry.json"
  — they did, and the regen overwrote your changes. The contract
  prevents this by making ownership explicit and writable.
- **Chat memory only.** "I told them in the NTM pane chat" — that
  message scrolled off; the sibling claimed the path anyway. Contracts
  live in artifacts, not transient chat.
- **Late contract.** "I'll write the contract once I know what I'm
  doing." By then you're already committing. Contracts go before
  claims.
- **Closeout without confirmation.** "I merged my PR; my work here is
  done." If the sibling hasn't consumed your artifact, your closeout
  is premature; the bead may need to stay open until the consumer
  verifies.

## Relation to Other Swarm Rules

This contract is **separate from but composable with** the other
multi-agent swarm rules:

- **Worktree isolation** (`worktree-isolation.md`) — physical
  isolation of each stream's checkout
- **Scope escape** (`scope-escape-template.md`) — what to do when you
  discover work outside your lane
- **Pre-spawn friction gates** (`pre-spawn-friction-gates.md`) — gates
  that fire before a swarm even starts

The coordination contract is the *operating protocol* on top of those
mechanics: even with worktree isolation, even with scope-escape
templates, streams still collide on generated artifacts unless they
declare ownership upfront.

## See Also

- `worktree-isolation.md` — physical isolation of each stream's checkout
- `scope-escape-template.md` — what to do when scope creeps outside
  the contract
- `pre-spawn-friction-gates.md` — gates that fire before a swarm
  even starts
- `agent-mail` skill — primary medium for coordination contract delivery
