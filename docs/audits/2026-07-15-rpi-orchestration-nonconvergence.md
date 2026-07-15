# Audit: RPI orchestration did not converge to a delivered result

Date: 2026-07-15

## Causal question

Why did the caller-requested AgentOps RPI hardening run consume 3 hours,
49 minutes, spawn 33 delegated agents, and leave the caller without a completed,
audited, consumable result?

This report treats "produced nothing" as a statement about the delivery boundary,
not a claim that no bytes were written. Twelve Plans, ten Candidates, and nine
verdicts exist. The program nevertheless had no final implementation for its last
requirement, no aggregate completion audit, no final isolated delta, no commit or
installation, and no program-level PASS. At the caller-visible outcome boundary,
the complaint is supported.

## Pinned inputs

- Main Codex rollout:
  `/home/bo/.codex/sessions/2026/07/15/rollout-2026-07-15T12-04-40-019f6685-eb4b-7c03-a5b0-493e9765ba79.jsonl`.
- Immutable pre-audit prefix: the first 3,584 JSONL records, ending before the
  audit turn, SHA-256
  `37fcf93dba347be99e688b88d50c31c71e576a1a1a1967eeaa7966fe122d2afc`.
- Active goal interval: `2026-07-15T18:55:20Z` through the caller stop at
  `2026-07-15T22:44:24Z` (3:49:04 wall clock).
- Worktree: `/home/bo/dev/agentops-rpi-loop-hardening`, branch
  `codex/rpi-loop-hardening-20260715`, based at
  `baaa9e22dadc423d7b7bb73c043112ecfca425cb` and initialized by replaying the
  source checkout's pre-existing dirty tracked and relevant untracked state.
- Stop-state tracked diff SHA-256:
  `748bd5a347845983bcec77314c683cdd65548e77639247c00ffe99b50a9fd9c6`.
- Stop-state sorted untracked-path-list SHA-256:
  `fa86b0b61acd20be735da0f27f2b6523cb8327a8caae92f5efc0835bb57e62f1`.
- Twelve Plan packets under `.agents/plans/`, ten Candidate packets under
  `.agents/candidates/`, and nine immutable verdicts under
  `.agentops/verdicts/sha256/`. These are local ignored evidence; the tracked
  audit is the portable review artifact.
- Final unimplemented Plan:
  `.agents/plans/slice-7-finding-index-learn-routing.plan-packet.json`, digest
  `4d8ad17f8cd2208f0f35be393b71bc00ce407d32579886bffb6c79b9e13a93cc`.
  Its first artifact, `schemas/finding-index.v1.schema.json`, remained absent at
  the stop boundary and no Candidate or verdict existed for the slice.
- RPI contract inspected at `skills-codex/rpi/SKILL.md`: one Plan, one
  Implement, exactly one fresh Validate after a Candidate, stop regardless of
  result, caller-owned continuation, and a schema-validated `rpi-report.v1`.

No acceptance test was re-run for this postmortem. Existing verdicts are treated
as the immutable semantic evidence for their individual slices.

## Outcome definition and observed outcome

The caller asked to create a worktree from `~/dev/agentops`, run the supplied RPI
improvement context, and orchestrate it to completion. Completion therefore
required all named improvements, a final audit proving the combined worktree,
and a usable handoff. It did not require an unauthorized push or release.

Observed at the stop boundary:

- The isolated worktree and branch existed.
- Five repair/completion slices had PASS verdicts.
- Four earlier Candidates had durable non-PASS verdicts: three FAIL and one
  NOT_PROVEN.
- Two invocations stopped without a Candidate.
- The last named P2 improvement had only a Plan.
- The promised repository-wide audit had not run.
- No aggregate artifact mapped the original improvement list to final evidence.
- No `rpi-report.v1` artifact or recorded `ao packet validate --schema
  rpi-report.v1` operation was found for the twelve invocations.
- The installed CLI was intentionally not updated, and no commit or delivery
  handoff isolated the completed source changes from the inherited dirty state.

The stop-state tracked diff contained 77 files with 1,869 insertions and 2,062
deletions, but that count includes the inherited cathedral-cut diff. Per-slice
manifests prove local changes; they do not provide a single program-level delta
from the initial replayed baseline.

## Evidence-backed timeline

### Prelude: the evidence that motivated the hardening run

- `16:04Z`: a Shield documentation Implement invocation stopped on a supplied
  Plan digest mismatch. The caller authorized use of the current packet.
- `16:15Z` to `16:20Z`: the documentation edit was made and a Candidate emitted.
- `17:35Z` to `17:44Z`: fresh validation returned NOT_PROVEN because absolute
  Plan scope could not be normalized by the relative-path scope helper.
- `17:57Z` to `18:09Z`: a relative-scope corrective Plan and implementation were
  produced; a line-wrapped acceptance command caused an additional attempt
  before the caller's requested correction was reported complete.
- `18:42Z` to `18:49Z`: a Learn analysis identified scope preflight, capability
  skew, offline schema, baseline evidence, Candidate construction, validator
  visibility, and finding identity as improvement opportunities.

This prelude supplied legitimate evidence for improving RPI. It did not supply
an execution budget, repair limit, fixed number of invocations, or policy for
reconciling RPI's stop boundary with "orchestrate to completion."

### Hardening goal

| Interval | Program behavior | Durable result | Elapsed from goal start |
|---|---|---|---:|
| 18:55-18:58 | Preserve dirty source state in an isolated worktree | Worktree created | 0:03 |
| 18:58-19:10 | First Plan-scope design | NOT_BUILT: v1 packet could not bootstrap its own mandatory locator | 0:15 |
| 19:11-19:36 | Compatible Plan v2 migration | FAIL: behavior passed, stale Premortem generated hash failed acceptance | 0:41 |
| 19:37-19:51 | Premortem hash-only repair | PASS `67092c38...2209` | 0:57 |
| 19:52-20:20 | Packet capability/offline-schema implementation | FAIL: ECMA regexp incompatibility and Cobra StringArray state contamination | 1:25 |
| 20:21-20:39 | Runtime repair | NOT_PROVEN: test merged `go run` stderr with JSON stdout | 1:44 |
| 20:39-20:59 | One-file parity-test repair | PASS `9c202015...9997` | 2:04 |
| 20:59-21:24 | Plan v3 baseline expectations | FAIL: exact prose assertion broke on Markdown wrapping | 2:29 |
| 21:24-21:36 | Prose-only repair, dogfooding v3 baseline | PASS `8e906237...d881` | 2:41 |
| 21:36-21:58 | Evidence receipts, manifests, Candidate v2 | NOT_BUILT: Go values were not JSON-normalized and contract literals wrapped | 3:03 |
| 21:59-22:19 | Candidate-builder repair and preserved-slice dogfood | PASS `681b8d2c...e12c3` | 3:24 |
| 22:21-22:36 | Validator progress and heartbeat contract | PASS `d649ad5d...56c52` | 3:41 |
| 22:37-22:44 | Learn finding identity/routing | Plan only; caller stopped run | 3:49 |

The first independently closed feature required 57 minutes. The second required
another 67 minutes. At 2:04 elapsed, only two major behaviors were durably
closed, but the run continued without a caller checkpoint or revised budget.

## Quantitative orchestration ledger

- 12 Plan packets.
- 10 Candidate packets.
- 9 semantic validators and verdicts.
- Verdict distribution: 5 PASS, 3 FAIL, 1 NOT_PROVEN.
- 33 delegated agents during the hardening goal:
  - 22 received `fork_turns: all`;
  - 11 received `fork_turns: none`, principally fresh validators.
- 401 `wait_agent` calls; 286 returned a 30-second timeout. Those timeout waits
  alone represent an upper-bound 143 minutes of wall-clock waiting.
- 97 messages were sent to delegated agents.
- 103 caller-facing assistant progress messages were emitted during the goal.
- 80 composed execution calls were made by the main context.
- Main-session token counter delta during the goal:
  - 79,539,163 total tokens;
  - 79,469,078 input tokens;
  - 78,478,080 cached input tokens;
  - 70,085 output tokens;
  - 26,421 reasoning-output tokens.

The cached-input dominance is consistent with repeatedly forking the entire,
ever-growing turn into Plan and Implement agents. It does not prove every cached
token added wall-clock latency, but it does prove extreme context amplification.

Plan size also contradicted the ordinary meaning of "one bounded behavior":

| Plan | Scenarios | Evidence requirements | Included paths | Candidate paths |
|---|---:|---:|---:|---:|
| Scope v2 migration | 6 | 8 | 35 | 47 |
| Packet runtime preflight | 5 | 8 | 27 | 45 |
| Plan v3 expectations | 5 | 7 | 40 | 50 |
| Atomic Candidate evidence | 10 | 10 | 36 | 55 |
| Validator progress | 4 | 9 | 19 | 19 |
| Finding identity/routing | 6 | 14 | 41 | not built |

These were internally cohesive architecture changes, but they were not small
experiments. Their generated projections, docs, schemas, and test surfaces
created many independent ways for a one-shot acceptance run to fail.

## Supported causal claims

### 1. A missing outer program boundary created an unbounded serial meta-loop

Confidence: high.

Evidence:

- RPI itself owns one Plan/Implement/Validate sequence and says the caller owns
  continuation after the report.
- The caller supplied one broad completion objective, then sent no messages
  between `18:55:20Z` and the stop at `22:44:24Z`.
- The main context automatically selected and launched each next repair or
  feature after NOT_BUILT, FAIL, NOT_PROVEN, and PASS results.
- Twelve RPI-shaped invocations accumulated without a program-level packet,
  maximum invocation count, repair count, wall-clock budget, or aggregate
  completion report.

The phrase "orchestrate it to completion" plausibly authorized continued safe
work, so automatic continuation was not clearly disallowed by the caller.
However, it was ambiguous against RPI's explicit stop/caller-choice boundary.
The implementation resolved that ambiguity toward unlimited continuation.

Counterfactual: if the outer layer had required a continuation policy such as
one invocation, one repair, or a 45-minute checkpoint, the run would have
returned a bounded result and asked the caller before consuming the remaining
three hours. If the claim were false, repeated non-PASS outcomes would have
produced a program stop or re-authorization event; none occurred.

### 2. Strict one-shot failure handling magnified small defects into full cycles

Confidence: high.

Evidence:

- A stale generated hash required a new Plan, Implement, and validator.
- A fresh-process test that used `CombinedOutput` required a new one-file Plan,
  Implement, and validator even though production behavior already worked.
- Markdown line wrapping caused a FAIL and then a full prose-only cycle.
- JSON normalization and a wrapped command literal caused NOT_BUILT and a new
  Candidate-builder repair cycle.
- Roughly half of the twelve Plans were bootstrap or repair packets rather than
  new caller-level behavior.

The one-shot rule preserved honest evidence and prevented silent acceptance
changes. That safety benefit is real. The nonconvergence came from applying the
same full restart cost to semantic defects, test-harness defects, generated
receipt drift, and formatting-sensitive assertions.

Counterfactual: if the loop allowed a declared, bounded in-phase TDD correction
budget under the unchanged acceptance contract, or short-circuited known
mechanical failures before fresh semantic validation, the line-wrap, hash, and
stdout/stderr defects would not each require a complete new invocation. If the
claim were false, repair cycles would be dominated by changed product intent;
the record instead shows several unchanged-intent mechanical repairs.

### 3. Plans admitted migration-sized scopes into a micro-experiment protocol

Confidence: high.

Evidence:

- Major Plans permitted 27-40 included paths; their Candidates changed 45-55
  paths.
- The atomic Candidate-evidence Plan carried ten scenarios and ten evidence
  requirements across schemas, CLI, runtime, skills, generated projections,
  documentation, and conformance checks.
- The final unimplemented Learn Plan allowed 41 included paths and 14 evidence
  requirements.
- Failures repeatedly occurred in companion projections, prose, harnesses, or
  schema adapters rather than the central behavior alone.

Counterfactual: if Plan admission had a complexity threshold and required
decomposition before implementation, fewer unrelated owners would participate
in each one-shot gate. If scope size were not contributing, failure incidence
would not cluster in the many companion surfaces; it did.

### 4. Full-history agent forks amplified cost and coordination load

Confidence: high for token cost; moderate for wall-clock causality.

Evidence:

- 22 of 33 agents received the entire growing turn via `fork_turns: all`.
- The goal added 79.5 million token-counter units, 78.5 million of them cached
  input.
- Exact Plan packets, Candidate packets, evidence, and task identity were
  sufficient for the 11 fresh validators that used `fork_turns: none`.
- The main context also sent 97 follow-up messages and polled 401 times.

Counterfactual: packet-only Plan and Implement contexts would preserve explicit
contracts while avoiding repeated ingestion of hours of commentary and prior
agent work. A replay can measure whether phase quality changes. The current
record cannot isolate how much cached-input amplification affected wall time,
so that part remains uncertain.

### 5. Progress reporting communicated activity but not control

Confidence: high.

Evidence:

- 103 progress messages described healthy progress, current gates, and pending
  validators.
- None established a wall-clock ceiling, remaining invocation count, repair
  budget, total completion percentage, or mandatory caller decision point.
- The main context continued after 57 minutes, 2:04, 2:41, and 3:24 elapsed
  without asking whether the remaining scope still justified the cost.
- At 3:26 elapsed, the run spent another 15 minutes implementing validator
  progress itself, while program completion and the final audit remained open.
- The caller, not the loop, supplied the effective stop condition at 3:49.

Counterfactual: progress carrying `elapsed`, `budget_remaining`,
`invocations_used`, `repairs_used`, and a hard checkpoint would make lack of
convergence visible as a control event, not merely another status update. If
activity reporting alone were sufficient, 103 updates would have prevented the
surprise at four hours; they did not.

### 6. Slice-level proof never became program-level proof or a usable handoff

Confidence: high.

Evidence:

- Five PASS verdicts prove five exact acceptance digests, not the original
  multi-item improvement program.
- No aggregate requirement ledger mapped the original P0/P1/P2 items to current
  code and verdicts.
- The final P2 behavior had no Candidate or verdict.
- The promised full repository audit did not run.
- No RPI report artifacts were found, and no program-level verdict exists.
- The initial dirty-state replay was checked by a transient diff hash, but no
  durable program-level baseline/final manifest isolated all session changes
  from inherited work for handoff.

Counterfactual: a fixed program acceptance map and aggregate report would have
shown "5 proven, 1 planned, final audit missing" before another slice began and
would have supported a useful partial handoff. If independent PASS verdicts
were enough, the caller would have received a complete deliverable; they did
not.

## Rejected or qualified claims

### "No work was produced"

Rejected at the filesystem level; supported at the delivery level.

Twelve Plans, ten Candidates, nine verdicts, packet runtime code, schemas,
evidence receipts, manifests, and skill changes exist. Five exact slices have
PASS verdicts. None of that became a completed, audited program result. The
distinction matters because optimizing artifact production would not fix the
reported failure; optimizing time-to-usable-outcome might.

### "The dirty source checkout caused the four-hour run"

Rejected as the primary cause.

Isolating and replaying the dirty checkout took about three minutes. The dirty
baseline increased attribution and generation complexity, but later one-file
and prose-only repairs still required 12-20 minute cycles. Dirty state was a
contributing condition, not a sufficient explanation.

### "Fresh validators simply hung"

Rejected.

Validators generally completed in four to ten minutes and wrote durable
verdicts. The larger delay came from choosing nine serial validators plus the
Plan and Implement agents around them. The 286 wait timeouts show long periods
of delegated work, not one indefinitely stuck validator.

### "Strict RPI behavior made all of this time necessary"

Qualified.

The boundaries caught real defects and preserved honest evidence. They also
converted formatting and harness mistakes into complete serial invocations,
and the main context automatically restarted after every stop. Safety explains
some cost; it does not explain the absence of budgets, decomposition limits,
minimal contexts, aggregate proof, or caller checkpoints.

### "The caller explicitly authorized unlimited autonomous repair"

Not proven.

"Orchestrate it to completion" supports continued work, but no repair count,
time budget, or waiver of RPI's caller-owned continuation boundary was stated.
The opposite interpretation—stop after every verdict—would also frustrate the
terminal request. The ambiguity is a contract gap that should be made explicit,
not retrospectively resolved as either unlimited authority or no authority.

## Unknowns and limitations

- The token counter is cumulative and includes cached input. It proves context
  amplification but does not directly measure monetary cost or model compute.
- The exact wall-clock share spent computing versus waiting is not recoverable
  from timeout counts alone. A 30-second timeout can represent useful child
  work.
- The initial dirty-state diff hash was reported in the session but was not
  persisted as a named baseline artifact, so this report cannot cleanly compute
  the total session-authored diff independently of the per-slice manifests.
- A counterfactual run with fewer agents or bounded in-phase repair has not been
  executed; expected savings are hypotheses until measured.
- The five PASS verdicts were not semantically revalidated here, by design.
- It is unknown whether the caller would have preferred stopping after the first
  P0 closure, after the runtime closure, or after a fixed time checkpoint. The
  missing continuation policy prevented that preference from being represented.

## Suggested experiments for improving the RPI loop

These are experiments for caller evaluation, not promoted rules or an
implementation plan.

### Experiment A: explicit continuation envelope

Replay a comparable multi-behavior request with an outer envelope that freezes:

- maximum RPI invocations;
- maximum repair invocations per behavior;
- wall-clock budget;
- maximum delegated agents;
- caller checkpoint conditions; and
- the exact aggregate completion map.

Compare time-to-first-usable-handoff, total invocations, and unfinished scope
against this session. A useful first treatment is 45 minutes, three invocations,
one repair per behavior, and a mandatory caller decision after any second
non-PASS result.

### Experiment B: packet-only phase contexts

Run matched Plan and Implement tasks with `fork_turns: none`, passing only the
caller intent, applicable skill contract, exact packet/evidence paths, subject
locator, and bounded repository references. Compare token input, wall time,
packet validity, scope errors, and validator outcomes against full-history
forks.

### Experiment C: bounded in-phase TDD correction

Under one frozen acceptance contract, compare the current one-post-check stop
policy with a small declared correction budget that records every attempt but
does not permit acceptance changes. Separately classify semantic failures,
test-harness defects, generated-receipt drift, and formatting-only failures.
Measure whether full repair invocations fall without masking real FAIL results.

### Experiment D: Plan complexity admission

Before Implement dispatch, score scenario count, evidence count, included path
owners, expected changed paths, generated companions, and distinct test suites.
Reject or decompose Plans above a threshold. Replay the 35-55-path migrations
as smaller packets and compare total time, repair rate, and aggregate proof.

### Experiment E: deterministic pre-verdict short circuit

For a Candidate whose required post-change evidence is mechanically MISMATCH,
compare immediate NOT_BUILT/deterministic failure reporting with spawning a
fresh semantic validator. Measure validator minutes saved and whether any case
would have received a materially different semantic classification.

### Experiment F: control-bearing progress

Add ephemeral progress fields for elapsed time, budget remaining, invocation
count, repair count, aggregate requirements proven/remaining, and the next hard
checkpoint. Compare caller interruption rate and time-to-stop against
activity-only progress. Keep provisional semantic judgment out of progress.

### Experiment G: aggregate baseline and completion receipt

At program start, persist a baseline manifest/diff identity that cleanly
separates inherited dirty state. After every RPI report, update a read-only
aggregate view mapping each original requirement to Plan, Candidate, verdict,
and remaining audit evidence. Test whether a partial but usable handoff can be
generated even when the final behavior is unfinished.

## Bottom line

The RPI micro-loop did what it was designed to do inside many individual
invocations: freeze acceptance, stop on bad evidence, and obtain fresh verdicts.
The session failed because that micro-loop was used as an unbounded program
runner without a program contract. Large Plans increased failure surface;
one-shot repair semantics converted small defects into full serial cycles;
full-history forks amplified context; progress lacked budgets; and slice-level
PASS results never rolled up into delivery.

The primary improvement target is therefore not "make validators faster." It is
to make continuation, boundedness, context transfer, and aggregate completion
explicit at the boundary above one RPI invocation, while experimentally testing
whether a small evidence-preserving correction budget belongs inside Implement.
