# AgentOps Effectiveness — Evidence Audit

> **Question under audit:** *Does AgentOps measurably improve Claude/Codex
> coding-agent behavior, or does it mostly add process/control without improving
> task outcomes?*
>
> **Stance:** "AgentOps improves agents" is an **unproven hypothesis**. This
> document is a non-marketing evidence audit, not a pitch. It records what is
> proven, what is not, where the eval substrate is red or stale, and the exact
> path to a defensible answer.
>
> **Audited:** 2026-06-16. **Verified against:** real code + fresh local runs
> (commands and outputs recorded inline). **Bead:** `age-d16-self-hosting-route-nkr.2`
> (RULER — corpus-delta PILOT). **Re-grounds:** the discovery prompt that asked
> to *build* an eval program — the re-baseline finding is that **most of that
> substrate already exists**; the gap is running it and consolidating the picture.

---

## TL;DR verdict

**Current evidence is insufficient to claim AgentOps improves live coding-agent
behavior — and the eval substrate to settle it largely EXISTS and is GREEN, but
has never been driven by a live agent.**

- The two existing *measured* signals point **away** from a naive "yes":
  - Skill/hook A/B on the workbench agent tasks: **aggregate delta = 0** (12 cases).
  - The earlier hook/context-injection A/B that motivated the hookless model:
    **delta = 0** (hook-layer, not a corpus A/B).
  - The first *measured* corpus A/B in adjacent work (knowledge-field S1) was
    **negative (−0.37)**: a thin/flat gold pull HURT.
- The one **positive** signal is a **deterministic fixture canary**
  (`context-packet-ab-wave0`, delta = +0.4667). It proves curated context *can*
  flip a fixture outcome. It does **not** prove a live Claude/Codex task uplift.
- The corpus-delta measurement apparatus (held tasks, sandboxed on/off runner,
  locked pre-registration, scorecard schema, plumbing tests) is **built and
  passing**. It has only ever been run with a **stub** runner (plumbing proof),
  never a live agent → the corpus delta **C is still `pending`**.
- **No promoted baselines exist** (`.agents/evals/baselines/` is absent/empty).
- **PROOF tier is blocked** on two un-tracked prerequisites (retrieval+injection
  wiring + cold-arm ceiling pre-screen). Until they land, every corpus-delta run
  is **PILOT-only** by the pre-registration's own rule.

**Recommendation: run the PILOT (advisory, no moat claim), and treat the two
PROOF prerequisites as the blocking gap before any moat claim or any
Olympus-Pi / SDK custom-agent experiment.**

---

## 1. What evidence exists today?

### 1.1 Measured signals (the only things that count as evidence)

| Signal | Source | Result | Axis | What tier |
|---|---|---|---|---|
| Skill/hook A/B, workbench agent tasks | `evals/workbench/results/2026-05-06-yjzp9-counterstat.json` | `skill_on=1`, `skill_off=1`, **aggregate_delta = 0** across 12 cases (all pass both legs) | skill/hook on-off | deterministic, ceiling-saturated |
| Hook/context-injection A/B | AgentOps 3.0 doctrine (`docs/3.0.md`) | **delta = 0** → motivated the hookless, pull-context model | hook-layer | historical |
| Context packet canary | `evals/agentops-core/context-packet-ab-wave0.json` | `context_off=fail`, `context_on=pass`, **delta = +0.4667** | context present-absent | **deterministic fixture** (`visibility: public_canary`) |
| First measured corpus A/B (adjacent) | knowledge-field S1 (memory: `knowledge-field-ab-first-delta`) | **−0.37** (gold pull hurt; honest FAIL) | corpus gold on-off | measured, negative |

### 1.2 Built apparatus (NOT evidence by itself — the instruments)

All of the following **already exist** and are **green** as of this audit:

- **Locked pre-registration** — `evals/workbench/corpus-delta-w1c-prereg.md`.
  Fixes (before compute): verdict categories (positive / null / inconclusive /
  ceiling-excluded / retrieval-failed / degraded), the statistic
  (common-random-number paired seeds; bootstrap-over-tasks 10k resamples, 95% CI;
  Wilson per-task), **MDE = 0.15 absolute aggregate delta**, the publication
  decision rule, the frozen experiment definition, and the PROOF/PILOT tier rule.
- **Corpus on/off runner** — `scripts/corpus-delta-harness.sh`. Per-arm sandbox
  (isolated `HOME` + workspace + `.agents`); `context_off` = empty corpus with
  always-loaded surfaces stripped; `context_on` = organic `.agents` + repo
  `CLAUDE.md` + `.claude/rules` + user-global + auto-memory copied in; K seeds;
  emits a `ContextDeltaScorecard`; default runner `codex` only; injectable
  `CORPUS_DELTA_RUNNER` for stub testing.
- **Per-task agent runner** — `scripts/eval-agent-harness.sh`. `--task`,
  `--agent codex` (`codex exec -C <ws> -s workspace-write`), `--compare`
  (skill/hook A/B), `--hooks-disabled`, `--runs`, `--retry`, `--dry-run`,
  workspace-isolated with an EXIT-trap cleanup.
- **Held task set** — 10 `evals/workbench/tasks/cd-*` tasks (CI, tracker,
  coordination, runtime, generated-artifact surfaces), each with
  `prompt.md` / `setup.sh` / `score.sh` / `golden-solution.sh`, selected from
  recurring AgentOps operational failures the corpus documents (NOT because the
  exact answer is in the corpus). Documented in
  `evals/workbench/tasks/corpus-delta-held-set.md`.
- **Scorecard schemas** — `ContextDeltaScorecard` (in the harness) and
  `DeltaScorecard` (in `docs/contracts/eval-baseline-ab.md`).
- **Deterministic smoke tests** — `tests/scripts/corpus-delta-harness.bats`
  (10 tests) and `tests/scripts/corpus-delta-tasks.bats` (4 tests).
- **Contracts** — `docs/contracts/eval-baseline-ab.md` (skill/hook axis, with
  the explicit boundary that it is NOT the context axis),
  `docs/contracts/context-usefulness-eval.md` (context axis),
  `docs/contracts/eval-environment.md`, `docs/contracts/eval-verdict-pipeline.md`.

### 1.3 Fresh verification performed in this audit (2026-06-16)

```text
# Plumbing smoke (stub runner, NO model): off=fail, on=pass, delta=1
CORPUS_DELTA_RUNNER=<stub> scripts/corpus-delta-harness.sh --task cd-beads-1 --seeds 3
  → context_off.passes=0/3, context_on.passes=3/3, aggregate_delta=1

# cd-* grader discrimination (all 10): empty FAILS, golden PASSES
  cd-beads-1 … cd-ci-4 → empty_pass=False golden_pass=True   (all 10)

# bats
  tests/scripts/corpus-delta-harness.bats → 10/10 ok
  tests/scripts/corpus-delta-tasks.bats   →  4/4 ok

# per-task harness dry-run (no model) → valid scorecard fragment, exit 0, no dirtying
  scripts/eval-agent-harness.sh --task cd-beads-1 --agent codex --dry-run
  → {"score":0,"total":1,"pass":false,"skipped":true,"reason":"dry-run mode"}

# codex available: codex-cli 0.139.0 at ~/.local/bin/codex
```

---

## 2. What does the existing evidence PROVE?

1. **The corpus-delta harness PLUMBING is correct.** With a stub runner that
   passes iff the corpus is present, `context_on` beats `context_off` by exactly
   the constructed amount. Sandbox isolation, seed loops, scorecard math, and the
   always-loaded-surface stripping all work as designed.
2. **The held graders discriminate.** Every `cd-*` golden solution passes its
   deterministic grader and every empty workspace fails it. A run's pass/fail is
   a real signal about the produced artifact, not a coin flip.
3. **Curated context can flip a deterministic fixture** (`context-packet-ab-wave0`,
   +0.4667). The mechanism — context presence changing an outcome — is real *in a
   fixture*.
4. **Skill/hook loading, as measured, added zero outcome delta** on the workbench
   agent tasks (and the hook-injection A/B was zero). This is honest negative/null
   evidence for the *hook* mechanism, and is the documented reason AgentOps went
   hookless.

## 3. What does it NOT prove?

1. **It does NOT prove a live Claude/Codex task uplift from the corpus.** Every
   corpus-delta run to date used a **stub** runner; `evidence_kind` on those
   scorecards is literally `harness_plumbing`. The harness's own header warns:
   *"a run with a STUB runner proves the harness PLUMBING only … Do not cite a
   stub/plumbing run as proof of the moat."*
2. **The +0.4667 canary is not behavioral evidence.** It is a `public_canary`
   deterministic fixture (`check-helpful-context.sh`) — it guards the contract
   that context *can* matter; it does not measure an agent.
3. **The 0-delta skill/hook result is ceiling-saturated.** All 12 cases pass both
   legs (`skill_off=1`), so there was **no headroom** to detect improvement —
   this is `ceiling-excluded`-shaped, not a clean null about value-add.
4. **"PILOT-present" ≠ "retrieved+injected."** The runner makes the corpus
   *present in the sandbox* (the agent must find it). It does **not** retrieve and
   inject the known-relevant decisions into the prompt. Per the pre-registration,
   that is exactly why any run today is **PILOT-only**.
5. **No promoted baseline exists** to anchor regression or "is this better than
   last week" claims (`.agents/evals/baselines/` absent).

## 4. Where are current evals RED or STALE?

| Surface | State | Read |
|---|---|---|
| `cd-*` corpus-delta tasks + harness + bats | **GREEN** (verified this audit) | the PILOT substrate is ready to run live |
| `make -C evals/workbench verify-golden` (go/py/ops `workbench-agent-v1` set) | **RED** — 11/12 golden legs fail (`FAIL (setup)` for go/ops, `FAIL (golden)` for py) | **STALE/environmental** — these are the OLD skill/hook A/B tasks (the delta=0 counterstat set), not the corpus-delta set; likely missing Go/Python toolchain in the bare workspace. Do **not** treat as a corpus-delta blocker; do flag for repair or retirement. |
| `scripts/eval-agentops.sh --fast --advisory` deterministic canaries | **PARTIAL** — last partial run: 18 pass / 9 fail of 27 before a hang | These are the **contract/canary** suites (curation-feedback-governance, distribution-install-update, finding-prevention-governance, goals-management-lifecycle, headless-runtime-skills, hidden-cli-internals, knowledge-flywheel-ingestion, operator-workflow-skills, optimization-dependency-governance). They guard repo contracts, **not** agent uplift — a red canary is a contract drift, not moat evidence either way. The hang itself is a substrate bug worth a bead. |
| `.agents/evals/baselines/` | **EMPTY/ABSENT** | no promoted baseline anywhere |
| Corpus delta **C** (the moat number) | **PENDING** | never measured with a live agent (n=1 live smoke this audit had both arms at the floor — see §6b) |
| `corpus-delta-harness.sh` scorecard label | **BUG** | hard-codes `evidence_kind: "harness_plumbing"` even on live runs — a live PILOT mislabels itself as plumbing (§6b) |

## 5. What would be REQUIRED to claim AgentOps improves Codex/Claude behavior?

Per the **locked** `corpus-delta-w1c-prereg.md`, a "moat positive" claim requires
a **PROOF-tier** run (tier is computed from the commit, not chosen). PROOF needs
**both** prerequisites green at HEAD (both currently UN-TRACKED post ledger-wipe):

1. **On-arm retrieval + attribution wiring** (was `ag-98g1a`). The `context_on`
   arm must *retrieve and inject* the task's known-relevant decision(s) AND log
   an attribution trail. Each `cd-*` task must also carry **frozen
   known-relevant decision id(s)** so the **`retrieval-failed`** exclusion can be
   computed by exact mechanical id-comparison (no human "looks mismatched").
   Today the `cd-*` tasks ship no such labels → the moat-saving exclusion cannot
   fire → no PROOF.
2. **Cold-arm ceiling pre-screen** (was `ag-2i5jg`). Tasks where `context_off`
   already passes at 1.0 across seeds have **no headroom** and must be
   mechanically `ceiling-excluded` before the aggregate — otherwise the 0-delta
   trap (cf. the skill/hook counterstat) repeats.

Then the publication rule binds: claim **moat positive** ONLY IF the aggregate
delta CI lower bound > 0 AND ceiling+retrieval exclusions ≤ N/3 AND ≥1 task is
independently `positive`; claim **honest null** ONLY IF off-arm had headroom AND
retrieval was verified present AND the CI straddles 0; otherwise **inconclusive**.
A fresh non-author reviewer renders the residual positive/null/inconclusive call.

**One genuine measurement gap to add (small):** the discovery prompt's primary
metric — **false-done rate** (agent claims done / emits a completion artifact but
deterministic acceptance fails, evidence is missing, or the gate would reject) —
is **not** an explicit field today. The harness records pass/fail; the prereg has
`degraded` / `retrieval-failed` / `ceiling-excluded`, but no `false_done` boolean.
This is a scorecard-field addition to the existing runner, not a new runner. See
follow-up below.

---

## 6. Re-baseline: requested deliverables → what already exists

The discovery prompt asked to *create* an eval program. The honest finding is
that the substrate is mostly built. Mapping the request to reality (so future
agents do not rebuild it):

| Requested | Status | Existing artifact |
|---|---|---|
| Evidence report (`agentops-effectiveness-evidence.md`) | **NEW (this doc)** | `docs/evals/agentops-effectiveness-evidence.md` |
| Live A/B contract preventing overclaim | **EXISTS** | `corpus-delta-w1c-prereg.md` (locked) + `eval-baseline-ab.md` + `context-usefulness-eval.md` |
| Repeatable bare-Codex vs Codex+AgentOps runner | **EXISTS** | `corpus-delta-harness.sh` (context axis) + `eval-agent-harness.sh` (`--compare`, skill/hook axis) |
| Scorecard schema | **EXISTS** | `ContextDeltaScorecard` (harness) + `DeltaScorecard` (contract) |
| Deterministic smoke test (no model) | **EXISTS** | `corpus-delta-harness.bats` (10) + `corpus-delta-tasks.bats` (4) |
| `--arm bare-codex` / `--arm agentops-codex` | **EQUIVALENT** | the two arms = `context_off` / `context_on` (corpus axis) or `--hooks-disabled` legs (skill axis). A literal `--arm` alias is cosmetic. |
| Future Arm C (Pi / SDK) placeholder | **NOT BUILT (correct)** | out of scope this pass; gated behind a positive PILOT/PROOF |

**Do NOT rebuild the above.** The remaining work is: run the PILOT, land the two
PROOF prerequisites, and (optionally) add the `false_done` field.

---

## 6b. Live PILOT smoke run (2026-06-16, n=1 — ADVISORY ONLY)

A tiny live probe was run this audit to de-risk the live path:

```text
scripts/corpus-delta-harness.sh --task cd-door9-1 --seeds 1 --agent codex
  → context_off: 0/1 (fail),  context_on: 0/1 (fail),  aggregate_delta = 0
  → codex-cli 0.139.0, codex exec -C <ws> -s workspace-write, timeout 120s, exit 0
```

**Reading (honest, advisory, n=1):** the live path *works end-to-end* (sandboxes
built, both arms invoked codex, scorecard emitted). But **bare codex failed the
task in BOTH arms** — so this is a `needs-more` PILOT point, not a delta signal:
with no passing arm there is no headroom to observe a corpus effect. Likely
causes to rule out before the full PILOT: (a) the default **120s timeout** is too
short for a write-a-guard-script task — raise `--timeout` via the inner harness or
pick a simpler task; (b) codex produced output the deterministic grader rejected
(a candidate **false-done** — see §4 gap); (c) the `cd-door9-1` grader is
stricter than the prompt implies. **Do not read delta=0 here as "corpus doesn't
help" — read it as "the arm scores are floor-bound, fix the run first."**

**Bug found during the live run (worth a follow-up bead):** the scorecard hard-codes
`"evidence_kind": "harness_plumbing"` in `corpus-delta-harness.sh` regardless of
whether a **live** agent or a **stub** ran. A live PILOT result therefore
*mislabels itself as plumbing*. The label must reflect the actual runner
(`live_agent` vs `harness_plumbing`) or the honesty guard the harness header
promises is defeated at the data layer.

## 6c. Full codex PILOT — RUN, with the three substrate fixes live (2026-06-16)

The n=1 smoke (§6b) was followed by the full prereg PILOT once the substrate was made
honest. Three fixes landed first — codex never actually launched/authenticated before
these: **`age-o9x`** (`--skip-git-repo-check`, the harness never launched the agent),
**`age-94f`** (carry `~/.codex` auth into the sandbox — codex 401'd in both arms),
**`age-t8n`** (propagate `degraded`/`elapsed`/`evidence_kind` so a broken run can't
masquerade as a null). Then: 5 frozen `cd-*` tasks × K=3 × 2 arms, sequential, `live_agent`.

**Verdict: INCONCLUSIVE — 5/5 unmeasurable (4 ceiling + 1 degraded).**

| task | off | on | disposition |
|---|---|---|---|
| cd-agents-1 | 3/3 | 3/3 | ceiling (off==1.0, no headroom) |
| cd-am-1 | 3/3 | 3/3 | ceiling |
| cd-beads-1 | 2/3 | 2/3 | **degraded** (timeouts; `delta_valid=false` — `age-t8n` caught it) |
| cd-bv-1 | 3/3 | 2/3 | ceiling (off==1.0; on dipped = noise) |
| cd-ci-1 | 3/3 | 3/3 | ceiling |

Exclusions > N/3 → **no claim** per the prereg. *Not* a moat positive, *not* a refutation.

**Root cause — and a two-session convergence.** Bare codex passes most `cd-*` tasks
*cold* because the **task prompts are self-contained** — they state the contract (e.g.
cd-ci-1: "exit 1 if a continue-on-error job appears in `summary.needs`"), so the corpus
is redundant. This is the SAME root failure the scenario-ab ruler hit via a different
mechanism (`age-9a9`: the control reads the corpus off disk because the arm runs in the
repo cwd). **Two independent rulers, one conclusion: the control already has the answer
→ the ruler is invalid → the moat is UNPROVEN, not disproven; the blocker is ruler
validity.** Consolidation + the corpus-free-sandbox isolation fix:
[`age-9a9-corpus-free-sandbox-isolation.md`](age-9a9-corpus-free-sandbox-isolation.md).
The cd-* line is retired (`age-rpm` closed, folded into scenario-ab); `age-t8n`'s
honesty machinery is harvested. Run artifacts: `.agents/evals/runs/pilot4-seq-*/`.

## 7. The exact next command (tiny live Codex PILOT)

PILOT-only (no moat claim), single task, single seed — the smallest live probe:

```bash
cd ~/dev/agentops
scripts/corpus-delta-harness.sh --task cd-door9-1 --seeds 1 --agent codex \
  --out .agents/evals/runs/pilot-live-smoke/cd-door9-1-live.json
```

Full PILOT (the frozen 5-task subset × K=3 = 30 on + 30 off ≤ 60 codex calls,
per the prereg's PILOT tier):

```bash
cd ~/dev/agentops
for t in $(ls evals/workbench/tasks | grep '^cd-' | sort | head -5); do
  scripts/corpus-delta-harness.sh --task "$t" --seeds 3 --agent codex \
    --out ".agents/evals/runs/$(date -u +%Y%m%dT%H%M%SZ)/$t.json"
done
# Then: label PILOT, assign per-task categories by the prereg, NO moat claim.
```

A live single-pass result is **advisory** until repeated-run variance exists
(`eval-baseline-ab.md` §SIL-vs-HIL: do not ship a single-shot live score as
authoritative). Record the result, feed it to the yield gauge's **C** only as
`positive-signal` / `needs-more` / `degraded` — never as a moat verdict.

---

## 7b. Dogfooding finding (2026-06-16) — an AgentOps-native agent hand-rolled what the skills already encode

Asked to run the corpus-delta eval with **Claude + ATM**, the operating agent (me)
spiked local tmux Claude panes from scratch — walking onboarding/trust/bypass gates,
building completion detection by hand — instead of routing through the surfaces that
already own this:

- **`ao agent bundle --runtime managed|codex-ntm`** — the sanctioned multi-runtime
  agent definition (Claude via **Managed Agents**, a Law-0-safe cloud path that is NOT
  `claude -p`; or Codex via NTM), carrying the AgentOps skills + tools, refusing to
  inline holdout content.
- **`eval-outcomes`** — already a **runtime comparison** transport with a designed
  **Claude path (Managed Agents)** and **Codex path (local Inspect/Qwen)**, grading
  both against the *same* compiled rubric → byte-identical verdict.
- **`using-atm`** — pane dispatch for the loop.

This is a **direct, failing dogfooding datapoint** for this very audit's question: an
AgentOps-native agent, mid-investigation into whether AgentOps changes behavior, did
**not** reach for the eval skills and reinvented the orchestration. Process existed;
it did not change the agent's behavior. That is exactly the "adds control without
improving outcomes" failure mode — observed live, on the operator, not hypothesized.

**Corrected skill-native path for a Claude corpus-delta arm:**
`ao agent bundle --runtime managed --sandbox self-hosted` (corpus present vs. stripped
= the on/off arms, controlled by the bundle, not a local HOME) → run the eval task →
`ao eval outcomes compile|ingest` to grade → one verdict. The local-tmux/`corpus-delta-harness`
Claude arm (`age-yms`) is a fallback, not the intended path.

**Open dependency:** the Managed-Agents Claude path targets the **bushido self-hosted
sandbox**; per the 2026-06-05 fleet teardown the Windows-node serves were removed, so
that sandbox's liveness must be verified before this path runs. If it's down, that is a
*second* dogfooding gap (the designed Claude path is not operational) — distinct from
the skill-bypass finding above.

## 8. Final answer

**Current evidence is insufficient to claim AgentOps improves Codex/Claude
behavior — and the blocking gap is NOT missing eval machinery (that is built and
green); it is (a) the PILOT has never been run with a live agent, so the corpus
delta C is unmeasured, and (b) PROOF is blocked on the retrieval+injection path +
cold-arm ceiling pre-screen, neither currently tracked.**

- **Proven now:** the harness plumbing, the grader discrimination, and that
  curated context can flip a deterministic fixture.
- **Unproven now:** any live coding-agent uplift from the AgentOps corpus. The
  only measured deltas on real comparisons are **0** (skill/hook, ceiling-bound)
  and **−0.37** (adjacent corpus gold). The candidate moat remains unmeasured.
- **Honest "inconclusive" is the correct standing verdict** until a live PILOT
  runs and the two PROOF prerequisites land.
- **Do not build a Pi/Olympus/SDK custom agent** until a PILOT shows a positive
  signal — the ruler must read positive before the positioning.

**Next command:** the tiny live PILOT in §7.
