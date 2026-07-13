# Standards Index

JIT loading map for validation agents. Load only what you need based on file types.

## Extension to Standard Map

| Extension | Standard File | Size |
|-----------|---------------|------|
| `.py` | `skills/standards/references/python.md` | Python |
| `.go` | `skills/standards/references/go.md` | Go |
| `.rs` | `skills/standards/references/rust.md` | Rust |
| `.ts`, `.tsx` | `skills/standards/references/typescript.md` | TypeScript |
| `.sh`, `.bash` | `skills/standards/references/shell.md` | Shell |
| `.yaml`, `.yml` | `skills/standards/references/yaml.md` | YAML |
| `.json` | `skills/standards/references/json.md` | JSON/JSONL |
| `.md` | `skills/standards/references/markdown.md` | Markdown |

## Universal Standards (Always Load)

| Standard | File | Purpose |
|----------|------|---------|
| **LLM Trust Boundary** | `skills/standards/references/llm-trust-boundary-checklist.md` | Evidence, authority, and injection boundaries |
| **Common Standards** | `skills/standards/references/common-standards.md` | Cross-language patterns: error handling, testing, security, docs, organization |
| **Behavioral Discipline** | `skills/standards/references/behavioral-discipline.md` | Assumptions, simplicity bias, blast-radius control, verification discipline |
| **Skill Structure** | `skills/standards/references/skill-structure.md` | Anthropic-compliant skill structure, frontmatter, quality checklist |

Load the trust-boundary checklist first, then common standards and behavioral
discipline for universal patterns.

## Pattern Files (Load When Relevant)

| Pattern Type | File | When to Load |
|--------------|------|--------------|
| Go patterns | `skills/standards/references/go.md` | Go architecture review |
| General patterns | `skills/standards/references/common-standards.md` | Design review |
| Codex skill standard | `skills/standards/references/codex-skill.md` | Codex skill files, converter output, `skills-codex/` |

## JIT Loading Pattern for Agents

```markdown
## Step 1: Detect File Types

Scan the target files to identify languages:
- Use Glob to find files
- Note extensions present

## Step 2: Load Relevant Standards

For each language detected, use Read tool:

Tool: Read
Parameters:
  file_path: "skills/standards/references/<language>.md"

Only load standards for languages actually present in the review.

## Step 3: Apply Standards

Reference the loaded standards when validating code.
Cite specific sections: "Per python.md section 3.2..."
```

## Example: Mixed Python/Go Review

```
Files detected: src/main.py, pkg/handler.go, scripts/deploy.sh

Load (3 standards only):
1. Read("skills/standards/references/python.md")
2. Read("skills/standards/references/go.md")
3. Read("skills/standards/references/shell.md")

Skip: typescript, yaml, json, markdown (not present)
```

## Context Budget

| Agent Model | Context Budget | Max Standards |
|-------------|----------------|---------------|
| haiku | ~100k | 3-4 standards |
| opus | ~200k | All if needed |

Keep agents lean. Load only what's needed.

## JIT Loading Order

1. **llm-trust-boundary-checklist.md** (universal — evidence and authority boundaries)
2. **common-standards.md** (universal — cross-language error handling, testing, security, docs, organization)
3. **behavioral-discipline.md** (universal — assumptions, simplicity, scope control, verification)
4. **Language standards** (per detected extensions)
5. **Pattern files** (if architecture/discovery review)
