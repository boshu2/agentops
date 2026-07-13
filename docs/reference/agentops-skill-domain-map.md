# AgentOps Skill Domain Map

Generated classification of checked-in skills across BC1-BC6. This is not a CLI
capability matrix. Edit `docs/contracts/bounded-contexts.yaml`,
`docs/contracts/skill-dispositions.yaml`, or skill frontmatter, then regenerate.

## Audit summary

<!-- BEGIN:audit-summary -->
| Signal | Result |
|---|---:|
| Skills audited | 63 |
| Domains classified | 6 of 6 (BC1-BC6) |
| Dispositions assigned | 63 / 63 |
<!-- END:audit-summary -->

## Domain taxonomy

<!-- BEGIN:domain-taxonomy -->
| Domain | Product layer | Responsibility |
|---|---|---|
| BC1 Corpus | Local bookkeeping and experimental context reuse | Capture, retrieve, cite, and promote local knowledge and evidence. |
| BC2 Validation | Verification membrane | Judge whether plans, code, docs, dependencies, and releases satisfy declared acceptance. |
| BC3 Loop | Operating loop | Shape intent, execute vertical slices, record evidence, and steer repair to verified done. |
| BC4 Factory | Skill and claim factory | Build, audit, package, and govern reusable skills, workflows, and claims. |
| BC5 Runtime | Harness and operator adapters | Adapt the core loop to harnesses, workspaces, shells, installers, and operator machines. |
| BC6 Orchestration | Multi-agent orchestration substrate | Dispatch and coordinate whole loop units across explicitly selected substrates. |
<!-- END:domain-taxonomy -->

## Dispositions

- `keep`: the skill remains a supported route.
- `update`: the route remains but its contract needs work.
- `merge`: another route should absorb the capability.
- `delete`: remove the route after its consumers are migrated.

## Full skill map

<!-- BEGIN:full-skill-map -->
| Skill | Domain | Hex role | First disposition | Rationale |
|---|---|---|---|---|
| `account-rotation` | BC5 Runtime | supporting | keep | Unified host-routed account rotation (macOS+Claude->claude-acct Keychain swap; else->caam file swap); supersedes the retired caam skill |
| `agent-mail` | BC6 Orchestration | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `agent-native` | BC5 Runtime | supporting | keep | Portable AgentWorker lifecycle for persistent role-shaped factories; NTM, Agent Mail, and GC remain adapters behind explicit operator routing |
| `agy-native` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `automation-shape-routing` | BC4 Factory | supporting | keep | Front-door router: decides Workflow vs NTM swarm vs plain skill, hands off to the right builder |
| `beads-br` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `beads-bv` | BC3 Loop | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `behavior-first-planning` | BC3 Loop | domain | keep | Generative discipline behind the bdd-foundry workflow; extracted so behavior-first planning survives independent of the orchestrator (age-3va.2) |
| `bootstrap` | BC4 Factory | driving-adapter | update | First-run factory entrypoint; needs current 3.0/domain packet shape |
| `cass` | BC1 Corpus | supporting | keep | Mines past agent sessions for prompts/decisions/patterns — a corpus reader; re-binned BC5→BC1 (ag-j3ge0, capability_class=corpus already) |
| `cc-hooks` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `codebase-recon` | BC1 Corpus | supporting | keep | Formalizes the existing recurring recon packs with evidence bounds and verified delta mode |
| `codex-exec` | BC5 Runtime | driving-adapter | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `converge` | BC5 Runtime | driving-adapter | keep | Skill-owned fix, re-run, and independent-judge loop; it uses current validation and council surfaces without relying on an ao converge command |
| `converter` | BC4 Factory | driven-adapter | keep | Cross-runtime packaging adapter (Codex/Cursor twins) the factory drives to emit converted-skill output; skill-builder consumes it. Re-graded generic→driven-adapter (ag-j3ge0 — it adapts the factory to runtime skill formats, a driven port, not an unclassified generic) |
| `council` | BC2 Validation | domain | update | Core judgment gate; strengthen scenario and verdict self-test |
| `crank` | BC3 Loop | domain | refactor | Wave executor; align with vertical-slice and conflict-free wave contract |
| `dcg` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `discovery` | BC3 Loop | domain | update | Creates execution packets; add explicit loop-shape SELF-TEST |
| `doc` | BC4 Factory | supporting | update | Documentation factory adapter; keep tied to doc-release gates |
| `domain` | BC4 Factory | domain | keep | Ubiquitous-language kernel; central to DDD |
| `dueling-idea-genies` | BC2 Validation | domain | keep | Original sealed idea challenge packet; ao plan-pawl remains the decision owner |
| `evolve` | BC3 Loop | domain | refactor | Autonomous improvement main loop with convergence STOP; promoted supporting→domain (ag-j3ge0 — the loop's core driver, not a peripheral helper). Demoted to experimental (heavy rpi chain, no measured uplift) |
| `gc-membrane` | BC6 Orchestration | supporting | keep | gc adoption (age-gc-integrate-8aom.1): JIT reference for the agentops-membrane pack — the fail-closed close door; loaded by using-gc |
| `goal-design` | BC3 Loop | driving-adapter | keep | Creates checked intent/driver packets before discovery or planning; separates per-objective intent from GOALS.md fitness management |
| `goals` | BC3 Loop | domain | keep | Fitness source; use as evolution selection input |
| `handoff` | BC1 Corpus | supporting | update | Session continuity artifact; clarify promotion vs local-only notes |
| `heal-skill` | BC4 Factory | supporting | update | Skill hygiene gate; should consume the new domain map |
| `idea-genie` | BC3 Loop | domain | keep | Original evidence-grounded opportunity portfolio with adaptive saturation; discovery retains BDD persistence |
| `implement` | BC3 Loop | driving-adapter | update | Slice executor; enforce first-failing-test language |
| `learn` | BC3 Loop | domain | keep | Fourth RPI umbrella; captures evidence-backed observations after an immutable Validate verdict without taking proof or delivery authority |
| `ms` | BC1 Corpus | supporting | keep | Wraps Jeffrey Emanuel's meta_skill (ms) — the skill-search/load engine over both corpora (agentops + jsm); MCP consume, CLI writes. A corpus reader like cass |
| `ntm` | BC6 Orchestration | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `operationalize` | BC1 Corpus | domain | keep | Seams epic ag-xwjlc: distill+route bridge from gathered context to automation shapes; fills the curate/forge-to-builders seam |
| `pattern-mining` | BC1 Corpus | supporting | keep | Promotes abstractions only after three exemplars, holdout, and back-application; weak evidence stays hypothesis |
| `pawl-review` | BC2 Validation | driving-adapter | keep | Immutable transport-neutral reviewer-lane execution; ao pawl alone binds the panel verdict |
| `plan` | BC3 Loop | domain | update | Must output vertical slices and wave-validity checks |
| `postmortem` | BC3 Loop | domain | update | Optional retrospective causal analysis after Validate and Learn; owns no general closeout, bookkeeping, promotion, or delivery authority |
| `pr-prep` | BC5 Runtime | driving-adapter | update | PR publication adapter; align to evidence and release discipline |
| `premortem` | BC2 Validation | domain | update | Plan risk gate; add scenario/verdict self-test |
| `product` | BC3 Loop | domain | keep | Product intent source; important for loop work selection |
| `push` | BC5 Runtime | driving-adapter | update | Git adapter; add branch/worktree disposition self-test |
| `rch` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `reality-check` | BC2 Validation | domain | keep | Seams epic ag-xwjlc: mid-epic drift audit (code vs claimed vision); complements status/validate/post-mortem |
| `refactor` | BC3 Loop | supporting | update | Refactor generator; loop-side change execution (re-binned BC2→BC3, ag-j3ge0 — produces code changes inside the operating loop, not a validation gate) |
| `release` | BC2 Validation | supporting | update | Release gate driver; keep tied to local CI and evidence export |
| `research` | BC1 Corpus | driving-adapter | update | Knowledge acquisition entrypoint; add source/citation self-test |
| `reverse-engineer` | BC1 Corpus | supporting | keep | External-system teardown -> steal-map (have/gap/steal/park/reject) -> route one-way doors to /discovery. Revived + renamed from reverse-engineer-rpi (cut ag-s43tg S24), upgraded with the steal-map discipline |
| `rpi` | BC3 Loop | domain | refactor | Loop-spine lifecycle orchestrator (Research→Plan→Implement); promoted supporting→domain (ag-j3ge0 — the per-turn executor IS the operating loop's core logic, not a peripheral helper) |
| `sbh` | BC5 Runtime | supporting | keep | New factory-built skill from the four-vendor corpus expansion; keep as clean-room AgentOps-owned capability |
| `scaffold` | BC4 Factory | supporting | update | Code/artifact scaffolder; add non-goal and validation examples |
| `scope` | BC5 Runtime | driven-adapter | keep | Runtime filesystem gate; hard boundary skill |
| `security` | BC2 Validation | driven-adapter | keep | Canonical security skill: continuous release-gate driver (security-gate.sh) PLUS the composable primitive library (security_suite.py / prompt_redteam.py) collapsed in from security-suite (cp-qqq, 2026-06-07) |
| `shared` | BC4 Factory | domain | keep | Shared contracts; avoid broad edits |
| `skill-builder` | BC4 Factory | supporting | update | Builder should scaffold SELF-TEST and domain metadata by default |
| `standards` | BC4 Factory | domain | keep | Current pilot upgraded with SELF-TEST; continue incremental patches |
| `status` | BC3 Loop | driving-adapter | update | Operator state surface; should show loop/domain/evidence status |
| `swarm` | BC6 Orchestration | supporting | update | Multi-agent runtime adapter; align with conflict-free wave rules |
| `test` | BC2 Validation | supporting | update | Test generator; central to first-failing-test loop |
| `toil-mining` | BC1 Corpus | supporting | keep | Seams epic ag-xwjlc: usage-history toil miner feeding automation-shape-routing; the flywheel's missing feeder |
| `using-gc` | BC6 Orchestration | supporting | keep | Operator-choice Gas City driver beside NTM, with optional portable worker/reviewer partnership and GC-native close door |
| `validate` | BC2 Validation | driving-adapter | keep | Live canonical post-implementation validator; owns scenario-bound PASS/WARN/FAIL judgment and absorbs retired review and vibe routes |
| `workflow-builder` | BC4 Factory | supporting | keep | Scaffolds Claude Workflow scripts (composite capability); counterpart to skill-builder |
<!-- END:full-skill-map -->
