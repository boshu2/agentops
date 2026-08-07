# AgentOps Eval Architecture — v1

> **Status:** PROPOSED (Bo to ratify) · **Date:** 2026-08-04
> **Terminology note (2026-08-07):** this dated proposal predates the
> operations-layer alignment and keeps its original wording ("the operating
> loop", "~150 skills"). The current product category and vocabulary live in
> [`../contracts/ubiquitous-language.md`](../contracts/ubiquitous-language.md);
> the catalog is 51 canonical skills. Read the decisions here against that
> current frame.
> **Inputs:** [skill-eval SOTA research](../research/skill-eval-sota-standards-2026-08.md) ·
> [effectiveness evidence audit](../evals/agentops-effectiveness-evidence.md) · the in-tree eval
> substrate (`cli/internal/eval/`, `cli/internal/evalsubstrate/`, `evals/`)
> **Shape:** a decision document. Each decision is DECIDED-unless-Bo-vetoes, with the rejected
> alternative named. The goal is one coherent system, not a menu.

---

## 0. The claim we are in business to prove

AgentOps' product is the harness: ~150 skills + the `ao` CLI + the operating loop, promising to
turn any model into a software engineer **whose output you can trust**. That promise decomposes
into three measurable claims:

| Claim | Metric | Where the headroom is |
|---|---|---|
| **Capability**: the harness helps the model complete more real tasks | paired Δ pass-rate on execution-verified tasks | Weak/mid models only. Frontier models saturate (our delta=0 results; SkillsBench's +4.5pp SWE floor). |
| **Trust**: the harness reduces confidently-wrong output | **false-PASS rate**: agent claims done + self-green, fresh deterministic/independent validation refutes it | **All model tiers, including Fable.** Frontier models still overclaim. Nobody else measures this. |
| **Economics**: trusted output at acceptable cost | tokens + wall-clock per *validated* task; pass^k for reliability | All tiers. "Cheap model + harness ≥ expensive model bare" is the money claim. |

**North-star metric: validated-throughput per dollar** — independently-validated task completions
per unit of spend — with **false-PASS rate as the trust guardrail**. Raw pass-rate is a
component, never the headline.

The reframe that falls out of all our null results: **at the frontier, the harness's measurable
value is the validation layer, not the capability layer.** Stop trying to prove Fable writes
better code with skills loaded; prove the harness catches Fable lying about being done, and
prove it lifts mini-class models into the trustworthy zone. Both are measurable. The first is
the moat; the second is the market story (the Devin-shaped claim: uplift curve across model
tiers, biggest at the bottom).

---

## 1. Four surfaces, one harness

Everything Bo asked about is a **treatment** applied to the same task substrate. One eval
engine, four treatment types:

| Surface | Treatment shape | Question it answers |
|---|---|---|
| **Skills** (SKILL.md packages) | inject / omit / placebo per skill | does this skill improve, do nothing, or degrade? |
| **System prompts** (CLAUDE.md / AGENTS.md) | whole-file and section-level ablation arms + rule-adherence canaries | is it load-bearing or exploded? |
| **Deterministic layer** (`ao` CLI, gates, hooks) | conventional software tests (it's deterministic — no LLM needed) + gates-on/gates-off arms at harness tier | is the CLI correct; do the gates pay their friction? |
| **Whole harness** | bare-model vs full-agentops arms, per model tier | the product claim itself |

This unification is the answer to "how do I know if my CLAUDE.md exploded" — it's the same
paired-arm machinery as skills, run at a coarser grain and lower cadence.

---

## 2. The decisions

### D1 — The unit of proof is the paired ablation; the unit of trust is false-PASS rate
Every treatment is measured as paired arms over identical, hermetic tasks with deterministic
verifiers, per the SOTA convergence. Every run additionally records the agent's **own completion
claim** (did it assert done/passing?) so claim-vs-truth is computed for free on every run. This
one logging decision turns every eval into a trust measurement.
**Anti-Goodhart pairing (from ponytail's harness):** every headline metric declares its
gaming-detector in the same eval pack — false-PASS↓ pairs with throughput/false-FAIL (a config
must not win by refusing to claim done); LOC↓ pairs with completeness + deterministic safety;
pass-rate↑ pairs with cost. A metric without its guard is not a valid pack.
*Rejected:* LLM-judged quality scores as primary metric (judge-bias literature; deterministic
verifiers exist for coding).

### D2 — One frozen reference config for all skill measurement
All Tier 1/2 skill measurement runs on **one pinned worker**: Codex CLI, current mini-class
model, reasoning=medium, pinned in `evalsubstrate/modelspec.go` and re-pinned once per model
generation. Rationale: (a) weak models have headroom → bigger, cheaper-to-detect effects
(SkillsBench: uplift shrinks as capability rises); (b) it spends the GPT quota, preserving
Claude Max for orchestration/judging; (c) LAW 0 makes headless Claude workers operationally
expensive anyway; (d) skills ship cross-runtime (`skills-codex/`) so Codex measurement is the
real product on its real second runtime, not a proxy.
Claude-native skills (hooks, Skill-tool mechanics, anything that only exists in the Claude
runtime) get a small **Claude-native subset** run through NTM panes / in-session subagents —
the sanctioned Claude lanes.
*Rejected:* measuring every skill on every model. The grid is dead; transfer is D6's job.

### D3 — Three-stage adaptive arms, not a fixed three-arm design
- **Stage A (screen):** {skill-off, skill-on}, paired seeds, 8–12 tasks × 3 trials.
- **Stage B (attribution, winners only):** add the **length-and-register-matched placebo arm**
  (same token count, same imperative prose register, generic content). Only skills that beat OFF
  earn a placebo run. This resolves the context-dilution confound the literature left open — and
  separates "the ladder works" from "terse commanding prose works," which ponytail's four-arm
  benchmark proved is a real distinction (its style-control arm cut LOC 20% but cost *more*;
  the ruleset arm cut 54% and cost less).
- **Stage C (transfer, top skills only):** held-out task variants + one cross-model spot-check.
*Rejected:* running placebo on everything (wastes a third of budget on losers).

### D4 — Sequential stopping, not fixed n
After each batch, compute the paired cluster bootstrap CI (`ao eval suite verdict`):
CI excludes 0 → conclude (improves/degrades). CI inside ±MDE → **inert**, stop. Neither, and
budget cap hit → **unmeasurable at affordable n**, recorded as such. MDE fixed in the pre-reg
(default 0.10 at reference config; we expect large effects on a weak worker or none at all).
This is how a weekly-renewable subscription budget does science: spend until the answer is
clear or provably not affordable, never "run the full design regardless."
*Rejected:* SkillsBench-style fixed 5 trials everywhere.

### D5 — Verdict vocabulary per skill
`improves | inert | degrades | unmeasurable` + effect, CI, n, config, date, evidence path —
persisted in `skills/catalog.json` and rendered as the **MEASURED badge** in SKILL-TIERS.md.
Two INERT probe results or a measured-inert Tier 2 → cull-or-merge proposal (cull = count is
already doctrine). A library where every skill carries an error bar is itself the product
differentiation: no other skill library ships evidence.

### D6 — The model/reasoning matrix policy (the explicit answer to "do we have enough tokens?")
**No, and we don't need them.** The full {Fable, Opus, GPT-5.6, Soul, Luna} × {reasoning
levels} × {skills} grid is a non-goal, permanently.
- **Reasoning levels:** one calibration study per model generation — reference task subset
  (~20 tasks), 2 levels (medium vs high), pick the operating point, freeze it in modelspec.
  Justified by HAL: higher reasoning effort *reduced* accuracy in 21/36 configurations — audit
  once, don't sweep.
- **Cross-model transfer:** top-5 measured skills × 2 additional configs (Fable-high as the
  ceiling reference; whatever mid-tier Codex model is current) × the reference subset, once per
  model generation. EvoClawBench says skill effects are model-dependent (one model collapsed
  78%→1% under a skill that left another flat) — so transfer must be *spot-checked*, but
  spot-checking is enough to catch catastrophes.
- **Fable's role:** never a bulk eval worker. Fable = orchestrator, judge-calibrator, and one
  arm of the quarterly harness-tier run. (Consistent with the existing model-routing law:
  orchestrate high, work cheap.)
- Model names live in one config table; the suite is model-agnostic by construction.

### D7 — The task substrate is a digital twin: hermetic, metamorphic, fault-injected
This is the Antithesis/Echelon port, aimed at the environment rather than the model. Ordering
note from the formal-verification research ([companion doc](../research/formal-verification-for-agent-code-2026-08.md)):
the DST practitioners themselves rank **oracles before explorers** — TigerBeetle puts assertion
density ahead of its own simulator; Antithesis says the simulator cannot judge without
properties you define. So the rungs below are climbed in this order: hermetic fixtures →
**assertion/invariant density** (in fixtures and in agent-generated code) → fault injection +
metamorphic exploration → anything heavier only where cost evidence supports it.
**The oracle, not the executor, is the scarce artifact** — which is the membrane thesis in
formal-methods language.
- **Hermetic fixtures** (the twin): every task = container/worktree with pinned deps, no
  network, seeded everything, deterministic `score.sh`. The existing workbench shape, hardened.
  Determinism of the *world* means outcome differences are attributable to the policy
  (model+treatment) — and any run is replayable for autopsy.
- **Arm-isolation is asserted, not assumed:** the runner **positively verifies treatment absence
  in control transcripts** (grep the injected content in session JSON), never just omits the
  plugin. Ponytail's baseline arm was silently contaminated by its own SessionStart hook (their historical bug, since retracted) and
  they had to retract a run; with ~150 skills symlinked into `~/.claude/skills` plus ambient
  hooks, our control arms carry the identical exposure. Control runs execute with a scrubbed
  skill/hook surface (dedicated HOME or `--setting-sources`-style isolation), and the assertion
  is part of `score.sh`, so a contaminated cell fails loudly instead of measuring zero.
- **Preserved workspaces + offline re-scoring:** every cell's workspace, transcript, and scores
  persist under the run directory; metric logic can change and re-score old runs without
  re-spending tokens (`--rescore`), and the whole harness self-tests with zero API cost
  (`--selftest`). Copied from ponytail's harness ergonomics, near verbatim.
- **Metamorphic variants** (the Antithesis exploration analog): each task ships a mutation
  recipe — identifier renames, file moves, distractor files, paraphrased instructions,
  equivalent-behavior refactors of the fixture. Each wave draws fresh variants. This kills
  contamination/memorization (LiveBench's concern), measures robustness (does the skill help
  the *class* of task or one phrasing?), and later lets GEPA optimize skill text on train
  variants while verdicts come only from held-out variants (anti-Goodhart). The
  `testing-metamorphic` skill is the in-house prior art.
- **Fault injection** (the "sometimes-assertion" analog): a seeded-bug task family — plant a
  known defect class in the fixture, measure detection/repair rate. The FW-GC-ALT-01 memo
  already specifies the single-injected-fault design; generalize it. Also **invariant
  assertions inside fixtures**: properties that must hold in the end state regardless of path
  (no secrets in diff, tests still green, no force-push, ledger balanced). Invariant violations
  are free-to-grade trust failures.
- **Formal-methods lane (bounded):** the `ao` CLI is deterministic Go — it gets property-based
  tests and fuzzing on parsers/state machines in CI (this is where "formal" is real today).
  Running agent output itself under Antithesis-style autonomous testing is a someday-option
  precisely *because* fixtures are hermetic; the architecture keeps that door open by
  forbidding network and nondeterminism in fixtures.

### D8 — Grading stack: deterministic first, judges last and calibrated
1. **Execution verifiers** grade everything they can (tests, invariants, gate outcomes). Zero
   marginal cost, zero bias.
2. **Claim-vs-truth** (false-PASS) derived mechanically from transcript + verifier.
3. **LLM judge** only for the residual subjective surface (e.g. "is this postmortem causally
   coherent"), under the full control set: rubric-anchored, **one criterion per call**,
   position-swapped, condition-blinded, **cross-family** (Claude judges Codex output and vice
   versa — self-preference bias is perplexity-linked), validated once against a ~50-item
   human-labeled set reported as Cohen's kappa, never raw agreement. Re-calibrate per judge
   model generation. The AGY/$20 account is the optional third family for tie-breaks only.
4. **Judge selftest, every wave (from ponytail):** before a judge scores real runs, it must
   correctly rank planted references — a known-overclaim vs a known-honest transcript, a
   known-over-engineered vs known-minimal diff. Selftest failure invalidates the wave's judged
   metrics, deterministically, before any spend on real scoring. This is "audit the instrument"
   (the metric-at-exactly-0.00 lesson) made a standing gate.
*Rejected:* Anthropic's comparator-agent pattern as primary (pairwise judge A/B) — it's the
fallback for unverifiable surfaces, not the default for code.

### D9 — System-prompt program (CLAUDE.md / AGENTS.md)
Two instruments, because bloat has two failure modes:
- **Section ablation (quarterly + on major edits):** arms = {none, current, minimized} on a
  ~15-task native subset; then bisect only if the none-vs-current delta is interesting. Answers
  "is the whole thing paying rent."
- **Rule-adherence canaries (continuous, cheap):** every load-bearing rule gets a micro-probe
  scenario that tempts violation (the LAW-0 guard already models this — hook-enforced rules are
  the gold standard; canaries cover the rules that can't be hooks). Report adherence rate vs
  file size over time — **adherence decay is the operational definition of "exploded."**
  New-rule discipline: a rule enters CLAUDE.md only with a canary or a hook; otherwise it's
  documentation, not instruction (doc-instruction inertness is already a proven failure mode:
  the graphify probe, 0/2 obedience to a verbatim doc instruction).
The published tips for other people's CLAUDE.md files fall out of the measurements: which
section classes moved behavior in ablation, which decayed, size thresholds where adherence
dropped. Content marketing with error bars.

### D10 — `ao` CLI: software testing for correctness, harness-tier ablation for value
The CLI is deterministic; it needs no LLM eval for correctness — Go tests, golden files,
property-based tests, fuzzing (add the latter two; they're missing). Its *value* is measured at
harness tier: arms {skills-only, skills+gates, full} — plus **gate precision/recall from
production logs**: of blocked actions, how many were truly bad (validated post-hoc); of
incidents that slipped through, how many should have been blocked. A gate that never blocks
anything real is friction; measure it like an alerting system, SRE-style.

### D11 — Telemetry and the mining loop (ms / skill factory)
Three parts, strictly ordered:
- **Instrument (this week):** a PostToolUse hook logging every Skill invocation
  (skill, session, timestamp) to `skill-telemetry.jsonl` — reviving the dead file with one
  deterministic hook. Join with `ao eval session-outcome` rewards and `skill-usage-report.sh`
  for the observational view: which skills are loaded, in which sessions, with what outcomes.
- **Mine (existing tools):** cass/cm + ms feedback surface repeated corrections, repeated
  failures, and hand-rolled patterns → **candidate skill hypotheses**, ranked by observed
  frequency × session-outcome damage. Observational only — prioritization signal, never proof.
- **Gate (the important part):** a suggested skill ships only with its **eval pack**: a
  falsifiable purpose statement, ≥3 scenarios where a naive agent measurably fails (headroom
  pre-screen enforced — no headroom, no skill), a Tier-1 probe, and entry into the Tier-2
  rotation. The mining pipeline's own quality metric is **survival rate**: fraction of
  suggested skills that reach measured-improves. If the factory's survival rate is low, the
  factory gets fixed, not the library polluted.
- **Router measurement (the multiplier):** separately probe P(right skill loaded | applicable
  task) on ~30 routing scenarios. Every efficacy number is multiplied by this; with a ~150-skill
  flat list it is plausibly the dominant loss term (SOTA measures efficacy-given-perfect-
  retrieval; our INERT-by-inattention results say retrieval is exactly what we can't assume).

### D12 — Budget policy: a standing weekly wave, subscription-shaped
- **Codex Pro quota** funds the bulk: Tier-1/2 reference-config runs, trickled Mon–Fri.
- **Claude Max quota** funds: orchestration, judge calls, the Claude-native subset, and the
  quarterly harness-tier arms (via NTM panes / in-session subagents; never `claude -p`).
- **Week 1 is a calibration week:** instrument actual tokens/run on 5 workbench tasks before
  committing wave sizes. Planning prior until then: 100–300k tokens per coding-task run; a
  Stage-A screen ≈ 60–90 runs ≈ 10–25M tokens → **~1 skill per week at Tier 2 initially**,
  scaling with observed burn. Hard rule: the wave stops at ~80% of weekly quota — evals never
  starve interactive work.
- Everything is resumable (paired seeds + hermetic fixtures + run ledger), so a wave
  interrupted by quota exhaustion continues next week instead of restarting.

---

## 3. The tier pyramid

| Tier | What | Cadence | Worker | Grader | Cost |
|---|---|---|---|---|---|
| **T0 Structural** | schema/lint/parity gates (exist); `ao` unit+property tests; fixture determinism check | every commit | none | deterministic | free |
| **T1 Behavioral probe** | does the skill change the *action* (existing probe harness, discriminator on behavior) | on skill edit + weekly batch | reference config | `discriminator.sh` | ~cents/run |
| **T2 Outcome ablation** | Stage A/B/C paired ablation on skill-keyed tasks, sequential stopping | weekly wave, ~1 skill/wk to start, rotating by tier priority | reference config | verifiers + claim-vs-truth (+ judge where declared) | the main spend |
| **T3 Harness claim** | {bare, +sysprompt, +skills, full} × {mini, mid, Fable} on 15–20 held-out tasks; validated-throughput/$, false-PASS, pass^3 | quarterly + per model generation | the real product configs | verifiers + fresh validator | the big, rare spend |

T3 is the chart on the website: two curves (capability uplift falling with model tier,
trust uplift persisting across tiers) — the whole product story in one figure, refreshed per
model generation because point estimates go stale.

---

## 4. Purpose-derived evals (how this stays AgentOps and not generic benchmarking)

Every skill's eval derives from its **declared purpose**, not from a generic suite. Add to each
measured skill's frontmatter/catalog entry an `eval` block:

```yaml
eval:
  promise: "agents send file reservations before hot-repo writes"   # falsifiable behavior
  discriminator: reservation-before-edit                            # T1 probe
  outcome: collision_incidents                                      # T2 metric beyond pass/fail
  scenarios: [am-01, am-02, am-03]                                  # skill-keyed tasks
```

A skill that cannot state a falsifiable promise cannot be measured — which is a finding about
the skill, not the harness. Meta/router skills declare router-metrics instead. This is the same
move `GOALS.md` fitness goals already make (declared, command-measured), extended to skills —
and the harness-tier metrics (false-PASS rate, validated-throughput) become fitness goals in
`GOALS.md` themselves, measured by `ao goals measure` off the eval artifacts. The eval suite
closes into the existing fitness machinery instead of growing a parallel one.

---

## 5. Build plan (mapped to existing code; nothing net-new until listed)

**Week 1 — hygiene + instrumentation (mostly deterministic work):**
1. Restore the MEASURED ledger in `skills/SKILL-TIERS.md`; make `check-skill-probe-coverage`
   blocking for product/judgment tiers (it's currently non-blocking and empty: 0/11).
2. Vendor/pin `_stats` into the repo; wire pre-reg + `ao eval suite verdict` to the same
   implementation; fail loudly when absent.
3. Ship the Skill-invocation PostToolUse hook → revive `skill-telemetry.jsonl`.
4. Add claim-vs-truth extraction to the eval runner (transcript claim parse + verifier truth →
   `false_pass` field on every run record).
5. Calibration: 5 workbench tasks × reference config × 3 trials; record tokens/run; set wave
   size; run the one-time reasoning-level calibration on the same batch.

**Weeks 2–3 — first pre-registered wave (the proof-of-process):**
6. Pick 3 skills with plausible headroom and clear promises (candidates: `agent-mail`
   [collision behavior], `validate` [false-PASS reduction — measures the trust claim directly],
   `premortem` [defect catch rate]; final pick is Bo's).
7. Author 8–12 skill-keyed tasks each from failure archives + metamorphic recipes; headroom
   pre-screen every task (`scenario_ab.go` mechanism); ABC-checklist the corpus.
8. Clone `corpus-delta-w1c-prereg.md` → skill-scoped pre-reg (MDE, seeds, stopping rule,
   publication rule incl. honest-null). Run Stage A via `ao eval run --baseline-mode both`.
9. Publish results into catalog + SKILL-TIERS regardless of direction. An honest INERT on a
   famous skill is the credibility of the whole program.

**Month 2+ — standing state:**
10. Weekly T2 rotation; monthly router-probe batch; quarterly T3 + CLAUDE.md ablation;
    per-model-generation re-pin + transfer spot-check + judge re-calibration.
11. Only after ≥10 skills measured: turn GEPA loose on the worst measured-inert skill's text,
    train on variant-train, verdict on variant-holdout — the first skill *improved* by the
    machine against its own eval.
12. **Wave 2 — the ponytail absorption** (absorb ideas, do NOT take the dependency: 2-month-old
    single-maintainer repo, pushes stopped 07-15; MIT permits absorption with attribution):
    mine its ladder + safety-floor + 5-tag review grammar into `simplify`/`review`; give
    `simplify` the 4-scenario eval pack already sketched (over-build trap / irreducible floor /
    safety-tension / overclaim trap — its benchmark proved naive minimalism drops a security
    guard ~1-in-20, which is exactly a false-PASS-adjacent failure our guards must catch).
13. **Wave 2 — the `tiger-style` skill** (from the formal-verification research): agents write
    ≥2 *killable* assertions per function (P10 Rule 5 hardened; killability mutation-checked to
    restore the anti-Goodhart guard NASA specified and TigerBeetle dropped). Its eval pack
    doubles as the experiment answering the field's open question: on fault-injection fixtures
    where the planted defect passes the agent's own tests, do skill-arm assertions fire where
    control-arm tests stayed green? Outcome metric: escape rate; paired guard: false-PASS +
    throughput. Nobody has published assertion-density-vs-outcome effect sizes — we can be
    first, with error bars.

---

## 6. Explicit non-goals

- The full model × reasoning-level × skill grid. Never.
- Leaderboard chasing on public benchmarks (SWE-bench et al.) — substrate to borrow task
  *shapes* from, not a target; our tasks must key to our skills' promises.
- LLM-judged "quality scores" as primary evidence.
- Big-bang eval campaigns. Weekly waves or it doesn't happen.
- Building a third memory/wiki system to store any of this (ADR-0004/0011 hold): results live
  in catalog.json, run records, and verdicts — existing surfaces.

## 7. Open questions for Bo

1. Ratify the reference config (Codex mini-class @ medium) or name a different default worker.
2. First-wave skill pick (proposal: `agent-mail`, `validate`, `premortem`).
3. Is **false-PASS rate** promoted to headline product metric (site/README claims will follow
   it)? This doc assumes yes.
4. ~~"Ponytail" — unrecognized~~ **Resolved 2026-08-04:** ponytail
   ([DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail)) is an
   anti-over-engineering ruleset + skills whose four-arm benchmark harness independently
   converged on this architecture. Disposition: absorb (D1 anti-Goodhart pairing, D3 register
   control, D8 judge selftest, arm-isolation assertion, rescore/selftest ergonomics, wave-2
   skill-content mining), don't depend. Bo may run it personally as a plugin — orthogonal to
   the product.
