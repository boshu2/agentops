# age-vx0 pre-push pawl evidence — dynamo-e2e proves the attempt-ordering rework join

**Bead:** age-vx0 — the dynamo rework-attribution e2e demonstrated a phase label, not the ordering mechanism it claims.
**Mode:** fresh-context (default). Author context: `age-vx0-claude-main`. Refuter context: a fresh-context subagent (distinct invocation).

## The bug

`scripts/dynamo-e2e.sh --scenario=rework` claimed to prove the ratchet penalizes rework, but its L>0 came entirely from an **explicit `phase=rework` row** (classifyUsage's phase branch). The attempt-**ordering** rework join — where an accepted bead's spend at an attempt < the accepting attempt is classified rework by `usageAttempt` — never fired. Worse, the scenario's attempt-1 700-token spend was emitted **after** its own REFUTE verdict, so `usageAttempt` attributed it forward to attempt-2 and it read **Productive** (mislabeled). The e2e proved a phase label, not the mechanism.

## The fix

- **New `rework-order` scenario**: attempt-1 production is emitted **before** its verdict (the realistic produce→gate order), so `usageAttempt` maps the 700-token spend to attempt-1 (< accepting attempt-2) → rework — classified **solely by the ordering join, no phase=rework label anywhere**. Both `use` rows carry `phase=implement`.
- **Tightened bats**: asserts `rework=700 productive=500` (the ordering join fired on the attempt-1 spend).
- **Corrected the `rework` comment**: its loss is the phase=rework label (rework=500); its attempt-1 700 reads Productive (post-verdict forward attribution). Both paths now have honest coverage.

## Live evidence

```
rework-order : L breakdown rework=700 productive=500, L=0.583  (ordering join, no phase label)
rework       : L breakdown rework=500 productive=700, L=0.417  (phase label; 700 attempt-1 reads productive — the bug, now documented)
clean        : productive=1000, L=0.000, 1/1 clean
bats tests/scripts/dynamo-e2e.bats → 5/5 ok
ao gate check --fast --scope head → 20/20 pass, 0 fail
shellcheck → clean (only the pre-existing SC2001 on line 117)
```

## Adversarial review (CONFIRMED)

A fresh-context reviewer verified and **CONFIRMED**, including the gold-standard regression check:

- **The scenario genuinely proves ordering, not a phase label** — both `use` rows are `phase=implement`; traced through `classifyUsage`, the only path to Rework is `usageAttempt < acceptingAttempt`, so `rework=700` has no other possible source.
- **The assertion bites** — the reviewer reintroduced the old bug (moved the attempt-1 `use 700` to AFTER its REFUTE verdict): the breakdown flipped to `rework=0 productive=1200`, L=0.000, and the harness exited 1 with the explicit "ordering join did NOT classify" assertion firing. The teeth are real, not cosmetic.
- **No regression** — `rework` (`rework=500 productive=700`, comment accurate) and `clean` (1/1) unchanged; all 5 bats pass; shellcheck no new findings.
