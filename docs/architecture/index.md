# Architecture

AgentOps is built from a small set of orthogonal components that compose into the full Research → Plan → Implement → Validate loop. The architecture is opinionated where it matters (bookkeeping format, validation contracts, explicit bootstrap) and permissive everywhere else (model choice, runtime, repo layout).

Read in this order if you're new:

1. **[Intent → Validated Code](intent-to-validated-code.md)** — what the product is: full flow via skills.
2. **[Skills Matrix](../skills-matrix.md)** — every skill on the loop.
3. **[Codebase Overview](codebase-overview.md)** — consolidated repo map (humans and agents).
4. **[Operating Loop](operating-loop.md)** — how work flows (primary navigation / discipline).
5. **[AgentOps 3.0](../3.0.md)** — north star doctrine.
6. **[How It Works](../how-it-works.md)** — mental model (Brownian Ratchet, context windowing, backends).
7. **[Component Map](component-map.md)** — route product components and trim/defer decisions.
8. **[Intent-to-Loop Hexagon](intent-to-loop-hexagon.md)** — ports/adapters for one turn.

The rest are specialized references. Skim titles and jump in when a topic becomes relevant.

<div class="grid cards" markdown>

-   :material-map: **[Codebase Overview](codebase-overview.md)**

    ---

    Consolidated repo map: BCs, directories, active waist, registries,
    gates, footguns, reading order.

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

-   :material-sitemap-outline: **[Component Map](component-map.md)**

    ---

    DDD + hexagonal routing map for components, open beads, and trim/defer decisions.

-   :material-hexagon-multiple: **[Intent-to-Loop Hexagon](intent-to-loop-hexagon.md)**

    ---

    Process-level ports and adapters from BDD intent through evidence ratchet.

-   :material-swap-horizontal: **[Fungibility Charter](fungibility-charter.md)**

    ---

    AgentOps 3.0's six doctrinal commitments — fungible by default, specialized when you opt in.

-   :material-stairs: **[Autonomy Ladder](autonomy-ladder.md)**

    ---

    The explicit L0–L4 dispatch progression — named promotion gates, auto-downgrade, and the Red Button — with the deterministic enforcement boundary held constant at every rung.

-   :material-school: **[Behavior-Shaping Environment](behavior-shaping-environment.md)**

    ---

    The *why* beneath the loop: AgentOps as an operant-conditioning system — arrange antecedents, reinforce or stop the agreed behaviors.

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

</div>
