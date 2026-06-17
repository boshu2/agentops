# age-zqc pre-push pawl evidence — escape→finding→membrane-check wire

**Bead:** age-zqc — Wire escape→finding→membrane-check derivation (epic age-cwo, the self-improving membrane).
**Mode:** fresh-context (default). Author context: `age-zqc-claude-main`. Refuter context: a fresh-context subagent (no shared accumulated context), distinct invocation.

## What landed

- `yieldledger.DetectEscapes(l, runID)` — pure read over the yield ledger. An escape = a CONFIRMED gate-verdict for a bead later REFUTED at a strictly higher attempt. Run-scoped; v1 one-escape-per-bead (the escape_rate gauge age-6ty owns finer accounting).
- `ao membrane derive-checks --run <id> [--dry-run] [--force]` — detects escapes, derives a finding (the fresh-context re-verification check that would have caught it), compiles it into a pre-mortem membrane check via the existing `FindingCompilerPort`, and writes `.agents/findings/<id>.md` + `.agents/pre-mortem-checks/<id>.md`.

## Adversarial review — round 1 (REFUTED)

A fresh-context reviewer red-teamed the diff and **REFUTED**, finding two real defects (both in the author's self-flagged weak spots):

1. **Partial-artifact fail-quiet** — `writeDerivedArtifacts` gated the whole write on the *finding* file's existence, so a deleted/missing compiled check was never repaired: the command reported `[skipped]` while the load-bearing membrane check stayed absent.
2. **Cross-run ID collision** — `deriveEscapeFindingID` was `escape-<bead>-<confSHA>` with no run component, while escapes are run-scoped. The same bead confirmed at the same head in two distinct runs collapsed to one artifact, silently dropping the second escape (under-counting the escape corpus — the compounding asset).

Test fidelity was confirmed good (fixtures built via the production `Writer` round-trip), but neither hole had a guarding test, which is why they shipped green.

## Fixes + round 2 (CONFIRMED)

- `writeDerivedArtifacts` now writes each target independently — a missing target is repaired even when siblings exist; existing targets are skipped (idempotent); `--force` overwrites. Atomic temp+rename (no observable partial body).
- `deriveEscapeFindingID` now keys on `run + bead + confirmed-sha + refuted-sha` (sanitized run id — no path traversal; each SHA capped at 12 chars — bounded id).
- Two new regression tests: `TestMembraneDeriveChecks_RepairsMissingCheck` and `TestMembraneDeriveChecks_CrossRunNoCollision`.

The fresh-context reviewer re-verified and **CONFIRMED**: it proved each new test is real by reverting each fix and observing the test fail, then passing with the fix restored. No regression; no new defect (run-id sanitization, bounded id, accurate `wroteAny` error path all checked).

## Deterministic evidence

```
go build ./...                                              # clean
go vet ./cmd/ao/ ./internal/yieldledger/                   # clean
go test ./internal/yieldledger/ ./cmd/ao/ -run 'Escape|Membrane'  # ok
ao gate check --fast --scope head                          # 31/31 pass, 0 fail
```

Live smoke (isolated temp project, real built binary): emitted CONFIRMED@1 + REFUTED@2 gate-verdicts → `ao membrane derive-checks` detected 1 escape → wrote the finding + the compiled pre-mortem check; dry-run wrote nothing; second run was idempotent (`[exists, skipped]`).
