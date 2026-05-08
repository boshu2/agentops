---
id: reconcile-2026-05-08-promotion-decision
type: promotion-decision
date: 2026-05-08
epic: soc-xlw8
related_epic: soc-e4ulx
related_bead: soc-f42z9
status: deferred
decision: option-b-defer
sample_size: 3
sample_threshold: 20
distinct_prs: 2
distinct_prs_threshold: 3
re_evaluate_when: sample_size >= 20 AND distinct_prs >= 3
---

# Factory Claim Ledger — Promotion Decision

> **Deferred 2026-05-08.** Sample size 3 < 20 threshold; distinct PRs 2 < 3 threshold.
> Re-run aggregator + this baseline section once advisory CI runs of
> `factory-claim-ledger-strict (advisory)` accumulate to ≥20 across ≥3 PRs.
> Aggregator: `scripts/aggregate-observation-log.sh`.

## How to compute the baseline

Run the aggregator first to refresh the log:

```bash
bash scripts/aggregate-observation-log.sh
```

Then compute false-positive metrics with `jq`:

```bash
LOG=.agents/reconcile/observation-log.jsonl

# Total observations
jq -s 'length' "$LOG"

# Pass / fail breakdown
jq -s 'group_by(.verdict) | map({verdict: .[0].verdict, count: length})' "$LOG"

# False-positive count: verdict=fail AND merged_anyway=true (a maintainer
# overrode the validator, evidence the failure was not blocking-worthy).
jq -s '[.[] | select(.verdict == "fail" and .merged_anyway == true)] | length' "$LOG"

# False-positive rate (FP / total fails). 0 means every fail was a real fail.
jq -s '
  ([.[] | select(.verdict == "fail")] | length) as $fails
  | ([.[] | select(.verdict == "fail" and .merged_anyway == true)] | length) as $fp
  | if $fails == 0 then 0 else ($fp / $fails) end
' "$LOG"

# Ledger-updated count: fail observations whose merge commit also touched
# docs/contracts/factory-claim-ledger.example.json (legitimate ledger fix
# after a real fail).
jq -s '[.[] | select(.verdict == "fail" and .ledger_updated == true)] | length' "$LOG"
```

## False-Positive Rate Baseline

Computed 2026-05-08 from `.agents/reconcile/observation-log.jsonl` after
running `scripts/aggregate-observation-log.sh` (read 200 most recent
`validate.yml` runs; aggregated 3 unique observations).

| Metric | Value | Source |
|--------|-------|--------|
| Total observations | **3** | `jq -s 'length' "$LOG"` |
| Pass count | 3 | group_by query |
| Fail count | 0 | group_by query |
| `merged_anyway=true` count | 0 | FP query |
| `ledger_updated=true` count | 0 (over `verdict=fail`) | ledger query |
| **False-positive rate** | **0** (vacuous — denominator is 0 fails) | FP / fail-count |
| Distinct PRs | 2 (`#264`, `#265`) | `jq -s '[.[].pr_number] | unique \| length'` |

Raw observations are timestamped 2026-05-08T02:49Z, 02:51Z, and 06:49Z —
all clustered within ~4 hours of the aggregator landing (commit
`af681e9c`, 2026-05-08). The sample reflects the workflow's first day
in production.

## Promotion Decision — **Option B (defer / extend sampling)**

The skeleton's promotion thresholds are not met:

- ✗ Sample size **3 < 20**
- ✗ Distinct PRs **2 < 3**
- ✓ FP rate 0 (but vacuous — no fail observations to compute against)
- ✗ Cannot demonstrate behavior under fail conditions

Vacuous-FP-rate alone does not justify promotion: with zero fail samples
we have no empirical evidence about the validator's false-positive
behavior. A first real fail post-promotion could block an unrelated PR
with no demotion runway. Defer.

**Re-evaluation trigger:** When all of the following hold, re-run the
aggregator and recompute this baseline:

- [ ] Sample size ≥ 20 advisory runs
- [ ] ≥ 3 distinct PRs represented
- [ ] At least one `verdict=fail` observation (so FP rate has a real denominator)

If the recomputed baseline meets Option A criteria (FP ≤ 5%,
`merged_anyway=true && ledger_updated=false` cases all explained), flip
`.github/workflows/validate.yml` per the action below.

### Action when promotion is later approved

Edit `.github/workflows/validate.yml` to flip
`factory-claim-ledger-strict (advisory)` to a required check
(remove `continue-on-error: true` from the validator step OR remove the
`(advisory)` suffix and add to required-checks). Update this file's
`status:` to `promoted` and append a new sign-off row.

## Demotion Plan

> Applies if promotion is later approved and subsequently reverted.

- **Trigger:** FP rate climbs above 5% post-promotion, OR a `merged_anyway=true && ledger_updated=false` case appears that is not explained by an unrelated CI flake.
- **Revert:** restore `continue-on-error: true` and `(advisory)` suffix in `validate.yml`.
- **Root-cause fix:** investigate the failure mode in the offending observation(s) before re-attempting promotion; document in this file's `## Demotion Plan` row.

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Operator | Bo Fuller | 2026-05-08 | Option B (defer); sample 3 < 20, 2 PRs < 3, vacuous FP rate |

---

**Cross-refs:**
- Aggregator: `scripts/aggregate-observation-log.sh`
- CI source: `.github/workflows/validate.yml` (`factory-claim-ledger-strict (advisory)` job)
- Plan: `.agents/plans/2026-05-07-drain-open-next-work-items.md` §soc-ejq2
- Wave 1E parent issue: soc-f42z9
