# Ownership boundaries for the RPI core

Shared by `rpi`, `plan`, `implement`, `validate`, and the `anti-ceremony`
guard. Each kernel names the one step at which to read this file; the rules
below are the contract at that step, stated once.

## What each core skill owns

- **rpi**: the traversal: one guard call, at most one Plan and one Implement
  dispatch, fresh Validate, the bounded repair phase, and the report.
- **plan**: the shape of the intent inside the caller's own source: one
  active behavior, acceptance, non-goals, write scope, first check.
- **implement**: subject edits and factual check receipts.
- **validate**: one semantic result over one exact subject; the sole
  `verdict.v2` writer when persistence is requested.
- **anti-ceremony**: one admission judgment, `CONTINUE` or `STOP`.

## What none of them owns

The caller's tracker, repository policy, and runtime keep:

- retries, budgets, queues, claims, and leases; new invocations, and any
  extension beyond the caller's declared `repair_rounds` (inside that bound,
  RPI admits repair rounds under the convergence law; the bound itself is
  the caller's declaration, never a core budget);
- Git: commit, push, land, reserve, rebase, merge, rollback;
- delivery, release, closure, and the caller's next decision or next work;
- lane budgets, wave selection, and the extension of any caller bound;
- tracker or delivery mutation as a side effect of a phase: parking,
  findings, and proposed amendments are fields in a response.

Facts the runtime derives (changed paths, subject manifest, digests, check
receipts, context identities) are read from the runtime, never transcribed
by the model into a packet. Plans, audits, reviews, dashboards, and prompts
are control artifacts: they earn no capability credit and are completion
subjects only when the caller explicitly asked for document review.

## Adapters and specialists

Premortem, Postmortem, Council, genie, factory, tracker, and runtime adapters
are caller-selected and leave phase order and core outcomes unchanged, with
two legs the traversal dispatches itself on a risky write scope: premortem at
Plan exit, and council on a risky split that survives repair. Both stay
judgment legs. Premortem returns blocking findings against a plan that has no
subject yet, so its block is the `NOT_PLANNED` status, never a verdict.
Council rules on findings, never on verdicts, and its rulings are recorded for
the caller rather than applied: it closes nothing, and the split is resolved
by the convergence law and the repair phase, never by a third judge or by
electing a leg. A convergence-law stop convenes no council at all. A

[`plan`](../../plan/SKILL.md) source owns `binding_judge`: it is the caller's
declared disposition for a split that survives repair, bound into the plan
identity so a disagreeing caller argument refuses the traversal rather than
silently rebinding it. Validate never reads it as an override. A
selected factory receives intent through its own coordinator (for Gas City,
the Mayor; see the `using-gc` skill); the core hands over intent and reads
native state, and the factory's own reconciler creates, scales, and repairs
its sessions. Learn is an optional later consumer of verdict collections.
Specialist skills (standards, domain, test, refactor, security) advise; none
is a hard dependency or a lifecycle authority.

## Delegation

A lane receives the frozen intent reference and the established facts it
needs, never the orchestrator's full conversation history. A lane that cannot
proceed from the intent alone reports that the plan failed the fresh-context
test; padding it with chat transcript or opening another planning lane needs
explicit caller authorization. Lanes whose write scopes share a regen surface
(the same generated outputs, mirrors, or manifests) serialize; only lanes with
disjoint source scopes and disjoint regen surfaces run in parallel.

## Judgment separation

The context that authors a candidate cannot issue its binding PASS. Judge legs
read and judge; the orchestrating context fixes. Validate emits no WARN,
confidence, disposition, briefing learning, owner, next action, repair, retry,
replan, helper, escalation, tracker, Git, release, closure, or delivery state.
RPI and Validate reports end with the evidence; the caller owns continuation.
A wrong subject needs no second judge: a subject or digest mismatch is
`NOT_PROVEN` before any cross-family leg runs. Validate never asks the model
to reconstruct Plan or Candidate packets; identity, scope, and freshness come
from runtime receipts.

## Invariants the traversal keeps

- The runtime derives complete changed-path coverage, or Validate returns
  `NOT_PROVEN`; a proven change outside `write_scope` makes the verdict `FAIL`.
- `PASS` requires nonempty, distinct author and validator context ids plus an
  explicit freshness attestation.
- RPI never creates a parallel revision artifact and never selects the next
  work; the caller owns continuation.
- The interactive report keeps raw digests, schema fields, and exhaustive check
  lists out of the response unless an integrity failure makes one necessary.
- Report statuses are exactly `PASS | FAIL | NOT_PROVEN | NOT_PLANNED |
  NOT_BUILT`; the last two describe progress, never a semantic verdict.
- Bounded repair changes orchestration cost, never acceptance, exact identity,
  fail-closed scope, or validation authority.
- `not_checked` keeps its meaning across rounds: an unverified in-scope surface
  stays `NOT_PROVEN`, and no round may empty it to reach PASS.

## Incident appendix

Dated origins of the rules above. Read for the mechanism; the ids are not
resolvable outside the repository that recorded them.

- **2026-07-15, scope as a class.** A skill-fold intent enumerated its write
  scope as a path list. The regen command rewrote companions the author never
  listed, burning two implement lanes and three intent revisions before scope
  was restated as "the hand-edited sources plus every output of the owning
  regen commands". Plan's scope admission follows.
- **2026-07-15, a mutating check destroyed the subject.** A deterministic-gate
  script run mid-validation regenerated `skills-codex/` from HEAD and
  overwrote the uncommitted subject, forcing `NOT_PROVEN`; only restoring the
  subject and revalidating in a fresh context produced the PASS. Validate's
  mutating-check quarantine follows.
- **2026-07-28, the planning spiral.** Three days of planning and validation
  artifacts produced zero implementation commits. RPI's phase lock, spiral
  breaker, and subject-first reporting follow.
