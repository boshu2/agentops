# Skill corpus invocation and engineering-practice audit

**Date:** 2026-09-18
**Subject:** the 34 live `skills/*/SKILL.md` files at `78c1429`, plus the two
`skills/_fixtures/*/SKILL.md` files as non-product test data
**Decision:** **REVISE** the invocation taxonomy and practice coverage; retain
the focused skills and the lean native path.

## Executive answer

The corpus is strong at **safe tool use, evidence boundaries, bounded agent
execution, and independent judgment**. Its core `plan` → `implement` →
`validate` story also represents a credible, intentionally lightweight blend
of BDD, TDD, DDD, and iterative delivery.

It does **not**, however, yet meet the stronger claim that the skill system is
cleanly divided into user-invoked, agent-invoked, and dual-invocation skills,
or that it comprehensively encodes XP and continuous delivery. Every live
skill is user-invocable, only three are human-only, and no live skill is
agent-only. That is a usable catalog, but not a meaningful three-way taxonomy.
The engineering-practice story is similarly uneven: behavior, testing, domain
language, safety, and review are explicit; feedback cadence, CI, deployment,
trunk integration, small releases, and operational learning mostly live in
repository docs and gates rather than in the skills that an installed agent
will actually load.

The right correction is **not** to make every skill mention every practice.
Keep each skill focused. Make invocation policy deliberate, add a small
delivery-focused capability only if behavioral evaluation demonstrates a real
routing gap, and use the existing skill graph to show where practices enter
and leave the engineering flow.

## Method and limits

This audit read every line of every source `SKILL.md`, including both fixtures.
The live corpus contained 34 files, 5,496 lines, and 287,711 bytes. Review was
bound to the exact files by the manifest below; generated `skills-codex/` and
runtime images were treated as projections rather than independent skills.

The assessment also read the current philosophy, skill API, router, quality
rubric, skill-system plans, BDD/TDD/XP learning, operating-loop/RPI
architecture, and generated graph/context map. Deterministic checks covered
frontmatter, descriptions, references, graph integrity, and probe coverage.
Those checks establish structure, not whether a skill improves model behavior.

The invocation recommendations apply the repository's own three states:

1. **Human-only:** `user-invocable: true` plus
   `disable-model-invocation: true`; explicit strategy, consequential host
   mutation, credential/runtime choice, or factory selection.
2. **Agent-only:** `user-invocable: false`; background knowledge or a helper
   that users should not need to select directly.
3. **Dual:** user-invocable and model-invocable; a concrete task capability
   that either a caller or the agent can recognize as useful.

This is consistent with the Matt Pocock-style idea visible in the adapted
engineering skills: narrowly triggered, composable expertise with progressive
disclosure, rather than a mandatory phase framework. This audit did not rely
on an unverified claim about the upstream repository's current contents; the
local adaptations and local invocation contract are the authoritative subject.

## Corpus-level findings

### 1. Invocation categories are syntactically supported but not actually balanced

Current distribution:

| Category | Count | Skills |
|---|---:|---|
| Human-only | 3 | `craft-goal`, `postmortem`, `rpi` |
| Agent-only | 0 | — |
| Dual | 31 | every other live skill |

Several dual skills explicitly say “only when the caller selects/requests” and
therefore advertise themselves to the model while instructing it not to select
them. The clearest cases are credential mutation, alternate runtimes, councils,
hooks, factories, storage recovery, and remote execution. This wastes
always-loaded description budget and makes policy depend on prose obedience
when frontmatter can express it mechanically.

There is no requirement that all three buckets be nonempty. In particular,
creating agent-only skills merely for symmetry would be ceremony. The problem
is that the current 31/3/0 split is inconsistent with the skills' own trigger
language, not that a quota is unmet.

### 2. BDD, TDD, and DDD form a coherent but narrow spine

- `plan` owns observable examples, edge cases, scope, and domain language.
- `implement` owns a bounded RED → GREEN → refactor experiment and small-batch
  execution.
- `test` distinguishes behavior from implementation and covers TDD,
  Gherkin-style BDD, properties, and failure modes.
- `domain` owns vocabulary, rule ownership, and bounded-context boundaries.
- `validate` binds fresh judgment to unchanged acceptance and exact content.
- `rpi` composes the three core responsibilities without making the workflow
  mandatory.

That is good engineering design. It avoids the common failure of turning BDD
into `.feature` file ceremony or DDD into a demand for aggregates everywhere.
The main weakness is discoverability and graph continuity: `test` and `domain`
are not declared context relationships of the main implementation path, and
the hard-dependency graph contains only the three RPI edges.

### 3. XP is represented as isolated techniques, not a complete feedback system

The corpus includes TDD, refactoring, small-batch flow, team topology,
independent review, and simple bounded changes. It does not directly encode or
route the broader XP system: rapid customer feedback, pair/mob collaboration,
continuous integration as a working habit, collective ownership, sustainable
pace, simple design, and frequent releases. Some of these belong in native
repository policy rather than a skill, but the catalog should be honest that
it offers selected XP practices, not comprehensive XP.

The historical XP/BDD/TDD learning is materially richer than the installed
skill surface: it discusses fast/slow lanes, paired review, branch freshness,
small releases, and local/remote CI. None of that should be copied wholesale
into `SKILL.md`; instead, retain the durable repository gates and expose only
the smallest reusable delivery decision that agents demonstrably miss.

### 4. Continuous delivery is the largest practice gap

No live skill declares a continuous-delivery practice. `implement` declares
small-batch flow, and several skills discuss release boundaries, but there is
no focused capability covering the path from integrated change through
deployable state, progressive release, observability, and rollback. AgentOps
correctly refuses to own Git, CI, release, or delivery; that product boundary
does not prevent it from offering advisory engineering guidance about those
activities.

Before adding a `delivery` skill, run a behavioral probe against realistic
requests such as “make this safely releasable,” “design the deployment check,”
and “reduce batch size without weakening acceptance.” Add it only if the base
agent repeatedly misses the repository's own delivery discipline. Otherwise,
document the limitation and leave delivery to repository policy.

### 5. Agent-loop safety is excellent; graph semantics are under-expressed

The strongest shared ideas are monotonic bounds, no retry-budget reset, honest
stops, one bounded helper for a genuine stall, author-distinct validation,
exact subject identity, and separation of runtime completion from semantic
acceptance. Runtime and factory adapters consistently avoid claiming tracker,
Git, validation, or lifecycle authority. This is a high-quality response to
known agent-loop failure modes.

The declared graph is much thinner than the prose architecture. It has 34
nodes but only three hard dependency edges; the context map adds 22 optional
relationships. Important flows such as `plan` → `domain`, `implement` → `test`,
`implement` → `refactor`, `validate` → `security`, and `memory` as optional
feedback are either absent or only implied in prose/data labels. A graph that
cannot show these optional practice routes cannot fully explain how the corpus
encodes an engineering loop.

### 6. Structural quality is good; behavioral evidence is not corpus-wide

All 34 live skills pass the repository's frontmatter and trigger checks, all
declared relationships resolve, and no broken relative Markdown links were
found. Every `SKILL.md` stays below the local 500-line progressive-disclosure
ceiling. Long skills such as `cc-hooks`, `cass`, `craft-goal`, and `using-gc`
are nevertheless expensive kernels and should be watched for extractable
reference material.

The probe suite covers selected judgment/product behavior, not every skill or
every advertised host/model combination. Therefore this audit can judge
coherence and visible practice coverage, but cannot claim that all 34 skills
cause better engineering outcomes.

## Per-skill assessment and invocation recommendation

“Good” means the skill accurately serves its stated narrow job. “Revise” means
the capability is useful but its invocation policy, density, graph placement,
or practice claim should change. None of the fixture judgments apply to the
live catalog.

| Skill | Assessment | Recommended invocation | Reason / practice contribution |
|---|---|---|---|
| `account-rotation` | Good, narrow adapter | Human-only | Mutates credentials and already requires an explicit account request. |
| `agent-mail` | Good boundaries | Human-only | Multi-writer coordination is caller-selected; never silently introduce its durable records. |
| `agent-native` | Good, dense | Human-only | Delegation must be authorized; runtime execution is not validation. |
| `agy-native` | Good | Human-only | Alternate runtime selection is explicitly a caller choice. |
| `cass` | Revise density | Dual | Useful agent-selected recall for consequential uncertainty, but 329 lines is heavy and operational details can move to references. |
| `cc-hooks` | Good safety, too large | Human-only | Hook installation/policy mutation is explicitly requested and host-specific. |
| `codex-exec` | Good | Human-only | Starting a separate paid/sandboxed runtime is an execution-shape choice. |
| `council` | Good | Human-only | The skill itself says multiple judges are caller-selected; voting is correctly rejected. |
| `craft-goal` | Good | Human-only (current) | Correctly reserved for an explicitly selected persistent-goal strategy. |
| `dcg` | Good | Dual | The agent should react to an actual block; configuration changes still require explicit authority. |
| `doc` | Good | Dual | Focused documentation and grounded handoffs are legitimate user- or agent-recognized tasks. |
| `domain` | Good | Dual | Strong DDD capability; should gain explicit optional graph links from planning/testing. |
| `idea-genie` | Good | Human-only | Exploration changes decision shape and is described as caller-selected, not an automatic implementation phase. |
| `implement` | Strong | Dual | Best expression of small-batch TDD/refactor execution; correctly returns facts, not PASS. |
| `memory` | Strong boundaries | Dual | Appropriate on-demand feedback/learning capability; correctly rejects mandatory recall and blind retention. |
| `ms` | Good adapter | Dual | Agent can recognize a concrete corpus-search need; preserve its separation from session recall and authoring. |
| `ntm` | Good | Human-only | Persistent pane topology is an explicit runtime choice, not an implicit implementation technique. |
| `plan` | Strong | Dual | Best BDD/DDD entry point; stops when actionable rather than manufacturing a plan artifact. |
| `postmortem` | Good | Human-only (current) | Correctly explicit, off the code-acceptance path, and permits an honest no-lesson outcome. |
| `premortem` | Good | Human-only | Its description asks the caller to select a fresh judge; policy should enforce that. |
| `rch` | Good | Human-only | Remote compilation/daemon mutation is an infrastructure choice with external effects. |
| `reality-check` | Good | Dual | The model can identify unsupported claims; it correctly avoids pretending a gap report is final validation. |
| `refactor` | Strong | Dual | Faithful behavior-preserving refactoring with baseline/regression evidence. |
| `research` | Strong | Dual | Bounded question/decision/evidence framing is broadly useful and avoids ritual reports. |
| `reverse-engineer` | Good, substantial | Human-only | External clone/binary analysis needs authorization and an explicit competitive question. |
| `rpi` | Strong | Human-only (current) | Lean explicit strategy, not a mandatory super-loop; preserves native execution. |
| `sbh` | Good | Human-only | Storage mutation/deletion must never arise from ambient model selection. |
| `security` | Strong | Dual | The model should recognize relevant security risk, while high-impact actions remain bounded. |
| `skill-builder` | Good | Dual | Appropriate specialist for explicit authoring or repair; should not be a routine response to weak guidance. |
| `skill-eval` | Strong | Dual | Correctly distinguishes conformance from behavioral benefit and allows removal/no-effect outcomes. |
| `test` | Strong | Dual | Explicit TDD/BDD/property-testing capability; should be connected to implementation in the optional graph. |
| `using-flywheel` | Good boundary | Human-only | Factory selection is explicitly caller-owned; convergence is correctly not PASS. |
| `using-gc` | Good boundary, too large | Human-only | Same factory-selection reason; extract volatile operational detail where possible. |
| `validate` | Strong | Dual | The model should recognize when proof is needed; freshness and author separation remain runtime facts. |
| `_fixtures/good-skill` | Valid fixture, not a product skill | Neither/catalog-excluded | Minimal positive gate fixture. |
| `_fixtures/bad-skill` | Intentionally invalid fixture | Neither/catalog-excluded | Secret-like values and missing metadata are test inputs, not guidance. |

Proposed distribution after invocation-only revisions: **18 human-only, 16
dual, 0 agent-only**. That is not a target quota; it follows the current
descriptions. If graph-consumer and routing-probe analysis proves any proposed
human-only skill is programmatically reached, keep that skill dual until the
consumer is redesigned.

## Exact source manifest

The hash prefixes below bind the line-by-line review to the reviewed bytes.

| Skill | Lines | SHA-256 (first 12) |
|---|---:|---|
| `account-rotation` | 60 | `2e458298e484` |
| `agent-mail` | 139 | `25a09de08822` |
| `agent-native` | 142 | `9b7231d42f16` |
| `agy-native` | 72 | `74f5201fdcd3` |
| `cass` | 328 | `3f5dc7b861e5` |
| `cc-hooks` | 362 | `a3d776a94fe5` |
| `codex-exec` | 135 | `9043f68a5164` |
| `council` | 193 | `eea17e95e791` |
| `craft-goal` | 292 | `dfa2ccc417e2` |
| `dcg` | 235 | `d96f65df7bd5` |
| `doc` | 132 | `29a3cb5adf08` |
| `domain` | 92 | `09e09cb37e66` |
| `idea-genie` | 124 | `653fa4cc0143` |
| `implement` | 139 | `76f826003d8b` |
| `memory` | 129 | `6ac12abe5e43` |
| `ms` | 209 | `7b88a64be286` |
| `ntm` | 117 | `bddbbbe65772` |
| `plan` | 125 | `6fea8be04a1c` |
| `postmortem` | 122 | `a9a6e157ea47` |
| `premortem` | 164 | `fa0ccd0f372c` |
| `rch` | 86 | `bbfa16a354da` |
| `reality-check` | 89 | `f5642c0647c4` |
| `refactor` | 122 | `7bf047eef937` |
| `research` | 145 | `5be1ebfb4cce` |
| `reverse-engineer` | 221 | `0a7d7daea098` |
| `rpi` | 134 | `68e252a47404` |
| `sbh` | 66 | `20c169144317` |
| `security` | 195 | `62e054be784b` |
| `skill-builder` | 142 | `acee76e82e39` |
| `skill-eval` | 224 | `c16691ee2b90` |
| `test` | 139 | `4dac24366be6` |
| `using-flywheel` | 115 | `5ae6593259d0` |
| `using-gc` | 359 | `00e7e11e287f` |
| `validate` | 148 | `b9b184792292` |
| `_fixtures/bad-skill` | 14 | `b2ea6a1060ea` |
| `_fixtures/good-skill` | 21 | `da48918e68ea` |

## Recommended sequence

1. **Fix policy before prose.** For each proposed human-only skill, run the
   repository's required consumer/graph/workflow/probe search. Add
   `disable-model-invocation: true` only when no model-invoked route depends on
   it, regenerate projections, and measure the description-token reduction.
2. **Strengthen optional graph truth.** Add only relationships that help a
   router or reader make a real decision: planning↔domain, implementation↔test,
   implementation↔refactor, validation↔security, and optional memory feedback.
   Do not turn these into mandatory dependencies.
3. **Probe the delivery gap.** Compare base versus focused guidance on several
   real delivery decisions. Add a small skill only if it improves observable
   outcomes; otherwise state that CI/CD remains repository-owned policy.
4. **Trim the largest kernels.** Move volatile command catalogs and extended
   recovery cases out of the four longest `SKILL.md` files into linked,
   one-level references while preserving trigger, boundary, procedure, and
   output in the kernel.
5. **Evaluate, do not declare victory.** Use direct, indirect, incomplete-input,
   should-not-trigger, and edge cases. Structural green proves packaging, not
   engineering benefit.

## Acceptance mapping

| Requested question | Evidence | Result |
|---|---|---|
| Read every skill line | Exact 36-file manifest; 34 live files counted separately from two fixtures | Checked |
| Assess three invocation categories | Frontmatter census plus per-skill trigger/boundary review | **Revise** |
| Assess BDD/DDD/TDD/XP/CD | Practice tags, skill procedures, philosophy, learning, and architecture comparison | **Partial**: strong BDD/DDD/TDD spine; selected XP; weak CD |
| Assess agent loops and graphs | RPI/runtime boundaries plus generated graph/context-map comparison | **Strong loops; partial graph expression** |
| Gather goals and docs context | Current philosophy, how-it-works, RPI traversal, API, router, rubric, plans, and learning included | Checked |

**Unchecked:** behavioral uplift for every skill/model/host combination; current
upstream Matt Pocock repository behavior; whether a future invocation-policy
patch would preserve all indirect routing until that exact patch is produced
and tested.
