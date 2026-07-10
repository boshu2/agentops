# AgentOps-native skill adoption specification

## Intent

Turn the most-used, still-useful behaviors in the external skill corpus into
AgentOps-owned contracts while deleting duplicate or weak wrappers. The result
must strengthen the existing codebase-recon capability and make NTM + Agent Mail
a first-class BC6 factory substrate for pawl and general software-factory roles,
beside (and optionally callable from) the GC adapter.

## Selected surfaces

| Surface | Disposition | Bounded context | Dependencies |
|---|---|---|---|
| `idea-genie` | create | BC3 Loop/domain | `research`, `behavior-first-planning` |
| `dueling-idea-genies` | create | BC2 Validation/judgment | `idea-genie`, `council` |
| `codebase-recon` | improve/formalize the existing recurring capability | BC1 Corpus/supporting | `research`, `validate`, `doc` |
| `pattern-mining` | create from the highest-value unowned method | BC1 Corpus/supporting | `research`, `operationalize`, `validate` |
| `ntm` | improve as the low-level NTM adapter manual | BC6 Orchestration/supporting | live NTM contract |
| `agent-native` | improve as the substrate-neutral worker lifecycle owner; merge `using-atm` into it | BC5 Runtime/driving adapter | `ntm`, `agent-mail`, loop skills, `AgentWorker` |
| `pawl-review` | create as the generalization/rename target of `pre-land-refuters` | BC2 Validation/driving adapter | `ao pawl`, cold reviewer adapters, optional NTM review lane |
| `agent-mail` | improve as the coordination adapter used by factories | BC6 Orchestration/supporting | live `am` contract, `br` truth |
| `using-gc` | improve with optional worker/review-lane composition | BC6 Orchestration/driving adapter | `gc-membrane`, optional `agent-native` / `pawl-review` |
| `gc-membrane` | converge its emitted verdict on the canonical schema before composition | BC2 Validation/guard adapter | `pawl-verdict.v1` |
| `operationalize` | repair retired downstream skill references | BC4 Factory | live CLI + post-mortem owners |

All new source skills use parity-only Codex twins unless a concrete runtime
divergence is proven. The operator authorized autonomous tier/dependency choices;
the table above is the frozen choice for this arc.

## Non-duplicating artifact boundaries

| New skill | Owns | Explicitly does not own |
|---|---|---|
| `idea-genie` | `idea-portfolio.v1`: evidence, overlap reconciliation, candidate scenarios, saturation/no-new-work termination | discovery's BDD intent packet, beads, plan, or selection gate |
| `dueling-idea-genies` | `idea-challenge.v1`: sealed perspectives, cross-review, dissent, refutations, proposed synthesis | council judging or `ao plan-pawl decide` PASS/REDO/BLOCKED logic |
| `codebase-recon` | a recon manifest + evidence-bounded report/delta pack | final PASS/WARN/FAIL verdict or code edits |
| `pattern-mining` | pattern/hypothesis artifact with exemplars and holdout result | automatic skill/gate/library creation; `operationalize` owns routing |
| `pawl-review` | immutable reviewer request and canonical lane results | deterministic panel decision, verdict writing, merge, or fixes |

The first two names are the operator-selected AgentOps vocabulary. They do not
restore the copied fixed-count workflows: they are thin leaf adapters around
new versioned artifacts and existing discovery/plan-pawl owners.

## Port boundaries

| Port behavior | Owner | Adapter |
|---|---|---|
| start and supervise an agent worker | existing `AgentWorker` contract | new real NTM robot adapter; native/GC adapters remain swappable |
| run one independent reviewer lane | new narrow `ReviewLanePort` | worker-backed adapter over NTM now; cold/native/GC adapters later |
| coordinate identities, leases, messages, ACKs | new narrow `AgentMailPort` | real Agent Mail CLI adapter after live capability discovery |
| execute a bead slice | BC3 Loop | `$rpi` / `$crank` inside a pane |
| judge an output | BC2 Validation | `$pawl-review` / `ao pawl` oracle lane |
| bind a verdict | verification membrane | `pawl-verdict.v1` + provenance ledger |
| close a GC quest | GC membrane adapter | `packs/agentops-membrane` |

GC may implement the same worker and review-lane ports for a bounded sub-job, but
no port depends on a concrete substrate. NTM and GC remain independently operable
and operator-selected. The legacy backend-selection-only `OrchestrationPort` is
not expanded into a lifecycle owner.

## Skill mesh

| Consumer entry point | Delegates to | Trigger boundary |
|---|---|---|
| `discovery` | `idea-genie` | open-ended opportunity shaping before research/plan |
| `plan`, `discovery` | `dueling-idea-genies` | contested one-way-door strategy before decomposition |
| `research`, `reverse-engineer`, `doc`, `validate` | `codebase-recon` | reusable repository mental model, delta recon, or bounded audit pack |
| `research`, `operationalize`, `refactor` | `pattern-mining` | repeated implementation shape may deserve promotion |
| `automation-shape-routing`, `swarm`, `crank`, `evolve` | `agent-native` | persistent substrate-neutral agent lifecycle over a bead graph |
| `crank`, `validate`, `push`, `using-gc` | `pawl-review` | independent reviewer execution before an admission door |
| `agent-native` | `ntm`, `agent-mail` | NTM-specific pane lifecycle plus optional multi-lane coordination |
| `pawl-review` | `ao pawl`, optional NTM/GC lane adapters | immutable reviewer request/result and evidence handoff |
| `using-gc` | `agent-native` / `pawl-review` through ports | explicitly delegated bounded worker or review lane inside a GC quest |

The entry points contain only routing criteria and artifact handoffs. Leaf
workflows remain single-owned by the delegated skills. Frontmatter dependencies,
`context_rel`, the disposition ledger, curated routers, and generated maps must
agree with this table.

## Generated graph contract

Extend the existing catalog and `ao skills graph` instead of inventing a second
graph store:

1. `skills/*/SKILL.md` frontmatter is the only graph source.
2. `metadata.dependencies` supplies directed execution/delegation edges.
3. `metadata.graph_root: true` is the explicit exceptional zero-inbound marker;
   `user-invocable` does not make an orphan reachable, and every new capability
   in this arc must have an inbound edge regardless of root flags.
4. `context_rel` supplies typed DDD relationship edges.
5. `consumes` / `produces` remain artifact-flow metadata and are rendered as a
   separate view, never confused with execution dependencies.
6. `skills/catalog.json` is the machine-readable generated projection.
7. Existing `docs/contracts/context-map.md` is extended as the generated human
   projection with dependency, relationship, entry-point, zero-inbound, and
   topology diagnostics. No second graph document or parser is added.
8. `ao skills graph` renders from the catalog; JSON output is suitable for
   Graphify or other explorers.
9. Existing catalog/context-map generators and `scripts/regen-all.sh` write the
   projections and `--check` verifies them.
   Dangling dependency/context targets and dependency cycles are blocking.

This cannot become a graveyard because there is no second graph store and no manual graph content to
remember: a skill edit either regenerates successfully or the normal drift gate
fails.

## Migration rules

1. Add failing acceptance tests before source skill creation.
2. Create original behavior contracts; do not run skill-builder's
   `absorb-external` mode.
3. Route discovery open-ended work through `idea-genie`; route contested
   strategic choices through `dueling-idea-genies`.
4. Keep the name `codebase-recon`; use prior recon packs as AgentOps-owned
   evidence and require delta mode.
5. Merge `using-atm` into `agent-native`, distributing NTM-specific mechanics to
   `ntm`, reviewer execution to `pawl-review`, and coordination to `agent-mail`.
6. Merge `pre-land-refuters` into `pawl-review` while preserving the old trigger
   as an additive migration alias.
7. Preserve `ntm` as the live NTM adapter manual and `agent-mail` as the
   coordination owner; general lifecycle belongs to `agent-native`.
8. Add optional GC worker/review-lane composition without automatic substrate
   routing, and make GC verdict output conform to the canonical pawl schema.
9. Wire every new skill into the existing entry-point mesh without copying leaf
   procedures into callers.
10. Regenerate every derived registry, Codex artifact, and command/skill map.
11. Generate and topology-check the complete skill graph from frontmatter.
12. Require focused tests, regen checks, fast gate, and an independent membrane
    verdict before landing.

## Evidence for done

- Every scenario in `behaviors.md` maps to a passing acceptance test.
- The official external corpus and local companions have one explicit
  disposition each in the audit artifact.
- `skills/using-atm/` and its Codex twin are absent; the disposition ledger names
  `agent-native` as the merge target and records split behavior destinations.
- `pre-land-refuters` resolves to `pawl-review`; no completion claim loses its
  independent-review entry point.
- No active skill/router/profile calls the substrate ATM.
- `agent-native` covers general worker lifecycle; `pawl-review` covers reviewer
  execution; both can use NTM and Agent Mail without owning those binaries.
- GC docs cover optional composition and preserve operator choice.
- A GC CONFIRMED artifact passes the canonical pawl-verdict schema and names
  existing evidence files.
- Every new skill has an inbound entry-point route and declared outbound handoff.
- The generated skill graph covers every live skill, has no dangling edges or
  dependency cycles, and is reproducible through `make regen-all`.
- Planted fixtures prove duplicate-node, dangling-target, dependency-cycle,
  stale-projection, and unreachable-non-root failures.
- `make regen-check`, focused Bats, Go tests for touched orchestration code (if
  any), and `ao gate check --fast --scope head` pass.
- A commit-bound pawl verdict exists and the coherent bead arc is pushed to main.
