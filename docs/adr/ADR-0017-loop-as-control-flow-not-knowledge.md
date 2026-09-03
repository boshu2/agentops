# ADR-0017: The Loop Is Control Flow, Not Knowledge

- **Status:** Accepted (2026-09-03)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven, position on the verification system), [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding demoted to hypothesis)
- **Origin:** `docs/plans/2026-09-03-loop-restore.md` (this decision's intent source), and the 2026-09-02 Train 1 run, where the repair loop had to be improvised by hand

## Context

The 2026-07-14 cut (`482307762`, 2,433 files, −416K lines) removed the
compounding-knowledge machinery and the iterate loop in the same pass. It took
out `converge`, `crank`, `discovery`, `evolve`, and the learn write-half
together, as one class.

They were not one class. ADR-0004 and ADR-0011 demoted a knowledge claim: that
an escape corpus accrues and compounds into an advantage. Neither ADR said
anything about control flow. Converge was not a knowledge store: it was the
criterion for when repair stops. Crank was not a corpus: it was a wave
executor. The cut removed them because they sat next to the unproven claim, not
because anything demoted them.

What that costs showed up on 2026-09-02. The Train 1 run needed repair and
re-validation, and the contract had no repair phase, so the loop was improvised
by hand: 8 validators and 2 stops, with the stopping rule living in an
orchestrator's judgment instead of in the contract. An improvised loop is not
reproducible and cannot be judged.

## Decision

Restore the control flow the cut over-reached on, and only that.

1. **Converge's criterion returns as RPI's repair phase**, bounded by the
   convergence law from the plan. A repair round is admitted only while all
   hold:
   1. `rounds_used < repair_rounds` (caller-declared, default 2).
   2. The open finding set, keyed by the validators' stable `findings[].id`
      (union across the fresh and, when used, cross-family validators), is not
      larger than the previous round's.
   3. No finding id closed in an earlier round reopens.
   4. Between rounds either the subject-manifest digest changed
      (generated-only changes count when they change the digest) or, for
      `NOT_PROVEN`, new digest-bound evidence was supplied that resolves a
      named gap.

   Converged means the fresh validator returns PASS and, when the diff touches
   a risky surface, the cross-family validator also returns PASS. On any
   violation RPI stops and reports the current status. No third judge, no
   escalation, no auto-replan.

2. **Crank's shape returns as a thin wave executor.** One invocation executes
   one caller-selected wave, runs the wave's acceptance once, returns wave
   evidence, and stops. It owns no wave selection, retry, budget, queue, claim,
   lease, Git, closure, or next work.

3. **Cross-family validation is the default on risky surfaces**
   (`cli/internal/gates/**`, `scripts/check-*.sh`, `tests/**`,
   `skills/*/scripts/**`, `skills/cc-hooks/policies/**`, `lib/**`, anything
   `security-gate.sh` scans), caller-elected elsewhere. Dispatch obeys the
   runtime floor: orchestrating in Claude, the judge leg is a read-only
   `codex exec`; orchestrating in Codex, the judge leg is a caller-selected
   interactive Claude session in an NTM pane. `claude -p` and `claude --print`
   are never used, directly or indirectly. With no authorized live adapter the
   request is disclosed as `diversity_unsatisfied`, and on a risky surface that
   is `NOT_PROVEN` rather than same-family convergence.

### Conformance assertions flipped

This ADR is the sole authority for flipping the assertions below. They encode
the cut's over-reach, not a demoted claim, and they are flipped by lane L-A of
the plan:

- `scripts/check-cathedral-cut-conformance.py`: crank listed as removed; RPI
  required to contain "Stop regardless"; loops in `run_once.py` rejected.
- `workflows/rpi.js`: single-pass traversal.
- `skills/rpi/scripts/validate.sh`: single-pass assertion.
- `skills/rpi/tests/test_run_once.py`: stop-on-FAIL.
- `evals/agentops-core/rpi-behavior.json`: single-pass behavior expectation.

### What stays unproven

- Compounding stays unproven. ADR-0004 and ADR-0011 remain in force; nothing
  here reopens a corpus, a knowledge store, or an accrual claim.
- The loop's own effect on outcomes is unproven. Restoring a repair phase is a
  contract change, not evidence that repair produces better subjects. That is
  owed a seeded-defect probe, and until one runs, the loop is justified by the
  improvisation it replaces, not by measured lift.

### What stays removed

- `ao converge` and `ao crank` as root commands.
- `evolve`, and the ADR-0007 operator-stop rules.
- The learn write-half.
- Discovery's packet machinery.
- Converge's Go command, its canary, and its findings registry; crank's flags,
  lifecycle tiers, and Sisyphus markers.

## Consequences

- RPI can end in a repaired PASS instead of only a first-pass verdict, and the
  stopping rule is in the contract where a validator can check it, rather than
  in an orchestrator's judgment.
- A repair round that cannot show a subject or evidence change is a stop, not a
  retry. That is the anti-ceremony guard: rounds that produce only control
  artifacts end the run.
- Risky-surface changes cost a second judge leg. When no legal adapter is
  available, the honest outcome is `NOT_PROVEN` and the change waits.
- The removed surfaces stay removed. A future proposal to bring back a
  knowledge store or a lifecycle command does not inherit this ADR's
  permission; it needs its own, with evidence.
