> Extracted from council/SKILL.md on 2026-04-11.

# Council Architecture & Execution Flow

## Context Budget Rule (CRITICAL)

Judges write ALL analysis to output files. Messages to the lead contain ONLY a
minimal completion signal: `{"type":"verdict","verdict":"...","confidence":"...","file":"..."}`.
The lead reads output files during consolidation. This prevents N judges from
exploding the lead's context window with N full reports via SendMessage.

**Consolidation runs inline as the lead** — no separate chairman agent. The lead
reads each judge's output file sequentially with the Read tool and synthesizes.

## Execution Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  Phase 1: Build Packet (JSON)                                   │
│  - Task type (validate/brainstorm/research)                     │
│  - Target description                                           │
│  - Context (files, diffs, prior decisions)                      │
│  - Perspectives to assign                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Phase 1a: Select spawn backend                                 │
│  codex_subagents | claude_teams | background_fallback           │
│  Team lead = spawner (this agent)                               │
└─────────────────────────────────────────────────────────────────┘
                              │
            ┌─────────────────┴─────────────────┐
            ▼                                   ▼
┌───────────────────────┐           ┌───────────────────────┐
│  RUNTIME-NATIVE JUDGES│           │     CODEX AGENTS      │
│ (spawn_agent or teams)│           │  (Bash tool, parallel)│
│                       │           │  Agent 1 (independent │
│  Agent 1 (independent │           │    or with preset)    │
│    or with preset)    │           │  Agent 2              │
│  Agent 2              │           │  Agent 3              │
│  Agent 3 (--deep only)│           │  (--mixed only)       │
│  (--deep/--mixed only)│           │                       │
│                       │           │  Output: JSON + MD    │
│  Write files, then    │           │  Files: .agents/      │
│ wait()/SendMessage to │           │    council/codex-*    │
│ lead                  │           │                       │
│  Files: .agents/      │           └───────────────────────┘
│    council/claude-*   │                       │
└───────────────────────┘                       │
            │                                   │
            └─────────────────┬─────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Phase 2: Consolidation (Team Lead — inline, no extra agent)    │
│  - Receive MINIMAL completion signals (verdict + file path)     │
│  - Read each judge's output file with Read tool                 │
│  - If schema_version is missing from a judge's output, treat    │
│    as version 0 (backward compatibility)                        │
│  - Compute consensus verdict                                    │
│  - Identify shared findings                                     │
│  - Surface disagreements with attribution                       │
│  - Generate Markdown report for human                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Phase 3: Cleanup                                               │
│  - Cleanup backend resources (close_agent / TeamDelete / none)  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Output: Markdown Council Report                                │
│  - Consensus: PASS/WARN/FAIL                                    │
│  - Shared findings                                              │
│  - Disagreements (if any)                                       │
│  - Recommendations                                              │
└─────────────────────────────────────────────────────────────────┘
```

## Step 1b: Load Project Reviewer Config

Check for project-level reviewer configuration before spawning judges:

```bash
REVIEWER_CONFIG=".agents/reviewer-config.md"
if [ -f "$REVIEWER_CONFIG" ]; then
    # Parse YAML frontmatter for reviewer list
    # Example .agents/reviewer-config.md:
    # ---
    # reviewers:
    #   - security-sentinel
    #   - architecture-strategist
    #   - code-simplicity-reviewer
    # plan_reviewers:
    #   - architecture-strategist
    # skip_reviewers:
    #   - performance-oracle
    # ---
    # Additional review context goes in the markdown body.
fi
```

If `reviewer-config.md` exists:
- Use `reviewers` list to select which judge perspectives to spawn
- Use `plan_reviewers` for plan validation specifically
- Use `skip_reviewers` to exclude perspectives even if preset includes them
- Pass markdown body as additional context to all judges

If no config exists, use defaults (current behavior unchanged).

For schema details and an example, see `reviewer-config-example.md`.

## Graceful Degradation

| Failure | Behavior |
|---------|----------|
| 1 of N agents times out | Proceed with N-1, note in report |
| Any Codex CLI pre-flight fails while `--mixed` is set | Hard error before spawning any judges; tell the operator to install/fix Codex CLI or drop `--mixed` |
| All Codex CLI agents fail after a successful pre-flight | Return error; `--mixed` cannot silently degrade to single-vendor review |
| All agents fail | Return error, suggest retry |
| Codex CLI not installed while `--mixed` is set | Hard error before spawning any judges |
| No multi-agent capability | Fall back to `--quick` (inline single-agent review) |
| No agent messaging | `--debate` unavailable, single-round review only |
| Output dir missing | Create `.agents/council/` automatically |

Timeout: 120s per agent (configurable via `--timeout=N` in seconds).

**Minimum quorum:** At least 1 agent must respond for a valid council. If 0 agents respond, return error.

## Effort Levels for Judges

Use the effort command to optimize token spend per judge role:

| Agent Role | Recommended Effort | Rationale |
|------------|-------------------|-----------|
| Judges (validate/research) | `low` | Judges review evidence, not implement — shallow reasoning suffices |
| Explorers | `low` | Fast breadth-first scanning |
| Chairman (consolidation) | `medium` | Needs balanced reasoning for consensus synthesis |

## Pre-Flight Checks

1. **Multi-agent capability:** Detect whether runtime supports spawning parallel subagents. If not, degrade to `--quick`.
2. **Agent messaging:** Detect whether runtime supports agent-to-agent messaging. If not, disable `--debate`.
3. **Codex CLI judges (--mixed only):** Check `which codex`, test model availability, test `--output-schema` support. Hard-error before spawning any judges when unavailable.
4. **Agent count:** Verify `judges * (1 + explorers) <= MAX_AGENTS (12)`
5. **Output dir:** `mkdir -p .agents/council`
