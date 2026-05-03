# Convergence Criteria

## Purpose

Decide when the audit-fix-rescan cycle (`references/audit-fix-rescan-cycle.md`) is finished. Without explicit criteria, agents either stop at the first quiet pass (premature) or keep re-reading forever (unbounded).

A pass is *converged* when each of the criteria below holds. The cycle is *complete* when two consecutive passes both converge.

## Stop Criteria

All of the following must be true on the same pass:

- [ ] No new HIGH or MEDIUM findings appear during the pass.
- [ ] All previously-deferred findings from earlier passes are resolved or filed as separate issues with a stated reason.
- [ ] The test suite is green on the post-fix tree (unit + integration where the project has them).
- [ ] Static scanners used in pass 1 are clean or only flag entries with recorded justifications.
- [ ] No regression filed during pass 3 of the most recent iteration is still open.

If any item is false, the cycle continues.

## Finding-Decay Signal

Track findings per pass in the audit report. A healthy cycle shows monotonic decay:

```
pass 1: 12 findings
pass 2:  4 findings  (re-read of touched files)
pass 3:  1 finding   (integration regression)
pass 4:  0 findings  (verification)
```

Warning shapes:

- **Flat curve** (`12 → 11 → 10 → 9 …`): you are skimming, not re-reading. Slow down or narrow scope.
- **Re-emergence** (`12 → 4 → 9`): a fix introduced a new defect class. Roll back the offending fix or scope-isolate it.
- **Late spike on verify** (`8 → 3 → 1 → 6`): pass 4 should not find more than pass 3. If it does, the rescan is using a tool or a perspective the earlier passes did not, and that tool should be promoted into pass 1 of the next cycle.

## Signal-to-Noise Ratio

Per pass, compute:

```
signal_ratio = real_bugs_fixed / (real_bugs_fixed + suppressed_false_positives)
```

- `signal_ratio > 0.6` on pass 1 means the scanner is well-calibrated for this scope. Trust its output.
- `signal_ratio < 0.3` on pass 1 means most findings are noise. Tighten scanner config or change scope before continuing.
- `signal_ratio` on pass 2 should be ≥ pass 1 (manual review is more selective). If it drops, you are pattern-matching instead of reading.

## Pre-Release Gate

For pre-release or pre-merge use, require these in addition to the stop criteria:

- [ ] Two consecutive passes meet stop criteria (the second is verification of the first).
- [ ] No HIGH severity findings remain unfixed (MEDIUM may be filed as follow-up issues with explicit deferral).
- [ ] Scanner exit code is 0 with `--fail-on-warning` or equivalent strict mode.
- [ ] Diff against merge base has been read end-to-end at least once.
- [ ] Bug-finding pyramid checks (BF4 chaos/negative, BF5 script functional, BF1 property — see `skills/bug-hunt/SKILL.md`) have either been run or recorded as out-of-scope.

For non-release use (refactor sweep, regression hunt, exploratory audit), the stop criteria alone are enough.

## When Not to Converge

Stop the cycle and escalate (do not declare convergence) if:

- The same defect class re-emerges across passes despite fixes (architectural smell, not a bug).
- Three countable failures accumulate on the same root cause per `references/failure-categories.md`.
- Test infrastructure is unreliable enough that pass 3 cannot give a clean signal — fix the test infrastructure first.

In each case, file the escalation in the audit report and stop the cycle. Convergence reached under unreliable signal is worse than an honest "did not converge".

---

> Pattern adopted from `multi-pass-bug-hunting` (jsm/ACFS skill corpus). Methodology only — no verbatim text.
