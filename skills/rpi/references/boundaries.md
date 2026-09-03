# Ownership boundaries for the RPI core

Shared by `rpi`, `plan`, `implement`, `validate`, and the `anti-ceremony`
guard. Each kernel names the one step at which to read this file; the rules
below are the contract at that step, stated once.

## What each core skill owns

- **rpi** — the traversal: one guard call, at most one Plan and one Implement
  dispatch, fresh Validate, the bounded repair phase, and the report.
- **plan** — the shape of the intent inside the caller's own source: one
  active behavior, acceptance, non-goals, write scope, first check.
- **implement** — subject edits and factual check receipts.
- **validate** — one semantic result over one exact subject; the sole
  `verdict.v2` writer when persistence is requested.
- **anti-ceremony** — one admission judgment, `CONTINUE` or `STOP`.

## What none of them owns

The caller's tracker, repository policy, and runtime keep:

- retries, budgets, queues, claims, leases, and admission of further rounds —
  `repair_rounds` is the caller's declaration, never a core budget;
- Git: commit, push, land, reserve, rebase, merge, rollback;
- delivery, release, closure, and the caller's next decision or next work;
- lane budgets, wave selection, and the extension of any caller bound;
- tracker or delivery mutation as a side effect of a phase — parking,
  findings, and proposed amendments are fields in a response.

Facts the runtime derives — changed paths, subject manifest, digests, check
receipts, context identities — are read from the runtime, never transcribed
by the model into a packet. Plans, audits, reviews, dashboards, and prompts
are control artifacts: they earn no capability credit and are completion
subjects only when the caller explicitly asked for document review.

## Adapters and specialists

Premortem, Postmortem, Council, genie, factory, tracker, and runtime adapters
are caller-selected and leave phase order and core outcomes unchanged. A
selected factory receives intent through its own coordinator (for Gas City,
the Mayor — see the `using-gc` skill); the core hands over intent and reads
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
