# observation-log fixtures

Used by `tests/integration/test-aggregate-observation-log.sh`. Mirrors what
the `factory-claim-ledger-strict (advisory)` CI job emits per run:

| File | Purpose |
|------|---------|
| `obs-good-1.json` | PR run, verdict=pass, run_id=1234567890. Pairs with `obs-duplicate.json` to exercise dedup. |
| `obs-good-2.json` | Push-to-main run (`pr_number=null`), verdict=fail. Exercises the null-pr_number backfill path. |
| `obs-duplicate.json` | Same `run_id` as `obs-good-1.json`, different `surfaces_touched`. Tests `unique_by(.run_id)` dedup. |
| `obs-malformed-null-runid.json` | Has `run_id=null`. Aggregator MUST reject (M3 fix). |

The schema field set matches what `validate.yml` emits today (`run_id`,
`pr_number`, `verdict`, `surfaces_touched`, `timestamp`); `merged_anyway`
and `ledger_updated` are added by the aggregator's backfill phase.

`pr_number` is emitted as a string by `validate.yml` (via `--arg pr_number`)
which becomes a string in JSON. The aggregator reads it as-is.
