---
name: standards
description: 'Use when enforcing code style, linting rules, formatting standards, or coding conventions across Python, Go, Rust, TypeScript, Shell, YAML, JSON, and Markdown files. Enforces naming conventions, validates code structure, checks formatting rules. Auto-loaded by /vibe, /implement, /doc, /bug-hunt, /complexity based on detected file types.'
skill_api_version: 1
context:
  window: isolated
  intent:
    mode: none
  sections:
    exclude: [HISTORY, INTEL, TASK]
  intel_scope: none
metadata:
  tier: library
  dependencies: []
  internal: true
---

# Standards Skill

Language-specific coding standards loaded on-demand by other skills.

## Purpose

This is a **library skill** - it doesn't run standalone but provides standards
references that other skills load based on file types being processed.

## Standards Available

| Standard | Reference | Loaded By |
|----------|-----------|-----------|
| Skill Structure | `references/skill-structure.md` | vibe (skill audits), doc (skill creation) |
| Python | `references/python.md` | vibe, implement, complexity |
| Go | `references/go.md` | vibe, implement, complexity |
| Rust | `references/rust.md` | vibe, implement, complexity |
| TypeScript | `references/typescript.md` | vibe, implement |
| Shell | `references/shell.md` | vibe, implement |
| YAML | `references/yaml.md` | vibe |
| JSON | `references/json.md` | vibe |
| Markdown | `references/markdown.md` | vibe, doc |
| SQL Safety | `references/sql-safety-checklist.md` | vibe, pre-mortem (when DB code detected) |
| LLM Trust Boundaries | `references/llm-trust-boundary-checklist.md` | vibe, pre-mortem (when LLM code detected) |
| Race Conditions | `references/race-condition-checklist.md` | vibe, pre-mortem (when concurrent code detected) |
| Codex Skills | `references/codex-skill.md` | vibe (when `skills-codex/` or converter files detected) |
| Test Pyramid | `references/test-pyramid.md` | plan, pre-mortem, implement, crank, validation, post-mortem |

## How It Works

Skills declare `standards` as a dependency:

```yaml
skills:
  - standards
```

Then the consuming skill loads the appropriate reference based on detected file extensions (`.py` → `references/python.md`, `.go` → `references/go.md`, `.rs` → `references/rust.md`, etc.).

## Domain-Specific Checklists

Specialized checklists for high-risk code patterns. Loaded automatically by `/vibe` and `/pre-mortem` when matching code patterns are detected:

| Checklist | Trigger Pattern | Risk Area |
|-----------|----------------|-----------|
| `sql-safety-checklist.md` | SQL queries, ORM calls, migration files, `database/sql`, `sqlalchemy`, `prisma` | Injection, migration safety, N+1, transactions |
| `llm-trust-boundary-checklist.md` | `anthropic`, `openai` imports, prompt templates, `*llm*`/`*prompt*` files | Prompt injection, output validation, cost control |
| `race-condition-checklist.md` | Goroutines, threads, `asyncio`, `sync.Mutex`, shared file I/O | Shared state, file races, database races |
| `codex-skill.md` | Files under `skills-codex/`, `convert.sh`, `skills-codex-overrides/` | Codex API conformance, prohibited primitives, tool mapping |

Skills detect triggers via file content patterns and import statements. Each checklist's "When to Apply" section defines exact detection rules.

## Deep Standards

For comprehensive audits, skills can load extended standards from
`vibe/references/*-standards.md` which contain full compliance catalogs.

| Standard | Size | Use Case |
|----------|------|----------|
| Tier 1 (this skill) | ~5KB each | Normal validation |
| Tier 2 (vibe/references) | ~15-20KB each | Deep audits, `--deep` flag |
| Domain checklists | ~3-5KB each | Triggered by code pattern detection |

## Integration

Skills that use standards:
- `/vibe` - Loads based on changed file types
- `/implement` - Loads for files being modified
- `/doc` - Loads markdown standards
- `/bug-hunt` - Loads for root cause analysis
- `/complexity` - Loads for refactoring recommendations

## Examples

### Vibe Loads Python Standards

**User says:** `/vibe` on a changeset containing `auth.py`
**Result:** Vibe detects `.py` files, loads `standards/references/python.md`, and validates type hints, docstrings, and error handling automatically.

### Implement Loads Go Standards

**User says:** `/implement ag-xyz-123` on an issue targeting `server.go`
**Result:** Implement loads `standards/references/go.md` and generates code conforming to Go error handling, naming, and package structure conventions.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Standards not loaded | File type not detected or standards skill missing | Check file extension matches reference; verify standards in dependencies |
| Wrong standard loaded | File type misidentified (e.g., .sh as .bash) | Manually specify standard; update file type detection logic |
| Deep standards missing | Vibe needs extended catalog, not found | Check `vibe/references/*-standards.md` exists; use `--deep` flag |
| Standard conflicts | Multiple languages in same changeset | Load all relevant standards; prioritize by primary language |

## Reference Documents

- [references/common-standards.md](references/common-standards.md)
- [references/examples-troubleshooting-template.md](references/examples-troubleshooting-template.md)
- [references/standards-index.md](references/standards-index.md)
