# AgentOps 3.3.1 Gas City Factory Visibility and Tuning

```yaml
schema_version: agentops-plan.v1
plan_id: agentops-3.3.1-gas-city-factory-observability-tuning
release: 3.3.1
status: deferred-until-3.3-ready
owner: AgentOps maintainers
intent_owner: this document
unit_of_work: bead
depends_on: agentops-3.3-gas-city-factory-reliability
first_acceptance_check: factory-status-projection-fixture
```

## Intent

Make the released 3.3 factory understandable and empirically tunable without
turning AgentOps into a costly general-purpose model-evaluation platform.

Operators must be able to see current factory state, distinguish queueing from
execution and delivery failures, measure useful yield, and evaluate model or
reasoning changes with bounded evidence. Event-driven issue intake and model
routing begin in shadow mode and cannot mutate production policy until their
own acceptance is proven.

This plan cannot begin until the 3.3 factory is `READY`. Observability must
consume the stable factory; it may not become a new lifecycle authority or a
precondition for semantic or delivery correctness.

## Product decision

Use the stable surfaces already present in Gas City and AgentOps:

- Gas City dashboard/API/SSE for current operational state;
- native OTel metrics to VictoriaMetrics;
- native OTel logs to VictoriaLogs;
- Grafana for historical trends and alerts;
- Beads, certificates, verdicts, delivery records, and landed receipts for
  durable factory truth;
- CAS/CAM/session indexes only for optional token, tool-call, and transcript
  enrichment.

Do not add Prometheus/Loki beside the stable Victoria defaults without a later
requirement proving unique value. Gas City v1.3.5 does not export distributed
traces and does not reliably populate all defined token/cost instruments;
3.3.1 reports missing values as unknown rather than inventing them.

## Read-only factory projection

Provide one projection, derived from existing sources, that shows:

- product and delivery queue depths;
- active semantic and delivery beads;
- worktree/session, role, actual model, reasoning, and fallback;
- candidate, verdict, certificate, delivery epoch, PR, CI, merge, and landed
  state;
- blocked reason, age, and last meaningful event;
- pool capacity, saturation, collector health, and store health.

The projection owns no queue, transition, retry, close, Git, or merge action.
Loss of the projection or telemetry cannot change factory state.

## Minimal yield contract

Derive these facts per bead and cohort:

- admitted count and `PASS | FAIL | NOT_PROVEN` outcomes;
- first-pass semantic yield;
- admitted-to-landed delivery yield;
- time to verdict and time to landing;
- queue wait versus execution time;
- successor/rework and delivery-epoch counts;
- operator intervention/nudge count;
- requested/actual model, reasoning, provider, and fallback;
- token/cost and failed-tool-call counts only when directly attested.

Existing fresh verdicts are the primary quality signal. No new LLM grader is
introduced. Small human samples may calibrate ambiguous comparisons. Organic
production work supplies the default evidence; paired trials are opened only
when that evidence suggests a material decision is available.

## Event-driven intake contract

GitHub issue intake is opt-in and begins in shadow mode:

1. An allowlisted repository issue receives an explicit factory label.
2. A GitHub Action or bounded poller derives one stable intake identity.
3. Shadow mode reports the bead and route it would create without dispatching.
4. Live mode creates one intake bead and uses a native GC Order or explicit
   adapter call to invoke `gc sling`.
5. Native demand and `gc hook --claim` wake the Mayor and enter the unchanged
   3.3 loops.

Do not expose Gas City's local-operator API publicly. Live mode requires a
repository allowlist, label allowlist, idempotency key, per-repository capacity
limit, and auditable source linkage. Delivery still follows the 3.3 automatic
or manual toggle.

## Model/reasoning tuning contract

The 3.3 defaults remain unchanged while organic cohorts accumulate:

- Fable adaptive for Mayor and Refiner;
- Sol high for plan review and validation;
- Terra high as default writer;
- Opus 4.8 medium as challenger/overflow;
- Luna high as support-only when retained.

A bounded comparison opens only when organic evidence suggests a meaningful
cost, latency, throughput, or quality difference. Each role study is capped at
12 matched pairs over at least three task classes with identical intent,
prompt, tools, permissions, harness, and validation policy. Prompts remain
frozen during a comparison. Three randomly selected cases receive human audit.

A default changes only when semantic quality is non-inferior, there is no new
serious authority or quality regression, operator intervention does not
increase, and median cost, latency, or throughput improves by at least 20%.
Otherwise the result is `INCONCLUSIVE` and the current default remains.

Recommendations run in shadow mode before any versioned routing-policy change.
No online optimizer or randomized production routing is admitted.

## Allowed write scope

Implementation is limited to these ownership classes:

- AgentOps GC observability deployment/profile sources and every generated
  dashboard, datasource, alert, and provisioning output owned by them;
- read-only factory status and yield projections over existing bead/evidence
  contracts;
- the opt-in GitHub intake adapter and its GC Order/pack projections;
- shadow routing recommendation policy and cohort records;
- focused fixtures, tests, runbooks, release notes, and generated documentation
  projections for those surfaces.

Gas City/Beads source, semantic/delivery transition logic, AgentOps core verdict
semantics, automatic online routing, public multi-tenant API work, and unrelated
skills or CLI commands are outside scope.

## Complexity admission

Before freezing a bead's scope, include:

- Grafana dashboards plus datasource, alert, and provisioning companions;
- OTel/Victoria configuration, retention, labels, and secret/env projections;
- API/status schemas plus CLI, docs, fixtures, and generated client projections;
- pack/Order sources plus imported-pack and command/schema projections;
- any skill source plus its generated Codex parity twin;
- CAS/CAM adapters and fixtures that assert correlation, token, cost, or tool
  fields;
- release docs and gates that enumerate commands, metrics, or config.

Generated write scope is the owning source plus all outputs of its regeneration
command, not a hand-curated file list.

## Finite bead graph

| Bead | Behavior and completion evidence |
|---|---|
| GC331-1 Telemetry profile | Ship supported VictoriaMetrics/VictoriaLogs/Grafana provisioning, retention, correlation, cardinality limits, and collector-health evidence. |
| GC331-2 Factory status | Produce the read-only two-loop/pool/verdict/PR projection from deterministic fixtures and one live released city. |
| GC331-3 Yield ledger | Derive minimal quality, latency, rework, intervention, and delivery metrics; preserve unknown token/cost/tool data honestly. |
| GC331-4 Alerts | Add stuck-bead, saturation, delivery-thrash, Validator-drift, collector-down, residual-process, and store-health alerts with bounded deduplication. |
| GC331-5 Intake shadow | Implement allowlisted issue-to-intake projection with source identity and idempotency, initially without bead creation or sling. |
| GC331-6 Intake live | After shadow acceptance, enable opt-in bead creation and native sling with repository/label/capacity safety policy. |
| GC331-7 Routing shadow | Generate versioned retain/change/inconclusive recommendations without altering live model assignments. |
| GC331-8 Bounded comparisons | Run only evidence-triggered matched studies within the 12-pair cap and publish quality, time, throughput, cost, and intervention together. |
| GC331-9 Release canary | Prove dashboards, alerts, correlation, shadow/live intake boundary, and one reversible routing-policy update without affecting factory correctness when monitoring is disabled. |

## Acceptance

### Current-state visibility

Given a released factory with semantic work, delivery work, and one blocked
bead, when the status projection is queried, then it shows both queue positions,
the active role/model/session, exact verdict/certificate/PR state, blocker and
age, and source timestamps without reading raw transcripts or changing state.

### Honest yield

Given completed beads with complete and incomplete provider usage data, when
the yield view is generated, then outcome, latency, rework, delivery, and
intervention metrics are deterministic, while unattested token/cost/tool fields
are `unknown` and excluded from model promotion arithmetic.

### Shadow issue intake

Given an allowlisted labeled issue, when shadow intake runs repeatedly, then it
reports one stable prospective intake identity and route without creating a
bead, slinging work, or exposing the GC API. An unallowlisted repository or
label produces no prospective work.

### Live issue intake

Given an accepted shadow result and enabled repository policy, when live intake
runs repeatedly, then exactly one source-linked bead is created and slung using
native GC routing, bounded by repository capacity.

### Shadow model recommendation

Given a bounded, comparable cohort, when a route is evaluated, then the system
reports retain/change/inconclusive with quality, cost, time, throughput, and
intervention evidence, but does not mutate the live role policy.

### Observability independence

Given collectors and Grafana are unavailable, when the 3.3 factory processes a
bead, then semantic and delivery state remain correct and recoverable; only
visibility is degraded.

## Required checks

The first useful check is a deterministic factory-status fixture containing
one semantic bead, one delivery epoch, one blocked bead, and one landed receipt.
It proves the projection reads existing truth before any collector or UI work
is admitted.

Then require:

1. deterministic status and yield fixture replay;
2. unknown/malformed telemetry and bounded-cardinality cases;
3. dashboard/datasource/alert provisioning validation;
4. alert deduplication and recovery cases;
5. issue allowlist, label, idempotency, shadow, and capacity tests;
6. routing comparison symmetry, cap, non-inferiority, and no-mutation checks;
7. one live released-factory projection and observability-loss canary.

## Non-goals and stop conditions

Do not build a 56-case pre-release eval matrix, LLM grader, autonomous online
optimizer, replacement scheduler/queue/merge queue, distributed trace system,
public multi-tenant GC API, or Gas City/Beads fork.

Stop when:

- 3.3 is not yet `READY`;
- a projection needs to own lifecycle state to function;
- token/cost/tool data is inferred rather than directly attested;
- issue intake cannot prove allowlist and idempotency before mutation;
- shadow routing can change live policy;
- a comparison exceeds 12 matched pairs or changes its prompt/harness midway;
- monitoring loss changes semantic or delivery behavior;
- added collection materially reduces factory throughput.

## Release-ready verdict

`READY` requires current-state visibility without transcript archaeology,
historical yield and delivery trends, honest unknown fields, actionable bounded
alerts, safe idempotent issue intake, non-mutating routing shadow, promotion
decisions that satisfy the bounded rule, and proof that observability loss does
not alter factory truth.

Anything less remains `DEFERRED` or `NOT READY`; inconclusive model evidence is
a valid result and preserves the 3.3 defaults.

## Evidence anchors

- `docs/plans/2026-07-17-gas-city-factory-operationalization.md`
- `.agents/research/gc-factory-two-loop-2026-07-20/DUELING_WIZARDS_REPORT.md`
- `.agents/research/gc-factory-two-loop-2026-07-20/GAS_CITY_V1_3_5_CODEBASE_REPORT.md`
- `docs/architecture/gas-city-factory.md`
- `docs/contracts/gas-city-execution-adapter.md`
- Gas City v1.3.5 API, events, dashboard, telemetry, Orders, and trust-boundary
  documentation cited by the pinned codebase report
