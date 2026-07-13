# Ports and Adapters

> One-page overview of AgentOps's runtime hexagonal seam. Companion to [ADR-0001: Adopt DDD + Hexagonal Architecture](../adr/ADR-0001-ddd-hexagonal-adoption.md) and [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md).

AgentOps adopts Alistair Cockburn's 2005 *Hexagonal Architecture* (a.k.a. Ports and Adapters) as the load-bearing structural style for the Go runtime. The inner hexagon — the domain — is the only thing the rest of the system is allowed to depend on. Everything that talks to a runtime, a filesystem, a tracker, an LLM, or a CI workflow is an adapter, plugging into a port the domain declares.

## Inner hexagon: domain

The domain lives at `cli/internal/domain/`. Its first inhabitant is the `ExecutionPacket` aggregate root at `cli/internal/domain/packet/`. The aggregate enforces four invariants in `invariants.go`:

- **I1 — `ErrPlanPathEmpty`.** `plan_path` is non-empty.
- **I2 — `ErrInvalidComplexity`.** `complexity ∈ {fast, standard, full}`.
- **I3 — `ErrInvalidTestLevel` / `ErrEmptyTestLevels`.** `test_levels` is non-empty and every entry is one of `{L0, L1, L2, L3}`.
- **I4 — `ErrEmptyProvenance`.** `provenance.created_at` and `provenance.source` are non-empty.

Invariants are exercised by `pgregory.net/rapid` property tests in `aggregate_property_test.go`. The domain package may not import from any other `cli/internal/*` subpackage — verified mechanically (see ADR-0001).

## Primary (driving) adapters

Primary adapters drive the domain from the outside: a human, an agent, or a workflow calls into them, and they translate the call into a domain operation. Categories currently in use:

- **CLI commands** in `cli/cmd/ao/` — every `ao <verb>` is a driving adapter.
- **Skills** in `skills/` — instruction adapters such as `plan`, `discovery`,
  `premortem`, and `validate` that shape how an agent drives the runtime.
- **Optional MCP adapters** — Model Context Protocol entry points available in
  archive/optional profiles; `ao mcp` is not on the default CLI surface.
- **Whole-loop execution** — `rpi` and `evolve` skills can drive an iteration;
  optional out-of-session scheduling belongs to an operator-selected substrate,
  not an AgentOps daemon.
- **CI gates** — `scripts/*.sh` and `.github/workflows/validate.yml` jobs that drive validation against the same domain types they would in interactive runs.

> **The agent at two altitudes (reconciliation).** Here the agent is on the **driving** side: it calls
> *into* this domain hexagon through the CLI, skill, optional MCP, and whole-loop driving adapters above. At
> the *process* control-plane altitude — [the-agent-factory.md](the-agent-factory.md) — the same agent is
> the **data-plane workload** (the AgentPod / actuator) that the controller schedules and gates. Both are
> true: the agent *drives* the code-level domain and *is driven by* the process-level controller. See
> [the-agent-factory.md → the agent at two altitudes](the-agent-factory.md) for the factory-altitude view.

## Secondary (driven) adapters

Secondary adapters are driven *by* application code through port interfaces.
Concrete packages live under `cli/internal/adapters/`; current examples include
filesystem corpus and packet storage, Git workspaces, CI status, Agent Mail,
review workers, runtime surfaces, and tracker adapters. Compile-time assertions
bind several of them to their ports—for example `storage_fs` to
`PacketRepository`, `corpus_fs` to the corpus ports, `workspace_git` to
`WorkspacePort`, and the legacy `tracker_bd` adapter to `IssueTracker`.

The product-repo tracker is `br`, usually invoked through the resolved tracker
facade or local shell. The `tracker_bd` package remains an implemented legacy
adapter for substrate paths; its presence does not make `bd` this repository's
tracker.

## Ports

Port interfaces live at `cli/internal/ports/`. The exact current inventory is
**32 interfaces**: 29 use the normalized `*Port` suffix and three foundational
interfaces retain pre-normalization names:

- **`PacketRepository`** (`storage.go`) — abstracts ExecutionPacket persistence: save / load / load-latest.
- **`IssueTracker`** (`tracker.go`) — abstracts epic and issue creation; production bootstrap/handoff paths call `br` directly; `tracker_bd` remains a legacy driven adapter behind the port interface.
- **`LLMClient`** (`llm.go`) — abstracts model completion calls behind a provider-neutral `Complete(ctx, prompt, opts)` shape.

The six bounded-context owners for all 32 interfaces are canonical in
[`bounded-contexts.yaml`](../contracts/bounded-contexts.yaml) and projected in
the [component map](component-map.md). `check-bounded-contexts-drift.sh` fails
closed when a Go interface is unowned, multiply owned, or declared without an
implementation. New interfaces should use the `*Port` suffix; the three names
above remain inventoried compatibility exceptions.

## Hexagon diagram

```text
                     Primary (driving)
                ┌────────────────────────┐
   CLI       → │                          │ ← agent skills
   MCP       → │      ╔═══════════╗       │ ← skills / loop drivers
                 │      ║   domain  ║       │
                 │      ║  (inner)  ║       │
   CI gates  → │      ╚═══════════╝       │ ← scripts/*.sh
                 │            ▲              │
                 └────────────│──────────────┘
                              │  ports
                              ▼
              ┌──────────────────────────────┐
              │ Secondary (driven) adapters  │
              │ storage_fs · corpus_fs · Git │
              │ tracker · reviewers · MCP …  │
              └──────────────────────────────┘
```

The diagram is intentionally ASCII / `text` fenced. No raster images: agent-context-friendly, mkdocs-strict-safe, and diffable.

## How to add a new adapter

A new driven adapter is a four-step recipe.

1. **Declare the port interface** in `cli/internal/ports/<name>.go`. Keep it small (1–3 methods). Document each method in godoc. If a second implementation is not plausible within the next epic, defer.
2. **Create the adapter package** at `cli/internal/adapters/<name>_<flavor>/` (e.g., `storage_fs`, `tracker_beads`, `llm_claude`). The adapter only imports `cli/internal/domain/...` and `cli/internal/ports`; it must not import `cli/cmd/...` or any sibling adapter.
3. **Add a compile-time interface check** at the top of the adapter file:

   ```go
   var _ ports.<PortName> = (*Adapter)(nil)
   ```

   This catches signature drift the moment the port interface changes.
4. **Write L2 tests** against `t.TempDir()` or a real-ish backing store (a fake server, a temp git repo, a recorded LLM transcript). L2 first, L1 always — per [Go conventions](../standards/golang-style-guide.md).

## References

- Alistair Cockburn, 2005. *Hexagonal Architecture* — <https://alistair.cockburn.us/hexagonal-architecture/>.
- [ADR-0001: Adopt DDD + Hexagonal Architecture](../adr/ADR-0001-ddd-hexagonal-adoption.md) — the decision record this page operationalizes.
- [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) — process-level ports/adapters from BDD intent through validation and ratchet evidence.
- [`PRACTICE-REGISTRY.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — canonical slugs for `ddd-bounded-context` and `hexagonal-architecture`.
- [Context Map](../contracts/context-map.md) — auto-generated bounded-context view of all skills by hexagonal role.
