---
title: "Competition RPI: Memory, Learning, Wiki, Dream, and Pruning Pipelines"
description: "Reverse-engineered comparison of competitor memory substrates, learning loops, wiki formats, dream cycles, pruning, skill suites, primitives, and pipelines."
permalink: /comparisons/competition-rpi-memory-pipelines
---

# Competition RPI: Memory, Learning, Wiki, Dream, and Pruning Pipelines

Reverse-engineered April 25, 2026 from public repositories and AgentOps source-of-truth docs.
This is a capability comparison, not a clone plan.

Generated RPI packets were produced with `skills/reverse-engineer-rpi/scripts/reverse_engineer_rpi.py`
for Superpowers, Compound Engineer, GSD, Claude-Flow/Ruflo, GitHub Spec Kit, and cc-sdd.

## Executive Read

| Product | Competitive signal | AgentOps implication |
|---------|--------------------|----------------------|
| Claude-Flow / Ruflo | Strongest mechanized memory: AgentDB, ReasoningBank, semantic/vector search, namespaces, hooks, and explicit consolidation. | Treat Ruflo as the benchmark for database-backed memory operations and pruning metrics. |
| Compound Engineer | Closest markdown compounding neighbor: `docs/solutions/`, YAML frontmatter, learning search agents, and refresh semantics. | Treat Compound Engineer as the benchmark for human-readable solution-library maintenance. |
| GSD | Strongest `.planning/` context engineering and lightweight cross-session threads. | Borrow low-friction working-memory primitives without weakening AgentOps' scored memory model. |
| SDD tools | Strongest spec lifecycle discipline and implementation-note propagation inside a spec. | Keep AgentOps complementary to specs; make learning handoff into spec/task loops obvious. |
| Superpowers | Strongest small-surface workflow discipline and skill TDD posture. | Use it as a skill-quality bar, not a memory benchmark. |
| Karpathy LLM Wiki pattern | Best framing for external knowledge accumulation: raw inputs become a navigable synthesized wiki. | AgentOps' `llm-wiki` should stay distinct from `.agents` repo memory but connect through forge, compile, and inject. |

AgentOps' current differentiation is the combination of repo-native `.agents` memory, scored promotion, citations, validation gates, and a private overnight Dream contract. Competitors usually have one or two of those pieces, not the full closed loop.

## Comparison Matrix

| Aspect | Superpowers | Compound Engineer | GSD | Claude-Flow / Ruflo | Spec Kit | cc-sdd | AgentOps |
|--------|-------------|-------------------|-----|---------------------|----------|--------|----------|
| Memory substrate | Specs, plans, worktrees, optional conversation search | `docs/solutions/`, YAML frontmatter, session history, auto-memory | `.planning/` files, `STATE.md`, threads, graph artifacts | `.swarm/memory.db`, AgentDB, ReasoningBank, namespaces, vector search | Specs, plans, tasks, constitution; community memory extensions | `.kiro/steering`, `brief.md`, specs, tasks, implementation notes | `.agents` markdown, JSONL, compiled context, citations, bd |
| Compounding loop | Skill use and skill authoring discipline | `compound` captures solutions; `compound-refresh` maintains corpus | `extract_learnings`, threads, spike/sketch wrap-up into project skills | Retrieve -> judge -> distill -> consolidate | Spec feedback loop; memory mostly extensions | Implementation notes injected into later tasks | Forge -> score -> promote -> inject; post-mortem and Dream loops |
| Wiki shape | None | Solution library, not raw-to-wiki | Planning tree and optional graph, not wiki | Database/vector memory, not wiki | Extension ecosystem, not core wiki | Spec workspace, not wiki | `llm-wiki`: raw sources -> synthesized wiki -> lint/promote |
| Dream/offline cycle | None found | Refresh/autofix, but no explicit overnight cycle | No; "dream extraction" is project-init wording | Periodic/nightly consolidation is closest analog | None in core | None found | `ao overnight` Dream contract with safe staging and morning packet |
| Automated pruning | None found | Keep, update, consolidate, replace, delete; uncertain docs marked stale | Cleanup/archive and health repair | Cleanup by days/namespace; consolidation reports pruned items | Not core; community optimize/memory lint concepts | No corpus pruning found | Maturity decay, dedup, contradictions, prune/evict, defrag |
| Skill suite | 14 skills, 3 commands, 1 agent in clone | Docs claim 42+ skills and 50+ agents; clone showed 36 skill dirs and 51 agent files | 85 GSD command files and 33 agents in clone | v2 docs claim 25 skills and 100+ MCP tools; clone showed 38 `.claude/skills`, 168 command docs, 7 top-level agents | Command templates and extensions rather than a skill suite | Docs claim 17 skills across 8 agents; public repo stores an installer skill plus templates/docs | 69 skills plus CLI, hooks, contracts, schemas |
| Primary pipeline | Brainstorm -> worktree -> plan -> subagent dev -> TDD -> review -> finish branch | Ideate -> brainstorm -> plan -> work -> review -> compound -> refresh | New project -> discuss/research -> plan -> execute waves -> verify/UAT -> ship -> extract learnings | Swarm/hive-mind -> hooks -> shared memory -> post-command memory -> session restore/consolidation | Constitution -> specify -> plan -> tasks -> implement | Discovery -> spec init -> requirements -> design -> tasks -> impl -> validate | Research -> plan -> pre-mortem -> crank -> vibe -> post-mortem -> flywheel |

| Aspect | Anthropic Managed Agents (May 2026) |
|--------|--------------------------------------|
| Memory substrate | Managed memory store + dreaming; per-agent memory plus cross-agent shared learnings surfaced by the dream cycle |
| Compounding loop | Dreaming reviews past sessions, extracts patterns, curates memory between runs; outcomes loop iterates against a rubric until a separate-context grader passes |
| Wiki shape | Memory blocks per agent, surfaced through the Console; not a navigable wiki |
| Dream/offline cycle | First-class. Scheduled, optional human-review gate before memory updates land |
| Automated pruning | Dreaming restructures memory to stay high-signal as it evolves |
| Skill suite | Outcomes (rubric → grader → iterate) + multiagent orchestration (lead + specialists with own model/prompt/tools) |
| Primary pipeline | Lead agent → delegate to specialists → parallel work on shared filesystem → outcomes grader → webhook on completion |

## AgentOps Baseline

AgentOps presents itself as an operational layer and context compiler. The public README describes four layers: bookkeeping, validation, primitives, and flows. It also describes repo-local `.agents` memory and the RPI flow: research, plan, pre-mortem, crank, vibe, post-mortem, and flywheel.

The product docs position AgentOps around durable learning, loop closure, validation primitives, bookkeeping primitives, and flows. Knowledge is managed like code, scored on specificity/actionability/novelty/context/confidence, promoted into patterns or planning rules, and kept local.

The knowledge flywheel is the main memory contract:

```text
Work -> Forge -> Pool -> Promote -> Learnings -> Inject -> Better Work
```

The curation path includes catalog, verify, index, score, reject, and constrain stages. Current docs note that catalog/verify are implemented as CLI checks, while later scoring/rejection/constraining stages remain staged work.

Dream is a separate contract. `ao overnight` runs a private local maintenance loop that ingests knowledge, reduces it through defrag/dedup/compile/prune, measures health, and commits only improved staged artifacts. It never mutates source code, never invokes RPI, and emits a morning packet.

Sources: [README](https://github.com/boshu2/agentops/blob/main/README.md), [PRODUCT](https://github.com/boshu2/agentops/blob/main/PRODUCT.md), [knowledge flywheel](../knowledge-flywheel.md), [curation pipeline](../curation-pipeline.md), [dream run contract](../contracts/dream-run-contract.md), [dream report contract](../contracts/dream-report.md).

## Superpowers

Source snapshot: [obra/superpowers at 6efe32c](https://github.com/obra/superpowers/tree/6efe32c9e2dd002d0c394e861e0529675d1ab32e).

Observed public surface:

- 14 skill directories, 3 command files, and 1 agent file in the cloned repo.
- Core workflow skills include brainstorming, creating git worktrees, writing plans, executing plans, subagent-driven development, TDD, code review, and finishing branches.
- The subagent-driven development skill uses a fresh implementer per task plus two-stage review and remediation.
- The using-superpowers skill enforces mandatory relevant skill invocation and cross-runtime skill access.

Memory handling:

- Memory is mostly workflow-local: plans, specs, worktrees, and task artifacts.
- There is no repo-native long-term knowledge corpus comparable to `.agents`, `docs/solutions`, or `.planning`.
- Skill authoring docs reference searching past conversations, but that is not a product-level memory flywheel in the public repo.

Compounding:

- Compounding happens through better skills and disciplined process, not through automatic extraction or promotion of learnings.
- The writing-skills workflow is closest to compounding because it pressure-tests reusable procedures.

Wiki, dream, pruning:

- No wiki-style raw-to-synthesis knowledge base found.
- No Dream or overnight maintenance cycle found.
- No automated pruning/decay mechanism found.

Pipeline primitives:

```text
brainstorm
  -> create worktree
  -> write plan
  -> execute plan or subagent-driven development
  -> TDD / debug / review
  -> finish branch
```

Competitive read:

Superpowers is a quality bar for small, crisp workflow skills and skill TDD. It is not a strong memory or autonomous pruning benchmark.

## Compound Engineer

Source snapshot: [EveryInc/compound-engineering-plugin at 1284290](https://github.com/EveryInc/compound-engineering-plugin/tree/1284290af27139c2df192488099626688fd4898b).

Observed public surface:

- README describes 50+ agents and 42+ skills.
- The cloned plugin artifact showed 36 skill directories under `plugins/compound-engineering/skills` and 51 agent files under `plugins/compound-engineering/agents`.
- Core skills include `ideate`, `brainstorm`, `plan`, `work`, `code-review`, `compound`, `compound-refresh`, and `optimize`.
- Research/context skills include session research and Slack research.

Memory handling:

- Main substrate is `docs/solutions/` with structured YAML frontmatter.
- `ce-compound` captures solved problems, lessons, and patterns into durable markdown.
- It can use auto-memory scan, session-history extraction, related-doc discovery, and solution extraction.
- The learnings researcher searches `docs/solutions/` before work so agents avoid rediscovering prior knowledge.

Compounding:

- `ce-compound` converts completed work into solution docs.
- `ce-compound-refresh` maintains the corpus through individual learning refresh and pattern refresh.
- The system detects overlap and can update an existing solution instead of creating duplicates.

Wiki format:

- Closest competitor to AgentOps on human-readable knowledge. It is a solution library rather than a Karpathy-style raw-source-to-wiki system.
- It relies on frontmatter, related docs, overlap checks, and topic directories.

Dream and pruning:

- No explicit source-safe overnight Dream contract found.
- `compound-refresh` can run interactively or in autofix mode.
- Maintenance actions include keep, update, consolidate, replace, delete, and stale marking.
- Its docs say delete is the archive path because git history is the archive.

Pipeline primitives:

```text
ideate
  -> brainstorm
  -> plan
  -> work
  -> code review
  -> compound
  -> compound refresh
```

Competitive read:

Compound Engineer is AgentOps' closest philosophical neighbor. Its strongest differentiator is a concrete, human-readable `docs/solutions` maintenance UX. AgentOps is stronger where learning is scored, injected, validated, and tied to Dream/flywheel gates.

## GSD

Source snapshot: [glittercowboy/get-shit-done at 470c1a0](https://github.com/glittercowboy/get-shit-done/tree/470c1a0bff9fb81f01584cda2510358a290fb700).

Observed public surface:

- 85 command files under `commands/gsd` and 33 agents in the clone.
- README describes multi-runtime support, context engineering, spec-driven execution, spikes, sketches, threads, planning health, and graph tooling.
- GSD stores project configuration in `.planning/config.json` and can keep `.planning` committed.

Memory handling:

- Main substrate is `.planning/`: `PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md`, `STATE.md`, research, phase artifacts, reports, threads, and optional graph files.
- `gsd-thread` creates lightweight persistent context threads in `.planning/threads`.
- `gsd-graphify` can build/query graph artifacts under `.planning/graphs`.
- `gsd-map-codebase` writes codebase intelligence documents into `.planning/codebase`.

Compounding:

- `gsd-extract-learnings` extracts decisions, lessons, patterns, and surprises from completed phase artifacts into `LEARNINGS.md`.
- It optionally integrates with a `capture_thought` memory/KB MCP, but `LEARNINGS.md` is the primary public mechanism.
- Spike and sketch wrap-up commands package findings into project-local skills.

Wiki format:

- Not a Karpathy wiki. The shape is a planning tree plus optional graph.
- It is strong at progressive disclosure and resumable project context, weaker at synthesized long-term knowledge governance.

Dream and pruning:

- No Dream cycle found. Some docs use the phrase "Dream extraction" for project initialization concepts, not overnight learning.
- Cleanup commands archive completed phase directories into milestones.
- Health repair checks `.planning` integrity. This is artifact hygiene, not maturity-based pruning.

Pipeline primitives:

```text
map codebase / new project
  -> discuss and research
  -> plan
  -> execute dependency waves
  -> verify and UAT
  -> ship
  -> extract learnings / thread / graph / cleanup
```

Competitive read:

GSD is a strong example of working-memory ergonomics. AgentOps should consider a lighter thread primitive and project-skill packaging for spike findings, while keeping `.agents` as the governed memory layer.

## Claude-Flow / Ruflo

Source snapshot: [ruvnet/ruflo at 01070ed](https://github.com/ruvnet/ruflo/tree/01070ede81fa6fbae93d01c347bec1af5d6c17f0).

Observed public surface:

- v2 docs describe 25 Claude skills, 100+ MCP tools, AgentDB integration, ReasoningBank, persistent memory, hooks, and hive-mind swarm orchestration.
- The clone showed 38 `.claude/skills` directories, 168 `.claude/commands` markdown files, and 7 top-level `.claude/agents` files.
- Memory commands cover store, query, stats, export, import, cleanup, namespaces, vector search, and status.

Memory handling:

- Main substrate is `.swarm/memory.db`, with AgentDB and ReasoningBank as memory backends.
- Public docs describe vector search, semantic search, namespaces, reflexion memory, skill-library consolidation, CRDT-style cross-session memory, and session-restore hooks.
- Swarms can store shared objective/progress/result memory under namespaces.

Compounding:

- ReasoningBank defines a closed loop:

```text
RETRIEVE -> JUDGE -> DISTILL -> CONSOLIDATE
```

- It stores trajectories and distilled patterns, then consolidates duplicates, contradictions, and low-value memories.
- Hooks can trigger memory loading before tasks and memory updates after tasks, edits, or commands.

Wiki format:

- Not a markdown wiki. This is database-backed semantic memory.
- Strength is retrieval mechanics and schema; weakness versus AgentOps is inspectability and repo-native review.

Dream and pruning:

- Periodic or nightly consolidation is the closest Dream analog.
- Memory cleanup supports retention by days and namespace.
- ReasoningBank consolidation reports created memories, duplicates, contradictions, and pruned items.

Pipeline primitives:

```text
swarm / hive-mind init
  -> load memory
  -> orchestrate agents
  -> store decisions and edits
  -> search/query memory
  -> consolidate
  -> restore future sessions from memory
```

Competitive read:

Ruflo is the benchmark for memory operations, memory CLI verbs, namespaces, and consolidation telemetry. AgentOps should not copy the database-first posture wholesale, but it should make prune/dedup/consolidate metrics as explicit as Ruflo does.

## GitHub Spec Kit

Source snapshot: [github/spec-kit at 171b65a](https://github.com/github/spec-kit/tree/171b65ac33a3bf51c23b9f7a5287032ed1ae72ba).

Observed public surface:

- Core workflow is spec-driven: constitution, specify, plan, tasks, implement.
- The repo contains command templates and an extension framework rather than a large skill suite.
- Community extensions listed in public docs include Archive, Memory Loader, Memory MD, MemoryLint, and Optimize.

Memory handling:

- Core memory is the spec workspace: specs, plans, tasks, constitution, and project principles.
- Memory-specific features appear mainly as community extensions:
  - Archive stores merged feature knowledge into project memory.
  - Memory Loader loads `.specify/memory/` before lifecycle commands.
  - Memory MD offers durable repo-native memory.
  - MemoryLint governs memory quality.
  - Optimize focuses on token budgets, compression, coherence, and rule health.

Compounding:

- Core compounding is spec refinement: operational reality updates the spec.
- There is no core learning flywheel comparable to Forge -> Promote -> Inject.

Wiki, dream, pruning:

- No native Karpathy-style wiki.
- No Dream cycle in core.
- Pruning and compression appear as extension concepts, not the core product contract.

Pipeline primitives:

```text
constitution
  -> specify
  -> plan
  -> tasks
  -> implement
```

Competitive read:

Spec Kit is the mainstream spec lifecycle benchmark. AgentOps should keep compatibility high and treat memory as what happens after specs, implementation, and validation produce learnings.

## cc-sdd

Source snapshot: [gotalab/cc-sdd at dcbd81d](https://github.com/gotalab/cc-sdd/tree/dcbd81d0178ad43ffe271e5b56e9d9121a154de9).

Observed public surface:

- README describes a one-command installer for an agentic SDLC workflow.
- Docs describe 17 skills across 8 agents.
- The checked-in repo stores examples, templates, docs, and an installer skill, not the fully installed runtime tree.

Memory handling:

- Main substrate is Kiro-style project state: `.kiro/steering/*.md`, `brief.md`, `roadmap.md`, specs, requirements, design, tasks, and validation artifacts.
- `brief.md` supports session persistence.
- Steering docs capture project memory and conventions.

Compounding:

- Implementation Notes in `tasks.md` propagate learning from one implementation task into later prompts.
- `kiro-impl` uses fresh implementer/reviewer/debugger agents with resume-safe one-task iterations.

Wiki, dream, pruning:

- No external wiki or synthesized knowledge base found.
- No Dream cycle found.
- No explicit corpus pruning found.

Pipeline primitives:

```text
kiro discovery
  -> spec init
  -> requirements
  -> design
  -> tasks
  -> implementation
  -> validation
```

Competitive read:

cc-sdd is strong at boundary-first spec work and intra-run learning propagation. AgentOps can borrow the "Implementation Notes flow forward" primitive for RPI worker loops.

## Karpathy LLM Wiki Pattern

Reference: [Karpathy gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) and AgentOps' [`llm-wiki` skill](https://github.com/boshu2/agentops/blob/main/skills/llm-wiki/SKILL.md).

Pattern:

```text
raw external sources
  -> entity and concept pages
  -> synthesized wiki pages
  -> backlinks and indexes
  -> query with citations
  -> lint for orphans, drift, contradictions, and stale pages
  -> promote durable conclusions into operational memory
```

AgentOps currently separates internal repo memory from external knowledge:

- `.agents` is for repo-native operational memory, learnings, patterns, RPI packets, and compiled context.
- `llm-wiki` is for external knowledge bases where raw materials are retained and synthesized into wiki pages.
- Promotion from wiki to AgentOps should happen only when external knowledge becomes operationally relevant to the current repo.

Competitive read:

None of the direct competitors has a full Karpathy-style public wiki loop as a core product primitive. GSD has a graph, Compound Engineer has a solution library, and Ruflo has semantic memory, but AgentOps can own the auditable raw-to-wiki-to-operational-memory bridge.

## Anthropic Managed Agents (May 2026)

Reference: Anthropic Code-with-Claude announcement, 2026-05-06. Dreaming research preview plus outcomes, multiagent orchestration, and webhooks shipped in public beta on the Claude Platform.

This is Anthropic externalizing the architecture AgentOps has been implementing. Memory + dream cycle + grader-graded outcomes + lead/specialist orchestration are the same primitives AgentOps ships locally. The wedge for AgentOps was always the deployment story, and that wedge is unchanged: this is off the API, cross-vendor, and the corpus stays in `.agents/` on disk.

Pattern:

```text
agent session
  -> memory blocks per agent
  -> scheduled dream cycle reviews past sessions
  -> patterns extracted, memory curated (auto or human-gated)
  -> outcomes rubric -> separate-context grader -> retry until pass
  -> lead agent delegates to specialists with their own model/prompt/tools
  -> parallel work on shared filesystem; persistent events; Console trace
  -> webhook on completion
```

Capability mapping against AgentOps:

| Managed Agents | AgentOps equivalent | Status |
|---|---|---|
| Dreaming | `/dream` + `ao overnight` + nightly CI dream-cycle proof job | Shipped |
| Outcomes (rubric → grader → iterate) | `/vibe` + `/council` + `/pre-mortem` (judges as separate-context graders) → unifying `ao outcomes run` verb in flight | Partial |
| Multiagent orchestration | `/swarm` + `/crank` + `/rpi` + worktree isolation | Shipped (per-worker tool isolation in flight) |
| Webhooks (completion notification) | GitHub Actions / git hooks; pattern doc in flight | Doc-only, off-API users already have the substrate |

Competitive read:

This is the validation event for the AgentOps thesis, not a competitive threat. Anthropic confirms the architecture; what stays yours is the corpus the system compounds on your behalf. Readers who want these primitives running off the API, against any vendor's model, with the corpus owned locally land at AgentOps.

## Borrowable Primitives

| Source | Primitive to consider | Why it matters |
|--------|-----------------------|----------------|
| Ruflo | Memory namespaces with explicit `stats`, `cleanup`, `consolidate`, and result counters | Makes pruning observable and less hand-wavy. |
| Ruflo | Retrieval scoring fields surfaced in CLI output | Helps users debug why a memory was injected or skipped. |
| Compound Engineer | Refresh actions: keep, update, consolidate, replace, delete, stale | Gives users an intuitive maintenance vocabulary. |
| Compound Engineer | Related-doc overlap checks before writing a new learning | Reduces duplicate knowledge. |
| GSD | Lightweight `.planning/threads`-style context threads | Useful for unresolved cross-session questions that are not mature learnings. |
| GSD | Spike/sketch wrap-up into project-local skills | Converts exploration into reusable project procedures. |
| cc-sdd | Implementation Notes injected into later task prompts | Keeps local tactical learning alive inside a single RPI run. |
| Spec Kit | Extension ecosystem around memory, lint, archive, and optimize | Lets memory features be adopted without forcing every workflow into the core. |
| Superpowers | Skill TDD and pressure scenarios | Raises the quality bar for AgentOps skills and Codex artifacts. |

## Gaps To Watch In AgentOps

- Curation docs describe later-stage score/reject/constrain work as planned; avoid claiming that full automated pruning is complete until gates prove it.
- Dream is better specified than most competitors, but users need visible run summaries that prove what was pruned, deduped, compiled, and left untouched.
- AgentOps has stronger repo-native memory than database-first systems, but retrieval debugging should be more transparent.
- Direct competitors have clearer lightweight primitives for threads, project-local skills, and per-task implementation notes.
- Public comparison pages still contain some stale counts. Keep detailed comparisons anchored to current clone evidence or update the marketing counts together.

## Bottom Line

AgentOps should compare itself against three distinct competitor archetypes:

- Ruflo: mechanized database memory and consolidation.
- Compound Engineer: readable solution-library compounding and refresh.
- GSD/SDD/Superpowers: ergonomic workflow discipline around plans, specs, tasks, and skills.

Anthropic's 2026-05-06 Managed Agents launch ships these primitives natively on the Claude Platform; the AgentOps wedge is the off-API + cross-vendor deployment story, not the primitives themselves.

AgentOps wins when it proves the whole loop is operational: capture evidence, score it, promote it, inject it with citations, validate that it helped, prune what decays, and let Dream run the maintenance work without touching source code.
