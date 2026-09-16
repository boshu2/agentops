# How it works

AgentOps connects a requested behavior to implementation and independent
validation. Your native coding agent owns the authorized change; optional
skills provide engineering guidance and `ao` supplies deterministic tools.
No RPI invocation or skill installation is required for the native path.

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

## Agree on observable behavior

Behavior-driven development (BDD) means agreeing on concrete examples before
coding and checking the result against those same examples. Keep behavior and
scope in the existing issue or conversation. Given/When/Then names a starting
situation, an action, and an observable result. It does not require a particular
test framework or a `.feature` file.

For example: given a Job has completed, when the worker receives it again, then
it returns the completed result without repeating the side effect. This names
two things to check, not just a successful return code.

Domain-driven design (DDD) keeps **Job**, **worker**, and **completed** consistent
with the caller's existing vocabulary and rule owners. Clarify ambiguity when
it would change behavior. Different bounded contexts can use the same word
differently; make the translation explicit at their boundary.

## Implement and validate the same promise

The implementer changes the relevant code, checks the accepted examples, and
directly repairs known defects within scope. A fresh author-distinct reviewer
reads the exact change and judges every acceptance criterion against evidence.
For the Job example, the evidence must establish both the returned result and
the absence of another side effect. Tests written after implementation cannot
redefine what was requested.

Planning, implementation, and validation are responsibilities, not a requirement
to run three permanent agents. The author cannot issue its own binding PASS.
By default, the fresh reviewer uses the author's model family; another model
or provider is an explicit choice. A different model alone does not establish
independence or truth.

PASS requires complete checked scope, distinct identities, attested freshness
and evidence for acceptance. Failed acceptance is FAIL; missing proof remains
NOT_PROVEN. A repair changes the subject and needs a new judgment of that content.

## Preserve useful work and evidence

The accepted example and its regression test preserve the Job behavior for
later changes. Keep useful decisions and handoffs with the existing repository
or tracker owner. Durable records support continuity across sessions; evidence
that later work used them successfully is what demonstrates benefit.

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
