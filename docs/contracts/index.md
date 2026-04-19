# Contracts

Every inter-component boundary in AgentOps is a **contract** — a versioned,
validatable interchange format. These are the interchange files used between
skills, the runtime, and external integrations.

<div class="grid cards" markdown>

-   :material-play-box: **[Repo Execution Profile](repo-execution-profile.md)**

    ---

    Repo-local bootstrap, validation, tracker, and done-criteria contract for
    autonomous orchestration.

-   :material-robot: **[Autodev Program](autodev-program.md)**

    ---

    Repo-local operational contract for bounded autonomous development.

-   :material-database: **[RPI Run Registry](rpi-run-registry.md)**

    ---

    RPI run registry specification.

-   :material-format-list-numbered: **[Next-Work Queue](next-work.schema.md)**

    ---

    Contract for `.agents/rpi/next-work.jsonl`.

-   :material-magnify: **[Finding Registry](finding-registry.md)**

    ---

    Canonical intake-ledger contract for reusable findings.

-   :material-hammer-wrench: **[Finding Compiler](finding-compiler.md)**

    ---

    V2 promotion ladder, executable constraint index, and lifecycle rules.

-   :material-hook: **[Hook Runtime Contract](hook-runtime-contract.md)**

    ---

    Canonical event mapping across Claude, Codex, and manual runtimes.

-   :material-console: **[Headless Invocation Standards](headless-invocation-standards.md)**

    ---

    Required flags, tool allowlists, and timeout strategy for non-interactive
    Claude/Codex execution.

-   :material-api: **[Codex Skill API](codex-skill-api.md)**

    ---

    Source of truth for Codex runtime skill structure, frontmatter, discovery
    paths, and multi-agent primitives.

-   :material-cube-outline: **[Context Assembly Interface](context-assembly-interface.md)**

    ---

    Interface contract for adaptive context assembly and token budgeting.

-   :material-shield-star: **[Session Intelligence Trust Model](session-intelligence-trust-model.md)**

    ---

    Artifact eligibility contract for runtime context assembly.

-   :material-moon-waning-crescent: **[Dream Run](dream-run-contract.md)**

    ---

    Process model, locking, keep-awake, and artifact floor for private
    overnight runs.

-   :material-file-chart: **[Dream Report](dream-report.md)**

    ---

    Canonical `summary.json` and `summary.md` schema for Dream outputs.

-   :material-shield-lock: **[MemRL Policy](memrl-policy-integration.md)**

    ---

    AO-exported deterministic MemRL policy contract for Olympus hooks.

-   :material-swap-horizontal: **[OL-AO Bridge](../ol-bridge-contracts.md)**

    ---

    Olympus-AgentOps interchange formats.

-   :material-alert-octagon: **[Scope Escape Report](scope-escape-report.md)**

    ---

    Structured template for agent scope-escape reporting.

-   :material-clipboard-check: **[Dispatch Checklist](dispatch-checklist.md)**

    ---

    Standard references for agent dispatch prompts.

-   :material-account-multiple-check: **[Swarm Evidence](swarm-evidence.md)**

    ---

    Permissive shape covering all historical swarm result files.

</div>
