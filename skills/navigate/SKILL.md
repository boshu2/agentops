---
name: navigate
description: 'Pick the next wave on a bead graph and keep the graph honest toward frozen acceptance. Use when: a goal starts a wave, or you ask what is next on an epic.'
practices: [lean-startup, bdd-gherkin, ddd-bounded-context]
hexagonal_role: supporting
consumes: [outer-goal-prompt, goal-acceptance, native-work-state, validation-result]
produces: [native-handoffs]
context_rel: [{kind: customer-of, with: craft-goal}, {kind: supplier-to, with: orchestrate}]
skill_api_version: 1
user-invocable: true
metadata:
  tier: execution
  dependencies: []
  capabilities: [observe_work_graph, select_next_wave, ratchet_work_graph, report_graph_hygiene]
  effects: [update_native_graph]
  canonical_status: canonical
  disposition: keep_strategy
  stability: experimental
output_contract: 'wave checkpoint in the existing handoff or root epic: acceptance matrix, frontier, wave and reasons, ratchets and churn, budget, helper use and native state, next thesis, open decisions; a single pass returns it with hygiene findings and writes nothing'
---

# Navigate

A crafted goal runs many RPIs over one bead graph: the root epic holds frozen acceptance
and each child bead is one experiment with one RPI. A running goal applies
Navigate each wave to pick beads and write results back; a person can run
[one pass](#one-pass-without-a-goal). It never edits acceptance, dispatches,
judges or closes: Craft Goal owns the prompt and HOLD,
[Plan](../plan/SKILL.md) shapes a bead, [Orchestrate](../orchestrate/SKILL.md)
dispatches, RPI runs, [Validate](../validate/SKILL.md) judges.

## Speak the domain

- **BDD:** each criterion is a Given/When/Then example with an observable
  result. Each bead names the example it moves; a bead that lacks one gets
  Plan first inside its RPI. A criterion with no observable result is an open
  decision for the caller, who can settle it with Interview: report it, never
  rewrite it.
- **DDD:** the root epic defines each domain term once, in one line. Titles,
  examples, code and tests reuse that exact word. A synonym is a hygiene
  finding; [Domain](../domain/SKILL.md) settles disputes.
- Write a bead as its title, then its id: `Redelivery test (ag-12)`.

## Bead graph contract

Root epic: outcome, acceptance examples, non-goals, authority, domain terms.
Child bead: the question and the criterion or uncertainty it serves; method,
expected observation, falsifier, scope, non-goals; notes enough to resume
after compaction; verdict, evidence refs, learning.

Edges: `parent-child` for membership, `blocks` only for real ordering, `related`
for alternatives, `discovered-from` for provenance. A requested retrospective
never `blocks` code judgment; both stay required.

The tracker owns status and closing. BD is the example; any tracker with
status, dependencies and notes fits. With BD, run `bd context --json` before any
write; BR is a different tool, never a fallback or alias for BD. `bv` rank is advice; live BD beats it and any saved plan.

## 1. Observe

```bash
bd context --json                   # verify the destination first
bd show <epic>                      # outcome, acceptance, domain terms
bd children <epic>                  # direct children only; recurse into child epics
bd ready --parent <epic> --json     # ready frontier across descendants
bd blocked --parent <epic>          # blocked work
bd show <bead>; bd comments <bead>  # prior verdicts and evidence refs
```

Build the acceptance matrix (criterion, evidence, status); note in-flight beads
and any result limit you hit. Only a cited Validate PASS proves a row; closed
status proves nothing. A closed bead with missing or stale bytes is not usable
readiness: report it. An empty ready list does not prove completion. Done when
every criterion has a row and every open row names its bead, blocker or gap.

## 2. Pick the wave

When every row is proven, or no ready bead serves an open row, pick nothing,
say which, and go to step 4. Otherwise pick the smallest set of ready beads
with the most decision-relevant information. Each serves an open row or a
named blocking uncertainty, has write and generated scopes disjoint from the
rest and from in-flight beads, and fits the declared wave budget; no budget
means one bead. Prefer an early falsifier.

Hand each bead to one RPI. When delegation is authorized, hand it to
Orchestrate or Agent Native to dispatch, one bead per worker; otherwise the
caller's runtime runs it. Each candidate gets one fresh, author-distinct Validate.
Done when each picked bead has a one-line reason and a named handoff.

## 3. Ratchet the graph

Record each verdict unchanged on its bead, for example
`bd update <id> --append-notes "verdict: FAIL; evidence: <refs>; learned: <decision it changes>"`.
Update its matrix row, then classify each discovery:

| Discovery | Action |
|---|---|
| Needed for frozen acceptance, within authority and budget | `bd create "<title>" --parent <epic> --deps discovered-from:<id> --acceptance "<example it serves>"` |
| Useful later | note or link it outside the epic; never run it in this goal |
| Changes acceptance, exceeds authority or budget | HOLD; the goal's breaker takes over |

A result ratchets when it proves part of acceptance, falsifies a live
hypothesis with discriminating evidence, or resolves an uncertainty so the next
experiment differs; FAIL and NOT_PROVEN can ratchet. Commits, counts, digests,
rewritten plans and red with no new information are churn. Split old defects
from regressions by before/after reproduction or equivalent causal evidence
under the same acceptance; counts, timestamps and new ids prove no cause.
Unknown cause, a reopened finding or recurrence of a closed finding class is
HOLD, not proof the design is wrong. Keep necessary findings necessary; nothing
resets a total. Done when every verdict sits on its bead and every discovery
has a class.

## 4. Checkpoint

Append this block to the existing handoff or root epic notes; no new artifact.
Stop after appending it: the goal continues, holds or ends.

```text
Acceptance: A1 Given a completed Job, when redelivered, then its side effect runs once: proven (<PASS ref>)
            A2 <Given/When/Then>: open (<bead title> <id>)
Frontier:   <ready beads, by title>
Wave:       <bead title>: <row or uncertainty it serves>
Ratchets:   <results that changed a decision>; churn: <results that did not>
Budget:     <remaining if measured, else unmeasured>
Helper:     <HOLD incident and helper use, or none>; native state: <observed continue/stop>
Next:       <thesis>; decisions: <open questions for the caller>
```

## One pass without a goal

Run steps 1 and 2 and return the wave instead of handing it off. Add hygiene
findings: cycles among the epic's beads (`bd dep cycles`, filtered to them),
beads tied to no criterion, `blocks` edges
that are not real ordering, criteria with no observable result, closed beads
with missing bytes and drifted terms. `bd graph <epic>` shows the shape. Reply
in the step 4 shape and write nothing; change edges only on the caller's
go-ahead. Stop after one pass.
