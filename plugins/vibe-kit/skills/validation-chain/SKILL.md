---
name: validation-chain
description: >
  This skill should be used when the user asks to "validate changes",
  "run validation gate", "check before merge", "quality check",
  or needs comprehensive validation across security, quality, and architecture.
version: 1.0.0
context: fork
allowed-tools: Read, Grep, Glob, Task, TodoWrite
skills:
  - beads
  - vibe
  - standards
triggers:
  - "validate changes"
  - "run validation"
  - "quality gate"
  - "pre-merge check"
---

# Validation Chain Skill

Orchestrate specialist agents for comprehensive change validation.

## Overview

Route changes to appropriate specialists based on file types and content patterns:
- **Security Expert**: Auth, API, secrets, user input
- **Code Quality Expert**: All code changes (default)
- **Architecture Expert**: Schema, config, service boundaries
- **UX Expert**: UI components, user-facing changes

## Execution Flow

```
1. Identify Changed Files  -> git diff --name-only or explicit list
2. Classify by Domain      -> Pattern matching to specialists
3. Dispatch Parallel       -> Task() to each relevant specialist
4. Aggregate Findings      -> Triage matrix consolidation
5. Report + Create Issues  -> Blockers become beads issues
```

---

## Phase 1: File Classification

### Pattern → Specialist Mapping

| Pattern | Specialist | Rationale |
|---------|------------|-----------|
| `**/auth/**`, `**/login/**` | security-expert | Auth code |
| `**/*.yaml`, `**/*.yml` | architecture-expert | Config/schema |
| `**/components/**`, `**/ui/**` | ux-expert | UI changes |
| `**/*.py`, `**/*.go`, `**/*.ts` | code-quality-expert | All code |
| `**/secrets/**`, `**/*cred*` | security-expert | Sensitive |
| `**/api/**`, `**/routes/**` | security-expert + code-quality-expert | API |

### Classification Logic

```bash
# Get changed files
git diff --name-only HEAD~1 HEAD > /tmp/changed_files.txt

# Or from staged
git diff --cached --name-only > /tmp/changed_files.txt
```

---

## Phase 2: Parallel Dispatch

Invoke specialists in parallel via Task():

```markdown
# For security-sensitive files
Task(
    subagent_type="security-expert",
    model="sonnet",
    prompt="Review these files for security vulnerabilities: $SECURITY_FILES"
)

# For all code files
Task(
    subagent_type="code-quality-expert",
    model="sonnet",
    prompt="Review code quality and complexity: $CODE_FILES"
)

# For config/schema files
Task(
    subagent_type="architecture-expert",
    model="sonnet",
    prompt="Validate architecture patterns: $CONFIG_FILES"
)

# For UI components
Task(
    subagent_type="ux-expert",
    model="sonnet",
    prompt="Check accessibility and UX patterns: $UI_FILES"
)
```

---

## Phase 3: Finding Aggregation

### Triage Matrix

| Severity | Criteria | Action |
|----------|----------|--------|
| **[Blocker]** | Security vulns, CC > 10, broken builds | Must fix before merge |
| **[High-Priority]** | Missing tests, performance issues | Fix within sprint |
| **[Medium-Priority]** | Code smells, minor UX issues | Follow-up issue |
| **[Nitpick]** | Style, naming preferences | Author discretion |

### Aggregated Report Format

```markdown
## Validation Chain Report

**Files Validated:** [count]
**Specialists Invoked:** [list]
**Overall Status:** [PASS | PASS WITH NOTES | BLOCKED]

### Blockers (Must Fix)
- [Specialist]: [Finding] (`file:line`)

### High-Priority (Should Fix)
- [Specialist]: [Finding] (`file:line`)

### Issues Created
- [beads-id]: [title]
```

---

## Phase 4: Issue Creation

For CRITICAL and HIGH findings, create beads issues:

```bash
# For blockers
bd create --title "[Blocker] $FINDING_TITLE" --type bug --priority 0

# For high-priority
bd create --title "[Quality] $FINDING_TITLE" --type task --priority 1
```

---

## Usage Examples

```bash
# Validate staged changes
/validation-chain staged

# Validate specific files
/validation-chain services/gateway/*.py

# Validate recent commits
/validation-chain HEAD~3..HEAD

# Full codebase validation (sampled)
/validation-chain all --sample 10%
```

---

## Integration with /crank

When `--validate` flag is used:

```markdown
/crank <epic> --validate
```

The validation chain runs after each wave completes, before proceeding to next wave.

---

## References

- **Security Expert**: `~/.claude/agents/security-expert.md`
- **Code Quality Expert**: `~/.claude/agents/code-quality-expert.md`
- **Architecture Expert**: `~/.claude/agents/architecture-expert.md`
- **UX Expert**: `~/.claude/agents/ux-expert.md`
