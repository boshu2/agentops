# CLI ↔ Skills Wiring Map

> Which `ao` commands are called by which skills — and vice versa.

Auto-audited 2026-04-24; targeted runtime-proof update 2026-04-28. 80 generated CLI command headings, 69 source skills. (AgentOps 3.0 is hookless — there is no runtime hook surface; lifecycle work is driven by skills + the `ao` CLI, with the installed local cockpit pre-push gate as routine authority and CI as tag/PR/manual backstop telemetry.)

Registry-first note: `/plan`, `/pre-mortem`, `/research`, `/vibe`, and `/post-mortem` now also read or write `.agents/findings/registry.jsonl` directly via skill contract. Those file-native prevention reads are intentionally not counted as `ao` command invocations in the tables below.

## Summary

| Category | Count |
|----------|-------|
| Generated CLI command headings | 59 |
| CLI command entries with skill callers | 32 |
| Orphan/hidden command entries (user utilities, hidden, CI-only) | 21 |
| Known phantom subcommands | 0 |

---

## CLI Commands → Callers

Every `ao` command that is actively called by at least one skill.

| Command | Skill Callers |
|---------|--------------|
| `ao inject` | crank, evolve, implement, inject, recover, research, retro |
| `ao forge` | flywheel, forge, post-mortem, retro, vibe, evolve, crank |
| `ao ratchet` | crank, handoff, implement, plan, pre-mortem, ratchet, rpi, status, vibe |
| `ao goals` | goals, evolve |
| `ao search` | crank, inject, plan, pre-mortem, provenance, research, using-agentops, vibe |
| `ao rpi` | autodev, council, crank, plan, quickstart, research, rpi, shared, swarm |
| `ao autodev` | autodev |
| `ao flywheel` | crank, evolve, flywheel, post-mortem, quickstart, retro, status |
| `ao pool` | crank, status |
| `ao lookup` | crank, implement, inject, plan, pre-mortem, research, using-agentops |
| `ao context` | crank, implement, swarm |
| `ao codex` | autodev, brainstorm, crank, discovery, handoff, implement, post-mortem, quickstart, recover, research, rpi, status, using-agentops, validation |
| `ao compile` | compile |
| `ao maturity` | flywheel |
| `ao constraint` | flywheel, post-mortem, retro |
| `ao badge` | flywheel, status |
| `ao retrieval-bench` | flywheel |
| `ao seed` | quickstart |
| `ao notebook` | retro |
| `ao dedup` | flywheel |
| `ao contradict` | flywheel |
| `ao metrics` | flywheel |
| `ao init` | quickstart |
| `ao session` | post-mortem, retro |
| `ao temper` | post-mortem |
| `ao curate` | flywheel |
| `ao status` | flywheel, quickstart |
| `ao task-feedback` | retro |
| `ao task-status` | status |
| `ao anti-patterns` | flywheel |

---

## Skills → Commands

Which `ao` commands each skill invokes.

| Skill | ao Commands Used |
|-------|-----------------|
| **autodev** | `autodev init`, `autodev validate`, `codex ensure-start`, `evolve`, `rpi` |
| **brainstorm** | `codex ensure-start` |
| **compile** | `compile` |
| **crank** | `codex ensure-start`, `context assemble`, `flywheel close-loop`, `flywheel status`, `forge transcript`, `inject`, `lookup`, `pool list`, `ratchet record`, `ratchet status`, `rpi phased`, `search` |
| **discovery** | `codex ensure-start` |
| **evolve** | `forge`, `goals measure`, `inject` |
| **flywheel** | `badge`, `constraint review`, `contradict`, `curate status`, `dedup`, `maturity`, `metrics cite-report`, `metrics health`, `anti-patterns`, `retrieval-bench`, `status` |
| **forge** | `forge markdown`, `forge transcript` |
| **goals** | `goals add`, `goals drift`, `goals export`, `goals history`, `goals init`, `goals measure`, `goals meta`, `goals migrate`, `goals prune`, `goals steer`, `goals validate` |
| **handoff** | `codex ensure-stop`, `ratchet status` |
| **implement** | `codex ensure-start`, `context assemble`, `lookup`, `ratchet record`, `ratchet skip`, `ratchet spec`, `ratchet status` |
| **inject** | `inject`, `lookup`, `search` |
| **plan** | `lookup`, `ratchet record`, `rpi cleanup`, `rpi status`, `search` |
| **post-mortem** | `codex ensure-stop`, `constraint activate`, `flywheel close-loop`, `forge`, `forge markdown`, `session close`, `temper validate` |
| **pre-mortem** | `lookup`, `ratchet record`, `search` |
| **provenance** | `search` |
| **quickstart** | `codex ensure-start`, `codex ensure-stop`, `codex status`, `flywheel status`, `hooks install`, `hooks test`, `init`, `quick-start`, `quickstart`, `rpi phased`, `seed`, `status` |
| **ratchet** | `ratchet check`, `ratchet record`, `ratchet skip`, `ratchet status` |
| **recover** | `codex ensure-start`, `codex status`, `lookup` |
| **research** | `codex ensure-start`, `inject`, `lookup`, `rpi phased`, `search` |
| **retro** | `constraint activate`, `constraint review`, `flywheel close-loop`, `forge`, `forge markdown`, `notebook update`, `session close`, `task-feedback` |
| **rpi** | `codex ensure-start`, `ratchet record`, `rpi cancel`, `rpi cleanup` |
| **status** | `badge`, `codex ensure-start`, `flywheel status`, `pool list`, `pool promote`, `pool stage`, `ratchet status`, `task-status` |
| **swarm** | `context assemble`, `rpi phased` |
| **using-agentops** | `codex ensure-start`, `codex ensure-stop`, `codex status`, `lookup`, `search` |
| **validation** | `codex ensure-stop`, `forge transcript` |
| **vibe** | `forge markdown`, `ratchet record`, `search` |
| council | `rpi phased` |
| shared | `rpi phased` |

Skills with **no ao commands**: beads, brainstorm, bug-hunt, codex-team, complexity, converter, doc, heal-skill, llm-wiki, openai-docs, pr-implement, pr-prep, pr-research, pr-validate, product, release, reverse-engineer-rpi, security, security-suite, standards, trace.

Conceptual slash commands such as `/knowledge` are documented elsewhere in the product docs, but they are not counted as source skill directories in this map.

## Repo-Native Prevention Surfaces

These are active skill-level reads or writes that do not go through an `ao` subcommand:

- `/plan` reads `.agents/findings/registry.jsonl` before decomposition and cites `Applied findings:`
- `/research` persists reusable findings to `.agents/findings/registry.jsonl`
- `/pre-mortem` reads `.agents/findings/registry.jsonl` in both quick and deep modes, injects `known_risks`, and can persist reusable findings
- `/vibe` reads `.agents/findings/registry.jsonl` before council review and can persist reusable findings
- `/post-mortem` writes normalized reusable findings to `.agents/findings/registry.jsonl`

---

## Orphan Commands

Commands that exist in the Go CLI but are not called by any skill. All are intentionally uncalled — user utilities, hidden internals, or CI-only. (`ao memory` and `ao extract` were previously invoked only by runtime hooks, which AgentOps 3.0 no longer ships; they remain available as direct commands.)

| Command | Category | Notes |
|---------|----------|-------|
| `ao completion` | User utility | Shell completion generation |
| `ao config` | User utility | Config management |
| `ao demo` | User utility | Council-first AgentOps 3.0 demonstration |
| `ao doctor` | CI/install | Called by install.sh and release-smoke-test.sh |
| `ao eval` | CI/test | Public AgentOps canary suites and baseline comparisons |
| `ao version` | User utility | Version query |
| `ao quick-start` / `ao quickstart` | User utility | Golden path for repo seed; `/quickstart` routes users to the next action |
| `ao vibe-check` | User utility | `/vibe` skill orchestrates directly |
| `ao trace` | User utility | Artifact tracing |
| `ao gate` | CI/test | Promotion gate — called in test scripts |
| `ao memory` | Internal | Memory sync (previously a hook caller) |
| `ao extract` | Internal | Learning extraction (previously a hook caller) |
| `ao feedback` | Hidden | UI for providing feedback on learnings |
| `ao feedback-loop` | Internal | Async feedback processing |
| `ao batch-feedback` | Hidden | Batch feedback processing |
| `ao session-outcome` | Hidden | Session outcome recording |
| `ao store` | Hidden | Vector store management |
| `ao index` | Hidden | Indexing utility |
| `ao task-sync` | Hidden | Task synchronization |
| `ao migrate` | Hidden | Migration utility (`migrate memrl`) |
| `ao worktree` | Hidden | Worktree GC utility |

---

## Phantom Subcommands

No known phantom subcommands are present in the current map. `scripts/validate-cli-skills-map.sh` fails if removed paths or the known stale gate/forge calls reappear.

---

## Session Lifecycle Flow (hookless)

AgentOps 3.0 ships no runtime hooks. Session-boundary maintenance is driven explicitly by skills calling `ao` commands. For Codex, the entry/closeout skills replace what runtime hooks used to do:

```
Codex Thread Entry
  → entry skill runs ao codex ensure-start
      → first call performs ao codex start semantics once per thread
      → later calls no-op for the same thread
      → ao flywheel close-loop (safe maintenance)
      → ao lookup citation writes for surfaced artifacts

During Session
  → ao lookup
      → appends citations to .agents/ao/citations.jsonl
  → ao search --cite <type>
      → appends citations to .agents/ao/citations.jsonl when search results are adopted

Codex Thread Closeout
  → closeout-owner skill runs ao codex ensure-stop
      → first call performs ao codex stop semantics once per thread
      → later calls no-op for the same thread
      → ao forge transcript (archived transcript or history fallback)
      → ao flywheel close-loop
      → ao dedup
      → ao contradict
      → ao maturity --expire --evict --curate

Codex Health
  → ao codex status
      → reads capture / retrieval / promotion / citation health
```

If you want a runtime hook of your own (e.g. a PostToolUse gate), author one with the `hooks-authoring` skill — AgentOps ships none by default.

---

## Regenerating

When skills or command usage changes, refresh this map as follows:

1. Re-scan source invocations in: `skills/*/SKILL.md`, `skills-codex/*/SKILL.md`.
2. Update the relevant rows in this document, keeping hidden/subcommands aligned with the live command tree (`ao anti-patterns`, `ao context assemble`, etc.).
3. Update the audit header date above.

## Maintaining This Document

Re-audit when:
- Adding a new `ao` CLI command (check it has skill callers or is intentionally orphaned)
- Adding a new skill that calls `ao` commands (verify the commands exist)
- Running `scripts/generate-cli-reference.sh` (companion to this doc)
