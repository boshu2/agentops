# CLI ↔ Skills/Hooks Wiring Map

> Which `ao` commands are called by which skills and hooks — and vice versa.

Auto-audited 2026-03-24. 53 CLI commands, 52 source skills, 7 runtime hook event sections.

Source-of-truth note: `hooks/hooks.json` currently declares 7 runtime hook event sections. Repository hook scripts such as `worktree-setup.sh` are support/setup scripts and are listed separately when relevant.

Registry-first note: `/plan`, `/pre-mortem`, `/vibe`, and `/post-mortem` now also read or write `.agents/findings/registry.jsonl` directly via skill contract. Those file-native prevention reads are intentionally not counted as `ao` command invocations in the tables below.

## Summary

| Category | Count |
|----------|-------|
| CLI commands with skill/hook callers | 30 |
| Orphan commands (user utilities, hidden, CI-only) | 20 |
| Phantom subcommands (bugs) | 2 |

---

## CLI Commands → Callers

Every `ao` command that is actively called by at least one skill or hook.

| Command | Skill Callers | Hook Callers |
|---------|--------------|--------------|
| `ao inject` | crank, evolve, implement, inject, recover, research, retro | session-start.sh, worktree-setup.sh |
| `ao forge` | flywheel, forge, post-mortem, retro, vibe, evolve, crank | session-end-maintenance.sh |
| `ao ratchet` | crank, handoff, implement, plan, pre-mortem, ratchet, rpi, status, vibe | ratchet-advance.sh, stop-auto-handoff.sh, prompt-nudge.sh, precompact-snapshot.sh |
| `ao goals` | goals, evolve | — |
| `ao search` | crank, inject, plan, pre-mortem, provenance, research, using-agentops, vibe | session-start.sh |
| `ao rpi` | council, crank, plan, quickstart, research, rpi, shared, swarm | — |
| `ao flywheel` | crank, evolve, flywheel, post-mortem, quickstart, retro, status | ao-flywheel-close.sh |
| `ao pool` | crank, status | session-end-maintenance.sh |
| `ao lookup` | crank, implement, inject, plan, research, using-agentops | session-start.sh |
| `ao context` | crank, implement, swarm | context-guard.sh |
| `ao codex` | brainstorm, crank, discovery, handoff, implement, post-mortem, quickstart, recover, research, rpi, status, using-agentops, validation | — |
| `ao maturity` | flywheel | session-end-maintenance.sh |
| `ao constraint` | flywheel, post-mortem, retro | — |
| `ao badge` | flywheel, status | — |
| `ao seed` | quickstart | — |
| `ao notebook` | retro | session-start.sh |
| `ao memory` | — | session-end-maintenance.sh |
| `ao dedup` | flywheel | session-end-maintenance.sh |
| `ao contradict` | flywheel | session-end-maintenance.sh |
| `ao metrics` | flywheel | — |
| `ao extract` | — | session-start.sh |
| `ao hooks` | quickstart | — |
| `ao init` | quickstart | — |
| `ao session` | post-mortem, retro | — |
| `ao temper` | post-mortem | — |
| `ao curate` | flywheel | — |
| `ao status` | flywheel, quickstart | — |
| `ao task-feedback` | retro | — |
| `ao task-status` | status | — |
| `ao anti-patterns` | flywheel | — |

---

## Skills → Commands

Which `ao` commands each skill invokes.

| Skill | ao Commands Used |
|-------|-----------------|
| **brainstorm** | `codex ensure-start` |
| **crank** | `codex ensure-start`, `context assemble`, `flywheel close-loop`, `flywheel status`, `forge transcript`, `inject`, `lookup`, `pool list`, `ratchet record`, `ratchet status`, `rpi phased`, `search` |
| **discovery** | `codex ensure-start` |
| **evolve** | `forge`, `goals measure`, `inject` |
| **flywheel** | `badge`, `constraint review`, `contradict`, `curate status`, `dedup`, `maturity`, `metrics cite-report`, `metrics health`, `anti-patterns`, `status` |
| **forge** | `forge markdown`, `forge transcript` |
| **goals** | `goals add`, `goals drift`, `goals export`, `goals history`, `goals init`, `goals measure`, `goals meta`, `goals migrate`, `goals prune`, `goals steer`, `goals validate` |
| **handoff** | `codex ensure-stop`, `ratchet status` |
| **implement** | `codex ensure-start`, `context assemble`, `lookup`, `ratchet record`, `ratchet skip`, `ratchet spec`, `ratchet status` |
| **inject** | `inject`, `lookup`, `search` |
| **plan** | `ratchet record`, `rpi cleanup`, `rpi status`, `search` |
| **post-mortem** | `codex ensure-stop`, `constraint activate`, `flywheel close-loop`, `forge`, `forge markdown`, `session close`, `temper validate` |
| **pre-mortem** | `ratchet record`, `search` |
| **provenance** | `search` |
| **quickstart** | `codex ensure-start`, `codex ensure-stop`, `codex status`, `flywheel status`, `hooks install`, `hooks test`, `init`, `rpi phased`, `seed`, `status` |
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

Skills with **no ao commands**: beads, brainstorm, bug-hunt, codex-team, complexity, converter, doc, heal-skill, openai-docs, oss-docs, pr-implement, pr-plan, pr-prep, pr-research, pr-retro, pr-validate, product, readme, release, reverse-engineer-rpi, security, security-suite, standards, trace, update.

Conceptual slash commands such as `/knowledge` are documented elsewhere in the product docs, but they are not counted as source skill directories in this map.

## Repo-Native Prevention Surfaces

These are active skill-level reads or writes that do not go through an `ao` subcommand:

- `/plan` reads `.agents/findings/registry.jsonl` before decomposition and cites `Applied findings:`
- `/pre-mortem` reads `.agents/findings/registry.jsonl` in both quick and deep modes, injects `known_risks`, and can persist reusable findings
- `/vibe` reads `.agents/findings/registry.jsonl` before council review and can persist reusable findings
- `/post-mortem` writes normalized reusable findings to `.agents/findings/registry.jsonl`

---

## Hooks → Commands

Which `ao` commands each hook invokes.

| Hook File | Event | ao Commands |
|-----------|-------|-------------|
| **session-start.sh** | SessionStart | `extract`, `inject`, `lookup`, `notebook update`, `search` |
| **session-end-maintenance.sh** | SessionEnd | `contradict`, `dedup`, `forge transcript`, `maturity`, `memory sync`, `notebook update`, `pool ingest` |
| **ao-flywheel-close.sh** | Stop | `flywheel close-loop` |
| **ratchet-advance.sh** | PostToolUse | `ratchet record` |
| **context-guard.sh** | UserPromptSubmit | `context guard` |
| **prompt-nudge.sh** | UserPromptSubmit | `ratchet status` |
| **precompact-snapshot.sh** | PreCompact | `ratchet status` |
| **stop-auto-handoff.sh** | Stop | `ratchet status` |
| **worktree-setup.sh** | setup script (outside `hooks/hooks.json`) | `inject` |

Hooks with **no ao commands**: citation-tracker.sh, config-change-monitor.sh, constraint-compiler.sh, dangerous-git-guard.sh, git-worker-guard.sh, pending-cleaner.sh, pre-mortem-gate.sh, skill-lint-gate.sh, standards-injector.sh, stop-team-guard.sh, subagent-stop.sh, task-validation-gate.sh, worktree-cleanup.sh.

---

## Orphan Commands

Commands that exist in the Go CLI but are not called by any skill or hook. All are intentionally uncalled — user utilities, hidden internals, or CI-only.

| Command | Category | Notes |
|---------|----------|-------|
| `ao completion` | User utility | Shell completion generation |
| `ao config` | User utility | Config management |
| `ao demo` | User utility | Interactive demonstration |
| `ao doctor` | CI/install | Called by install.sh and release-smoke-test.sh |
| `ao version` | User utility | Version query |
| `ao quick-start` | User utility | `/quickstart` skill is the orchestrator |
| `ao vibe-check` | User utility | `/vibe` skill orchestrates directly |
| `ao plans` | User utility | Plan management |
| `ao trace` | User utility | Artifact tracing |
| `ao gate` | CI/test | Promotion gate — called in test scripts |
| `ao feedback` | Hidden | UI for providing feedback on learnings |
| `ao feedback-loop` | Internal | Async feedback processing |
| `ao batch-feedback` | Hidden | Batch feedback processing |
| `ao session-outcome` | Hidden | Session outcome recording |
| `ao store` | Hidden | Vector store management |
| `ao index` | Hidden | Indexing utility |
| `ao task-sync` | Hidden | Task synchronization |
| `ao migrate` | Hidden | Migration utility (`migrate memrl`) |
| `ao worktree` | Hidden | Worktree GC utility |
| `ao anti-patterns` | Hidden | Anti-pattern list |

---

## Phantom Subcommands (Bugs)

References to subcommands that don't exist under their parent command.

| Phantom Call | Location | Problem | Fix |
|-------------|----------|---------|-----|
| `ao gate check` | `tests/rpi-e2e/run-full-rpi.sh:172,176,217` | `check` is a subcommand of `ao ratchet`, not `ao gate` | Change to `ao ratchet check` |
| `ao forge index` | `scripts/test-flywheel.sh:94` | `index` doesn't exist under `forge` (has: `transcript`, `markdown`, `batch`) | **Fixed** — changed to `ao forge markdown` |

---

## Session Lifecycle Flow

How hooks chain `ao` commands across a session:

```
Session Start
  → session-start.sh
      → ao extract (lean mode: extract + inject with auto-shrink)
      → ao inject
      → ao lookup (JIT knowledge retrieval)

During Session
  → ratchet-advance.sh (PostToolUse)
      → ao ratchet record
  → context-guard.sh (UserPromptSubmit)
      → ao context guard
  → prompt-nudge.sh (UserPromptSubmit)
      → ao ratchet status
  → citation-tracker.sh (PostToolUse)
      → appends citation events to .agents/ao/citations.jsonl (no ao command)

Session End
  → session-end-maintenance.sh
      → ao forge transcript
      → ao notebook update
      → ao memory sync
      → ao pool ingest
      → ao maturity --expire --evict
      → ao dedup
      → ao contradict

Stop Event
  → ao-flywheel-close.sh
      → ao flywheel close-loop (citation → utility → maturity)
  → stop-auto-handoff.sh
      → ao ratchet status (check for incomplete gates)

Pre-Compaction
  → precompact-snapshot.sh
      → ao ratchet status (snapshot before context loss)
```

## Codex Hookless Lifecycle Flow

How Codex sessions replace missing runtime hooks with explicit lifecycle commands:

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

Codex Health
  → ao codex status
      → reads capture / retrieval / promotion / citation health
```

---

## Regenerating

When skills, hooks, or command usage changes, refresh this map as follows:

1. Re-scan source invocations in: `skills/*/SKILL.md`, `skills-codex/*/SKILL.md`, `hooks/*.sh`, `hooks/hooks.json`.
2. Update the relevant rows in this document, keeping hidden/subcommands aligned with the live command tree (`ao anti-patterns`, `ao context assemble`, etc.).
3. Run `bash scripts/validate-hooks-doc-parity.sh` and ensure no stale hook-count wording remains.
4. Update the audit header date above.

## Maintaining This Document

Re-audit when:
- Adding a new `ao` CLI command (check it has skill/hook callers or is intentionally orphaned)
- Adding a new skill that calls `ao` commands (verify the commands exist)
- Adding a new hook that calls `ao` commands
- Running `scripts/generate-cli-reference.sh` (companion to this doc)
