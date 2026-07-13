# Snapshot Pattern For Long-Cycle Gates

Gates that depend on **multi-session corpus state** (flywheel
compounding, corpus freshness, knowledge-registry health, AOP-claim
coverage) cannot be evaluated by a single CI run on a greenfield
checkout — the corpus isn't there. The 4-step **snapshot pattern**
converts these into single-commit-validatable artifacts: the live gate
computes the metric on demand, the operator snapshots it to a
git-tracked JSON file with a staleness window, and the CI gate
validates the snapshot file rather than re-running the live metric.

This is how disk-is-truth (see long-loop-discipline (disk-is-truth axiom))
extends into CI: the **disk file is the gate's truth surface**, and
the live metric only refreshes it.

## The 4-Step Pattern

### 1. Live Gate (Operator-Local)

A script that computes the multi-session metric on demand. Runs in
the operator's full corpus, not in CI.

```bash
# Example: live gate for flywheel compounding
bash scripts/check-flywheel-compounding.sh
# Internally: ao flywheel status → exits 0 if compounding, 1 if not
```

### 2. Snapshot Command (Operator Cadence)

Wraps the live result in a JSON envelope with `recorded_at`,
`git_sha`, and the metric payload. Writes to a git-tracked path.

```bash
# Example: snapshot the live result
bash scripts/snapshot-flywheel-compounding.sh
# Writes docs/releases/flywheel-compounding-snapshot.json
```

The snapshot file is **the durable proof**. Operator runs the
snapshot script on cadence (e.g., before each release, or weekly).

### 3. CI Gate (Snapshot Validation)

CI does **not** re-run the live metric (it can't — no corpus). It
validates the snapshot file:

- Structural shape (required keys present)
- Staleness threshold (e.g., < 14 days old)
- Asserted value (`compounding == true`, `freshness_score >= N`)

```bash
# Example: CI gate validates the snapshot
bash scripts/check-flywheel-compounding-snapshot.sh
# Fails if snapshot missing | > 14 days old | shows metric flipped
```

### 4. Wire As A Regular Gate

- `GOALS.md` row naming the metric
- `.github/workflows/validate.yml` CI job
- `scripts/pre-push-gate.sh` lane invocation
- `AGENTS.md` "CI Jobs and What They Check" row
- (per the parity-surface inventory — see
  `docs/learnings/2026-05-12-parity-surface-inventory-grew-from-4-to-7-across-cycles-64-70.md`)

Once wired, the snapshot pattern looks like a regular gate; the
operator-cadence-refresh is the only non-standard piece.

## Why This Matters

Without this pattern, long-cycle gates stay in the Roadmap column
forever because **CI can't run them green** — a clean CI runner has
no corpus, no flywheel history, no AOP-claim accumulation. Three
GOALS.md gates closed in one /evolve session because of this pattern
(G1 flywheel-snapshot, D11 corpus-freshness, D10 workbench-delta).

The pattern is the implementation of long-loop-discipline (disk-is-truth axiom)
at the CI layer: the disk file is the load-bearing state, the live
metric is the refresh path, and CI consumes the disk.

## Evidence (anchored)

> "Gates that depend on multi-session corpus state (e.g., flywheel
> compounding, corpus freshness, knowledge registry health) cannot be
> evaluated by a single CI run on a greenfield checkout — the corpus
> isn't there. … The snapshot file is the durable proof. The CI gate
> ensures the operator refreshes it on cadence. Each surface is
> testable on a clean CI runner without N sessions of warmup."
— `.agents/learnings/2026-05-11-quick-snapshot-pattern-for-long-cycle-gates.md`
(retro-quick, validate phase)

> "Applications validated this session: corpus snapshot (D11, cycle
> 22), flywheel compounding (G1, cycle 24), workbench delta (D10,
> cycle 26), README claim manifest (PG4, cycle 30), AOP-CLAIM
> all-claims map (cycle 35). The structural-floor contract gate
> (cycle 41) is the generalization — same pattern at the corpus level."
— same source

## How To Apply (When Adding A New Long-Cycle Gate)

1. **Identify the metric.** Does it need a corpus, history, or
   multi-session state to compute? If yes, snapshot pattern. If no, a
   regular CI gate suffices.

2. **Write the live gate** under `scripts/check-<metric>.sh`. Exit 0
   on success, non-zero on regression. This runs in the operator's
   environment.

3. **Write the snapshot command** under `scripts/snapshot-<metric>.sh`.
   Output JSON to a git-tracked path:
   ```json
   {
     "recorded_at": "2026-05-18T19:42:00Z",
     "git_sha": "abc1234",
     "metric_name": "flywheel_compounding",
     "value": true,
     "details": { /* metric-specific payload */ }
   }
   ```

4. **Write the snapshot validator** under
   `scripts/check-<metric>-snapshot.sh`. Read the JSON, assert shape +
   staleness + value.

5. **Wire all 7 parity surfaces** per the parity-surface inventory:
   pre-push lane, validate.yml job, summary.needs[], summary echo,
   AGENTS.md row, registry.json (if a new hook), bats stub (if pre-push).

6. **Set a refresh cadence.** Document in `AGENTS.md` or `GOALS.md`
   when the operator runs the snapshot script (before release,
   weekly, before-each-evolve-session).

## Failure Modes

- **Skipping step 4 (snapshot validator).** Without the validator,
  the snapshot file drifts undetected — operators stop refreshing it,
  and CI claims green forever. The validator forces the cadence.
- **Snapshot in chat / ephemeral state.** The snapshot MUST be
  git-tracked. A snapshot in a session JSONL is invisible to CI.
- **Staleness window too long.** > 30 days means the gate
  effectively never fails. Tighten to 7-14 days.
- **Live gate and snapshot validator disagree on shape.** Schema
  drift between the two scripts; CI passes but the metric is wrong.
  Lock the JSON shape in the snapshot command's documentation.

## Applications (Empirical Reference)

The pattern shipped 6 times across cycles 22-41 of the 2026-05-11
evolution-roadmap drain:

| Cycle | Metric | Snapshot path |
|---|---|---|
| 22 (D11) | corpus freshness | `docs/releases/corpus-snapshot.json` |
| 24 (G1) | flywheel compounding | `docs/releases/flywheel-compounding-snapshot.json` |
| 26 (D10) | workbench delta | `docs/releases/workbench-delta-snapshot.json` |
| 30 (PG4) | README claim manifest | `docs/releases/readme-claim-manifest.json` |
| 35 | AOP-CLAIM all-claims map | `docs/releases/aop-claims-all-snapshot.json` |
| 41 | structural-floor contract gate (the generalization) | per-contract snapshot |

Reuse-rate after first ship: 5 reuses in 19 cycles. The pattern is
durable.

## See Also

- long-loop-discipline (disk-is-truth axiom) — the principle this
  pattern implements at the CI layer (sibling reference in this dir,
  landing concurrently)
- [cycle-history.md](cycle-history.md) — the JSONL ledger pattern
  (same disk-truth principle, different consumer)
- [gate-hygiene.md](gate-hygiene.md) — gate-output parsing; complements
  snapshot validation
- `docs/learnings/2026-05-12-parity-surface-inventory-grew-from-4-to-7-across-cycles-64-70.md`
  — the 9-surface checklist for wiring any new gate, including
  snapshot gates
