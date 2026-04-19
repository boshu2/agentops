# Coding Standards

## Skills Integration

These standards are also available as a **library skill** in domain-kit:

```
plugins/domain-kit/skills/standards/references/
├── python.md
├── go.md
├── typescript.md
├── shell.md
├── yaml.md
├── json.md
├── rust.md
├── markdown.md
└── openai.md (+ related OpenAI references)
```

Skills like `/bug-hunt` and `/complexity` depend on the standards library for consistent language-specific guidance.

---

## Standards Index

| Language/Format | Document | Gate | Pre-commit | AI-Friendly |
|-----------------|----------|------|------------|-------------|
| **Python** | [python-style-guide.md](./python-style-guide.md) | CC ≤ 10 | ruff | ★★★★★ |
| **Shell/Bash** | [shell-script-standards.md](./shell-script-standards.md) | shellcheck | shellcheck | ★★★★★ |
| **Go** | [golang-style-guide.md](./golang-style-guide.md) | golangci-lint, CC ≤ 10 | - | ★★★★★ |
| **Rust** | — | cargo clippy, CC ≤ 10 | - | ★★★★★ |
| **TypeScript** | [typescript-standards.md](./typescript-standards.md) | tsc --strict | eslint | ★★★★★ |
| **YAML/Helm** | [yaml-helm-standards.md](./yaml-helm-standards.md) | yamllint | yamllint | ★★★★★ |
| **Markdown** | [markdown-style-guide.md](./markdown-style-guide.md) | markdownlint | - | ★★★★★ |
| **JSON/JSONL** | [json-jsonl-standards.md](./json-jsonl-standards.md) | jq/prettier | - | ★★★★★ |
| **Documentation Tags** | [tag-vocabulary.md](./tag-vocabulary.md) | - | - | ★★★★★ |

---

## What Makes These Standards AI-Agent-Friendly

Every standard in this repository is optimized for AI agent execution:

| Feature | Implementation |
|---------|----------------|
| **Tables over prose** | Scannable, parallel parsing |
| **Decision trees** | Executable if/then logic |
| **Common Errors tables** | Symptom → Cause → Fix |
| **Anti-Patterns (named)** | Recognizable error states |
| **AI Agent Guidelines** | ALWAYS/NEVER rules |
| **Explicit thresholds** | Numbers, not "be careful" |
| **Copy-paste examples** | Ready to use, not fragments |

---

## Complexity Requirements

All functions MUST meet complexity thresholds:

| Language | Tool | Threshold | Style Guide |
|----------|------|-----------|-------------|
| Python | radon/xenon | CC ≤ 10 | [Yes](./python-style-guide.md#code-complexity) |
| Go | gocyclo | CC ≤ 10 | [Yes](./golang-style-guide.md#code-complexity) |
| Rust | cargo clippy | Clippy warnings = 0 | — |
| Shell | shellcheck | Pass all | [Yes](./shell-script-standards.md) |
| TypeScript | tsc --strict | No errors | [Yes](./typescript-standards.md) |
| YAML/Helm | yamllint | Pass all | [Yes](./yaml-helm-standards.md) |

---

## Pre-commit Enforcement

Standards are enforced via pre-commit hooks where available.

### Quick Start

```bash
# Install hooks (one-time)
pre-commit install

# Run manually
pre-commit run --all-files

# Or skip hooks (not recommended)
git commit --no-verify
```

### Manual Validation Commands

| Language | Command |
|----------|---------|
| **Python** | `ruff check scripts/ && xenon scripts/ --max-absolute B` |
| **Go** | `golangci-lint run ./... && gocyclo -over 10 ./...` |
| **Rust** | `cargo clippy --all-targets --all-features -- -D warnings && cargo test` |
| **Shell** | `shellcheck scripts/*.sh` |
| **TypeScript** | `tsc --noEmit && eslint . --ext .ts,.tsx` |
| **YAML** | `yamllint .` |
| **Markdown** | `npx markdownlint '**/*.md'` |
| **JSON** | `jq empty config.json` (validates syntax) |

---

## Standard Document Structure

Each standard follows a consistent format:

```markdown
# Standard Name

## Quick Reference
[Table of key rules and values]

## [Topic Sections]
[Detailed guidance with examples]

## Common Errors
[Symptom | Cause | Fix table]

## Anti-Patterns
[Named patterns to avoid]

## AI Agent Guidelines
[ALWAYS/NEVER rules table]

## Summary
[Key takeaways list]
```

---

## Common Configuration Files (Examples)

These are common config files you may choose to add to a project that adopts these standards.
AgentOps does not require them, and this repository does not ship every file listed below.

| File | Purpose |
|------|---------|
| `.pre-commit-config.yaml` | Hook definitions |
| `.yamllint.yml` | YAML linting rules |
| `.markdownlint.yml` | Markdown linting rules |
| `.shellcheckrc` | Shell linting rules |
| `.prettierrc` | JSON/Markdown formatting |
| `pyproject.toml` | Python tool settings (ruff, pytest) |
| `.golangci.yml` | Go linting configuration |
| `rustfmt.toml` | Rust formatting rules |
| `clippy.toml` | Rust linting configuration |
| `tsconfig.json` | TypeScript compiler settings |
| `eslint.config.js` | TypeScript/JS linting |

---

## Contributing

To update a standard:

1. Edit the standard directly in `docs/standards/`
2. Run validation to ensure format compliance

### Adding a New Standard

1. Follow the document structure template above
2. Include: Quick Reference, Common Errors, Anti-Patterns, AI Guidelines
3. Add to this README's index table
4. Update related links in other standards

---

**Created:** 2024-12-30
**Last Updated:** 2026-02-09
**Related:** [SKILL-TIERS.md](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md)
