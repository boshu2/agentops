# The Spine — AgentOps from first principles

> First-principles teardown, 2026-05-28. Companion: [slop-map.md](slop-map.md), [strangler-plan.md](strangler-plan.md).
> Method: three parallel codebase surveys (Go packages, doc/skill trees, CI machinery) against the thesis already recorded in operator memory.

## The thesis (already written; this just enforces it)

Operator memory already states what AgentOps *is*. The teardown's job is to make the artifact match the thesis, not to invent a new one:

1. **AgentOps IS a Gas City distribution/config** — it has no sovereign core of its own. (`project_agentops_is_gascity_reference_config`)
2. **Skills are the portable agent runtime.** `rpi` = inner loop, `evolve` = outer loop, fractal all the way down. (`project_fractal_loops_skills_runtime`)
3. **The corpus / context-compiler is the moat.** Context is the artifact handed off at every loop edge; compiling and compounding it is the only durable advantage. (`project_context_compiler_thesis`)
4. **Hookless-first.** Skills → CLI ports → adapters → (thin) hooks → CI. CI is the authoritative gate. (`project_hookless_first_architecture`)

Everything below is judged by one question: **does it serve one of those four, or does it exist to keep a redundant projection in sync?**

## The irreducible core (what survives a teardown)

### Tier 0 — the runtime spine (keep, protect, never delete)
| Surface | Lines | Why it is the spine |
|---|---|---|
| `skills/` (75 skills) | ~83k md | THE product. The portable runtime. Single source of truth for behavior. |
| `cli/cmd/ao` core verbs: `rpi`, `evolve`, `inject`, `compile`, `goals`, `doctor` | — | The loop drivers + the context compiler + the fitness gate + the repair port. |
| `cli/internal/rpi` | 9,073 | Inner-loop engine (discovery → crank → validate). |
| `cli/internal/goals` | 12,183 | Fitness function. `ao goals measure` is the gate evolve steers on. |
| `cli/internal/ratchet` | 13,732 | Locks in shaped behavior — the mechanism that makes the flywheel compound. |
| `cli/internal/doctor` | 9,159 | Repair port. Self-healing CLI. |
| `cli/internal/types`, `config`, `storage` | ~3k | Core hubs (types: 122 importers). Foundational. |
| corpus/context surface: `inject`, `compile`, `harvest`, `forge`, `maturity`, `knowledge` | — | The moat. Compiles + compounds context across loop edges. |

### Tier 1 — legitimate features (keep; not slop)
`ratchet`, `pool`, `search`, `context`, `feedback`, `reconcile`, `eval`, `sessions`, `worktree`, `codex` command. Coherent, imported, tested. These are the body around the spine.

### What the spine does NOT need
- **No sovereign daemon.** There is no `agentopsd` binary in `cli/` — only diagnostic *hints* in `doctor/fix_bridges.go`. The thesis (#1) says AgentOps borrows Gas City's core. Correct as-is; just delete the dangling GC-config compat (`internal/gascity`, `internal/bridge/gc.go`) now that the bridge is severed.
- **No second skill runtime.** `skills-codex/` is a hand-maintained 70k-line parallel copy of `skills/`. The spine has ONE runtime; cross-runtime is a *projection*, not a second source of truth.
- **No hand-maintained inventory maps.** `registry.json`, `catalog.json`, `skill-domain-map.json`, the SKU catalog, `cli/docs/COMMANDS.md`, `AGENTS-CI.md` are all *derived* from the spine. They must be generated-on-read or build-time artifacts — never checked-in facts that need a drift gate.

## The one-sentence spine

> **AgentOps = a Gas City config whose payload is a single tree of skills (the agent runtime), a thin `ao` CLI that drives the rpi/evolve loops and compiles the context corpus, and a fitness function (`goals`) that CI gates — everything else is a projection of those three and must be derived, not maintained.**

## The core distortion to remove

The codebase is not slop-*code*. It is **over-projected**: the same facts (the skill set, the command set, the practice list, the CI policy) are written down in 3–6 places, each place needs a generator to fill it, and each generator needs a CI drift-gate to police it. That apparatus — ~15 of 65 CI jobs, ~8 generator scripts, the entire `skills-codex` tree, and 6 dead Go packages — is the slop. It exists to serve consistency between redundant copies, not to serve any of the four thesis points.

**Collapse the projections to single sources of truth and the generators + the gates that police them evaporate together.** That is the rebuild.
