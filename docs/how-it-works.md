# How it works

Your native coding agent owns the authorized change. AgentOps supplies a short
repository brief, deterministic AO tools and optional specialist guidance.
No RPI invocation or skill installation is required.

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

Keep behavior and scope in the existing issue or conversation. Use normal and
edge acceptance examples when useful. Implement, check and directly repair
known defects. A fresh author-distinct reviewer reads the exact change and
judges every acceptance criterion against evidence. Passing tests alone cannot
prove accepted work.

PASS requires complete checked scope, distinct identities, attested freshness
and evidence for acceptance. Failed acceptance is FAIL; missing proof remains
NOT_PROVEN. A repair changes the subject and needs a new judgment of that content.

When a caller or declared consumer needs durable evidence, the reviewer authors
`verdict.v2` and AO structurally verifies and atomically stores it in an explicit
protected external non-Git root:

```text
<evidence-root>/verdicts/sha256/<artifact-digest>.json
```

Identical stored content is idempotent. A digest collision with different
content fails integrity checks. Storage and check success cannot substitute
for a semantic judgment. Legacy `.agents/` evidence remains preserved under
owner policy.

The [RPI workflow](../skills/rpi/SKILL.md) remains available by explicit choice.
Native goals supply continuity and the existing tracker stores work and
handoffs. Memory is optional retrieval or separately scoped curation. Skills,
reviews and learning earn their cost through later engineering outcomes.
