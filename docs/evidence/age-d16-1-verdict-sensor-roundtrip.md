# Evidence — M1: verdict sensor fires end-to-end (age-d16-self-hosting-route-nkr.1)

**Scope:** PROVE the verdict provenance sensor round-trips end-to-end; fix only what blocks it. Non-goal: new emitter surface / new ledger schema.

## Verdict: PROVEN. No production code change was needed — the round-trip worked on first exercise.

The ledger had **0 verdict rows** not because the path is broken, but because the producer (`pawl-verdict.sh write`) had **never been run** — no pawl-verdict artifact existed anywhere in the repo (only the schema). The sensor was correctly wired all along.

## Live demonstration (2026-06-16, worktree `fix/age-d16-1-verdict-sensor-roundtrip`)

Drove one real verdict through the **full existing producer→sensor path**, using the real `origin/main` HEAD SHA per the footgun (the gate rewrites landed SHAs; the verdict edge must bind the SHA that actually lands):

```
$ pawl-verdict.sh write age-d16-self-hosting-route-nkr.1 0 \
    --disposition CONFIRMED --head 611615d9b78717eca0fa1b2d1eb75a54c9dc6970 \
    --author-context m1-proof-author --refuter claude:CONFIRMED:m1-proof-refuter
pawl-verdict: wrote .../age-d16-self-hosting-route-nkr.1.json (disposition=CONFIRMED)
emitted age-d16-self-hosting-route-nkr.1@611615d --wasDerivedFrom--> 611615d   # auto-fired sensor
```

- **Scenario 1 (row appended):** `docs/provenance/ledger.jsonl` grew 23 → **24**; the appended row is a schema-valid, hash-chained `verdict --wasDerivedFrom--> commit` edge whose `to_id` is the full real SHA.
- **Scenario 2 (reader maps it back):** `ao provenance verify` → `OK: provenance ledger chain intact (24 record(s))`; a parser mapped the edge → `verdict age-d16-self-hosting-route-nkr.1 --wasDerivedFrom--> commit 611615d (CONFIRMED)`.
- **Idempotency:** re-emitting the same artifact → `already present (idempotent no-op)`; row count stayed 1.

The probe row was **reverted** from the committed ledger — the audit authority must not carry a synthetic proof verdict. Real verdict rows flow in production via M3 (the gate writing the binding verdict).

## What this arc ships (regression locks, not new surface)

- `cli/cmd/ao/provenance_emit_verdict_test.go` → `TestEmitVerdict_ConsumesRealProducerArtifact`: the sensor consumes the producer's **real on-disk shape** (fixture = the actual `pawl-verdict.sh write` output), ignoring its extra fields while extracting bead_id/head_sha/disposition, and appends a chain-intact edge whose `to_id` is the full head_sha. Guards the producer/sensor **schema-drift** that would silently return the feed to 0 rows.
- `tests/scripts/pawl-verdict-provenance-roundtrip.bats`: the producer emits a schema-shaped artifact **and fires the sensor** against it (the wiring). Hermetic (`ao` stubbed).
- `tests/fixtures/provenance/pawl-verdict-real-sample.json`: the real producer artifact, as fixture (`pr: 0` is the probe value — the real producer emits whatever PR it's given; it does not strict-validate against `pawl-verdict.v1`, whose `pr` minimum is 1. The sensor ignores `pr` entirely).

## Finding for M3 (age-d16-self-hosting-route-nkr.4) — silent-swallow

`scripts/pawl-verdict.sh:399-400` fires the sensor as `command -v ao >/dev/null && ao provenance emit-verdict --file "$out" 2>/dev/null || true`. In production the verdict feed **silently dies** if `ao` is not on PATH at landing time or the emit errors — no row, no warning. M1 leaves this as-is (scope = prove the round-trip, not harden the landing path). M3 ("prove the gate writes the binding verdict") should decide whether the emit must be observable/blocking so a dead feed surfaces instead of recurring as 0 rows.
