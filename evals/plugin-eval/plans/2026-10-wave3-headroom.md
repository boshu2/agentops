# Wave 3 headroom bars: the new behavior cases

Execution status (2026-10-26): [results and limitations](../../../docs/evals/2026-10-26-wave3-opus-5-5.md).
The registration below is preserved as written; its "not run" statements refer to registration time.

Pre-registered 2026-10-25 for epic age-pv2j (bead age-pv2j.39), before any of these cases was
run. Nothing here has been run. The bars and the fold-or-cut conditions below are fixed; a run
lane may not change a bar or a criterion after its first counted run.

## Question

The 2026-10-25 skill deep dive found that about half the skills' cases test what Claude Opus
5.5 already does without the skill, so four skills (validate, research, test and the folded
claim audit) showed no measured value for want of headroom, and five fold-or-cut calls were
left conditional on cases nobody had run. These twelve cases target the rules that make each
skill distinct. For each case: does the agent miss the rule without the plugin, and does the
skill, once loaded, close the gap?

## Cases

All under `evals/plugin-eval/behavior/`. Each case file names, in its header comment, the rule
each criterion tests.

| Case | Skill | Kind | Criteria | Decision it informs |
|---|---|---|---:|---|
| `council-vote-tally` | council | pasted | 4 | value of Council's majority rule |
| `research-absence-proof` | research | seeded repository | 4 | research cut to the claim-audit ledger |
| `review-merge-base` | review | seeded repository | 5 | review cut |
| `test-isolated-revert` | test | seeded repository | 4 | test folded into implement |
| `validate-mutating-check` | validate | seeded repository | 4 | value of Validate's mutating-check rule |
| `agent-native-harder` | agent-native | pasted, harder | 5 | agent-native folded into orchestrate |
| `agy-native-harder` | agy-native | pasted, harder | 5 | agy-native folded into codex-exec |
| `codex-exec-harder` | codex-exec | pasted, harder | 4 | value of the one-deadline rule |
| `postmortem-harder` | postmortem | pasted, harder | 5 | value of the cause test |
| `premortem-harder` | premortem | pasted, harder | 4 | value of the traced one-way door |
| `research-claim-check-harder` | research | pasted, harder | 4 | value of the claim-audit ledger that a research cut would keep |
| `navigate-harder` | navigate | pasted, harder | 5 | re-measure of round 4's +0.40 (copied unchanged; its case name is `navigate-hard`) |

Also in this wave, re-measured in the next full-suite run and not part of these bars:
`orchestrate` gained c5 (integrate one at a time in dependency order, checks after each) and
`security` gained c6 (the f-string `DELETE` as SQL injection).

The seeded cases carry `context.scaffold_script: seed.sh`, which builds a small Git repository
in the run's workspace; they need `--scaffold --allow-tools Edit Write Bash`. Every case
except `navigate-harder` is written without a slash command, as the README requires.

## Arms and runs

Nine runs per case, three per arm, the arms of one case run concurrently in the same window:

| Arm | Plugin | Prompt | How |
|---|---|---|---|
| forced | yes | the case's prompt prefixed with `/agentops:<skill> ` | a copy of the case in the run directory, `--ablation none` |
| without | no | the case as written | the no-plugin arm of `--ablation with-without` on the case as written |
| natural | yes | the case as written | the with-plugin arm of that same run |

`navigate-harder` already starts with `/agentops:navigate`, so for it forced and without both
use the case as written (its with-plugin and no-plugin arms, as round 4 measured it), and
natural uses a copy with the prefix removed.

Harness: `claude plugin eval`, model `claude-opus-5-5`, `--runs 3`, `--no-publish`,
`--trust-plugin`, `--threshold 0`, JSON written outside the repository; subject is the
repository's `plugin/` tree at the commit the run starts from (record the SHA). Grading:
`evals/plugin-eval/grade.py`, one Opus call per response. Score: criteria met over criteria
graded, pooled over the arm's three runs, `skill-loaded` excluded. Loading: the deterministic
`skill-loaded` grader on the natural arm (a forced run injects the skill and cannot register a
Skill tool call).

## Bars, fixed in advance

1. **Informative.** A case can show anything only if the without arm scores **at or below
   0.70**. Above 0.70 the case has no headroom and says nothing about the skill's value either
   way. With these denominators (12 or 15 criterion-runs per arm) 0.70 itself cannot occur: a
   4-criterion case is informative at 8 of 12 (0.67) or fewer, a 5-criterion case at 10 of 15
   (0.67) or fewer.
2. **Value.** On an informative case, the skill shows value only if **forced minus without
   exceeds 0.15**. Report the difference with its Newcombe 95% interval and the Fisher exact
   p on pooled criterion-runs; at three runs per arm the interval is wide, and the deep dive's
   "strong" tier (interval excluding zero) is not settled by this plan.
3. **Per criterion.** A criterion missed in 2 or 3 of the 3 forced runs is a revise candidate;
   a skill-text fix for it needs its own pre-set rerun bar (round 4's: target criterion passes
   2 of 3 from 1 of 3 or fewer, other criteria lose at most one run in total).
4. **Natural loading** is reported per case (loads of 3) and decides nothing here; loading
   for implement, triage, security and review is decided in
   [the loading plan](2026-10-wave3-loading.md). Postmortem loads by name, and no request here
   names it, so its natural loads are expected near zero.

## Fold-or-cut conditions

Each condition is the deep dive's, mapped onto the bars above. "Shows value" means
informative and forced minus without above 0.15.

| Decision | Case | Condition met (the fold or cut is supported) | Otherwise |
|---|---|---|---|
| agent-native into orchestrate | `agent-native-harder` | The case does not show value (no headroom, or informative with a difference of 0.15 or less). This settles only the evidence leg: the fold still needs the deep dive's other conditions, a trimmed body under about 60 lines and a home for `model-dispatch.md` and the Codex agent TOMLs. | Shows value: keep agent-native separate; its scope and whole-batch rules earned their place. |
| agy-native into codex-exec | `agy-native-harder` | No headroom: the without arm scores above 0.70, so even the AGY-specific facts are already known. | Shows value: keep. Informative but a difference of 0.15 or less: the facts did not land when loaded; report it as a revise for the text that carries them, not a fold, and leave the call to the owner. |
| research cut to the claim-audit ledger | `research-absence-proof` | The case does not show value: the without arm scores 9 of 12 (0.75) or more (the deep dive's "0.70 or more"), or it is informative with a difference of 0.15 or less. | Shows value: keep Research's investigation body. In either branch, report `research-claim-check-harder` beside it: if the ledger case also fails to show value, the part a cut would keep has no measured value on this model either. |
| review cut | `review-merge-base` | The case misses the bar: it does not show value. The cut cannot land before `skills/review/references/advice-or-acceptance.md` moves to a home Validate and Research can still link. | Shows value: keep Review. |
| test into implement | `test-isolated-revert` | No headroom: the without arm scores above 0.70. | Shows value: keep. Informative but a difference of 0.15 or less: not the deep dive's condition; report no measured value and leave the call to the owner. In every branch, note that Implement loads 0 of 37 natural runs, so a fold delivers nothing more until the loading plan shows Implement can load. |

The other seven cases carry no fold or cut; each reports whether its skill shows value on its
distinctive rule (validate, council, codex-exec, postmortem, premortem, the claim audit) or
whether round 4's navigate gain reproduces.

## Before the first counted run

- **Parse check.** `claude plugin eval . --eval-dir <copied case dir> --case 'zz-none-*' --no-publish`
  loads every copied case with no load failure, for both the natural and the forced copies.
- **Grader calibration.** For each case, grade one hand-written answer that plainly meets every
  criterion and one that plainly meets none. A criterion the passing answer fails, or the
  failing answer meets, is reworded now; this is the only point at which a criterion may change.
- **Seeded smoke.** One natural run of each seeded case, not counted. In October's sandbox the
  Xcode `git` shim failed and agents found Homebrew's `git` themselves; these cases also need
  `python3`. If the smoke run shows the agent cannot run `git` or `python3` at all, stop the
  seeded cases and report the environment, rather than score runs the environment decided.

## Stop

- Evaluator spend reaches **$20** (envelope $25).
- Any case fails the parse check, or errors on every run of an arm.
- A run that errors is reported as an error, and a replacement run is added and labelled as
  one, never written over the failed attempt. `grade.py` exits nonzero on any ungraded run;
  no score is computed over fewer runs unnoticed.

## Spend estimate

At the October per-run cost ($0.14 with the plugin, $0.07 without):

| Item | Runs | Cost |
|---|---:|---:|
| 12 cases x (3 forced + 3 natural) with the plugin | 72 | $10.08 |
| 12 cases x 3 without the plugin | 36 | $2.52 |
| Seeded smoke, 4 runs with the plugin, not counted | 4 | $0.56 |
| Grading, 108 responses with `grade.py` at about $0.02, plus 24 calibration answers | 132 | $2.64 |
| **Total at the October rate** | **112 runs** | **about $15.80** |
| Allowance: the 36 seeded runs at up to 1.5x (16 turns and tool output instead of 12 pasted-only turns) | | +$2.10 |
| **Upper estimate** | | **about $17.90** |

## Limits known in advance

- Three runs per arm show only large effects; a difference just above 0.15 is one or two
  criterion-runs.
- The criteria come from each skill's own rules and the cases were written by an agent that
  had read the skills. A pass shows a rule landed on one request, not a better outcome on a
  real task, another model or another runtime.
- The judge sees only the final response, so the seeded cases grade what the agent reports
  doing (which copy it ran a check in, whether it deleted a function), not the workspace.
- One model and one runtime.
