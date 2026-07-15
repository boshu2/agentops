# Operating Loop

AgentOps is a semantic work-and-proof protocol, not CI, Git workflow, tracker,
queue, or autonomous controller.

```text
intent
  -> existing bead or caller source
  -> one bounded implementation experiment
  -> runtime-derived subject-manifest.v1 + check receipts
  -> one fresh independent validation
  -> durable verdict.v2
  -> report and stop
```

## Roles

| Role | Owns | Does not own |
|---|---|---|
| Caller | intent source, invocation, optional strategies, any later revision or delivery | semantic PASS unless acting in a fresh validator context |
| Plan | refining acceptance and write boundary in the existing source | a duplicate planning artifact, scheduling, ownership, readiness |
| Implement | one subject change and factual evidence | validation, repair loop, Git, closure, delivery |
| Validate | exact identity, independent judgment, durable verdict | subject edits, retries, next actions, release |
| RPI | one ordered dispatch and report | a controller around repeated invocations |

One model may fill multiple roles across distinct contexts. PASS requires
nonempty distinct author and validator context IDs and an explicit freshness
attestation. The attestation is a declared trust fact, not cryptographic process
isolation.

## Intent source

Plan shapes one active behavior in the caller-owned bead, issue, or conversation.
That source records:

- acceptance examples where they reduce ambiguity;
- non-goals and required evidence;
- `write_scope.include` and `write_scope.exclude`, including generated companions;
- a first acceptance command or artifact path;
- optional decomposition with no scheduling semantics.

The runtime stores the exact resolved source bytes under
`.agentops/intents/sha256/<digest>.intent` and derives the acceptance digest
from those bytes. This also makes conversation-only intent available to a fresh
validator. The model does not author a second PlanPacket.

Owner, ready, claim, priority, attempt, wave, queue, lease, admission, next
action, close, release, and delivery fields are outside the contract.

## One bounded experiment

Implement consumes the resolved intent once. A behavior change captures a
right-reason RED, makes the smallest coherent change that turns it GREEN, and
refactors under the unchanged acceptance check. Docs-only and pure-refactor
work record an honest pre-change baseline.

The runtime derives the author context, subject manifest, actual changed paths,
coverage fact, and check receipts. These facts can be passed directly to
Validate; the model does not transcribe a CandidatePacket. A failed check is
evidence, not loop authority.

## Content identity

`subject-manifest.v1` is independent of Git. It contains normalized relative
paths, file/symlink/deletion kinds, executable bits, content or target digests,
declared roots and exclusions, an optional base-manifest digest, and one
canonical manifest digest. Git commit/tree information may be attached as
read-only metadata.

The pure helper lives at `skills/validate/scripts/validate.py`. It makes no Git,
tracker, queue, network, release, or delivery call.

## Fresh Validate

Validate recomputes subject identity, confirms intent-source continuity and
complete changed-path coverage, compares actual changes with Plan scope, checks
the evidence, and judges every acceptance criterion.

- Proven out-of-scope change: `FAIL`.
- Incomplete path coverage, subject mutation, digest mismatch, missing/colliding
  identities, or missing freshness: `NOT_PROVEN`.
- Complete evidence satisfying every criterion, with nonempty checked scope and
  evidence references: `PASS`.

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

If a caller wants another experiment, it updates the existing bead or caller
intent and starts a new invocation. Prior verdicts and manifests remain durable
evidence, but AgentOps does not require a model-authored revision packet.
Changed acceptance is represented once in the intent source.

## Optional ports

- Premortem, Postmortem, Council, and genie skills are optional judgment
  strategies selected by the caller.
- `dispatch_once(explicit_disjoint_work, executor)` may dispatch explicit
  disjoint work exactly once. It does not select, queue, persist, retry,
  validate, integrate, close, or deliver.
- Learn may later inspect collections of durable verdicts. It cannot alter a
  verdict, plan, or core result.
- Consumer repository Git, CI, merge, rollback, and release mechanisms operate
  after and outside this loop.
