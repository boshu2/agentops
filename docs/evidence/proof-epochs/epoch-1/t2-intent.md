# T2 intent — shadow skill contract compiler and recoverable publication

Date: 2026-07-24

Author context: `codex-root-t2-author-20260724`

Base commit: `ae2086fc1fd1bba3e9f7888d3e1f1ec5e6971a49`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Intent

Make `skill-builder` the single compiler and publication owner for the next
skill contract without changing live catalog authority. T2 must add a strict,
closed-world `skill-contract.v3` shadow rail, strict Go readers for catalog
versions 1 through 4, a truthful 49-row migration-readiness ledger, and a
recoverable transactional publisher for shared generated projections.

This is one infrastructure experiment. It changes the grammar and publication
mechanism needed by later migration tranches; it does not migrate those
tranches.

## Acceptance

### T2-1 — Closed semantic grammar

One source-owned compiler validates a closed `metadata.contract_v3` shape with:

- exactly one primary layer from `product | campaign | experiment | judgment |
  evidence | implementation | evolution | runtime | support`;
- zero or more legal lifecycle seams from `product_input | goal_design |
  goal_observe | option_shaping | plan_input | plan_review | implement_method |
  validate_evidence | validate_strategy | post_verdict | runtime_transport |
  cross_cutting | standalone`;
- authority verbs restricted to `advise | read_evidence | refine_intent |
  mutate_subject | dispatch_phase | write_verdict | transport`;
- structured effects carrying kind, scope, authorization, cleanup, and receipt
  semantics;
- typed consumed and produced artifacts;
- positive, negative, ambiguity, alias, and nearest-neighbor trigger fixtures;
- explicit unavailable, timeout, partial-evidence, partial-mutation, and
  cleanup failure behavior; and
- a proof class, executable probe command, and fixture references.

Unknown fields, duplicate set members, forbidden authority, malformed effects,
bad artifacts, incomplete trigger families, incomplete failure semantics, and
invalid proof declarations fail closed. Only `rpi` may hard-depend on `plan`,
`implement`, and `validate`; no contract may acquire experiment-selection or
campaign-continuation authority.

### T2-2 — Shadow-only authority

Legacy API1 `SKILL.md` metadata and generated `skill-catalog.v3` remain the
sole live authority. The compiler reads `metadata.contract_v3` only as a
shadow readiness rail. T2 does not publish catalog v4, advertise new routing,
or make any later tranche's contract authoritative.

### T2-3 — Skill-builder is the only newly ready skill

`skills/skill-builder/SKILL.md` gains a truthful `metadata.contract_v3`
declaration and its behavior is narrowed to:

- compile or audit one semantic source contract;
- create or heal an explicitly named source package;
- publish owned projections transactionally when explicitly invoked; and
- emit typed build, audit, compile, or publication receipts.

It owns no scheduling, Git, validation verdict, retry, campaign, work
selection, or post-failure continuation.

### T2-4 — Complete readiness ledger

A machine-readable ledger contains exactly one row for each of the 49
canonical skills. Every row binds the source path and digest, target tranche,
contract/compiler status, probe status and receipt, proof/verdict identity when
present, blockers, and `cutover_ready`. Only `skill-builder` is
`cutover_ready: true` at T2 close; the other 48 rows remain explicit rather
than inferred from the dated plan.

### T2-5 — Strict catalog readers

The Go catalog loader dispatches explicitly by schema/catalog version and
supports the declared legacy v1, v2, and v3 envelopes plus shadow v4 fixtures.
Every branch rejects duplicate JSON object keys, duplicate skill names,
unknown fields, trailing JSON values, invalid version/type combinations,
count mismatches, and malformed version-specific entries. The v4 branch
preserves all typed contract fields rather than silently dropping them.

### T2-6 — Recoverable transactional publication

One owner map declares every shared generated projection and its source owner.
The publisher:

1. serializes publication;
2. classifies every target as `CLEAN_CURRENT | DIRTY_PRESERVE | MISSING |
   UNOWNED`;
3. snapshots exact bytes, mode, kind, and symlink target before mutation;
4. renders the complete owned set into staging;
5. proves check/write byte parity and idempotence;
6. replaces owned live paths and writes the publication manifest last; and
7. restores the exact pre-run state on injected failure or abnormal exit.

Unowned collisions abort before mutation. Recovery never reads bytes from Git
or overwrites unrelated dirty state. Fault-injection tests cover dirty regular
files, executable modes, symlinks, missing targets, manifest-last ordering,
and restoration after partial publication.

### T2-7 — Honest receipts and behavioral proof

Compiler and publisher receipts bind exact input, before, rendered, after, and
owner-map identities as applicable. Hostile fixtures prove every closed-world
rejection and every recovery branch for the intended cause. Check mode is
read-only; write mode emits identical bytes to checked staging output.

### T2-8 — Safe resting state

All focused Python, shell, and Go tests pass; full Go tests, race tests, vet,
the pinned lint wrapper, regeneration parity, skill mesh, structural floor,
Codex projection checks, and the active proof-contract checker remain green.
T2 closes with an exact subject manifest, a fresh semantic verdict under the
active epoch-1 proof contract, a completion-ledger row, a plan status update,
and a fresh pause drill.

## Write scope

Allowed:

- `skills/skill-builder/**`;
- new skill-contract, readiness, owner-map, publication, and receipt schemas;
- compiler, publisher, generated-owner, and focused test sources;
- `cli/internal/skills/**` and versioned catalog fixtures/tests;
- T2 evidence under `docs/evidence/proof-epochs/epoch-1/**`;
- the migration plan, architecture contract, and completion ledger at close;
- generated projections only through the transactional owner.

## Non-goals

- Do not migrate or rewrite the other 48 skills.
- Do not publish `skill-catalog.v4` as live authority.
- Do not rename `goals`, retire `shared`, or change installed skill roots.
- Do not add experiment selection, retry, queue, Git, release, or delivery
  authority.
- Do not validate G0 through G2 semantically in this experiment.
- Do not touch `/Users/bo/dev/agentops` or use it as a restore source.

## First useful checks

```bash
bash skills/skill-builder/scripts/heal.sh --check --strict skills/skill-builder
bash skills/skill-builder/scripts/audit.sh --strict skills/skill-builder
go test ./internal/skills/... ./internal/projections/...
bash scripts/regen-all.sh --check
python3 scripts/check-proof-contract.py
```

If compiler/catalog work and publisher recovery cannot share one exact subject
without obscuring independent failure, freeze the same intent into two
separately validated candidates before either can claim T2 complete. T2 itself
remains incomplete until both candidates PASS and the closeout evidence binds
their identities.
