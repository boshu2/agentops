---
name: codebase-pattern-extraction
description: >-
  Mine patterns that recur across multiple projects and generalize into reusable artifacts.
  Use when "I've seen this before", DRY across repos, or building shared libraries.
practices:
- pragmatic-programmer
skill_api_version: 1
hexagonal_role: supporting
consumes: []
produces:
- codebase-pattern-extraction
context:
  window: fork
metadata:
  tier: knowledge
  stability: stable
  dependencies: []
user-invocable: true
---

<!-- TOC: Problem | THE EXACT PROMPT | Using CASS | The Pipeline | Quick Start | Pattern Types | Diff/Align | Abstraction | Package Types | Validation | Discovery | Anti-Patterns | References -->

# Codebase Pattern Extraction

> **Core Insight:** If you solved it twice, you'll solve it again. Extract once, reuse forever.

## The Problem

You notice similar code across projects. Copy-paste spreads bugs. You need a systematic way to extract patterns into reusable artifacts (libraries, skills, templates).

---

## THE EXACT PROMPT

### Research Prompt (Multi-Project Analysis)

```
Research best practices across these mature projects to inform improvements:

1. Analyze /data/projects/project_a, project_b, project_c, project_d
2. Identify common patterns in:
   - CLI/TUI architecture (interactive vs robot modes)
   - Distribution & installation (multi-platform, checksums, idempotency)
   - Performance & search (caching, parallelism)
   - Agent ergonomics (structured JSON output, deterministic flags)
3. Extract the invariant patterns that work across all projects
4. Synthesize into reusable best practices document

Output: Research report with concrete patterns and code examples.
```

### Extraction Prompt

```
Extract a reusable pattern from these projects:

1. Collect: Find 3+ instances of the pattern across different projects
2. Diff: What's common? What varies per-project?
3. Abstract: Pull out the invariant core
4. Parameterize: Make the varying parts configurable
5. Package: Create library/skill/template with tests

Output: Reusable artifact + usage examples from original projects.
```

---

## Using CASS for Pattern Discovery

CASS is powerful for finding patterns across your session history:

```bash
# Find workflow patterns across ALL workspaces
cass search "retry backoff exponential" --robot --limit 30

# Find similar patterns by technique name
cass search "installation script multi-platform" --robot --limit 20

# Find patterns in specific project clusters
cass search "CLI robot mode json" --robot --limit 20

# Aggregate by workspace to see which projects use similar patterns
cass search "pattern_name" --aggregate workspace --limit 50
```

**Why CASS helps:**
- Searches across ALL your past sessions, not just current project
- Reveals patterns you've used successfully before
- Shows how patterns evolved across different implementations

---

## The Pipeline

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  Project A  │   │  Project B  │   │  Project C  │
│  pattern    │   │  pattern    │   │  pattern    │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └────────────┬────┴────────────────┘
                    ▼
           ┌───────────────┐
           │   DIFF/ALIGN  │
           │ Common vs Vary │
           └───────┬───────┘
                   ▼
           ┌───────────────┐
           │   ABSTRACT    │
           │ Invariant core │
           └───────┬───────┘
                   ▼
           ┌───────────────┐
           │ PARAMETERIZE  │
           │ Make flexible  │
           └───────┬───────┘
                   ▼
           ┌───────────────┐
           │   PACKAGE     │
           │ Lib/Skill/Tmpl │
           └───────────────┘
```

---

## Quick Start

```bash
# Phase 1: Collect instances across projects
rg "pattern_signature" /data/projects/*/src/ --files-with-matches
# Look for similar function names, struct shapes, imports

# Phase 2: Use CASS to find past implementations
cass search "pattern_keyword" --robot --limit 30 | jq '.hits[:10]'

# Phase 3: Extract and compare
# Copy relevant code snippets to a scratch file
# Align them side-by-side

# Phase 4: Identify
# Common: The invariant logic
# Varying: Project-specific config, names, types

# Phase 5: Create reusable artifact
# Library: shared crate/package
# Skill: shared .claude skill
# Template: cookiecutter/copier template
```

---

## Pattern Types

| Pattern Type | Extract Into | Example |
|--------------|--------------|---------|
| Code logic | Library/crate | Error handling, retry logic |
| Workflow | Skill | Code review, deployment, porting |
| Project structure | Template | New project scaffolding |
| Config | Shared config | ESLint, tsconfig, Cargo defaults |
| CLI flags | Shared CLI module | Common --verbose, --json, --robot flags |
| Installation | Install script | Multi-platform installers with checksums |

---

## Common Extractable Patterns (Discovered via CASS)

### 1. CLI Robot Mode Pattern

```
Pattern: CLI with dual interactive/robot modes
Found in: cass, bv, ubs, br, xf, dcg

Invariant:
- --json flag for structured output
- --robot flag for deterministic behavior
- Human-readable default, machine-parseable on request

Variance:
- Specific output schemas
- Which commands support robot mode
```

### 2. Multi-Platform Installation Pattern

```
Pattern: TL;DR installer with fallbacks
Found in: mcp_agent_mail, ubs, br, bv

Invariant:
- Auto-detect platform (Linux/macOS/Windows)
- SHA256 checksum verification
- Idempotent (safe to re-run)
- Fallback to source build if binary unavailable

Variance:
- Binary names, install paths
- Additional dependencies
```

### 3. Dual Persistence Pattern

```
Pattern: SQLite + Git-tracked JSONL
Found in: br (beads_rust), cass, meta_skill

Invariant:
- SQLite for fast queries
- JSONL for Git-tracked history
- Sync between the two

Variance:
- Schema specifics
- Sync triggers
```

---

## Diff/Align Technique

```
PROJECT A                    PROJECT B                    PROJECT C
─────────────────────────────────────────────────────────────────────
fn retry<T>(                fn with_retry<T>(           fn retry_call<T>(
  f: impl Fn() -> T,          op: impl Fn() -> T,         func: impl Fn() -> T,
  max: usize,                 attempts: u32,              max_attempts: usize,
  delay: Duration             wait: Duration              backoff: Duration
) -> Result<T, Error>       ) -> Result<T, E>           ) -> Result<T, Error>
─────────────────────────────────────────────────────────────────────
COMMON: retry logic with max attempts and delay
VARIES: naming, integer types, error types
```

---

## Abstraction Checklist

- [ ] **3+ instances:** Don't abstract from 1-2 examples
- [ ] **Clear invariant:** What MUST be the same?
- [ ] **Clear variance:** What SHOULD be configurable?
- [ ] **Sensible defaults:** Don't require config for common case
- [ ] **Escape hatch:** Allow override for edge cases

---

## Package Types

### Library (Code Reuse)

```rust
// Extracted to shared crate
pub fn retry<T, E>(
    f: impl Fn() -> Result<T, E>,
    config: RetryConfig,
) -> Result<T, E> {
    // Common logic here
}

pub struct RetryConfig {
    pub max_attempts: usize,      // Configurable
    pub initial_delay: Duration,   // Configurable
    pub backoff_factor: f64,       // Configurable with default
}

impl Default for RetryConfig {
    fn default() -> Self {
        Self { max_attempts: 3, initial_delay: Duration::from_millis(100), backoff_factor: 2.0 }
    }
}
```

### Skill (Workflow Reuse)

```yaml
---
name: pattern-name
description: >-
  [What it does]. Use when [trigger 1], [trigger 2], or [context].
---

# Pattern Name

## THE EXACT PROMPT
[Common workflow extracted from multiple projects]

## Customization Points
- [What varies per project]
```

### Template (Structure Reuse)

```
template/
├── {{project_name}}/
│   ├── src/
│   │   └── main.rs.j2
│   ├── Cargo.toml.j2
│   └── README.md.j2
└── copier.yaml           # Variables and defaults
```

---

## Validation

Before publishing extracted pattern:

```bash
# 1. Does it work for all original projects?
# Apply extracted pattern back to each source project

# 2. Is it actually simpler?
# LOC(extracted) < sum(LOC(instances)) / count(instances)

# 3. Does it have tests?
# Test the invariant core + edge cases

# 4. Is it documented?
# README + usage examples from original projects
```

---

## Discovery Techniques

```bash
# Find similar function signatures across projects
rg "fn \w+.*impl Fn" /data/projects/*/src/

# Find similar struct shapes
rg "struct \w+ \{" /data/projects/*/src/ -A 5

# Find similar imports (indicates shared patterns)
rg "^use " /data/projects/*/src/*.rs | sort | uniq -c | sort -rn | head -20

# Find similar error handling
rg "\.map_err\(|\.context\(" /data/projects/*/src/

# Use CASS to find workflow patterns
cass search "pattern_keyword" --robot --limit 20

# Find installation patterns
rg "curl.*sh|wget.*sh|brew install" /data/projects/*/scripts/
rg "sha256|checksum|verify" /data/projects/*/scripts/

# Find CLI patterns
rg "#\[arg\(long.*json|--json|--robot" /data/projects/*/src/
```

---

## Output Template

```markdown
## Extracted Pattern: [Name]

**Source Projects:**
- project_a/src/foo.rs:42
- project_b/src/bar.rs:100
- project_c/lib/baz.rs:15

**CASS Sessions:**
- cass search "[keyword]" found N instances

**Invariant Core:**
[The common logic that doesn't change]

**Variance Points:**
- [What differs] → [How it's parameterized]

**Packaged As:**
- [ ] Library: /data/projects/shared_utils/retry.rs
- [ ] Skill: /cs/retry-pattern/SKILL.md
- [ ] Template: /templates/retry/

**Usage Example:**
[How to use the extracted artifact]

**Tests:**
- [ ] Unit tests for core logic
- [ ] Applied back to all source projects
```

---

## Anti-Patterns

| Don't | Do |
|-------|-----|
| Abstract from 1 instance | Wait for 3+ instances |
| Over-parameterize | Start minimal, add config when needed |
| Lose context | Document WHY the pattern exists |
| Skip tests | Test the extracted artifact |
| Force uniformity | Allow escape hatches for edge cases |
| Ignore CASS | Mine your session history for patterns |
| Skip validation | Apply back to source projects |

---

## Checklist

- [ ] **Discover:** Use CASS + rg to find 3+ instances
- [ ] **Collect:** Gather concrete examples from projects
- [ ] **Diff:** Align code, identify common vs varying
- [ ] **Abstract:** Extract invariant core
- [ ] **Parameterize:** Make variance configurable
- [ ] **Package:** Create lib/skill/template
- [ ] **Test:** Verify works for all source projects
- [ ] **Document:** README + usage examples

---

## References

| Need | File |
|------|------|
| Common extractable patterns | [PATTERNS.md](references/PATTERNS.md) |
| Packaging strategies | [PACKAGING.md](references/PACKAGING.md) |
| Example extractions | [EXAMPLES.md](references/EXAMPLES.md) |
