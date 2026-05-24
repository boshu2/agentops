# Idempotency and Resume in /validation

## Re-run Behavior

`/validation` is **not idempotent** — each invocation produces fresh artifacts:

| Step | On Re-run | Output |
|------|-----------|--------|
| Vibe | New council report | `.agents/council/*vibe*.md` |
| Post-mortem | New retrospective | `.agents/council/*post-mortem*.md` |
| Retro | New learning capture | `.agents/learnings/*.md` |
| Forge | Append-only (deduped) | `.agents/learnings/*.md` |

This is intentional — validation should always reflect the current state of the code, not a cached result.

## Resume via `/rpi --from=validation`

Requirements:
- Epic-id as argument OR readable from `.agents/rpi/execution-packet.json`
- No dependency on Phase 1 or Phase 2 having run in the current session

## Standalone Mode

`/validation` without an epic-id runs in standalone mode:
- Vibe reviews recent changes (no epic scoping)
- Post-mortem is **skipped** (requires epic-id)
- Retro and forge run normally
