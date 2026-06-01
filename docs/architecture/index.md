# Architecture

AgentOps is built from a small set of orthogonal components that compose into the full Research → Plan → Implement → Validate loop. The architecture is opinionated where it matters (bookkeeping format, validation contracts, hook lifecycles) and permissive everywhere else (model choice, runtime, repo layout).

Read in this order if you're new:

1. **[How It Works](../how-it-works.md)** — start here for the mental model (Brownian Ratchet, context windowing, backends).
2. **[System Overview](../ARCHITECTURE.md)** — then zoom out to see where every component sits.
3. **[Primitive Chains](primitive-chains.md)** — then drill into the audited primitive set and lifecycle chains.

The rest are specialized references. Skim titles and jump in when a topic becomes relevant.

<div class="grid cards" markdown>

-   :material-cogs: **[How It Works](../how-it-works.md)**

    ---

    Brownian Ratchet, Ralph Wiggum Pattern, agent backends, hooks, context
    windowing.

-   :material-factory: **[Software Factory](../software-factory.md)**

    ---

    Explicit automation surface for briefings, RPI flows, and
    operator-controlled closeout.

-   :material-sitemap: **[System Overview](../ARCHITECTURE.md)**

    ---

    Full system design and component overview.

-   :material-pipe: **[Primitive Chains](primitive-chains.md)**

    ---

    Audited primitive set, lifecycle chains, and terminology drift ledger.

-   :material-link-variant: **[Codex Hookless Lifecycle](codex-hookless-lifecycle.md)**

    ---

    Runtime-aware lifecycle fallback for Codex when hooks are unavailable.

-   :material-shield-check: **[PDC Framework](pdc-framework.md)**

    ---

    Prevent, Detect, Correct quality control approach.

-   :material-alert-circle: **[FAAFO Alignment](faafo-alignment.md)**

    ---

    FAAFO promise framework for vibe-coding value.

-   :material-close-octagon: **[Failure Patterns](failure-patterns.md)**

    ---

    The 12 failure patterns reference guide.

-   :material-tune: **[Command Customization](ao-command-customization-matrix.md)**

    ---

    External command dependencies and customization policy tiers.

-   :material-hexagon-multiple: **[Ports & Adapters](#ports-adapters-hexagonal)**

    ---

    Hexagonal design: 14 typed ports, their `cmd/ao` adapters, and the
    five bounded contexts.

</div>

## Ports & Adapters (Hexagonal) {: #ports-adapters-hexagonal }

The `ao` CLI follows a hexagonal (ports-and-adapters) design. Domain logic
depends only on small, typed Go interfaces ("ports"); the concrete bindings
to the filesystem, the LLM, git, CI, and the rest of the outside world live
in adapters that are wired in explicitly via dependency-passing (there is no
DI container).

- **Ports (14)** live in [`cli/internal/ports/`](https://github.com/boshu2/agentops/tree/main/cli/internal/ports) —
  `CorpusReaderPort`, `CorpusWriterPort`, `CitationPort`, `ClaimEvidencePort`,
  `ClaimEvidenceBinderPort`, `EventBusPort`, `FactoryAdmissionPort`,
  `FindingCompilerPort`, `GateRunnerPort`, `HarnessPort`, `LoopReaderPort`,
  `LoopWriterPort`, `OperatorPort`, and `CIStatusPort`. Each port ships an
  `InMemoryX` test double with a compile-time
  `var _ XPort = (*InMemoryX)(nil)` drift guard.
- **Adapters** live alongside the commands as `cli/cmd/ao/*_adapter.go`
  (for example `corpus_reader_adapter.go`, `citation_port_adapter.go`,
  `gate_runner_adapter.go`, `harness_adapter.go`). They translate a port
  interface into a real-world binding.
- **Bounded contexts (5)** group the ports by domain: **Corpus**,
  **Validation**, **Loop**, **Factory**, and **Runtime**.

For the authoritative catalog — per-port status, adapter delivery state, and
the file-triplet shape each BC follows — see the contract
[BC Ports Inventory](../contracts/bc-ports-inventory.md).
