---
id: research-2026-04-24-agentops-evaluation-environment
type: research
date: 2026-04-24
---

# Research: AgentOps Evaluation Environment

**Backend:** inline
**Scope:** Existing validation, runtime smoke, RPI, scenario, skill, hook, CLI, and baseline machinery relevant to evaluating AgentOps skills, hooks, and the `ao` CLI across Claude and Codex.

## Summary

AgentOps already has many proof fragments: structural runtime smoke, Codex parity guards, headless inventory checks, retrieval benchmarks, RPI phase evaluator artifacts, holdout scenario contracts, and baseline comparison precedent in the security suite. The missing piece is a unified eval substrate: checked-in eval suites, repeatable run records, baseline promotion/comparison, live runtime/model matrices, and scorecards that answer whether a skill/hook/CLI change improved or degraded agent outcomes.

## Key Files

| File | Purpose |
|------|---------|
| `.github/workflows/validate.yml` | Runs Codex runtime, RPI contract, lifecycle, headless runtime, and retrieval-quality gates. |
| `scripts/validate-headless-runtime-skills.sh` | Opens headless Claude/Codex sessions and compares visible skill inventory against repo definitions. |
| `scripts/validate-codex-rpi-contract.sh` | Enforces Codex-specific RPI orchestration and no-beads handoff contracts. |
| `scripts/validate-codex-lifecycle-guards.sh` | Enforces `ao codex ensure-start` / `ensure-stop` lifecycle guard wording across Codex skills. |
| `scripts/audit-codex-parity.py` | Detects Claude-era primitives and stale runtime language in Codex artifacts. |
| `skills/validation/references/step-1.8-behavioral-validation.md` | Defines holdout scenario satisfaction scoring during validation. |
| `skills/scenario/SKILL.md` | Documents scenario authoring, isolation, lifecycle, and validation intent. |
| `cli/cmd/ao/scenario*.go` | Implements `ao scenario init/list/validate`; no `add` command exists. |
| `schemas/scenario.v1.schema.json` | Defines scenario fields, acceptance vectors, thresholds, and statuses. |
| `hooks/holdout-isolation-gate.sh` and `hooks/hooks.json` | Blocks `.agents/holdout` reads unless `AGENTOPS_HOLDOUT_EVALUATOR=1`. |
| `cli/cmd/ao/retrieval_bench.go` | Implements deterministic retrieval evals with train/holdout splits, P@K, MRR, and section metrics. |
| `scripts/check-retrieval-quality-ratchet.sh` | Warn-then-fail retrieval quality ratchet with a fallback eval manifest. |
| `cli/cmd/ao/rpi_evaluator.go` | Emits per-phase evaluator artifacts with verdicts, evidence paths, transcript-derived reward, and findings. |
| `skills/heal-skill/references/skill-stocktake.md` | Describes a not-yet-integrated AI quality stocktake for skill keep/improve/merge/retire decisions. |
| `skills/security-suite/SKILL.md` | Shows a mature baseline-capture and candidate-vs-baseline comparison pattern. |

## Findings

### Existing Proof Fragments

CI already runs several relevant gates. The Codex runtime job validates runtime sections, generated artifacts, backbone prompts, override coverage, Codex RPI contract, lifecycle guards, and headless runtime skills (`.github/workflows/validate.yml:70`, `.github/workflows/validate.yml:79`, `.github/workflows/validate.yml:94`, `.github/workflows/validate.yml:98`, `.github/workflows/validate.yml:103`). Retrieval quality has a dedicated job that builds `ao`, runs `retrieval-bench --json`, and currently treats low precision as advisory until a stronger baseline is stable (`.github/workflows/validate.yml:482`, `.github/workflows/validate.yml:497`, `.github/workflows/validate.yml:503`).

The headless runtime validator compares expected skill inventories against live Claude and Codex sessions, but it is still Tier I inventory proof rather than Tier E execution proof. The script prints the proof-tier matrix (`scripts/validate-headless-runtime-skills.sh:22`), builds expected inventories from `skills/` and `skills-codex/` (`scripts/validate-headless-runtime-skills.sh:365`), runs Claude with `--no-session-persistence` and a budget cap (`scripts/validate-headless-runtime-skills.sh:440`), installs the Codex plugin into an isolated `CODEX_HOME` (`scripts/validate-headless-runtime-skills.sh:541`), and runs Codex in read-only JSON mode (`scripts/validate-headless-runtime-skills.sh:552`).

Codex-specific RPI behavior already has contract-level validation. `validate-codex-rpi-contract.sh` requires file-backed handoff via `$crank .agents/rpi/execution-packet.json`, standalone `$validation`, and explicit `$skill` chaining defaults (`scripts/validate-codex-rpi-contract.sh:36`, `scripts/validate-codex-rpi-contract.sh:48`, `scripts/validate-codex-rpi-contract.sh:58`). Lifecycle guard validation enforces `ao codex ensure-start` on entry skills and `ao codex ensure-stop` on closeout skills (`scripts/validate-codex-lifecycle-guards.sh:39`, `scripts/validate-codex-lifecycle-guards.sh:67`, `scripts/validate-codex-lifecycle-guards.sh:73`).

RPI itself is already structured for phase isolation and file-backed handoff. The CLI help text describes three fresh-context phases (`cli/cmd/ao/rpi_phased.go:63`), dry-run support (`cli/cmd/ao/rpi_phased.go:82`), runtime selection (`cli/cmd/ao/rpi_phased.go:104`), and mixed-model execution (`cli/cmd/ao/rpi_phased.go:108`). RPI post-processing writes execution packets, extracts pre-mortem/vibe verdicts, and emits evaluator artifacts (`cli/cmd/ao/rpi_phased_gates.go:46`, `cli/cmd/ao/rpi_phased_gates.go:161`, `cli/cmd/ao/rpi_phased_gates.go:269`). Per-phase evaluator artifacts include run id, phase, verdict, findings, evidence, and transcript outcome fields (`cli/cmd/ao/rpi_evaluator.go:22`, `cli/cmd/ao/rpi_evaluator.go:36`, `cli/cmd/ao/rpi_evaluator.go:87`).

Retrieval has the closest existing eval pattern. `retrieval-bench` has explicit query cases, expected results, best IDs, train/holdout splits, Precision@K, MRR, and section-aware scoring (`cli/cmd/ao/retrieval_bench.go:26`, `cli/cmd/ao/retrieval_bench.go:36`, `cli/cmd/ao/retrieval_bench.go:61`). It documents determinism for fixed corpora (`cli/cmd/ao/retrieval_bench.go:595`) and defaults include holdout queries (`cli/cmd/ao/retrieval_bench.go:584`). A local run on this worktree returned 6/6 passing queries with `avg_precision_at_k=1.0` and `avg_mrr=1.0`.

The security suite provides a useful baseline pattern: capture a known-good baseline, run a candidate, compare contract drift, enforce policy, and write machine-consumable outputs (`skills/security-suite/SKILL.md:31`, `skills/security-suite/SKILL.md:49`, `skills/security-suite/SKILL.md:80`, `skills/security-suite/SKILL.md:87`).

### Missing Eval Substrate

There is no single `ao eval` command, checked-in suite layout, result schema, or baseline promotion flow that covers skills, hooks, CLI behavior, and live runtime/model execution. Existing gates answer "does this contract still structurally load?" much more often than "did this change make the agent complete the task better?"

Scenario support is promising but incomplete. The scenario skill says scenarios define measurable acceptance vectors and live in `.agents/holdout` (`skills/scenario/SKILL.md:12`), documents `/scenario add` (`skills/scenario/SKILL.md:23`), and says validation consumes scenarios in STEP 1.8 (`skills/scenario/SKILL.md:98`). The Go CLI only registers `init`, `list`, and `validate` (`cli/cmd/ao/scenario.go:5`, `cli/cmd/ao/scenario_init.go:11`, `cli/cmd/ao/scenario_list.go:14`, `cli/cmd/ao/scenario_validate.go:14`), and `ao scenario --help` confirms no `add` command exists. This is a source-of-truth mismatch that must be fixed before scenario authoring can anchor evals.

Holdout storage is also not durable by default. `.agents/holdout` is ignored by `.gitignore`, so human-authored eval scenarios under `.agents/holdout` will not naturally become a versioned baseline. The existing isolation hook is useful (`hooks/holdout-isolation-gate.sh:8`, `hooks/hooks.json:147`), but an AgentOps eval environment needs a checked-in public canary corpus plus optional private holdouts, not only ignored local state.

Validation STEP 1.8 specifies satisfaction scoring and a scenario-results artifact (`skills/validation/references/step-1.8-behavioral-validation.md:22`, `skills/validation/references/step-1.8-behavioral-validation.md:28`, `skills/validation/references/step-1.8-behavioral-validation.md:37`), but there is no unified runner that executes those scenarios against candidate branches, models, runtimes, and baselines.

Skill pruning/refactor evaluation is described but not productized. The stocktake reference proposes deterministic inventory plus AI evaluation using actionability, scope fit, uniqueness, currency, and trigger clarity, with Keep/Improve/Update/Retire/Merge verdicts (`skills/heal-skill/references/skill-stocktake.md:13`, `skills/heal-skill/references/skill-stocktake.md:37`, `skills/heal-skill/references/skill-stocktake.md:44`). That should become part of the eval environment, not a standalone reference.

### Prior Knowledge Applied

- `.agents/findings/f-2026-04-14-001.md` applies because any `ao eval` CLI work must pair production command changes with command tests; the finding explicitly asks for paired `cli/cmd/ao` tests and pre-push validation (`.agents/findings/f-2026-04-14-001.md:42`).
- `.agents/findings/f-2026-04-14-002.md` applies because eval baselines and closure artifacts must point at durable, committed files rather than ephemeral `.agents` seed paths (`.agents/findings/f-2026-04-14-002.md:42`).
- `.agents/learnings/2026-04-14-scrub-rpi-runtime-from-raw-validation.md` applies because model/runtime evals must scrub host variables such as `AGENTOPS_RPI_RUNTIME` before comparing results (`.agents/learnings/2026-04-14-scrub-rpi-runtime-from-raw-validation.md:16`).
- `.agents/learnings/2026-04-07-v2.35.0-release-postmortem.md` applies because local and remote CI have different failure surfaces; the eval environment needs local, CI, and live/nightly tiers instead of assuming one gate is sufficient (`.agents/learnings/2026-04-07-v2.35.0-release-postmortem.md:16`).

## Quality Validation

Coverage checked: docs/index orientation, product context, CI workflow, runtime validation scripts, Codex parity scripts, RPI CLI/skill contracts, scenario CLI/schema/hook support, retrieval benchmark, flywheel gate, security baseline precedent, test fixtures, and prior findings/learnings.

Depth ratings:

| Area | Depth | Notes |
|------|-------|-------|
| Runtime loading/proof tiers | 3/4 | Inventory proof is clear; live execution scoring remains missing. |
| RPI phase/evaluator contracts | 3/4 | Phase artifacts and evaluator output are clear enough for planning. |
| Scenario/holdout validation | 2/4 | Intent is clear, but command support and durable corpus design are incomplete. |
| Baseline comparison | 3/4 | Retrieval and security suite provide concrete patterns. |
| Skill quality/pruning | 2/4 | Stocktake is specified as a reference but not integrated into executable flows. |

Assumptions challenged:

- "100% reliable" should mean 100% pass rate on a defined critical canary suite over repeated runs, not mathematical proof over all possible agent behavior.
- Public checked-in scenarios are necessary for reproducibility, while private holdouts are necessary to detect overfitting.
- Live model evals should start advisory or nightly because model availability, auth, cost, and nondeterminism make them too unstable for every PR.

## Recommendations

Build a layered `ao eval` subsystem instead of adding more isolated validators. The first version should define a checked-in eval pack format, run deterministic suites locally, support optional live Claude/Codex headless execution, produce scorecards, and compare candidates against a known-good baseline. Use existing retrieval-bench and security-suite patterns for scoring and baseline promotion, and treat RPI as a top-level canary suite with strict phase/artifact checks plus behavioral satisfaction scoring.
