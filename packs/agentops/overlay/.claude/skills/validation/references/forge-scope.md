# Forge Scope in /validation

## Default Behavior

When `/validation` invokes forge (Step 4), it runs:

```bash
ao forge transcript --last-session --queue --quiet 2>/dev/null || true
```

This mines **the current session only** — not the full transcript corpus.

## Scope Boundaries

| Scope | Command | When to Use |
|-------|---------|-------------|
| Current session | `ao forge transcript --last-session` | Default in `/validation` |
| Full corpus | `ao forge transcript` | Only via `/compile` or manual invocation |
| Specific session | `ao forge transcript <path/to/session.jsonl>` | Manual targeted mining |

## Rationale

Session-scoped forge prevents `/validation` from:
1. **Re-mining old sessions** that have already been forged
2. **Producing duplicate learnings** from previously captured patterns
3. **Consuming excessive time** on large transcript corpora

Full-corpus mining is `/compile`'s responsibility (Mine → Grow → Defrag cycle).

## Deduplication

Even with session scoping, forge may extract learnings that overlap with existing `.agents/learnings/` content. The `ao forge` pipeline handles dedup internally via content hashing — duplicate findings are silently dropped.

## Skipping Forge

Use `--no-forge` when:
- The session was purely mechanical (no novel decisions or patterns)
- `ao` CLI is not available (forge is auto-skipped in this case)
- You want faster validation turnaround

Forge is always optional — `/validation` succeeds regardless of forge outcome.
