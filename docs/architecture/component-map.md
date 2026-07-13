# AgentOps Component Map

> **Status:** routing map for validation-centered DDD + hexagonal architecture.
> **Purpose:** line up product direction, bounded contexts, ports/adapters,
> and open bead posture before new work is shaped.
> **Use with:** [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md),
> [Skill Ports and Adapters](../contracts/skill-ports-and-adapters.md), and
> [Bounded Contexts](../contracts/bounded-contexts.yaml).

AgentOps narrows to one product direction:

> **Repo-local autonomous code validation: an in-session loop, verification
> membrane, and context compiler that prove agent output before autonomy
> expands.**

Everything else is either an adapter that helps that route, a substrate concern
outside the product core, or backlog sprawl to defer or trim.

## Routing Rule

Before creating a skill, workflow, CLI command, gate, or bead, route it through
this map:

1. Name the component and bounded context.
2. Name the port behavior, not the tool.
3. Name concrete adapters separately.
4. Keep the work only if it proves code or agent output, advances the golden
   route, measures the corpus delta, hardens the validation membrane, or
   preserves the repo-local corpus.
5. Defer or trim anything that only expands surfaces, duplicates the loop, or
   belongs to an external product/domain.

## Components

| Component | Bounded context | Owns | Ports / objects | Product posture |
|---|---|---|---|---|
| Context Corpus | BC1 Corpus | `.agents/`, `.ao/wiki`, retrieval, capture, citation, learning promotion, corpus durability | `CorpusReaderPort`, `CorpusWriterPort`, `ContextCompilerPort`, `CitationPort`, `FindingCompilerPort`, `Finding`, `Learning`, `ContextPacket` | Supporting core. This is the durable asset and candidate moat, but the quality claim is unproven until measured. |
| Verification Membrane | BC2 Validation | gates, CI/local gate verdicts, council/premortem/validate, safety policy, evidence binding, release readiness | `GateRunnerPort`, `CIStatusPort`, `SafetyPolicyPort`, `ClaimEvidenceBinderPort`, `CriterionVerdict`, `Evidence` | Product center and correctness floor. Keep it fail-closed; autonomy expands only when this stays trustworthy. |
| Work Graph and Loop Engine | BC3 Loop | BDD intent, br beads, work selection, slice plans, RPI, evolve, closeout, convergence | `shape_intent`, `persist_intent`, `plan_slices`, `execute_slice`, `execute_wave`, `record_evidence`, `steer_goal`, `LoopReaderPort`, `LoopWriterPort`, `CloseoutPort`, `ConvergenceCheckPort`, `WorkItem`, `Slice`, `Wave` | Core route. One loop body, one inner tick, one in-session driver. |
| Skill / Claim Factory | BC4 Factory | skill/workflow admission, skill quality, claim governance, reusable standards, generated registry sources | `SkillCatalogPort`, `SkillScorerPort`, `FactoryAdmissionPort`, `ClaimEvidencePort`, skill/workflow front-door checks | Core governance, but sequence after route-critical validation and corpus work. Avoid creating new peer loops. |
| Runtime and Workspace Adapters | BC5 Runtime | harness packages, `ao` CLI driving adapters, git/workspace/session/operator shell, installer and release surfaces | `HarnessPort`, `WorkspacePort`, `OperatorPort`, `EventBusPort`, `GitPort`, `IssueTrackerPort`, runtime skill bundles | Adapter layer. Keep only surfaces that make the core loop usable across real harnesses. |
| Orchestration Boundary | BC6 Orchestration | substrate dispatch, backend selection, agent messaging/wakeups, swarm coordination, Factory driver boundary | `OrchestrationPort`, `SwarmDispatchPort`, `AgentMailPort`, `ConvergencePort`, `WorkSpec`, `SelectionTrace` | Adapter/substrate boundary. AgentOps owns the loop invariants; the substrate owns when/where/who-supervises. |

## Bead Alignment

This is a routing snapshot, not a mutation. Verify live status with read-only
`br` before execution.

| Disposition | Beads | Component | Reason |
|---|---|---|---|
| KEEP - route-critical | `ag-tkief`, `ag-ledger-cache-reconcile-uk3us`, `ag-verify-close-landed-sweep-33vh1` | Work Graph and Loop Engine | Ledger truth must be trustworthy before agents can decide what is open, closed, or landed. |
| KEEP - route-critical | `ag-wuz9f`, `ag-o6dya`, `ag-3l86`, `ag-8rns`, `ag-yv25`, `ag-c3c7` | Verification Membrane | Red trunk, local release authority, and admission/verdict failures block "verified done." |
| KEEP - route-critical | `ag-bl7qf`, `ag-yk1ff`, `ag-rcgw4`, `ag-tkief` | Work Graph and Loop Engine | br/offline tracker migration is not bookkeeping cleanup; it removes fail-open delivery risk. |
| KEEP - measured moat | `ag-98g1a`, `ag-8p8o`, `ag-epgk`, `ag-aekxj`, `ag-2i5jg`, `ag-k7tq9`, `ag-lhu0` | Context Corpus | The corpus claim must become measured retrieval/injection/yield evidence, not file presence. |
| KEEP - governance | `ag-3fp54`, `ag-cuhad`, `ag-paai`, `ag-czzf`, `ag-jc4z`, `ag-vb3q` | Skill / Claim Factory | New skills/workflows need a governed front door, but this should reduce surfaces rather than expand them. |
| KEEP AS ADAPTER, then collapse | `ag-zqtzc`, `ag-am-read-path-resilience-1anjd`, `ag-cross-runtime-comms-gateway-sz4c8`, `ag-am-sibling-deps-build-gate-t1zqq`, `ag-luklh`, `ag-am-archive-perf-date-test-6989t` | Orchestration Boundary / Runtime | AM/comms reliability matters only to keep agents coordinated; after the incident, collapse to the one real blocker and close stale siblings. |
| KEEP AS ADAPTER | `soc-3csze`, `ag-s3z`, `ag-ywk`, `ag-n90tx`, `ag-a81zu` | Runtime and Workspace Adapters | Installer/runtime proof matters when it proves real first value; do not let it become another product core. |
| DEFER | `ag-v1xk`, `ag-egpu`, `ag-egpu.1`, `ag-dz9xc`, `ag-xanm`, `ag-2jp7l`, `ag-w0wr2` | Orchestration Boundary / future substrate | Workload objects, reconcile controllers, eGPU, peercred, and meta-orchestration are scale/adversarial capabilities. They do not outrank the route-critical loop. |
| DEFER or close-candidate | `ag-o2tc`, `ag-d6obl`, `ag-h1l7y`, `ag-v4wo`, `ag-w65c`, `ag-4oj9*` | Legacy tracker/substrate | Dolt/bd hardening is historical unless it is needed to archive or complete the br migration. |
| TRIM from AgentOps priority | `ag-i77w`, `ag-64hr`, `ag-lvtc`, `ag-4cfz`, `ag-4xgl*`, `ag-real*`, `ag-yqhs`, `ag-ecy`, `ag-3zy`, `ag-jbre*`, `ag-b4p*`, `ag-rtvk`, `soc-likqx`, `soc-mtzf`, `soc-9jaa` | External domains | Relay, business/GTM, family support, career prep, and personal content may be valid work, but they are not AgentOps product components. Track them in their own lane/view. |

## Trim Rules

**Keep** work that advances one of these:

- autonomous code validation and agent-output proof;
- autonomous goal -> verified done;
- fail-closed validation and release authority;
- br-backed work graph truth;
- measured corpus retrieval/injection/yield;
- repo-local corpus durability and portability;
- runtime install/use proof for the core loop.

**Defer** work when it is real but not needed for the route:

- hosted or always-on control planes;
- workload-object/controller APIs;
- adversarial multi-tenant Linux gates;
- eGPU/specialized compute;
- broad AM/ATM/NTM gateways after the active incident is contained;
- skill polish that lacks a measured failure or route-critical gate.

**Trim or move out of the AgentOps product lane** when it is:

- an external product domain such as relay, personal AI, content, GTM, or
  family support;
- a duplicate peer loop instead of a step/profile of RPI/evolve;
- stale bd/Dolt infrastructure after br migration;
- a generated surface hand-edit;
- a public claim not backed by current evidence.

## Default Sequencing

1. Reconcile ledger truth and landed/open drift: `ag-tkief`,
   `ag-ledger-cache-reconcile-uk3us`, `ag-verify-close-landed-sweep-33vh1`.
2. Restore validation/release health: `ag-wuz9f`, `ag-o6dya`, `ag-3l86`.
3. Prove the corpus path mechanically: `ag-98g1a` before `ag-8p8o` / `ag-epgk`.
4. Govern new factory entries: `ag-3fp54` / `ag-cuhad`.
5. Only then resume broad adapters, substrate, UI, or product-market lanes.

## Generated And Curated Surfaces

The generated maps and registries remain derived artifacts. Do not hand-edit
`registry.json`, `docs/contracts/context-map.md`,
`docs/reference/agentops-skill-domain-map.md`, `cli/docs/COMMANDS.md`, or
`docs/cli-surface.*`. Change their sources and run the generator/check path
instead.

`docs/SKILLS.md` and `skills/SKILL-TIERS.md` are curated routers/ledgers, not
pure generated projections. Edit their narrative rows deliberately, do not add
hard-coded skill counts to `docs/SKILLS.md`, and let `scripts/sync-skill-counts.sh`
own only the count-bearing markers that remain in `skills/SKILL-TIERS.md` and
other count surfaces.
