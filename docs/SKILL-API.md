# Skill API Reference

> Definitive reference for AgentOps SKILL.md frontmatter fields. Schema: `schemas/skill-frontmatter.v1.schema.json`.

Publicly, AgentOps talks about bookkeeping, validation, primitives, and flows. This document describes the internal API and taxonomy behind that operating model.

## Frontmatter Format

Every skill has a YAML frontmatter block between `---` delimiters at the top of `SKILL.md`:

```yaml
---
name: my-skill
description: 'What this skill does. Triggers: "keyword1", "keyword2".'
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
---
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier (must match directory name) |
| `description` | string | What the skill does, including trigger phrases |
| `skill_api_version` | integer | Always `1` (const) |

## Optional Fields

### `context`

Controls what knowledge `ao lookup --for=<skill>` provides. Two forms:

**String form** (backward compat):
```yaml
context: fork
```

**Object form** (recommended):
```yaml
context:
  window: isolated
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: full
```

#### `context.window`

How the skill's execution context relates to the parent session.

| Value | Meaning |
|-------|---------|
| `isolated` | Fresh context, no parent inheritance. For validation and mechanical skills. |
| `fork` | Copy parent context as starting point. For skills that need to know what you're working on. |
| `inherit` | Use full parent context as-is. For session utilities (status, handoff, recover). |

**v1 status:** Parsed and stored. Not enforced at runtime (Phase 2).

#### `context.sections`

Filter which knowledge sections are injected.

```yaml
sections:
  include: [INTEL, TASK]     # Allowlist — only these sections
  exclude: [HISTORY]         # Blocklist — everything except these
```

If both `include` and `exclude` are set, `include` takes precedence.

Valid section names:

| Section | Knowledge Fields |
|---------|-----------------|
| `HISTORY` | Past session summaries |
| `INTEL` | Learnings and patterns from the knowledge flywheel |
| `TASK` | Current bead ID and predecessor context |

**v1 status:** Actively enforced at runtime. `ao lookup --for=<skill>` zeroes excluded/non-included sections.

#### `context.intent.mode`

Declares what the skill is doing.

| Value | Meaning |
|-------|---------|
| `task` | Executing work (implement, plan, validate) |
| `questions` | Exploring or researching |
| `none` | Operational utility (status, push, update) |

**v1 status:** Parsed and stored. Not enforced at runtime (Phase 2 — orchestrators will use this to adapt behavior).

#### `context.intel_scope`

How much of the knowledge flywheel to inject.

| Value | Meaning |
|-------|---------|
| `full` | All learnings and patterns |
| `topic` | Only learnings matching the current query/task |
| `none` | No learnings or patterns injected |

**v1 status:** `none` is actively enforced (zeroes learnings + patterns). `topic` and `full` are declaration-only (Phase 2).

### `allowed-tools`

Restricts which tools the skill can auto-approve.

```yaml
# Array form
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash

# String form (comma-separated)
allowed-tools: Read, Grep, Glob, Bash
```

### `model`

Preferred model for skill execution.

```yaml
model: haiku    # Use cheaper/faster model for lightweight skills
```

Currently used by `flywheel` and `status`. Declaration-only — no CLI enforcement.

### `user-invocable`

Whether the skill appears in the slash-command list.

```yaml
user-invocable: true   # Shows as /skill-name
user-invocable: false  # Hidden from user, used by other skills
```

### `metadata`

Skill classification and dependency information.

```yaml
metadata:
  tier: execution           # See tier values below
  dependencies: [standards] # Skills loaded as context
  internal: false           # If true, not published externally
  version: "1.0.0"
  author: "Gas Town"
  triggers: ["keyword"]     # Additional trigger phrases
  replaces: old-skill-name  # Supersedes another skill
```

#### Tier Values

| Tier | Purpose | Example Skills |
|------|---------|----------------|
| `judgment` | Legacy internal tier name for validation and review gates | council, vibe, pre-mortem, post-mortem |
| `execution` | Single-task implementation | implement, bug-hunt, complexity, security-suite |
| `orchestration` | Multi-skill coordination | rpi, crank, swarm, evolve |
| `session` | Session lifecycle | handoff, recover, status, quickstart |
| `background` | Mechanical utilities | push, ratchet, flywheel, forge |
| `knowledge` | Knowledge management | compile, trace |
| `product` | Product strategy | product, readme, release, goals |
| `library` | Shared references | shared, standards, beads |
| `meta` | System-level | using-agentops, update, heal-skill |
| `contribute` | External contributions | pr-plan, pr-implement, pr-research, oss-docs |
| `cross-vendor` | Cross-platform | openai-docs, codex-team, converter, grafana-platform-dashboard |

### `output_contract`

Path to a JSON Schema file that defines the skill's structured output format.

```yaml
output_contract: skills/council/schemas/verdict.json
```

Used by downstream skills to validate that upstream output matches the expected schema. Paths are relative to repo root.

**v1 status:** Declaration-only. No runtime enforcement. Consumed by skill chaining validation tooling.

### Other Fields

| Field | Type | Description |
|-------|------|-------------|
| `license` | string | License identifier (e.g., `MIT`) |
| `compatibility` | string | Runtime requirements (e.g., `Requires git, gh CLI`) |

## Context Declaration Quick Reference

Core skills and their context policies:

| Skill | Window | Sections | Intent | Intel Scope |
|-------|--------|----------|--------|-------------|
| **Validation (tier: judgment)** | | | | |
| council | isolated | exclude: HISTORY | task | full |
| vibe | fork | exclude: HISTORY | task | — |
| pre-mortem | fork | exclude: HISTORY | task | — |
| post-mortem | fork | exclude: HISTORY | task | — |
| **Orchestration** | | | | |
| rpi | fork | — | — | — |
| crank | fork | exclude: HISTORY | task | full |
| swarm | fork | exclude: HISTORY | task | full |
| evolve | fork | exclude: HISTORY | task | full |
| **Execution** | | | | |
| implement | isolated | exclude: HISTORY | task | topic |
| bug-hunt | fork | exclude: HISTORY | task | topic |
| doc | fork | exclude: HISTORY | task | topic |
| complexity | fork | exclude: HISTORY | task | topic |
| security | fork | exclude: HISTORY | task | topic |
| security-suite | fork | exclude: HISTORY | task | topic |
| reverse-engineer-rpi | fork | exclude: HISTORY | task | topic |
| grafana-platform-dashboard | fork | exclude: HISTORY, TASK | questions | none |
| **Knowledge** | | | | |
| research | fork | exclude: HISTORY, TASK | questions | topic |
| trace | fork | exclude: HISTORY | task | full |
| compile | fork | exclude: TASK | task | full |
| forge | fork | exclude: TASK | task | full |
| flywheel | fork | exclude: TASK | task | full |
| retro | fork | — | — | — |
| **Session** | | | | |
| handoff | inherit | — | none | none |
| recover | inherit | — | none | none |
| status | inherit | — | none | none |
| quickstart | inherit | — | none | none |
| **Background** | | | | |
| push | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| ratchet | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| update | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| heal-skill | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| **Product** | | | | |
| product | fork | exclude: HISTORY | task | full |
| readme | fork | exclude: HISTORY | task | full |
| release | fork | exclude: HISTORY | task | full |
| goals | fork | exclude: HISTORY | task | topic |
| **Contribute** | | | | |
| pr-plan | fork | exclude: HISTORY | task | topic |
| pr-implement | fork | exclude: HISTORY | task | topic |
| pr-prep | fork | exclude: HISTORY | task | topic |
| pr-research | fork | exclude: HISTORY | task | topic |
| pr-retro | fork | exclude: HISTORY | task | topic |
| pr-validate | fork | exclude: HISTORY | task | topic |
| oss-docs | fork | exclude: HISTORY | task | topic |
| **Library/Meta** | | | | |
| shared | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| standards | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| using-agentops | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| converter | isolated | exclude: HISTORY, INTEL, TASK | none | none |
| beads | fork | exclude: HISTORY | task | topic |
| inject | fork | — | — | — |
| provenance | fork | — | — | — |
| **Cross-Vendor** | | | | |
| openai-docs | fork | exclude: HISTORY, TASK | questions | none |
| codex-team | fork | — | — | — |
| brainstorm | inherit | exclude: INTEL, HISTORY, TASK | none | none |
| plan | fork | — | task | topic |

## Enforcement Summary (v1)

| Field | Runtime Enforcement |
|-------|-------------------|
| `sections.include` | **Active** — zeroes non-included knowledge |
| `sections.exclude` | **Active** — zeroes excluded knowledge |
| `intel_scope: none` | **Active** — zeroes learnings + patterns |
| `intel_scope: topic/full` | Declaration-only (Phase 2) |
| `context.window` | Declaration-only (Phase 2) |
| `context.intent.mode` | Declaration-only (Phase 2) |
| `allowed-tools` | **Active** — controls auto-approval |
| `model` | Declaration-only |

## See Also

- [Skills Reference](SKILLS.md) — Skill descriptions and router
- [Skill Tiers](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md) — Taxonomy and dependency graph
- Schema: `schemas/skill-frontmatter.v1.schema.json`
