# Go G0-G2 finite-stdin cancellation test synchronization intent

Date: 2026-07-25

Author context:
`codex-go-g0-g2-finite-stdin-test-sync-author-20260725`

Base commit:
`aa99cf94f2f7375d081f5a8c76c7c5565702bef6`

Canonical base PASS verdict:
`.agents/ao/verdicts/sha256/fadb23c2bcce38aa02f2e59d01d400296dfc18bf7c5ee58a8799dcae9544e6f1.json`

Canonical base PASS verdict digest:
`fadb23c2bcce38aa02f2e59d01d400296dfc18bf7c5ee58a8799dcae9544e6f1`

FABLE program review digest:
`147f114a9950ec4033f7d1e7561d35a5d4efcfceacf6de30c7351e0deaf23453`

Parent repair intent:
`docs/evidence/proof-epochs/epoch-1/go-g0-g2-darwin-fast-exit-repair-intent.md`

## Intent

Strengthen the finite-stdin cancellation proof without changing production
behavior. The started-child case must establish that `Cmd.Start` succeeded
before cancellation, while a separate pre-start canceled-context case must
prove the distinct `CleanupNotStarted` outcome. This is a test-only
synchronization repair over the canonical passing subject.

## Acceptance

### GTS-1 — Started cancellation is causally synchronized

`TestRunStopsBlockedFiniteStdinRelayAfterCancellation` uses `Command.OnStart`
to observe a positive child PID and triggers cancellation only after that
callback proves the child exists. It requires `context.Canceled` and the exact
completed cleanup contract: status `CleanupCompleted`, attempted and completed
both true, and an empty cleanup diagnostic. It does not use a deadline as a
proxy for child start.

### GTS-2 — Pre-start cancellation is independently proven

A separate deterministic canceled-context test cancels before calling `Run`,
fails if `OnStart` is invoked, requires `context.Canceled`, and requires the
exact not-started cleanup contract: status `CleanupNotStarted`, attempted and
completed both false, and an empty cleanup diagnostic.

### GTS-3 — Production subject remains byte-identical

Every non-test file is byte-identical to base commit
`aa99cf94f2f7375d081f5a8c76c7c5565702bef6`. The implementation delta is
limited to focused lifecycle-test synchronization and makes no production,
dependency, generated-surface, schema, or contract change.

### GTS-4 — Deterministic regression evidence

The started-child cancellation test passes under high-repetition race
execution. The full subprocess race suite passes shuffled, followed by the
full shuffled test suite, full race suite, vet, pinned lint, and exact scope
and production-identity checks.

## First useful check

Because this is a pure test synchronization repair, record the honest green
baseline:

```bash
cd cli
go test -race -count=200 \
  -run '^TestRunStopsBlockedFiniteStdinRelayAfterCancellation$' \
  ./internal/subprocess
```

After the edit, repeat that check and inspect the test source to prove
cancellation is causally downstream of `OnStart`.

## Write scope

- Planning: this intent file only.
- Implementation: `cli/internal/subprocess/run_test.go` only.
- Runtime-owned content-addressed intent, manifest, scope-index, receipt, and
  report stores may be derived but are not part of the tracked candidate.

## Non-goals

- Do not modify production source.
- Do not reinterpret, revise, or overwrite the canonical PASS or either prior
  FAIL verdict.
- Do not change subprocess lifecycle, cleanup semantics, error identity,
  process-group behavior, bounded I/O behavior, dependencies, or supported
  platform policy.
- Do not issue the later binding verdict from the author context.
