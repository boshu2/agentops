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

-   :material-server: **[AgentOps Daemon](agentops-daemon.md)**

    ---

    Architecture boundary for `agentopsd`, the daemon ledger, job queue, local
    trust, projections, and migration from foreground command flows.

-   :material-factory: **[AgentOpsd Control Plane](agentopsd-control-plane.md)**

    ---

    Production control-plane contract for worker slots, worktree ownership,
    lifecycle telemetry, validation gates, yield, and operator status.

-   :material-shield-check: **[Factory Admission](factory-admission.md)**

    ---

    Daemon-owned work-order admission contract for fail-closed local factory
    pilots and RPI handoff.

-   :material-routes: **[Routing Policy](routing-policy.md)**

    ---

    Schema-backed model/provider/runtime lane policy, authority levels, and
    milestone-1 GasCity / Mt. Olympus production-routing guardrails.

-   :material-chart-timeline-variant: **[Factory Yield Ledger](factory-yield-ledger.md)**

    ---

    Baseline/treatment yield observations correlated to routing, validation,
    manual merge decisions, cost, latency, defects, and artifacts.

-   :material-shield-search: **[Factory Claim Ledger](factory-claim-ledger.md)**

    ---

    Machine-readable posture ledger tying public software-factory claims to
    evidence level, owner issue, closure gate, and anti-overclaim wording.

-   :material-lock-check: **[Daemon Idempotency](daemon-idempotency.md)**

    ---

    Submit retry contract for `request_id`, `idempotency_key`, and
    daemon-submitting CLI helpers.

-   :material-file-code: **[JobSpec OpenAPI v0](jobspec-openapi-v0.yaml)**

    ---

    Machine-readable OpenAPI contract for the current `agentopsd` job,
    readiness, ledger replay, projection, and OpenClaw consumer HTTP surface.

-   :material-api: **[GasCity Integration](gascity-integration.md)**

    ---

    Public GasCity API/SSE boundary, mutation headers, request IDs, readiness,
    replay, versioning, and adapter rules.

-   :material-lan-connect: **[Remote Compute](remote-compute.md)**

    ---

    Product-neutral RemoteTarget, RemoteSession, command ledger, recovery, and
    GasCity-first remote execution contract.

-   :material-robot-outline: **[AgentWorker Runtime](agent-worker.md)**

    ---

    Headless Claude/Codex worker session lifecycle contract consumed by
    wiki/forge and future daemon jobs.

-   :material-application-braces-outline: **[OpenClaw Consumer API](openclaw-consumer-api.md)**

    ---

    Read-only projection resources, snapshot versions, mutation gates, and
    `.agents` non-ownership rules for OpenClaw clients.

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

    Process model, generator authoring, locking, keep-awake, and artifact floor
    for private overnight runs.

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
