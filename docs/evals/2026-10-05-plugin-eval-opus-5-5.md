# Plugin evaluation on Claude Opus 5.5 (2026-10-05)

Two questions. Does an agent with the AgentOps plugin installed behave
differently from one without it? And did the 3.10 skill edits change that?

## Result

With AgentOps 3.10 installed, Claude Opus 5.5 met 363 of 387 practice criteria
(94%) across 29 requests, one per skill, three runs each. With no plugin it
met 272 of 387 (70%).

On the 28 skills that also exist in 3.9.0:

| | Criteria met | Matching skill loaded |
|---|---:|---:|
| No plugin | 269 of 372 (72%) | |
| AgentOps 3.9.0 | 292 of 372 (78%) | 22 of 72 runs |
| AgentOps 3.10 | 349 of 372 (94%) | 59 of 72 runs |

Loading is counted over the 24 of those skills that the model can load by
itself. The other four are invoked by name.

Those 28 requests were also used to tune the 3.10 descriptions, so loading on
them flatters 3.10. A second set of 25 requests was written by an agent that had
not seen the descriptions, and was not used for tuning. On that set, over the 24
skills both releases have:

| | Matching skill loaded |
|---|---:|
| AgentOps 3.9.0 | 11 of 48 runs |
| AgentOps 3.10 | 29 of 48 runs |

With the new `claude-exec` request included, 3.10 loaded the matching skill in
31 of 50 runs.

## What this shows, and what it does not

- **The plugin changes what the agent does on these requests.** Fourteen of the 29
  cases moved by 0.15 or more with 3.10. The largest gains were `craft-goal`,
  `claude-exec`, `plan`, `memory` and `skill-builder`.
- **Fifteen cases made no measurable difference.** In eight of them the agent met
  every criterion with no plugin, so the case had no room to show a gain:
  `council`, `navigate`, `postmortem`, `reality-check`, `research`, `review`,
  `test` and `validate`.
- **Most of the 3.9.0 to 3.10 change is loading.** A skill that never loads
  cannot help. `plan` went from 5 of 15 criteria to 15 of 15, and `memory`
  from 2 of 15 to 15 of 15, once they loaded.
- **Loading is still the weak point.** Nine skills did not load on their blind
  request: `doc`, `implement`, `navigate`, `reality-check`, `refactor`,
  `research`, `review`, `security` and `validate`. Seven of them loaded in
  at least two of three runs on the request their description was tuned
  against, which means the descriptions fit those requests better than they fit
  requests in general.
- **`implement` never loads.** A request to fix a bug is ordinary coding, and
  the agent does it without a skill. Its description no longer claims every
  edit.
- **This is not an outcome measurement.** The criteria were written from each
  skill's own rules, by the same audit that proposed the edits, before the
  edits were made. A pass shows that a rule landed on one request. It does not
  show a better result on a real task, on another model, or at what cost. The
  earlier [coding pilot](https://github.com/boshu2/agentops/pull/1125) found no
  end-to-end difference.
- **Three runs per case is a small sample.** One run meeting or missing one
  criterion moves a five-criterion case by 0.07. No significance is claimed.

## Per skill

Criteria met over three runs, pooled. "Loaded" is how many of the three runs
with 3.10 called the Skill tool for that skill.

| Skill | With 3.10 | Without | Delta | Loaded (of 3) | With 3.9.0 |
|---|---:|---:|---:|---:|---:|
| `craft-goal` | 12/12 | 3/12 | +0.75 | by name | 12/12 |
| `claude-exec` | 14/15 | 3/15 | +0.73 | 3 | new |
| `plan` | 15/15 | 5/15 | +0.67 | 3 | 5/15 |
| `memory` | 15/15 | 6/15 | +0.60 | 3 | 2/15 |
| `skill-builder` | 8/12 | 1/12 | +0.58 | 3 | 4/12 |
| `interview` | 12/12 | 5/12 | +0.58 | by name | 12/12 |
| `using-gc` | 12/12 | 6/12 | +0.50 | 3 | 12/12 |
| `idea-genie` | 15/15 | 9/15 | +0.40 | 3 | 4/15 |
| `security` | 13/15 | 7/15 | +0.40 | 3 | 6/15 |
| `orchestrate` | 10/12 | 7/12 | +0.25 | 3 | 9/12 |
| `domain` | 15/15 | 12/15 | +0.20 | 3 | 15/15 |
| `reverse-engineer` | 15/15 | 12/15 | +0.20 | 3 | 13/15 |
| `skill-eval` | 15/15 | 12/15 | +0.20 | 3 | 12/15 |
| `codex-exec` | 11/12 | 9/12 | +0.17 | 3 | 9/12 |
| `agent-native` | 15/15 | 13/15 | +0.13 | 2 | 15/15 |
| `doc` | 10/12 | 9/12 | +0.08 | 3 | 11/12 |
| `premortem` | 12/12 | 11/12 | +0.08 | 3 | 12/12 |
| `refactor` | 12/12 | 11/12 | +0.08 | 2 | 9/12 |
| `agy-native` | 12/15 | 11/15 | +0.07 | 3 | 14/15 |
| `council` | 12/12 | 12/12 | +0.00 | 0 | 11/12 |
| `implement` | 6/12 | 6/12 | +0.00 | 0 | 5/12 |
| `navigate` | 15/15 | 15/15 | +0.00 | 3 | 15/15 |
| `postmortem` | 12/12 | 12/12 | +0.00 | by name | 12/12 |
| `reality-check` | 15/15 | 15/15 | +0.00 | 3 | 15/15 |
| `research` | 12/12 | 12/12 | +0.00 | 2 | 12/12 |
| `review` | 12/12 | 12/12 | +0.00 | 0 | 11/12 |
| `rpi` | 12/15 | 12/15 | +0.00 | by name | 11/15 |
| `test` | 12/12 | 12/12 | +0.00 | 2 | 12/12 |
| `validate` | 12/12 | 12/12 | +0.00 | 3 | 12/12 |

Notes on single rows:

- `agy-native` and `doc` are the two cases below their 3.9.0 score. `doc` is
  one criterion-run lower (10 of 12 against 11 of 12), which is inside the
  noise. `agy-native`'s second criterion expects a refusal to run `claude -p`
  when `agy` is missing. 3.10 removed the retired ban on print mode from the
  skill: it still forbids a silent fallback, and an explicitly requested Claude
  run as a separate step is now allowed. So the criterion fails by design with
  3.10 (0 of 3 runs). The old ban text in 3.9.0 met it in 2 of 3 runs, and that
  is the whole of the case's drop. It is kept unchanged.
- `rpi`'s third criterion expects a fresh reviewer for every change. That
  contradicts the rule both releases carry, one fresh review only where a
  mistake is costly, so it fails by design with 3.10 (0 of 3) and mostly with
  3.9.0 (1 of 3). The agent with no plugin met it in 3 of 3.
- `council` did not load on its behavior request, so its content was not
  exercised there. It loaded on its blind request.
- `craft-goal`, `interview`, `postmortem` and `rpi` are user-only and were
  invoked by slash command. The arm with no plugin saw an unknown command.

## Blind requests

Two runs each. A run passes when the agent called the Skill tool for the
matching skill.

| Held-out request for | Loaded on 3.10 | Loaded on 3.9.0 |
|---|---:|---:|
| `agent-native` | 2/2 | 0/2 |
| `agy-native` | 2/2 | 2/2 |
| `claude-exec` | 2/2 | new |
| `codex-exec` | 2/2 | 0/2 |
| `council` | 2/2 | 2/2 |
| `doc` | 0/2 | 0/2 |
| `domain` | 2/2 | 2/2 |
| `idea-genie` | 2/2 | 0/2 |
| `implement` | 0/2 | 0/2 |
| `memory` | 2/2 | 0/2 |
| `navigate` | 0/2 | 0/2 |
| `orchestrate` | 2/2 | 0/2 |
| `plan` | 2/2 | 0/2 |
| `premortem` | 2/2 | 0/2 |
| `reality-check` | 0/2 | 0/2 |
| `refactor` | 0/2 | 0/2 |
| `research` | 0/2 | 0/2 |
| `reverse-engineer` | 2/2 | 1/2 |
| `review` | 0/2 | 0/2 |
| `security` | 0/2 | 0/2 |
| `skill-builder` | 2/2 | 2/2 |
| `skill-eval` | 2/2 | 0/2 |
| `test` | 1/2 | 0/2 |
| `using-gc` | 2/2 | 2/2 |
| `validate` | 0/2 | 0/2 |

## Method

- **Evaluator:** `claude plugin eval`, Claude Code 2.1.282. Model
  `claude-opus-5-5`. Each run had the Read, Glob, Grep and Skill tools, an empty
  working directory and at most 12 turns.
- **Cases:** [`evals/plugin-eval/`](../../evals/plugin-eval/README.md). The 28
  behavior cases and their criteria were written on 2026-10-04 from an audit of
  the skills, and were not changed afterwards. The audit read a checkout from
  just before 3.9.0, which is why two criteria contradict rules that 3.9.0
  already had. The `claude-exec`
  case was written without sight of the skill's text.
- **Subjects:** 3.10 is this repository at the release commit, loaded as the
  full plugin. 3.9.0 is the `v3.9.0` skills directory loaded as a plugin without
  the bundle's agents and hooks; the hooks act only on Bash, Edit and Write
  calls, which these cases cannot make.
- **No-plugin arm:** generated once, on 2026-10-04, for the 28 older cases, and
  reused. A run with no plugin does not depend on the plugin version. The
  `claude-exec` no-plugin runs are from 2026-10-05.
- **Grading:** `evals/plugin-eval/grade.py`, one Claude Opus 5.5 call per
  response covering all of its criteria. On 101 responses also graded by the
  evaluator's three-vote Opus judge, the two agreed on 439 of 458 criteria
  (95.9%); `grade.py` was the stricter one in 15 of the 19 disagreements. The
  counts are in the scorecard's `grader_agreement` block.
  Loading uses the evaluator's deterministic check on the Skill tool call.
- **Scorecard:** per-criterion counts for every case and arm are in
  [the scorecard](../../evals/plugin-eval/scorecards/2026-10-05-opus-5-5.json).

## Departures from a single clean run

- A session limit interrupted the first 3.10 run, so the 3.10 with-plugin arm
  was assembled from several invocations of the same cases.
- After the first full pass, three descriptions changed. `implement` was
  narrowed on an independent reader's advice. `claude-exec` and `council` were
  reworded because neither loaded on its behavior request; `claude-exec` then
  loaded in every run and `council` still did not. All cases for those three
  skills were rerun.
- An independent read then found small defects in five skills (`plan`, `doc`,
  `skill-eval`, `security` and `claude-exec`). They were fixed and those five
  behavior cases were rerun, so every 3.10 number here is from the released
  text. Their blind requests were not rerun, because no description changed.
- The 3.9.0 arm was first graded by the evaluator's own Opus judge and then
  regraded with `grade.py`, so that every arm uses one grader.

## Reproduce

```bash
OUT=$(mktemp -d)   # run output holds every prompt and response; keep it out of the repository
claude plugin eval . --eval-dir evals/plugin-eval/behavior \
  --model claude-opus-5-5 --no-publish --json "$OUT/behavior.json"
python3 evals/plugin-eval/grade.py "$OUT/behavior.json" --out "$OUT/grades.json"

claude plugin eval . --eval-dir evals/plugin-eval/routing \
  --ablation none --model claude-opus-5-5 --runs 2 --no-publish --json "$OUT/routing.json"
```
