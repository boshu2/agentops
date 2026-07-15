# Agent workflow reference

The public AgentOps workflow has one pass:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## 1. Plan

Create one `plan-packet.v1` with one active behavior, normal and edge Gherkin
scenarios, non-goals, required evidence, explicit included/excluded write scope,
and the first acceptance check. Decomposition is advisory. The packet contains
no owner, claim, priority, attempt, queue, lease, next action, closure, release,
or delivery state.

## 2. Implement

Run one bounded RED-GREEN-refactor experiment when the behavior supports it.
Return a `candidate-packet.v1` containing factual evidence, actual changed
paths, the Plan digest, author context ID, and a `subject-manifest.v1`. Implement
does not commit, claim, repair, retry, close, push, or deliver.

## 3. Validate

An author-distinct context judges the exact candidate against acceptance and
writes one content-addressed `verdict.v2`. Validate is the only verdict writer.
It returns `PASS`, `FAIL`, or `NOT_PROVEN`, lists checked and unchecked scope,
and stops. It does not repair, re-plan, choose a next action, or authorize Git.

## 4. Caller continuation

The caller receives the RPI report and decides what happens next. A revision is
a new invocation supplied with an explicit `revision-packet.v1`; RPI neither
creates nor consumes one automatically. Changing acceptance starts a new intent.

## Optional surfaces

Premortem, postmortem, councils, idea genies, runtime adapters, research tools,
and factory dispatch are caller-selected. They never become hard dependencies
or lifecycle authorities. `dispatch_once` executes only explicitly supplied,
disjoint packets once and performs no selection, retry, validation, integration,
Git, closure, or delivery.

## Repository mechanics

Git branches, worktrees, trackers, pull requests, merge queues, CI, pushing,
rollback, and release are repository or caller policy. `ao gate check` may run
deterministic checks; it conveys no semantic verdict.
