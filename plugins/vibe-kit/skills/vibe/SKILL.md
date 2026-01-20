---
name: vibe
description: >
  Talos-class comprehensive code validation. Use for "validate code",
  "run vibe", "check quality", "security review", "architecture review",
  "accessibility audit", "complexity check", or any validation need.
  One skill to validate them all.
allowed-tools: "Read,Bash,Glob,Grep,Write,TodoWrite,Task"
version: 3.0.0
author: "AI Platform Team"
license: "MIT"
context: fork
context-budget:
  skill-md: 10KB
  references-total: 12KB
  typical-session: 15KB
skills:
  - beads
  - standards
---

# Vibe - Talos Comprehensive Validation

**One skill to validate them all.**

Vibe is a Talos-class validator that combines fast static analysis with deep
semantic verification across all quality dimensions: code quality, security,
architecture, accessibility, complexity, and more.

## Philosophy

> **Mono over Micro**: Instead of chaining small skills, Vibe provides comprehensive
> validation in one invocation. Trade-off: larger context, but simpler mental model
> and guaranteed coverage.

## Quick Start

```bash
/vibe recent              # Full validation of recent changes
/vibe services/           # Validate a directory
/vibe --fast recent       # Prescan only (no LLM, CI-friendly)
/vibe --security recent   # Security-focused deep dive
/vibe --all-aspects all   # Nuclear option: everything on everything
```

---

## Context Inference

When `/vibe` is invoked without a target, check the preceding conversation for context:

### Priority Order

1. **Explicit target** - If user provides a path/target, use it
2. **Recent code changes in conversation** - If code was just written or edited, validate those files
3. **Staged git changes** - If `git diff --cached` shows staged files, validate those
4. **Unstaged changes** - If `git diff` shows modified files, validate those
5. **Default to `recent`** - Fall back to recent git changes

### Detection Logic

```markdown
## On Invocation Without Target

1. Check if files were edited in this conversation:
   - Look for recent Edit/Write tool calls
   - Extract file paths from tool results
   - If found: validate those specific files

2. Check git state:
   ```bash
   git diff --cached --name-only  # Staged changes
   git diff --name-only           # Unstaged changes
   ```

3. If nothing found, use `recent` (HEAD~1..HEAD)
```

### Example

```
User: [writes some code to services/auth/handler.py]
User: /vibe

→ Vibe infers target from conversation: services/auth/handler.py
→ Validates that specific file instead of requiring explicit path
```

---

## Aspects

Vibe validates across 8 aspects. By default, all aspects run.

| Aspect | Prefix | Focus | Agent |
|--------|--------|-------|-------|
| **Quality** | QUAL-xxx | Code smells, patterns, maintainability | code-quality-expert |
| **Security** | SEC-xxx | OWASP, injection, auth, crypto | security-expert |
| **Architecture** | ARCH-xxx | Boundaries, coupling, scalability | architecture-expert |
| **Accessibility** | A11Y-xxx | WCAG, keyboard, screen reader | ux-expert |
| **Complexity** | CMPLX-xxx | Cyclomatic, cognitive, function size | code-quality-expert |
| **Semantic** | SEM-xxx | Docstrings, names, claims vs reality | code-quality-expert |
| **Performance** | PERF-xxx | N+1, unbounded loops, resource leaks | architecture-expert |
| **Slop** | SLOP-xxx | AI artifacts, hallucinations, cargo cult | code-quality-expert |

### Aspect Selection

```bash
/vibe recent                          # All aspects (default)
/vibe recent --only security          # Security only
/vibe recent --only security,arch     # Security + Architecture
/vibe recent --exclude slop,a11y      # Skip slop and accessibility
/vibe recent --fast                   # Prescan only (no LLM aspects)
```

---

## Modes

| Mode | Flag | LLM | Speed | Use Case |
|------|------|-----|-------|----------|
| **Full** | (default) | Yes | Slow | PR review, pre-merge |
| **Fast** | `--fast` | No | Fast | CI, quick check |
| **Deep** | `--deep` | Yes | Slowest | Audit, new codebase |
| **Security** | `--security` | Yes | Medium | Security-focused |
| **Arch** | `--arch` | Yes | Medium | Architecture review |

---

## Execution Flow

### Phase 1: Prescan (Static, No LLM)

Fast pattern detection using static analysis tools.

> **Static vs Semantic:** Phase 1 patterns (P1-P10) are detected via static tools
> (git, grep, shellcheck, AST parsing). They run fast but may have false positives.
> Phase 2 patterns (QUAL/SEC/ARCH/A11Y/SLOP) use LLM semantic analysis for deeper
> understanding but require more context.

```bash
./scripts/prescan.sh "$TARGET"
```

**Patterns Detected (Static):**

| ID | Pattern | Severity | Tool |
|----|---------|----------|------|
| P1 | Phantom modifications | CRITICAL | git diff |
| P2 | Hardcoded secrets | CRITICAL | gitleaks/grep |
| P3 | SQL injection patterns | CRITICAL | regex |
| P4 | TODO/FIXME/commented code | HIGH | grep |
| P5 | Cyclomatic complexity >15 | HIGH | radon/gocyclo |
| P6 | Functions >50 lines | HIGH | wc/ast |
| P7 | Bare except/empty catch | HIGH | ast/shellcheck |
| P8 | Unused imports/functions | MEDIUM | ast |
| P9 | Docstring mismatches | MEDIUM | ast |
| P10 | Missing error handling | MEDIUM | ast |

**Exit Codes:** 0=clean, 2=CRITICAL, 3=HIGH

### Phase 2: Semantic Analysis (LLM-Powered)

Deep analysis requiring semantic understanding.

**Quality (QUAL-xxx):**
- QUAL-001: Dead code paths
- QUAL-002: Inconsistent naming
- QUAL-003: Magic numbers/strings
- QUAL-004: Missing tests for complexity
- QUAL-005: Copy-paste with slight changes

**Security (SEC-xxx):**
- SEC-001: Injection vulnerabilities (SQL, command, XSS)
- SEC-002: Authentication bypass potential
- SEC-003: Authorization missing/weak
- SEC-004: Cryptographic weaknesses
- SEC-005: Sensitive data exposure
- SEC-006: Security theater (looks secure, isn't)

**Architecture (ARCH-xxx):**
- ARCH-001: Layer boundary violations
- ARCH-002: Circular dependencies
- ARCH-003: God classes/functions
- ARCH-004: Missing abstraction
- ARCH-005: Inappropriate coupling
- ARCH-006: Scalability concerns

**Accessibility (A11Y-xxx):**
- A11Y-001: Missing ARIA labels
- A11Y-002: Keyboard navigation broken
- A11Y-003: Color contrast insufficient
- A11Y-004: Missing alt text
- A11Y-005: Focus management issues

**Complexity (CMPLX-xxx):**
- CMPLX-001: Cyclomatic complexity >10
- CMPLX-002: Cognitive complexity high
- CMPLX-003: Nesting depth >4
- CMPLX-004: Parameter count >5
- CMPLX-005: File too long (>500 lines)

**Semantic (SEM-xxx):**
- SEM-001: Docstring lies (claims vs implementation)
- SEM-002: Misleading function names
- SEM-003: Misleading variable names
- SEM-004: Comment rot (outdated comments)
- SEM-005: API contract violations

**Performance (PERF-xxx):**
- PERF-001: N+1 query patterns
- PERF-002: Unbounded loops/recursion
- PERF-003: Missing pagination
- PERF-004: Resource leaks (unclosed handles)
- PERF-005: Blocking in async context

**Slop (SLOP-xxx):**
- SLOP-001: Hallucinated imports/APIs
- SLOP-002: Cargo cult patterns
- SLOP-003: Excessive boilerplate
- SLOP-004: AI conversation artifacts
- SLOP-005: Over-engineering for simple tasks

### Phase 3: Expert Routing

For CRITICAL/HIGH findings, optionally invoke expert agents via explicit request:

```markdown
# Security findings → security-expert
"Use the security-expert agent to deep dive on SEC-xxx findings..."

# Architecture findings → architecture-expert
"Use the architecture-expert agent to validate ARCH-xxx concerns..."

# Accessibility findings → ux-expert
"Use the ux-expert agent to audit A11Y-xxx issues..."
```

> **Note:** Custom agents are invoked via explicit request, not Task().
> Task() only supports built-in subagent_types (Explore, Plan, etc.).

Enable with `--expert-routing` flag.

### Phase 4: Report Generation

Output to multiple formats:

| Format | Path | Use |
|--------|------|-----|
| Markdown | `.agents/assessments/{date}-vibe-{target}.md` | Human review |
| JSON | `reports/vibe-report.json` | Tooling/CI |
| JUnit XML | `reports/vibe-junit.xml` | CI integration |
| SARIF | `reports/vibe.sarif` | Security tools |

### Phase 5: Issue Creation

When `--create-issues` is set, create beads issues for findings:

```bash
/vibe recent --create-issues   # Create issues for CRITICAL/HIGH
/vibe recent --create-issues --threshold medium  # Include MEDIUM
```

---

## Severity Matrix

| Level | Definition | Action | Exit Code |
|-------|------------|--------|-----------|
| **CRITICAL** | Security vuln, data loss, broken build | Block merge | 2 |
| **HIGH** | Significant quality/security gap | Fix before merge | 3 |
| **MEDIUM** | Technical debt, minor issues | Follow-up issue | 0 |
| **LOW** | Nitpicks, style preferences | Optional | 0 |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VIBE_CREATE_ISSUES` | `false` | Auto-create beads issues |
| `VIBE_FAIL_THRESHOLD` | `critical` | Exit non-zero threshold |
| `VIBE_EXPERT_ROUTING` | `false` | Spawn expert agents for deep dive |
| `VIBE_OUTPUT_FORMAT` | `markdown` | Report format (markdown/json/junit/sarif) |
| `VIBE_SKIP_PRESCAN` | `false` | Skip static analysis phase |
| `VIBE_SKIP_SEMANTIC` | `false` | Skip LLM analysis phase |

---

## Examples

```bash
# Standard full validation
/vibe recent

# CI pipeline (fast, fail on critical)
/vibe --fast recent

# Security audit
/vibe --security services/auth/

# Architecture review before major change
/vibe --arch services/ --expert-routing

# Accessibility audit for frontend
/vibe --only a11y frontend/src/components/

# New codebase deep audit
/vibe --deep all --create-issues

# Pre-commit hook style
VIBE_FAIL_THRESHOLD=high /vibe recent

# Specific aspects
/vibe recent --only security,complexity,semantic

# Exclude noisy aspects
/vibe recent --exclude slop
```

---

## Plugin Validation Mode

Validate Claude Code plugins (commands, skills, agents):

```bash
/vibe --plugin ./my-skill
```

**Plugin Checks:**
- Description matches implementation
- Triggers are accurate (skills)
- Arguments are used (commands)
- Tools declared are used
- No painted doors (documented but missing features)

---

## Integration with Workflow

### Pre-commit
```bash
# .git/hooks/pre-commit
/vibe --fast staged || exit 1
```

### CI Pipeline
```yaml
- name: Vibe Validation
  run: |
    VIBE_FAIL_THRESHOLD=high /vibe recent
```

### PR Review
```bash
# Full review with issue creation
/vibe recent --create-issues --expert-routing
```

### Crank Integration
```bash
# In /crank workflow, vibe runs automatically after implementation
/implement <issue> && /vibe recent --create-issues
```

---

## Output Format

```markdown
## Vibe Validation Report

**Target:** services/gateway/
**Date:** 2026-01-18
**Mode:** Full (all aspects)

### Summary
| Aspect | Critical | High | Medium | Low |
|--------|----------|------|--------|-----|
| Security | 1 | 2 | 0 | 0 |
| Quality | 0 | 3 | 5 | 2 |
| Architecture | 0 | 1 | 2 | 0 |
| Complexity | 0 | 2 | 3 | 0 |
| ... | ... | ... | ... | ... |

### Critical Findings (Block Merge)
- **SEC-001** `auth/handler.go:42` - SQL injection in user lookup
  - Fix: Use parameterized query

### High Priority (Fix Before Merge)
- **CMPLX-001** `services/processor.py:89` - CC=23
  - Fix: Extract validation logic to separate function
...

### Beads Issues Created
- sec-0001: Fix SQL injection in auth handler
- cmplx-0002: Refactor processor.py complexity
```

---

## References

- **Pattern Details**: `references/patterns.md`
- **Report Formats**: `references/report-format.md`
- **Prescan Script**: `scripts/prescan.sh`
- **Expert Agents**: `~/.claude/agents/`
