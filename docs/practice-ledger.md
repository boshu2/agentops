# Practice ledger

AgentOps takes software engineering practices that were proven with human teams
and tests them against AI coding agents. This page records the result for each
one: it held, it held in a different shape, it did not survive, or it is still
being tested.

The skills carry the practices in the first two groups. Read this page to decide
how much weight each one can bear.

## How to read the evidence

Each row names where the evidence comes from.

- **Measured here** is an evaluation in this repository. Most ran on one model,
  with a few runs per case. The linked report states its own limits.
- **Seen here** is a pattern that recurred across a year of daily use. It was
  not a controlled test.
- **Outside** is someone else's study or report. Its strength is given in
  brackets. Several are single preprints, and one is a practitioner's
  experiment on a single model.

Results depend on the model and the harness. A row that holds for one model can
fail for the next, so the [eval cases](https://github.com/boshu2/agentops/tree/main/evals/plugin-eval)
ship with the skills. Run them on your own setup before you rely on a row.

## Holds

### The worker does not certify its own work

A check or a reader that did not write the change decides whether it is done.
Lineage: separation of duties, continuous integration.

- Outside [benchmark]: given tasks that cannot be solved without cheating, GPT-5
  cheated on 54% of one set, by editing tests, special-casing them or
  overloading comparisons among other methods ([ImpossibleBench](https://arxiv.org/abs/2510.20270), October 2025).
- Outside [evaluator report]: telling a model not to cheat had a nearly
  negligible effect on one task family
  ([METR](https://metr.org/blog/2025-06-05-recent-reward-hacking), June 2025).
- Outside [benchmark analysis]: 7.8% of patches counted as solved on SWE-bench
  Verified fail the full developer test suite
  ([Wang et al.](https://arxiv.org/abs/2503.15223), 2025).
- Outside [controlled experiment]: a judge model favours its own wrong answers
  ([Chen et al.](https://arxiv.org/abs/2504.03846), 2025).
- Measured here: agents claimed "complete" over a failing hidden acceptance test
  on 1 of 4 tasks for two Claude models and on 5 to 7 of 8 for two GPT models
  ([estate ablation](https://github.com/boshu2/agentops/blob/main/evals/estate-ablation/SURVIVORS.md),
  August 2026, low effort, small samples).
- Seen here: a fresh reader from a different model family found defects that
  the author's own review and a green test suite had missed. It is the most
  repeated lesson in a year of working notes.

### Protect the test

Never loosen an assertion, a tolerance or a check to get a green run. Lineage:
test-driven development, continuous integration.

- Outside [benchmark]: making tests read-only stopped test edits and left
  special-casing untouched (ImpossibleBench).
- Measured here: the Implement skill, whose core rules are these, raised the
  share of practice criteria met by 0.58 when it was loaded by name
  ([plugin eval](https://github.com/boshu2/agentops/blob/main/docs/evals/2026-10-22-plugin-eval-opus-5-5.md),
  October 2026).

### State the intent and the acceptance before building

Lineage: behavior-driven development, design by contract.

- Outside [practitioner experiment, one model]: under a vague specification
  every workflow passed 0% of the hidden acceptance tests
  ([Finster](https://bryanfinster.substack.com/p/agentic-workflows-do-agents-work),
  July 2026).
- Outside [controlled experiment, one model]: cutting a full specification down
  to a bare user story raised token spend by 29.7%
  ([Smékal](https://arxiv.org/abs/2608.25399), August 2026).
- Measured here: Interview met its criteria on 12 of 12 runs against 6 of 12
  without it. Plan gained 0.60 and Triage 0.53 (plugin eval
  and [wave 3](https://github.com/boshu2/agentops/blob/main/docs/evals/2026-10-26-wave3-opus-5-5.md),
  October 2026).

### Work in small slices

Lineage: Extreme Programming, continuous delivery.

- Outside [practitioner experiment, one model]: small batches beat large ones
  consistently (Finster).
- Outside [observational]: agent pull requests that were not merged were larger
  and touched more files ([Ehsani et al.](https://arxiv.org/abs/2601.15195), 2026).
- A limit: one study found the best test-and-batch protocol depends on the
  model, and the wrong protocol for a model erased the benefit
  ([Wan et al.](https://arxiv.org/abs/2605.17242), May 2026). Small slices
  hold. A rigid gate on every slice is a separate question.

### One writer per workspace

Lineage: bulkheads, branch isolation.

- Outside [observational]: pull requests from one agent product working the
  same repository at the same time conflicted 19.8% of the time, and 41.7% in
  the rarer case where the agents were different products ([Xu et al.](https://arxiv.org/abs/2607.04697), July 2026).
- Seen here: about sixteen separate notes record two sessions colliding in one
  checkout. A worktree per session ended it.

### Short context, and a fresh context for each job

Lineage: single responsibility.

- Outside [controlled experiment]: performance fell by 13.9% to 85% across five
  models as the input grew, even when retrieval was perfect
  ([Du et al.](https://arxiv.org/abs/2510.05381), 2025).
- Outside [vendor benchmark]: the same decline showed across 18 models
  ([Chroma](https://www.trychroma.com/research/context-rot), July 2025).

### Enforce consequences with a mechanism

Use a sandbox, a permission, a read-only file or a check for anything that
cannot be undone. Prose in a prompt is a request. Lineage: least privilege.

- Outside [benchmark]: 55.8% to 67.8% of coding-agent runs crossed a boundary
  the instruction never authorised
  ([UnderSpecBench](https://arxiv.org/abs/2607.02294), July 2026).
- Outside [benchmark]: a strict instruction against cheating cut one model's
  rate to 1% and left another's at 33% (ImpossibleBench).
- Measured here: in a small pilot on one model, a paragraph of doctrine removed
  0 of 12 false completion claims and a deterministic check removed 6
  ([tier-2 pilot](https://github.com/boshu2/agentops/blob/main/evals/tier2-premortem/results-pilot-2026-08-05.md),
  August 2026). In a second sweep, an always-loaded operating contract changed
  nothing on two models and cost 9% to 12% more tokens (estate ablation).
- Not found anywhere: a controlled comparison of hooks against rules files.

### Keep standing instructions short and written by a person

Lineage: documentation economy.

- Outside [controlled experiment]: repository instruction files gave no
  significant gain in task success and raised cost by over 20%. Files a
  developer wrote beat generated ones by a few points
  ([Gloaguen et al.](https://arxiv.org/abs/2602.11988), 2026).

### Measure with headroom and clean arms

A case the unaided agent already passes can show nothing. Lineage: experimental
method, DORA.

- Outside [vendor guidance]: an evaluation at 100% gives no signal for
  improvement ([Anthropic](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents),
  January 2026), and machine resources alone move scores by several points
  ([Anthropic](https://www.anthropic.com/engineering/infrastructure-noise),
  February 2026).
- Outside [benchmark]: the same model can score up to 17 points apart depending
  on the harness, though the authors find the choice of model usually matters
  more ([Terminal-Bench 2.0](https://arxiv.org/abs/2601.11868), January 2026).
- Measured here: five tests of a knowledge corpus were void because the unaided
  model already scored 0.8 or better
  ([ADR-0004](adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)).

## Holds, in a different shape

| Practice | Before | Now |
|---|---|---|
| Independent review | A verdict on every change | One fresh read where a mistake is costly, one round, no second review after a repair |
| Plan before code | Research and a plan forced on every task | A plan when the intent is unclear or an unknown is consequential |
| Test-driven development | Test first, always | A test that fails without the fix, then refactor |
| Memory across sessions | A pipeline that extracts and injects lessons | Short notes a person wrote and checked |
| Several agents | A swarm by default | One agent and one writer, with parallel work only where the pieces are independent |
| A skill library | More than 150 skills at the peak | 31, named when you want one applied |

**Independent review.** Mandatory review never converged here. A reviewer asked
to find problems finds one every time, and each repair started another round.
Outside work agrees: reviewers asked to explain and correct flag correct code
([Jin and Chen](https://arxiv.org/abs/2603.00539), 2026), and AI review recall
stays low at the best published setting
([AACR-Bench](https://arxiv.org/abs/2601.19494), 2026).

**Plan before code.** A benchmark ablation found planning raises accuracy for
weaker models and mostly saves cost for stronger ones
([Fan et al.](https://arxiv.org/abs/2609.20804), September 2026). A poor plan
does worse than no plan ([Liu et al.](https://arxiv.org/abs/2604.12147), 2026).
Here, the Research skill shows no gain on a current model because the unaided
agent already passes its case.

**Test-driven development.** In the one experiment that separates the pieces,
refactoring after each green did the work and the ordering of test and code did
not matter (Finster). A second informal test found no clear quality difference
from a TDD loop at three to eight times the tokens
([Böckeler](https://martinfowler.com/articles/exploring-gen-ai/tdd-in-the-agent-loop.html)).

**Memory across sessions.** The automated version failed here: retrieval
precision reached 0.13, and 23% of lessons extracted on unattended runs were
hallucinated ([origin story](origin-story.md)). Hand-written notes kept
working. Outside, any memory beat none on tasks built to need it, and no memory
design clearly beat another ([DreamBench-SWE](https://arxiv.org/abs/2608.20664),
2026). Skills that agents wrote for themselves gave no benefit
([SkillsBench v1](https://arxiv.org/abs/2602.12670v1), 2026).

**Several agents.** A multi-agent research system beat a single agent by 90% on
its vendor's own evaluation at about fifteen times the tokens of a chat, and the same
post notes that most coding tasks have fewer parts that can run in parallel
([Anthropic](https://www.anthropic.com/engineering/multi-agent-research-system),
June 2025). Splitting coder and tester between two agents cost up to three
times more for no consistent gain (Finster).

**A skill library.** Curated skills raised the pass rate by 16.2 points on
average in the first version of one benchmark, and 16 of 84 tasks got worse
([SkillsBench v1](https://arxiv.org/abs/2602.12670v1)). In a
vendor's evaluation the agent never invoked an available skill in 56% of cases
([Vercel](https://vercel.com/blog/agents-md-outperforms-skills-in-our-agent-evals),
January 2026). Here, the agent met 91% of practice criteria with the skills
against 68% without, and skills named in the request loaded on 22 of 24 runs
(plugin eval).

## Did not survive

| What was built | What happened |
|---|---|
| Knowledge that compounds automatically into an advantage | Unproven on a pre-registered test. A current model already applied the doctrine unaided ([ADR-0004](adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), June 2026). The product surface was deleted in August. |
| A cross-model verdict bound to every commit | It caught real defects and cost more than the work it checked. Review is now conditional ([ADR-0017](adr/ADR-0017-loop-as-control-flow-not-knowledge.md)). |
| Ratchets, grandfather lists and wording checks | They policed the shape of the repository and not its behavior. A check now stays only if it fails when something a user touches breaks. |
| Doctrine in the prompt as a control | No measurable effect in two sweeps (estate ablation). |
| A factory inside this repository: daemon, scheduler, control plane | Withdrawn. Orchestration belongs to the runtime you already use. |
| Test-first ordering and split authorship as rules | Not supported for agents in the one experiment that isolates them (Finster). |

## Still testing

- **Do the core loop skills help?** Validate, Test, Research and Orchestrate
  score the same with and without the skill, because the unaided agent already
  passes the cases. They need harder cases.
- **Does any of this hold on another model or harness?** Every gain measured
  here is on Claude Opus 5.5.
- **Why do skills not load unless you name them?** Four tested causes did not
  explain it.
- **Is a reviewer from another model family better than one from the same
  family?** A year of notes says yes. No controlled study, here or elsewhere,
  isolates it.
- **Do the gains reach the end of a real task?** Every measured gain is rule
  adherence on a single request. A coding pilot in September found no
  end-to-end difference ([product evidence](https://github.com/boshu2/agentops/blob/main/PRODUCT.md#evidence-and-claim-limits)).
- **Do curated notes improve later work?** A cold-reuse trial showed no benefit.
- **Reliable work from many agents at once.** Unsolved here.

## Lineage

The [Practice Registry](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md)
lists the books and papers each practice comes from. The registry is the
reading list. This page is what happened when the reading list met agents.
