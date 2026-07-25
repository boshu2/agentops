# Go G0-G2 cleanup-outcome repair intent

Date: 2026-07-24

Author context: `codex-root-go-g0-g2-cleanup-repair-author-20260724`

Base commit: `7af1f9608`

Parent intent:
`docs/evidence/proof-epochs/epoch-1/go-g0-g2-intent.md`

Prior terminal verdict:
`docs/evidence/proof-epochs/epoch-1/verdicts/83cb2e9095d61bc14a00bee250215367bd9697bd05ff56fe384b662965752680.json`

Active proof contract:
`f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`

## Intent

Repair the one fresh-validator finding without changing the already-passing
G0-G2 behavior. Make process-tree cleanup an explicit, stable result fact and
preserve cleanup failure alongside cancellation, timeout, abnormal exit, and
wait errors rather than discarding it.

## Acceptance

### GOR-1 — Typed cleanup outcome

`subprocess.Result` contains a typed cleanup outcome that distinguishes not
started, completed, and failed cleanup. When a process starts, every return
path records whether tree cleanup was attempted, whether it completed, and a
bounded diagnostic when it failed. Start failure remains explicitly
not-started.

### GOR-2 — Error preservation

Context cancellation, deadline, wait failure, nonzero exit, and cleanup failure
retain their identities. Cleanup failure is joined with the primary terminal
error rather than shadowing it or being shadowed by it. Successful parent exit
with failed cleanup is an error.

### GOR-3 — Caller-visible propagation

Every migrated eval, gate, goal, and adapter call path that returns or
serializes subprocess facts preserves the relevant cleanup outcome. Structured
operation results expose cleanup state without adding a human preamble.
Callers that intentionally collapse details document and test that boundary.

### GOR-4 — Behavioral proof

Deterministic tests cover start failure, ordinary success, nonzero exit,
timeout, caller cancellation, descendant cleanup, successful parent with
background descendant, injected cleanup failure, and combined primary plus
cleanup failure. Tests assert both returned error identity and cleanup facts.
POSIX descendant behavior runs locally; Windows source has deterministic unit
coverage and cross-compiles but is not claimed as behaviorally executed.

### GOR-5 — No regression

All original GO-1 through GO-4 and GO-7 checks remain green. Focused tests,
full Go tests, race, vet, pinned lint, regeneration parity, and Windows
cross-compilation pass. A fresh author-distinct validator judges the exact
repair subject; the prior FAIL remains immutable.

## Write scope

- `cli/internal/subprocess/**`;
- migrated callers that must propagate the typed cleanup outcome;
- focused tests and generated CLI reference only if the public wire changes;
- this repair intent and later exact validation evidence.

## Non-goals

- Do not reinterpret or overwrite the prior FAIL.
- Do not change identifier containment, temporary-root ownership, dry-run
  policy, output negotiation, or catalog-reader behavior.
- Do not expand cleanup reporting into retry, release, Git, or delivery policy.
- Do not claim Windows behavioral execution from this macOS host.
