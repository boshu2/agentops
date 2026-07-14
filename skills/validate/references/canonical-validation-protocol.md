# Canonical Validation Protocol

This reference carries the detailed mode, target, and evidence rules for the
`validate` kernel. Load only the sections selected by the current mode.

## Mode details

- Default: one independent fresh-context judge; PASS requires that judge to
  return PASS. Additional judges are an explicit depth or council choice, not
  part of the default validation contract.
- `--quick`: inline structured review. It may guide work but is stamped
  `waived`, never independently validated.
- `--deep`: four perspectives—missing requirements, feasibility, scope, and
  specification completeness. PASS uses the declared majority rule, with any
  unresolved blocker still failing closed.
- `--mixed`: the same perspectives across explicitly selected model families.
- `--debate`: two adversarial rounds with critique and rebuttal.
- `--mode=post-impl`: complexity, bug sweep, acceptance roll-up, then isolated
  judges. A refactor that changes acceptance/unit test text is a new behavior
  slice, not a refactor.
- `--mode=pre-impl`: plan/spec checks, optionally specialized by `--target`.
- `--mode=pr`: upstream alignment first, contribution rules, atomic isolation,
  scope containment, then tests/lint.

The eight-mode budget is fixed. A ninth mode must replace or merge an existing
mode rather than growing the public surface.

## Pre-implementation targets

| Target | Required checks |
|---|---|
| default plan | temporal interrogation, error/rescue map, FAIL patterns, test pyramid, enum/input validation |
| scenario | holdout scenario and falsifying edge |
| fitness | each GOALS.md gate against current measured state |
| ratchet | current checkpoint, evidence, and legal next transition |
| scope | frozen paths, authority boundary, and escape prevention |
| skill | strict hygiene plus profile-aware deep audit |
| health | executable repository health probes and disclosed gaps |

A goal-design packet containing `intent.md` and `driver.md` first runs
`scripts/check-goal-design-packet.sh <packet-dir>`; nonzero is FAIL evidence.

## Post-implementation acceptance

Every Given/When/Then maps to a passing acceptance test for its own vertical
slice. Activity logs and parent-issue summaries do not close a bead. Apply the
Completion-Claim Kernel in `skills/shared/validation-contract.md` to every
DONE/closed/green claim.

The acceptance judge is blind and context-isolated. `judge_id == author_id`
cannot independently PASS. Inline fallback is marked waived and cannot satisfy
an assurance close.

## PR checks

Run in order:

1. upstream alignment (`git rev-list --count HEAD..origin/main`) and conflict risk;
2. repository contribution rules;
3. one thematic/atomic change shape;
4. scope-creep containment;
5. relevant tests and lint.

FAIL includes executable remediation such as rebase or split-by-type; it never
mutates the branch from the judge lane.

## Evidence discipline

### Deterministic request admission

Validation admission is a two-step portable protocol:

1. `validation-request.py freeze` accepts an explicit base and candidate and
   derives the candidate commit/tree, subtree trees, changed surfaces, and
   owned blob/deletion identities from Git. The subtree set is nonempty and
   unique; declared owned paths must exactly equal every changed-surface path,
   including deletions. It hashes acceptance, evidence, resolvable
   repository-relative claim references, the factual-gate registry, toolchain
   files, and the selected registry entries into a closed request. Both claim
   references and their digest projection are retained. The semantic identity
   is the SHA-256 of canonical JSON containing complete owned paths, acceptance
   digest, claim-dependency digests, and evidence-dependency digests.
2. `validation-request.py run` rechecks every frozen identity, atomically and
   durably reserves a canonical Git-common-dir run claim keyed by request ID and
   canonical-JSON request digest, then separately reserves the caller's receipt path. Changing
   `--output` cannot replay the request. It invokes every selected gate at most
   once at the candidate with its declared argument vector. Gate stdout must be
   one JSON object with status `PASS`, `FAIL`, `ERROR`, or `UNKNOWN`. The runner
   records output digests and facts; it does not interpret arbitrary prose as a
   factual result.
3. `validation-request.py check-receipt` binds a completed receipt back to its
   request and rejects missing gate executions, missing mandatory proof,
   contradictory authority, or attribution that does not match the exact-base
   result. The receipt schema carries the same fail-closed invariants so a
   portable consumer cannot authorize from a shape-only READY object.

Preflight fails closed before any gate or judge when the candidate moved, the
explicit base is not an ancestor, a claim reference or evidence file is absent
or stale, the registry is malformed, a selected entry changed,
backing/toolchain files are missing or stale, no mandatory lane is selected,
the author and validator identities match, or the route is not one fresh
validator. ERROR and UNKNOWN in any lane and mandatory FAIL forbid model spend.
For FAIL in any lane, the runner executes the same gate once more in a detached
worktree at the exact base and retains attribution. Only mandatory baseline
PASS promotes global REPAIR; only mandatory baseline FAIL promotes global NOTE.
Diagnostic and release FAIL remain nonbinding and release authorization stays
with its later owner. An inconclusive baseline blocks globally.

The exclusive canonical request claim is the durable at-most-once boundary.
Concurrent invocations, alternate receipt paths, completed claims, empty
reservations, and stale reservations all refuse without another gate. A crash
may leave HOLD evidence; the runner never claims automatic recovery or deletes
the claim. Final blocked or complete receipts replace their receipt-path
reservation atomically only after wire invariants validate, then the canonical
claim records the receipt digest and disposition.

The default green transition is `VALIDATE_SINGLE_FRESH`. Repository size,
skill count, changed-file count, or any other inventory count never selects
rigor or validator count. Risk and explicit mode selection are separate policy
inputs outside this request foundation.

### Persistent run-budget admission

`VALIDATE_SINGLE_FRESH` is a proposed transition until the persistent RPI run
governor durably admits it. Validate passes the frozen request, its factual
receipt, the unchanged run ID, and explicit reviewer-token, elapsed-time,
review-context, and deterministic-execution charges to
`validation-budget.py admit`. The adapter first verifies the factual receipt
against its request, then calls the `.17` governor's `semantic-review` port. It
does not initialize run state or reinterpret its counters.

The returned `schemas/validation-budget-receipt.v1.schema.json` artifact binds
request and factual-receipt digests to the requested charge and the governor's
admission ID, usage, disposition, reason, and helper fact. Only
`status:AUTHORIZED`, `validator_dispatch_allowed:true`, and
`next_action:VALIDATE_SINGLE_FRESH` authorize dispatch. A mandatory factual
`FAIL`, any `ERROR` or `UNKNOWN`, missing proof, or an unavailable meter stops
before the governor or judge. Diagnostic and release `FAIL` stay nonbinding:
the adapter consumes the aggregate S1/S8 authority fields and never duplicates
their lane classifier. Missing/corrupt state and governor refusal remain
nonauthorizing. A genuinely spent hard ceiling retains the typed
`hard-ceiling:<meter>` result and `helper_allowed:false`. An absent factual file
records availability `absent` with no invented digest; invalid JSON records
availability `invalid_json` plus the raw-file digest. Both produce a
schema-valid `NONAUTHORIZING` receipt without calling the governor.

This receipt is evidence about the one governor, not local budget state.
Validate creates no counter, helper allowance, or escalation transition and
cannot convert any refusal into WARN or PASS.

### Deterministic and semantic boundary

Factual registry entries declare exactly one `proof_kind`: `syntax`, `schema`,
`identity`, `paths`, `generated_drift`, `executable_assertion`, or
`evidence_integrity`. The request and receipt retain that kind so consumers can
audit what the command was allowed to prove. A registry entry cannot declare
semantic prose quality, usefulness, preferred wording, or reviewer judgment as
machine proof.

Missing or stale gate registration, entry identity, or backing is a typed
`registry_integrity` defect and stops before execution. It is not evidence that
the candidate failed. Deterministic gates may still verify that a semantic
verdict is present, schema-valid, independently authored, and bound to the
candidate; only the independent reviewer decides whether its reasoning is
correct.
An advisory semantic observation never becomes deterministic authority. It
never blocks delivery by being promoted into a strict prose score.

The closed wire formats are:

- `schemas/validation-candidate.v1.schema.json`
- `schemas/validation-gate-registry.v1.schema.json`
- `schemas/validation-request.v1.schema.json`
- `schemas/validation-receipt.v1.schema.json`
- `schemas/validation-budget-receipt.v1.schema.json`

The validator reruns cited commands on the actual artifact. It does not accept
the author's evidence file as proof. Every count, timing, commit, and pass rate
is pasted from captured output. Uncited figures fail until measured.

Corrections are appended as dated errata crediting the source measurement;
silently rewriting an evidence claim is forbidden.

Verdict text uses anchored lines:

```text
VERDICT: PASS

COMMANDS RUN:
judge=<validator_session> command=<command rerun by this judge>
<bounded output excerpts produced by this judge>
REASONS:
- each reason cites a command or artifact
```

Exactly one verdict is allowed. `COMMANDS RUN:` must include at least one
`judge=<validator_session> command=<command>` line, and the pinned author identity
must differ from that validator session for PASS. A judge that ran nothing is a
reader, not a verifier.

## Judge isolation

Every judge brief includes this exact clamp:

> READ-ONLY except writing your single verdict file at `<path>`. Do NOT commit,
> push, or run tracker/infra ops (git push, br/bd, dolt).

Register dispatch intent before spawning. Two validators accidentally assigned
to the same lane/bead are a dedup incident, not an independent quorum.

## Verdict boundary

Validation may use runtime-native fresh-context judges. The result is an
immutable proof artifact, not implementation, learning, retry, tracker, or
delivery authority. After the verdict is written, Validate returns it to the
caller and stops. Repository-specific publication policy remains outside this
skill.

## Failure handoff

WARN or FAIL identifies the owning producer, evidence, and one executable next
action. The caller decides whether to repair, re-plan, stop, or escalate;
Validate does not control that loop.
