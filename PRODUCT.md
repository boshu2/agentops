---
last_reviewed: 2026-07-13
---

# PRODUCT.md

## Mission

AgentOps is an open-source validation layer for agentic software development.
It helps an operator answer two questions: is this change right, and is the
evidence strong enough to grant the worker more autonomy?

The proven product is a validation membrane plus a sovereign evidence trail:
fresh-context judgment binds a verdict to an exact candidate, and the
hash-chained provenance ledger preserves that verdict with the repository. No
evidence-backed verdict means not done. Whether accumulated context produces a
measurable quality advantage is still an experiment, not a product claim
([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md),
[ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

## Product contract

AgentOps provides two operator surfaces:

- skills that agents load inside supported harnesses;
- the `ao` CLI for explicit repository commands, gates, evidence, and CI.

The normal loop runs through four explicit umbrellas: `/discovery` shapes
observable acceptance, `/crank` implements bounded vertical slices,
`/validate` gives an independent PASS/WARN/FAIL judgment against the exact
candidate, and `/learn` records verdict-bound observations and plan impact for
the orchestrator's next decision. `ao gate check`, `ao verify`, `ao provenance`,
and `ao done` provide deterministic and recorded evidence around that loop. An
external orchestration substrate may repeat or parallelize it. Runtime hooks are not an AgentOps default;
neither are a daemon, scheduler, hidden bootstrap, or automatic session setup.

The merge pawl is defined by [the pawl contract](docs/contracts/pawls.md). A
worker cannot turn its own claim into the binding verdict. Green tests establish
facts; a fresh reviewer judges whether the behavior and evidence satisfy the
brief. REFUTED returns to the earliest invalid move. A breaker HOLD receives one
bounded fresh-context consultation before operator escalation, unless a hard
budget or human-only decision is genuinely exhausted.

## What is owned

Tracked repository truth lives in declared contracts, schemas, source code,
generated projections, tests, and the provenance ledger. The `.agents/`
directory is workspace-local, gitignored runtime state: useful evidence and
memory, but fallible and non-authoritative until promoted into a tracked owner.
The product is sovereign because its durable contracts and receipts remain
portable across models and harnesses, not because every local scratch artifact
is committed.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-CONTEXT-ARTIFACT -->
The highest-leverage input to a coding agent is validated context: product
intent, boundaries, prior evidence, failed approaches, and the gates that must
hold. AgentOps makes those inputs explicit and reviewable. It does not treat
retrieval volume or a growing corpus as proof that the next run improved.

## Engineering model

- BDD/Gherkin describes observable intent.
- DDD supplies stable names and six bounded contexts.
- Hexagonal architecture keeps domain policy behind ports and adapters.
- TDD supplies local behavioral proof for behavior-changing work.
- Small vertical slices keep failures attributable and diffs reviewable.
- CI, SRE, ADRs, and provenance make repeated operation trustworthy.

The stable architecture owners are the
[component map](docs/architecture/component-map.md),
[ports-and-adapters map](docs/architecture/ports-and-adapters.md), and
[bounded-context contract](docs/contracts/bounded-contexts.yaml). The generated
[skill domain map](docs/reference/agentops-skill-domain-map.md) classifies skills;
it is not a CLI capability matrix.

## Users and first value

The primary user is an agent-heavy maintainer who already feels session amnesia,
repeated mistakes, and low confidence in plausible-looking output. A successful
first use leaves four inspectable things: a testable brief, a bounded diff,
passing deterministic evidence, and an independent verdict bound to the exact
candidate. The next increase in autonomy is earned from that evidence.

Quality-first maintainers use the same membrane before release. Operators who
run repeated or parallel agents add an explicit orchestration substrate only
after the single-loop contract is reliable. One-off prompt users, hosted-control-
plane buyers, and teams unwilling to inspect evidence are not the initial
audience.

## Proven, experimental, and excluded

Proven:

- independent validation and commit-bound verdict recording;
- tamper-evident, repository-owned provenance;
- explicit gates and reproducible command surfaces.

Experimental:

- measurable cross-session improvement from accumulated local context;
- unattended repetition and multi-agent throughput economics;
- factory admission and scoring beyond current validation contracts.

Excluded from the default product:

- an AgentOps-hosted cloud control plane;
- automatic runtime hooks or orchestration startup;
- a requirement for multiple model vendors;
- self-approval, manufactured evidence, or corpus size as a success proxy.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-FACTORY-GRADE-THROUGHPUT -->
The aspiration is factory-grade confidence first and throughput second. Scale is
earned only while the validation boundary, evidence quality, and escape rate
hold.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-EVOLVE-RECONCILE -->
Evolution means reconciling measured behavior with product intent: strengthen a
skill, test, contract, or gate when evidence exposes a gap; delete or archive a
surface when it no longer has a live consumer. It does not mean accumulating
scripts, routers, or prose controls.

## Distribution

Distribution/runtime reach: 63 shared skills, 62 checked-in Codex artifacts, and 13 Codex overrides.

Counts are generated and drift-checked; they are inventory, not value. Current
command truth comes from `cli/cmd/ao/` and generated `cli/docs/COMMANDS.md`.

## Success measures

Measure verified outcomes, refute/escape rate, time and cost per verified done,
candidate-to-verdict binding, and whether promoted learning changes a later
plan, test, skill, or gate. Do not substitute file count, token volume, agent
count, or prose coverage for behavioral evidence.

The executable destination and fitness checks live in [GOALS.md](GOALS.md).
The always-loaded operating contract lives in [AGENTS.md](AGENTS.md); it remains
small and routes deeper mechanics only when the task triggers them.
