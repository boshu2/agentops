# Reviewer Config Example

Drop this file at `.agents/reviewer-config.md` in your project to customize which council judges run.

## Example

```markdown
---
reviewers:
  - security-sentinel
  - architecture-strategist
  - code-simplicity-reviewer
plan_reviewers:
  - architecture-strategist
  - scope
skip_reviewers:
  - performance-oracle
---

## Project Review Context

This is a Go CLI project focused on developer tooling. Performance is secondary to correctness and UX. Security reviews should focus on input validation and file system operations.
```

## Schema

| Field | Type | Description |
|-------|------|-------------|
| `reviewers` | list of strings | Judge perspectives for `/vibe` and `/council validate` |
| `plan_reviewers` | list of strings | Judge perspectives for `/pre-mortem` and `/council validate` on plans |
| `skip_reviewers` | list of strings | Perspectives to exclude even if preset includes them |

Markdown body below the frontmatter is passed as additional context to all judges.
