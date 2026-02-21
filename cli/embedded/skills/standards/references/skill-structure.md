# Skill Structure Standard

**Version:** 2.0.0
**Last Updated:** 2026-02-20
**Source:** Claude Code official documentation (https://code.claude.com/docs/en/skills)
**Purpose:** Defines the required structure, frontmatter, and quality standards for all AgentOps skills.

---

## Table of Contents

1. [File Structure](#file-structure)
2. [YAML Frontmatter](#yaml-frontmatter)
3. [Description Field](#description-field)
4. [Body Structure](#body-structure)
5. [Progressive Disclosure](#progressive-disclosure)
6. [Quality Checklist](#quality-checklist)
7. [AgentOps Extensions](#agentops-extensions)

---

## File Structure

```
skill-name/
├── SKILL.md              # Required — exact case, no variations
├── scripts/              # Optional — executable code
├── references/           # Optional — progressive disclosure docs
└── assets/               # Optional — templates, fonts, icons
```

### Rules

| Rule | ALWAYS | NEVER |
|------|--------|-------|
| Entry point | `SKILL.md` (exact case) | `skill.md`, `SKILL.MD`, `Skill.md` |
| Folder name | kebab-case (`bug-hunt`) | spaces, underscores, capitals |
| Name match | Folder name = `name:` field | Mismatch between folder and frontmatter |
| README | None inside skill folder | `README.md` in skill directories |
| Reserved | Any valid kebab-case name | `claude-*` or `anthropic-*` prefixes |

---

## YAML Frontmatter

### Required Fields

```yaml
---
name: skill-name
description: 'What it does. When to use it. Trigger phrases.'
---
```

Only `description` is technically required (recommended). If `name` is omitted, the directory name is used.

### All Claude Code Frontmatter Fields

| Field | Required | Purpose |
|-------|----------|---------|
| `name` | No | Display name. Lowercase letters, numbers, hyphens only (max 64 chars). Defaults to directory name. |
| `description` | Recommended | What the skill does and when to use it. Claude uses this to decide when to load the skill. |
| `argument-hint` | No | Hint shown during autocomplete (e.g., `[issue-number]`, `[filename] [format]`). |
| `disable-model-invocation` | No | Set to `true` to prevent Claude from auto-loading. User must invoke with `/name`. Default: `false`. |
| `user-invocable` | No | Set to `false` to hide from `/` menu. Use for background knowledge. Default: `true`. |
| `allowed-tools` | No | Tools Claude can use without permission when skill is active (e.g., `Read, Grep, Glob`). |
| `model` | No | Model to use when skill is active (`sonnet`, `opus`, `haiku`, `inherit`). |
| `context` | No | Set to `fork` to run in a forked subagent context. |
| `agent` | No | Which subagent type to use when `context: fork` is set (e.g., `Explore`, `Plan`, `general-purpose`). |
| `hooks` | No | Hooks scoped to this skill's lifecycle. |

### Invocation Control Matrix

| Frontmatter | User can invoke | Claude can invoke | Context loading |
|-------------|----------------|-------------------|-----------------|
| (default) | Yes | Yes | Description always in context, full skill loads when invoked |
| `disable-model-invocation: true` | Yes | No | Description not in context, full skill loads when user invokes |
| `user-invocable: false` | No | Yes | Description always in context, full skill loads when invoked |

### String Substitutions

| Variable | Description |
|----------|-------------|
| `$ARGUMENTS` | All arguments passed when invoking the skill |
| `$ARGUMENTS[N]` | Specific argument by 0-based index |
| `$N` | Shorthand for `$ARGUMENTS[N]` |
| `${CLAUDE_SESSION_ID}` | Current session ID |

### Dynamic Context Injection

The `` !`command` `` syntax runs shell commands before skill content is sent to Claude:

```yaml
## Context
- Current branch: !`git branch --show-current`
- Recent changes: !`git log --oneline -5`
```

### AgentOps Extension Fields (under `metadata:`)

AgentOps uses these custom fields under `metadata:` for tooling integration:

```yaml
metadata:
  tier: solo          # solo, team, orchestration, library, background, meta
  dependencies:       # List of skill names this skill depends on
    - standards
    - council
  internal: true      # true for non-user-facing skills
  replaces: old-name  # Deprecated skill this replaces
```

**Tier values and their constraints:**

| Tier | Max Lines | Purpose |
|------|-----------|---------|
| `solo` | 200 | Single-agent, no spawning |
| `team` | 500 | Spawns workers |
| `orchestration` | 500 | Coordinates multiple skills/teams |
| `library` | 200 | Referenced by other skills, not invoked directly |
| `background` | 200 | Hooks/automation, not user-invoked |
| `meta` | 200 | Explains the system itself |

### Security Restrictions

- No XML angle brackets (`<` `>`) in frontmatter
- No `claude` or `anthropic` in skill names
- YAML safe parsing only (no code execution)

---

## Description Field

The description is the **most critical field** — it determines when Claude loads the skill.

### Structure

```
[What it does] + [When to use it] + [Key capabilities]
```

### Requirements

- Under 1024 characters
- MUST include trigger phrases users would actually say
- MUST explain what the skill does (not just when)
- No XML tags

### Good Examples

```yaml
# Specific + actionable + triggers
description: 'Investigate suspected bugs with git archaeology and root cause analysis. Triggers: "bug", "broken", "doesn''t work", "failing", "investigate bug".'

# Clear value prop + multiple triggers
description: 'Comprehensive code validation. Runs complexity analysis then multi-model council. Answer: Is this code ready to ship? Triggers: "vibe", "validate code", "check code", "review code", "is this ready".'
```

### Bad Examples

```yaml
# Too vague
description: Helps with projects.

# Missing triggers
description: Creates sophisticated multi-page documentation systems.

# Too technical, no user triggers
description: Implements the Project entity model with hierarchical relationships.
```

### Internal Skills Exception

Library/background/meta skills that are auto-loaded (not user-invoked) may describe their loading mechanism instead of user triggers:

```yaml
description: 'Auto-loaded by /vibe, /implement based on file types.'
```

---

## Body Structure

### Recommended Template

```markdown
---
name: skill-name
description: '...'
metadata:
  tier: solo
---

# Skill Name

## Quick Start

Example invocations showing common usage patterns.

## Instructions

### Step 1: [First Major Step]
Specific, actionable instructions with exact commands.

### Step 2: [Next Step]
...

## Examples

### Example 1: [Common scenario]
User says: "..."
Actions: ...
Result: ...

## Troubleshooting

### Error: [Common error]
Cause: ...
Solution: ...
```

### Requirements

| Aspect | Requirement |
|--------|-------------|
| Size | Under 5,000 words; keep SKILL.md under 500 lines |
| Instructions | Specific and actionable (exact commands, not "validate the data") |
| Examples | At least 2-3 usage examples for user-facing skills |
| Error handling | Troubleshooting section for common failures |
| References | Link to `references/` for detailed docs (don't inline everything) |

---

## Progressive Disclosure

Skills use three levels:

1. **Frontmatter** — Always in system prompt. Minimal: name + description.
2. **SKILL.md body** — Loaded when skill is relevant. Core instructions.
3. **references/** — Loaded on-demand. Detailed docs, schemas, examples.

### Rules

- Keep SKILL.md focused on core workflow
- Move detailed reference material to `references/`
- Explicitly link to references: "Read `references/api-patterns.md` for..."
- Move scripts >20 lines to `scripts/` directory
- Move inline bash >30 lines to `scripts/` or `references/`

---

## Quality Checklist

### Before Commit

- [ ] `SKILL.md` exists (exact case)
- [ ] Folder name matches `name:` field
- [ ] Folder name is kebab-case
- [ ] Description includes WHAT + WHEN (triggers)
- [ ] Description under 1024 characters
- [ ] No XML tags in frontmatter
- [ ] No `claude`/`anthropic` in name
- [ ] `metadata.tier` is set and valid
- [ ] SKILL.md under 5,000 words
- [ ] User-facing skills have examples section
- [ ] User-facing skills have troubleshooting section
- [ ] Detailed docs in references/, not inlined
- [ ] No README.md in skill folder

### Trigger Testing

- [ ] Triggers on 3+ obvious phrases
- [ ] Triggers on paraphrased requests
- [ ] Does NOT trigger on unrelated topics

---

## AgentOps Extensions

These are AgentOps-specific patterns not in the Claude Code spec:

### Tier System

Controls line limits and categorization. Enforced by `tests/skills/lint-skills.sh`.

### Dependencies

Declared under `metadata.dependencies`. Validated by `tests/skills/validate-skill.sh`.

### Skill Tiers Document

Full taxonomy at `skills/SKILL-TIERS.md`.

### Standards Loading

Language standards loaded JIT by `/vibe`, `/implement` — see `standards-index.md`.
