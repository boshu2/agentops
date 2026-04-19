# Create Your First Skill

AgentOps is built out of skills. A good first contribution is not "make a huge framework." It is "teach the system one reusable intent."

This guide shows the smallest current path to adding a new skill without tripping the repo gates.

## Before You Start

Pick a skill idea that is:

- Narrow: one clear job, not an entire workflow
- Reusable: something you would invoke more than once
- Observable: it should produce an artifact, decision, or validation step someone can check

Good first-skill ideas:

- A focused validator for one common failure mode
- A domain-specific research or triage helper
- A contribution helper for one external tool or service
- A narrow knowledge-management skill that transforms one artifact type into another

Avoid first-skill ideas that:

- duplicate an existing skill in `docs/SKILLS.md`
- require a big new runtime abstraction
- mix discovery, implementation, and release into one entrypoint

## The Minimum Shape

Create a directory:

```bash
mkdir -p skills/your-skill-name
```

Then create `skills/your-skill-name/SKILL.md` using the current frontmatter contract:

```md
---
name: your-skill-name
description: 'What this skill does. Triggers: "trigger phrase", "other phrase".'
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

# your-skill-name

## Purpose

What this skill is for and what problem it solves.

## When to Use

- Trigger condition one
- Trigger condition two

## Inputs

- What the user should provide
- Any repo or runtime assumptions

## Instructions

1. The first concrete step.
2. The main execution flow.
3. The validation or closeout step.

## Output

- What artifact, decision, or side effect this skill should produce

## Examples

```text
Example prompt or invocation
```
```

Use [templates/skill.template.md](templates/skill.template.md) as a starting point if you want a copyable scaffold.

## Pick The Right Tier

Most first contributions should use one of these:

- `execution`: a focused task skill
- `session`: onboarding, status, or recovery help
- `knowledge`: transforms or traces knowledge artifacts
- `product`: product, docs, or release oriented
- `contribute`: contribution-specific workflow support

See [SKILL-API.md](SKILL-API.md) for the full frontmatter contract and [../skills/SKILL-TIERS.md](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md) for the full taxonomy.

## Keep The Entry Point Lean

Your `SKILL.md` should be the operator surface, not the whole encyclopedia.

If the skill needs more detail, add:

- `references/*.md` for deeper guidance
- `scripts/*.sh` for helper logic or validation
- `schemas/*.json` only when downstream tooling consumes a structured contract

If you add files under `references/`, make sure `SKILL.md` links to them. CI fails when reference files exist but are not linked.

## Common CI Footguns

These are the most common ways first skill PRs fail:

- Missing or stale frontmatter
- New references not linked from `SKILL.md`
- Adding a skill directory without syncing counts
- Leaving `TODO` or `FIXME` text in `SKILL.md`
- Adding symlinks anywhere in the repo

This repo is strict because skills are shipped artifacts, not informal notes.

## Validate Before You Open A PR

Run the current local checks:

```bash
# Required for any skill change
bash skills/heal-skill/scripts/heal.sh --strict

# Required when docs or skill counts change
bash tests/docs/validate-doc-release.sh

# If you added or removed a skill directory
scripts/sync-skill-counts.sh

# Recommended fast gate before push
scripts/pre-push-gate.sh --fast
```

If your change affects Codex behavior or the checked-in Codex bundle, also run:

```bash
bash scripts/audit-codex-parity.sh --skill your-skill-name
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

## Where To Look For Good Examples

Start from a simple, high-signal skill rather than the biggest orchestration layer.

Useful examples:

- [skills/research/SKILL.md](skills/research.md)
- [skills/retro/SKILL.md](skills/retro.md)
- [skills/doc/SKILL.md](skills/doc.md)
- [skills/implement/SKILL.md](skills/implement.md)

## Opening The PR

Explain four things clearly:

- the user intent the skill handles
- why an existing skill was not enough
- what artifact or outcome it produces
- what checks you ran locally

Useful supporting docs:

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [SKILL-API.md](SKILL-API.md)
- [testing-skills.md](testing-skills.md)
- [docs/SKILLS.md](SKILLS.md)
