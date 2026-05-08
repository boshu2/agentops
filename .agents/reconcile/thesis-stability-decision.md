---
id: thesis-stability-decision-template
type: decision-template
gate: soc-r3y8b
epic: soc-e4ulx
status: template
---

# Thesis-Stability Gate Decision

> Template. Fill in once `bash scripts/check-thesis-stability.sh` returns
> exit 1 (drift detected) and the operator must consciously decide whether
> to proceed to Wave 2 of the Reconciliation Engine arc.

## Gate run

| Field | Value |
|-------|-------|
| Date | _YYYY-MM-DD_ |
| Operator | _name_ |
| Snapshot SHA | _from `.agents/reconcile/wave-0-thesis-snapshot.md` header_ |
| Current SHA | `git rev-parse HEAD` |
| Script verdict | _PASS / FAIL_ |
| Surfaces drifted | _README.md, PRODUCT.md, GOALS.md, none_ |

## Drift summary

If FAIL, paste the script's drift output (or a meaningful subset). Highlight
which sentences/claims changed and why.

```diff
<paste relevant diff blocks here>
```

## Operator decision

Pick exactly one. Do not hedge.

- [ ] **Accept drift.** The thesis HAS evolved since Wave 0. The new thesis
      is what we'd ship today. Wave 2-4 acceptance criteria must be
      re-validated against the new thesis below.
- [ ] **Re-brainstorm.** The drift indicates the plan is no longer aligned
      with the thesis we want to enforce. Restart from `/brainstorm`
      before any Wave 2-4 work.
- [ ] **Incidental edit.** The drift is mechanical (typo, link fix, format)
      and not a thesis change. Regenerate the snapshot from a clean SHA;
      document the regeneration command below.

## Acceptance re-validation (if Accept drift)

For each Wave 2-4 acceptance criterion, confirm it still holds against the
new thesis. If any criterion no longer holds, file a follow-up issue
adjusting the plan and link it here.

| Wave | Criterion | Still holds? | Follow-up |
|------|-----------|--------------|-----------|
| 2 |   |   |   |
| 3 |   |   |   |
| 4 |   |   |   |

## Snapshot regeneration (if Incidental edit)

```bash
# Capture the new snapshot from the current closure SHA
git rev-parse HEAD  # → record this SHA in the snapshot header
# Re-run the awk extractor against the chosen SHA, paste into snapshot file
git show <SHA>:README.md | awk 'NR==1, /^## / {if (!/^## /) print}'
# (repeat for PRODUCT.md, GOALS.md)
```

Document the rationale for regeneration here:

```
<reason>
```

## Sign-off

| Role | Name | Date |
|------|------|------|
| Operator |   |   |
| Reviewer (optional) |   |   |

> After sign-off, this file becomes the durable record of the gate decision.
> The Reconciliation Engine arc consults this when deciding whether to
> proceed past the Wave 1/2 boundary.
