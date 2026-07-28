# Go G0-G2 Darwin fast-exit repair intent

Date: 2026-07-24

Author context: `codex-root-go-g0-g2-darwin-fast-exit-repair-author-20260724`

Base commit: `c4b7b2dafce039fa42c2120f4decef5195d3fbc4`

Parent repair intent:
`docs/evidence/proof-epochs/epoch-1/go-g0-g2-cleanup-repair-intent.md`

Prior terminal verdict:
`docs/evidence/proof-epochs/epoch-1/verdicts/8a1d6d59f33df856825f9b6ce04e567dfa0e43fb1e6412e709daa6a20456eb47.json`

Prior frozen subject digest:
`c6c5ba4cc071eddc1c0f0d470a4a6a0e023f9ed5f5a83b3b1fd2538ec904f30b`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Intent

Repair the one fresh-validator regression without changing the already-passing
G0-G2 behavior. A fast successful Darwin child must remain observable without
being reaped even when it exits before the first completion probe, and the
runner must finish process-group cleanup while the original process identity
is still reserved. A legitimate `exit 0` must never become a cleanup failure
because kqueue registration lost the child or a reaped PID/process-group ID was
reused.

## Acceptance

### GDR-1 — Race-free non-reaping completion

Darwin completion observation uses a non-reaping primitive that can observe a
child that exited before the first probe. Its native ABI assumptions are
explicitly represented and tested. Completion observation never calls
`Cmd.Wait`; unsupported or opaque observation fails closed.

### GDR-2 — Identity-preserving cleanup order

After root completion is proven, process-tree termination and absence
verification run before `Cmd.Wait` releases the root PID. Natural completion,
cancellation, timeout, injected cleanup failure, and unproven termination each
retain their existing error identities and perform at most one `Cmd.Wait`.
No cleanup operation signals a process group by a PID that has already been
reaped.

### GDR-3 — Honest Darwin group absence

Darwin distinguishes an exited zombie-only owned process group from a group
with a live member. A zombie-only group may complete cleanup; a live or
unobservable member remains a cleanup failure. `EPERM` alone is never
reinterpreted as absence.

### GDR-4 — Behavioral regression proof

A concurrent fast-`exit 0` regression test fails the prior exact subject with
the recorded `ESRCH`/`EPERM` outcome and passes repeatedly after repair.
Deterministic tests prove the completion ABI, cleanup-before-wait ordering,
zombie-only completion, live-member refusal, cancellation, descendants,
bounded I/O, and cleanup error propagation.

### GDR-5 — No regression and fresh judgment

The original GO-1 through GO-7 and GOR-1 through GOR-5 behavior remains green.
Focused and shuffled stress tests, race and goleak tests, full Go tests, vet,
pinned lint, regeneration parity, and Darwin, Linux, and Windows compile/vet
checks pass. A fresh author-distinct validator judges the exact repair subject;
both prior FAIL verdicts remain immutable.

## Write scope

- `cli/internal/subprocess/**`;
- `cli/go.mod` and `cli/go.sum` only for the platform syscall dependency;
- focused lifecycle tests whose synchronization was proven racy by the stress
  check;
- this repair intent and later exact validation evidence.

## Non-goals

- Do not reinterpret or overwrite either prior FAIL.
- Do not change identifier containment, temporary-root ownership, dry-run
  policy, output negotiation, catalog-reader behavior, or skill semantics.
- Do not broaden subprocess cleanup into retry, release, Git, or delivery
  policy.
- Do not claim Linux or Windows runtime execution from this macOS host.
