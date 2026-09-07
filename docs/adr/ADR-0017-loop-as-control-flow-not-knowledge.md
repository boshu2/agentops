# ADR-0017: The Loop Is Control Flow, Not Knowledge

- **Status:** Accepted, amended for selected CDLC 2026-09-06 (2026-09-03)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven, position on the verification system), [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding demoted to hypothesis)
- **Origin:** `docs/plans/2026-09-03-loop-restore.md` (this decision's intent source), and the 2026-09-02 Train 1 run, where the repair loop had to be improvised by hand

## Active disposition — 2026-09-06 selected CDLC

Retain the bounded repair law, once-only Plan/Implement, exact subject and fresh
author-distinct Validate. Replace the blanket exclusion of knowledge writes and
outer re-planning with selected **Context Delivery Lifecycle** contracts:
maintained external context/environment around disposable agents, no weight
training and no deterministic-inference promise. Discovery (Plan shapes),
Implement and Validate are the delivery phases. An explicitly selected bounded
outer goal may authorize a new experiment within its accepted envelope; RPI
itself neither replans nor owns native budgets, stops, queues or delivery.

Replace Decision 3's NTM-only and blanket headless-Claude ban with the single
[authorized bounded model-dispatch recipe](../../skills/agent-native/references/model-dispatch.md).
Both fresh and required cross-family exact-subject legs remain, with independent
initial inputs and actual model/context receipts. Risk or caller-required
unavailable diversity stays `diversity_unsatisfied` / `NOT_PROVEN`; no preferred
judge, majority vote or author can supply binding PASS. Native runtimes own
finite input/output, timeout and verified cleanup. Exit/process facts are not
semantic verdicts. Executable Door9 and specialist provider guards stay intact.

The selected new roots are Recall first and a small evolve only after native
stop/continuation proof. Recall and broader Learn source/curation behavior have
later owners; T25 alone may restore evolve and amend `REMOVED_SKILLS`. Thus
"What stays removed" below describes the current executable inventory, not a
permanent prohibition on the selected later learning contract. No Recall or
evolve entrypoint is advertised as runnable by T04; existing entrypoint checks
and dependency invariants remain enforced. Selected recall is instruction-level
composition, not a new hard edge or Discovery root. The old evolve implementation,
operator-only unlimited rules, CLI/controller and packet machinery stay retired.

[ADR-0016](ADR-0016-state-tiers.md) admits caller-owned external reviewed
Markdown/OKF memory and protected non-Git drafts/evidence. This replaces
scratch-only curation without restoring a transcript lake or a second work
store. Factual support, destination disclosure before any Git object/index/stash,
and later utility are separate decisions. Source authors cannot approve their
own knowledge, and learning cannot change completed product verdicts.

ADR-0004/0011's honesty remains: measured mechanism and net benefit are distinct.
The current cold-reuse trial requested no optional knowledge body and showed no
memory benefit. Trial acceptance preserves its failed worker and null result;
it cannot imply improvement. The selected skills-plus-Go evidence path needs
later implementation and shared conformance, not a shipped Python expansion.
The original removal rationale and repair evidence below remain historical.

## Active stopping amendment — 2026-09-07

Replace finding-count monotonicity and digest movement as repair admission
proxies with acceptance evidence: a fresh digest-bound receipt proves closure
of a named acceptance finding or proof gap. New ids need causal evidence of
pre-existing discovery; introduced regression or unknown cause stops repair.
Reopened ids and recurring closed classes warrant causal HOLD, never an
automatic design-failure diagnosis. The existing pure reference consumes those
receipt facts without creating a runtime, persisted schema, or budget account.

Informative red can justify a different experiment in an explicitly selected
bounded outer goal under unchanged acceptance. Its causal HOLD gets exactly one
bounded fresh helper per incident inside the remaining allowance; unhelpful
advice stops implementation. Cancellation, explicit refusal/judgment, or a spent
hard time/cost/quota skips the helper. New subjects, compaction, and repeated
continuations never reset allowance or helper use. RPI itself stops and reports.

Size validation by effect on acceptance/enforcement, as owned by Validate.
Policy and stopping changes need cross-family judgment even as documentation;
unknown risk takes the stronger path. Narrow nonbehavioral wording changes may
use one fresh judge prospectively. This amendment does not waive its own or
any already-required review leg. Exact subject, all acceptance, and empty
`not_checked` remain necessary for binding PASS. Reuse applicable exact-input
receipts and fast discriminating checks; no additional progress ledger is owed.

Native objective text demonstrates neither pause nor aggregate allowance
operations. Report observed controls and missing measurement truthfully. Evidence
and provenance are retained while beliefs may be revised; knowledge is not
monotonically true or useful. The historical rationale below is preserved.

## Historical context

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
   2. New digest-bound evidence proves closure of a named acceptance finding
      or proof gap. Changed bytes or a reduced finding count alone do not count.
   3. No closed id reopens, closed class recurs, or introduced regression is
      admitted. A new finding needs causal evidence of prior existence under
      the same acceptance; unknown cause stops repair. The open set remains
      the union of required judges' stable ids, including necessary discoveries.
      The precise current law and receipt bindings live in `skills/rpi/SKILL.md`.

   Converged means the fresh validator returns PASS and, when the diff touches
   a risky surface, the cross-family validator also returns PASS. On any
   violation RPI stops and reports the current status. No third judge, no
   escalation, no auto-replan.

2. **Crank's shape returns as a thin wave executor.** One invocation executes
   one caller-selected wave, runs the wave's acceptance once, returns wave
   evidence, and stops. It owns no wave selection, retry, budget, queue, claim,
   lease, Git, closure, or next work.

3. **Cross-family validation follows effect-based risk**, as owned by
   `skills/validate/SKILL.md`, with its conservative path cues and stronger
   treatment of unknown risk. Fresh author-distinct judgment always remains.
   The prospective rule cannot waive a leg already required for the change
   being judged. Dispatch uses the authorized bounded model-dispatch recipe
   cited above; required unavailable diversity remains `diversity_unsatisfied`
   / `NOT_PROVEN`. Same-family agreement cannot satisfy a required second leg.

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

Two narrow authorizations ride on this decision: the skill mesh permits exactly
one non-core hard dependency, `crank` on `rpi` (`scripts/generate-skill-mesh.py`),
and the Claude conveyor's cross-family leg is a caller-supplied read-only command
whose model family the script cannot verify and does not claim to.

## Consequences

- RPI can end in a repaired PASS instead of only a first-pass verdict, and the
  stopping rule is in the contract where a validator can check it, rather than
  in an orchestrator's judgment.
- A repair round without new acceptance-relevant proof stops even when bytes
  or counts move. Newly discovered pre-existing defects do not become
  regressions merely because they increase the count; unknown cause still stops
  for examination. Control artifacts alone never justify repair.
- Risky-surface changes cost a second judge leg. When no legal adapter is
  available, the honest outcome is `NOT_PROVEN` and the change waits.
- The removed surfaces stay removed. A future proposal to bring back a
  knowledge store or a lifecycle command does not inherit this ADR's
  permission; it needs its own, with evidence.
