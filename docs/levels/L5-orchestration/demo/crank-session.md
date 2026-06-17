# Demo: /crank Session

A session showing autonomous epic execution via the ODMCR loop.

---

## Before

Epic created with `/plan`:

```
$ BEADS_DIR=$PWD/_beads br show agentops-epic-xyz
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

$ BEADS_DIR=$PWD/_beads br sync   # Optional ledger sync check
$ git push
```

---

## Parallel Mode (NTM Swarm)

When disjoint issues can run concurrently, the loop dispatches them to an
NTM tmux swarm, coordinated through MCP Agent Mail (locks + inboxes):

```
You: /crank agentops-epic-xyz

Claude: Loading epic...

**Epic:** Add real-time notifications
**Mode:** parallel (NTM swarm + Agent Mail)
**Issues:** 6 total

[OBSERVE] 2 issues ready (a1, b2)

[DISPATCH] NTM pane agentops-a1 → /implement
[DISPATCH] NTM pane agentops-b2 → /implement

[MONITOR] Swarm status (Agent Mail):
  → agentops-a1: in_progress (pane-1)
  → agentops-b2: in_progress (pane-2)

[Panes working in parallel...]

[COLLECT] Both complete
  ✓ agentops-a1 CLOSED
  ✓ agentops-b2 CLOSED

[OBSERVE] 3 issues ready (c3, d4, e5)

[DISPATCH] NTM pane agentops-c3 → /implement
[DISPATCH] NTM pane agentops-d4 → /implement
[DISPATCH] NTM pane agentops-e5 → /implement

...continues until epic CLOSED
```

---

## What You Learned

1. `/crank` runs the ODMCR loop until epic is CLOSED
2. Auto-detects sequential vs parallel mode
3. NO human prompts - fully autonomous
4. Handles dependencies automatically via beads
5. Dispatches parallel work to an NTM swarm coordinated by MCP Agent Mail
