# Plugin evaluation of AgentOps 3.11 on Claude Opus 5.5 (2026-10-22 to 2026-10-23)

Five rounds of `claude plugin eval` against the AgentOps 3.11 plugin, run on
two days with every decision rule written down before its runs started. The
questions, in the order they were asked: does an agent with 3.11 installed
behave differently from one without it; did 3.11 change what the 3.10 edits
had measured; why do some skills not load; which skill fixes do the results
support; and did the consolidation shipped as
[#1235](https://github.com/boshu2/agentops/pull/1235) restore loading.

This report supersedes the [2026-10-05 report](2026-10-05-plugin-eval-opus-5-5.md)
as the current evidence for the shipped skills. The 2026-10-05 report remains
the record of 3.9.0 against 3.10.

## Result

| | No plugin | AgentOps 3.10 | AgentOps 3.11 |
|---|---:|---:|---:|
| Practice criteria met on the 29 requests both releases have | 265 of 387 (68%) | 363 of 387 (94%), stored 2026-10-05; 362 of 387 when re-run on 2026-10-22 | 704 of 774 (91%) |
| Matching skill loaded on its own request, 25 model-loadable skills | | 62 of 75 runs, stored and again when re-run | 104 of 150 runs |
| Practice criteria met on all 32 requests | 278 of 432 (64%) | | 744 of 864 (86%) |

The 3.11 arm is six runs per case (two batches of three), the other arms three.
Loading fell from 3.10 to 3.11 by 0.13 [-0.24, -0.01], Fisher exact p = 0.037.
The content difference, 91% against 94%, is inside the 0.15 noise bar set from
the A/A runs (90th percentile gap 0.13). The 3.10 re-run reproduced its stored
loading exactly, so the drop is not Claude Code drift.

## What this shows, and what it does not

- **The plugin changes what the agent does.** 16 of the 32 cases moved
  by 0.15 or more with 3.11. The largest gains were `claude-exec` (+0.77),
  `skill-builder` (+0.67), `plan` (+0.60), `craft-goal` (+0.58), `using-gc`
  (+0.50), `idea-genie` (+0.43), `interview` (+0.42) and `ergonomics` (+0.40).
- **Seven cases had no room to show a gain.** The agent met every criterion
  with no plugin on `agent-native`, `navigate`, `postmortem`, `reality-check`, `research`, `test`, `validate`. Two more scored the
  same with and without it: `orchestrate`, `triage`.
- **The rules 3.11 added help where they were tested.** On eleven new cases
  written from the 3.10 to 3.11 diff and run with the skill forced to load,
  3.11 beat 3.10 by more than the noise bar on `craft-goal` (+0.67),
  `navigate` (+0.47), `interview` (+0.42), `plan` (+0.40), `rpi` (+0.33) and
  `implement` (+0.27). `research`, `validate`, `postmortem` and `premortem`
  showed no detectable difference.
- **Loading is the constraint, not content.** Eleven skills loaded in fewer
  than four of six runs. Forced to load, the content of six of them beats the
  no-plugin arm clearly: `architecture` (+0.73), `memory` (+0.73), `implement`
  (+0.58), `triage` (+0.53), `council` (+0.25) and `refactor` (+0.25).
  `implement` and `research` never loaded on their own in any round: 0 of 25
  natural runs each across the suite's requests.
- **Four causes of the loading drop were tested; none explains it.** Not
  evaluator drift (the 3.10 re-run matched its stored loading). Not the empty
  workspace (in a seeded Git repository with edit tools, `implement` loaded
  0/12 and `research` 0/9 while `plan` loaded 3/3).
  Not Research's new description (reverting it changed nothing). Not any one
  skill: removing one entry moved the crowding probe from 2/6 to 3 or 4 of 6,
  removing three entries of any kind took it to 6/6, which pointed at listing
  length; but removing the nine workflow entries in round 5 left it at 6/12
  against 5/12. Every miss loaded no skill at all, so no rival description took
  the request.
- **Four skills show no headroom even on harder cases.** `validate`,
  `research`, `reality-check` and `test` scored 1.00 forced and 1.00 without
  the plugin on cases written to be harder. This eval cannot show their value
  on Opus 5.5; it does not show they have none. `navigate` is the one clear
  harder-case gain (+0.40 [+0.05, +0.64]).
- **Fixes were kept only when a rerun with the edited text met a bar set in
  advance.** Five did (security, ergonomics, two for navigate, one for
  agent-native) and shipped in
  [#1233](https://github.com/boshu2/agentops/pull/1233) and
  [#1234](https://github.com/boshu2/agentops/pull/1234). Craft-goal's
  template correction (a decision bead is not an RPI) also shipped in
  [#1233](https://github.com/boshu2/agentops/pull/1233) as a consistency fix
  with no rerun. Two did not (a skill-builder wording change, a review fix)
  and were dropped. Two new
  descriptions (triage, architecture) missed their adoption bar and were not
  shipped.
- **Naming a skill works.** After #1235, `rpi`, `postmortem`, `interview` and
  `craft-goal` loaded on 22 of 24 requests that named them in plain
  words and on 0 of 72 that did not.
- **This is not an outcome measurement.** The criteria were written from each
  skill's own rules. A pass shows a rule landed on one request. It does not
  show a better result on a real task, on another model or runtime (Codex,
  Gemini and Cursor were not measured), or at what cost. Six runs per case
  shows only large per-skill effects; pooled comparisons carry the statistics.

## Round 1: behavior and loading

Criteria met over six 3.11 runs, pooled, against three runs without the plugin.
"Delta" is the 3.11 rate minus the no-plugin rate with a Newcombe 95% interval;
criteria within a run are correlated, so intervals are directional. "Loaded" is
how many of the six 3.11 runs called the Skill tool for that skill. The verdict
follows the rule fixed before the runs: a skill loaded in fewer than four of six
runs gets "revise (loading)" because its content was not measured; "revise"
names one criterion missed in four or more loaded runs; "retain" is loaded in at
least four and within the noise bar of both the no-plugin and the stored 3.10
rate. "Remove" could not be issued at this sample size.

| Skill | With 3.11 | Without | Delta [95%] | 3.10 stored | Loaded (of 6) | Round-1 verdict |
|---|---:|---:|---:|---:|---:|---|
| `claude-exec` | 29/30 (0.97) | 0.20 | 0.77 [0.48, 0.90] | 0.93 | 6 | retain |
| `skill-builder` | 18/24 (0.75) | 0.08 | 0.67 [0.33, 0.81] | 0.67 | 6 | revise |
| `plan` | 26/30 (0.87) | 0.27 | 0.60 [0.30, 0.78] | 1.00 | 5 | retain |
| `craft-goal` | 24/24 (1.00) | 0.42 | 0.58 [0.29, 0.81] | 1.00 | by name | retain |
| `using-gc` | 24/24 (1.00) | 0.50 | 0.50 [0.22, 0.75] | 1.00 | 6 | retain |
| `idea-genie` | 29/30 (0.97) | 0.53 | 0.43 [0.18, 0.67] | 1.00 | 6 | retain |
| `interview` | 24/24 (1.00) | 0.58 | 0.42 [0.15, 0.68] | 1.00 | by name | retain |
| `ergonomics` | 18/30 (0.60) | 0.20 | 0.40 [0.09, 0.60] | - | 6 | revise |
| `memory` | 17/30 (0.57) | 0.20 | 0.37 [0.06, 0.57] | 1.00 | 2 | revise (loading) |
| `security` | 22/30 (0.73) | 0.40 | 0.33 [0.03, 0.57] | 0.87 | 6 | revise |
| `rpi` | 27/30 (0.90) | 0.60 | 0.30 [0.04, 0.55] | 0.80 | by name | retain |
| `council` | 24/24 (1.00) | 0.75 | 0.25 [0.04, 0.53] | 1.00 | 0 | revise (loading) |
| `refactor` | 24/24 (1.00) | 0.75 | 0.25 [0.04, 0.53] | 1.00 | 3 | revise (loading) |
| `domain` | 30/30 (1.00) | 0.80 | 0.20 [0.03, 0.45] | 1.00 | 6 | retain |
| `premortem` | 24/24 (1.00) | 0.83 | 0.17 [-0.02, 0.45] | 1.00 | 6 | retain |
| `skill-eval` | 29/30 (0.97) | 0.80 | 0.17 [-0.02, 0.42] | 1.00 | 4 | retain |
| `reverse-engineer` | 30/30 (1.00) | 0.87 | 0.13 [-0.02, 0.38] | 1.00 | 6 | retain |
| `doc` | 19/24 (0.79) | 0.67 | 0.12 [-0.15, 0.42] | 0.83 | 6 | revise |
| `codex-exec` | 20/24 (0.83) | 0.75 | 0.08 [-0.17, 0.38] | 0.92 | 6 | revise |
| `implement` | 12/24 (0.50) | 0.42 | 0.08 [-0.24, 0.37] | 0.50 | 0 | revise (loading) |
| `architecture` | 10/30 (0.33) | 0.27 | 0.07 [-0.22, 0.31] | - | 0 | revise (loading) |
| `review` | 23/24 (0.96) | 0.92 | 0.04 [-0.13, 0.31] | 1.00 | 0 | revise (loading) |
| `agent-native` | 30/30 (1.00) | 1.00 | 0.00 [-0.11, 0.20] | 1.00 | 3 | revise (loading) |
| `navigate` | 30/30 (1.00) | 1.00 | 0.00 [-0.11, 0.20] | 1.00 | 6 | retain |
| `orchestrate` | 20/24 (0.83) | 0.83 | 0.00 [-0.23, 0.30] | 0.83 | 6 | retain |
| `postmortem` | 24/24 (1.00) | 1.00 | 0.00 [-0.14, 0.24] | 1.00 | by name | retain |
| `reality-check` | 30/30 (1.00) | 1.00 | 0.00 [-0.11, 0.20] | 1.00 | 2 | revise (loading) |
| `research` | 24/24 (1.00) | 1.00 | 0.00 [-0.14, 0.24] | 1.00 | 0 | revise (loading) |
| `test` | 24/24 (1.00) | 1.00 | 0.00 [-0.14, 0.24] | 1.00 | 1 | revise (loading) |
| `triage` | 12/30 (0.40) | 0.40 | 0.00 [-0.29, 0.27] | - | 0 | revise (loading) |
| `validate` | 24/24 (1.00) | 1.00 | 0.00 [-0.14, 0.24] | 1.00 | 6 | retain |
| `agy-native` | 23/30 (0.77) | 0.80 | -0.03 [-0.25, 0.24] | 0.80 | 6 | revise |

Notes on single rows:

- `agy-native` c2 and `codex-exec` c3 failed every arm or read two ways; they
  were reworded in #1234 after this round. After rewording the plugin arm leads
  on each (`codex-exec` c3 3/6 against 0/3 without the plugin).
- `rpi` c3 expected a fresh reviewer for every change, which contradicts the
  rule both releases carry. Reworded in #1234; 6/6 against 0/3 after.
- `craft-goal`, `interview`, `postmortem` and `rpi` were slash-only in 3.11.0
  and were invoked as `/agentops:<skill>`, which injects the skill without a
  Skill tool call; their runs count as loaded.

## Round 2: where loading comes from

Three sources of loading, read together. The behavior request is the one the
descriptions were tuned on. The blind requests are seven per skill: the round-1
routing request three times, and two new requests, written by an agent that had
not seen the descriptions, twice each. A skill "routes" when it loads on at
least five of seven, is "request-sensitive" on one to four, and "does not route"
on zero. Of the 28 model-loadable skills, 16 route, 7 are
request-sensitive and 5 do not route.

| Skill | Behavior request, 3.11 (of 6) | 3.10 re-run (of 3) | 3.10 stored (of 3) | Blind requests, 3.11 (of 7) | Class | 3.10 stored blind (of 2) |
|---|---:|---:|---:|---:|---|---:|
| `agent-native` | 3/6 | 3/3 | 2/3 | 6/7 | routes | 2/2 |
| `agy-native` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `architecture` | 0/6 | new | new | 7/7 | routes | new |
| `claude-exec` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `codex-exec` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `council` | 0/6 | 0/3 | 0/3 | 7/7 | routes | 2/2 |
| `doc` | 6/6 | 3/3 | 3/3 | 0/7 | does not route | 0/2 |
| `domain` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `ergonomics` | 6/6 | new | new | 7/7 | routes | new |
| `idea-genie` | 6/6 | 3/3 | 3/3 | 2/7 | request-sensitive | 2/2 |
| `implement` | 0/6 | 0/3 | 0/3 | 0/7 | does not route | 0/2 |
| `memory` | 2/6 | 3/3 | 3/3 | 6/7 | routes | 2/2 |
| `navigate` | 6/6 | 3/3 | 3/3 | 4/7 | request-sensitive | 0/2 |
| `orchestrate` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `plan` | 5/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `premortem` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `reality-check` | 2/6 | 3/3 | 3/3 | 1/7 | request-sensitive | 0/2 |
| `refactor` | 3/6 | 3/3 | 2/3 | 2/7 | request-sensitive | 0/2 |
| `research` | 0/6 | 2/3 | 2/3 | 0/7 | does not route | 0/2 |
| `reverse-engineer` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `review` | 0/6 | 0/3 | 0/3 | 2/7 | request-sensitive | 0/2 |
| `security` | 6/6 | 3/3 | 3/3 | 0/7 | does not route | 0/2 |
| `skill-builder` | 6/6 | 3/3 | 3/3 | 6/7 | routes | 2/2 |
| `skill-eval` | 4/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `test` | 1/6 | 0/3 | 2/3 | 1/7 | request-sensitive | 1/2 |
| `triage` | 0/6 | new | new | 3/7 | request-sensitive | new |
| `using-gc` | 6/6 | 3/3 | 3/3 | 7/7 | routes | 2/2 |
| `validate` | 6/6 | 3/3 | 3/3 | 0/7 | does not route | 0/2 |

Skills with the same description in 3.10 and 3.11 also lost loading on the
behavior request (`reality-check` 3/3 to 2/6, `refactor` 3/3 to 3/6,
`agent-native` 3/3 to 3/6), so the cause sits at the level of the listing, not
only each skill's own wording.

### Content once forced to load

The round-1 behavior cases for the skills that loaded in fewer than four of six
runs, with the prompt prefixed `/agentops:<skill>` so the content is measured.
Three runs each, against the round-1 no-plugin arm.

| Skill | Forced 3.11 | Without | Delta [95%] |
|---|---:|---:|---:|
| `agent-native` | 15/15 (1.00) | 1.00 | +0.00 [-0.20, +0.20] |
| `architecture` | 15/15 (1.00) | 0.27 | +0.73 [+0.41, +0.89] |
| `council` | 12/12 (1.00) | 0.75 | +0.25 [-0.04, +0.53] |
| `implement` | 12/12 (1.00) | 0.42 | +0.58 [+0.22, +0.81] |
| `memory` | 14/15 (0.93) | 0.20 | +0.73 [+0.39, +0.87] |
| `reality-check` | 15/15 (1.00) | 1.00 | +0.00 [-0.20, +0.20] |
| `refactor` | 12/12 (1.00) | 0.75 | +0.25 [-0.04, +0.53] |
| `research` | 12/12 (1.00) | 1.00 | +0.00 [-0.24, +0.24] |
| `review` | 12/12 (1.00) | 0.92 | +0.08 [-0.17, +0.35] |
| `test` | 12/12 (1.00) | 1.00 | +0.00 [-0.24, +0.24] |
| `triage` | 14/15 (0.93) | 0.40 | +0.53 [+0.20, +0.74] |

### Rules added in 3.11

Eleven new cases, one per operating-loop skill, each testing a rule that the
3.10 to 3.11 diff added. The skill is forced in both plugin arms. Three runs
each. A 3.11 advantage over 3.10 above 0.15 counts as evidence the addition
helped on that case.

| Skill | 3.11 | 3.10 | Without | 3.11 minus 3.10 [95%] |
|---|---:|---:|---:|---:|
| `craft-goal` | 1.00 | 0.33 | 0.40 | +0.67 [+0.34, +0.85] |
| `navigate` | 1.00 | 0.53 | 0.53 | +0.47 [+0.17, +0.70] |
| `interview` | 1.00 | 0.58 | 0.67 | +0.42 [+0.09, +0.68] |
| `plan` | 0.93 | 0.53 | 0.53 | +0.40 [+0.08, +0.64] |
| `rpi` | 0.93 | 0.60 | 0.80 | +0.33 [+0.03, +0.58] |
| `implement` | 0.67 | 0.40 | 0.27 | +0.27 [-0.08, +0.54] |
| `review` | 0.73 | 0.60 | 0.60 | +0.13 [-0.19, +0.42] |
| `postmortem` | 1.00 | 0.93 | 0.93 | +0.07 [-0.14, +0.30] |
| `premortem` | 0.93 | 0.87 | 0.80 | +0.07 [-0.18, +0.32] |
| `validate` | 1.00 | 0.93 | 1.00 | +0.07 [-0.14, +0.30] |
| `research` | 0.93 | 0.93 | 0.80 | +0.00 [-0.24, +0.24] |

Grader calibration on these cases: plainly passing answers met 53 of 54
criteria, plainly failing answers 0 of 54.

## Round 3: listing ablation and a seeded repository

Both experiments were run before any fix was decided. Loads of six per cell,
natural prompts.

| Skill | Full 3.11 | Research description reverted to 3.10 | Without architecture, triage, ergonomics |
|---|---:|---:|---:|
| `reality-check` | 1/6 | 0/6 | 2/6 |
| `refactor` | 5/6 | 3/6 | 3/6 |
| `agent-native` | 1/6 | 4/6 | 6/6 |
| `memory` | 4/6 | 3/6 | 5/6 |
| `skill-eval` | 5/6 | 6/6 | 6/6 |
| pooled, unchanged descriptions (reality-check, refactor, agent-native) | 7/18 | 7/18 | 11/18 |

The pre-set rule asked for at least 15 of 18 on the unchanged-description
skills when the three new skills were removed. 11 of 18 (+0.22 [-0.10, +0.49],
p = 0.32) did not meet it, so the cause stayed unexplained. Directionally,
`agent-native` went from 1/6 to 6/6 without the new skills.

In a working directory seeded with the files each prompt pastes, as a Git
repository with Edit, Write and Bash tools, `implement` loaded 0/12 and
`research` 0/9 while the `plan` control loaded 3/3. The runs edited
code and ran Git. The empty, edit-less environment does not explain the gap.

## Round 4: crowding, descriptions, harder cases and fixes

Base: main at 0c30caf, after #1233.

### Crowding

The `agent-native` behavior case, six runs per arm, arms run concurrently. Its
content passes with or without the plugin, so only loading moves.

| Listing | agent-native loaded (of 6) |
|---|---:|
| Full 3.11 (control) | 2 |
| Without architecture | 3 |
| Without triage | 3 |
| Without ergonomics | 4 |
| Without using-gc, agy-native, reverse-engineer | 6 |

The pre-set rule read this as listing length: no single removal reached 5/6,
and the size-matched removal of three unrelated skills did. One case, six runs
per arm; entry count and characters move together in every arm, so the test
cannot separate them.

### Description trials

A new `triage` description loaded on 3 of 6 new disposition-shaped requests
against 1 of 6 for the shipped one; the bar was 4 of 6, so it was not adopted.
A new `architecture` description loaded 0 of 6 on its behavior case against 0
of 6; not adopted. Neither trial took a request away from `plan` or
`navigate`.

### Harder cases

New cases for the eight skills whose round-1 case left no room, with the skill
forced to load, run with and without the plugin and once more naturally for
loading. Three runs each. A case is useful only when the no-plugin arm scores
at or below 0.70.

| Skill | Forced 3.11 | Without | Delta [95%] | Natural loads (of 3) | Room to measure |
|---|---:|---:|---:|---:|---|
| `navigate` | 9/15 (0.60) | 0.20 | +0.40 [+0.05, +0.64] | 2 | yes |
| `review` | 6/12 (0.50) | 0.33 | +0.17 [-0.20, +0.48] | 0 | yes |
| `postmortem` | 12/12 (1.00) | 0.75 | +0.25 [-0.04, +0.53] | 0 | no |
| `agent-native` | 10/12 (0.83) | 0.75 | +0.08 [-0.24, +0.39] | 0 | no |
| `validate` | 12/12 (1.00) | 1.00 | +0.00 [-0.24, +0.24] | 0 | no |
| `research` | 9/9 (1.00) | 1.00 | +0.00 [-0.30, +0.30] | 0 | no |
| `reality-check` | 12/12 (1.00) | 1.00 | +0.00 [-0.24, +0.24] | 0 | no |
| `test` | 15/15 (1.00) | 1.00 | +0.00 [-0.20, +0.20] | 0 | no |

Grader calibration on these cases: passing answers 33 of 33 criteria, failing
answers 1 of 33.

### Fixes

A proposed skill-text fix was kept only if, on three fresh forced runs of the
same case with the edited skill, its target criterion passed in at least two
(from one or fewer) and the case's other criteria lost no more than one run in
total. Results, as shipped:

| Skill | Criterion | Before | After | Shipped in |
|---|---|---:|---:|---|
| `security` | no ship, hold or block call | 1/6 | 5/6 | #1233 |
| `security` | findings marked proven or suspected | 3/6 | 5/6 | #1233 |
| `ergonomics` | consequence and fix line under every finding | 0/6 | 6/6 | #1233 |
| `ergonomics` | unexercised paths listed as gaps | 3/6 | 6/6 | #1233 |
| `ergonomics` | no score on any scale | 4/6 | 6/6 | #1233 |
| `navigate` | retypes a decision filed as build work | 1/3 | 2/3 | #1234 |
| `navigate` | leaves the caller's question open | 0/3 | 3/3 | #1234 |
| `agent-native` | compares a lane with the current target by content | 1/3 | 2/3 | #1234 |
| `skill-builder` | no new hook or gate from one incident | 6/6 proposing one | 6/6 | dropped |
| `review` | c2 on its harder case | 0/3 | 0/3 | dropped |

The #1233 fixes used a bar of at least 5 of 6 on six runs. Criteria for `rpi`,
`doc`, `orchestrate`, `codex-exec` and `agy-native` that failed every arm by
design or read two ways were reworded in #1234; stored runs were regraded for
consistency only, never to claim a gain.

## Round 5: the consolidation candidate

Subjects run concurrently in the same time windows: main at 0c30caf (3.11 plus
#1233) and the #1235 candidate at e8638dfa2 (no workflow entries; `rpi`,
`postmortem`, `interview` and `craft-goal` loadable by name; Reality Check
folded into Research). Loading only, read from each run's Skill calls. 168
runs.

| Case | Main (3.11 + #1233) | Candidate (#1235) |
|---|---:|---:|
| `agent-native` behavior | 5/12 | 6/12 |
| `implement` behavior | 0/6 | 0/6 |
| `research` behavior | 0/6 | 0/6 |
| `memory` behavior | 3/6 | 6/6 |
| `refactor` behavior | 3/6 | 5/6 |
| `skill-eval` behavior | 4/6 | 4/6 |
| `architecture` behavior | 0/6 | 0/6 |
| `triage` behavior | 0/6 | 0/6 |
| `plan` behavior | 5/6 | 6/6 |
| `navigate` behavior | 6/6 | 6/6 |
| `reality-check` prompt (candidate counts Research loads) | 3/6 | 0/6 |

| Rule, fixed before the candidate was built | Result | Verdict |
|---|---|---|
| B1: `agent-native` loads at least 10/12 on the candidate | 6/12 against 5/12, p = 1.0 | fail |
| B2: no pooled harm over nine skills (54 runs per arm) | 27/54 against 21/54, +0.11 [-0.07, +0.29] | pass |
| B3: `implement` and `research` natural loads, report only | 0/6 and 0/6 on both arms | reported |
| B4: the Reality Check prompt loads Research at least 3/6 | 0/6 against Reality Check 3/6 on main | fail |
| B5: each by-name skill loads at least 4/6 when named | `rpi` 6/6, `postmortem` 6/6, `interview` 6/6, `craft-goal` 4/6 | pass |
| B5: by-name skills load at most once on 72 unnamed requests | 0/72 | pass |

The workflow cut did not restore loading, so the size effect seen when removing
skills did not carry over to removing workflow entries. The owner kept the fold
and the removal on their other grounds (unused, and the owner does not use the
workflows). Claim-audit requests now load nothing on this model, where Reality
Check had shown no content gain.

## Decisions taken from these results

- [#1233](https://github.com/boshu2/agentops/pull/1233): the security and
  ergonomics fixes above, and the unmeasured craft-goal template correction.
- [#1234](https://github.com/boshu2/agentops/pull/1234): the navigate and
  agent-native fixes; cases for triage, architecture and ergonomics; the
  reworded criteria. The suite now covers every skill.
- [#1235](https://github.com/boshu2/agentops/pull/1235): the workflows removed
  from the plugin and the repository; four skills loadable by name; Reality
  Check folded into Research. 32 skills become 31.
- Open, for the owner: `implement` and `research` still never load on their
  own, and no tested cause explains the 3.10 to 3.11 loading drop. Cutting or
  consolidating skills has not been shown to help loading.

## Method

- **Evaluator:** `claude plugin eval`, Claude Code 2.1.308 for round 1 and
  2.1.309 for rounds 2 to 5. Model `claude-opus-5-5`. Each run had a sandboxed
  home, the Read, Glob, Grep and Skill tools (round 3's seeded runs added
  Edit, Write and Bash) and at most 12 turns.
- **Subjects:** 3.11 is the v3.11.0 `plugin/` tree. 3.10 is the stored
  2026-10-05 scorecard plus a same-day re-run of its 29 behavior cases. Round 4
  ran on main at 0c30caf; round 5 compared that with the #1235 candidate at
  e8638dfa2.
- **Cases:** the repository suite in [`evals/plugin-eval/`](../../evals/plugin-eval/README.md)
  holds the behavior and routing cases, including the three written on
  2026-10-22 for triage, architecture and ergonomics and the criteria reworded
  in #1234. The other case sets (56 blind routing requests, 11 method cases, 8
  harder cases, 2 triage disposition requests and 8 by-name requests) and the
  pre-registered decision rules for each round live in the run directory
  outside Git, with every prompt, response and trace.
- **Grading:** `evals/plugin-eval/grade.py`, one Claude Opus 5.5 call per
  response. Calibrated on the new cases before each round: round 1 passing
  answers 15 of 15, failing 1 of 15; round 2 53 of 54 and 0 of 54; round 4 33
  of 33 and 1 of 33. Loading uses the evaluator's deterministic check on the
  Skill tool call.
- **Statistics:** Wilson 95% per arm, Newcombe hybrid interval for a
  difference, Fisher exact two-sided. Noise bar 0.15, the larger of 0.15 and
  the 90th percentile over cases of the A/A gap (0.13).
- **Cost:** about $158 of evaluator spend over the five rounds (48.04,
  51.81, 15.88, 21.77 and 20.45). Grading was not metered; round 1's 288
  responses cost about $6.
- **Scorecards:** round 1 in
  [`evals/plugin-eval/scorecards/2026-10-22-opus-5-5.json`](../../evals/plugin-eval/scorecards/2026-10-22-opus-5-5.json),
  the same shape as the 2026-10-05 scorecard. Rounds 2 to 5, as written by each
  round's analyzer, in
  [`evals/plugin-eval/scorecards/2026-10-22-rounds-2-5-opus-5-5.json`](../../evals/plugin-eval/scorecards/2026-10-22-rounds-2-5-opus-5-5.json).
  Counts only; no prompt or response text.

## Departures from a single clean run

- Round 1's rule was corrected after reading only the first interview result:
  the four slash-only skills inject without a Skill tool call, so their runs
  count as loaded unless the run errored.
- Round 1 stopped at the $48 evaluator limit before three routing cases
  (`claude-exec`, `agy-native`, `using-gc`); round 2 ran them first.
- Round 2's first forced-case build failed to parse in the evaluator (YAML
  emitted by a script) and spent nothing; it was rebuilt by text edit. Seven
  method criteria were sharpened before any run, and two weak ones noted.
- Round 3's sandbox broke the Xcode `git` shim; agents found Homebrew's `git`
  themselves at the cost of a turn or two.
- Round 4 briefly inflated the runner's spend total by about $8 with regrade
  inputs written under its output directory; moved before any run was skipped.
- Round 5 changed several listing properties at once (entry count, four new
  entries, one removed skill, new descriptions, position), so B1 and B2
  measure the combined listing, not each part.

## Reproduce

The repository suite, as on 2026-10-05:

```bash
OUT=$(mktemp -d)   # run output holds every prompt and response; keep it out of the repository
claude plugin eval . --eval-dir evals/plugin-eval/behavior \
  --model claude-opus-5-5 --no-publish --json "$OUT/behavior.json"
python3 evals/plugin-eval/grade.py "$OUT/behavior.json" --out "$OUT/grades.json"

claude plugin eval . --eval-dir evals/plugin-eval/routing \
  --ablation none --model claude-opus-5-5 --runs 2 --no-publish --json "$OUT/routing.json"
```

Forced loading is a prompt prefixed `/agentops:<skill>`. The other case sets
are not in the repository.
