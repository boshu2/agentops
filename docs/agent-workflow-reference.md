# Agent workflow reference

This document expands the repository contract in [AGENTS.md](../AGENTS.md).

The public AgentOps workflow has one pass:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## 1. Plan

Resolve one active behavior in the caller-owned tracker, issue, or conversation.
Keep acceptance, important non-goals, required evidence, write scope, and the
first useful check in that source. Do not create a second planning artifact.

The runtime snapshots the exact resolved source bytes under
`.agentops/intents/sha256/<digest>.intent`. This is derived identity, not a
model-authored packet, and makes conversation-only intent readable by a fresh
validator. The pure helper accepts a file or stdin:

```bash
python3 skills/validate/scripts/validate.py snapshot-intent --source PATH  # use - for stdin
```

## 2. Implement

Run one bounded RED-GREEN-refactor experiment when the behavior supports it.
The runtime derives factual check receipts, actual changed paths, author context
ID, and `subject-manifest.v1`; the model does not transcribe them into a
candidate packet. Implement does not commit, claim, repair, retry, close, push,
or deliver.

## 3. Validate

An author-distinct context judges the exact subject against acceptance and
writes one content-addressed `verdict.v2`. Validate is the only verdict writer.
It returns `PASS`, `FAIL`, or `NOT_PROVEN`, lists checked and unchecked scope,
and stops. PASS requires nonempty checked scope, top-level evidence, and evidence
for every criterion. A `NOT_PROVEN` finding states the concrete missing runtime
precondition or examined uncertainty; it does not manufacture a next action.
Validate does not repair, re-plan, choose a next action, or authorize Git.

## 4. Caller continuation

The caller receives the RPI report and decides what happens next. A revision
updates the caller-owned intent source and starts a new invocation. RPI creates
no parallel revision artifact. Changing acceptance changes the intent digest.

## Optional surfaces

Premortem, postmortem, councils, idea genies, runtime adapters, research tools,
and factory dispatch are caller-selected. They never become hard dependencies
or lifecycle authorities. `dispatch_once` executes only explicitly supplied,
disjoint work once and performs no selection, retry, validation, integration,
Git, closure, or delivery.

## Repository mechanics

Git branches, worktrees, trackers, pull requests, merge queues, CI, pushing,
rollback, and release are repository or caller policy. `ao gate check` may run
deterministic checks; it conveys no semantic verdict.
