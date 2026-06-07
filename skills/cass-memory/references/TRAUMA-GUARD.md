# Trauma Guard — Safety System

## Table of Contents
- [Overview](#overview)
- [How It Works](#how-it-works)
- [Built-in Doom Patterns](#built-in-doom-patterns-20)
- [Pattern Storage](#pattern-storage)
- [Commands](#commands)
- [Pattern Lifecycle](#pattern-lifecycle)

---

## Overview

The "hot stove" principle—learn from past incidents and prevent recurrence.

Trauma Guard blocks dangerous commands before they execute by matching against patterns learned from past mistakes.

---

## How It Works

```
Session History              Trauma Registry              Runtime Guard
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│ rm -rf /* (oops)│ ──────▶ │ Pattern: rm -rf │ ──────▶ │ BLOCKED: This   │
│ "sorry, I made  │  scan   │ Severity: FATAL │  hook   │ command matches │
│  a mistake..."  │         │ Session: abc123 │         │ a trauma pattern│
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

1. **Scan**: `cm trauma scan` identifies dangerous patterns from past sessions
2. **Register**: Patterns are stored in trauma registry
3. **Guard**: Runtime hook blocks matching commands

---

## Built-in Doom Patterns (20+)

| Category | Examples |
|----------|----------|
| **Filesystem** | `rm -rf /`, `rm -rf ~`, recursive deletes |
| **Database** | `DROP DATABASE`, `TRUNCATE`, `DELETE FROM` without WHERE |
| **Git** | `git push --force` to main/master, `git reset --hard` |
| **Infrastructure** | `terraform destroy -auto-approve`, `kubectl delete namespace` |
| **Cloud** | `aws s3 rm --recursive`, destructive CloudFormation |

---

## Pattern Storage

| Scope | Location | Purpose |
|-------|----------|---------|
| **Global** | `~/.cass-memory/traumas.jsonl` | Personal patterns |
| **Project** | `.cass/traumas.jsonl` | Commit to repo for team |

---

## Commands

```bash
# List active patterns
cm trauma list

# Add custom pattern
cm trauma add "DROP TABLE" --description "Mass deletion" --severity critical

# Temporarily bypass (with reason)
cm trauma heal t-abc --reason "Intentional migration"

# Remove pattern
cm trauma remove t-abc

# Scan sessions for trauma patterns
cm trauma scan --days 30

# Import shared patterns
cm trauma import shared-traumas.yaml

# Install hooks
cm guard --install       # Claude Code hook
cm guard --git          # Git pre-commit hook
cm guard --install --git # Both
cm guard --status       # Check installation
```

---

## Pattern Lifecycle

```
┌──────────┐     heal      ┌──────────┐
│  Active  │──────────────▶│  Healed  │
└──────────┘               └──────────┘
     │                          │
     │ remove                   │ expires or
     ▼                          │ re-activate
┌──────────┐                    │
│ Deleted  │◀───────────────────┘
└──────────┘
```

- **Active**: Blocks matching commands
- **Healed**: Temporarily bypassed (with reason and timestamp)
- **Deleted**: Removed (can be re-added)

---

## Pattern Format

```json
{
  "id": "t-8f3a2c",
  "pattern": "rm -rf /",
  "description": "Recursive delete from root",
  "severity": "fatal",
  "source_session": "/path/to/session.jsonl",
  "created_at": "2025-01-15T10:30:00Z",
  "status": "active"
}
```

### Severity Levels

| Level | Effect |
|-------|--------|
| `critical` | Always block, require explicit override |
| `high` | Block with warning |
| `medium` | Warn but allow |
| `low` | Log only |

---

## Healing a Trauma

Sometimes you need to run a dangerous command intentionally:

```bash
# Temporary bypass with reason
cm trauma heal t-abc --reason "Intentional database migration"

# The pattern remains in registry but won't block
# Audit trail preserved

# Re-activate when done
cm trauma activate t-abc
```

---

## Sharing Team Patterns

Export patterns to share with your team:

```bash
# Export your patterns
cm trauma export > team-traumas.yaml

# Team members import
cm trauma import team-traumas.yaml
```

Commit `.cass/traumas.jsonl` to your repo to share project-specific patterns.
