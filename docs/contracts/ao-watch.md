# Spec: `ao watch --follow`

> **Status:** Spec only. Implementation gated on Wave 3 (yjzp.6) landing first.
> **Issue:** soc-yjzp.7
> **Anthropic analog:** Managed Agents Console trace (May 2026)
> **Kill criterion:** Close as wontfix if implementation has not started by **2026-06-15** and no real observability complaint has surfaced.

## Goal

Live event stream of all worker activity during `/crank` waves and `/swarm` dispatches. The off-API analog of Anthropic's Console trace: see what every worker is doing in real time, post-hoc replay, no managed cloud service.

## Naming and the trace.go collision

**The verb is `ao watch`, NOT `ao trace`.** `cli/cmd/ao/trace.go` already exists for artifact provenance (decisions → artifacts mapping) and we must not clobber it. Documented in the soc-yjzp execution packet's `do_not_touch` list.

## CLI surface

```
ao watch              # interactive TUI (post-MVP; deferred)
ao watch --follow     # tail-style follow of the event log; SIGINT to exit
ao watch --since=<ts> # replay events since timestamp (no follow)
ao watch --html       # write static HTML viewer to .agents/watch/index.html
ao watch --json       # stream raw JSON lines (machine consumption)
```

## Event log

Append-only JSONL at `.agents/watch/events.jsonl`. One event per line. Producers (`/crank`, `/swarm`, gc bridge) append; consumers tail.

### Event schema

```json
{
  "ts": "2026-05-06T22:55:00.123Z",
  "event": "worker.started",
  "wave": 2,
  "epic_id": "soc-yjzp",
  "worker_id": "wave-2-yjzp.4",
  "issue_id": "soc-yjzp.4",
  "role": "impl",
  "model": "claude-opus-4-7",
  "tools": ["Read", "Write", "Edit", "Bash"],
  "files_claimed": ["docs/patterns/completion-notifications.md"],
  "parent_event": null,
  "data": { /* event-specific payload */ }
}
```

Required: `ts`, `event`, `worker_id`. Everything else optional but recommended.

### Event types (v1)

| event | Producer | When |
|---|---|---|
| `wave.started` | crank | Beginning of a wave |
| `wave.completed` | crank | End of a wave (verdict in `data`) |
| `worker.spawned` | swarm | Subprocess/Agent created |
| `worker.started` | swarm | Worker begins task |
| `worker.tool_call` | swarm | Worker invokes a tool (sampled, not every call) |
| `worker.completed` | swarm | Worker finished (success/fail in `data.verdict`) |
| `worker.failed` | swarm | Worker errored / timed out |
| `validation.started` | crank | Vibe gate begins |
| `validation.completed` | crank | Vibe gate verdict |
| `epic.closed` | crank | All issues closed |

## Implementation skeleton

| File | Action | Purpose |
|---|---|---|
| `cli/cmd/ao/watch.go` | NEW | Cobra command + --follow/--since/--html/--json flags |
| `cli/cmd/ao/watch_test.go` | NEW | L1 unit tests |
| `cli/internal/watch/log.go` | NEW | `Append(ev Event) error`, atomic JSONL append |
| `cli/internal/watch/follow.go` | NEW | Tail-style follow with inotify or polling |
| `cli/internal/watch/html.go` | NEW | Static HTML viewer template + writer |
| `cli/cmd/ao/crank.go` | MODIFY | Emit wave.started/wave.completed |
| `cli/cmd/ao/swarm.go` | MODIFY | Emit worker.spawned/started/completed/failed |
| `cli/cmd/ao/gc_events.go` | RECONSIDER | Existing surface — broaden scope from GC-only to all-agent activity, OR keep GC-specific and let watch read from both |

## Dependencies between events

Event chain identity via `parent_event` (event ID hash from `ts:worker_id:event`). Wave events are roots; worker events parent to a wave event; tool calls parent to a worker event. Lets the HTML viewer build a tree without the producer needing to know about tree structure.

## Retention

Default: keep `events.jsonl` for 14 days. After that, gzip rotate to `.agents/watch/archive/events-YYYY-MM-DD.jsonl.gz`. Configurable via `AO_WATCH_RETENTION_DAYS`.

## Tests (at implementation time)

### L1
- `TestLog_Append` — atomic append, file lock, roundtrip
- `TestLog_AppendConcurrent` — N parallel appenders, no interleaved lines
- `TestFollow_TailsAppends` — append → follower receives within 100ms
- `TestHTML_RendersFromJSONL` — fixture JSONL → expected HTML

### L2
- `tests/integration/watch.bats`:
  - `ao watch --since=<ts>` returns expected events from fixture
  - `ao watch --html` writes valid HTML
  - `ao watch --follow` (with timeout + inotify) sees a programmatically appended event

## Acceptance for the spec stage (this issue, soc-yjzp.7)

- This file exists at `docs/contracts/ao-watch.md` (tracked) and is linked from `docs/documentation-index.md`.
- Event schema is JSON-validatable (could be promoted to `schemas/watch-event.v1.schema.json` at implementation time).
- The `do_not_touch trace.go` rule is documented (already in `.agents/rpi/execution-packet.json`).

The spec is the deliverable for soc-yjzp.7. Implementation is a separate epic conditional on demand surfacing.

## Reference: pre-mortem fix (from `.agents/council/2026-05-06-pre-mortem-managed-agents-parity.md`)

This spec implements Fix 4 (kill criterion) verbatim — kill date 2026-06-15.
