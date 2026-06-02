# Demo: /crank Session

A session showing autonomous epic execution via the ODMCR loop.

---

## Before

Epic created with `/plan`:

```
$ bd show agentops-epic-xyz
Epic: "Add real-time notifications"
Status: open
Children: 6 issues

Issues:
  [P1] agentops-a1: Set up WebSocket server
  [P1] agentops-b2: Create notification service
  [P2] agentops-c3: Add browser client (blocked by a1)
  [P2] agentops-d4: Create notification types (blocked by b2)
  [P2] agentops-e5: Add user preferences (blocked by b2)
  [P3] agentops-f6: Integration tests (blocked by c3, d4, e5)
```

---

## Session

```
You: /crank agentops-epic-xyz

Claude: Loading epic...

**Epic:** Add real-time notifications
**Mode:** crew (sequential)
**Issues:** 6 total

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[OBSERVE] 2 issues ready, 4 blocked

[DISPATCH] /implement agentops-a1
  → Set up WebSocket server

[Working...]

✓ agentops-a1 CLOSED

[COLLECT] Updated dependencies, 1 issue unblocked

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[OBSERVE] 2 issues ready (b2, c3), 3 blocked

[DISPATCH] /implement agentops-b2
  → Create notification service

[Working...]

✓ agentops-b2 CLOSED

[COLLECT] Updated dependencies, 2 issues unblocked

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[OBSERVE] 3 issues ready (c3, d4, e5), 1 blocked

[DISPATCH] /implement agentops-c3
  → Add browser client

[Working...]

✓ agentops-c3 CLOSED

[DISPATCH] /implement agentops-d4
  → Create notification types

[Working...]

✓ agentops-d4 CLOSED

[DISPATCH] /implement agentops-e5
  → Add user preferences

[Working...]

✓ agentops-e5 CLOSED

[COLLECT] Updated dependencies, 1 issue unblocked

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[OBSERVE] 1 issue ready (f6), 0 blocked

[DISPATCH] /implement agentops-f6
  → Integration tests

[Working...]

✓ agentops-f6 CLOSED

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[OBSERVE] 0 issues remaining

**Epic CLOSED!**

Summary:
- 6/6 issues closed
- Mode: crew (sequential)
- All tests passing

$ bd vc status   # Optional Dolt status check
$ git push
```

---

## Mayor Mode (Parallel)

If run from `~/gt` or a mayor directory:

```
You: /crank agentops-epic-xyz

Claude: Loading epic...

**Epic:** Add real-time notifications
**Mode:** mayor (parallel via NTM)
**Issues:** 6 total

[OBSERVE] 2 issues ready (a1, b2)

[DISPATCH] gt sling agentops-a1 athena
[DISPATCH] gt sling agentops-b2 athena

[MONITOR] Convoy status:
  → agentops-a1: in_progress (polecat-1)
  → agentops-b2: in_progress (polecat-2)

[Polecats working in parallel...]

[COLLECT] Both complete
  ✓ agentops-a1 CLOSED
  ✓ agentops-b2 CLOSED

[OBSERVE] 3 issues ready (c3, d4, e5)

[DISPATCH] gt sling agentops-c3 athena
[DISPATCH] gt sling agentops-d4 athena
[DISPATCH] gt sling agentops-e5 athena

...continues until epic CLOSED
```

---

## What You Learned

1. `/crank` runs the ODMCR loop until epic is CLOSED
2. Auto-detects crew (sequential) vs mayor (parallel) mode
3. NO human prompts - fully autonomous
4. Handles dependencies automatically via beads
5. Integrates with NTM for multi-agent parallelization
