# Skill Domain Map

> **Status:** V0 thin map.
> **Owner bead:** `soc-m6v5.9.9.1`.
> **Purpose:** give agents one small DDD + hexagonal map for skill refactors
> without reading the whole skill catalog.

This map assigns every shared `skills/*/SKILL.md` contract to one primary skill
domain. It is intentionally thinner than a full software design document: the
goal is to choose the next bounded context and port boundary, not predict every
future refactor.

The generated [Context Map](context-map.md) remains the source for current
frontmatter role, consumes, produces, and context relationship fields. This map
adds the domain interpretation agents need before changing skill bodies.

## Map Rules

- Every shared skill appears exactly once in the roster below.
- Domain names are explicit; abbreviations such as BC1 are not the primary
  language.
- Ambiguous placements are marked `needs-follow-up`.
- Ports name the boundary. Adapters name concrete tools, CLIs, hooks, runtimes,
  files, or external systems.
- Raw child-skill prose should not cross boundaries by default. Prefer artifact
  links, execution packets, verdicts, criteria, findings, learnings, and
  handoffs.

## Domain Summary

| Skill domain | Responsibility | Count |
|---|---|---:|
| Domain Language and Standards | Shared vocabulary, contracts, and coding standards | 4 |
| Product and Discovery | Shape goals into researched, bounded intent | 8 |
| Planning and Work Graph | Turn intent into tracked slices, goals, scenarios, and work state | 5 |
| Execution and Orchestration | Drive implementation cycles, waves, and autonomous loops | 9 |
| Validation and Risk | Produce verdicts, tests, reviews, and risk evidence | 15 |
| Corpus and Learning Flywheel | Compile, retrieve, score, and adapt reusable context | 13 |
| Documentation and Skill Packaging | Author docs, skills, conversions, hygiene, and runtime packages | 10 |
| PR and Release Delivery | Plan, execute, validate, prepare, and ship PR/release work | 8 |
| Runtime Continuity and Operator State | Recover, summarize, report status, and keep the operator loop usable | 5 |
| **Total** |  | **77** |

## Domain Language and Standards

**Responsibility:** keep humans and agents using the same names, standards, and
shared contracts.

**Inbound ports:** vocabulary lookup, standards lookup, shared-contract lookup.

**Outbound ports:** canonical concept entry, coding standard, reusable skill
contract.

**Primary artifacts:** `skills/domain/references/*.md`,
`skills/standards/references/*.md`, shared reference files, stdout guidance.

**Adapters:** SKILL.md reference bodies, docs links, standards injector as a
guard adapter.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `domain` | domain | Owns ubiquitous language for agent work. |
| `shared` | domain | Owns shared AgentOps skill contracts. |
| `standards` | domain | Owns repo coding standards. |
| `using-agentops` | generic | Explains the workflow vocabulary to operators. |

## Product and Discovery

**Responsibility:** turn fuzzy goals and external facts into bounded, researched
intent.

**Inbound ports:** operator goal, product question, research question, upstream
API/doc question, dashboard question.

**Outbound ports:** BDD intent, discovery artifact, research finding, product
decision, execution packet seed.

**Primary artifacts:** `.agents/research/*.md`, `.agents/plans/*.md`,
`execution-packet.json`, product notes, external-doc summaries.

**Adapters:** search tools, external APIs, repo reads, council/design child
skills, domain/product docs.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `brainstorm` | domain | Separates goals from implementation choices. |
| `design` | domain | Validates product fit before discovery. |
| `discovery` | domain | Compiles dense discovery output and execution packets. |
| `grafana-platform-dashboard` | driven-adapter | Specialized discovery/validation adapter for dashboards. |
| `openai-docs` | driven-adapter | External-doc adapter for OpenAI surfaces. |
| `product` | domain | Shapes product intent and positioning. |
| `research` | driving-adapter | Explores code and facts into reusable findings. |
| `reverse-engineer-rpi` | supporting | Reconstructs product/spec intent from RPI behavior. |

## Planning and Work Graph

**Responsibility:** convert intent into trackable slices, issue graphs, goals,
and scenario boundaries.

**Inbound ports:** BDD intent issue, discovery packet, operator goal, scenario
candidate.

**Outbound ports:** slice plan, bd issue graph, GOALS update, scenario holdout,
autodev program.

**Primary artifacts:** `.agents/plans/*.md`, bd issues, `GOALS.md`, scenario
files, `PROGRAM.md`/autodev state.

**Adapters:** bd/br/bv, GOALS parser, scenario files, task-list fallback.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `autodev` | supporting | Manages bounded autonomous development loops. |
| `beads` | driven-adapter | Adapts issue graph storage and triage. |
| `goals` | domain | Owns measurable product fitness directives. |
| `plan` | domain | Decomposes BDD intent into slices and waves. |
| `scenario` | supporting | Manages holdout scenarios and test boundaries. |

## Execution and Orchestration

**Responsibility:** move planned work through implementation cycles and worker
coordination.

**Inbound ports:** issue/slice, execution packet, wave plan, operator cycle
request, worktree/workspace request.

**Outbound ports:** git changes, worker result, cycle ledger entry, implementation
summary, next-work item.

**Primary artifacts:** git diffs, `.agents/swarm/results/*.json`,
`.agents/rpi/*.md`, cycle histories, worktree state.

**Adapters:** Codex/Claude workers, Skill invocations, bd issue graph, git,
worktrees, daemon/schedule surfaces.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `codex-team` | supporting | Coordinates multiple Codex workers. |
| `crank` | domain | Executes epics through waves. |
| `dream` | supporting | Runs long compounding execution cycles. |
| `evolve` | supporting | Runs autonomous improvement loops. |
| `implement` | driving-adapter | Performs one tracked implementation issue. |
| `refactor` | supporting | Executes bounded refactors. |
| `rpi` | supporting | Orchestrates discovery, crank, and validation. |
| `scaffold` | supporting | Creates implementation starting surfaces. |
| `swarm` | supporting | Dispatches parallel agents. |

## Validation and Risk

**Responsibility:** challenge plans and work products before they become trusted
context or shipped code.

**Inbound ports:** plan, diff, artifact, PR, repo surface, security target,
dependency target.

**Outbound ports:** verdict, test plan, risk report, review finding, security
report, dependency report.

**Primary artifacts:** `result.json`, `verdict.json`, security reports, test
plans, review reports, validation summaries.

**Adapters:** tests, linters, security tools, council judges, CI gates,
repository scanners.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `bug-hunt` | domain | Investigates root causes and edge-case risks. |
| `complexity` | domain | Finds focused refactor hotspots. |
| `council` | domain | Runs multi-judge consensus. |
| `deps` | driven-adapter | Audits dependency risks and updates. |
| `perf` | domain | Profiles and optimizes hotspots. |
| `pre-mortem` | domain | Stress-tests plans before mutation. |
| `red-team` | supporting | Probes docs and skills for weaknesses. |
| `review` | driving-adapter | Reviews diffs, PRs, mocks, and code risk. |
| `scope` | driven-adapter | Adapts filesystem scope guardrails. |
| `security` | driven-adapter | Runs repository security scans. |
| `security-suite` | driven-adapter | Composes security analysis. |
| `test` | supporting | Generates tests and coverage plans. |
| `validate` | driving-adapter | Produces PASS/WARN/FAIL verdicts for inputs. |
| `validation` | domain | Runs post-implementation validation. |
| `vibe` | domain | Validates code readiness. |

## Corpus and Learning Flywheel

**Responsibility:** compile, retrieve, cite, score, and adapt context so it
compounds across sessions.

**Inbound ports:** transcript, learning candidate, citation, feedback signal,
research artifact, context request.

**Outbound ports:** context packet, learning, finding, pattern, wiki surface,
feedback event, ratchet record.

**Primary artifacts:** `.agents/learnings/*.md`, `.agents/findings/*.md`,
`.agents/patterns/*.md`, `.agents/research/*.md`, `.agents/ao/citations.jsonl`,
compiled wiki files.

**Adapters:** `ao lookup`/search, context assembly CLI, MemRL feedback, file
resolver, citation recorder, LLM wiki builder.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `compile` | supporting | Compiles `.agents` knowledge into wiki surfaces. |
| `curate` | supporting | Mines corpus and work state for durable deltas. |
| `flywheel` | domain | Checks knowledge flywheel health. |
| `forge` | domain | Mines transcripts into learnings. |
| `harvest` | supporting | Promotes `.agents` knowledge. |
| `inject` | driving-adapter | Loads relevant `.agents` context. `needs-follow-up`: deprecated CLI path must be reconciled with `ao context assemble` and `ao lookup`. |
| `knowledge-activation` | supporting | Activates mature knowledge into runtime context. |
| `llm-wiki` | supporting | Builds external-knowledge wikis. |
| `post-mortem` | domain | Reviews completed work and emits learning input. |
| `provenance` | driven-adapter | Traces artifact origin and citations. |
| `ratchet` | domain | Records one-way progress gates. |
| `retro` | domain | Captures session learnings. |
| `trace` | supporting | Traces decisions through artifacts. |

## Documentation and Skill Packaging

**Responsibility:** package knowledge into docs, skills, converted runtime
formats, and hygiene gates.

**Inbound ports:** documentation request, skill authoring request, conversion
request, hook authoring request, skill audit request.

**Outbound ports:** documentation, converted skill, skill audit report, hook
contract, install/update result.

**Primary artifacts:** docs markdown, `skills/*/SKILL.md`,
`skills-codex/*/SKILL.md`, converted-skill outputs, hook docs.

**Adapters:** markdown docs, Codex skill artifacts, Claude/OpenCode/Cursor
formats, heal scripts, hook manifests.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `bootstrap` | driving-adapter | Initializes project files and install surfaces. |
| `converter` | generic | Converts skill formats across runtimes. |
| `doc` | supporting | Generates and validates repo docs. |
| `heal-skill` | supporting | Repairs skill hygiene. |
| `hooks-authoring` | domain | Authors runtime hook contracts. `needs-follow-up`: may belong to Runtime once hook guard adapters are fully mapped. |
| `oss-docs` | generic | Scaffolds or audits OSS docs. |
| `readme` | generic | Drafts or improves README docs. |
| `skill-auditor` | supporting | Audits SKILL.md quality. |
| `skill-builder` | supporting | Scaffolds or absorbs SKILL.md files. |
| `update` | supporting | Syncs installed AgentOps skills. |

## PR and Release Delivery

**Responsibility:** package validated work for PRs, commits, and release
readiness.

**Inbound ports:** plan, diff, upstream repo context, validation verdict,
release checklist.

**Outbound ports:** PR plan, PR body, pushed branch, release verdict, PR retro,
validated PR scope.

**Primary artifacts:** git commits, PR body, release report, validation report,
PR retro artifact.

**Adapters:** git, GitHub CLI/API, upstream repositories, release scripts,
validation gates.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `pr-implement` | driving-adapter | Implements scoped OSS PR work. |
| `pr-plan` | supporting | Plans open source PR work. |
| `pr-prep` | driving-adapter | Prepares PR commits and body. |
| `pr-research` | driven-adapter | Researches upstream repositories. |
| `pr-retro` | supporting | Learns from PR outcomes. |
| `pr-validate` | driving-adapter | Validates PR scope and quality. |
| `push` | driving-adapter | Validates, commits, and pushes. |
| `release` | supporting | Runs release validation. |

## Runtime Continuity and Operator State

**Responsibility:** keep the operator session recoverable, resumable, and
healthy without adding resident context bloat.

**Inbound ports:** current session state, prior handoff, repo state, system
health signal, operator status request.

**Outbound ports:** handoff, status report, recovery summary, next action,
system cleanup action.

**Primary artifacts:** `.agents/research/*.md`, `.agents/rpi/*.md`, stdout
status, recovery notes.

**Adapters:** shell process inspection, git state, `.agents` state, handoff
files, operator CLI output.

| Skill | Current hexagonal role | Primary reason |
|---|---|---|
| `handoff` | supporting | Writes compact session handoffs. |
| `quickstart` | driving-adapter | Shows the next useful AgentOps action. |
| `recover` | driving-adapter | Recovers session context. |
| `status` | driving-adapter | Shows current AgentOps work status. |
| `system-tuning` | supporting | Restores system responsiveness. |

## First Refactor Guidance

Use this map to choose one domain and one boundary at a time. The first pilot is
Discovery to Planning:

```gherkin
Feature: Discovery hands dense intent to planning
  Scenario: Discovery delegates to plan
    Given discovery has research and design artifacts
    When it invokes planning
    Then the boundary is expressed as a named port
    And the handoff carries density fields and artifact links instead of raw child prose
```

Do not use this V0 map to retire or merge skills directly. Retire, merge, or
split decisions require the separate lease-on-life audit and a proof bead.
