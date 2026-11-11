# Recheck of 3.12.0 after the catalog audit (2026-11-11)

AgentOps 3.12.0 shipped 28 skills, 24 of them changed by an audit that was not
measured. This run measured all 28. One change regressed: the audit rewrote
Orchestrate's description, and with the new line Orchestrate loaded on 6 of 30
runs where the 3.11 text loaded its skill on 21 of 30. The same line also cut
Agy Native's loading from 10 of 12 runs to 4 of 12, although Agy Native's own
text did not change. A new description line, chosen in a same-day trial, loads
Orchestrate on 22 of 30 runs and Agy Native on 6 of 6. It ships in 3.12.1.

No other skill fell more than 0.15 below its same-judge 3.11 score on a case
with headroom, except Implement when named (10/12 against 12/12), which a
same-day re-run did not reproduce (24/24 on both). Smaller differences in
either direction are inside the noise bar.

## Subject and method

The subject is `plugin/` at tag `v3.12.0`, commit
`5221ed81b1d8fd03532809910de884a687080c79`, Git tree
`e1dc4723aaa0c6f8e9b45f6cdac2835425f57a50`; its `skills/` directory was compared
with the installed 3.12.0 plugin and is identical. The runner was Claude Code
2.1.309 with `claude-opus-5-5`, publishing disabled, raw output kept outside
Git. Medium effort was requested with `CLAUDE_CODE_EFFORT_LEVEL`; the evaluator
has no effort option and its traces do not record effort, so medium is requested,
not observed.

The suite is the committed one: 42 behavior cases and 30 routing cases covering
28 skills, three runs per case. Each behavior case ran in three arms:

- **without**: no plugin;
- **as typed**: the plugin installed and the request as committed;
- **by name**: the same request prefixed with `/agentops:<skill>`, so the skill
  text is in play whether or not the agent would have loaded it.

Craft Goal, Postmortem and RPI requests already name the skill, so their
as-typed and by-name requests are the same.

The rules were written down before any paid run: a case has headroom when its
without score is 0.70 or below. A case is suspect when its with-skill score is
more than 0.15 below the stored 3.11 score, or a by-name run misses a criterion
in two of three runs; a skill is suspect when it loads at least twice less than
3.11.0 did on the same requests. A suspect is confirmed as a regression when
the same judge puts 3.12 more than 0.15 below 3.11 on a case with headroom, or
when its loading rate is lower by 0.33 or more over six runs per request
against the 3.11 text run the same day. Before the first targeted run it was
also fixed that Implement's stored-response difference, whose cause was
unknown, would count only if the same-day arm showed it. No case, criterion, grader or gate was changed.

### Grading

Claude Fable 5.2 graded every response through the judge call in
`evals/plugin-eval/grade.py`, imported unchanged: one call per response, all of
its criteria at once. Before counted grading it gave the expected grade to all
34 hand-written calibration answers from the 2026-10-26 round.

Grading was blind to arm and to version. A worker assigned each response an
opaque id, shuffled them and wrote the id-to-arm mapping to a sealed file; the
judge saw only criteria and response text; the mapping was opened after every
response had a verdict. The 3.12 responses were mixed with 399 stored 3.11
responses from the 2026-10-22 and 2026-10-26 runs, graded against the current
criteria of the case with the identical request. That is the same-judge 3.11
result used below. Thirty-six stored responses were left out because no current
case has their request (the old Interview request, Skill Builder's changed
request, and the two removed skills). In all, 945 responses were graded and
none was left ungraded.

On the 336 stored responses whose criteria are unchanged, the Fable grades
agree with the stored Opus 5.5 grades on 1,465 of 1,503 criteria (97.5%).

Loading is not judged. It is the evaluator's `skill-loaded` check on the Skill
tool call.

## Results for 3.12.0

| | Without | As typed | By name |
|---|---:|---:|---:|
| Practice criteria met, 42 cases | 372 of 573 (65%) | 511 of 573 (89%) | 551 of 573 (96%) |
| On the 20 cases with headroom | 122 of 282 (43%) | 233 of 282 (83%) | 266 of 282 (94%) |

The matching skill loaded on 65 of 114 as-typed behavior runs and on 62 of 90
routing runs. On the 23 routing requests unchanged since 3.11.0, 3.12.0 loaded
the skill 44 times and the stored 3.11.0 run 41 times. Implement went from 0 of
3 to 3 of 3, the result of the description line landed on 2026-10-31.

Against the same-judge 3.11 result, on the 41 cases with a stored as-typed
response, 3.12.0 met 499 of 558 criteria (89%) and 3.11 met 822 of 960 (86%).
On the 22 cases with a stored by-name response, 3.12.0 met 278 of 291 and 3.11
met 283 of 291.

### By skill

Every skill has a graded result. Routing counts are loads over runs; the last
column is the same routing suite on the repaired tree.

| Skill | Behavior cases graded | Without | As typed | By name | Routing cases run | Routing loads | Routing loads, repaired |
|---|---:|---:|---:|---:|---:|---:|---:|
| agy-native | 2 of 2 | 17/30 | 23/30 | 30/30 | 1 of 1 | 3/3 | 3/3 |
| architecture | 1 of 1 | 3/15 | 4/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| claude-exec | 1 of 1 | 4/15 | 14/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| codex-exec | 2 of 2 | 18/24 | 21/24 | 24/24 | 1 of 1 | 3/3 | 3/3 |
| council | 2 of 2 | 18/24 | 21/24 | 24/24 | 1 of 1 | 3/3 | 3/3 |
| craft-goal | 1 of 1 | 4/12 | 12/12 | 12/12 | 1 of 1 | 3/3 | 3/3 |
| doc | 1 of 1 | 9/12 | 12/12 | 12/12 | 1 of 1 | 0/3 | 0/3 |
| domain | 1 of 1 | 12/15 | 15/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| ergonomics | 1 of 1 | 6/15 | 15/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| idea-genie | 1 of 1 | 7/15 | 15/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| implement | 1 of 1 | 5/12 | 6/12 | 10/12 | 1 of 1 | 3/3 | 3/3 |
| interview | 1 of 1 | 5/12 | 12/12 | 12/12 | 1 of 1 | 3/3 | 3/3 |
| memory | 1 of 1 | 3/15 | 11/15 | 14/15 | 1 of 1 | 3/3 | 3/3 |
| navigate | 2 of 2 | 20/30 | 27/30 | 25/30 | 1 of 1 | 0/3 | 0/3 |
| orchestrate | 3 of 3 | 35/45 | 36/45 | 38/45 | 2 of 2 | 3/6 | 6/6 |
| plan | 1 of 1 | 5/15 | 14/15 | 15/15 | 1 of 1 | 2/3 | 3/3 |
| postmortem | 2 of 2 | 23/27 | 27/27 | 27/27 | 1 of 1 | 3/3 | 3/3 |
| premortem | 2 of 2 | 18/24 | 24/24 | 24/24 | 1 of 1 | 3/3 | 3/3 |
| refactor | 1 of 1 | 8/12 | 12/12 | 12/12 | 1 of 1 | 1/3 | 0/3 |
| research | 4 of 4 | 44/51 | 45/51 | 48/51 | 2 of 2 | 0/6 | 0/6 |
| review | 2 of 2 | 23/27 | 27/27 | 26/27 | 1 of 1 | 0/3 | 0/3 |
| rpi | 1 of 1 | 9/15 | 15/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| security | 1 of 1 | 9/18 | 17/18 | 16/18 | 1 of 1 | 0/3 | 0/3 |
| skill-builder | 1 of 1 | 8/15 | 12/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| skill-eval | 1 of 1 | 12/15 | 15/15 | 15/15 | 1 of 1 | 3/3 | 3/3 |
| test | 2 of 2 | 22/24 | 24/24 | 24/24 | 1 of 1 | 2/3 | 3/3 |
| triage | 1 of 1 | 6/15 | 11/15 | 14/15 | 1 of 1 | 3/3 | 3/3 |
| validate | 2 of 2 | 19/24 | 24/24 | 24/24 | 1 of 1 | 0/3 | 0/3 |

The routing suite stopped at its spend cap one run short, so Validate's routing
case had two runs; the third was run with the first targeted batch and is
included (0 of 3, as on 3.11.0).

### By case

"Named" marks a request that names its skill, which has no loading check. A
dash means no stored 3.11 response exists for that request and arm.

| Case | Headroom | Without | As typed (loads) | By name | 3.11 as typed, same judge | 3.11 by name, same judge |
|---|---|---:|---:|---:|---:|---:|
| `agy-native` | no | 11/15 | 14/15 (3/3) | 15/15 | 25/30 | - |
| `agy-native-harder` | yes | 6/15 | 9/15 (1/3) | 15/15 | 12/15 | 15/15 |
| `architecture` | yes | 3/15 | 4/15 (0/3) | 15/15 | 9/30 | 15/15 |
| `claude-exec` | yes | 4/15 | 14/15 (3/3) | 15/15 | 29/30 | - |
| `codex-exec` | no | 9/12 | 12/12 (3/3) | 12/12 | 21/24 | - |
| `codex-exec-harder` | no | 9/12 | 9/12 (0/3) | 12/12 | 9/12 | 12/12 |
| `council` | no | 9/12 | 12/12 (0/3) | 12/12 | 24/24 | 12/12 |
| `council-vote-tally` | no | 9/12 | 9/12 (0/3) | 12/12 | 10/12 | 12/12 |
| `craft-goal` | yes | 4/12 | 12/12 (named) | 12/12 | 24/24 | - |
| `doc` | no | 9/12 | 12/12 (3/3) | 12/12 | 24/24 | - |
| `domain` | no | 12/15 | 15/15 (3/3) | 15/15 | 30/30 | - |
| `ergonomics` | yes | 6/15 | 15/15 (3/3) | 15/15 | 21/30 | - |
| `idea-genie` | yes | 7/15 | 15/15 (3/3) | 15/15 | 29/30 | - |
| `implement` | yes | 5/12 | 6/12 (0/3) | 10/12 | 13/24 | 12/12 |
| `interview` | yes | 5/12 | 12/12 (named) | 12/12 | 12/12 | - |
| `memory` | yes | 3/15 | 11/15 (2/3) | 14/15 | 15/30 | 14/15 |
| `navigate` | no | 15/15 | 15/15 (3/3) | 15/15 | 30/30 | - |
| `navigate-hard` | yes | 5/15 | 12/15 (0/3) | 10/15 | 16/30 | - |
| `orchestrate` | yes | 8/15 | 9/15 (0/3) | 13/15 | 27/30 | - |
| `orchestrate-packets` | no | 15/15 | 15/15 (0/3) | 15/15 | 30/30 | 15/15 |
| `orchestrate-packets-harder` | no | 12/15 | 12/15 (0/3) | 10/15 | 11/15 | 12/15 |
| `plan` | yes | 5/15 | 14/15 (3/3) | 15/15 | 26/30 | - |
| `postmortem` | no | 12/12 | 12/12 (named) | 12/12 | 24/24 | - |
| `postmortem-harder` | no | 11/15 | 15/15 (3/3) | 15/15 | 15/15 | 15/15 |
| `premortem` | no | 12/12 | 12/12 (3/3) | 12/12 | 24/24 | - |
| `premortem-harder` | yes | 6/12 | 12/12 (3/3) | 12/12 | 11/12 | 11/12 |
| `refactor` | yes | 8/12 | 12/12 (3/3) | 12/12 | 23/24 | 12/12 |
| `research` | no | 12/12 | 12/12 (0/3) | 12/12 | 24/24 | 12/12 |
| `research-absence-proof` | no | 9/12 | 9/12 (0/3) | 12/12 | 9/12 | 12/12 |
| `research-claim-check` | no | 15/15 | 15/15 (0/3) | 15/15 | 30/30 | 15/15 |
| `research-claim-check-harder` | yes | 8/12 | 9/12 (0/3) | 9/12 | 9/12 | 9/12 |
| `review` | no | 12/12 | 12/12 (0/3) | 12/12 | 24/24 | 12/12 |
| `review-merge-base` | no | 11/15 | 15/15 (3/3) | 14/15 | 13/15 | 15/15 |
| `rpi` | yes | 9/15 | 15/15 (named) | 15/15 | 30/30 | - |
| `security` | yes | 9/18 | 17/18 (3/3) | 16/18 | 29/36 | - |
| `skill-builder` | yes | 8/15 | 12/15 (3/3) | 15/15 | - | - |
| `skill-eval` | no | 12/15 | 15/15 (3/3) | 15/15 | 29/30 | - |
| `test` | no | 12/12 | 12/12 (1/3) | 12/12 | 24/24 | 12/12 |
| `test-isolated-revert` | no | 10/12 | 12/12 (1/3) | 12/12 | 10/12 | 12/12 |
| `triage` | yes | 6/15 | 11/15 (3/3) | 14/15 | 12/30 | 15/15 |
| `validate` | no | 12/12 | 12/12 (3/3) | 12/12 | 24/24 | - |
| `validate-mutating-check` | yes | 7/12 | 12/12 (3/3) | 12/12 | 11/12 | 12/12 |

The six skills the audit named as most likely to differ: Doc, Premortem,
Security and Triage score at or above their same-judge 3.11 results as typed;
Triage by name is 14/15 against 15/15, inside the noise bar. Doc's case has no headroom (9 of 12 without the plugin), so it does not
test the handoff rules that moved to a reference. Orchestrate regressed, and
Implement is covered below.

## Suspected regressions and their confirmation

| Suspect | First sign | Method | Same-judge or same-day result | Reading |
|---|---|---|---|---|
| Orchestrate, loading | 0 of 3 loads on its own request (6 of 6 on 3.11.0); 0 of 9 on the two packet requests it took over from Agent Native | 3.11 text as a same-day third arm, 6 runs per request, five requests | 3.11 text 21 of 30; 3.12.0 6 of 30 | confirmed |
| Orchestrate, score on its own case as typed | 9/15 | stored 3.11 responses regraded, and the same-day arm | stored 27/30 against 9/15; same day 23/30 against 19/30 | follows the loading: when the skill does not load, the one-at-a-time integration criterion is met in 0 of 6 runs |
| Agy Native, loading on its harder case | 1 of 3 loads; 9/15 against a same-judge 12/15 | 3.11 text as a same-day third arm, 6 runs, run twice | 3.11 text 10 of 12; 3.12.0 4 of 12 | confirmed |
| Implement, by name | 10/12 against a same-judge 12/12 from stored responses, which meets the score rule; one criterion missed in 2 of 3 runs | 3.11 text as a same-day third arm, 6 runs | 24/24 on both | not reproduced; no change made |

Three other cases were flagged only because a by-name run missed a criterion
in two of three runs (`navigate-hard`, `research-claim-check-harder`,
`security`). Each scores at or above its same-judge 3.11 result, so none is a
regression.

### Cause

Orchestrate's description was the same in 3.11.0 and on main just before the
audit. The audit merged Agent Native into Orchestrate and rewrote the line to
cover launching, without a trial, as the 3.12.0 notes said. Two checks tie the
loading drop to that line:

- A copy of 3.12.0 that differs only by restoring the 3.11 line loads
  Orchestrate on its own request 12 of 12 times; 3.12.0 loads it 6 of 12.
- Agy Native's description never changed, yet it loads on 6 of 6 runs on main
  just before the audit, 10 of 12 on 3.11.0 and 4 of 12 on 3.12.0. Two copies of
  3.12.0 that differ only in Orchestrate's line load it 5 of 6 and 6 of 6.

Why one skill's line changes whether another loads is not explained. In the
runs that missed, no skill loaded; none loaded Orchestrate in Agy Native's
place.

## Description trial

Four candidate lines were tried, each as a copy of 3.12.0 with only
Orchestrate's description replaced. Every candidate was run through the
180-character budget check and the routing goldens before its paid runs. A
candidate is adopted when its pooled loading is at or above the 3.11 text's, no
request loads less than on 3.12.0, its score on Orchestrate's own case is at or
above the 3.11 text's, and both gates pass unchanged.

| Subject | Own request (12 runs) | Packet requests (18 runs) | Pooled | Own case score | Gates |
|---|---:|---:|---:|---:|---|
| 3.11 text | 12 | 9 (Agent Native) | 21 of 30 | 23/30 | n/a |
| 3.12.0 | 6 | 0 | 6 of 30 | 19/30 | pass |
| D1, the 3.11 line restored | 12 | 0 | 12 of 30 | 25/30 | fails 1 of 12 goldens |
| D2, a merged line | 12 | 0 | 12 of 30 | 26/30 | pass |
| D3, with "planning parallel work, even two" | 12 | 3 | 15 of 30 | 27/30 | pass |
| D4, with "running or planning agents in parallel, even two" | 12 | 10 | 22 of 30 | 25/30 | pass |

D4 is adopted:

> Dispatch parallel workers without write collisions and decide what idle or
> finished ones do next. Use when: managing agents, or running or planning
> agents in parallel, even two.

The packet requests loaded only with the clause the audit had dropped from
Agent Native's old line. The harder packet request still loads rarely (1 of 6
with D4, 0 of 6 on the 3.11 text).

## Re-run on the repaired tree

The repaired `plugin/` tree is byte-identical to the tested D4 subject, so the
Orchestrate and Agy Native loading rows are the D4 trial runs; the routing suite
and Agy Native's plain case were run on the repaired tree itself.

| Check | 3.11 text | 3.12.0 | Repaired |
|---|---:|---:|---:|
| Orchestrate loads, five requests, 6 runs each | 21 of 30 | 6 of 30 | 22 of 30 |
| Orchestrate's own case, as typed | 23/30 | 19/30 | 25/30 |
| `orchestrate-packets`, as typed | 27/30 | 29/30 | 30/30 |
| `orchestrate-packets-harder`, as typed | 25/30 | 24/30 | 23/30 |
| Agy Native loads on its harder case, 6 runs | 5 of 6 | 2 of 6 | 6 of 6 |
| `agy-native-harder`, as typed | 17/30 | 19/30 | 30/30 |
| `agy-native`, as typed, 3 runs | stored 25/30 | 14/15 | 13/15 |
| Routing loads, 30 requests, 3 runs each | | 62 of 90 | 66 of 90 |

`orchestrate-packets-harder` has no headroom (12 of 15 without the plugin), so
its two-criterion difference is not a regression under the rule. On the
repaired tree no routing request loaded Orchestrate in place of its own skill,
and no skill lost two or more routing loads; Refactor lost one (1 of 3 to
0 of 3) and Plan and Test each gained one.

## Accounting

| Launch | Run attempts | Evaluator-reported cost |
|---|---:|---:|
| Full suite: without, as typed, by name, routing | 467 | $64.51 |
| Targeted 1: 3.11 text, 3.12.0, D1, D2; Implement; Agy Native; the cut Validate run | 145 | $18.23 |
| Targeted 2: D3, D4; Agy Native on five subjects | 90 | $11.65 |
| Targeted 3: repaired tree, routing and Agy Native's plain case | 96 | $17.09 |

That is 798 attempts with no errored run and $111.47 as the evaluator reports
it, not a verified invoice. The 945 judge calls and 34 calibration calls are
not in that figure; `grade.py` does not report their spend.

The [scorecard](../../evals/plugin-eval/scorecards/2026-11-11-recheck-opus-5-5.json)
holds the per-case and per-skill counts, every boolean grade, the registered
rules and the loading counts of each targeted subject. Prompts, responses,
session identities and local paths are excluded.

## Limits

- One model and one runtime: Claude Code with Claude Opus 5.5. Nothing here
  describes Codex, Gemini, Cursor or another Claude model.
- Three runs per case in the full suite and six per request in the targeted
  runs detect only large changes. A difference under about 0.15 is noise.
- The criteria come from each skill's own rules. A pass shows the rule landed
  on that request, not a better outcome on a real task.
- Twenty-two of 42 cases score above 0.70 without the plugin, so they cannot
  show a content regression. Codex Exec, Council, Doc, Domain, Postmortem, Review, Skill Eval
  and Test are measured only on such cases.
- The 3.11 comparison is to 3.11.0. Twelve skill commits sit between 3.11.0 and
  3.12.0, so a difference from 3.11 is not by itself evidence about the audit;
  the pre-audit subject was run only for Agy Native.
- The description candidates were written after seeing these requests, so the
  loading counts are in-sample for the adopted line.
- Skill Builder's request changed after 3.11, so it has no same-judge 3.11
  result. It scores 12/15 as typed and 15/15 by name.
- Doc, Navigate, Research, Review, Security and Validate did not load on their
  routing request on either version. That is unchanged from 3.11.0 and was not
  in scope here.
