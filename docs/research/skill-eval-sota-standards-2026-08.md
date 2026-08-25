# SOTA standards for evaluating agent skills — and what it takes to prove ours work

> **HISTORICAL** — dated research snapshot (2026-08-04). The in-repo `ao eval` surface it cites was later removed unconsumed; see docs/MIGRATION.md.

> **Date:** 2026-08-04 · **Status:** research synthesis (no code changes)
> **Method:** bounded deep-research workflow (3 search angles → 14 sources fetched → 70 claims
> extracted → top 8 adversarially verified: 3 confirmed, 5 killed), a targeted follow-up pass on
> the two sub-questions the first sweep missed (LLM-as-judge, tooling), plus a full inventory of
> this repo's existing eval substrate. Verification status is labeled per claim below —
> **verified** means it survived an adversarial refutation pass against the primary source;
> **extracted, unverified** means it was pulled from the source but not independently re-checked;
> **refuted** claims are listed so nobody re-imports them.
>
> **The question:** agentops is mainly a skill harness (~150 SKILL.md packages). We have no
> scientific evidence that the skills improve model output. What is the current SOTA for
> measuring that, and what would it take to run it here?

---

## TL;DR

1. **The field converged on one design in 2026:** the paired with-skill vs without-skill ablation
   over an execution-verified task suite, graded by deterministic verifiers (pytest, not LLM
   judges), with paired statistics at the task level. Two purpose-built substrates now exist:
   **SkillsBench** (84 tasks, 11 domains, 7 agent×model configs, 5 trials/condition) and
   **AFTER** (382 enterprise tasks, 6 roles, 22 procedural skills, explicit transfer splits).
2. **Skills help in aggregate — +16.2pp mean pass-rate uplift** (verified, independently
   replicated in direction) — but the aggregate is the least actionable number: domain effects
   span **+4.5pp (software engineering) to +51.9pp (healthcare)**, and a real minority of tasks
   *regress* under skill injection.
3. **In-context gain is not evidence of a good skill.** Cross-role transfer of an evolved skill
   loses 4.8–7.5 points; skills evolved from narrow traces show *source-context overfitting*
   (train gains, negative held-out deltas). Held-out / cross-model / cross-role splits are
   mandatory, not optional.
4. **The literature is statistically weak at the flagship level** — SkillsBench's launch results
   shipped with no error bars despite 5 trials/task. The standard worth copying (bootstrap CIs,
   paired permutation tests, Holm correction, task-level aggregation) appears in exactly one
   replication study.
5. **agentops already owns ~80% of the machinery** — a `skill-on|skill-off|both` A/B mode in
   `ao eval run`, ceiling pre-screen, holdout burn ledger, paired cluster bootstrap with
   power-derived n, a locked pre-registration template, deterministic graders. What's missing is
   a **skill-keyed task corpus with headroom**, a graded quality metric, and actually running it.
6. **Our existing null results (delta = 0 on 12 workbench tasks; 2 probes INERT) are exactly what
   SOTA predicts**, not evidence skills are useless: software engineering is the lowest-uplift
   domain, and frontier models on tasks without headroom saturate both arms. The binding
   constraint is task difficulty, not tooling.
7. **Tooling:** OpenAI Evals is deprecated (shutdown 2026-11-30; official migration target is
   promptfoo). Inspect (UK AISI) is the strongest OSS substrate; Terminal-Bench's harness is the
   closest fit for driving real Claude Code/Codex sessions over custom tasks. Anthropic's own
   Skills doctrine prescribes exactly the baseline-first A/B — and supplies zero statistics,
   which is the layer this repo already has.

---

## Part 1 — External SOTA

### 1.1 The converged experimental design

**Verified.** The paired ablation is now the established design: every task runs under both a
vanilla and a skill-augmented condition in identical containers, the with/without delta is the
measured quantity, and grading is execution-based wherever the task admits it (85/87 SkillsBench
tasks grade via pytest with CTRF output — no LLM judge on the primary metric).

- SkillsBench: 84 tasks × 11 domains × 7 agent-model configurations (Claude Code with Opus
  4.5/4.6, Sonnet 4.5, Haiku 4.5; Gemini CLI with Gemini 3 Flash/Pro; Codex with GPT-5.2),
  5 trials per condition, 7,308 trajectories, CI-based leakage audits proving the skill supplies
  *guidance not solutions*. Result: **+16.2pp mean uplift across all 7 configurations.**
  Sources: [skillsbench.ai launch post](https://www.skillsbench.ai/blogs/introducing-skillsbench),
  [arXiv 2602.12670](https://arxiv.org/abs/2602.12670).
- Independent replication (direction, not magnitude): a 30-task oracle-validated subset found
  **+18.0 to +36.0pp** with every skill-vs-no-skill CI above zero —
  [arXiv 2605.31408](https://arxiv.org/abs/2605.31408) (surfaced during verification; reported
  second-hand).

### 1.2 Heterogeneity: the mean hides everything a maintainer needs

**Verified as qualitative shape; per-task specifics unverified.** The same sources report domain
effects from **+4.5pp (Software Engineering) to +51.9pp (Healthcare)** and a nontrivial minority
of individual tasks with *negative* deltas (reported as 16 of 84; the standalone worst-case
figure −39.3pp failed adversarial verification — do not cite it).

Two operational consequences:

- A library-mean uplift number is nearly useless. **Inspect the per-task delta distribution**,
  not the mean — that is where both the winners and the regressions live.
- Software engineering — agentops' home domain — is where skills help *least*. Frontier coding
  models already carry most of the procedural knowledge a generic coding skill would inject.
  Expect small true effects here, which raises the sample-size bar (see 1.5).

### 1.3 Skills can actively hurt, and transfer is the real test

**Verified (with a required numeric correction).** From AFTER / EvoSkill
([arXiv 2606.23127](https://arxiv.org/abs/2606.23127)):

- In-role skill evolution gains +11.7 (PM) and +6.2 (DS); **applying a skill evolved for one role
  to another loses 4.8–7.5 points** (verbatim from §4.4; rests on one skill, `pdf`).
- **Source-context overfitting** (abstract, verbatim): "Skills evolved from narrow experience
  often exhibit source-context overfitting: they improve specificity while degrading generality."
  Corrected Table 3 numbers: narrow (n=1 source trace) **+14.9 train / −2.7 held-out**; diverse
  (n=5) **−5.7 train / −3.9 held-out**. (The widely-quoted "+14.9/−3.9" pair is a cross-condition
  splice — never cite it.) Caveats: unrefereed preprint, single solver (Qwen3.5-35B-A3B), no
  located replication.

**Extracted, unverified — but directionally important:** EvoClawBench
([arXiv 2607.09711](https://arxiv.org/html/2607.09711v1)) reports self-authored skill injection
collapsing one model (DeepSeek-V4-Pro: 77.8% baseline → 4.8% pre-skill → 1.0% post-skill) while
GPT-5.4 on the same runtime stayed flat and MiniMax improved. If it holds, **skill effects are
strongly model-dependent** — a skill that is inert on one backbone can be catastrophic on
another. Cross-model arms are not optional.

This is the single most important structural lesson for agentops: **a skill that "works" in the
session that authored it is the *weakest* form of evidence.** The eval that matters is held-out
tasks, other models, other contexts.

### 1.4 The AFTER substrate (what a skill-keyed benchmark looks like)

**Verified.** AFTER ("Managing Procedural Memory in LLM Agents," Belikova et al., 2026-06-22):
382 realistic enterprise tasks across 6 professional roles and 22 procedural skills, with
controlled splits for local improvement, cross-task, cross-role, and cross-model transfer.
Composition: 56 adapted from existing benchmarks (SkillsBench 19, SWE-bench Pro 6, SWE-bench
Verified 4, MLE-bench 4, FeatureBench 4, Terminal-bench 3, RE-Bench 2, others), 38
author-written, 288 LLM-drafted then expert-refined (two independent reviewers, 8-point rubric,
unanimous accept). Grading: pytest suites — M1 = average test-passage rate, M2 = full-pass
accuracy.

Caveats: unrefereed; **its own effect magnitudes shipped without CIs and were refuted at
verification — cite the design, not the numbers.** Also 288/382 tasks were drafted by Claude
Sonnet 4.6, a disclosed-but-uncorrected family bias when evaluating Claude-family agents.

### 1.5 Statistical rigor: the field's weakness, and the standard to copy

**Verified critique:** the SkillsBench launch post contains no error bars, CIs, or significance
tests anywhere despite 5 trials/task; AFTER's headline effects are single-shot point estimates.
The one located study doing it right ([arXiv 2605.31408](https://arxiv.org/abs/2605.31408)) used:
**bootstrap 95% CIs, paired Monte Carlo permutation tests (100k samples), Holm correction for
multiplicity, task-level (not trial-level) aggregation.**

**The supporting statistics stack (Miller confirmed in the follow-up pass; rest extracted):**

- Miller, *Adding Error Bars to Evals* ([arXiv 2411.00640](https://arxiv.org/abs/2411.00640),
  Anthropic, Nov 2024 — recommendations confirmed against the source): treat an eval as sampling
  questions from a superpopulation; CLT-based SEM with 95% CI; **clustered standard errors** when
  questions group naturally (clustered SEs can be >3× naive); run inference on **question-level
  paired differences** (question scores correlate across conditions, so pairing is a free
  variance reduction); **power analysis** to size the question count before running. Still the
  canonical reference, and it maps 1:1 onto a with/without-skill contrast.
- **pass^k** (τ-bench, [arXiv 2406.12045](https://arxiv.org/abs/2406.12045)): probability the
  agent succeeds on *all* k i.i.d. trials of a task — the standard consistency metric. GPT-4o
  fell below 25% at pass^8 on τ-bench retail, which made run-to-run variance a first-class
  agent-eval concern. For a skill claiming to make behavior *reliable* (most agentops skills),
  pass^k is the right headline metric, not pass@1.
- **Agentic Benchmark Checklist** (ABC, [arXiv 2507.02825](https://arxiv.org/abs/2507.02825)):
  audits benchmarks for task-validity and outcome-validity flaws — SWE-bench Verified's
  insufficient tests, τ-bench counting empty responses as success — with distortions up to 100%
  relative. Run it against any task corpus before trusting the numbers it produces.
- **HAL Reliability** ([hal.cs.princeton.edu/reliability](https://hal.cs.princeton.edu/reliability/)):
  multi-run measurement of consistency, robustness, and abstention — the operational successor to
  the variance-across-runs agenda.
- For small deltas under low compute: **paired multi-seed runs (same seeds both arms), BCa
  bootstrap CIs, sign-flip permutation tests on per-seed deltas**
  ([arXiv 2511.19794](https://arxiv.org/abs/2511.19794), extracted/unverified).

**No source gives a sample-size prescription.** Nothing in either research pass states the
trials-per-task needed to detect, say, +5pp on a 20-task suite. This is an open question the
field has not answered; power analysis on observed variance is the only honest path.

### 1.6 The context-dilution confound is UNRESOLVED

The one direct result on skill verbosity/count (compact skills +18.9pp vs comprehensive +5.7pp;
2–3 skills +20.0pp vs 4+ skills +5.2pp) **failed adversarial verification — do not cite those
numbers.** What survives is adjacent evidence that added context has a real cost:

- **Context rot** (Chroma, [trychroma.com/research/context-rot](https://www.trychroma.com/research/context-rot),
  extracted/unverified): on LongMemEval, all 18 tested models scored significantly higher on
  ~300-token focused prompts than ~113k-token full prompts *containing the same relevant
  content*. Irrelevant surrounding context degrades performance even when the needed information
  is present.
- More inference effort hurt accuracy in 21 of 36 tested configurations
  ([arXiv 2510.11977](https://arxiv.org/abs/2510.11977), extracted/unverified) — "more
  tokens/more thinking" is not monotonically beneficial.

Consequence: **a with-skill regression may be a length effect, not a content effect — and a
with-skill gain may partially be "any structured text here helps."** No located study runs the
control that would separate these: a **length-matched placebo arm** (same token count, generic
content). If agentops runs one, that is publishable methodology, not just internal QA.

### 1.7 LLM-as-judge: failure modes and current practice

The follow-up pass covered this directly. The failure modes are empirically quantified:

- **Position bias**: pairwise verdicts flip on candidate reordering; the CALM framework
  quantifies it alongside 11 other biases (verbosity, authority, bandwagon, distraction, …) —
  [arXiv 2410.02736](https://arxiv.org/abs/2410.02736). Standard mitigation: run both orderings,
  count inconsistent verdicts as ties. And it is **not pairwise-only** — 2026 work shows
  rubric-based pointwise grading exhibits order sensitivity too
  ([arXiv 2602.02219](https://arxiv.org/pdf/2602.02219)).
- **Verbosity/length bias**: judges systematically favor longer outputs
  ([arXiv 2410.02736](https://arxiv.org/abs/2410.02736)).
- **Self-preference**: judges score their own family's outputs higher, mechanistically linked to
  output perplexity — judges favor text that reads low-perplexity *to them*, i.e.
  style-over-substance ([arXiv 2410.21819](https://arxiv.org/abs/2410.21819)). Directly relevant
  when a Claude judge grades Claude-produced skill output — cross-family judging is the control,
  which is already house doctrine here.
- **Agreement inflation**: exact-match agreement is not chance-corrected; Cohen's kappa deflates
  measured judge agreement by 33–41pp on MT-Bench
  ([arXiv 2606.19544](https://arxiv.org/abs/2606.19544), extracted/unverified). A judge validated
  only on raw agreement is overstated.

**Practice consensus 2025–2026** ([survey, arXiv 2411.15594](https://arxiv.org/html/2411.15594v6)):
rubric-anchored grading — explicit criteria, **one criterion per judge call**, forced evidence
citation — for regression tracking; pairwise with position-swap for comparisons; calibrate
against a human-labeled set and report chance-corrected agreement before trusting the judge.
Deterministic/execution-based graders wherever the outcome is checkable, with the judge reserved
for the residual subjective surface — explicitly Anthropic's guidance for Skills and the core
argument of the ABC paper.

**For multi-step agent trajectories specifically:** the successor line to "Judging
LLM-as-a-Judge" is **Agent-as-a-Judge** (Meta, DevAI; surveyed in
[arXiv 2508.02994](https://arxiv.org/pdf/2508.02994)) — an agentic evaluator inspecting the whole
trajectory (tool calls, intermediate artifacts), not just the final answer. Current practice
splits evaluation into three levels: end-to-end task success, trajectory-level (tool-call
correctness, path efficiency, error handling as separate axes), and component-level. HAL
operationalized this at scale — LLM-aided inspection of 2.5B tokens of rollout logs surfaced
failure modes invisible to outcome-only scoring, including agents googling benchmark answers
([arXiv 2510.11977](https://arxiv.org/abs/2510.11977)).

Two skill-specific data points:

- **Anthropic's skill-creator** ships **"comparator agents"** — blinded pairwise judge ablation,
  skill vs no-skill or v1 vs v2, judge blind to condition — plus a benchmark mode tracking pass
  rate, elapsed time, and token usage
  ([claude.com blog](https://claude.com/blog/improving-skill-creator-test-measure-and-refine-agent-skills)).
- A progressive-disclosure ablation on SkillsBench (82 tasks, GPT-5.4, 5 trials/condition,
  [arXiv 2606.11543](https://arxiv.org/pdf/2606.11543), extracted/unverified): no skill 29.0% →
  flat skill 42.0% → progressive-disclosure skill 46.1%. The with/without gap (~13pp) was ~3×
  the formatting gap — *having* the skill matters more than its internal structure.

### 1.8 Tooling landscape (what to build on, what to avoid)

From the follow-up pass; feature-level claims about paired-stats support are assessments unless
marked verified.

| Harness | Status / fit for with-vs-without-skill |
|---|---|
| **UK AISI Inspect** ([inspect.aisi.org.uk](https://inspect.aisi.org.uk/)) | Actively maintained; dataset→Task→Solver→Scorer; sandboxed agent support (Docker/K8s), 200+ prebuilt evals. Ships `stderr` with a `cluster` parameter (the Miller recommendation) and `bootstrap_stderr`. Best-in-class OSS substrate; per-sample logs make paired analysis straightforward, though no built-in paired significance test. |
| **OpenAI Evals** | **Dead end — deprecated June 2026, read-only Oct 31, shutdown Nov 30 2026.** Official migration target is promptfoo ([deprecation notice](https://community.openai.com/t/deprecation-notice-evals-will-be-shut-down-on-november-30th-2026/1385537)). Do not build on it. |
| **promptfoo** ([promptfoo.dev](https://www.promptfoo.dev/docs/intro/)) | OSS CLI; deterministic assertions + model-graded metrics; native side-by-side matrix across configs (maps directly to skill-on vs skill-off system contexts); CI GitHub Action. Built-in significance testing unconfirmed. |
| **Terminal-Bench harness** ([github.com/laude-institute/terminal-bench](https://github.com/laude-institute/terminal-bench)) | Container + instruction + deterministic verifier per task; supports custom tasks and runs real installed CLI agents (Claude Code, Codex CLI adapters — from prior knowledge, unverified). **Closest existing substrate to "run our own task suite through a real Claude Code session with/without a skill."** |
| **HAL harness** ([github.com/princeton-pli/hal-harness](https://github.com/princeton-pli/hal-harness)) | Standardized cost-aware third-party evaluation; 21,730 rollouts across 9 models × 9 benchmarks; the scaffold dimension is exactly "agent config with vs without skill". Heavyweight (VM fleet) but open. |
| **DSPy + GEPA** ([arXiv 2507.19457](https://arxiv.org/pdf/2507.19457), ICLR 2026) | Not an eval harness — a **skill-text optimizer**: reflective prompt evolution reading execution traces, Pareto-frontier candidates, ~35× fewer rollouts than RL baselines. Once a skill has a task metric, GEPA can *evolve the SKILL.md body against it* — a skill is functionally a prompt module. The eval corpus is the prerequisite. |
| **Braintrust / LangSmith** | Hosted experiment-diffing platforms; run-vs-run diffs map to with/without, but vendor lock-in and no CLI-agent harness out of the box (background knowledge, not searched). |

**Anthropic's own Skills eval doctrine**
([platform.claude.com best-practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices))
mandates evaluation-driven development: **baseline Claude *without* the skill first, build ≥3
evaluation scenarios targeting observed gaps, write minimal instructions, re-run vs baseline** —
and explicitly notes "There is not currently a built-in way to run these evaluations." No
sample-size or statistics guidance anywhere in the vendor surface. In other words: the vendor
prescribes exactly the A/B shape, provides no rigor layer, and leaves the statistics to the
maintainer — which is precisely the gap this repo's `_stats` + pre-reg machinery already fills.

### 1.9 Ecological validity — the critique that matters most for agentops

One independent review (Tao Dong, "Evaluating Evals") argues the +16.2pp average **"may be an
upper bound" because every benchmark injects the correct skill by construction** — they measure
*efficacy given perfect retrieval*, not end-to-end library value including routing failures. The
end-to-end quantity a maintainer cares about is:

```
library value = P(right skill loaded) × E[uplift | loaded] − P(wrong/irrelevant skill loaded) × E[cost | mis-loaded]
```

No located benchmark measures the joint. For a ~150-skill library injected as a flat list into
context — exactly our situation, and exactly the failure mode our own INERT graphify probe
demonstrated (agents ignoring a verbatim doc instruction) — **routing may dominate efficacy.**

### 1.10 Refuted claims — do not re-import these

Killed 0–1 by adversarial verification; they circulate widely, so they're listed to stay dead:

| Refuted claim | Why it matters |
|---|---|
| SkillsBench task accounting "86 − 2 broken verifiers = 84" as design description | detail didn't survive source check |
| Negative-tail specifics: "16/84 negative, worst −39.3pp" as standalone numbers | qualitative shape survives; exact numbers don't |
| Verbosity/count confound: compact +18.9 vs comprehensive +5.7; 2–3 skills +20.0 vs 4+ +5.2 | was the only direct context-dilution evidence; **the confound is unresolved** |
| AFTER's own effect magnitudes (+2.8 M2 static; +3.7–6.7 refined; +14.2 Gemma) | single-shot, no CIs |
| SkillOpt +23.5pp direct-chat ablation on frozen GPT-5.5 (Microsoft Research blog) | did not survive verification |

### 1.11 Time-sensitivity

SkillsBench is ~6 months old and its tested models are already superseded. **Point estimates go
stale within ~2 model generations; the structural findings (heterogeneity, negative transfer,
overfitting, model-dependence) are the durable part.** Any eval we build must be cheap enough to
re-run per model generation — a one-shot study is obsolete on arrival.

---

## Part 2 — What agentops already has (inventory, 2026-08-04)

The surprising answer: **a near-complete scientific eval program already exists in-tree — it is
pointed at the `.agents` knowledge corpus, not at skills, and it has barely been run.**

### Already built (the good news)

| SOTA requirement | Existing repo surface |
|---|---|
| Paired A/B runner | `cli/internal/eval/baseline_ab.go` — `ao eval run --baseline-mode skill-on\|skill-off\|both` with a `DeltaScorecard` |
| Ceiling/headroom pre-screen | `cli/internal/eval/scenario_ab.go` — runs control first, aborts if it already clears threshold ("no headroom, delta uninterpretable") |
| Deterministic grading | `answer_key_judge.go` (whole-token, zero judge noise); workbench `score.sh` per task |
| Holdout isolation | `cli/internal/evalsubstrate/holdout_guard.go` + `holdout_burn.go` (burn ledger with per-suite quotas) |
| Paired statistics + power | `ao eval suite verdict` / `n-required` → paired cluster bootstrap (B=10k, 95% CI), power-derived n, seeded + inputs-hashed |
| Pre-registration discipline | `evals/workbench/corpus-delta-w1c-prereg.md` — locked MDE (0.15), fixed seeds, locked publication rule, mechanical category assignment |
| Task corpus (generic) | `evals/workbench/` — 22 tasks with setup/score/golden, `scripts/eval-agent-harness.sh` |
| Behavioral probe harness | `evals/skill-probes/` + `scripts/probe-skill.sh` — the only with/without-skill harness; discriminator checks the *action*, fixtures for replay |
| Honesty doctrine | `docs/evals/agentops-effectiveness-evidence.md`, `applied-ood-claim-rule.md`, `moat_claim.go` (`moat_positive\|honest_null\|inconclusive`) |

This is more rigor than the flagship published benchmark shipped with. The repo's problem is not
tooling.

### The evidence to date is null — and SOTA explains why

- Skill/hook A/B on 12 workbench tasks: **aggregate delta = 0** — every case passed both arms
  (ceiling-saturated).
- Both behavioral probes ever run (`crank`, `graphify-tool-preference`): **INERT** — the frontier
  model already did the right thing unaided, or ignored the doc instruction in both arms.
- First corpus A/B in adjacent work: **−0.37** (a thin gold pull actively hurt — consistent with
  context-rot).

SkillsBench found its smallest uplift (+4.5pp) in exactly our domain, on tasks hard enough to
have headroom. Our tasks weren't. **Three independent local measurements failed the same way:
no headroom → no measurable treatment effect.** The nulls are diagnostic, not damning.

### The gaps (priority order)

1. **No skill-keyed task corpus.** 2 probe questions cover 2 of ~150 skills; the 22 workbench
   tasks discriminate corpus-presence, and nothing maps task → skill-under-test.
2. **Ceiling is the binding constraint.** New tasks must *start* from the headroom pre-screen and
   use weaker producer models (`probe-skill.sh --model` already supports this) or genuinely
   harder/OOD tasks. `scenario_ab.go` implements the abort; the workbench A/B doesn't use it.
3. **No graded quality metric.** Everything is binary pass/fail or behavior present/absent — the
   probe README explicitly disclaims quality measurement. `evalsubstrate/rubric.go` +
   `schemas/outcomes-rubric.v1.schema.json` are the transport; no skill-output rubric exists and
   no judge is calibrated in-repo (no kappa, no position-bias control).
4. **`_stats` is an unpinned external dependency** at `~/.agents/evals/_stats/` — nothing vendors
   it or fails loudly if absent; the workbench pre-reg and `_stats` are two disconnected
   statistical stories.
5. **The MEASURED probe ledger is empty.** `scripts/check-skill-probe-coverage.sh` reports
   `{"gated_total":11,"measured":0}` — the ledger section in `skills/SKILL-TIERS.md` was wiped by
   a regeneration, and the gate is non-blocking so nobody noticed. Small, high-value fix.
6. **No baseline runs promoted.** `.agents/evals/baselines/` is empty — there is no "without"
   reference to compare against over time.
7. **No skill→outcome linkage.** `skill-telemetry.jsonl` has 1 record (April); nothing joins
   "skill X loaded in session S" to "session S outcome Y", so even observational signal is
   uncomputable.
8. **No skill-scoped pre-registration.** The corpus-delta pre-reg is exemplary but scoped to the
   corpus axis.

---

## Part 3 — The recommended program

Design principle: **copy the one rigorous replication's statistics, AFTER's transfer splits,
SkillsBench's deterministic-verifier discipline, and add the placebo arm nobody ran.** Reuse
in-tree machinery everywhere; build only the task corpus.

### Phase 0 — hygiene (a day)

- Restore the `## Behavioral Probe Ledger (MEASURED)` section in `skills/SKILL-TIERS.md`; make
  `check-skill-probe-coverage` blocking so regeneration can't silently wipe it again.
- Vendor/pin `_stats` into the repo (or fail loudly when absent); wire the pre-reg statistic and
  `ao eval suite verdict` to the same implementation.

### Phase 1 — design + pre-registration (before any run)

- **Pick 10–15 skills where a true effect is plausible and valuable** — tier `product`/`judgment`
  skills with real procedural content (e.g. `validate`, `premortem`, `council`, `beads-workflow`,
  `agent-mail`), not thin router aliases.
- **Author 8–20 skill-keyed tasks per skill** where a naive agent plausibly fails: derived from
  the skill's own failure archive (memory files, postmortems, refuted-verdict history), OOD
  variants, and real past incidents. Every task: `setup.sh` + deterministic `score.sh` (or graded
  rubric where binary is too coarse), plus a leakage audit confirming the skill gives guidance,
  not the answer. This is also exactly Anthropic's prescribed shape — baseline without the skill
  first, ≥3 scenarios targeting observed gaps — with the rigor layer the vendor doesn't provide.
  Sanity-check the corpus against the ABC checklist (task validity + outcome validity) before
  trusting anything it produces.
- **Headroom pre-screen every task** (existing `scenario_ab.go` mechanism): run skill-off first;
  if the naive arm already passes, the task is dead for measurement. Use weaker producers
  (`--model gpt-5-mini`, local models) where frontier models saturate.
- **Three arms, not two:** skill-on / skill-off / **length-matched placebo** (equal-length
  well-formed but generic guidance). This resolves the confound the literature left open and
  distinguishes "this skill helps" from "any structured text helps".
- **Pre-register** (clone `corpus-delta-w1c-prereg.md`): frozen skill list and task set, seed
  list, ≥5 trials per task per arm, MDE fixed before compute, paired cluster bootstrap + sign-flip
  permutation, Holm correction across skills, locked publication rule including the honest-null
  outcome. Run `ao eval suite n-required` on pilot variance before committing scope — with
  per-task deltas ranging ±40pp in the literature, +5pp per-skill effects may be undetectable at
  affordable n; if so, claim at the tier/library level, not per-skill, and say so in advance.

### Phase 2 — run and analyze

- Execute through the existing `ao eval run --baseline-mode both` path; workers via `codex exec`
  (per probe harness convention — never `claude -p`).
- **Report the per-task delta distribution, not just the mean.** A skill whose distribution has a
  negative tail is a regression risk even at positive mean. For reliability-flavored skills,
  report **pass^k**, not pass@1 — consistency is the product claim.
- Judges only where execution can't grade, and then with the full control set: rubric-anchored,
  one criterion per call, position-swapped, condition-blinded (Anthropic's comparator pattern),
  **cross-family** (a Codex/Gemini judge for Claude output — self-preference bias is
  perplexity-linked), validated with Cohen's kappa against a human-labeled calibration set, never
  raw agreement.

### Phase 3 — transfer and routing (the part SOTA says we'll otherwise fake)

- **Cross-model arm** (Claude pane + Codex + AGY): a skill's effect on one backbone doesn't
  transfer (EvoClawBench), and cross-family checks are already house doctrine.
- **Held-out task split per skill:** tasks the skill author never saw. In-context gain without
  held-out gain = source-context overfitting = the skill overfits its origin story.
- **Measure routing separately:** the benchmarks measure efficacy-given-perfect-retrieval; our
  real risk is the flat ~150-skill list. Revive `skill-telemetry.jsonl` (or mine session
  transcripts via `scripts/skill-usage-report.sh`) to estimate P(right skill loaded), and probe
  it directly: given task T whose matching skill exists, does the agent load it? That number
  multiplies every uplift we ever measure.

### Phase 4 (later) — optimization, only after measurement exists

Once a skill has a task metric, **GEPA-style reflective evolution can optimize the SKILL.md body
against it** (a skill is functionally a prompt module). This is the payoff of building the
corpus: the same tasks that prove a skill works become the fitness function that improves it.
Strictly after Phases 1–3 — optimizing against an unvalidated metric is how you Goodhart a
library.

### What honest output looks like

Per skill: `improves | inert | degrades | unmeasurable (no headroom at affordable n)`, with CI,
n, and the arm distributions — persisted through the existing verdict/scorecard machinery, and
re-runnable per model generation because the point estimates will go stale.

Expected outcome worth stating in advance: **many skills will come back inert or unmeasurable.**
That is the SOTA-consistent result (+4.5pp domain mean in software engineering, tails both ways)
and it is actionable: inert skills are candidates for culling (cull = count is already house
doctrine), and the ones that survive get a MEASURED badge that finally means something.

---

## Sources

**Verified primary:** [SkillsBench launch](https://www.skillsbench.ai/blogs/introducing-skillsbench) ·
[SkillsBench paper, arXiv 2602.12670](https://arxiv.org/abs/2602.12670) ·
[AFTER / EvoSkill, arXiv 2606.23127](https://arxiv.org/abs/2606.23127)

**Second-hand (surfaced in verification):** [arXiv 2605.31408](https://arxiv.org/abs/2605.31408)
(rigorous replication — bootstrap CI, 100k permutation, Holm)

**Confirmed in follow-up pass:** [Miller, Adding Error Bars to Evals, arXiv 2411.00640](https://arxiv.org/abs/2411.00640) ·
[τ-bench / pass^k, arXiv 2406.12045](https://arxiv.org/abs/2406.12045) ·
[Agentic Benchmark Checklist, arXiv 2507.02825](https://arxiv.org/abs/2507.02825) ·
[HAL, arXiv 2510.11977](https://arxiv.org/abs/2510.11977) + [Reliability leaderboard](https://hal.cs.princeton.edu/reliability/) ·
[OpenAI Evals deprecation notice](https://community.openai.com/t/deprecation-notice-evals-will-be-shut-down-on-november-30th-2026/1385537) ·
[Anthropic Skills best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices) ·
[Anthropic skill-creator blog](https://claude.com/blog/improving-skill-creator-test-measure-and-refine-agent-skills) ·
[GEPA, arXiv 2507.19457](https://arxiv.org/pdf/2507.19457) ·
[Inspect (UK AISI)](https://inspect.aisi.org.uk/) + [inspect_ai](https://github.com/UKGovernmentBEIS/inspect_ai) ·
[promptfoo](https://www.promptfoo.dev/docs/intro/) ·
[Terminal-Bench harness](https://github.com/laude-institute/terminal-bench) ·
[HAL harness](https://github.com/princeton-pli/hal-harness)

**Judge-bias literature (follow-up pass):** [CALM, arXiv 2410.02736](https://arxiv.org/abs/2410.02736) ·
[rubric position bias, arXiv 2602.02219](https://arxiv.org/pdf/2602.02219) ·
[self-preference bias, arXiv 2410.21819](https://arxiv.org/abs/2410.21819) ·
[LLM-as-judge survey, arXiv 2411.15594](https://arxiv.org/html/2411.15594v6) ·
[Agent-as-a-Judge survey, arXiv 2508.02994](https://arxiv.org/pdf/2508.02994)

**Extracted, unverified:** [arXiv 2511.19794](https://arxiv.org/abs/2511.19794) (BCa bootstrap + sign-flip protocol) ·
[arXiv 2606.19544](https://arxiv.org/abs/2606.19544) (judge kappa deflation) ·
[arXiv 2607.09711](https://arxiv.org/html/2607.09711v1) (EvoClawBench) ·
[arXiv 2606.11543](https://arxiv.org/pdf/2606.11543) (progressive disclosure ablation) ·
[arXiv 2510.04618](https://arxiv.org/abs/2510.04618) (ACE, evolved-context uplift) ·
[Chroma context-rot](https://www.trychroma.com/research/context-rot) ·
[Microsoft SkillOpt blog](https://www.microsoft.com/en-us/research/blog/skillopt-agent-skills-as-trainable-parameters/) (headline claim refuted — design reference only)

**In-repo:** `docs/evals/agentops-effectiveness-evidence.md` ·
`evals/workbench/corpus-delta-w1c-prereg.md` · `evals/skill-probes/` ·
`cli/internal/eval/` + `cli/internal/evalsubstrate/` · `docs/evals/2026-07-08-skill-probe-*.md`
