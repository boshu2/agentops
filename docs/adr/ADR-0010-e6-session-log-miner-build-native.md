# ADR-0010: The E6 session-log miner is BUILD-NATIVE — own the PROV-O graph

> **Status:** Accepted (2026-06-22). Resolves the E6.0 spike
> (`age-membrane-memory-arch-tz2s.6.1`). Cross-family decided (Claude + Codex).
> Scopes E6.1 (`...6.2`) and constrains E6.TEST (`...6.3`).

## Context

E6 mines session JSONL — the raw ore — into **per-inference typed provenance
events** (`tool_call`, `context_entered`, `context_missed`) for the membrane.
Parsing is expensive, so the miner must be **incremental** (watermark + dedup).
The spike question: **adopt** Jeffrey-Emanuel flywheel tools (cass / `cm`) or
**build native**? E6.1's stated rule: *steal technique, build native, own the
PROV-O graph; no langchain core dep.*

Verified during the spike:

- **cass** (`ADOPT-UPSTREAM`) is a *unified TUI **search**/index over agent
  histories* — it does not emit typed PROV-O provenance events.
- **`cm`/cass-memory** (`ADOPT-UPSTREAM`) is *procedural **memory*** (`cm
  context`) — a learning-loop substrate, not a provenance miner.
- **`ms`/meta-skill** turns mined know-how into skill artifacts — *downstream*
  of mining, not the miner.
- **OpenKB port** (`cli/internal/llmwiki`) compiles *sources → wiki artifacts*
  (summary/concept/entity/index) — a related extraction pattern, but its product
  is the wiki, not the provenance graph.
- The build foundation is **already native**: `cli/internal/parser/parser.go`
  parses session transcripts into a `ParseResult` (the same parser `ao yield
  tokens` uses), and `scripts/assay/self-improvement-tick.sh` is the existing
  bounded ASSAY orchestration with a **pluggable `--mine-cmd`** that "does NOT
  rebuild any miner."

## Decision

**BUILD-NATIVE.** There is no adoption case for cass/`cm` *as the miner*. Neither
owns the output AgentOps needs — deterministic per-inference PROV-O events — and
the miner's *product is the provenance graph itself*, which is AgentOps's owned
core (the membrane's gradient is deterministic provenance, ADR-0004). Renting
that core to cass/`cm` creates the wrong dependency boundary. cass and `cm` stay
adopted **for their own lanes** (session search, procedural memory).

- **STEAL (technique, not code):** cass's incremental-index discipline —
  *stale-is-usable*, *skip consumed input*, *bounded refresh*, *recover loudly*,
  *never rebuild expensive state unnecessarily*.
- **OWN:** the native Go parser extension, the event schema, idempotent event
  IDs, the PROV-O relation mapping, ledger/write semantics, tests, and gate
  compatibility. **No langchain core dep.**

## E6.1 scope (`...6.2`)

Implement `ao provenance mine-session --file <session.jsonl> --state
<state.json> --json` as a native MVP over `cli/internal/parser`:

- Emit schema-validated JSONL events for `tool_call`, `context_entered`,
  `context_missed` **only** when deterministically evidenced by the
  transcript/tool output (no inference of un-evidenced "missed context").
- Stable event IDs keyed by `session / source-line / kind / tool identity` (so
  reruns are idempotent).
- Incremental state with last-line/size/checksum **rollback detection** (a
  truncated/rewritten transcript re-mines cleanly).
- Fixtures for **Claude and Codex** tool-call shapes; tests proving **idempotent
  reruns** and **append-only incremental mining**.

## Consequences

- E6.TEST (`...6.3`) asserts: skips consumed, dedups, emits typed edges.
- ASSAY wires `mine-session` as a possible `--mine-cmd`; ASSAY policy is otherwise
  unchanged.

## Deferred

cass/`cm` integration into the miner; semantic/vector inference of missed
context; cross-session clustering; TUI/search UX; memory promotion; meta-skill
generation; broad graph visualization.

## What would reverse this

A structured, deterministic per-inference provenance **export** appearing in
cass/`cm` upstream (tool-call + context events with stable IDs) that we'd be
foolish to re-implement — at which point re-evaluate adopt-the-export while still
owning the PROV-O relation mapping + ledger semantics.
