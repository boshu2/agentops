# Agent workflow reference

This document expands the repository contract in [AGENTS.md](../AGENTS.md).

AgentOps is the operations layer for agentic engineering; the standard
traversal through its federated integration graph is one pass
(exact semantics: [rpi-traversal.md](architecture/rpi-traversal.md)):

```text
RPI -> anti-ceremony guard -> Plan -> Implement -> fresh Validate -> report and stop
```

## 1. Pre-dispatch guard

RPI invokes Anti-Ceremony's artifact-free quick guard once before Plan. `STOP`
dispatches none of Plan, Implement, or Validate, reports `NOT_PLANNED` with the
guard's one-sentence reason, and stops. `CONTINUE` creates no process artifact
and preserves Plan -> Implement -> fresh Validate.

## 2. Plan

Resolve one active behavior in the caller-owned tracker, issue, or conversation.
Keep acceptance, important non-goals, required evidence, write scope, and the
first useful check in that source. Do not create a second planning artifact.

The runtime leaves a durable caller-owned source in place and carries its
reference plus the digest of its exact resolved bytes. Only when no durable
source exists does it snapshot those bytes under
`.agents/ao/intents/sha256/<digest>.intent`. This fallback is derived identity,
not a model-authored packet, and makes conversation-only intent readable by a
fresh validator. The pure fallback helper accepts a file or stdin:

```bash
python3 skills/validate/scripts/validate.py snapshot-intent --source PATH  # use - for stdin
```

## 3. Implement

Run one bounded RED-GREEN-refactor experiment when the behavior supports it.
The runtime derives factual check receipts, actual changed paths, author context
ID, and `subject-manifest.v1`; the model does not transcribe them into a
candidate packet. Implement does not commit, claim, repair, retry, close, push,
or deliver.

## 4. Validate

An author-distinct context judges the exact subject against acceptance. It
returns `PASS`, `FAIL`, or `NOT_PROVEN`, lists checked and unchecked scope, and
stops. PASS requires nonempty checked scope, top-level evidence, and evidence
for every criterion. A `NOT_PROVEN` finding states the concrete missing runtime
precondition or examined uncertainty; it does not manufacture a next action.
Validate does not repair, re-plan, choose a next action, or authorize Git.

The returned result is sufficient for interactive use. Validate persists the
same result as content-addressed `verdict.v2` only when the caller requests a
machine-readable artifact or a declared downstream consumer requires one.

## 5. Caller continuation

The caller receives the RPI report and decides what happens next. A revision
updates the caller-owned intent source and starts a new invocation. RPI creates
no parallel revision artifact. Changing acceptance changes the intent digest.

## Optional surfaces

Apart from RPI's required artifact-free Anti-Ceremony quick guard, premortem,
postmortem, councils, idea genies, runtime adapters, research tools, and factory
dispatch are caller-selected. Those optional surfaces never become hard
dependencies or lifecycle authorities. `dispatch_once` executes only explicitly
supplied, disjoint work once and performs no selection, retry, validation,
integration, Git, closure, or delivery.

## Repository mechanics

Git branches, worktrees, trackers, pull requests, merge queues, CI, pushing,
rollback, and release are repository or caller policy. `ao gate check` may run
deterministic checks; it conveys no semantic verdict.
