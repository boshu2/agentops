# Goals

**AgentOps is the validation apparatus for stochastic agentic code work** — the membrane, map, evidence record, and learning loop that answer: *is this code right, and is this agent output proven enough to trust farther?* It drives a goal through four explicit umbrellas: Discovery shapes behavior, Crank executes small slices, Validate gives completed work to a fresh context that did not create it, and Learn routes the result into the next experiment. Deterministic tests remain ground truth; one fresh sub-agent is the default independent judge, while mixed-model councils are an optional rigor tier. Validation produces a verdict and proof record. Git delivery remains the repository's policy.

> Canonical 3.0 + navigator architecture: [docs/3.0.md](docs/3.0.md); product framing: [PRODUCT.md](PRODUCT.md); component routing and trim/defer posture: [docs/architecture/component-map.md](docs/architecture/component-map.md). The directives below are measured against them.

## The Destination — what drives this repo

A navigator with no destination is a map you stare at. AgentOps had a *scope* ("a control plane for everything in agentic SDLC"), not a destination — which is exactly why the backlog sprawled to 100+ open epics with nothing to prioritize against. The destination, set 2026-06-14:

> **Autonomous goal → verified done.** Give AgentOps a goal carrying an acceptance contract, and the navigator drives a stochastic agent to a *verified* done — shape intent → slice → build → fresh-context Validate → Learn → recover or continue from evidence — **without a human in each step.**

The first wedge is narrower than "autonomous software engineering": it is
**autonomous code validation**. AgentOps earns broader autonomy by proving code
and agent output repeatedly, then raising only that proven lane on the
[Autonomy Ladder](docs/architecture/autonomy-ladder.md). Orchestration scales
after validation, not before it.

The self-hosting line is this: **AgentOps is done-enough when the navigator drives its OWN next epic to verified-done, end-to-end, unattended** with author≠validator, acceptance-bound evidence, four ordered receipts, and no delivery-policy assumption. That is the finish line — see Directive 16.

The prioritization rule this destination creates — and the reason it *drives* the repo: **an epic is pursued only if it advances a route milestone toward autonomous-goal→verified-done; everything else is deferred, not abandoned.** The routing source for that rule is [docs/architecture/component-map.md](docs/architecture/component-map.md): new work is KEEP only if it advances route-critical ledger/tracker truth, validation/release health, measured corpus proof, the governed factory front door, or runtime proof for the core loop; otherwise it is DEFER/TRIM until the route earns it. That rule is what turns 100+ undifferentiated epics into a route. Every North Star below is a *property* the navigator needs to reach the destination; every Directive is a *paving stone on the route*.

## One self-contained factory, delivery outside the membrane

The destination is reached by AgentOps itself: one self-contained factory with
its own validation membrane, navigator loop, corpus/flywheel, and CDLC. The
trust boundary is between stochastic production and an explicit,
acceptance-bound verdict from fresh context. It is not a merge boundary.

- **AgentOps produces, senses, judges, and learns.** It drives work, emits deterministic evidence (**SENSOR**), obtains an independent verdict through Validate (**JUDGE**), and mines evidence into proposed improvements through Learn (**ASSAY**).
- **Repositories deliver.** Direct push, PR, merge queue, hosted CI, and release policy are adapters outside domain completion. Pawl remains available only as an opt-in high-assurance delivery adapter.

The factory runs the core loop on **two axes**:

1. **The work axis (per artifact):** SENSOR emits evidence-backed claims → fresh-context Validate writes the verdict → **the verdict is recorded as a first-class provenance-ledger event**. Evidence out, verdict back — and the verdict is now new sensor data.
2. **The recurrence axis (per pattern):** Learn mines accumulated observations and returned verdicts → proposes an apparatus change (a skill, check, or rule) → deterministic replay and shadow evidence earn stronger enforcement.

The trust comes from **separation of duties** inside the one factory: stochastic workers do not self-approve; fresh-context validation checks acceptance evidence and the provenance ledger preserves the result. *"Trust the environment, not the agent"* — at the factory altitude. The destination — *autonomous goal → verified done* — is AgentOps running this loop unattended end-to-end without taking ownership of the consumer repository's delivery system.

## North Stars

<!-- agentops:claim:AOP-CLAIM-GOALS-DREAM-VALIDATED -->
- **The verification membrane is the product.** Nothing an agent generates is trusted until an independent verifier — never the authoring context; a fresh same-family agent is sufficient by default — proves it against an explicit acceptance contract. Mixed families and councils raise rigor when risk warrants them. It cannot *guarantee* stochastic output — it records bounded assurance.
- **Beneath the membrane, the corpus compounds.** A versioned, provenance-tracked context corpus makes each session measurably better than the last — fewer tokens, fewer failures, a rollback path. The knowledge flywheel is the compounding *layer under* the membrane — measured, not asserted; not the headline.
- **Independent verification is table stakes; sovereignty is the differentiator.** Fresh context with author≠validator is the price of entry, not a moat. What is durable is ownership: your proof and corpus, in your formats, in your repo, portable across whichever model wins next quarter. The candidate quality moat — a measurable corpus delta — remains unproven and must stay on the ruler.
- **Compute is fungible; rigor is risk-routed.** One fresh validator is the default membrane. Cross-family judges and councils are optional strategies for higher-risk work, not mandatory infrastructure or a universal quorum.
- Skills work identically across Claude Code, Codex CLI, Cursor, and OpenCode.
- The wiki maintains itself: every session contributes to `.agents/` by default, and knowledge captured in one session is retrieved and applied in the next — including autonomously between sessions (dream cycle), not just on-demand.
- A new user goes from install to first validated flow in under 5 minutes.
- AgentOps speaks public SDLC language while executing the CDLC internally: every high-value context token carries intent, boundary, evidence, decision, constraint, or next action.

## Anti Stars

- Product promises with no automated verification
- Goals that measure code metrics instead of user outcomes
- Quarantined tests that hide real regression risk

## Directives

### 1. Close the multi-runtime promise gap

README and PRODUCT.md promise skills work across 4 runtimes. The current contract is tiered: Tier S structural/install proof must stay green in CI, Tier I live inventory proof may skip when external CLIs/auth are absent unless strict mode is enabled, and Tier E live execution proof remains opt-in/nightly. Keep the Tier S gates green for Claude Code, Codex, Cursor, and OpenCode, and expand Tier I/E only where the runtime can be provisioned reliably.

**Progress:** Tier S is active in CI through `tests/smoke-test.sh`: `tests/skills/test-runtime-claude-code-smoke.sh`, `tests/skills/test-runtime-codex-smoke.sh`, `tests/skills/test-runtime-cursor-smoke.sh`, and `tests/skills/test-runtime-opencode-smoke.sh`. `tests/scripts/test-headless-runtime-skills.sh` exercises the Claude/Codex headless validator contract with mocked runtimes, while `scripts/validate-headless-runtime-skills.sh` performs live Tier I inventory proof when local CLIs/auth are available. Remaining gap: live hosted-runtime execution proof is not a default CI gate.

**Directive ID:** d-close-the-multi-runtime-promise-gap
**Steer:** increase (runtime coverage count)
**Scenarios:** s-2026-05-24-001

### 2. Gate the install path

Three install scripts (`install.sh`, `install-codex.sh`, `install-opencode.sh`) have zero automated testing. A broken install is the fastest way to lose a user. Add install-path smoke tests that verify each script produces a working skill set.

**Progress:** `install-smoke` gate added (`tests/install/test-install-smoke.sh`, weight 5) — validates syntax and structure of all install scripts. Gate is active in CI. Runtime execution tests added: when a local `cli/bin/ao` binary exists, the gate now verifies `ao --version`, `ao help`, and that `flywheel`, `goals`, and `inject` subcommands are registered. Remaining gap: end-to-end install execution (running `scripts/install.sh` against a clean environment) requires a sandboxed CI environment with network access — documented as out-of-scope for local gate.

**Directive ID:** d-gate-the-install-path
**Steer:** increase (install scripts with smoke tests)
**Scenarios:** s-2026-05-24-002

### 3. Resurrect quarantined E2E tests

`tests/_quarantine/` currently has zero active quarantined suites. Keep it empty: newly disabled workflow tests must either be promoted back to CI, deleted as obsolete, or tracked as explicit follow-up work before they can remain quarantined.

**Directive ID:** d-resurrect-quarantined-e2e-tests
**Steer:** decrease (quarantined test count)
**Scenarios:** s-2026-05-24-003

### 4. Verify knowledge lifecycle end-to-end

The flywheel-compounding gate proves σρ > δ (escape velocity). But the full lifecycle — capture quality, injection correctness, citation in downstream work — has no gate. Add a gate that traces one learning from extraction through injection to retrieval.

**Progress:** `flywheel-lifecycle` gate now traces 5 stages: capture → retrieval → inject → round-trip → citation (`scripts/check-flywheel-lifecycle.sh`). Stage 5 (citation) checks for cross-citations between learnings, briefings directory population, and corpus density. Citation checks are soft-fail on sparse corpus (structurally valid but no accumulated sessions yet) — they hard-fail only if the corpus is populated and citations are structurally absent. Gate is active in CI.

**Directive ID:** d-verify-knowledge-lifecycle-end-to-end
**Steer:** increase (lifecycle stages gated)
**Scenarios:** s-2026-05-24-004

### 5. Keep complexity regressions at zero

CC 20 ceiling was achieved. Gate enforces the threshold — the directive is to maintain zero violations and prevent future regressions via pre-commit checks.

**Progress:** cli/ threshold (20) is green. cli/internal/ threshold (18) is green. Previously `validateRoutingLaneGates` was CC 19; refactored into `validateYieldGate` and `validateLaneAuthority` helpers (2026-05-04).

**Directive ID:** d-keep-complexity-regressions-at-zero
**Steer:** decrease (functions exceeding CC 20)
**Scenarios:** s-2026-05-24-005

### 7. Enforce codex parity proactively

CI catches codex drift at push time, but 40% of fix commits in the March 2026 integration were codex parity issues caught too late. The PreToolUse hook warns during editing; the goal gate blocks push if drift exists.

**Directive ID:** d-enforce-codex-parity-proactively
**Steer:** decrease (codex parity findings count)
**Scenarios:** s-2026-05-24-007

> **Experimental tier: Directives 8 and 9.** Both build on the corpus-compounding
> hypothesis, which is named unproven ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md),
> [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).
> They stay maintained but do NOT count toward route priority under Directive 16's
> prioritization rule until the compounding evidence lands. Work here is DEFER-class
> unless it also advances a route milestone.

### 8. Automate the dream cycle (nightly flywheel consolidation)

Today harvest/forge/inject are on-demand — an operator runs them when they remember to. Anthropic's "dream cycle" concept validates what we've known: consolidation should happen automatically between sessions. Ship a GitHub Action (or scheduled Claude task) that runs nightly: harvest new learnings from recent sessions, forge patterns from accumulated learnings, defrag stale knowledge, and report flywheel health. The dream cycle is what turns the flywheel from "useful when invoked" to "always compounding."

**Progress:** Implemented in nightly CI. `.github/workflows/nightly.yml` now runs a dedicated dream-cycle proof job (`harvest -> forge -> close-loop -> defrag -> metrics health`) against the checked-in knowledge corpus, uploads the full report artifact, and updates a rolling GitHub issue with a visible compounding summary. The in-tree scheduling/overnight CLI (`ao schedule`, `ao overnight`, `ao daemon run --schedule-file`) was retired in soc-2rtm0 — AgentOps is no longer the out-of-session orchestration substrate; end-user repos now drive the same loop via GC scheduling. The nightly-CI dream-cycle proof job (built on the KEEP harvest/forge/inject/defrag primitives) remains the in-repo automation surface.

**Directive ID:** d-automate-the-dream-cycle-nightly-flywheel-consolidation
**Steer:** increase (automated consolidation runs per week)
**Scenarios:** s-2026-05-24-008

### 9. Build the pattern-to-skill pipeline (self-programming)

When the same pattern appears across 3+ sessions — a debugging technique, a validation sequence, a refactoring approach — the system should propose a new skill. Today skills are hand-authored. The next step is semi-automated: `/compile` or `/forge` detects recurring patterns, drafts a skill skeleton (SKILL.md + frontmatter), and presents it for human review before promotion. This is Anthropic's "Skillify" concept — compound growth without manual authoring.

**Progress:** Prototype implemented. `ao flywheel close-loop` now generates review-only draft skills under `.agents/skill-drafts/` when a pattern has evidence across 3+ session artifacts. The remaining gap is promotion polish: richer section synthesis, stronger tier heuristics, and a cleaner review/publish path from draft to shipped skill.

**Directive ID:** d-build-the-pattern-to-skill-pipeline-self-programming
**Steer:** increase (auto-proposed skill drafts)
**Scenarios:** s-2026-05-24-009

### 10. Measure skill value through real-task evaluation

The existing eval suites are CI canaries (contract checks). None answers "did this skill change make agents better?" Ship a behavioral eval system with a known-good workbench project, task definitions with golden solutions, and scoring scripts that measure correctness, safety, and process adherence. The eval engine already supports A/B comparison via `--baseline-mode=both` and statistical verdict — the gap is eval content, not infrastructure.

<!-- agentops:claim:AOP-CLAIM-GOALS-EVAL-WORKBENCH -->
**Progress:** Workbench built: 3 components (Go CLI, Python FastAPI, DevOps scripts), 12 tasks with setup/score scripts, behavioral eval suite (`workbench-behavioral-v1`) with 12 cases covering bug-fix, feature implementation, security, refactoring, test-writing, and edge-case handling. `make -C evals/workbench verify` passes golden (12/12) and broken detection (12/12). A/B comparison via DeltaScorecard validated. Agent harness script with industry-proven eval patterns shipped. `eval-skill-delta` CI gate added to `validate.yml` (structural, runs on eval file changes). `--two-pass` mode added to pre-push head gate for local skill-delta validation. Remaining gap: expanding eval-skill-delta from structural-only to a default blocking gate with full skill-on vs skill-off execution across the workbench.

**Directive ID:** d-measure-skill-value-through-real-task-evaluation
**Steer:** increase (behavioral eval tasks with scoring scripts)
**Scenarios:** s-2026-05-24-010

### 12. Operating loop is the execution primitive

Non-trivial work must run through the [operating loop](docs/architecture/operating-loop.md): BDD-shaped intent issue → vertical slices → conflict-free wave (when parallel) → bead acceptance against acceptance examples → evidence + ratcheted learning. BDD/Gherkin + DDD + Hexagonal + TDD is the narrow waist: observable intent, bounded language, explicit ports/adapters, and executable proof. The doctrine source is [`.agents/research/2026-05-15-cdlc-dojo-doctrine.md`](.agents/research/2026-05-15-cdlc-dojo-doctrine.md); the templates are [`docs/templates/intent-issue.md`](docs/templates/intent-issue.md) and [`docs/templates/slice-validation.md`](docs/templates/slice-validation.md).

A bead is "non-trivial" when it crosses sessions, agents, files, or bounded contexts — the threshold under which the loop is overhead. Trivial one-shot work (typo fix, dep bump, doc nudge) is exempt. Everything else must, before implementation begins: name a route from the [Component Map](docs/architecture/component-map.md) and the generated [context map](docs/contracts/context-map.md); carry at least one Given/When/Then acceptance example; decompose into vertical slices with one nameable first-failing-test per slice; mark its wave plan parallel only after the wave-validity check passes; close only when every acceptance example maps to a passing test.

This directive starts in **warn-only** posture. The gate is `scripts/check-loop-shape.sh`: it currently inspects legacy `bd` JSON (open + in_progress beads, or a `--json` fixture) and warns when a bead tagged non-trivial lacks a Gherkin block (Given/When/Then) or a slice candidate. Reconcile that live path to `br` before any strict-mode flip. It runs always-warn in `scripts/pre-push-gate.sh` and never blocks a push; `--strict` (or `AGENTOPS_LOOP_SHAPE_STRICT=1`) exits non-zero on offenders, reserved for the flip to blocking once the corpus-wide pass rate is stable. Regression coverage: `bash scripts/check-loop-shape.sh --self-test`.

Waterfall-shaped speculative plans fail this directive when they create context bulk before proof. The acceptable unit is atomic: one behavior, one bounded context, one first failing test, one write scope, one acceptance proof, and one learning only when it changes future behavior.

**Directive ID:** d-operating-loop-is-the-execution-primitive
**Steer:** increase (beads with BDD intent + slice decomposition before implementation)

**Scenarios:** s-2026-05-24-011
**Tags:** loop-shape, warn-only

### 11. Durability of the corpus across runtime cleanup

On 2026-05-07, routine maintenance wiped most of `.agents/` runtime subdirs (only `.agents/nightly/` is git-tracked); a fresh `scripts/corpus-stats.sh` returns near-zero counts even though the 2026-05-04 stable snapshot recorded ~1,842 learnings, ~186 patterns, ~80 planning rules, and ~3,867 cited decisions. The dogfood receipts claim — and the broader "corpus is the moat" positioning — depends on that asset being durable across cleanup, machine moves, and reinstalls. This directive tracks the design and implementation of a snapshot/restore mechanism: scheduled snapshots of `.agents/` runtime state to durable storage, restore tooling that can rehydrate a fresh checkout, and a freshness/coverage gate so degradation is visible before the receipts go stale. Historical tracker reference: legacy bd issue `soc-rv5p`; map it to a br bead before implementation.

**Directive ID:** d-durability-of-the-corpus-across-runtime-cleanup
**Steer:** increase (snapshots / restore mechanism)

**Scenarios:** s-2026-05-24-012
**Tags:** corpus-state

### 13. Agent-ergonomic ao CLI surface

The `ao` CLI's primary user is an AI agent: the first command an agent guesses must work or be redirected with a useful hint. Every new or changed `ao` command surface follows the agent-ergonomics contract — read-side commands expose `--json` with stdout-as-data / stderr-as-diagnostics separation; the CLI stays self-describing via `ao capabilities` (machine-readable contract) and `ao robot-docs` (agent handbook); parent commands emit a JSON subcommand listing under `--json` instead of human help; an unknown flag returns a Levenshtein typo hint naming the corrected flag; and every error names the exact command or flag the agent should have used instead of a bare "see --help". The reference surfaces are `cli/cmd/ao/capabilities.go`, `cli/cmd/ao/robot_docs.go`, `cli/cmd/ao/flag_suggest.go`, and `cli/cmd/ao/group_json.go`; the doctor presentation module (`cli/internal/commands/doctor/module.go`) is the precedent the top-level surfaces mirror, with root wiring in `cli/cmd/ao/doctor_module.go`.

**Progress:** Top-level `ao capabilities` and `ao robot-docs` shipped (0 → 2 introspection surfaces); CLI-wide flag-typo correction, required-flag hints, parent-command JSON listing, and corrective-command error messages landed across `autodev` / `claim` / `citation` / `constraint`. `ao plans list/search/diff` fixed to honor `--json` (0/3 → 3/3 read-side `plans` commands). Remaining gap: a wider sweep of read-side leaf commands for `--json` fidelity is still owed, and the doctor extended exit-code dictionary is not shared by other diagnostic commands.

**Directive ID:** d-agent-ergonomic-ao-cli-surface
**Steer:** increase (read-side `ao` commands honoring `--json` + error-teaches)
**Scenarios:** s-2026-05-24-013

### 14. Every behavior has a consequence (reinforcement contract)

Behavior shaping needs a consequence system, not just intent shape (that is directive #12). Reinforce the behaviors you and the agent agree on — gates (`/vibe`, validation, CI green) are the reward; the [ratchet](docs/architecture/operating-loop.md#the-promotion-ratchet) locks a reinforced behavior permanently. Extinguish unwanted behaviors by removing their cue and reward (delete the scenario or gate), not by prose prohibitions. A behavior with no gate is unreinforced and drifts. Promotion to canonical/merged is a reinforcer the operator applies, not one the agent self-administers.

This is the consequence half of [Behavior-Shaping Environment](docs/architecture/behavior-shaping-environment.md) (directive #12 is the intent-shape half); vocabulary at [`skills/domain/references/behavior-shaping.md`](skills/domain/references/behavior-shaping.md). Posture: **warn-only doctrine** — the measurable signal is that shipped behaviors carry a gate, reusing directive #12's scenario + loop-shape signals rather than adding a new gate. The anti-pattern is gateless prose-spec work.

**Directive ID:** d-every-behavior-has-a-consequence-reinforcement-contract
**Steer:** increase (behaviors that ship with a reinforcing gate, not prose prohibitions)
**Scenarios:** s-2026-05-24-014

### 15. Teardown-removed apparatus stays removed (anti-regeneration)

The teardown (epic ag-097 and its waves) deletes over-projected apparatus — dead packages, the gascity compat cluster, duplicated projections. Nothing in the fitness function rewards that subtraction, so the `/evolve` loop could rebuild what was deliberately removed. This directive makes "the slop we removed stays removed" a measured outcome: the `no-apparatus-regrowth` gate (`scripts/check-no-apparatus-regrowth.sh`) reads the committed `scripts/removed-apparatus.txt` manifest and fails only when a surface the teardown explicitly removed comes BACK.

It is a stay-removed OUTCOME guard, not a code-size metric (GOALS.md "## Anti Stars": "Goals that measure code metrics instead of user outcomes"). It does not count lines, files, or CI jobs and does not penalize legitimate new growth — it fires only on regrowth of specifically-removed surfaces. Each teardown wave appends its deleted path to the manifest in the same change, locking the removal in; an intentional return must delete the manifest line in that same change with a rationale.

**Directive ID:** d-teardown-removed-apparatus-stays-removed
**Steer:** decrease (teardown-removed surfaces that regrew)
**Scenarios:** s-2026-05-24-015

### 16. Reach the self-hosting line — autonomous goal → verified done

**The destination (see "## The Destination"), made measurable.** AgentOps reaches it when the navigator drives its OWN next epic to verified-done end-to-end, unattended. This directive *prioritizes all others*: an epic is on the route only if it advances one of the route milestones below; an epic that advances none is deferred. This is the goal that drives the repo.

Route milestones (the paving stones, in dependency order — each carries an honest status, not a claim):

1. **Position signal** — the navigator must know where it is. The SDLC provenance ledger exists and is tamper-evident (`ag-8jf97`, landed `479891017`). The bead→commit **land emitter is now wired + live** (`ao provenance emit-landed`, `ag-62jrm` landed `e17d68b9e`): the pre-push gate feeds the ledger on every landing — non-blocking and self-terminating. The **gate-verdict + claude-code-review-verdict emitters remain** (`ag-cm8nd`), so the signal feeds on *landings* but not yet on *verdicts*. *Status: feeding on landings; verdict emitters pending.*
2. **Resilience / the role state-machine** — the navigator must recover without a human: mechanical failures fix-forward, substantive ones re-scope with the failure as the new acceptance, blockers pull the andon (stop-the-line). `ship-beads` builds but cannot self-merge yet. *Status: build half works, recovery half is a stub.*
3. **The membrane** — "done" must be *verified*, not merely asserted: Validate binds an acceptance-grounded verdict from a fresh context, and deterministic tests provide the windshield that catches a hallucinated done. A fresh same-family validator is sufficient by default; mixed families are opt-in. Delivery adapters may consume the proof but do not create it. *Status: four-umbrella membrane live; provenance windshield poured.*
4. **Self-improvement** — each drive must measurably improve the next: SENSOR (the ledger) → ASSAY (bounded periodic miners over it) → GATE (suggestions re-enter the same front door). *Status: designed; sensor floor just poured; miners + tick unbuilt.*
5. **Governance front-door** — nothing new is born ungoverned: the kind-unified factory admits a skill / workflow / loop only with a bounded context + role + runnable acceptance (epic `ag-3fp54`; S0 schema landed `692f420ac`). *Status: schema landed; front-door unbuilt.*

The drive is unattended-end-to-end only when all five hold at once. Until then, the route milestone with the lowest status is the highest-priority work.

**Directive ID:** d-reach-the-self-hosting-line-autonomous-goal-verified-done
**Steer:** increase (route milestones reaching verified-done, toward the unattended end-to-end drive)
**Tags:** destination, north-star

### 17. Verification economics — hold the escape SLO at minimum token cost

The membrane's guarantee (no verdict = not done) is priced, not free. This directive makes
**discrimination per token** a measured fitness axis (assessment:
[docs/audits/verification-economics.md](docs/audits/verification-economics.md); epic
`age-verification-economics-ebec`). The ruler, in dependency order: **TPVD** (tokens per
verified done), **VOR** (verification overhead ratio = verify spend ÷ produce spend),
**CPCD** (cost per caught defect), and an explicit **escape SLO** with an error budget
(current evidence: 0 escapes across the verdict corpus; rule-of-three CI bounds the true
rate ≤ ~2%). Spend to the budget, not to zero: when the budget sits untouched at current
spend, the membrane is over-provisioned — move a lane one tier cheaper and watch the meter.

Honesty rules (binding): thresholds are set FROM measured meter data (two weeks minimum),
never invented; claims stay inside the confidence interval the sample supports; the meter
reads the harness, never producer self-report; cost columns print UNMEASURED until the
meter bead (`.1`) lands. The observational surface is `scripts/verification-economics-report.sh`
(warn-only gate `verification-economics` in the Gates table below); it FAILS only on a dead
instrument, never on a threshold, until the ruler earns thresholds from data. Phase-2 work
(risk router, context diet, cheap-tier default) stays DEFER until the ruler reads real
numbers — act on data, not on the assessment.

**Progress:** report instrument + warn-only gate landed 2026-07-06 (`b0149df9a`, bead `.2`;
live: 322 verdict edges, 2.8% refute rate). Benched-family stall tax removed from the default
pawl route 2026-07-07 (`335300b17`, bead `.7`, council-decided) — ~3.5 min/land recovered.
Meter (`.1`) and data-derived thresholds pending.

**Directive ID:** d-verification-economics-escape-slo-at-minimum-cost
**Steer:** decrease (tokens per verified done at a held escape SLO)
**Tags:** economics, warn-only

## Three-Gap Contract Proof Surface

AgentOps defines a three-gap contract ([context lifecycle](docs/context-lifecycle.md)) covering the failure modes that persist after prompt construction and agent routing. Honesty rule: gates only appear in the **Currently enforcing** column when they (a) run in CI/pre-push/release automation AND (b) reliably go green in single-session work. Gates that are declared but not yet enforced — usually because they measure cross-session or corpus-level state — sit in the **Roadmap** column.

| Gap | What fails without it | Currently enforcing | Roadmap (declared, not yet enforced) |
|-----|-----------------------|---------------------|---------------------------------------|
| **1. Judgment validation** — agents ship without risk context | Plans skip architecture fit; implementations pass happy path but miss edge cases | `go-vet-clean`, `go-complexity-ceiling`, `security-gate`, `contract-compatibility`; `/premortem` and `/vibe` supply the non-mechanical judgment layer | — |
| **2. Durable learning** — solved problems recur | Same auth bug fixed Monday returns Wednesday; agents re-run dead-end investigations | `compile-no-oscillation` (defrag stability), `flywheel-proof` (cross-session evidence, soc-45sg.2), `flywheel-compounding-snapshot` (corpus-state evidence, soc-45sg.1) | `flywheel-compounding` (live long-cycle), `compile-freshness` (runtime-artifact dependency) |
| **3. Loop closure** — completed work doesn't produce better next work | Sessions end with diffs but no extracted lessons; next session starts cold | `release-cadence` (where wired), `goals-validate` (soc-45sg.4), `flywheel-proof` (soc-45sg.2) | — |

**Design rule:** prefer current gates over new scripts unless a true gap is found. The Roadmap column is itself a tracked gap — moving a gate left is the work, not adding new gates.

**Canonical reference:** `docs/context-lifecycle.md` — evidence map and mechanism inventory for all three gaps.

**Today's enforcement state:** Gap 1 is mechanically enforced. Gaps 2 and 3 are partial: scripts exist (`scripts/proof-run.sh`, `scripts/check-flywheel-compounding.sh`, etc.) but are not invoked from automation that blocks merges. `flywheel-compounding` is explicitly long-cycle by design — its green path requires multi-session corpus growth, not a single push. The right way to read this table: PRODUCT.md and GOALS.md are allowed to run ahead of the repo because they are desired-state specifications. The Current Proof column is actual state; the Roadmap column is the reconcile queue that `/evolve`, dream, validation gates, and follow-up work drive toward closure.

`ao goals measure` runs every declared gate on demand and is the canonical way to inspect current state, including roadmap gates.

## Scenario satisfaction layer — PRE-PRODUCTION (do not read as RED)

**Status (cp-m8md, 2026-06-10): the executable-spec scenario-satisfaction layer is built but PRE-PRODUCTION — the producer was never wired.** `ao goals measure --scenarios-only` reports `unknown / 0% satisfied / 0 evaluated` for **every** directive. This is **not** a fleet-wide failure: it is a dead instrument. Read those rows as "never measured," not "failing."

What exists (consumer side, fully built): the `scenario-results.v1` schema, loader, writer, the per-directive `EvaluateSatisfaction` aggregator (`cli/internal/goalsfitness/`), and the `ao goals measure` scenario table that displays the verdict. The directive↔scenario link graph is clean (`ao goals scenarios --lint` → "No executable-spec link defects found"); the scenario specs themselves are authored and valid under `spec/scenarios/`.

What is missing (producer side — the dead instrument): **nothing writes `.agents/rpi/scenario-results.json`.** The only callers of `scenarioresults.Writer.Append` are unit tests. There is no `ao scenario evaluate`/`run` command — `ao scenario` exposes only `add | init | list | validate`. ADR-0003 and the loader comments describe an "RPI evaluator" / council-judge STEP 1.8 that produces the artifact, but that producer was never shipped. With no artifact, `EvaluateSatisfaction` correctly returns `VerdictUnknown` (zero evidence is never a pass and never a fail) and satisfaction renders as 0%.

**Why no fixture was written:** faking a `scenario-results.json` to turn the panel green would manufacture satisfaction with no evaluation behind it — the exact anti-pattern the verification membrane forbids (a self-declared verdict). The honest state is "instrument not yet wired," recorded here.

**Light-it-up plan (proposal — filed in `evidence/cp-m8md-goals-realignment.md`):** ship a producer — either (a) an `ao scenario evaluate` subcommand that runs each linked scenario's Given/When/Then against a council judge and appends results via the existing `scenarioresults.Writer`, or (b) a council STEP-1.8 hook in the validation path that writes the artifact per merged bead. Wire that producer into a cadence (the nightly dream-cycle job or the controller tick) so satisfaction accumulates. Until then, the directives' real fitness signal is the **Gates** table below — the scenario layer is observational, not a gate, and is excluded from the weighted score.

## Gates

The optional `Tags` column lets a gate declare classification metadata that
flows through to `ao goals measure --json` (each measurement carries a `tags`
field). The `long-cycle` and `corpus-state` tags mark gates whose green path
depends on multi-session corpus growth rather than the current commit, so
operator tooling (e.g. /evolve selection) can distinguish "code-actionable"
failures from corpus-bound ones without lowering weights or removing the gate.
The `runtime-artifact` tag marks gates whose green path requires a gitignored
artifact produced by a separate run (e.g. `ao defrag` writing
`.agents/defrag/latest.json`); such flips do not propagate across environments.

| ID | Check | Weight | Description | Tags |
|----|-------|--------|-------------|------|
| flywheel-compounding | `bash scripts/check-flywheel-compounding.sh` | 3 | Knowledge flywheel above escape velocity (σρ > δ); requires multi-session citation activity, not movable by single-session automation — see `.agents/findings/f-2026-04-29-001.md`. **Feeder context (cp-m8md):** this FAIL is the cp-aa5t "0%-routed corpus influence" finding expressed as math — corpus influence σρ (≈0.002–0.006) sits far below δ because the corpus has accumulated content but little cross-session citation. The number is fed by the in-flight corpus work: **cp-s82z** (corpus backfill) raises σ (citation volume), **cp-0gyc** (operator library) raises ρ (citation concentration / influence). This gate stays RED by design until those feeders land — it is a long-cycle/corpus-state signal, NOT a single-push-fixable code defect. Do not threshold-game it; close the feeders. | long-cycle, corpus-state |
| flywheel-compounding-snapshot | `bash scripts/check-flywheel-compounding-snapshot.sh` | 5 | CI-readable corpus-state evidence: validates `docs/releases/flywheel-compounding-snapshot.json` exists, is < 14 days old, and contains a readable `escape_velocity_compounding` health value. Operator refresh: `bash scripts/snapshot-flywheel-compounding.sh`. Keeps G1 observable in CI without pretending a long-cycle corpus health regression can be fixed by a single push. Use `AGENTOPS_FLYWHEEL_SNAPSHOT_REQUIRE_COMPOUNDING=1` for strict local enforcement. |  |
| flywheel-proof | `bash scripts/proof-run.sh` | 7 | Flywheel compounds across sessions (automated proof) |  |
| skill-frontmatter | `bash scripts/validate-skill-frontmatter.sh` | 6 | Every skill has valid YAML frontmatter (schema-validated via validate-skill-frontmatter.sh; supersedes the stale head-10 inline check which false-failed skills with metadata blocks before description) |  |
| spine-integrity | `timeout 60 bash scripts/check-spine-integrity.sh` | 6 | The 15-skill membrane+bookkeeper spine is intact: each spine skill exists, carries spine:true, passes its validate.sh + schema contract, and leads the SKILLS.md router. Partition-durability guard (age-focus-membrane-bookkeeper-m1wg.23). |  |
| go-cli-builds | `cd cli && go build -o /dev/null ./cmd/ao` | 8 | Go CLI compiles without errors |  |
| go-cli-tests | `cd cli && timeout 240 go test -race ./...` | 8 | All Go tests pass with race detector |  |
| go-vet-clean | `cd cli && go vet ./...` | 5 | No common bugs detected by vet |  |
| go-complexity-ceiling | `timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/ --threshold 20 && timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/internal/ --threshold 18` | 6 | No Go function exceeds CC thresholds (cli/: 20, cli/internal/: 18) |  |
| security-gate | `test -x scripts/security-gate.sh && timeout 60 bash tests/scripts/test-security-gate.sh` | 6 | Security toolchain gate is executable and passes |  |
| manifest-versions-match | `test "$(jq -r '.metadata.version' .claude-plugin/marketplace.json)" = "$(jq -r '.version' .claude-plugin/plugin.json)"` | 5 | Plugin and marketplace versions in sync |  |
| contract-compatibility | `timeout 60 bash scripts/check-contract-compatibility.sh` | 5 | Contract schemas and references exist on disk |  |
| goals-validate | `bash -c 'd=$(mktemp -d "${TMPDIR:-/tmp}/ao-goals-val.XXXXXX"); cd cli && go build -o "$d/ao" ./cmd/ao && cd .. && "$d/ao" goals validate --json 2>/dev/null \| jq -e ".valid == true"; rc=$?; rm -rf "$d"; exit $rc'` | 5 | GOALS.md parses and validates without structural errors |  |
| compile-freshness | `bash scripts/check-compile-health.sh` | 4 | Compile defrag report is fresh and stale learnings are low | runtime-artifact |
| compile-no-oscillation | `bash scripts/check-compile-oscillation.sh` | 4 | No evolve goals oscillating across consecutive cycles | runtime-artifact |
| codex-parity-drift | `bash scripts/check-codex-parity-drift.sh` | 5 | No codex parity findings from audit |  |
| quarantine-empty | `bash scripts/check-quarantine-empty.sh` | 4 | tests/_quarantine/ holds zero `.sh`/`.bats` suites (Directive D3). Single-cycle override: set `ALLOW_QUARANTINE=1` when intentionally parking a flaky suite. |  |
| corpus-freshness | `bash scripts/check-corpus-freshness.sh` | 4 | Newest corpus snapshot under `$AGENTOPS_CORPUS_SNAPSHOT_DIR` (default `~/.agentops/corpus-snapshots/`) is within 7 days. Skips cleanly when no snapshots exist. Override: `AGENTOPS_CORPUS_FRESHNESS_SKIP=1`. Companion: `ao corpus snapshot` / `ao corpus restore` (Directive D11). |  |
| finding-registry | `bash scripts/check-finding-registry.sh` | 4 | Finding-registry contract enforced: schema is valid JSON, required-field list cross-checks with contract doc, canonical path documented, and live `.agents/findings/registry.jsonl` lines (when present) validate against required fields. CI runs structural-only (no registry); operator boxes also validate live lines (A2 audit follow-up). |  |
| contracts-structural-floor | `bash scripts/check-contracts-structural-floor.sh` | 4 | Every `docs/contracts/*.md` meets the structural floor: top-level # heading, cataloged in `docs/documentation-index.md`, body >= 200 bytes, paired schema (if any) is valid JSON. Floor under the per-contract strong gates (finding-registry, etc.). A2 audit follow-up: covers all partial+doc-only contracts at minimum enforcement level. |  |
| three-gap-supergate | `bash scripts/check-three-gap-supergate.sh --gap=all` | 5 | Three-Gap super-gates (TG1 council-coverage / TG2 durable-learning / TG3 loop-closure) emit unified status. Composes existing gates (flywheel-compounding-snapshot, flywheel-proof, compile-health, goals-validate) so each Gap's closure status is one command instead of N. Operator surface: `--gap=council-coverage|durable-learning|loop-closure|all`. |  |
| install-smoke | `timeout 30 bash tests/install/test-install-smoke.sh` | 5 | Install scripts pass syntax and structure validation |  |
| flywheel-lifecycle | `timeout 30 bash scripts/check-flywheel-lifecycle.sh` | 6 | Knowledge lifecycle traces capture → index → inject → retrieval |  |
| eval-workbench-verify | `timeout 60 bash scripts/check-eval-workbench.sh` | 6 | Behavioral eval workbench golden state, task scoring, and suite structure verified |  |
| state-path-resolver-coverage | `bash scripts/check-paths-resolver-coverage.sh` | 3 | Tracks executable-code sites that still hardcode `.agents/` paths instead of sourcing the canonical resolver (lib/ao-paths.sh / cli/internal/paths from soc-irg1.1). Warn-only initially per warn-then-fail-ratchet pattern; flip to blocking is a separate follow-up issue under epic soc-irg1 after 2 weeks of baseline data. See `.agents/patterns/2026-05-01-state-path-resolver.md`. | warn-only |
| executable-spec-link-integrity | `ao goals scenarios --lint && ao goals trace --orphans` | 4 | Directive↔scenario link lint and whole-chain orphan/gap audit (F1.6, soc-58nt.1.9). Warn-only until two consecutive clean CI runs on main; promote to blocking by removing `continue-on-error: true` from the `executable-spec-link-integrity` CI job and replacing warn with fail in pre-push check 38. | warn-only |
| no-apparatus-regrowth | `bash scripts/check-no-apparatus-regrowth.sh` | 5 | Anti-regeneration (Directive D15): teardown-removed apparatus stays removed. Reads the committed `scripts/removed-apparatus.txt` manifest and FAILS only when a surface the teardown (epic ag-097) explicitly removed comes BACK. Stay-removed OUTCOME guard, NOT a size/line/file/job metric — legitimate new growth is fine; only regrowth of the listed surfaces fails. Future teardown waves append the deleted path to the manifest in the same change. |  |
| door9-no-api-print | `CP="${CONTROL_PLANE_ROOT:-/Users/bo/dev/control-plane}"; if [ -x "$CP/bin/no-api-print-scan.sh" ]; then ( cd "$CP" && bash bin/no-api-print-scan.sh ); else echo "door9-no-api-print: SKIP (control-plane absent at $CP — set CONTROL_PLANE_ROOT)"; fi` | 6 | LAW 0 as fitness: no `claude -p` / `claude --print` (forbidden API-print) invocations across tracked control-plane files. Runs the control-plane door-9 scanner (`bin/no-api-print-scan.sh`) from the control-plane root (the scanner is cwd-sensitive — it scans `git ls-files`). Cross-repo: resolves `$CONTROL_PLANE_ROOT` (default `/Users/bo/dev/control-plane`); SKIPs cleanly with a notice when the control-plane root is absent. Measured 2026-06-10: CLEAN over 218 tracked files. | cross-repo |
| image-conformance | `CP="${CONTROL_PLANE_ROOT:-/Users/bo/dev/control-plane}"; if [ -x "$CP/bin/image-conformance.sh" ]; then ( cd "$CP" && bash bin/image-conformance.sh --built-only ); else echo "image-conformance: SKIP (control-plane absent at $CP — set CONTROL_PLANE_ROOT)"; fi` | 5 | Agent-image bundles conform: every built image (claude-control-plane / claude-worker / codex-worker / agy-worker) has a complete bundle, SSOT-valid manifest, role-correct duties + dcg-pack, and a green verify.sh. Runs the control-plane conformance matrix `--built-only` **from the control-plane root** (the per-image verify.sh has cwd-sensitive checks — e.g. the worker Rust build-farm-redirect check only resolves from the repo root, so the gate must `cd` there to run RELIABLY per the three-gap honesty rule). Cross-repo: resolves `$CONTROL_PLANE_ROOT`; SKIPs cleanly when absent. Measured 2026-06-10 from the CP root: 28/28 PASS. | cross-repo |
| retirement-ceiling | `CP="${CONTROL_PLANE_ROOT:-/Users/bo/dev/control-plane}"; if [ -x "$CP/bin/ctl-retirement-count.sh" ]; then ( cd "$CP" && bash bin/ctl-retirement-count.sh --gate 37 ); else echo "retirement-ceiling: SKIP (control-plane absent at $CP — set CONTROL_PLANE_ROOT)"; fi` | 4 | Consolidation-by-deletion ceiling (cp-retirement-ledger): the bash control-loop census in control-plane `bin/` must stay at or below the ceiling, so kernel milestones retire loops rather than accrete them. Ceiling set at the REAL measured count (37 as of 2026-06-10, baseline 37, delta 0 — the anticipated +2 ctl scripts did not land tonight, so the honest ceiling is 37, not 39). Cross-repo: resolves `$CONTROL_PLANE_ROOT`; SKIPs cleanly when absent. | cross-repo |
| gated-close-rate | `bash scripts/check-gated-close-rate.sh --threshold 70 --window 20` | 4 | Close-admission discipline (cp-m8md, kin of cp-irmu): of the last 20 closes in the control-plane `br` ledger, the fraction whose `close_reason` carries the `close-admission gate PASS` stamp must stay >= 70%. A high rate means closes go THROUGH the gate, not around it. Reads the ledger READ-ONLY via `br list --status closed --json`. Cross-repo: resolves `$CONTROL_PLANE_ROOT` (default `/Users/bo/dev/control-plane`); SKIPs cleanly when `br`/`jq`/the control-plane root are absent. Measured 2026-06-10: 16/20 = 80%, threshold set just below at 70%. | cross-repo |
| gate-fixhints-live | `bash scripts/check-gate-fixhints-live.sh` | 3 | Self-unfixable-gate meta-check (age-verification-economics-ebec.12): a gate whose fix/remedy/repair directive names a DEAD `ao <cmd>` is a self-inflicted wedge (2026-07-07: corpus-freshness blocked a land with `ao corpus snapshot` after `ao corpus` was archived). Scans the gate-backing scripts and asserts each fix-hint's `ao` subcommand resolves in the live command tree, exempting lines with a removal/historical marker. Warn-only; `--strict` blocks. Measured 2026-07-07: PASS (0 dead hints). | warn-only |
| skill-count-sync | `bash scripts/check-skill-count-sync.sh` | 3 | Skill COUNT is gated, not vibes (age-skills-audit-fable-l6ic.6; audit found declared counts drifted 63/66/77 with nothing comparing them): skills/ dirs vs ACTIVE dispositions-ledger rows vs SKILL-TIERS declared totals must agree. Measured 2026-07-07: all three at 58. | warn-only |
| provenance-feed-health | `bash scripts/check-provenance-feed-health.sh` | 4 | Directive-16 milestone-1 anti-regression (ag-z2q93): assert the SENSOR stays fed. Fails (in `--strict`) when `docs/provenance/ledger.jsonl` has stopped growing beyond the genesis row — i.e. the emitters (`ao provenance emit-landed`/`emit-verdict`, ag-62jrm/ag-cm8nd) silently died. Keeps milestone-1 "fed" an enforced outcome, not a hope. Warn-only by default; `--strict` (or `AGENTOPS_PROVENANCE_FEED_STRICT=1`) makes a dead sensor blocking. | warn-only |
| verification-economics | `bash scripts/verification-economics-report.sh` | 3 | Observational economics surface (epic age-verification-economics-ebec; assessment `docs/audits/verification-economics.md`): structural verdict edges from the provenance ledger — totals, refute rate, per-family and per-month breakdowns — plus a git bind-subject volume cross-check (SKIPs on shallow clones). FAILS only when the instrument is dead (unreadable ledger or zero verdict edges; kin of provenance-feed-health). Cost columns (cost/verdict, cost/catch, VOR) print UNMEASURED until the meter bead (.1) attaches usage fields to verdict edges; thresholds arrive with the ruler bead (.3), set from measured data, never invented. Measured 2026-07-06: 322 verdict edges, 2.8% refute rate. | warn-only |
