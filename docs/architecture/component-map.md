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

| Component | Bounded context | Canonical responsibility | Implemented ports | Target ports | Product posture |
|---|---|---|---|---|---|
| Context Corpus | BC1 Corpus | Capture, retrieve, cite, and promote local knowledge and evidence. | `CorpusReaderPort`, `CorpusWriterPort`, `ContextCompilerPort`, `CitationPort`, `FindingCompilerPort`, `FreshnessPolicyPort`, `FrontmatterCodecPort`, `WikiIndexPort` | — | Supporting context layer; measured uplift remains unproven. |
| Verification Membrane | BC2 Validation | Judge whether plans, code, docs, dependencies, and releases satisfy declared acceptance. | `GateRunnerPort`, `CIStatusPort`, `SafetyPolicyPort`, `ClaimEvidenceBinderPort`, `ReviewLanePort` | `ScenarioRunnerPort` | Product center and correctness floor. |
| Work Graph and Loop Engine | BC3 Loop | Shape intent, execute vertical slices, record evidence, and steer repair to verified done. | `LoopReaderPort`, `LoopWriterPort`, `HypothesisLedgerPort`, `ConvergenceCheckPort`, `CloseoutPort`, `FindingRecurrenceReducerPort`, `PacketRepository` | `WorkSelectorPort` | Core route; one ordered operating loop. |
| Skill / Claim Factory | BC4 Factory | Build, audit, package, and govern reusable skills, workflows, and claims. | `ClaimEvidencePort` | `SkillCatalogPort`, `SkillScorerPort`, `FactoryAdmissionPort` | Governance layer, not a peer execution loop. |
| Runtime and Workspace Adapters | BC5 Runtime | Adapt the core loop to harnesses, workspaces, shells, installers, and operator machines. | `HarnessPort`, `WorkspacePort`, `OperatorPort`, `EventBusPort`, `IssueTracker`, `LLMClient` | `GitPort`, `AgentWorkerPort` | Swappable adapters around the core. |
| Orchestration Boundary | BC6 Orchestration | Dispatch and coordinate whole loop units across explicitly selected substrates. | `OrchestrationPort`, `AgentMailPort`, `PreflightPort`, `VerifyPort`, `SubstrateProbePort` | `SwarmDispatchPort`, `ConvergencePort` | Optional substrate boundary; never implicit. |

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
