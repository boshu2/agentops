# Demo: /plan Session

A session showing goal decomposition into tracked issues.

---

## Before

- Beads initialized (`BEADS_DIR="$(ao beads dir)" br init` run previously)
- Goal: Add email notifications to the app
- No existing issues for this feature

---

## Session

```
You: /plan "add email notifications for order updates"

Claude: I'll research the notification requirements and create a plan.

[Research] Looking at existing order system...

[Read] src/orders/processor.ts
[Read] src/notifications/ (doesn't exist)
[Grep] "email|notify|send" in src/

**Analysis:**

Orders are processed in processor.ts. No notification system exists.
Need to:
1. Set up email service
2. Add notification triggers to order events
3. Create email templates
4. Add user preference handling

**Creating issues:**

$ BEADS_DIR="$(ao beads dir)" br create --title "Set up email service (SendGrid)" --type task --priority 1
Created: ag-a1b

$ BEADS_DIR="$(ao beads dir)" br create --title "Add notification triggers to order processor" --type task --priority 1
Created: ag-c2d

$ BEADS_DIR="$(ao beads dir)" br create --title "Create order email templates" --type task --priority 2
Created: ag-e3f

$ BEADS_DIR="$(ao beads dir)" br create --title "Add email preference settings" --type feature --priority 2
Created: ag-g4h

$ BEADS_DIR="$(ao beads dir)" br create --title "Write notification tests" --type task --priority 3
Created: ag-i5j

**Setting dependencies:**

$ BEADS_DIR="$(ao beads dir)" br dep add ag-c2d ag-a1b
(triggers depend on email service)

$ BEADS_DIR="$(ao beads dir)" br dep add ag-e3f ag-a1b
(templates depend on email service)

$ BEADS_DIR="$(ao beads dir)" br dep add ag-i5j ag-c2d
$ BEADS_DIR="$(ao beads dir)" br dep add ag-i5j ag-e3f
(tests depend on triggers and templates)

**Plan saved to:** .agents/plans/2025-01-15-email-notifications.md

**Ready to work:**
$ BEADS_DIR="$(ao beads dir)" br ready
1. [P1] ag-a1b: Set up email service (SendGrid)
2. [P2] ag-g4h: Add email preference settings

Start with: `/implement ag-a1b`
```

---

## After

```
_beads/
└── issues.jsonl                # Issues stored here (git-JSONL ledger)

.agents/
└── plans/
    └── 2025-01-15-email-notifications.md
```

Issues created:
- 2 ready (no blockers)
- 3 blocked (have dependencies)

---

## What You Learned

1. `/plan` researches before creating issues
2. Issues get IDs like `ag-xyz`
3. Dependencies control execution order
4. `br ready` shows what can be worked now
5. Plan document captures the reasoning
