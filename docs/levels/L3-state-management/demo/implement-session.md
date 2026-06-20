# Demo: /implement with Issue Tracking

A session showing issue-driven implementation.

---

## Before

```
$ BEADS_DIR="$(ao beads dir)" br ready --json
1. [P1] agentops-a1b: Set up email service (SendGrid)
2. [P2] agentops-g4h: Add email preference settings
```

You'll implement the email service issue.

---

## Session

```
You: /implement agentops-a1b

Claude: Working on agentops-a1b: "Set up email service (SendGrid)"

$ BEADS_DIR="$(ao beads dir)" br update agentops-a1b --claim --json
✓ Updated

[Read] package.json - checking for existing email deps
[Read] .env.example - checking for env patterns

**Implementation:**

$ npm install @sendgrid/mail

[Write] src/services/email.ts
- EmailService class with send() method
- Configured from SENDGRID_API_KEY env var
- Error handling with retry

[Write] src/services/email.test.ts
- Tests for send success/failure
- Mock SendGrid API

$ npm test src/services/email.test.ts
2 passed

$ npm run lint
No issues

**Committing:**

$ git add src/services/email.ts src/services/email.test.ts package.json
$ git commit -m "feat: add SendGrid email service

- EmailService with send() method
- Retry logic for transient failures
- Tests with mocked API

Closes: agentops-a1b"

$ BEADS_DIR="$(ao beads dir)" br close agentops-a1b --reason "Email service implemented with tests"
✓ Closed agentops-a1b

**Newly unblocked:**
$ BEADS_DIR="$(ao beads dir)" br ready --json
1. [P1] agentops-c2d: Add notification triggers to order processor
2. [P2] agentops-e3f: Create order email templates
3. [P2] agentops-g4h: Add email preference settings

Three issues now ready! (c2d and e3f were blocked by a1b)
```

---

## After

- `src/services/email.ts` created
- Tests passing
- Issue closed, dependencies unblocked
- Ready for next issue

---

## Session Close

Before ending:
```
$ BEADS_DIR="$(ao beads dir)" br sync --flush-only
$ git -C "$(ao beads dir)" push   # Sync the private br ledger
$ git push
```

---

## What You Learned

1. `/implement <id>` targets a specific issue
2. Issue auto-marked `in_progress` then closed
3. Commit message references the issue
4. Closing an issue unblocks dependents
5. Issue updates auto-sync to JSONL; push before ending session
