# Description trial for six skills (2026-10-31)

Two of six description lines ship: Triage's candidate D2 loaded on 13 of 13 runs against the
control's 4 and raised the behavior score from 13/30 to 20/30; Implement's candidate D1 loaded on
all seven blind requests where the control loaded on none. Architecture, Council and Security keep
their current lines: no candidate was better on at least one request without being worse on
another. Domain's control already loaded on every run, so its one candidate could only equal it,
which it did on all 13 runs and all 30 criteria; it is reported as equal at ceiling and not landed.
All eleven candidates passed the 180-character shipping budget and the 13
routing goldens before any paid run, closing the preflight gap the Wave 3 report named.

The six lines were the description defects the 2026-10-25 re-score confirmed and wave 2.5
([PR #1254](https://github.com/boshu2/agentops/pull/1254)) declined on the frozen
no-description-change decision. The [Wave 3 report](2026-10-26-wave3-opus-5-5.md) had found that
every variant it tried failed the budget as serialized, so none could land.

## Subject and method

The subject is `plugin/` at commit `73cbeae8be281b970389d8b2ea5636cfa972a9fa`, Git tree
`455faa6cfeb22caa3f95b3c3d1816191f25bafc4`, after wave 2.5. All 243 package files were hashed before
launch and re-hashed after the last run. Copies were kept outside Git. The control C is unchanged;
each D subject is C with exactly one skill's `description:` replaced, so a candidate can neither help
nor steal from another. The runner was Claude Code 2.1.309 with `claude-opus-5-5`, publishing
disabled and raw output kept in protected storage outside Git. Every run had a fresh sandbox memory
path, and both arms carried the same two native built-ins, `cc-plugin-agents-md` and
`cc-plugin-plugin-authoring`.

The [registration](../../evals/plugin-eval/plans/2026-10-31-description-trial.md) fixed the
candidates, requests, harness, decision rule and stop conditions before any subject was built; the
tracker record is bead age-pv2j.103. No skill body, case criterion or bar was changed during the trial.

Requests and run counts are those of the Wave 3 loading experiment: the skill's behavior case six
times (natural), its routing case three times (r1) and the two October blind requests twice each
(r2, r3), 13 runs per arm. The arms of one skill ran in the same time window, and every subject passed
the evaluator's parse check (`--case 'zz-none-*'`) before its first counted run. Loading is an
observed Skill tool call for the target in the run trace, never response text; the evaluator's own
deterministic `skill-loaded` grader agreed on every spot-checked run. Behavior on the natural runs is
graded by `evals/plugin-eval/grade.py`, one Opus call per response, `skill-loaded` excluded. The
grader and its 34-answer calibration are unchanged from Wave 3; no new calibration answers were
written for this trial.

**Decision rule, fixed in advance.** Keep a candidate only if, against the same-day control, its
loads are no lower on every one of the four requests and higher on at least one, and its natural
behavior score is within 0.15 of the control's. Two passing candidates for one skill: the one with
more pooled loads wins, and a tie goes to the one closer to the current wording.

## Candidates

Every line passed `tests/skills/test-token-budgets.sh` (gate-counted characters in brackets) and
`scripts/check-routing-probe-goldens.sh` (13/13) applied to the worktree one at a time and restored,
before any subject was built. Domain had one candidate: its current line already loaded on every
measured run, so a second near-duplicate added no information.

| Skill | Candidate | Line |
|---|---|---|
| architecture | D1 [178] | Compare interface designs and find structural friction before any code moves. Use when: a change spreads across modules or a boundary is unclear. Moving code is Refactor or Plan. |
| architecture | D2 [178] | Find where structure makes change costly and compare options for a new boundary. Use when: a change spreads across modules or a boundary is unclear. Code moves: Refactor or Plan. |
| council | D1 [176] | Compare independent opinions from several models or contexts without inflating agreement. Use when: wanting several separate takes, a debate, or synthesizing collected reviews. |
| council | D2 [178] | Compare independent opinions without inflating agreement. Use when: asked for independent takes from several models, a debate or vote, or to synthesize reviews already collected. |
| domain | D1 [178] | Settle what domain terms mean per context, or which repository conventions and code, data, risk or test standards apply. Use when: names disagree or a rename or rule is at issue. |
| implement | D1 [170] | Make the code change a ticket or bug report asks for, then prove it with the repo's own checks. Use when: told to fix, change or build something, such as "fix this bug". |
| implement | D2 [173] | Fix red CI or build what a ticket calls for without loosening the test; prove it with the repo's checks. Use when: asked for a patch or a fix, or whether to relax an assert. |
| security | D1 [175] | Review code, config, CI workflows and dependencies for security problems and secrets. Use when: asked if code is safe to ship, or if anything is exploitable, nasty or leaking. |
| security | D2 [180] | Find what an attacker could exploit in code, config, CI workflows or dependencies and report it. Use when: asked if code is safe to ship, what is exploitable or if a secret leaked. |
| triage | D1 [178] | Decide what happens to something that just arrived: take it, park it, close as duplicate, ask for more or hand it off. Use when: a new issue, alert or outside PR needs that call. |
| triage | D2 [173] | Make the call on a new bug report, issue, outside PR or alert: take it, ask for more, or close as duplicate or already fixed, each with evidence. Use when: one lands on you. |

The author of these lines had read every request, so they are not unseen holdouts. No candidate
shares three consecutive words with any of its skill's requests except security D1 ("for security
problems", already in the shipped line) and triage D1 ("what happens to", also in the Wave 3
variant). The Wave 3 implement variant quoted the routing request's own closing words; implement D1
quotes the skill's declared trigger "fix this bug" instead.

## Loading and behavior

Loads per request are out of 6, 3, 2 and 2 runs. Pooled is out of 13. Behavior is criteria met on the
six natural runs, pooled, by the independent grader. The difference column is the candidate's pooled
loading minus the control's, with a Newcombe 95% interval and a two-sided Fisher p; runs within an arm
share a subject, so these are directional. Wilson intervals for each arm are in the companion scorecard.

| Skill | Arm | natural | r1 | r2 | r3 | Pooled | Behavior | Difference [95%]; Fisher p | Rule | Disposition |
|---|---|---:|---:|---:|---:|---:|---:|---|---|---|
| architecture | C | 0 | 3 | 2 | 2 | 7/13 | 9/30 | | | |
| architecture | D1 | 0 | 3 | 2 | 2 | 7/13 | 11/30 | +0.00 [-0.34, +0.34]; p=1 | equal on every request | null |
| architecture | D2 | 0 | 3 | 2 | 2 | 7/13 | 10/30 | +0.00 [-0.34, +0.34]; p=1 | equal on every request | null |
| council | C | 1 | 3 | 2 | 2 | 8/13 | 22/24 | | | |
| council | D1 | 0 | 3 | 2 | 2 | 7/13 | 24/24 | -0.08 [-0.40, +0.27]; p=1 | worse on natural | null |
| council | D2 | 0 | 3 | 2 | 2 | 7/13 | 24/24 | -0.08 [-0.40, +0.27]; p=1 | worse on natural | null |
| domain | C | 6 | 3 | 2 | 2 | 13/13 | 30/30 | | | |
| domain | D1 | 6 | 3 | 2 | 2 | 13/13 | 30/30 | +0.00 [-0.23, +0.23]; p=1 | equal; control at ceiling | equal at ceiling |
| implement | C | 0 | 0 | 0 | 0 | 0/13 | 16/24 | | | |
| implement | D1 | 0 | 3 | 2 | 2 | 7/13 | 13/24 | +0.54 [+0.20, +0.77]; p=0.0052 | passes; tie-break on wording | **kept** |
| implement | D2 | 6 | 0 | 0 | 1 | 7/13 | 19/24 | +0.54 [+0.20, +0.77]; p=0.0052 | passes; loses tie-break | not landed |
| security | C | 6 | 0 | 2 | 2 | 10/13 | 35/36 | | | |
| security | D1 | 6 | 3 | 1 | 2 | 12/13 | 34/36 | +0.15 [-0.14, +0.43]; p=0.59 | worse on r2 (1/2 against 2/2) | null |
| security | D2 | 4 | 2 | 0 | 2 | 8/13 | 30/36 | -0.15 [-0.45, +0.19]; p=0.67 | worse on natural and r2 | null |
| triage | C | 0 | 3 | 0 | 1 | 4/13 | 13/30 | | | |
| triage | D1 | 2 | 3 | 2 | 2 | 9/13 | 13/30 | +0.38 [+0.00, +0.64]; p=0.12 | passes; fewer pooled loads | not landed |
| triage | D2 | 6 | 3 | 2 | 2 | 13/13 | 20/30 | +0.69 [+0.34, +0.87]; p=0.0005 | passes; more pooled loads | **kept** |

**Triage.** The control reproduced Wave 3's control exactly (0, 3, 0, 1). D2 loaded on every run and
met criteria c2 to c4 (duplicate searches disclosed, no fix without a reproduction, a named missing
fact) 3, 3 and 2 times out of 6 where the control met them 0, 0 and 1 times. D1, the Wave 3 variant
brought under budget, loaded on 9 of 13 and left behavior unchanged.

**Implement.** Both candidates passed and tied at 7 of 13 pooled loads with opposite profiles: D1
loaded on all seven blind requests and never on the natural case; D2 loaded on all six natural runs
and on one blind request. The registered tie-break is closeness to the current wording. The measure,
chosen after the tie was seen, is shared words with the shipped line: D1 shares `change`, `code` and
`report`, D2 shares `without` and `what`, so D1 lands. The natural case loaded Implement in neither
the control nor D1, so their 16/24 against 13/24 is the model's own variation, inside the noise bar.
D2's profile is a hypothesis for a later line, not a result of this one.

**Security.** Today's control took both blind pairs (r2, r3) where Wave 3's control took 1 of 7, which
is why the comparison uses a same-day control. D1 took the routing request 3 of 3 against 0 of 3 (the
request the defect named) and loaded on 12 of 13 overall, but on r2 it loaded on 1 of 2 runs against
the control's 2 of 2, which fails the no-worse condition as written. A two-run request decides this
by a single run; a larger rerun of that request would be a new trial. D2 lost natural loading, as the
Wave 3 attacker-framed variant did.

**Architecture and Council.** Every arm routed the blind requests 7 of 7 and the natural case 0 of 6
(Council's control once). Neither candidate changed loading, so the rule returns null for both.
Their recorded defects are correctness defects in the hand-off sentence (Architecture) and the
second-opinion trigger (Council); a correctness-only change is outside what this rule can admit and
is left to the owner.

**Domain.** The control loaded on all 13 runs and met all 30 criteria, as the registration
anticipated, and the candidate matched it exactly. The rule's second condition cannot hold at
ceiling, so the candidate is reported as equal at ceiling and not landed. Its recorded defect is a
correctness defect: the line names two languages where the body covers six languages, three
formats, three risk checklists and the test pyramid. The measurement shows the corrected line costs
nothing in routing or behavior on these requests; landing a correctness-only change is the owner's
decision.

## Steal checks

One run each of two neighbours' routing requests on every candidate arm, 22 runs. No neighbour request
loaded the candidate's skill. Plan's request loaded Plan on 6 of the 7 arms where it ran and nothing on
the other; Premortem's loaded Premortem on 2 of 2; Reverse Engineer's loaded Reverse Engineer on 2 of
2; Refactor's loaded Refactor on 1 of 4; Review's (4 arms), Navigate's (2) and Research's (1) loaded
nothing. These are single runs, reported as observed, not a bar.

## Shipping gate on the kept lines

The two kept lines were written into `skills/triage/SKILL.md` and `skills/implement/SKILL.md`,
projections were regenerated with `scripts/regen-all.sh` (catalog, router, plugin folder), and
`scripts/regen-all.sh --check`, `tests/skills/test-token-budgets.sh` (173 and 170 gate-counted
characters) and `scripts/check-routing-probe-goldens.sh` (13/13) passed on the resulting tree. No
gate was relaxed and no unmeasured wording was substituted.

## Accounting and verification

Generation ran from 17:14 to 17:38 UTC on 2026-10-31. All 243 run attempts have native JSON
receipts and copied traces in protected storage outside Git, and all 243 sandbox identities were unique. There were no generation errors,
replacements, partial results or usage-limit stops. Every run reported `claude-opus-5-5` and CLI
2.1.309, every run's memory path was isolated, and all original package hashes were unchanged after
the last run.

| Skill | Run attempts | Evaluator-reported cost |
|---|---:|---:|
| triage | 43 | $5.76 |
| implement | 43 | $5.28 |
| security | 43 | $6.83 |
| architecture | 43 | $6.71 |
| council | 43 | $13.32 |
| domain | 28 | $3.61 |
| **Total** | **243** | **$41.52** |

Council's responses are long, so each of its arms cost about twice the others. These are the
evaluator's reported costs, below the registered $45 stop, not a verified invoice. The separate
grader made 102 response calls, about $0.02 each by the registration's estimate, which the evaluator
does not meter. Before the domain arms the launcher's own worst-case guard (spend plus every remaining
arm at the $4 per-arm cap) would have crossed the stop, so the per-arm cap was lowered to $3.50 before
that launch; measured arms cost $1.35 to $4.36.

Local repository checks on the landed tree: `scripts/regen-all.sh --check`, token budgets, 13/13
routing goldens, `ao gate check --full --scope worktree` and `tests/run-all.sh`; results are in the
pull request.

The [machine scorecard](../../evals/plugin-eval/scorecards/2026-10-31-description-trial-opus-5-5.json)
holds the per-request loads and Skill calls, Wilson intervals, individual boolean grades, the
candidate pre-check results and the per-arm records. Raw prompts, responses, session identities,
local paths and private tracker records are excluded.

## Scope and limits

This measures loading on one model and one runtime; Codex, Gemini and Cursor were not run. Six
natural and seven blind runs per arm detect only large changes, and the two-run blind requests make
the no-worse condition sensitive to a single run, as Security shows. A zero-crossing interval does not
establish equivalence. The candidates were written by someone who had read the requests; the steal
check and the behavior case only partly cover a line tuned to the requests' style. Behavior scores
are rule adherence on one request each, not outcomes on real tasks. The control's blind loading moved
between Wave 3 and today for Security while staying identical for Triage and Implement, so numbers
from different days are not pooled here.
