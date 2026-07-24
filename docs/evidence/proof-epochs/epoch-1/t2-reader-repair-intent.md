# T2 compiler and strict-reader repair intent

Date: 2026-07-24

Author context: `codex-root-t2-reader-repair-author-20260724`

Base commit: `d64609d63`

Parent intent:
`docs/evidence/proof-epochs/epoch-1/t2-intent.md`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Trigger

Two author-distinct cross-reviews of the integrated T2 subject found:

- the Go v4 catalog branch preserves every typed field but accepts semantic
  contracts that the source compiler rejects;
- the positive v4 round-trip fixture is not accepted by the source compiler,
  so it proves only self-consistency of an invalid payload;
- the probe receipt binds the command string and declared fixtures but not the
  repo-owned wrapper and test-harness bytes that decide PASS;
- the proof runner executes a broadly permitted command in the live repository
  without process-tree cleanup, bounded capture, or mutation isolation;
- hostile proof coverage is guarded by a minimum case count instead of an
  exact closed invariant inventory; and
- compile failures bypass the declared typed receipt and emit stderr only.

The transactional publisher was separately exercised on the integrated tree.
It stopped before mutation on five ignored `__pycache__` trees inside the
owned `skills-codex` target, as required for unknown preexisting entries. After
those transient caches were moved to a recoverable `/tmp` backup, check mode
reported zero unowned collisions, write mode published the six real drifts,
and a second check returned `CLEAN`. That behavior is accepted and is not part
of this repair.

## Intent

Close both sides of the shadow rail. Make the Go v4 catalog reader agree with
the source compiler on every semantic invariant that can be decided from
catalog bytes, make its positive fixture a real compiler-valid contract, and
make compiler proof execution content-bound, isolated, bounded, cleanup-aware,
and typed on failure. Keep filesystem existence and executable availability
checks in the source compiler, where repository context exists.

## Acceptance

### T2R-1 — Skill-scoped authority

The Go reader rejects `refine_intent` outside `plan`, `dispatch_phase` outside
`rpi`, `write_verdict` outside `validate`, and `transport` outside the runtime
layer. An `rpi` entry must declare `dispatch_phase`.

### T2R-2 — Effect and authority coupling

The Go reader enforces the compiler's mutating-effect set, receipt-required
effect set, cleanup-required effect set, and `mutate_subject` authorization
rules. Hostile tests prove every rejection for the intended cause.

### T2R-3 — Binding artifacts

A produced binding artifact requires both a non-null schema reference and a
non-null validator. Artifact names remain unique across consumed and produced
sets. Repository-path existence remains a compiler concern.

### T2R-4 — Trigger integrity

Trigger IDs are unique across all five families. Prompts and aliases cannot
collide after trim, case-fold, and whitespace normalization. Every alias
resolves to its owning skill, and a nearest neighbor cannot name the owning
skill. The reader does not invent a repository skill inventory when consuming
standalone bytes.

### T2R-5 — Hard dependency boundary

Only `rpi` may declare hard skill dependencies, and its set is exactly
`plan`, `implement`, and `validate`. Every other v4 entry has an empty set.

### T2R-6 — Producer/consumer compatibility

The positive v4 fixture uses a real canonical skill name and repository-valid
schema, validator, proof command, and fixture references. The source Python
compiler accepts its `contract_v3` with the fixture entry's name and
dependencies before the Go round-trip test may claim compatibility. Go still
round-trips every typed field without loss.

### T2R-7 — Stable strictness

The hostile matrix contains the eight cross-review probes: forbidden verdict
authority, mutation without authority, missing required receipt, unvalidated
binding output, normalized trigger collision, alias to another skill,
self-neighbor, and non-RPI hard dependency. Focused Go tests, race tests, vet,
the compiler tests, projection parity, and regeneration check pass on the
integrated subject.

### T2R-8 — Exact proof-harness closure

`contract_v3.proof` declares a nonempty `harness_refs` set in addition to its
fixture refs. The source compiler accepts only a repo-owned executable or an
approved interpreter followed by a repo-owned script; inline interpreter code
and unrestricted PATH commands fail closed. Compile and probe receipts bind the
command entrypoint, every harness ref, every fixture ref, and their exact
digests. Readiness recomputes those identities rather than trusting a stale
PASS receipt. The Go v4 branch preserves and validates `harness_refs`.

### T2R-9 — Isolated bounded execution and cleanup

The probe runs only in a disposable repository copy, never in the live source
tree. Stdout and stderr are drained while running under hard retained-byte
bounds with total-byte and truncation facts. Timeout or caller interruption
terminates the whole proof process group, escalates under a bound, waits for
reaping, and records cleanup outcome. The receipt reports isolated changed
paths and refuses PASS for out-of-scope or incomplete cleanup.

### T2R-10 — Exact hostile invariant inventory

One source-owned inventory names every closed-world schema and semantic
rejection branch. The hostile fixture corpus maps cases to that inventory
exactly; missing, duplicated, unknown, or threshold-only coverage fails.
Coverage includes all skill-scoped authority branches, cross-family trigger
IDs, proof containment and executable rules, hard-dependency shape and
cardinality, duplicate YAML/JSON keys, and source-mutation detection.

### T2R-11 — Typed compile failure

Both check and record modes emit a schema-valid typed compile receipt on
contract rejection. A FAIL receipt contains a nonempty typed error list,
identifies the skill and source facts available at the failure boundary, binds
the compiler and schema identities, and never fabricates unavailable contract
or fixture evidence. Human diagnostics may accompany it only on stderr; they
do not replace the receipt.

## Write scope

- `cli/internal/skills/**`;
- `skills/skill-builder/**` and the contract/receipt schemas it owns;
- this repair intent and later T2 evidence;
- generated projections only through their declared transactional owner.

## Non-goals

- Do not publish catalog v4 or make it live authority.
- Do not duplicate compiler-only filesystem existence or executable
  availability checks in the standalone Go byte reader.
- Do not execute arbitrary PATH tools or inline interpreter programs as proof
  harnesses.
- Do not alter v1, v2, or live v3 compatibility.
- Do not change any of the other 48 skill contracts.
- Do not repair or reinterpret the independent G0-G2 validation candidate.
