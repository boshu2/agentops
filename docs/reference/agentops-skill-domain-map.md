# AgentOps Skill Domain Map

This map is the control surface for the next evolution loop. It classifies all
106 checked-in AgentOps skills before any broad rewrite, using current
`origin/main` product direction, GOALS Directive 12, the DDD/hexagonal ADR, and
the `soc-y5vh` Loop epic.

The product frame matters: AgentOps 3.0 is a context compiler and SDLC control
plane for LLM agents. Skills, CLI commands, hooks, docs, tests, beads, and
knowledge artifacts are not separate products; they are aligned adapter surfaces
around small provable changes.

## Audit Summary

> Generated from skills/ + docs/contracts/skill-dispositions.yaml.
> Do not hand-edit the table below — run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:audit-summary -->
| Signal | Result |
|---|---:|
| Skills audited | 106 |
| Domains classified | 5 of 5 (BC1-BC5) |
| Dispositions assigned | 106 / 106 |
<!-- END:audit-summary -->

Observed gap: the catalog has strong operational kernels but weak productized
skill packaging. The highest-leverage first pass is not rewriting prose; it is
adding self-tests, splitting overloaded kernels into references where needed,
and aligning loop-facing skills to the bounded-context architecture.

## Domain Taxonomy

> Generated from docs/contracts/bounded-contexts.yaml.
> Do not hand-edit; edit yaml and run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:domain-taxonomy -->
| Domain | Product layer | Responsibility |
|---|---|---|
| BC1 Corpus | Bookkeeping + Context Compiler + Knowledge Flywheel | Capture, retrieve, compile, cite, and promote knowledge. |
| BC2 Validation | Validation Gates | Judge whether plans, code, docs, dependencies, and releases are fit. |
| BC3 Loop | Operating loop | Select work, execute RPI, log cycles, measure fitness, and stop at convergence. |
| BC4 Factory | Skill and claim factory | Build, audit, package, and govern reusable skills and product claims. |
| BC5 Runtime | Harness and operator adapters | Adapt the control plane to harnesses, hooks, PRs, shells, and local machines. |
<!-- END:domain-taxonomy -->

## Full Skill Map

Disposition meanings:

- `keep` means the responsibility is sound; improve later only through evidence.
- `update` means add tests, references, triggers, or validation without changing the core responsibility.
- `refactor` means the skill likely needs structural reshaping or port alignment.
- `merge-review` means compare with neighboring skills before investing.
- `cut-review` means keep only if a concrete operator workflow still justifies it.

> Generated from docs/contracts/skill-dispositions.yaml.
> Do not hand-edit the table — edit the yaml and run `bash scripts/generate-skill-domain-map.sh`.

<!-- BEGIN:full-skill-map -->
| Skill | Domain | Hex role | First disposition | Rationale |
|---|---|---|---|---|
| `acfs` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agent-mail` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agent-native` | BC5 Runtime | supporting | keep | Hookless reframe: makes out-of-session agents (Managed/SDK/sandbox) AgentOps-native via skills + ao CLI + CI; extends standards+converter, never a hook revival. |
| `agy-headless-evidence` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agy-mcp-plugins` | BC5 Runtime | driven-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agy-native` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agy-project-worktree-permissions` | BC5 Runtime | supporting | keep | Coverage-gap skill proving AGY project/worktree isolation invariants before the image joins quorum; clean-room AgentOps-owned.. |
| `agy-rules-workflows` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `agy-sidecar-scheduled-tick` | BC5 Runtime | supporting | keep | Coverage-gap skill for recurring AGY loop ticks via Antigravity sidecar + agentapi evidence; clean-room AgentOps-owned.. |
| `autodev` | BC3 Loop | supporting | refactor | Must compose with PROGRAM.md and RPI as one vertical-slice executor. |
| `automation-shape-routing` | BC4 Factory | supporting | keep | Front-door router: decides Workflow vs NTM swarm vs plain skill, hands off to the right builder. |
| `bead-completion-audit` | BC2 Validation | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `beads` | BC3 Loop | driven-adapter | update | Tracker adapter is core; add BDD/slice acceptance self-test. |
| `beads-br` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `beads-bv` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `beads-workflow` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `bootstrap` | BC4 Factory | driving-adapter | update | First-run factory entrypoint; needs current 3.0/domain packet shape. |
| `brainstorm` | BC3 Loop | domain | update | Intent-shaping skill; should emit BDD-ready language. |
| `bug-hunt` | BC2 Validation | domain | update | Validation generator; needs acceptance examples and result contract. |
| `burndown` | BC3 Loop | domain | keep | Bounded epic-completion loop; terminating counterpart to evolve (finite target → all-merged → STOP). |
| `caam` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `casr` | BC4 Factory | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cass` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cass-memory` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cc-cron-ticks` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cc-hooks` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cc-loop-driver` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cc-subagents` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `cc-worktree-isolation` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `codebase-audit` | BC2 Validation | supporting | keep | Canonicalized from ~/.claude/skills ghost dir (cp-801l); domain-parameterized audit skill, AgentOps-owned.. |
| `codex-approval` | BC5 Runtime | driving-adapter | keep | Codex-side approval bridge that routes plan reviews through an interactive ATM/NTM Fable validator, with persisted tmux capture and council artifact evidence.. |
| `codex-exec` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `codex-goals` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `codex-mcp-plugins` | BC5 Runtime | driven-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `codex-sandbox-evidence` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `compile` | BC1 Corpus | supporting | refactor | Corpus compiler is core; align read/write flows to Corpus ports. |
| `complexity` | BC2 Validation | domain | update | Generator for refactor work; add self-test and threshold evidence. |
| `converter` | BC4 Factory | generic | keep | Cross-runtime packaging adapter (Codex/Cursor twins); skill-builder consumes it — live producer→consumer edge (resolved 2026-05-24). |
| `council` | BC2 Validation | domain | update | Core judgment gate; strengthen scenario and verdict self-test. |
| `crank` | BC3 Loop | domain | refactor | Wave executor; align with vertical-slice and conflict-free wave contract. |
| `cross-vendor-trust-gate` | BC4 Factory | domain | keep | Coverage-gap skill operating the skill-factory final trust gate (cross-vendor parity); shared kernel with heal-skill.. |
| `curate` | BC1 Corpus | supporting | keep | Designed-future canonical unified miner (m6v5.D Phase 1, epic soc-cp7pv); not redundant cruft — epic GO/REVERT is a separate decision (resolved KEEP 2026-05-24). |
| `dcg` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `deps` | BC2 Validation | driven-adapter | update | Dependency risk generator; add threshold and no-network fallback tests. |
| `design` | BC3 Loop | domain | update | Product-fit pre-discovery gate; should produce BDD-ready intent. |
| `discovery` | BC3 Loop | domain | update | Creates execution packets; add explicit loop-shape SELF-TEST. |
| `doc` | BC4 Factory | supporting | update | Documentation factory adapter; keep tied to doc-release gates. |
| `domain` | BC4 Factory | domain | keep | Ubiquitous-language kernel; central to DDD. |
| `eval-outcomes` | BC2 Validation | supporting | keep | Holdout-safe Outcomes grading transport projecting the locked eval substrate; extends validation+ratchet, emits the one council verdict — never an alternate bar. |
| `evolve` | BC3 Loop | supporting | refactor | Main loop; must use `soc-y5vh` typed Loop ports and convergence STOP. |
| `flywheel` | BC1 Corpus | domain | update | Flywheel health kernel; needs productized self-test. |
| `forge` | BC1 Corpus | domain | update | Learning extraction; align to capture quality and promotion ratchet. |
| `goals` | BC3 Loop | domain | keep | Fitness source; use as evolution selection input. |
| `handoff` | BC1 Corpus | supporting | update | Session continuity artifact; clarify promotion vs local-only notes. |
| `heal-skill` | BC4 Factory | supporting | update | Skill hygiene gate; should consume the new domain map. |
| `implement` | BC3 Loop | driving-adapter | update | Slice executor; enforce first-failing-test language. |
| `inject` | BC1 Corpus | driving-adapter | refactor | Context injection should be explicit CorpusReader adapter use. |
| `multi-model-triangulation` | BC4 Factory | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `ntm` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `ntm-browser-test-coordination` | BC2 Validation | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `ntm-review-worker-orchestration` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `operating-loop-skill` | BC5 Runtime | domain | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `operating-loop-workflow` | BC3 Loop | driving-adapter | keep | Installs + invokes the operating-loop Workflow so plugin users get the seven-move loop. |
| `perf` | BC2 Validation | domain | update | Performance generator; add thresholds and proof examples. |
| `plan` | BC3 Loop | domain | update | Must output vertical slices and wave-validity checks. |
| `planning-workflow` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `post-mortem` | BC3 Loop | domain | update | Loop closeout; connect to next-work and ratchet evidence. |
| `pr-implement` | BC5 Runtime | driving-adapter | update | GitHub PR implementation adapter; map to loop slices. |
| `pr-prep` | BC5 Runtime | driving-adapter | update | PR publication adapter; align to evidence and release discipline. |
| `pr-research` | BC5 Runtime | driven-adapter | update | Upstream repo research adapter; add clean source/citation self-test. |
| `pre-mortem` | BC2 Validation | domain | update | Plan risk gate; add scenario/verdict self-test. |
| `product` | BC3 Loop | domain | keep | Product intent source; important for loop work selection. |
| `push` | BC5 Runtime | driving-adapter | update | Git adapter; add branch/worktree disposition self-test. |
| `quickstart` | BC3 Loop | driving-adapter | update | Operator routing entrypoint; align to current 3.0 first-value path. |
| `ratchet` | BC3 Loop | domain | update | Loop evidence ratchet; connect to cycle trace contract. |
| `rch` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `recover` | BC1 Corpus | driving-adapter | refactor | Session recovery is valuable but currently structurally heavy. |
| `red-team` | BC2 Validation | supporting | update | Probe generator; add severity/evidence contract. |
| `refactor` | BC2 Validation | supporting | update | Refactor generator; align with complexity thresholds and TDD proof. |
| `release` | BC2 Validation | supporting | update | Release gate driver; keep tied to local CI and evidence export. |
| `research` | BC1 Corpus | driving-adapter | update | Knowledge acquisition entrypoint; add source/citation self-test. |
| `reverse-engineer-rpi` | BC1 Corpus | supporting | update | External product-spec extraction; ensure clean-room rules stay explicit. |
| `review` | BC2 Validation | driving-adapter | update | Human-facing review gate; align to validator output contract. |
| `rpi` | BC3 Loop | supporting | refactor | Lifecycle orchestrator; should consume BDD intent and preserve objective spine. |
| `sbh` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `scaffold` | BC4 Factory | supporting | update | Code/artifact scaffolder; add non-goal and validation examples. |
| `scenario` | BC2 Validation | supporting | update | Holdout scenario manager; important for behavioral evals. |
| `scope` | BC5 Runtime | driven-adapter | keep | Runtime filesystem gate; hard boundary skill. |
| `security` | BC2 Validation | driven-adapter | keep | Canonical security skill: continuous release-gate driver (security-gate.sh) PLUS the composable primitive library (security_suite.py / prompt_redteam.py) collapsed in from security-suite (cp-qqq, 2026-06-07). |
| `session-bootstrap` | BC5 Runtime | driving-adapter | keep | Universal init prompt; the fungibility contract for any-agent spawn (soc-vuu6.25). |
| `shared` | BC4 Factory | domain | keep | Shared contracts; avoid broad edits. |
| `ship-loop` | BC5 Runtime | driving-adapter | keep | Bot-paired internal-ship PR cycle; distinct from pr-* contribute family. |
| `skill-auditor` | BC4 Factory | supporting | update | Audit role should consume new quality/domain rubrics. |
| `skill-builder` | BC4 Factory | supporting | update | Builder should scaffold SELF-TEST and domain metadata by default. |
| `standards` | BC4 Factory | domain | keep | Current pilot upgraded with SELF-TEST; continue incremental patches. |
| `status` | BC3 Loop | driving-adapter | update | Operator state surface; should show loop/domain/evidence status. |
| `swarm` | BC5 Runtime | supporting | update | Multi-agent runtime adapter; align with conflict-free wave rules. |
| `test` | BC2 Validation | supporting | update | Test generator; central to first-failing-test loop. |
| `trace` | BC1 Corpus | supporting | update | Decision trace builder; align to provenance and cycle trace. |
| `ubs` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `using-agentops` | BC4 Factory | generic | update | Operator education; align to 3.0 first-value path. |
| `using-atm` | BC4 Factory | supporting | keep | Shippable AgentOps-scoped guide for the ATM leg of the out-of-session substrate (ATM+Agent Mail+managed-agents): spawn agent panes that run the /rpi and /evolve skills over a bead queue. Replaces the dangling external ntm/vibing-with-ntm pointers in automation-shape-routing with an owned, skills-as-runtime skill.. |
| `validate` | BC2 Validation | driving-adapter | keep | Designed-future canonical unified validator (m6v5.D Phase 1, epic soc-cp7pv); not redundant cruft — epic GO/REVERT is a separate decision (resolved KEEP 2026-05-24). |
| `vibe` | BC2 Validation | domain | update | Code-readiness validator; add self-test and tighten result contract. |
| `vibing-with-ntm` | BC4 Factory | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability.. |
| `workflow-builder` | BC4 Factory | supporting | keep | Scaffolds Claude Workflow scripts (composite capability); counterpart to skill-builder. |
<!-- END:full-skill-map -->

## Priority Queue

1. **Loop spine:** `evolve`, `rpi`, `discovery`, `plan`, `crank`,
   `validation`, `post-mortem`, `ratchet`.
2. **Factory spine:** `standards`, `skill-builder`, `skill-auditor`,
   `heal-skill`, `converter`.
3. **Corpus spine:** `compile`, `inject`, `flywheel`, `forge`, `harvest`,
   `dream`.
4. **Validation spine:** `council`, `vibe`, `pre-mortem`, `test`, `review`,
   `security-suite`, `release`.
5. **Runtime spine:** `hooks-authoring`, `scope`, `push`, `swarm`,
   `codex-team`.
