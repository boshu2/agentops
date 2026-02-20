# Contributing to AgentOps

Thank you for your interest in contributing to AgentOps! This guide will help you create and submit high-quality skills.

## Table of Contents

- [Getting Started](#getting-started)
- [Ways to Contribute](#ways-to-contribute)
- [How to Add a Skill](#how-to-add-a-skill)
- [Skill Structure](#skill-structure)
- [Quality Standards](#quality-standards)
- [Testing Your Skill](#testing-your-skill)
- [Submission Process](#submission-process)
- [Code of Conduct](#code-of-conduct)

## Getting Started

### Prerequisites

- GitHub account
- Claude Code installed and configured
- Basic understanding of Claude Code plugins
- Familiarity with markdown and YAML frontmatter

### Fork and Clone

```bash
# Fork the repository on GitHub
# Then clone your fork
git clone https://github.com/YOUR_USERNAME/agentops.git
cd agentops
ao init          # Set up .agents/ dirs + .gitignore
ao init --hooks --full  # Optional: register all 8 lifecycle hooks
```

## Ways to Contribute

You do not need to create a new skill to contribute.

High-leverage contribution paths:
- Docs quality: README clarity, broken links, missing examples
- Testing and validation: smoke tests, scripts, CI checks
- CLI/runtime improvements: command UX, error handling, reliability
- Skills: new skills, existing skill fixes, reference updates

### First Contribution in 30 Minutes

1. Pick a small scope: typo fix, unclear sentence, broken link, or missing example.
2. Create a branch: `git checkout -b docs/first-contribution-fix`
3. Make your change and run a targeted check (for docs, verify links/commands you touched).
4. Commit and open a PR with clear before/after context.

For ideas, check open issues and prioritize items labeled `good first issue` or `documentation`.

## How to Add a Skill

### 1. Create Skill Directory

```bash
# Create your skill directory
mkdir -p skills/your-skill-name
```

### 2. Create SKILL.md

Create `skills/your-skill-name/SKILL.md` with YAML frontmatter:

```markdown
---
name: your-skill-name
description: 'Brief description of what your skill does. Trigger: /your-skill-name'
tier: solo
metadata:
  internal: false
dependencies:
  - council    # If your skill uses council for validation
---

# Your Skill Name

## Purpose

What this skill does and when to use it.

## Instructions

Step-by-step instructions for the AI agent executing this skill.

## Output

What this skill produces and where it writes results.

## Examples

Concrete usage examples.
```

**Required Frontmatter Fields:**
- `name`: Kebab-case identifier (e.g., `your-skill-name`)
- `description`: Clear, concise purpose with trigger pattern

**Recommended Frontmatter Fields:**
- `tier`: One of `solo`, `library`, `orchestration`, `team`, `background`, `meta` (see `skills/SKILL-TIERS.md`)
- `metadata.internal`: Set `true` for skills not intended for direct user invocation
- `dependencies`: List of skills this skill depends on

### 3. Add References (Optional)

For complex skills, keep SKILL.md lean and add detailed docs in `references/`:

```
skills/your-skill-name/
├── SKILL.md              # Entry point (lean)
├── references/           # Progressive disclosure docs
│   ├── patterns.md       # Detailed patterns
│   └── examples.md       # Extended examples
└── scripts/              # Validation scripts (optional)
    └── validate.sh
```

### 4. Add Validation Scripts (Optional)

If your skill has specific validation requirements, add a `scripts/validate.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
# Custom validation for your skill
# Exit 0 on success, non-zero on failure
```

## Skill Structure

### Directory Layout

```
skills/your-skill-name/
├── SKILL.md              # Skill entry point (required)
├── references/           # Detailed documentation (optional)
│   └── *.md
└── scripts/              # Validation scripts (optional)
    └── validate.sh
```

### File Naming Conventions

- **Skills:** `kebab-case/` (e.g., `bug-hunt/`)
- **SKILL.md:** Always uppercase (entry point)
- **References:** `kebab-case.md` (e.g., `security-patterns.md`)

### Skill Tiers

See `skills/SKILL-TIERS.md` for the complete taxonomy:

| Tier | Description | Examples |
|------|-------------|----------|
| **solo** | Standalone, invoked directly by users | research, plan, vibe |
| **library** | Reference skills loaded JIT by other skills | beads, standards |
| **orchestration** | Multi-skill coordinators | crank, council, swarm |
| **team** | Require human collaboration | implement (guided mode) |
| **background** | Hook-triggered or automatic | inject, forge, extract |
| **meta** | Skills about skills | using-agentops |

## Quality Standards

### Documentation Requirements

Every skill must have:
- SKILL.md with valid YAML frontmatter (`name` and `description` fields)
- Clear purpose statement
- Usage examples
- Output description

### Code Quality

Required:
- No hardcoded credentials or secrets
- Valid YAML frontmatter in SKILL.md
- Working markdown links (references must exist)
- Dependencies must reference existing skills

Recommended:
- Include anti-patterns to avoid
- Provide troubleshooting guidance
- Document known limitations

## Testing Your Skill

### 1. Static Validation

```bash
# Validate your skill structure
./tests/skills/validate-skill.sh skills/your-skill-name

# Run all skill validations
./tests/skills/run-all.sh
```

### 2. Smoke Test

```bash
# Verify the plugin loads correctly with your skill
./tests/smoke-test.sh
```

### 3. Plugin Load Test

```bash
# Test with Claude Code directly
claude --plugin ./
```

### 4. Full Test Suite

```bash
# Run everything
./tests/run-all.sh
```

## Submission Process

### 1. Create Feature Branch

```bash
git checkout -b add-your-skill-name
```

### 2. Commit Changes

```bash
git add skills/your-skill-name/
git commit -m "feat: add your-skill-name skill

## Context
Adding skill for [purpose] to help users [benefit].

## Testing
- Static validation passed
- Smoke test passed
- Plugin loads correctly
"
```

### 3. Push and Create PR

```bash
git push origin add-your-skill-name
```

Then create a Pull Request on GitHub with:

**Title:** `feat: add [skill-name] skill`

**Description:**
```markdown
## Skill Information

- **Name:** your-skill-name
- **Tier:** solo
- **Description:** [Brief description]

## Features

- [Feature 1]
- [Feature 2]

## Testing Checklist

- [ ] `./tests/skills/validate-skill.sh skills/your-skill-name` passes
- [ ] `./tests/smoke-test.sh` passes
- [ ] SKILL.md has valid YAML frontmatter
- [ ] Documentation complete
- [ ] Examples provided

## Related Issues

Closes #X (if applicable)
```

### 4. PR Review Process

We will review:
- Skill structure and SKILL.md completeness
- YAML frontmatter validity
- Documentation quality
- Test validation results
- Security (no secrets, no dangerous shell patterns)

Review timeline:
- Initial review: 2-3 business days
- Feedback provided if changes needed
- Approval once all requirements met

### 5. After Merge

Once merged:
- Your skill is available in the plugin
- Users can install with: `npx skills@latest add boshu2/agentops --all -g`
- You'll be credited as author

## Release Cadence

AgentOps follows a **weekly release train** to avoid notification spam for watchers and stargazers (GitHub sends an email for every published release — there's no way to filter).

### Schedule

| Type | When | Version | Example |
|------|------|---------|---------|
| **Regular release** | Weekly (Fridays) | Minor bump (`vX.Y.0`) | Features + fixes accumulated all week |
| **Security hotfix** | Immediately | Patch bump (`vX.Y.Z`) | Only for security vulnerabilities |
| **Skip week** | When nothing meaningful shipped | — | No release, no tag |

### Rules

- **One published release per week, max.** Accumulate features and fixes in `[Unreleased]` in CHANGELOG.md.
- **Security hotfixes are the only exception.** Ship same day as patch version.
- **No 1-commit releases** for non-security fixes. Batch them.
- **No multiple releases in one day** unless one is a security hotfix.
- **Draft releases don't notify anyone.** Use drafts freely for CI testing.

### For Maintainers

When cutting a release:

1. Ensure CHANGELOG.md `[Unreleased]` section has all changes
2. Write curated release notes to `.agents/releases/YYYY-MM-DD-v<version>-notes.md`
3. Run `/release <version>` — handles changelog, version bumps, tag, and draft GitHub Release
4. Push tag: `git push origin main --tags`
5. CI builds, validates, and publishes automatically

The release pipeline reads curated notes from `.agents/releases/` and shows them as highlights on the GitHub Release page, with the full changelog in a collapsible section.

## Code of Conduct

### Our Standards

**Positive behavior:**
- Be respectful and inclusive
- Provide constructive feedback
- Collaborate openly
- Welcome newcomers
- Share knowledge generously

**Unacceptable behavior:**
- Harassment or discrimination
- Trolling or insulting comments
- Personal or political attacks
- Publishing others' private information
- Other unprofessional conduct

### Enforcement

Violations may result in:
1. Warning from maintainers
2. Temporary ban from contributing
3. Permanent ban from project

Report issues to: fullerbt@users.noreply.github.com

## Getting Help

### Questions?

- **Documentation:** Check README.md and existing skills
- **Examples:** Browse `skills/` for reference implementations
- **Skill Taxonomy:** See `skills/SKILL-TIERS.md`
- **GitHub Discussions:** Ask usage questions and share workflow results
- **GitHub Issues:** Report confirmed bugs or request concrete features
- **Email:** fullerbt@users.noreply.github.com

### Useful Resources

**Project Documentation:**
- [AGENTS.md](AGENTS.md) - Project instructions for AI agents
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture
- [docs/SKILLS.md](docs/SKILLS.md) - Skills reference
- [skills/SKILL-TIERS.md](skills/SKILL-TIERS.md) - Skill taxonomy
- [docs/testing-skills.md](docs/testing-skills.md) - Testing guide

**External:**
- [Claude Code Plugins](https://docs.claude.com/en/docs/claude-code/plugins)

## Maintainer Guidelines

### For Repository Maintainers

**Reviewing PRs:**
1. Check skill structure (SKILL.md with valid frontmatter)
2. Run `./tests/skills/validate-skill.sh skills/<name>`
3. Review documentation quality
4. Check for security issues
5. Provide constructive feedback

**Merging:**
- Require all checks passed
- Require approval from at least 1 maintainer
- Use squash merge for cleaner history
- Update CHANGELOG.md

**Communication:**
- Respond to PRs within 2-3 business days
- Be welcoming and supportive
- Provide clear, actionable feedback
- Thank contributors for their work

## License

By contributing to this project, you agree that your contributions will be licensed under the Apache License 2.0.

---

**Thank you for contributing to AgentOps!**

**Questions?** Open an issue or email fullerbt@users.noreply.github.com
