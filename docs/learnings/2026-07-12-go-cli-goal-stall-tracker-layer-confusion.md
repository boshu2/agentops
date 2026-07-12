# Post-mortem: Go CLI Goal Stall and Tracker-Layer Confusion

**Date:** 2026-07-12  
**Scope:** Agent execution of the Go CLI production-hardening goal  
**Impact:** The operator returned after roughly ten hours to an opaque, stopped
workflow, no useful status summary, and no initially visible decomposition in
the repository's issue tracker. The follow-up explanation then blurred the
boundary between the repository tracker and the Gas City substrate store.

## Summary

This was an execution-control failure, not a Go runtime incident.

The agent treated ordinary difficulty as a reason to stop the goal and surface
an andon before exhausting the model-adjudicable recovery path. It also failed
to make the non-trivial work legible as `br` beads when the work left the
prompt. When challenged, it described `br` and `bd` as if one had replaced the
other everywhere. That explanation was wrong because the two stores operate at
different layers.

The correct operating model is:

| Context | Store | Meaning |
|---|---|---|
| Work tracked for this AgentOps repository | `br` / beads_rust, resolved with `BEADS_DIR="$(ao beads dir)"` | Private git-JSONL repository ledger (`age-*` beads) |
| Gas City substrate operated by AgentOps users | `bd` / Dolt | Native durable substrate store used by a gas-city factory |

`br` is authoritative for work **in this repository**. `bd` remains a valid,
first-class product dependency where AgentOps is composed with the Gas City
substrate. They are not competing global tracker choices.

## What Happened

1. A long Go audit/refactoring run lost continuity across an account login and
   panel relaunch.
2. The visible session title did not communicate the active goal, phase,
   checkpoint, or next action.
3. When execution encountered uncertainty, the goal stopped at the human-facing
   andon boundary instead of first routing the blocker through a fresh helper,
   council, or equivalent bounded adjudication pass.
4. The work had not been presented as a durable `br` decomposition, so the
   operator could not inspect the graph and recover the state independently.
5. The first tracker explanation overcorrected: it accurately named `br` as
   this repository's tracker, but framed `bd` as broadly retired instead of
   distinguishing repository bookkeeping from substrate state.

## Expected Behavior

For an active goal, normal red evidence should cause automatic re-planning:

```text
RED -> diagnose -> amend plan/beads -> retry
```

If the same blocker repeats or presents an architecture/plan-shape decision,
the next step is a bounded fresh-context helper:

```text
repeated blocker -> council/helper -> UNSTUCK -> retry
                                  \-> ESCALATE -> human andon
```

Human escalation is appropriate only after that helper cannot resolve the
blocker, or when the decision is explicitly reserved for the operator. An
andon is a stop-the-line exception, not a generic synonym for a failed attempt.

For tracking, any non-trivial goal that leaves the prompt must be represented in
the live private repository ledger:

```sh
export BEADS_DIR="$(ao beads dir)"
br ready
br update <age-id> --claim
```

The agent must not use `bd` for AgentOps repository issues. That local rule must
not be generalized into a claim that `bd` is retired from every AgentOps
deployment.

## Contributing Causes

### 1. The recovery policy was applied as a stop rule

The agent recognized a breaker condition but skipped the helper tier. This
collapsed a three-tier policy—automatic repair, model adjudication, human
escalation—into a binary retry-or-stop policy.

### 2. Execution state was not operator-legible

The work had internal artifacts, but the operator-facing state did not answer
four basic recovery questions: what goal is active, what phase is running,
what is committed, and what happens next. After a relaunch, the title alone was
not a handoff.

### 3. Tracking was treated as an implementation detail

The bead graph should have been the durable, inspectable representation of the
goal. Delaying or omitting that graph made the run dependent on conversational
context precisely when conversational continuity failed.

### 4. Tracker terminology lacked a layer qualifier

Several historical documents use phrases such as "`bd` retired" as shorthand
for the AgentOps repository tracker migration. Without the qualifier "for this
repository's tracking," that shorthand contradicts the current Gas City
composition model and invites a false global conclusion.

## What Went Well

- The operator identified both failures precisely: premature escalation and
  tracker-layer conflation.
- The Go audit work itself was recoverable; committed checkpoints and evidence
  were not lost.
- The existing doctrine already described the intended helper-before-human
  behavior, so recovery did not require inventing a new process.
- The existing `age-nw28h` program could be reused rather than replaced with a
  duplicate epic.

## Corrective Actions Taken

1. The Go production-hardening goal packet now states that ordinary red is work,
   not an andon, and that human escalation is legal only after a bounded helper
   fails or on an explicit refusal lane.
2. An independent packet-breaker helper was used during goal design and returned
   `UNSTUCK`; the repaired packet then passed independent validation.
3. The existing `age-nw28h` epic was reused. Twenty-two one-owner child beads,
   `age-nw28h.7.6.1` through `age-nw28h.7.6.22`, were created in the private
   `br` ledger instead of creating a parallel tracker or duplicate epic.
4. The active RPI re-plan now routes compatibility uncertainty through bounded
   research, planning, and pre-mortem passes while keeping the goal active.
5. All direct repository-tracker mutations in the recovery use
   `BEADS_DIR="$(ao beads dir --require)" br ...`; no `bd` mutations are used for
   AgentOps repository work.

## Follow-up Actions

| Priority | Action | Evidence of completion |
|---|---|---|
| P0 | Keep the helper-before-human transition executable in goal packets and plan-pawl checks. | A repeated blocker cannot reach `HUMAN` without a helper receipt, except an explicitly classified refusal lane. |
| P0 | Emit a phase-boundary status that survives account or panel relaunch. | The active goal, phase, bead, last durable commit, verdict, and next action are recoverable without conversation history. |
| P1 | Audit public and contributor docs for unqualified "`bd` retired" claims. | Every live claim distinguishes "not this repo's tracker" from "valid Gas City substrate store"; historical migration records remain clearly labeled. |
| P1 | Gate repository workflow examples on the resolved private `br` ledger. | Repo-tracking examples use `BEADS_DIR="$(ao beads dir)" br`; tests reject accidental `bd` repo-tracker instructions. |
| P1 | Make bead creation/reuse part of non-trivial goal admission. | A goal cannot enter implementation without a referenced existing bead or a deliberate one-shot exemption. |

## Durable Lessons

1. **A goal is an autonomy contract.** Difficulty changes its route; it does not
   silently terminate the contract.
2. **An andon must be earned.** Automatic repair and one bounded fresh-context
   helper come first unless the operator explicitly owns the decision.
3. **The tracker is the recovery interface.** If an operator cannot reconstruct
   the run from the bead graph and durable receipts, the work is not adequately
   tracked.
4. **Retirement claims require a scope.** "Not authoritative here" and "not a
   supported substrate anywhere" are different statements.
5. **A title is not a handoff.** Long-running work needs a compact, durable phase
   receipt with a precise next action.

## References

- [Operating Loop](../architecture/operating-loop.md)
- [The Flywheel: three-tier andon policy](../architecture/the-flywheel.md)
- [Go CLI Production-Readiness Audit](../audits/2026-07-12-go-cli-production-readiness.md)
- [Repository tracker instructions](../../AGENTS.md)
- [Gas City composition](../3.0.md)

