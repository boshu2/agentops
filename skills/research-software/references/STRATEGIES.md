# Tool-Specific Research Strategies

Deep-dive strategies for different tool categories.

---

## CLI Tools (wrangler, cargo, bun, etc.)

### Where to Look

```
src/cli/ or src/cli.rs or bin/
├── Command definitions
├── Argument parsing (clap, yargs, etc.)
├── Hidden/experimental flags
└── Default values (often different from docs)
```

### Key Searches

```bash
# Rust CLI
rg "hidden\s*=\s*true" /tmp/[repo]-research --type rust
rg "#\[arg\(" /tmp/[repo]-research --type rust

# TypeScript CLI
rg "hidden:|experimental:" /tmp/[repo]-research --type ts
rg "process\.env\." /tmp/[repo]-research --type ts

# Go CLI
rg "Hidden:\s*true" /tmp/[repo]-research --type go
rg "os\.Getenv" /tmp/[repo]-research --type go
```

### Output Focus

- Commands table with all subcommands
- Flags table (including hidden)
- Environment variables
- Config file schema
- Common patterns

---

## Libraries/Frameworks (React, Next.js, etc.)

### Where to Look

```
packages/[core]/src/
├── Exported APIs (index.ts, exports.ts)
├── Internal APIs (not exported)
├── Deprecation warnings
└── Experimental/canary exports
```

### Key Searches

```bash
# Find exports
rg "^export " /tmp/[repo]-research/packages/*/src/index.ts

# Find deprecations
rg "deprecated|@deprecated" /tmp/[repo]-research

# Find experimental
rg "experimental|unstable|canary" /tmp/[repo]-research
```

### Output Focus

- API reference table
- New APIs (latest release)
- Deprecated APIs (with migration)
- Config options
- Patterns from examples/

---

## Runtimes (Bun, Deno, Node)

### Where to Look

```
src/
├── Built-in modules
├── Runtime flags
├── Environment variables
├── Compatibility layers
└── Performance options
```

### Key Searches

```bash
# Runtime flags
rg "flag|--" /tmp/[repo]-research/src/cli

# Built-in modules
rg "Bun\.|Deno\.|node:" /tmp/[repo]-research

# Env vars
rg "process\.env|Deno\.env|Bun\.env" /tmp/[repo]-research
```

### Output Focus

- CLI flags table
- Built-in APIs
- Node.js compatibility status
- Performance tuning options
- Environment variables

---

## Databases/Services (D1, R2, Postgres)

### Where to Look

```
src/
├── Query syntax
├── Connection options
├── Limits and quotas
├── Error codes
└── Migration tools
```

### Key Searches

```bash
# Limits
rg "limit|max|quota" /tmp/[repo]-research

# Error codes
rg "error|Error" /tmp/[repo]-research --type ts -A 2

# Config
rg "config|options|settings" /tmp/[repo]-research
```

### Output Focus

- Query syntax examples
- Config options table
- Limits/quotas table
- Error codes and fixes
- Migration patterns

---

## Monorepo Navigation

Many tools live in monorepos. Quick navigation:

```bash
# Find the main package
ls /tmp/[repo]-research/packages/

# Find entry points
rg "\"main\":|\"bin\":" /tmp/[repo]-research/packages/*/package.json

# Find CLI entry
rg "#!/" /tmp/[repo]-research --type ts | head -5
```

---

## Version Detection

```bash
# From package.json
jq '.version' /tmp/[repo]-research/package.json

# From Cargo.toml
grep '^version' /tmp/[repo]-research/Cargo.toml

# From git tag
git -C /tmp/[repo]-research describe --tags --abbrev=0

# Latest release via GitHub API
gh release view -R [org]/[repo] --json tagName
```

---

## Changelog Mining

```bash
# Find changelog
ls /tmp/[repo]-research/CHANGELOG* /tmp/[repo]-research/HISTORY* 2>/dev/null

# Recent entries
head -100 /tmp/[repo]-research/CHANGELOG.md

# Search for breaking changes
rg -i "breaking|removed|deprecated" /tmp/[repo]-research/CHANGELOG.md
```

---

## Test Mining

Tests often show real usage patterns:

```bash
# Find test files
fd "test|spec" /tmp/[repo]-research --type f

# Find integration tests
fd "integration|e2e" /tmp/[repo]-research --type d

# Search tests for patterns
rg "it\(|test\(|describe\(" /tmp/[repo]-research --type ts -A 5
```
