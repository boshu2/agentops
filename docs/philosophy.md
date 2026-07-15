---
last_reviewed: 2026-07-14
---

# AgentOps Philosophy

AI agents are stochastic authors. Tests can establish deterministic facts, but
they do not judge every semantic claim, and an author should not certify its
own work. AgentOps therefore provides a small evidence protocol:

```text
intent -> one bounded experiment -> exact subject identity
       -> fresh independent judgment -> durable verdict
```

The protocol is behavior-first. Plan expresses one behavior as normal and edge
Given/When/Then scenarios, non-goals, write scope, and required evidence.
Implement performs one bounded RED-to-GREEN-to-refactor experiment. Validate
binds criterion-level judgment to a deterministic content manifest and stores
the result as `verdict.v2`. RPI invokes those responsibilities once and stops.

This is a trust floor, not a workflow engine. AgentOps does not own retries,
budgets, queues, claims, leases, Git, CI, closure, release, or delivery. A FAIL
is evidence for the caller, not permission for AgentOps to repair or continue.

Learn remains an optional, off-path hypothesis: collections of verdicts may
reveal repeated defect classes worth turning into guidance or deterministic
checks. Promotion is deliberate and evidence-backed; it never changes the
validity of the candidate that produced the observation.

The product remains portable. Its core contracts operate in a non-Git
directory without the `ao` binary. The CLI supplies repository utilities and
ordinary deterministic checks. Optional councils, genies, runtimes, trackers,
and software-factory adapters can consume the protocol without becoming its
authority.
