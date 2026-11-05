# Description trial: architecture, council, domain, implement, security, triage

Execution status (2026-10-31): [results and limitations](../../../docs/evals/2026-10-31-description-trial-opus-5-5.md).
The registration below is preserved as written.

Pre-registered 2026-10-31 for epic age-pv2j (bead age-pv2j.103), after wave 2.5 landed in
[PR #1254](https://github.com/boshu2/agentops/pull/1254) and before any subject was built or any
run started. The bars below are fixed; a run lane may not change them after its first counted run.

## Question

The 2026-10-25 re-score left one confirmed defect in each of six `description:` lines, and wave
2.5 declined all six on the frozen no-description-change decision. The
[Wave 3 report](../../../docs/evals/2026-10-26-wave3-opus-5-5.md) found that every description
variant it tried failed the 180-character shipping budget as serialized, so none could land.
For each skill this trial asks: does a candidate line that answers the recorded defect and
passes the budget before any paid run route at least as well as the shipped line on every
request, and better on at least one?

| Skill | Recorded defect |
|---|---|
| architecture | "For code changes, use Refactor" contradicts the body: one behavior-preserving transformation goes to Refactor, anything larger or behavior-changing to Plan. |
| council | "wanting a second opinion" contradicts the body: one opinion or look-over is Review. |
| domain | Names "language standards (Go, Python)" while the body links six languages, three formats, three risk checklists and the test pyramid; Use-when covers only naming disputes. |
| implement | Loaded 0/25 natural and 0/7 blind on the shipped line. |
| security | Use-when names only the ship phrasing; blind 0/7, the routing request 0/3. |
| triage | "Use when: intake arrives" is not user phrasing; natural 0/6 on the shipped line. |

## Subjects

Copies of the repository's `plugin/` tree at the commit the run starts from, kept outside Git
as in October and wave 3. C is the unchanged control. Each D subject is C with exactly one
skill's `description:` replaced, so a candidate can neither help nor steal from another.
At most two candidates per skill; domain gets one because its shipped line already loads on
every measured run, so a second near-duplicate adds no information.

## Candidates

Every candidate passed `tests/skills/test-token-budgets.sh` (gate-counted characters in
brackets) and `scripts/check-routing-probe-goldens.sh` (13/13) with a tree-built `ao`, applied
one at a time to the worktree and restored, before any subject was built. A candidate over
budget is discarded, not trimmed into meaninglessness.

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

The author of these lines has read every request below, so they are not unseen holdouts. No
candidate shares three consecutive words with any of its skill's four requests except security
D1 ("for security problems", already in the shipped line) and triage D1 ("what happens to", also
in the wave-3 variant). The wave-3 implement variant quoted the routing request's own closing
words; implement D1 quotes the skill's declared trigger "fix this bug" instead.

## Requests

Per skill and per arm, 13 runs, the same set and weights as wave 3's loading experiment:

- **Natural, 6 runs:** the skill's behavior case, `evals/plugin-eval/behavior/<skill>/case.yaml`, as written.
- **Blind, 7 runs:** the routing case `evals/plugin-eval/routing/<skill>/case.yaml` three times, and
  the two October blind requests (`<skill>-r2`, `<skill>-r3`, written by an agent that had not seen
  the descriptions) twice each, copied unchanged from the October run directory.

Steal check, D arms only, reported and not part of the bar: one run each of two neighbours'
routing requests (architecture: plan, refactor; council: review, premortem; domain: research, plan;
implement: plan, refactor; security: review, reverse-engineer; triage: plan, navigate). A
neighbour request that loads the candidate's skill instead of its own is reported to the owner
before any shipping change.

## Harness

`claude plugin eval` as installed (version recorded per run), model `claude-opus-5-5`,
`--ablation none`, `--no-publish`, `--trust-plugin`, `--threshold 0`, `--keep-temp`,
`--concurrency 2`, `--max-cost-usd` per arm, JSON written outside the repository. The arms of
one skill run in the same time window. Before any counted run, the parse check
(`--case 'zz-none-*'`) must load every copied case of every subject with no load failure.
Loading is read from the run trace (an observed Skill tool call for the target), never from the
response text. Behavior on the natural runs is graded by `evals/plugin-eval/grade.py`, one Opus
call per response, with the deterministic `skill-loaded` grader excluded.

## Decision rule, fixed in advance

Keep a candidate only if, against the same-day C arm, all three hold:

1. its loads are no lower than C's on every one of the four requests (natural, r1, r2, r3);
2. its loads are higher than C's on at least one of them; and
3. its natural behavior score is not lower than C's by more than 0.15, the suite's noise bar.

If two candidates for one skill pass, keep the one with more pooled loads; a tie goes to the
one closer to the current wording. Otherwise the current line stays and the null is recorded
with its numbers. If C loads on all 13 runs, condition 2 cannot hold; a candidate equal on all
four requests and within the behavior bar is reported as "equal at ceiling", not landed, and the
correctness-only change is the owner's decision. A kept line still passes the repository's
shipping checks before it merges (`scripts/regen-all.sh --check`, `tests/skills/test-token-budgets.sh`,
`scripts/check-routing-probe-goldens.sh`).

## Stop

- A 429 or weekly usage limit: stop, record what ran, report. Never switch accounts.
- Evaluator-reported spend reaches $45 (envelope $50); per-arm cap $4, steal cap $1.50.
- A subject fails the parse check, or a request errors on every run of an arm: that skill stops.
- A run that errors is reported as an error; a replacement is labelled, never written over it.

## Spend estimate

From the wave-3 accounting (per 13-run arm $1.26 to $1.92, mean about $1.55; steal runs about
$0.15 each):

| Item | Runs | Cost |
|---|---:|---:|
| 17 arms (C for six skills, 11 candidates) x 13 runs | 221 | about $26 |
| Steal check, 11 D arms x 2 neighbour requests x 1 run | 22 | about $3.30 |
| Grading the natural runs with `grade.py`, 17 arms x 6 responses at about $0.02 | 102 judge calls | about $2, not metered |
| **Total** | **243 runs** | **about $31** |

Each run is a Claude Code session of up to 12 turns, so roughly 500 to 1,500 Opus model calls,
plus the evaluator's own Haiku judge calls on the natural cases (three per criterion, about 1,500
small calls, inside the per-arm cost). Account weekly usage was 22% before launch.

## Limits known in advance

- One model and one runtime; Codex, Gemini and Cursor loading are not measured.
- Six natural and seven blind runs per arm show only large loading changes; a zero-crossing
  interval does not establish equivalence.
- The candidates were written by someone who had read the requests; the steal check and the
  behavior case only partly cover a line tuned to the requests' style.
