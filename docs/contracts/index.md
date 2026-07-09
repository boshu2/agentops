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

-   :material-call-merge: **[AO / MTO Seam](ao-mto-seam.md)**

    ---

    Reduction contract separating the lean AO image from the outer MTO factory.

-   :material-database: **[RPI Run Registry](rpi-run-registry.md)**

    ---

    RPI run registry specification.

-   :material-lan-connect: **[Remote Compute](remote-compute.md)**

    ---

    Product-neutral RemoteTarget, RemoteSession, command ledger, recovery, and
    Remote execution contract (RETIRED 2026-06-11 — GasCity removed; out-of-session substrate is NTM + MCP Agent Mail).

-   :material-clipboard-pulse: **[Eval Environment](eval-environment.md)**

    ---

    Evaluation suite, run, scorecard, baseline, canary, and holdout contract.

-   :material-clipboard-text-clock: **[Eval Verdict Pipeline](eval-verdict-pipeline.md)**

    ---

    Verdict compiler pipeline from eval run manifests to learning utility and
    retirement signals.

-   :material-magnify-scan: **[Retrieval Comparison](retrieval-comparison.md)**

    ---

    Deterministic search-eval backend comparison, promotion thresholds,
    optional rerank behavior, and deferred vector/graph-store policy.

-   :material-clipboard-check-outline: **[Release Readiness](release-readiness.md)**

    ---

    8/10 release readiness score, SIL/VIL/HIL evidence, artifact manifest
    requirements, and HIL waiver policy.

-   :material-brain: **[MemRL Policy Schema](memrl-policy.schema.json)**

    ---

    Deterministic retry/escalation policy profile for memory-reinforcement
    feedback loops.

-   :material-format-list-numbered: **[Next-Work Queue](next-work.schema.md)**

    ---

    Contract for `.agents/rpi/next-work.jsonl`.

-   :material-magnify: **[Finding Registry](finding-registry.md)**

    ---

    Canonical intake-ledger contract for reusable findings.

-   :material-hammer-wrench: **[Finding Compiler](finding-compiler.md)**

    ---

    V2 promotion ladder, executable constraint index, and lifecycle rules.

-   :material-console: **[Headless Invocation Standards](headless-invocation-standards.md)**

    ---

    Required flags, tool allowlists, and timeout strategy for non-interactive
    Claude/Codex execution.

-   :material-clipboard-flow: **[Codex Task Packet](codex-task-packet.md)**

    ---

    Non-mutating Codex dispatch packet, auth guard, sandbox, stdin closure,
    timeout, resume, and run-receipt evidence contract.

-   :material-call-split: **[Codex Fanout Approval Packet](codex-fanout-approval-packet.md)**

    ---

    PerspectivePlan, SynthesisPacket, and ApprovalEdge contract for Fable-gated
    Codex discovery before bead creation.

-   :material-bullseye-arrow: **[Goal Design Artifacts](goal-design-artifacts.md)**

    ---

    Two-artifact packet contract for goal-design `intent.md` and `driver.md`
    files, including schema validation, digest integrity, and route-back rules.

-   :material-api: **[Codex Skill API](codex-skill-api.md)**

    ---

    Source of truth for Codex runtime skill structure, frontmatter, discovery
    paths, and multi-agent primitives.

-   :material-cube-outline: **[Context Assembly Interface](context-assembly-interface.md)**

    ---

    Interface contract for adaptive context assembly and token budgeting.

-   :material-vector-polyline: **[Skill Domain Map](skill-domain-map.md)**

    ---

    V0 DDD map assigning every shared skill to one primary skill domain with
    ports, artifacts, and adapters.

-   :material-call-split: **[Skill Ports and Adapters](skill-ports-and-adapters.md)**

    ---

    V0 skill-boundary vocabulary for inbound ports, outbound ports, adapters,
    context packets, and guard surfaces.

-   :material-clipboard-check-outline: **[Skill Lease Audit](skill-lease-audit.md)**

    ---

    V0 lease-on-life audit classifying every shared skill as keep, merge,
    split, retire, or unknown before any cut is attempted.

-   :material-shield-star: **[Session Intelligence Trust Model](session-intelligence-trust-model.md)**

    ---

    Artifact eligibility contract for runtime context assembly.

-   :material-file-chart: **[Dream Report](dream-report.md)**

    ---

    Canonical `summary.json` and `summary.md` schema for Dream outputs.

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
