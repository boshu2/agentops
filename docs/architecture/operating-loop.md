# Operating Loop

AgentOps is a semantic work-and-proof protocol, not CI, Git workflow, tracker,
queue, or autonomous controller.

```text
intent
  -> PlanPacket
  -> one bounded implementation experiment
  -> CandidatePacket + subject-manifest.v1
  -> one fresh independent validation
  -> durable verdict.v2
  -> report and stop
```

## Roles

| Role | Owns | Does not own |
|---|---|---|
| Caller | intent, invocation, optional strategies, any later revision or delivery | semantic PASS unless acting in a fresh validator context |
| Plan | acceptance and write boundary | scheduling, ownership, readiness, continuation |
| Implement | one subject change and factual evidence | validation, repair loop, Git, closure, delivery |
| Validate | exact identity, independent judgment, durable verdict | subject edits, retries, next actions, release |
| RPI | one ordered dispatch and report | a controller around repeated invocations |

One model may fill multiple roles across distinct contexts. PASS requires
nonempty distinct author and validator context IDs and an explicit freshness
attestation. The attestation is a declared trust fact, not cryptographic process
isolation.

## PlanPacket

Plan shapes one active behavior. It records:

- intent and acceptance digests;
- normal and edge Given/When/Then scenarios;
- non-goals and required evidence;
- `write_scope.include` and `write_scope.exclude`, including generated companions;
- a first acceptance command or artifact path;
- optional advisory decomposition with no scheduling semantics.

Owner, ready, claim, priority, attempt, wave, queue, lease, admission, next
action, close, release, and delivery fields are outside the contract.

## One bounded experiment

Implement consumes the exact PlanPacket once. A behavior change captures a
right-reason RED, makes the smallest coherent change that turns it GREEN, and
refactors under the unchanged acceptance check. Docs-only and pure-refactor
work record an honest pre-change baseline.

The CandidatePacket binds the Plan digest, acceptance digest, author context,
subject locator, manifest, actual changed paths, changed-path coverage fact,
and factual evidence. A failed check is evidence, not loop authority.

## Content identity

`subject-manifest.v1` is independent of Git. It contains normalized relative
paths, file/symlink/deletion kinds, executable bits, content or target digests,
declared roots and exclusions, an optional base-manifest digest, and one
canonical manifest digest. Git commit/tree information may be attached as
read-only metadata.

The pure helper lives at `skills/validate/scripts/validate.py`. It makes no Git,
tracker, queue, network, release, or delivery call.

## Fresh Validate

Validate recomputes subject identity, confirms acceptance continuity and
complete changed-path coverage, compares actual changes with Plan scope, checks
the evidence, and judges every acceptance criterion.

- Proven out-of-scope change: `FAIL`.
- Incomplete path coverage, subject mutation, digest mismatch, missing/colliding
  identities, or missing freshness: `NOT_PROVEN`.
- Complete evidence satisfying every criterion: `PASS`.

The verdict records criterion results, findings, evidence references, checked
and not-checked surfaces, timestamp, identities, freshness, and artifact digest.
It carries no WARN, confidence, disposition, learning, owner, next action,
retry, closure, release, or delivery state.

Validate alone persists verdicts. Default storage is
`.agentops/verdicts/sha256/<digest>.json`; a caller may provide `verdict_dir`.
The digest is SHA-256 over canonical JSON without `artifact_digest`. Writes are
same-directory, flushed, fsynced, and atomically renamed. Exact existing content
is idempotent. Conflicting content is an integrity failure and cannot produce
PASS. Provenance may record a verdict afterward, but ledger availability never
affects validity.

## Stop boundary and revision

RPI invokes Plan, Implement, and Validate at most once and then stops. A FAIL or
NOT_PROVEN report does not repair, replan, consult a helper, escalate, or invoke
a second phase. `NOT_PLANNED` and `NOT_BUILT` describe RPI progress only and are
not verdict values.

If a caller wants another experiment, it creates `revision-packet.v1` with the
prior verdict digest, prior/current manifest digests, unchanged acceptance
digest, responses keyed by finding ID, reusable exact-input evidence, and new
author context. The caller supplies that packet to a new invocation. Changed
acceptance is a new intent, not a revision.

## Optional ports

- Premortem, Postmortem, Council, and genie skills are optional judgment
  strategies selected by the caller.
- `dispatch_once(explicit_disjoint_packets, executor)` may dispatch explicit
  factory packets exactly once. It does not select, queue, persist, retry,
  validate, integrate, close, or deliver.
- Learn may later inspect collections of durable verdicts. It cannot alter a
  verdict, plan, or core result.
- Consumer repository Git, CI, merge, rollback, and release mechanisms operate
  after and outside this loop.
